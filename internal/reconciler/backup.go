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

package reconciler

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

type BackupConfig struct {
	Endpoint        string
	Bucket          string
	Path            string
	SecretName      string
	SecretNamespace string
}

func ResolveStorageConfig(backup *gamesv1alpha1.GameServerBackup, platformConfig *BackupConfig) *BackupConfig {
	cfg := &BackupConfig{
		Endpoint:        platformConfig.Endpoint,
		Bucket:          platformConfig.Bucket,
		Path:            platformConfig.Path,
		SecretName:      platformConfig.SecretName,
		SecretNamespace: platformConfig.SecretNamespace,
	}

	if backup.Spec.Storage != nil {
		storage := backup.Spec.Storage
		if storage.Endpoint != "" {
			cfg.Endpoint = storage.Endpoint
		}
		if storage.Bucket != "" {
			cfg.Bucket = storage.Bucket
		}
		if storage.Path != "" {
			cfg.Path = storage.Path
		}
		if storage.SecretRef != nil && storage.SecretRef.Name != "" {
			cfg.SecretName = storage.SecretRef.Name
		}
	}

	if cfg.Path == "" {
		cfg.Path = backup.Namespace + "/" + backup.Spec.TargetRef.Name
	}

	cfg.SecretNamespace = backup.Namespace

	return cfg
}

const rcloneCommand = `/usr/local/bin/rclone copy /data "s3_backup:$BACKUP_BUCKET/$BACKUP_PATH/$(date -u +%Y-%m-%dT%H-%M-%SZ)/"`

func buildBackupEnv(backup *gamesv1alpha1.GameServerBackup, cfg *BackupConfig) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0, 8)
	env = append(env,
		corev1.EnvVar{Name: "RCLONE_CONFIG_S3_BACKUP_TYPE", Value: "s3"},
		corev1.EnvVar{Name: "RCLONE_CONFIG_S3_BACKUP_PROVIDER", Value: "Other"},
		corev1.EnvVar{Name: "RCLONE_CONFIG_S3_BACKUP_ENDPOINT", Value: cfg.Endpoint},
		corev1.EnvVar{Name: "RCLONE_CONFIG_S3_BACKUP_ACCESS_KEY_ID", ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cfg.SecretName,
				},
				Key: "S3_ACCESS_KEY",
			},
		}},
		corev1.EnvVar{Name: "RCLONE_CONFIG_S3_BACKUP_SECRET_ACCESS_KEY", ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cfg.SecretName,
				},
				Key: "S3_SECRET_KEY",
			},
		}},
		corev1.EnvVar{Name: "BACKUP_PATH", Value: cfg.Path},
		corev1.EnvVar{Name: "BACKUP_BUCKET", Value: cfg.Bucket},
	)

	retention := int32(5)
	if backup.Spec.Retention != nil {
		retention = *backup.Spec.Retention
	}
	env = append(env, corev1.EnvVar{
		Name:  "BACKUP_RETENTION",
		Value: fmtInt32(retention),
	})

	includeMetadata := true
	if backup.Spec.IncludeMetadata != nil {
		includeMetadata = *backup.Spec.IncludeMetadata
	}
	env = append(env, corev1.EnvVar{
		Name:  "INCLUDE_METADATA",
		Value: fmtBool(includeMetadata),
	})

	return env
}

func buildBackupPod(env []corev1.EnvVar, pvcName string) corev1.PodSpec {
	const volumeName = "data"
	container := corev1.Container{
		Name:    "backup",
		Image:   "rclone/rclone:latest",
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{rcloneCommand},
		Env:     env,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeName,
				MountPath: "/data",
				ReadOnly:  true,
			},
		},
	}

	return corev1.PodSpec{
		Containers:    []corev1.Container{container},
		RestartPolicy: corev1.RestartPolicyOnFailure,
		Volumes: []corev1.Volume{
			{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
						ReadOnly:  true,
					},
				},
			},
		},
	}
}

func BuildCronJob(backup *gamesv1alpha1.GameServerBackup, cfg *BackupConfig, targetKind, targetName, pvcName string) *batchv1.CronJob {
	env := buildBackupEnv(backup, cfg)
	podSpec := buildBackupPod(env, pvcName)
	labels := GameServerBackupLabels(backup)

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name + "-backup",
			Namespace: backup.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   backup.Spec.Schedule,
			SuccessfulJobsHistoryLimit: ptr.To(int32(3)),
			FailedJobsHistoryLimit:     ptr.To(int32(3)),
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: podSpec,
					},
				},
			},
		},
	}
}

func BuildBackupOnDeleteJob(backup *gamesv1alpha1.GameServerBackup, cfg *BackupConfig, targetKind, targetName, pvcName string) *batchv1.Job {
	env := buildBackupEnv(backup, cfg)
	podSpec := buildBackupPod(env, pvcName)
	labels := GameServerBackupLabels(backup)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name + "-backup-on-delete",
			Namespace: backup.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}
}

func GameServerBackupLabels(backup *gamesv1alpha1.GameServerBackup) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "gameserverbackup",
		"app.kubernetes.io/instance":   backup.Name,
		"app.kubernetes.io/managed-by": "gobehost-operator",
		"games.gobehost.com/backup":    backup.Name,
	}
}

func GetPVCName(targetKind, targetName string) string {
	switch targetKind {
	case "GameServer":
		return "data-" + targetName + "-0"
	default:
		return ""
	}
}

func fmtInt32(v int32) string {
	return fmt.Sprintf("%d", v)
}

func fmtBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
