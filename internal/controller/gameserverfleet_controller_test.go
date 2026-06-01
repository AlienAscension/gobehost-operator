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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func makeFleetForTest(name string) *gamesv1alpha1.GameServerFleet {
	return &gamesv1alpha1.GameServerFleet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: gamesv1alpha1.GameServerFleetSpec{
			Replicas: 1,
			Strategy: gamesv1alpha1.UpdateStrategy{
				Type: gamesv1alpha1.RollingUpdateStrategyType,
			},
			Template: gamesv1alpha1.GameServerTemplate{
				Spec: gamesv1alpha1.GameServerSpec{
					Game: gamesv1alpha1.GameSpec{
						Type:    "minecraft",
						Version: "1.21",
					},
					Runtime: gamesv1alpha1.RuntimeSpec{
						Image:           "itzg/minecraft-server:latest",
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
					Storage: gamesv1alpha1.StorageSpec{
						Size: resource.MustParse("10Gi"),
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
					Network: gamesv1alpha1.NetworkSpec{
						Ports: []gamesv1alpha1.PortSpec{
							{
								Name:       "minecraft",
								Port:       25565,
								TargetPort: ptr.To(int32(25565)),
								Protocol:   corev1.ProtocolTCP,
							},
						},
						ServiceType: corev1.ServiceTypeLoadBalancer,
					},
				},
			},
		},
	}
}

func cleanupFleet(ctx context.Context, name string) {
	nn := types.NamespacedName{Name: name, Namespace: "default"}

	fleet := &gamesv1alpha1.GameServerFleet{}
	if err := k8sClient.Get(ctx, nn, fleet); err == nil {
		fleet.Finalizers = nil
		_ = k8sClient.Update(ctx, fleet)
		_ = k8sClient.Delete(ctx, fleet)
	}

	gs := &gamesv1alpha1.GameServer{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-gs", Namespace: "default"}, gs); err == nil {
		gs.Finalizers = nil
		_ = k8sClient.Update(ctx, gs)
		_ = k8sClient.Delete(ctx, gs)
	}

	_ = k8sClient.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}})
}

var _ = Describe("GameServerFleet Controller", func() {
	var reconciler *GameServerFleetReconciler

	BeforeEach(func() {
		reconciler = &GameServerFleetReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	Context("When reconciling a new GameServerFleet", func() {
		It("should add the finalizer on first reconciliation", func() {
			const name = "test-fleet-finalizer"
			defer cleanupFleet(ctx, name)

			fleet := makeFleetForTest(name)
			Expect(k8sClient.Create(ctx, fleet)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, fleet)).To(Succeed())
			Expect(fleet.Finalizers).To(ContainElement(gamesv1alpha1.GameServerFleetFinalizer))
		})

		It("should create a GameServer on first reconcile", func() {
			const name = "test-fleet-create"
			defer cleanupFleet(ctx, name)

			fleet := makeFleetForTest(name)
			Expect(k8sClient.Create(ctx, fleet)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			gs := &gamesv1alpha1.GameServer{}
			gsName := name + "-gs"
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: "default"}, gs)).To(Succeed())
			Expect(gs.Spec.Game.Type).To(Equal("minecraft"))
			Expect(gs.Spec.Game.Version).To(Equal("1.21"))
			Expect(gs.Labels[gamesv1alpha1.FleetNameLabel]).To(Equal(name))
			Expect(gs.OwnerReferences).NotTo(BeEmpty())
			Expect(gs.OwnerReferences[0].Name).To(Equal(name))
		})

		It("should set status.currentGameServer after creating GameServer", func() {
			const name = "test-fleet-status"
			defer cleanupFleet(ctx, name)

			fleet := makeFleetForTest(name)
			Expect(k8sClient.Create(ctx, fleet)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, fleet)).To(Succeed())
			Expect(fleet.Status.CurrentGameServer).To(Equal(name + "-gs"))
			Expect(fleet.Status.Phase).To(Equal(gamesv1alpha1.FleetProgressing))
		})

		It("should create a stable Service for the fleet", func() {
			const name = "test-fleet-service"
			defer cleanupFleet(ctx, name)

			fleet := makeFleetForTest(name)
			Expect(k8sClient.Create(ctx, fleet)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			svc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, svc)).To(Succeed())
			Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
			Expect(svc.OwnerReferences).NotTo(BeEmpty())
		})

		It("should set Available status when GameServer is Ready", func() {
			const name = "test-fleet-available"
			defer cleanupFleet(ctx, name)

			fleet := makeFleetForTest(name)
			Expect(k8sClient.Create(ctx, fleet)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			gsName := name + "-gs"
			gs := &gamesv1alpha1.GameServer{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: "default"}, gs)).To(Succeed())

			gs.Status.Phase = gamesv1alpha1.GameServerPhaseRunning
			gs.Status.Ready = true
			Expect(k8sClient.Status().Update(ctx, gs)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, fleet)).To(Succeed())
			Expect(fleet.Status.Phase).To(Equal(gamesv1alpha1.FleetAvailable))
			Expect(fleet.Status.ReadyReplicas).To(Equal(int32(1)))
		})

		It("should set Degraded status when GameServer is not Ready", func() {
			const name = "test-fleet-degraded"
			defer cleanupFleet(ctx, name)

			fleet := makeFleetForTest(name)
			Expect(k8sClient.Create(ctx, fleet)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			gsName := name + "-gs"
			gs := &gamesv1alpha1.GameServer{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: "default"}, gs)).To(Succeed())

			gs.Status.Phase = gamesv1alpha1.GameServerPhaseProvisioning
			gs.Status.Ready = false
			Expect(k8sClient.Status().Update(ctx, gs)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, fleet)).To(Succeed())
			Expect(fleet.Status.Phase).To(Equal(gamesv1alpha1.FleetDegraded))
			Expect(fleet.Status.ReadyReplicas).To(Equal(int32(0)))
		})
	})

	Context("Rolling updates", func() {
		It("should detect template hash change and start rolling update", func() {
			const name = "test-fleet-rolling"
			defer cleanupFleet(ctx, name)

			fleet := makeFleetForTest(name)
			Expect(k8sClient.Create(ctx, fleet)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			gsName := name + "-gs"
			gs := &gamesv1alpha1.GameServer{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: "default"}, gs)).To(Succeed())

			gs.Status.Phase = gamesv1alpha1.GameServerPhaseRunning
			gs.Status.Ready = true
			Expect(k8sClient.Status().Update(ctx, gs)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, fleet)).To(Succeed())
			fleet.Spec.Template.Spec.Game.Version = "1.22"
			Expect(k8sClient.Update(ctx, fleet)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, fleet)).To(Succeed())
			Expect(fleet.Status.UpdatedGameServer).NotTo(BeEmpty())
			Expect(fleet.Annotations).To(HaveKeyWithValue(gamesv1alpha1.UpdatePhaseAnnotation, gamesv1alpha1.UpdatePhaseWaitingForReady))
		})

		It("should compute template hash correctly", func() {
			fleet1 := makeFleetForTest("test1")
			fleet2 := makeFleetForTest("test2")
			Expect(computeTemplateHash(fleet1.Spec.Template)).To(Equal(computeTemplateHash(fleet2.Spec.Template)))

			fleet2.Spec.Template.Spec.Game.Version = "different"
			Expect(computeTemplateHash(fleet1.Spec.Template)).NotTo(Equal(computeTemplateHash(fleet2.Spec.Template)))
		})
	})

	Context("When reconciling a nonexistent GameServerFleet", func() {
		It("should return without error", func() {
			nn := types.NamespacedName{Name: "does-not-exist", Namespace: "default"}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When deleting a GameServerFleet", func() {
		It("should remove owned GameServers finalizer and allow fleet deletion", func() {
			const name = "test-fleet-delete"
			defer cleanupFleet(ctx, name)

			fleet := makeFleetForTest(name)
			Expect(k8sClient.Create(ctx, fleet)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, fleet)).To(Succeed())
			Expect(fleet.Finalizers).To(ContainElement(gamesv1alpha1.GameServerFleetFinalizer))

			gsName := name + "-gs"
			gs := &gamesv1alpha1.GameServer{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: "default"}, gs)).To(Succeed())

			Expect(k8sClient.Delete(ctx, fleet)).To(Succeed())

			Expect(k8sClient.Get(ctx, nn, fleet)).To(Succeed())
			Expect(fleet.DeletionTimestamp).NotTo(BeNil())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			gsAfter := &gamesv1alpha1.GameServer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: "default"}, gsAfter)
				if errors.IsNotFound(err) {
					return true
				}
				if err != nil {
					return false
				}
				if gsAfter.DeletionTimestamp != nil {
					_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
				return false
			}, 10*time.Second, time.Second).Should(BeTrue())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, nn, &gamesv1alpha1.GameServerFleet{})
				return errors.IsNotFound(err)
			}, 5*time.Second, time.Second).Should(BeTrue())
		})
	})
})
