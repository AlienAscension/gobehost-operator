/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
	"github.com/gobehost/operator/internal/adapter"
	"github.com/gobehost/operator/internal/reconciler"
)

const requeueInterval = 30 * time.Second

type GameServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

func (r *GameServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	gs := &gamesv1alpha1.GameServer{}
	if err := r.Get(ctx, req.NamespacedName, gs); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !gs.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, gs)
	}

	if !reconciler.HasFinalizer(gs) {
		if _, err := reconciler.AddFinalizer(ctx, r.Client, gs); err != nil {
			if errors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	a, err := adapter.Get(gs)
	if err != nil {
		log.Error(err, "Failed to get game adapter", "game-type", gs.Spec.Game.Type)
		reconciler.SetPhase(gs, gamesv1alpha1.GameServerPhaseFailed)
		reconciler.SetReady(gs, false, "AdapterNotFound", fmt.Sprintf("No adapter for game type %q", gs.Spec.Game.Type))
		_ = r.Status().Update(ctx, gs)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	_ = a

	if gs.Status.Phase == "" || gs.Status.Phase == gamesv1alpha1.GameServerPhasePending {
		reconciler.SetPhase(gs, gamesv1alpha1.GameServerPhaseProvisioning)
	}

	pvc := reconciler.BuildPVC(gs)
	if err := ctrl.SetControllerReference(gs, pvc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to reconcile PVC")
		reconciler.SetReady(gs, false, "PVCReconcileFailed", "Could not reconcile PVC")
		_ = r.Status().Update(ctx, gs)
		return ctrl.Result{}, err
	}
	log.V(1).Info("Reconciled PVC", "operation", op)

	headlessSvc := reconciler.BuildHeadlessService(gs)
	if err := ctrl.SetControllerReference(gs, headlessSvc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	op, err = controllerutil.CreateOrUpdate(ctx, r.Client, headlessSvc, func() error {
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to reconcile headless Service")
		reconciler.SetReady(gs, false, "ServiceReconcileFailed", "Could not reconcile headless Service")
		_ = r.Status().Update(ctx, gs)
		return ctrl.Result{}, err
	}
	log.V(1).Info("Reconciled headless Service", "operation", op)

	sts, err := reconciler.BuildStatefulSet(gs)
	if err != nil {
		log.Error(err, "Failed to build StatefulSet")
		return ctrl.Result{}, err
	}
	if err := ctrl.SetControllerReference(gs, sts, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	op, err = controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to reconcile StatefulSet")
		reconciler.SetReady(gs, false, "StatefulSetReconcileFailed", "Could not reconcile StatefulSet")
		_ = r.Status().Update(ctx, gs)
		return ctrl.Result{}, err
	}
	log.V(1).Info("Reconciled StatefulSet", "operation", op)

	extSvc := reconciler.BuildService(gs)
	if err := ctrl.SetControllerReference(gs, extSvc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	op, err = controllerutil.CreateOrUpdate(ctx, r.Client, extSvc, func() error {
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to reconcile external Service")
		reconciler.SetReady(gs, false, "ServiceReconcileFailed", "Could not reconcile external Service")
		_ = r.Status().Update(ctx, gs)
		return ctrl.Result{}, err
	}
	log.V(1).Info("Reconciled external Service", "operation", op)

	if err := r.Get(ctx, client.ObjectKeyFromObject(sts), sts); err != nil {
		log.Error(err, "Failed to get StatefulSet status")
	} else {
		if sts.Status.ReadyReplicas >= 1 {
			reconciler.SetPhase(gs, gamesv1alpha1.GameServerPhaseRunning)
			reconciler.SetReady(gs, true, "Available", "GameServer is running")
		} else {
			reconciler.SetReady(gs, false, "Provisioning", "StatefulSet not ready")
		}
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(extSvc), extSvc); err != nil {
		log.Error(err, "Failed to get Service for address update")
	} else {
		reconciler.UpdateAddress(gs, extSvc)
	}

	gs.Status.ObservedGeneration = gs.Generation
	if err := r.Status().Update(ctx, gs); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *GameServerReconciler) finalize(ctx context.Context, gs *gamesv1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	reconciler.SetPhase(gs, gamesv1alpha1.GameServerPhaseStopping)

	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(gs), sts); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	} else {
		if sts.Status.Replicas == 0 || (sts.Spec.Replicas != nil && *sts.Spec.Replicas != 0) {
			sts.Spec.Replicas = ptrTo(int32(0))
			if err := r.Update(ctx, sts); err != nil {
				if errors.IsConflict(err) {
					return ctrl.Result{Requeue: true}, nil
				}
				return ctrl.Result{}, err
			}
		}

		if sts.Status.ReadyReplicas > 0 {
			log.Info("Waiting for StatefulSet to scale down", "readyReplicas", sts.Status.ReadyReplicas)
			_ = r.Status().Update(ctx, gs)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	if err := reconciler.RemoveFinalizer(ctx, r.Client, gs); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Removed finalizer, GameServer will be deleted")
	return ctrl.Result{}, nil
}

func ptrTo[T any](v T) *T {
	return &v
}

func (r *GameServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gamesv1alpha1.GameServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Named("gameserver").
		Complete(r)
}
