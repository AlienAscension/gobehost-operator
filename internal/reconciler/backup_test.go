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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func makeTestBackup() *gamesv1alpha1.GameServerBackup {
	return &gamesv1alpha1.GameServerBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-backup",
			Namespace: "test-ns",
		},
		Spec: gamesv1alpha1.GameServerBackupSpec{
			TargetRef: gamesv1alpha1.TargetReference{
				Kind: "GameServer",
				Name: "my-server",
			},
			Schedule:        "0 */6 * * *",
			Retention:       ptr.To(int32(5)),
			IncludeMetadata: ptr.To(true),
			BackupOnDelete:  ptr.To(true),
		},
	}
}

func makePlatformConfig() *BackupConfig {
	return &BackupConfig{
		Endpoint:        "https://s3.example.com",
		Bucket:          "platform-bucket",
		Path:            "platform/path",
		SecretName:      "platform-secret",
		SecretNamespace: "platform-ns",
	}
}

var _ = Describe("ResolveStorageConfig", func() {
	It("should use platform defaults when spec storage is nil", func() {
		backup := makeTestBackup()
		backup.Spec.Storage = nil
		cfg := ResolveStorageConfig(backup, makePlatformConfig())
		Expect(cfg.Endpoint).To(Equal("https://s3.example.com"))
		Expect(cfg.Bucket).To(Equal("platform-bucket"))
		Expect(cfg.Path).To(Equal("platform/path"))
		Expect(cfg.SecretName).To(Equal("platform-secret"))
	})

	It("should override platform defaults with spec values", func() {
		backup := makeTestBackup()
		backup.Spec.Storage = &gamesv1alpha1.BackupStorageSpec{
			Endpoint: "https://custom-s3.example.com",
			Bucket:   "custom-bucket",
			Path:     "custom/path",
			SecretRef: &gamesv1alpha1.BackupSecretRef{
				Name: "custom-secret",
			},
		}
		cfg := ResolveStorageConfig(backup, makePlatformConfig())
		Expect(cfg.Endpoint).To(Equal("https://custom-s3.example.com"))
		Expect(cfg.Bucket).To(Equal("custom-bucket"))
		Expect(cfg.Path).To(Equal("custom/path"))
		Expect(cfg.SecretName).To(Equal("custom-secret"))
	})

	It("should use platform defaults for fields not overridden", func() {
		backup := makeTestBackup()
		backup.Spec.Storage = &gamesv1alpha1.BackupStorageSpec{
			Bucket: "custom-bucket",
		}
		cfg := ResolveStorageConfig(backup, makePlatformConfig())
		Expect(cfg.Endpoint).To(Equal("https://s3.example.com"))
		Expect(cfg.Bucket).To(Equal("custom-bucket"))
		Expect(cfg.Path).To(Equal("platform/path"))
		Expect(cfg.SecretName).To(Equal("platform-secret"))
	})

	It("should compute default path from namespace/target-name when path is empty", func() {
		backup := makeTestBackup()
		backup.Spec.Storage = nil
		platformCfg := &BackupConfig{
			Endpoint:        "https://s3.example.com",
			Bucket:          "platform-bucket",
			Path:            "",
			SecretName:      "platform-secret",
			SecretNamespace: "platform-ns",
		}
		cfg := ResolveStorageConfig(backup, platformCfg)
		Expect(cfg.Path).To(Equal("test-ns/my-server"))
	})

	It("should always set SecretNamespace to backup namespace", func() {
		backup := makeTestBackup()
		cfg := ResolveStorageConfig(backup, makePlatformConfig())
		Expect(cfg.SecretNamespace).To(Equal("test-ns"))
	})

	It("should handle empty storage spec with overrides", func() {
		backup := makeTestBackup()
		backup.Spec.Storage = &gamesv1alpha1.BackupStorageSpec{}
		cfg := ResolveStorageConfig(backup, makePlatformConfig())
		Expect(cfg.Endpoint).To(Equal("https://s3.example.com"))
		Expect(cfg.Bucket).To(Equal("platform-bucket"))
		Expect(cfg.SecretName).To(Equal("platform-secret"))
	})
})

var _ = Describe("BuildCronJob", func() {
	var (
		backup *gamesv1alpha1.GameServerBackup
		cfg    *BackupConfig
	)

	BeforeEach(func() {
		backup = makeTestBackup()
		cfg = &BackupConfig{
			Endpoint:        "https://s3.example.com",
			Bucket:          "my-bucket",
			Path:            "ns/server",
			SecretName:      "s3-creds",
			SecretNamespace: "test-ns",
		}
	})

	It("should set correct name and namespace", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		Expect(cj.Name).To(Equal("my-backup-backup"))
		Expect(cj.Namespace).To(Equal("test-ns"))
	})

	It("should set schedule from spec", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		Expect(cj.Spec.Schedule).To(Equal("0 */6 * * *"))
	})

	It("should set history limits", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		Expect(*cj.Spec.SuccessfulJobsHistoryLimit).To(Equal(int32(3)))
		Expect(*cj.Spec.FailedJobsHistoryLimit).To(Equal(int32(3)))
	})

	It("should set labels", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		Expect(cj.Labels["games.gobehost.com/backup"]).To(Equal("my-backup"))
		Expect(cj.Labels["app.kubernetes.io/managed-by"]).To(Equal("gobehost-operator"))
	})

	It("should configure container with rclone image", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(container.Name).To(Equal("backup"))
		Expect(container.Image).To(Equal("rclone/rclone:latest"))
	})

	It("should set S3 env vars from config and secret refs", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(envValue(container.Env, "RCLONE_CONFIG_S3_BACKUP_TYPE")).To(Equal("s3"))
		Expect(envValue(container.Env, "RCLONE_CONFIG_S3_BACKUP_PROVIDER")).To(Equal("Other"))
		Expect(envValue(container.Env, "RCLONE_CONFIG_S3_BACKUP_ENDPOINT")).To(Equal("https://s3.example.com"))
		Expect(envValue(container.Env, "BACKUP_PATH")).To(Equal("ns/server"))
		Expect(envValue(container.Env, "BACKUP_BUCKET")).To(Equal("my-bucket"))
		Expect(envValue(container.Env, "BACKUP_RETENTION")).To(Equal("5"))
		Expect(envValue(container.Env, "INCLUDE_METADATA")).To(Equal("true"))
	})

	It("should set secret key refs for S3 credentials", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		secretRef := findEnvVar(container.Env, "RCLONE_CONFIG_S3_BACKUP_ACCESS_KEY_ID")
		Expect(secretRef).NotTo(BeNil())
		Expect(secretRef.ValueFrom.SecretKeyRef.Name).To(Equal("s3-creds"))
		Expect(secretRef.ValueFrom.SecretKeyRef.Key).To(Equal("S3_ACCESS_KEY"))
	})

	It("should mount PVC read-only at /data", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(container.VolumeMounts).To(HaveLen(1))
		Expect(container.VolumeMounts[0].Name).To(Equal("data"))
		Expect(container.VolumeMounts[0].MountPath).To(Equal("/data"))
		Expect(container.VolumeMounts[0].ReadOnly).To(BeTrue())
	})

	It("should set volume to reference the PVC", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		volumes := cj.Spec.JobTemplate.Spec.Template.Spec.Volumes
		Expect(volumes).To(HaveLen(1))
		Expect(volumes[0].Name).To(Equal("data"))
		Expect(volumes[0].PersistentVolumeClaim).NotTo(BeNil())
		Expect(volumes[0].PersistentVolumeClaim.ClaimName).To(Equal("data-my-server-0"))
		Expect(volumes[0].PersistentVolumeClaim.ReadOnly).To(BeTrue())
	})

	It("should use custom retention from spec", func() {
		backup.Spec.Retention = ptr.To(int32(10))
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(envValue(container.Env, "BACKUP_RETENTION")).To(Equal("10"))
	})

	It("should default includeMetadata to true when nil", func() {
		backup.Spec.IncludeMetadata = nil
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(envValue(container.Env, "INCLUDE_METADATA")).To(Equal("true"))
	})

	It("should set includeMetadata to false when spec says false", func() {
		backup.Spec.IncludeMetadata = ptr.To(false)
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(envValue(container.Env, "INCLUDE_METADATA")).To(Equal("false"))
	})

	It("should set restart policy to OnFailure", func() {
		cj := BuildCronJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		Expect(cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyOnFailure))
	})
})

var _ = Describe("BuildBackupOnDeleteJob", func() {
	var (
		backup *gamesv1alpha1.GameServerBackup
		cfg    *BackupConfig
	)

	BeforeEach(func() {
		backup = makeTestBackup()
		cfg = &BackupConfig{
			Endpoint:        "https://s3.example.com",
			Bucket:          "my-bucket",
			Path:            "ns/server",
			SecretName:      "s3-creds",
			SecretNamespace: "test-ns",
		}
	})

	It("should set correct name and namespace", func() {
		job := BuildBackupOnDeleteJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		Expect(job.Name).To(Equal("my-backup-backup-on-delete"))
		Expect(job.Namespace).To(Equal("test-ns"))
	})

	It("should set labels", func() {
		job := BuildBackupOnDeleteJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		Expect(job.Labels["games.gobehost.com/backup"]).To(Equal("my-backup"))
		Expect(job.Labels["app.kubernetes.io/managed-by"]).To(Equal("gobehost-operator"))
	})

	It("should configure container with same spec as CronJob", func() {
		job := BuildBackupOnDeleteJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		container := job.Spec.Template.Spec.Containers[0]
		Expect(container.Name).To(Equal("backup"))
		Expect(container.Image).To(Equal("rclone/rclone:latest"))
		Expect(envValue(container.Env, "RCLONE_CONFIG_S3_BACKUP_ENDPOINT")).To(Equal("https://s3.example.com"))
		Expect(envValue(container.Env, "BACKUP_PATH")).To(Equal("ns/server"))
		Expect(envValue(container.Env, "BACKUP_BUCKET")).To(Equal("my-bucket"))
	})

	It("should mount PVC read-only at /data", func() {
		job := BuildBackupOnDeleteJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		container := job.Spec.Template.Spec.Containers[0]
		Expect(container.VolumeMounts).To(HaveLen(1))
		Expect(container.VolumeMounts[0].Name).To(Equal("data"))
		Expect(container.VolumeMounts[0].MountPath).To(Equal("/data"))
		Expect(container.VolumeMounts[0].ReadOnly).To(BeTrue())
	})

	It("should set volume to reference the PVC", func() {
		job := BuildBackupOnDeleteJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		volumes := job.Spec.Template.Spec.Volumes
		Expect(volumes).To(HaveLen(1))
		Expect(volumes[0].PersistentVolumeClaim.ClaimName).To(Equal("data-my-server-0"))
	})

	It("should set restart policy to OnFailure", func() {
		job := BuildBackupOnDeleteJob(backup, cfg, "GameServer", "my-server", "data-my-server-0")
		Expect(job.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyOnFailure))
	})
})

var _ = Describe("GameServerBackupLabels", func() {
	It("should return correct labels", func() {
		backup := makeTestBackup()
		labels := GameServerBackupLabels(backup)
		Expect(labels["app.kubernetes.io/name"]).To(Equal("gameserverbackup"))
		Expect(labels["app.kubernetes.io/instance"]).To(Equal("my-backup"))
		Expect(labels["app.kubernetes.io/managed-by"]).To(Equal("gobehost-operator"))
		Expect(labels["games.gobehost.com/backup"]).To(Equal("my-backup"))
	})
})

var _ = Describe("GetPVCName", func() {
	It("should return correct PVC name for GameServer", func() {
		Expect(GetPVCName("GameServer", "my-server")).To(Equal("data-my-server-0"))
	})

	It("should return empty string for GameServerFleet", func() {
		Expect(GetPVCName("GameServerFleet", "my-fleet")).To(Equal(""))
	})

	It("should return empty string for unknown kind", func() {
		Expect(GetPVCName("Unknown", "something")).To(Equal(""))
	})
})

func findEnvVar(env []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range env {
		if env[i].Name == name {
			return &env[i]
		}
	}
	return nil
}
