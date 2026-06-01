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
	"encoding/json"
	"fmt"
	"hash/fnv"
	"maps"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
	"github.com/gobehost/operator/internal/reconciler"
)

const fleetRequeueInterval = 10 * time.Second

// GameServerFleetReconciler reconciles a GameServerFleet object
type GameServerFleetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameserverfleets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameserverfleets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameserverfleets/finalizers,verbs=update
// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *GameServerFleetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	fleet := &gamesv1alpha1.GameServerFleet{}
	if err := r.Get(ctx, req.NamespacedName, fleet); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !fleet.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, fleet)
	}

	if !hasFleetFinalizer(fleet) {
		if err := r.addFinalizer(ctx, fleet); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	gsList, err := r.listOwnedGameServers(ctx, fleet)
	if err != nil {
		log.Error(err, "Failed to list owned GameServers")
		return ctrl.Result{}, err
	}

	updatePhase := fleet.Annotations[gamesv1alpha1.UpdatePhaseAnnotation]

	switch updatePhase {
	case gamesv1alpha1.UpdatePhaseWaitingForReady:
		return r.handleWaitingForReady(ctx, fleet, gsList)
	case gamesv1alpha1.UpdatePhaseDrainingOld:
		return r.handleDrainingOld(ctx, fleet, gsList)
	default:
		return r.handleSteadyState(ctx, fleet, gsList)
	}
}

func (r *GameServerFleetReconciler) handleSteadyState(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet, gsList *gamesv1alpha1.GameServerList) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var currentGS *gamesv1alpha1.GameServer
	for i := range gsList.Items {
		gs := gsList.Items[i]
		if gs.Name == fleet.Status.CurrentGameServer {
			currentGS = &gs
			break
		}
	}

	if currentGS == nil && len(gsList.Items) == 1 {
		currentGS = &gsList.Items[0]
	} else if currentGS == nil && len(gsList.Items) == 0 {
		return r.createGameServer(ctx, fleet)
	} else if currentGS == nil {
		currentGS = &gsList.Items[0]
	}

	r.syncStatus(ctx, fleet, currentGS)

	if currentGS.Status.Phase == gamesv1alpha1.GameServerPhaseFailed {
		log.Info("GameServer in Failed phase, deleting and recreating", "gameserver", currentGS.Name)
		if err := r.Delete(ctx, currentGS); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete failed GameServer")
			return ctrl.Result{}, err
		}
		fleet.Status.CurrentGameServer = ""
		fleet.Status.Phase = gamesv1alpha1.FleetProgressing
		if err := r.Status().Update(ctx, fleet); err != nil {
			if errors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: fleetRequeueInterval}, nil
	}

	templateHash := computeTemplateHash(fleet.Spec.Template)
	currentHash := currentGS.Annotations[gamesv1alpha1.TemplateHashAnnotation]

	if currentHash != "" && currentHash != templateHash {
		log.Info("Template hash differs, starting rolling update", "current", currentHash, "desired", templateHash)
		return r.startRollingUpdate(ctx, fleet, currentGS, templateHash)
	}

	if currentHash == "" {
		patchedGS := currentGS.DeepCopy()
		if patchedGS.Annotations == nil {
			patchedGS.Annotations = make(map[string]string)
		}
		patchedGS.Annotations[gamesv1alpha1.TemplateHashAnnotation] = templateHash
		if err := r.Patch(ctx, patchedGS, client.MergeFrom(currentGS)); err != nil {
			log.Error(err, "Failed to patch GameServer template hash")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: fleetRequeueInterval}, nil
}

func (r *GameServerFleetReconciler) createGameServer(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	gs := r.buildGameServer(fleet, "")
	if err := ctrl.SetControllerReference(fleet, gs, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, gs); err != nil {
		log.Error(err, "Failed to create GameServer", "name", gs.Name)
		return ctrl.Result{}, err
	}

	log.Info("Created GameServer", "name", gs.Name)

	fleet.Status.CurrentGameServer = gs.Name
	fleet.Status.Phase = gamesv1alpha1.FleetProgressing
	setFleetCondition(fleet, gamesv1alpha1.FleetProgressingCondition, metav1.ConditionTrue, "Creating", "GameServer created")
	if err := r.Status().Update(ctx, fleet); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	return r.ensureFleetService(ctx, fleet, gs)
}

func (r *GameServerFleetReconciler) startRollingUpdate(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet, currentGS *gamesv1alpha1.GameServer, newHash string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if fleet.Spec.Strategy.Type == gamesv1alpha1.RecreateStrategyType {
		log.Info("Using Recreate strategy, deleting current GameServer", "gameserver", currentGS.Name)

		fleet.Status.History = append(fleet.Status.History, gamesv1alpha1.RolloutRecord{
			StartedAt:   metav1.Now(),
			FromVersion: currentGS.Spec.Game.Version,
			ToVersion:   fleet.Spec.Template.Spec.Game.Version,
		})

		if err := r.Delete(ctx, currentGS); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete current GameServer")
			return ctrl.Result{}, err
		}

		fleet.Status.CurrentGameServer = ""
		fleet.Status.Phase = gamesv1alpha1.FleetProgressing
		if err := r.Status().Update(ctx, fleet); err != nil {
			if errors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: fleetRequeueInterval}, nil
	}

	log.Info("Starting RollingUpdate strategy", "currentGS", currentGS.Name)
	newGS := r.buildGameServer(fleet, newHash[:8])
	newGS.Annotations[gamesv1alpha1.TemplateHashAnnotation] = newHash
	if err := ctrl.SetControllerReference(fleet, newGS, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, newGS); err != nil {
		log.Error(err, "Failed to create new GameServer for rolling update", "name", newGS.Name)
		return ctrl.Result{}, err
	}

	log.Info("Created new GameServer for rolling update", "name", newGS.Name)

	patchedFleet := fleet.DeepCopy()
	if patchedFleet.Annotations == nil {
		patchedFleet.Annotations = make(map[string]string)
	}
	patchedFleet.Annotations[gamesv1alpha1.UpdatePhaseAnnotation] = gamesv1alpha1.UpdatePhaseWaitingForReady
	if err := r.Patch(ctx, patchedFleet, client.MergeFrom(fleet)); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.Get(ctx, types.NamespacedName{Name: fleet.Name, Namespace: fleet.Namespace}, patchedFleet); err != nil {
		return ctrl.Result{}, err
	}
	patchedFleet.Status.UpdatedGameServer = newGS.Name
	patchedFleet.Status.Phase = gamesv1alpha1.FleetProgressing
	patchedFleet.Status.History = append(patchedFleet.Status.History, gamesv1alpha1.RolloutRecord{
		StartedAt:   metav1.Now(),
		FromVersion: currentGS.Spec.Game.Version,
		ToVersion:   fleet.Spec.Template.Spec.Game.Version,
	})
	setFleetCondition(patchedFleet, gamesv1alpha1.FleetProgressingCondition, metav1.ConditionTrue, "RollingUpdate", "Rolling update in progress")
	if err := r.Status().Update(ctx, patchedFleet); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: fleetRequeueInterval}, nil
}

func (r *GameServerFleetReconciler) handleWaitingForReady(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet, gsList *gamesv1alpha1.GameServerList) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	updatedGSName := fleet.Status.UpdatedGameServer
	if updatedGSName == "" {
		log.Info("No updated GameServer name in status, clearing update phase")
		return r.clearUpdatePhase(ctx, fleet)
	}

	var updatedGS *gamesv1alpha1.GameServer
	var currentGS *gamesv1alpha1.GameServer
	for i := range gsList.Items {
		if gsList.Items[i].Name == updatedGSName {
			updatedGS = &gsList.Items[i]
		} else {
			currentGS = &gsList.Items[i]
		}
	}

	if updatedGS == nil {
		log.Info("Updated GameServer not found, clearing update phase", "name", updatedGSName)
		return r.clearUpdatePhase(ctx, fleet)
	}

	r.syncStatus(ctx, fleet, updatedGS)

	if updatedGS.Status.Phase != gamesv1alpha1.GameServerPhaseRunning || !updatedGS.Status.Ready {
		log.Info("Waiting for updated GameServer to be ready", "name", updatedGS.Name, "phase", updatedGS.Status.Phase, "ready", updatedGS.Status.Ready)
		return ctrl.Result{RequeueAfter: fleetRequeueInterval}, nil
	}

	log.Info("Updated GameServer is ready, proceeding to drain old", "name", updatedGS.Name)

	if currentGS != nil {
		patchedGS := currentGS.DeepCopy()
		if patchedGS.Annotations == nil {
			patchedGS.Annotations = make(map[string]string)
		}
		patchedGS.Annotations["games.gobehost.com/drain"] = "true"
		if err := r.Patch(ctx, patchedGS, client.MergeFrom(currentGS)); err != nil {
			log.Error(err, "Failed to annotate old GameServer for drain", "name", currentGS.Name)
			return ctrl.Result{}, err
		}

		patchedFleet := fleet.DeepCopy()
		if patchedFleet.Annotations == nil {
			patchedFleet.Annotations = make(map[string]string)
		}
		patchedFleet.Annotations[gamesv1alpha1.UpdatePhaseAnnotation] = gamesv1alpha1.UpdatePhaseDrainingOld
		if err := r.Patch(ctx, patchedFleet, client.MergeFrom(fleet)); err != nil {
			if errors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return r.completeRollingUpdate(ctx, fleet, nil, updatedGS)
}

func (r *GameServerFleetReconciler) handleDrainingOld(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet, gsList *gamesv1alpha1.GameServerList) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	updatedGSName := fleet.Status.UpdatedGameServer
	var updatedGS *gamesv1alpha1.GameServer
	var oldGS *gamesv1alpha1.GameServer
	for i := range gsList.Items {
		if gsList.Items[i].Name == updatedGSName {
			updatedGS = &gsList.Items[i]
		} else {
			oldGS = &gsList.Items[i]
		}
	}

	if oldGS == nil {
		log.Info("Old GameServer already gone, completing rolling update")
		if updatedGS != nil {
			return r.completeRollingUpdate(ctx, fleet, nil, updatedGS)
		}
		return r.clearUpdatePhase(ctx, fleet)
	}

	if oldGS.Status.Phase == gamesv1alpha1.GameServerPhaseStopped || oldGS.DeletionTimestamp != nil {
		log.Info("Old GameServer stopped or being deleted, completing cutover", "name", oldGS.Name)
		if updatedGS != nil {
			return r.completeRollingUpdate(ctx, fleet, oldGS, updatedGS)
		}
		return r.clearUpdatePhase(ctx, fleet)
	}

	log.Info("Waiting for old GameServer to stop", "name", oldGS.Name, "phase", oldGS.Status.Phase)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *GameServerFleetReconciler) completeRollingUpdate(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet, oldGS *gamesv1alpha1.GameServer, updatedGS *gamesv1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if oldGS != nil {
		if err := r.Delete(ctx, oldGS); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete old GameServer", "name", oldGS.Name)
			return ctrl.Result{}, err
		}
		log.Info("Deleted old GameServer", "name", oldGS.Name)
	}

	fleet.Status.CurrentGameServer = updatedGS.Name
	fleet.Status.UpdatedGameServer = ""
	fleet.Status.Phase = gamesv1alpha1.FleetAvailable
	fleet.Status.ReadyReplicas = 1
	setFleetCondition(fleet, gamesv1alpha1.FleetAvailableCondition, metav1.ConditionTrue, "RollingUpdateComplete", "GameServer rolling update completed")
	setFleetCondition(fleet, gamesv1alpha1.FleetProgressingCondition, metav1.ConditionFalse, "RollingUpdateComplete", "Rolling update completed")

	if len(fleet.Status.History) > 0 {
		lastIdx := len(fleet.Status.History) - 1
		now := metav1.Now()
		fleet.Status.History[lastIdx].CompletedAt = &now
		fleet.Status.History[lastIdx].Result = "Success"
		fleet.Status.History[lastIdx].Message = fmt.Sprintf("Rolling update to version %s completed", updatedGS.Spec.Game.Version)
		if len(fleet.Status.History) > 10 {
			fleet.Status.History = fleet.Status.History[len(fleet.Status.History)-10:]
		}
	}

	fleet.Status.ObservedGeneration = fleet.Generation

	patchedFleet := fleet.DeepCopy()
	if patchedFleet.Annotations != nil {
		delete(patchedFleet.Annotations, gamesv1alpha1.UpdatePhaseAnnotation)
	}
	if err := r.Patch(ctx, patchedFleet, client.MergeFrom(fleet)); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	fleet.Annotations = patchedFleet.Annotations

	if err := r.Status().Update(ctx, fleet); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	svc := &corev1.Service{}
	svcErr := r.Get(ctx, types.NamespacedName{Name: fleet.Name, Namespace: fleet.Namespace}, svc)
	if svcErr == nil {
		if err := r.updateFleetServiceSelector(ctx, fleet, updatedGS, svc); err != nil {
			log.Error(err, "Failed to update fleet Service selector")
			return ctrl.Result{}, err
		}
	} else if !errors.IsNotFound(svcErr) {
		log.Error(svcErr, "Failed to get fleet Service")
	}

	log.Info("Rolling update completed", "gameserver", updatedGS.Name)
	return ctrl.Result{RequeueAfter: fleetRequeueInterval}, nil
}

func (r *GameServerFleetReconciler) clearUpdatePhase(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet) (ctrl.Result, error) {
	patchedFleet := fleet.DeepCopy()
	if patchedFleet.Annotations != nil {
		delete(patchedFleet.Annotations, gamesv1alpha1.UpdatePhaseAnnotation)
	}
	if err := r.Patch(ctx, patchedFleet, client.MergeFrom(fleet)); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *GameServerFleetReconciler) syncStatus(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet, gs *gamesv1alpha1.GameServer) {
	log := logf.FromContext(ctx)

	if gs == nil {
		fleet.Status.ReadyReplicas = 0
		fleet.Status.Phase = gamesv1alpha1.FleetProgressing
		setFleetCondition(fleet, gamesv1alpha1.FleetAvailableCondition, metav1.ConditionFalse, "NoGameServer", "No GameServer found")
		return
	}

	if gs.Status.Ready {
		fleet.Status.ReadyReplicas = 1
		fleet.Status.Phase = gamesv1alpha1.FleetAvailable
		setFleetCondition(fleet, gamesv1alpha1.FleetAvailableCondition, metav1.ConditionTrue, "Available", "GameServer is ready")
		setFleetCondition(fleet, gamesv1alpha1.FleetProgressingCondition, metav1.ConditionFalse, "Available", "GameServer is ready")
	} else {
		fleet.Status.ReadyReplicas = 0
		switch gs.Status.Phase {
		case gamesv1alpha1.GameServerPhaseFailed:
			fleet.Status.Phase = gamesv1alpha1.FleetFailed
			setFleetCondition(fleet, gamesv1alpha1.FleetDegradedCondition, metav1.ConditionTrue, "GameServerFailed", "GameServer is in Failed phase")
		default:
			fleet.Status.Phase = gamesv1alpha1.FleetDegraded
			setFleetCondition(fleet, gamesv1alpha1.FleetDegradedCondition, metav1.ConditionTrue, "GameServerNotReady", "GameServer is not ready")
		}
		setFleetCondition(fleet, gamesv1alpha1.FleetAvailableCondition, metav1.ConditionFalse, "NotAvailable", "GameServer is not ready")
	}

	fleet.Status.ObservedGeneration = fleet.Generation

	if err := r.Status().Update(ctx, fleet); err != nil {
		log.Error(err, "Failed to update fleet status")
	}
}

func (r *GameServerFleetReconciler) ensureFleetService(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet, gs *gamesv1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	svc := reconciler.BuildFleetService(fleet, gs)
	if err := ctrl.SetControllerReference(fleet, svc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Selector = reconciler.GameServerLabels(gs)
		svc.Spec.Type = gs.Spec.Network.ServiceType
		svc.Spec.Ports = buildServicePorts(gs)
		svc.Annotations = gs.Spec.Network.Annotations
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to reconcile fleet Service")
		return ctrl.Result{}, err
	}
	log.V(1).Info("Reconciled fleet Service", "operation", op)

	return ctrl.Result{RequeueAfter: fleetRequeueInterval}, nil
}

func (r *GameServerFleetReconciler) updateFleetServiceSelector(ctx context.Context, _ *gamesv1alpha1.GameServerFleet, gs *gamesv1alpha1.GameServer, svc *corev1.Service) error {
	patched := svc.DeepCopy()
	patched.Spec.Selector = reconciler.GameServerLabels(gs)
	return r.Patch(ctx, patched, client.MergeFrom(svc))
}

func (r *GameServerFleetReconciler) listOwnedGameServers(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet) (*gamesv1alpha1.GameServerList, error) {
	gsList := &gamesv1alpha1.GameServerList{}
	if err := r.List(ctx, gsList,
		client.InNamespace(fleet.Namespace),
		client.MatchingLabels{gamesv1alpha1.FleetNameLabel: fleet.Name},
	); err != nil {
		return nil, err
	}
	return gsList, nil
}

func (r *GameServerFleetReconciler) buildGameServer(fleet *gamesv1alpha1.GameServerFleet, nameSuffix string) *gamesv1alpha1.GameServer {
	template := fleet.Spec.Template
	gsName := fleet.Name + "-gs"
	if nameSuffix != "" {
		gsName = fleet.Name + "-" + nameSuffix
	}

	gs := &gamesv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:        gsName,
			Namespace:   fleet.Namespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: template.Spec,
	}

	maps.Copy(gs.Labels, template.Labels)
	maps.Copy(gs.Annotations, template.Annotations)
	gs.Labels[gamesv1alpha1.FleetNameLabel] = fleet.Name
	gs.Annotations[gamesv1alpha1.TemplateHashAnnotation] = computeTemplateHash(template)

	return gs
}

func (r *GameServerFleetReconciler) finalize(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	gsList, err := r.listOwnedGameServers(ctx, fleet)
	if err != nil {
		return ctrl.Result{}, err
	}

	stillExists := false
	for i := range gsList.Items {
		gs := &gsList.Items[i]
		if gs.DeletionTimestamp != nil {
			stillExists = true
			continue
		}
		if containsString(gs.Finalizers, gamesv1alpha1.GameServerFinalizer) {
			gs.Finalizers = removeString(gs.Finalizers, gamesv1alpha1.GameServerFinalizer)
			if err := r.Update(ctx, gs); err != nil {
				if errors.IsConflict(err) {
					return ctrl.Result{Requeue: true}, nil
				}
				log.Error(err, "Failed to remove GameServer finalizer", "name", gs.Name)
				return ctrl.Result{}, err
			}
		}
		if err := r.Delete(ctx, gs); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete owned GameServer", "name", gs.Name)
			return ctrl.Result{}, err
		}
		stillExists = true
	}

	if stillExists {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	svc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: fleet.Name, Namespace: fleet.Namespace}, svc); err == nil {
		if svc.DeletionTimestamp == nil {
			if err := r.Delete(ctx, svc); err != nil && !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete fleet Service")
				return ctrl.Result{}, err
			}
		}
	}

	if len(gsList.Items) > 0 {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	patchedFleet := fleet.DeepCopy()
	patchedFleet.Finalizers = removeString(patchedFleet.Finalizers, gamesv1alpha1.GameServerFleetFinalizer)
	if err := r.Update(ctx, patchedFleet); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Removed finalizer, GameServerFleet will be deleted")
	return ctrl.Result{}, nil
}

func (r *GameServerFleetReconciler) addFinalizer(ctx context.Context, fleet *gamesv1alpha1.GameServerFleet) error {
	patchedFleet := fleet.DeepCopy()
	patchedFleet.Finalizers = append(patchedFleet.Finalizers, gamesv1alpha1.GameServerFleetFinalizer)
	return r.Update(ctx, patchedFleet)
}

func hasFleetFinalizer(fleet *gamesv1alpha1.GameServerFleet) bool {
	return slices.Contains(fleet.Finalizers, gamesv1alpha1.GameServerFleetFinalizer)
}

func computeTemplateHash(template gamesv1alpha1.GameServerTemplate) string {
	h := fnv.New32a()
	data, _ := json.Marshal(template.Spec)
	h.Write(data)
	return fmt.Sprintf("%08x", h.Sum32())
}

func setFleetCondition(fleet *gamesv1alpha1.GameServerFleet, condType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	for i, c := range fleet.Status.Conditions {
		if c.Type == condType {
			fleet.Status.Conditions[i] = metav1.Condition{
				Type:               condType,
				Status:             status,
				Reason:             reason,
				Message:            message,
				ObservedGeneration: fleet.Status.ObservedGeneration,
				LastTransitionTime: now,
			}
			return
		}
	}
	fleet.Status.Conditions = append(fleet.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: fleet.Status.ObservedGeneration,
		LastTransitionTime: now,
	})
}

func removeString(slice []string, s string) []string {
	return slices.DeleteFunc(slice, func(item string) bool { return item == s })
}

func containsString(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

func buildServicePorts(gs *gamesv1alpha1.GameServer) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, len(gs.Spec.Network.Ports))
	for _, p := range gs.Spec.Network.Ports {
		sp := corev1.ServicePort{
			Name:     p.Name,
			Port:     p.Port,
			Protocol: p.Protocol,
		}
		if p.TargetPort != nil {
			sp.TargetPort = intstr.FromInt(int(*p.TargetPort))
		} else {
			sp.TargetPort = intstr.FromInt(int(p.Port))
		}
		ports = append(ports, sp)
	}
	return ports
}

// SetupWithManager sets up the controller with the Manager.
func (r *GameServerFleetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gamesv1alpha1.GameServerFleet{}).
		Owns(&gamesv1alpha1.GameServer{}).
		Owns(&corev1.Service{}).
		Named("gameserverfleet").
		Complete(r)
}
