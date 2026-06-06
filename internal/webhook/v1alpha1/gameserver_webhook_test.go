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

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

var _ = Describe("GameServer Webhook", func() {
	var (
		obj       *gamesv1alpha1.GameServer
		oldObj    *gamesv1alpha1.GameServer
		validator GameServerCustomValidator
		defaulter GameServerCustomDefaulter
		ctx       = context.Background()
	)

	BeforeEach(func() {
		obj = validGameServer()
		oldObj = validGameServer()
		validator = GameServerCustomValidator{}
		defaulter = GameServerCustomDefaulter{}
	})

	Context("When creating GameServer under Defaulting Webhook", func() {
		It("Should default ImagePullPolicy to IfNotPresent", func() {
			obj.Spec.Runtime.ImagePullPolicy = ""
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.Runtime.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		})

		It("Should not override an already-set ImagePullPolicy", func() {
			obj.Spec.Runtime.ImagePullPolicy = corev1.PullAlways
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.Runtime.ImagePullPolicy).To(Equal(corev1.PullAlways))
		})

		It("Should default Storage AccessModes to ReadWriteOnce", func() {
			obj.Spec.Storage.AccessModes = nil
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.Storage.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))
		})

		It("Should not override existing AccessModes", func() {
			obj.Spec.Storage.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.Storage.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))
		})

		It("Should default ServiceType to LoadBalancer", func() {
			obj.Spec.Network.ServiceType = ""
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.Network.ServiceType).To(Equal(corev1.ServiceTypeLoadBalancer))
		})

		It("Should not override an already-set ServiceType", func() {
			obj.Spec.Network.ServiceType = corev1.ServiceTypeNodePort
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.Network.ServiceType).To(Equal(corev1.ServiceTypeNodePort))
		})

		It("Should default Port Protocol to TCP", func() {
			obj.Spec.Network.Ports[0].Protocol = ""
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.Network.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
		})

		It("Should default Port TargetPort to Port when nil", func() {
			obj.Spec.Network.Ports[0].TargetPort = nil
			port := obj.Spec.Network.Ports[0].Port
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.Network.Ports[0].TargetPort).To(Equal(ptr.To(port)))
		})

		It("Should not override an already-set TargetPort", func() {
			obj.Spec.Network.Ports[0].TargetPort = ptr.To(int32(25566))
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(*obj.Spec.Network.Ports[0].TargetPort).To(Equal(int32(25566)))
		})

		It("Should default Security when nil", func() {
			obj.Spec.Security = nil
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.Security).NotTo(BeNil())
			Expect(*obj.Spec.Security.RunAsNonRoot).To(BeTrue())
			Expect(obj.Spec.Security.SeccompProfile).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
			Expect(*obj.Spec.Security.DropAllCapabilities).To(BeTrue())
		})

		It("Should default Security fields when partially set", func() {
			obj.Spec.Security = &gamesv1alpha1.SecuritySpec{}
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(*obj.Spec.Security.RunAsNonRoot).To(BeTrue())
			Expect(obj.Spec.Security.SeccompProfile).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
			Expect(*obj.Spec.Security.DropAllCapabilities).To(BeTrue())
		})

	})

	Context("When creating or updating GameServer under Validating Webhook", func() {
		It("Should admit a valid GameServer", func() {
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(warnings).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny creation if game type is missing", func() {
			obj.Spec.Game.Type = ""
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation if game version is missing", func() {
			obj.Spec.Game.Version = ""
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation if runtime image is missing", func() {
			obj.Spec.Runtime.Image = ""
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation if network ports are empty", func() {
			obj.Spec.Network.Ports = nil
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation if storage size is zero", func() {
			obj.Spec.Storage.Size = resource.Quantity{}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation if port is out of range (0)", func() {
			obj.Spec.Network.Ports[0].Port = 0
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation if port is out of range (>65535)", func() {
			obj.Spec.Network.Ports[0].Port = 70000
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation if targetPort is out of range", func() {
			obj.Spec.Network.Ports[0].TargetPort = ptr.To(int32(0))
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit creation with valid port values", func() {
			obj.Spec.Network.Ports[0].Port = 25565
			obj.Spec.Network.Ports[0].TargetPort = ptr.To(int32(25565))
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When updating GameServer under Validating Webhook", func() {
		It("Should deny update if game type changes", func() {
			oldObj.Spec.Game.Type = "minecraft"
			obj.Spec.Game.Type = "valheim"
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should allow update if game version changes", func() {
			oldObj.Spec.Game.Version = "1.20"
			obj.Spec.Game.Version = "1.21"
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow update if game type stays the same", func() {
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When deleting GameServer under Validating Webhook", func() {
		It("Should always allow deletion", func() {
			warnings, err := validator.ValidateDelete(ctx, obj)
			Expect(warnings).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func validGameServer() *gamesv1alpha1.GameServer {
	return &gamesv1alpha1.GameServer{
		Spec: gamesv1alpha1.GameServerSpec{
			Game: gamesv1alpha1.GameSpec{
				Type:    "minecraft",
				Version: "1.21",
			},
			Runtime: gamesv1alpha1.RuntimeSpec{
				Image: "itzg/minecraft-server:latest",
			},
			Storage: gamesv1alpha1.StorageSpec{
				Size: resource.MustParse("10Gi"),
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
			},
		},
	}
}
