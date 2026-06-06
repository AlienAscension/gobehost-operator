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
	"slices"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
	"github.com/gobehost/operator/internal/reconciler"
)

const backupRequeueInterval = 30 * time.Second

type GameServerBackupReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	BackupConfigNamespace string
}

// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameserverbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameserverbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameserverbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=games.gobehost.com,resources=gameserverfleets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

func (r *GameServerBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	backup := &gamesv1alpha1.GameServerBackup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !backup.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, backup)
	}

	if !hasBackupFinalizer(backup) {
		if err := r.addFinalizer(ctx, backup); err != nil {
			if errors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	targetKind, targetName, err := r.resolveTarget(ctx, backup)
	if err != nil {
		return ctrl.Result{}, err
	}
	if targetKind == "" {
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	}

	cfg, err := r.resolveStorageConfig(ctx, backup)
	if err != nil {
		log.Error(err, "Failed to resolve storage config")
		setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupStorageMissing, "Storage config is missing or invalid")
		backup.Status.ObservedGeneration = backup.Generation
		_ = r.Status().Update(ctx, backup)
		return ctrl.Result{}, err
	}
	if cfg == nil {
		log.Info("Storage config not available")
		setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupStorageMissing, "Storage config not found")
		backup.Status.ObservedGeneration = backup.Generation
		_ = r.Status().Update(ctx, backup)
		return ctrl.Result{RequeueAfter: backupRequeueInterval}, nil
	}

	pvcResult, err := r.checkPVC(ctx, backup, targetKind, targetName)
	if err != nil || pvcResult != "" {
		if pvcResult != "" {
			return ctrl.Result{RequeueAfter: backupRequeueInterval}, nil
		}
		return ctrl.Result{}, err
	}

	pvcName := reconciler.GetPVCName("GameServer", targetName)
	if err := r.reconcileCronJob(ctx, backup, cfg, targetKind, targetName, pvcName); err != nil {
		return ctrl.Result{}, err
	}

	r.updateBackupJobStatus(ctx, backup)

	setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionTrue, gamesv1alpha1.BackupCronJobCreated, "CronJob created")
	backup.Status.ObservedGeneration = backup.Generation
	if err := r.Status().Update(ctx, backup); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: backupRequeueInterval}, nil
}

func (r *GameServerBackupReconciler) resolveTarget(ctx context.Context, backup *gamesv1alpha1.GameServerBackup) (string, string, error) {
	log := logf.FromContext(ctx)
	targetKind := backup.Spec.TargetRef.Kind
	targetName := backup.Spec.TargetRef.Name

	if targetKind == "GameServerFleet" {
		fleet := &gamesv1alpha1.GameServerFleet{}
		if err := r.Get(ctx, types.NamespacedName{Name: targetName, Namespace: backup.Namespace}, fleet); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Target GameServerFleet not found", "name", targetName)
				setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupTargetNotFound, "GameServerFleet not found")
				backup.Status.ObservedGeneration = backup.Generation
				_ = r.Status().Update(ctx, backup)
				return "", "", nil
			}
			return "", "", err
		}
		if fleet.Status.CurrentGameServer == "" {
			log.Info("GameServerFleet has no current GameServer", "name", targetName)
			setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupTargetNotFound, "GameServerFleet has no current GameServer")
			backup.Status.ObservedGeneration = backup.Generation
			_ = r.Status().Update(ctx, backup)
			return "", "", nil
		}
		return "GameServer", fleet.Status.CurrentGameServer, nil
	}

	gs := &gamesv1alpha1.GameServer{}
	if err := r.Get(ctx, types.NamespacedName{Name: targetName, Namespace: backup.Namespace}, gs); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Target GameServer not found", "name", targetName)
			setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupTargetNotFound, "GameServer not found")
			backup.Status.ObservedGeneration = backup.Generation
			_ = r.Status().Update(ctx, backup)
			return "", "", nil
		}
		return "", "", err
	}
	return targetKind, targetName, nil
}

func (r *GameServerBackupReconciler) checkPVC(ctx context.Context, backup *gamesv1alpha1.GameServerBackup, targetKind, targetName string) (string, error) {
	log := logf.FromContext(ctx)
	pvcName := reconciler.GetPVCName(targetKind, targetName)
	if pvcName == "" {
		log.Info("Cannot determine PVC name for target", "targetKind", targetKind, "targetName", targetName)
		setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupPVCNotReady, "Cannot determine PVC name for target")
		backup.Status.ObservedGeneration = backup.Generation
		_ = r.Status().Update(ctx, backup)
		return "requeue", nil
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: backup.Namespace}, pvc); err != nil {
		if errors.IsNotFound(err) {
			log.Info("PVC not found", "pvcName", pvcName)
			setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupPVCNotReady, "PVC not found")
			backup.Status.ObservedGeneration = backup.Generation
			_ = r.Status().Update(ctx, backup)
			return "requeue", nil
		}
		return "", err
	}
	if pvc.Status.Phase != corev1.ClaimBound {
		log.Info("PVC not bound", "pvcName", pvcName, "phase", pvc.Status.Phase)
		setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupPVCNotReady, "PVC is not bound")
		backup.Status.ObservedGeneration = backup.Generation
		_ = r.Status().Update(ctx, backup)
		return "requeue", nil
	}
	return "", nil
}

func (r *GameServerBackupReconciler) reconcileCronJob(ctx context.Context, backup *gamesv1alpha1.GameServerBackup, cfg *reconciler.BackupConfig, targetKind, targetName, pvcName string) error {
	log := logf.FromContext(ctx)
	cj := reconciler.BuildCronJob(backup, cfg, targetKind, targetName, pvcName)
	if err := ctrl.SetControllerReference(backup, cj, r.Scheme); err != nil {
		return err
	}

	existingCJ := &batchv1.CronJob{}
	err := r.Get(ctx, types.NamespacedName{Name: cj.Name, Namespace: cj.Namespace}, existingCJ)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		if err := r.Create(ctx, cj); err != nil {
			log.Error(err, "Failed to create CronJob", "name", cj.Name)
			setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupCronJobFailed, "Failed to create CronJob")
			backup.Status.ObservedGeneration = backup.Generation
			_ = r.Status().Update(ctx, backup)
			return err
		}
		log.Info("Created CronJob", "name", cj.Name)
	} else {
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, existingCJ, func() error {
			existingCJ.Spec.Schedule = cj.Spec.Schedule
			existingCJ.Spec.SuccessfulJobsHistoryLimit = cj.Spec.SuccessfulJobsHistoryLimit
			existingCJ.Spec.FailedJobsHistoryLimit = cj.Spec.FailedJobsHistoryLimit
			existingCJ.Spec.JobTemplate = cj.Spec.JobTemplate
			existingCJ.Labels = cj.Labels
			return nil
		})
		if err != nil {
			log.Error(err, "Failed to update CronJob", "name", cj.Name)
			setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupCronJobFailed, "Failed to update CronJob")
			backup.Status.ObservedGeneration = backup.Generation
			_ = r.Status().Update(ctx, backup)
			return err
		}
		log.V(1).Info("Reconciled CronJob", "operation", op)
	}
	return nil
}

func (r *GameServerBackupReconciler) updateBackupJobStatus(ctx context.Context, backup *gamesv1alpha1.GameServerBackup) {
	log := logf.FromContext(ctx)
	jobList := &batchv1.JobList{}
	if err := r.List(ctx, jobList,
		client.InNamespace(backup.Namespace),
		client.MatchingLabels(reconciler.GameServerBackupLabels(backup)),
	); err != nil {
		log.Error(err, "Failed to list Jobs for backup")
		return
	}
	if len(jobList.Items) == 0 {
		return
	}

	latestJob := &jobList.Items[0]
	for i := range jobList.Items {
		if jobList.Items[i].CreationTimestamp.After(latestJob.CreationTimestamp.Time) {
			latestJob = &jobList.Items[i]
		}
	}

	for _, c := range latestJob.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			setBackupCondition(backup, gamesv1alpha1.BackupSucceeded, metav1.ConditionTrue, "Succeeded", "Last backup job succeeded")
			backup.Status.LastBackupStatus = "Success"
			if latestJob.Status.CompletionTime != nil {
				backup.Status.LastBackupTime = latestJob.Status.CompletionTime
			}
			break
		}
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			setBackupCondition(backup, gamesv1alpha1.BackupSucceeded, metav1.ConditionFalse, gamesv1alpha1.BackupCronJobFailed, c.Message)
			backup.Status.LastBackupStatus = "Failed"
			recordEvent(ctx, backup, "Warning", "BackupFailed", c.Message)
			break
		}
	}
}

func (r *GameServerBackupReconciler) finalize(ctx context.Context, backup *gamesv1alpha1.GameServerBackup) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	backupOnDelete := true
	if backup.Spec.BackupOnDelete != nil {
		backupOnDelete = *backup.Spec.BackupOnDelete
	}

	if backupOnDelete {
		targetKind := backup.Spec.TargetRef.Kind
		targetName := backup.Spec.TargetRef.Name

		if targetKind == "GameServerFleet" {
			fleet := &gamesv1alpha1.GameServerFleet{}
			if err := r.Get(ctx, types.NamespacedName{Name: targetName, Namespace: backup.Namespace}, fleet); err != nil {
				if errors.IsNotFound(err) {
					log.Info("Target GameServerFleet not found during finalization, removing finalizer", "name", targetName)
					return r.removeFinalizer(ctx, backup)
				}
				return ctrl.Result{}, err
			}
			if fleet.Status.CurrentGameServer != "" {
				targetKind = "GameServer"
				targetName = fleet.Status.CurrentGameServer
			}
		}

		cfg, err := r.resolveStorageConfig(ctx, backup)
		if err != nil || cfg == nil {
			log.Info("Storage config not available during finalization, removing finalizer")
			return r.removeFinalizer(ctx, backup)
		}

		pvcName := reconciler.GetPVCName("GameServer", targetName)
		if pvcName == "" {
			log.Info("Cannot determine PVC name during finalization, removing finalizer")
			return r.removeFinalizer(ctx, backup)
		}

		jobName := backup.Name + "-backup-on-delete"
		existingJob := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: backup.Namespace}, existingJob); err != nil {
			if errors.IsNotFound(err) {
				deleteJob := reconciler.BuildBackupOnDeleteJob(backup, cfg, targetKind, targetName, pvcName)
				if err := ctrl.SetControllerReference(backup, deleteJob, r.Scheme); err != nil {
					return ctrl.Result{}, err
				}
				if err := r.Create(ctx, deleteJob); err != nil {
					log.Error(err, "Failed to create backup-on-delete Job")
					return ctrl.Result{}, err
				}
				log.Info("Created backup-on-delete Job", "name", deleteJob.Name)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			return ctrl.Result{}, err
		}

		for _, c := range existingJob.Status.Conditions {
			if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
				log.Info("Backup-on-delete Job completed successfully")
				return r.removeFinalizer(ctx, backup)
			}
			if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
				log.Info("Backup-on-delete Job failed, removing finalizer anyway", "message", c.Message)
				return r.removeFinalizer(ctx, backup)
			}
		}

		log.Info("Waiting for backup-on-delete Job to complete", "name", jobName)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return r.removeFinalizer(ctx, backup)
}

func (r *GameServerBackupReconciler) removeFinalizer(ctx context.Context, backup *gamesv1alpha1.GameServerBackup) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	if !hasBackupFinalizer(backup) {
		return ctrl.Result{}, nil
	}
	backup.Finalizers = removeString(backup.Finalizers, gamesv1alpha1.GameServerBackupFinalizer)
	if err := r.Update(ctx, backup); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	log.Info("Removed finalizer, GameServerBackup will be deleted")
	return ctrl.Result{}, nil
}

func (r *GameServerBackupReconciler) addFinalizer(ctx context.Context, backup *gamesv1alpha1.GameServerBackup) error {
	backup.Finalizers = append(backup.Finalizers, gamesv1alpha1.GameServerBackupFinalizer)
	return r.Update(ctx, backup)
}

func hasBackupFinalizer(backup *gamesv1alpha1.GameServerBackup) bool {
	return slices.Contains(backup.Finalizers, gamesv1alpha1.GameServerBackupFinalizer)
}

func (r *GameServerBackupReconciler) resolveStorageConfig(ctx context.Context, backup *gamesv1alpha1.GameServerBackup) (*reconciler.BackupConfig, error) {
	log := logf.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	cmName := "gobehost-backup-config"
	if err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: r.BackupConfigNamespace}, cm); err != nil {
		if errors.IsNotFound(err) {
			if backup.Spec.Storage == nil {
				log.Info("Backup config ConfigMap not found and no spec overrides")
				return nil, nil
			}
		} else {
			return nil, err
		}
	}

	platformConfig := &reconciler.BackupConfig{}
	if cm.Data != nil {
		platformConfig.Endpoint = cm.Data["endpoint"]
		platformConfig.Bucket = cm.Data["bucket"]
		platformConfig.Path = cm.Data["path"]
		platformConfig.SecretName = cm.Data["secretName"]
	}

	if backup.Spec.Storage != nil && backup.Spec.Storage.SecretRef != nil {
		secretRef := backup.Spec.Storage.SecretRef
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: secretRef.Name, Namespace: backup.Namespace}, secret); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Referenced Secret not found", "secretName", secretRef.Name)
				setBackupCondition(backup, gamesv1alpha1.BackupReady, metav1.ConditionFalse, gamesv1alpha1.BackupInvalidCreds, "Referenced Secret not found")
				backup.Status.ObservedGeneration = backup.Generation
				_ = r.Status().Update(ctx, backup)
				return nil, nil
			}
			return nil, err
		}
		_ = secret
	}

	cfg := reconciler.ResolveStorageConfig(backup, platformConfig)
	return cfg, nil
}

func setBackupCondition(backup *gamesv1alpha1.GameServerBackup, condType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	cond := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: backup.Status.ObservedGeneration,
		LastTransitionTime: now,
	}
	for i, c := range backup.Status.Conditions {
		if c.Type == condType {
			backup.Status.Conditions[i] = cond
			return
		}
	}
	backup.Status.Conditions = append(backup.Status.Conditions, cond)
}

func recordEvent(ctx context.Context, backup *gamesv1alpha1.GameServerBackup, eventType, reason, message string) {
	log := logf.FromContext(ctx)
	log.Info("Recording event", "type", eventType, "reason", reason, "message", message)
}

func (r *GameServerBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gamesv1alpha1.GameServerBackup{}).
		Owns(&batchv1.CronJob{}).
		Named("gameserverbackup").
		Complete(r)
}
