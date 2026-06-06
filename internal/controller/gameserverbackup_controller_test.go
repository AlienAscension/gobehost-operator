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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

const testBackupConfigNS = "gobehost-system"

func makeGameServerBackupForTest(name, targetKind, targetName, schedule string) *gamesv1alpha1.GameServerBackup {
	return &gamesv1alpha1.GameServerBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: gamesv1alpha1.GameServerBackupSpec{
			TargetRef: gamesv1alpha1.TargetReference{
				Kind: targetKind,
				Name: targetName,
			},
			Schedule: schedule,
		},
	}
}

func makeGameServerWithPVCForTest(name string) (*gamesv1alpha1.GameServer, *corev1.PersistentVolumeClaim) {
	gs := makeGameServerForTest(name)
	pvcName := "data-" + name + "-0"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	return gs, pvc
}

func makeBackupConfigConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gobehost-backup-config",
			Namespace: testBackupConfigNS,
		},
		Data: map[string]string{
			"endpoint":   "https://s3.example.com",
			"bucket":     "backups",
			"path":       "game-backups",
			"secretName": "backup-creds",
		},
	}
}

func makeBackupSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Data: map[string][]byte{
			"S3_ACCESS_KEY": []byte("test-access-key"),
			"S3_SECRET_KEY": []byte("test-secret-key"),
		},
	}
}

func cleanupGameServerBackup(ctx context.Context, name string) {
	nn := types.NamespacedName{Name: name, Namespace: "default"}

	backup := &gamesv1alpha1.GameServerBackup{}
	if err := k8sClient.Get(ctx, nn, backup); err == nil {
		backup.Finalizers = nil
		_ = k8sClient.Update(ctx, backup)
		_ = k8sClient.Delete(ctx, backup)
	}

	_ = k8sClient.Delete(ctx, &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name + "-backup", Namespace: "default"}})
}

func ensureTestNamespace(ctx context.Context) {
	ns := &corev1.Namespace{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: testBackupConfigNS}, ns); err != nil {
		if errors.IsNotFound(err) {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: testBackupConfigNS},
			}
			_ = k8sClient.Create(ctx, ns)
		}
	}
}

var _ = Describe("GameServerBackup Controller", func() {
	var reconciler *GameServerBackupReconciler

	BeforeEach(func() {
		reconciler = &GameServerBackupReconciler{
			Client:                k8sClient,
			Scheme:                k8sClient.Scheme(),
			BackupConfigNamespace: testBackupConfigNS,
		}
		ensureTestNamespace(ctx)
	})

	Context("When reconciling a new GameServerBackup", func() {
		It("should add the finalizer on first reconciliation", func() {
			const name = "test-backup-finalizer"
			defer cleanupGameServerBackup(ctx, name)

			backup := makeGameServerBackupForTest(name, "GameServer", "test-gs", "0 */6 * * *")
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, backup)).To(Succeed())
			Expect(backup.Finalizers).To(ContainElement(gamesv1alpha1.GameServerBackupFinalizer))
		})

		It("should set TargetNotFound condition when target GameServer does not exist", func() {
			const name = "test-backup-target-not-found"
			defer cleanupGameServerBackup(ctx, name)

			backup := makeGameServerBackupForTest(name, "GameServer", "nonexistent-gs", "0 */6 * * *")
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, backup)).To(Succeed())
			var readyCond *metav1.Condition
			for i := range backup.Status.Conditions {
				if backup.Status.Conditions[i].Type == gamesv1alpha1.BackupReady {
					readyCond = &backup.Status.Conditions[i]
					break
				}
			}
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCond.Reason).To(Equal(gamesv1alpha1.BackupTargetNotFound))
		})

		It("should create a CronJob when target GameServer exists with a bound PVC", func() {
			const name = "test-backup-cronjob"
			const gsName = "test-backup-gs"
			defer cleanupGameServerBackup(ctx, name)
			defer cleanupGameServer(ctx, gsName)

			gs, pvc := makeGameServerWithPVCForTest(gsName)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			gsReconciler := &GameServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			gsNN := types.NamespacedName{Name: gsName, Namespace: "default"}
			_, err := gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())
			_, err = gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: "default"}, pvc)).To(Succeed())
			pvc.Status.Phase = corev1.ClaimBound
			Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())

			cm := makeBackupConfigConfigMap()
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, cm) }()

			secret := makeBackupSecret("backup-creds")
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, secret) }()

			backup := makeGameServerBackupForTest(name, "GameServer", gsName, "0 */6 * * *")
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			cronJob := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-backup", Namespace: "default"}, cronJob)).To(Succeed())
			Expect(cronJob.Spec.Schedule).To(Equal("0 */6 * * *"))
			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Name).To(Equal("backup"))

			Expect(k8sClient.Get(ctx, nn, backup)).To(Succeed())
			var readyCond *metav1.Condition
			for i := range backup.Status.Conditions {
				if backup.Status.Conditions[i].Type == gamesv1alpha1.BackupReady {
					readyCond = &backup.Status.Conditions[i]
					break
				}
			}
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCond.Reason).To(Equal(gamesv1alpha1.BackupCronJobCreated))
		})

		It("should set PVCNotReady condition when the PVC is not bound", func() {
			const name = "test-backup-pvc-not-bound"
			const gsName = "test-backup-pvc-gs"
			defer cleanupGameServerBackup(ctx, name)
			defer cleanupGameServer(ctx, gsName)

			gs, pvc := makeGameServerWithPVCForTest(gsName)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			gsReconciler := &GameServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			gsNN := types.NamespacedName{Name: gsName, Namespace: "default"}
			_, err := gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())
			_, err = gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())

			cm := makeBackupConfigConfigMap()
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, cm) }()

			backup := makeGameServerBackupForTest(name, "GameServer", gsName, "0 */6 * * *")
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, backup)).To(Succeed())
			var readyCond *metav1.Condition
			for i := range backup.Status.Conditions {
				if backup.Status.Conditions[i].Type == gamesv1alpha1.BackupReady {
					readyCond = &backup.Status.Conditions[i]
					break
				}
			}
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCond.Reason).To(Equal(gamesv1alpha1.BackupPVCNotReady))
		})

		It("should resolve storage config from platform ConfigMap", func() {
			const name = "test-backup-storage-config"
			const gsName = "test-backup-sc-gs"
			defer cleanupGameServerBackup(ctx, name)
			defer cleanupGameServer(ctx, gsName)

			gs, pvc := makeGameServerWithPVCForTest(gsName)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			gsReconciler := &GameServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			gsNN := types.NamespacedName{Name: gsName, Namespace: "default"}
			_, err := gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())
			_, err = gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: "default"}, pvc)).To(Succeed())
			pvc.Status.Phase = corev1.ClaimBound
			Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())

			cm := makeBackupConfigConfigMap()
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, cm) }()

			secret := makeBackupSecret("backup-creds")
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, secret) }()

			backup := makeGameServerBackupForTest(name, "GameServer", gsName, "0 */6 * * *")
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			cronJob := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-backup", Namespace: "default"}, cronJob)).To(Succeed())

			container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
			envVars := container.Env

			getEnvValue := func(name string) string {
				for _, env := range envVars {
					if env.Name == name {
						if env.Value != "" {
							return env.Value
						}
						if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
							return env.ValueFrom.SecretKeyRef.Name + "/" + env.ValueFrom.SecretKeyRef.Key
						}
					}
				}
				return ""
			}

			Expect(getEnvValue("RCLONE_S3_ENDPOINT")).To(Equal("https://s3.example.com"))
			Expect(getEnvValue("BACKUP_BUCKET")).To(Equal("backups"))
			Expect(getEnvValue("AWS_ACCESS_KEY_ID")).To(Equal("backup-creds/S3_ACCESS_KEY"))
			Expect(getEnvValue("BACKUP_RETENTION")).To(Equal("5"))
			Expect(getEnvValue("INCLUDE_METADATA")).To(Equal("true"))
		})

		It("should allow spec storage overrides to take precedence", func() {
			const name = "test-backup-override"
			const gsName = "test-backup-override-gs"
			defer cleanupGameServerBackup(ctx, name)
			defer cleanupGameServer(ctx, gsName)

			gs, pvc := makeGameServerWithPVCForTest(gsName)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			gsReconciler := &GameServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			gsNN := types.NamespacedName{Name: gsName, Namespace: "default"}
			_, err := gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())
			_, err = gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: "default"}, pvc)).To(Succeed())
			pvc.Status.Phase = corev1.ClaimBound
			Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())

			cm := makeBackupConfigConfigMap()
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, cm) }()

			secret := makeBackupSecret("backup-creds")
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, secret) }()

			customSecret := makeBackupSecret("custom-s3-creds")
			Expect(k8sClient.Create(ctx, customSecret)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, customSecret) }()

			backup := makeGameServerBackupForTest(name, "GameServer", gsName, "0 */3 * * *")
			backup.Spec.Storage = &gamesv1alpha1.BackupStorageSpec{
				Endpoint: "https://custom-s3.example.com",
				Bucket:   "custom-backups",
				Path:     "custom-path",
				SecretRef: &gamesv1alpha1.BackupSecretRef{
					Name: "custom-s3-creds",
				},
			}
			backup.Spec.Retention = ptr.To(int32(10))
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			cronJob := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-backup", Namespace: "default"}, cronJob)).To(Succeed())
			Expect(cronJob.Spec.Schedule).To(Equal("0 */3 * * *"))

			container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
			envVars := container.Env

			getEnvValue := func(envName string) string {
				for _, env := range envVars {
					if env.Name == envName {
						if env.Value != "" {
							return env.Value
						}
						if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
							return env.ValueFrom.SecretKeyRef.Name + "/" + env.ValueFrom.SecretKeyRef.Key
						}
					}
				}
				return ""
			}

			Expect(getEnvValue("RCLONE_S3_ENDPOINT")).To(Equal("https://custom-s3.example.com"))
			Expect(getEnvValue("BACKUP_BUCKET")).To(Equal("custom-backups"))
			Expect(getEnvValue("BACKUP_PATH")).To(Equal("custom-path"))
			Expect(getEnvValue("AWS_ACCESS_KEY_ID")).To(Equal("custom-s3-creds/S3_ACCESS_KEY"))
			Expect(getEnvValue("BACKUP_RETENTION")).To(Equal("10"))
		})

		It("should set StorageConfigMissing condition when no config is available", func() {
			const name = "test-backup-no-storage"
			const gsName = "test-backup-nostorage-gs"
			defer cleanupGameServerBackup(ctx, name)
			defer cleanupGameServer(ctx, gsName)

			gs := makeGameServerForTest(gsName)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			backup := makeGameServerBackupForTest(name, "GameServer", gsName, "0 */6 * * *")
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, backup)).To(Succeed())
			var readyCond *metav1.Condition
			for i := range backup.Status.Conditions {
				if backup.Status.Conditions[i].Type == gamesv1alpha1.BackupReady {
					readyCond = &backup.Status.Conditions[i]
					break
				}
			}
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCond.Reason).To(Equal(gamesv1alpha1.BackupStorageMissing))
		})

		It("should set owner reference on the CronJob", func() {
			const name = "test-backup-owner-ref"
			const gsName = "test-backup-owner-gs"
			defer cleanupGameServerBackup(ctx, name)
			defer cleanupGameServer(ctx, gsName)

			gs, pvc := makeGameServerWithPVCForTest(gsName)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			gsReconciler := &GameServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			gsNN := types.NamespacedName{Name: gsName, Namespace: "default"}
			_, err := gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())
			_, err = gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: "default"}, pvc)).To(Succeed())
			pvc.Status.Phase = corev1.ClaimBound
			Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())

			cm := makeBackupConfigConfigMap()
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, cm) }()

			secret := makeBackupSecret("backup-creds")
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, secret) }()

			backup := makeGameServerBackupForTest(name, "GameServer", gsName, "0 */6 * * *")
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			cronJob := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-backup", Namespace: "default"}, cronJob)).To(Succeed())
			Expect(cronJob.OwnerReferences).To(HaveLen(1))
			Expect(cronJob.OwnerReferences[0].Name).To(Equal(name))
			Expect(*cronJob.OwnerReferences[0].Controller).To(BeTrue())
		})

		It("should resolve GameServerFleet to current GameServer", func() {
			const name = "test-backup-fleet"
			const fleetName = "test-backup-fleet-resolve"
			defer cleanupGameServerBackup(ctx, name)

			fleet := makeFleetForTest(fleetName)
			Expect(k8sClient.Create(ctx, fleet)).To(Succeed())
			defer cleanupFleet(ctx, fleetName)

			fleetReconciler := &GameServerFleetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			fleetNN := types.NamespacedName{Name: fleetName, Namespace: "default"}
			_, err := fleetReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: fleetNN})
			Expect(err).NotTo(HaveOccurred())
			_, err = fleetReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: fleetNN})
			Expect(err).NotTo(HaveOccurred())

			gsName := fleetName + "-gs"
			gsNN := types.NamespacedName{Name: gsName, Namespace: "default"}
			gs := &gamesv1alpha1.GameServer{}
			Expect(k8sClient.Get(ctx, gsNN, gs)).To(Succeed())

			gsReconciler := &GameServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err = gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())
			_, err = gsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: gsNN})
			Expect(err).NotTo(HaveOccurred())

			pvcName := "data-" + gsName + "-0"
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pvcName,
					Namespace: "default",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("10Gi"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: "default"}, pvc)).To(Succeed())
			pvc.Status.Phase = corev1.ClaimBound
			Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())

			cm := makeBackupConfigConfigMap()
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, cm) }()

			secret := makeBackupSecret("backup-creds")
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, secret) }()

			backup := makeGameServerBackupForTest(name, "GameServerFleet", fleetName, "0 */6 * * *")
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			cronJob := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-backup", Namespace: "default"}, cronJob)).To(Succeed())

			Expect(k8sClient.Get(ctx, nn, backup)).To(Succeed())
			var readyCond *metav1.Condition
			for i := range backup.Status.Conditions {
				if backup.Status.Conditions[i].Type == gamesv1alpha1.BackupReady {
					readyCond = &backup.Status.Conditions[i]
					break
				}
			}
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("When reconciling a nonexistent GameServerBackup", func() {
		It("should return without error", func() {
			nn := types.NamespacedName{Name: "does-not-exist", Namespace: "default"}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
