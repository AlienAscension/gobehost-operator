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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func makeGameServerForTest(name string) *gamesv1alpha1.GameServer {
	return &gamesv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
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
	}
}

func cleanupGameServer(ctx context.Context, name string) {
	nn := types.NamespacedName{Name: name, Namespace: "default"}

	gs := &gamesv1alpha1.GameServer{}
	if err := k8sClient.Get(ctx, nn, gs); err == nil {
		gs.Finalizers = nil
		_ = k8sClient.Update(ctx, gs)
		_ = k8sClient.Delete(ctx, gs)
	}

	_ = k8sClient.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}})
	_ = k8sClient.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}})
	_ = k8sClient.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name + "-headless", Namespace: "default"}})
	_ = k8sClient.Delete(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name + "-data", Namespace: "default"}})
}

var _ = Describe("GameServer Controller", func() {
	var reconciler *GameServerReconciler

	BeforeEach(func() {
		reconciler = &GameServerReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	Context("When reconciling a new GameServer", func() {
		It("should add the finalizer on first reconciliation", func() {
			const name = "test-add-finalizer"
			defer cleanupGameServer(ctx, name)

			gs := makeGameServerForTest(name)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, gs)).To(Succeed())
			Expect(gs.Finalizers).To(ContainElement(gamesv1alpha1.GameServerFinalizer))
		})

		It("should create Services and StatefulSet", func() {
			const name = "test-child-resources"
			defer cleanupGameServer(ctx, name)

			gs := makeGameServerForTest(name)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			headlessSvc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-headless", Namespace: "default"}, headlessSvc)).To(Succeed())
			Expect(headlessSvc.Spec.ClusterIP).To(Equal("None"))
			Expect(headlessSvc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(headlessSvc.Spec.Ports).To(HaveLen(1))
			Expect(headlessSvc.Spec.Ports[0].Name).To(Equal("minecraft"))
			Expect(headlessSvc.Spec.Ports[0].Port).To(Equal(int32(25565)))

			extSvc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, extSvc)).To(Succeed())
			Expect(extSvc.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
			Expect(extSvc.Spec.Ports).To(HaveLen(1))
			Expect(extSvc.Spec.Ports[0].Name).To(Equal("minecraft"))
			Expect(extSvc.Spec.Ports[0].Port).To(Equal(int32(25565)))

			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, sts)).To(Succeed())
			Expect(*sts.Spec.Replicas).To(Equal(int32(1)))
			Expect(sts.Spec.ServiceName).To(Equal(name + "-headless"))
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("itzg/minecraft-server:latest"))
			Expect(sts.Spec.Template.Spec.Containers[0].Name).To(Equal("server"))
		})

		It("should set phase to Provisioning and Ready condition to False", func() {
			const name = "test-status-provisioning"
			defer cleanupGameServer(ctx, name)

			gs := makeGameServerForTest(name)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, gs)).To(Succeed())
			Expect(gs.Status.Phase).To(Equal(gamesv1alpha1.GameServerPhaseProvisioning))
			Expect(gs.Status.Ready).To(BeFalse())

			var readyCond *metav1.Condition
			for i := range gs.Status.Conditions {
				if gs.Status.Conditions[i].Type == "Ready" {
					readyCond = &gs.Status.Conditions[i]
					break
				}
			}
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCond.Reason).To(Equal("Provisioning"))
		})

		It("should set owner references on child resources", func() {
			const name = "test-owner-refs"
			defer cleanupGameServer(ctx, name)

			gs := makeGameServerForTest(name)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, gs)).To(Succeed())

			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, sts)).To(Succeed())
			Expect(sts.OwnerReferences).To(HaveLen(1))
			Expect(sts.OwnerReferences[0].Name).To(Equal(name))
			Expect(*sts.OwnerReferences[0].Controller).To(BeTrue())

			headlessSvc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-headless", Namespace: "default"}, headlessSvc)).To(Succeed())
			Expect(headlessSvc.OwnerReferences).To(HaveLen(1))
			Expect(*headlessSvc.OwnerReferences[0].Controller).To(BeTrue())

			extSvc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, extSvc)).To(Succeed())
			Expect(extSvc.OwnerReferences).To(HaveLen(1))
			Expect(*extSvc.OwnerReferences[0].Controller).To(BeTrue())
		})

		It("should set observed generation on status", func() {
			const name = "test-observed-gen"
			defer cleanupGameServer(ctx, name)

			gs := makeGameServerForTest(name)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, gs)).To(Succeed())
			Expect(gs.Status.ObservedGeneration).To(Equal(gs.Generation))
		})
	})

	Context("When deleting a GameServer", func() {
		It("should finalize and remove finalizer allowing deletion", func() {
			const name = "test-finalize"
			defer cleanupGameServer(ctx, name)

			gs := makeGameServerForTest(name)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, gs)).To(Succeed())
			Expect(gs.Finalizers).To(ContainElement(gamesv1alpha1.GameServerFinalizer))

			Expect(k8sClient.Delete(ctx, gs)).To(Succeed())

			Expect(k8sClient.Get(ctx, nn, gs)).To(Succeed())
			Expect(gs.DeletionTimestamp).NotTo(BeNil())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, nn, &gamesv1alpha1.GameServer{})
				return errors.IsNotFound(err)
			}).Should(BeTrue())
		})

		It("should scale down StatefulSet during finalization", func() {
			const name = "test-sts-scaledown"
			defer cleanupGameServer(ctx, name)

			gs := makeGameServerForTest(name)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, sts)).To(Succeed())
			Expect(*sts.Spec.Replicas).To(Equal(int32(1)))

			Expect(k8sClient.Get(ctx, nn, gs)).To(Succeed())
			Expect(k8sClient.Delete(ctx, gs)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, sts)).To(Succeed())
			Expect(*sts.Spec.Replicas).To(Equal(int32(0)))
		})
	})

	Context("When reconciling an unknown game type", func() {
		It("should set Failed phase and AdapterNotFound condition", func() {
			const name = "test-unknown-type"
			defer cleanupGameServer(ctx, name)

			gs := makeGameServerForTest(name)
			gs.Spec.Game.Type = "unknown-game"
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, nn, gs)).To(Succeed())
			Expect(gs.Status.Phase).To(Equal(gamesv1alpha1.GameServerPhaseFailed))

			var readyCond *metav1.Condition
			for i := range gs.Status.Conditions {
				if gs.Status.Conditions[i].Type == "Ready" {
					readyCond = &gs.Status.Conditions[i]
					break
				}
			}
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCond.Reason).To(Equal("AdapterNotFound"))
			Expect(readyCond.Message).To(ContainSubstring("unknown-game"))
		})

		It("should not create child resources for unknown game type", func() {
			const name = "test-no-resources-unknown"
			defer cleanupGameServer(ctx, name)

			gs := makeGameServerForTest(name)
			gs.Spec.Game.Type = "nonexistent"
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			nn := types.NamespacedName{Name: name, Namespace: "default"}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			Expect(errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, &appsv1.StatefulSet{}))).To(BeTrue())
			Expect(errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, &corev1.Service{}))).To(BeTrue())
			Expect(errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-headless", Namespace: "default"}, &corev1.Service{}))).To(BeTrue())
			Expect(errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-data", Namespace: "default"}, &corev1.PersistentVolumeClaim{}))).To(BeTrue())
		})
	})

	Context("When reconciling a nonexistent GameServer", func() {
		It("should return without error", func() {
			nn := types.NamespacedName{Name: "does-not-exist", Namespace: "default"}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
