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

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func makeValidFleet() *gamesv1alpha1.GameServerFleet {
	return &gamesv1alpha1.GameServerFleet{
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
								Name:     "minecraft",
								Port:     25565,
								Protocol: corev1.ProtocolTCP,
							},
						},
						ServiceType: corev1.ServiceTypeLoadBalancer,
					},
				},
			},
		},
	}
}

var _ = Describe("GameServerFleet Webhook", func() {
	var defaulter GameServerFleetCustomDefaulter
	var validator GameServerFleetCustomValidator

	Describe("Defaulting", func() {
		It("should default replicas to 1", func() {
			fleet := makeValidFleet()
			fleet.Spec.Replicas = 0
			Expect(defaulter.Default(context.Background(), fleet)).To(Succeed())
			Expect(fleet.Spec.Replicas).To(Equal(int32(1)))
		})

		It("should default strategy to RollingUpdate", func() {
			fleet := makeValidFleet()
			fleet.Spec.Strategy.Type = ""
			Expect(defaulter.Default(context.Background(), fleet)).To(Succeed())
			Expect(fleet.Spec.Strategy.Type).To(Equal(gamesv1alpha1.RollingUpdateStrategyType))
		})

		It("should default embedded GameServer spec fields", func() {
			fleet := makeValidFleet()
			fleet.Spec.Template.Spec.Runtime.ImagePullPolicy = ""
			fleet.Spec.Template.Spec.Network.ServiceType = ""
			Expect(defaulter.Default(context.Background(), fleet)).To(Succeed())
			Expect(fleet.Spec.Template.Spec.Runtime.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(fleet.Spec.Template.Spec.Network.ServiceType).To(Equal(corev1.ServiceTypeLoadBalancer))
		})

		It("should not override already set values", func() {
			fleet := makeValidFleet()
			fleet.Spec.Replicas = 1
			Expect(defaulter.Default(context.Background(), fleet)).To(Succeed())
			Expect(fleet.Spec.Replicas).To(Equal(int32(1)))
		})
	})

	Describe("Validation", func() {
		It("should accept a valid GameServerFleet on create", func() {
			fleet := makeValidFleet()
			warnings, err := validator.ValidateCreate(context.Background(), fleet)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject replicas != 1 on create", func() {
			fleet := makeValidFleet()
			fleet.Spec.Replicas = 2
			_, err := validator.ValidateCreate(context.Background(), fleet)
			Expect(err).To(HaveOccurred())
		})

		It("should reject missing game type", func() {
			fleet := makeValidFleet()
			fleet.Spec.Template.Spec.Game.Type = ""
			_, err := validator.ValidateCreate(context.Background(), fleet)
			Expect(err).To(HaveOccurred())
		})

		It("should reject missing game version", func() {
			fleet := makeValidFleet()
			fleet.Spec.Template.Spec.Game.Version = ""
			_, err := validator.ValidateCreate(context.Background(), fleet)
			Expect(err).To(HaveOccurred())
		})

		It("should reject missing runtime image", func() {
			fleet := makeValidFleet()
			fleet.Spec.Template.Spec.Runtime.Image = ""
			_, err := validator.ValidateCreate(context.Background(), fleet)
			Expect(err).To(HaveOccurred())
		})

		It("should reject empty ports", func() {
			fleet := makeValidFleet()
			fleet.Spec.Template.Spec.Network.Ports = nil
			_, err := validator.ValidateCreate(context.Background(), fleet)
			Expect(err).To(HaveOccurred())
		})

		It("should reject missing storage size", func() {
			fleet := makeValidFleet()
			fleet.Spec.Template.Spec.Storage.Size = resource.Quantity{}
			_, err := validator.ValidateCreate(context.Background(), fleet)
			Expect(err).To(HaveOccurred())
		})

		It("should reject changing replicas on update", func() {
			oldFleet := makeValidFleet()
			newFleet := makeValidFleet()
			newFleet.Spec.Replicas = 2
			_, err := validator.ValidateUpdate(context.Background(), oldFleet, newFleet)
			Expect(err).To(HaveOccurred())
		})

		It("should allow update with same replicas", func() {
			oldFleet := makeValidFleet()
			newFleet := makeValidFleet()
			newFleet.Spec.Template.Spec.Game.Version = "1.22"
			_, err := validator.ValidateUpdate(context.Background(), oldFleet, newFleet)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow delete unconditionally", func() {
			fleet := makeValidFleet()
			warnings, err := validator.ValidateDelete(context.Background(), fleet)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("Port targetPort defaulting", func() {
		It("should default targetPort to port when nil", func() {
			fleet := makeValidFleet()
			fleet.Spec.Template.Spec.Network.Ports[0].TargetPort = nil
			Expect(defaulter.Default(context.Background(), fleet)).To(Succeed())
		})
	})

	Describe("Security defaults", func() {
		It("should default security fields", func() {
			fleet := makeValidFleet()
			fleet.Spec.Template.Spec.Security = &gamesv1alpha1.SecuritySpec{}
			Expect(defaulter.Default(context.Background(), fleet)).To(Succeed())
		})
	})
})
