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

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

var _ = Describe("Status helpers", func() {
	Describe("SetCondition", func() {
		It("should add a condition", func() {
			gs := &gamesv1alpha1.GameServer{}
			gs.Status.ObservedGeneration = 3
			SetCondition(gs, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
			Expect(gs.Status.Conditions).To(HaveLen(1))
			Expect(gs.Status.Conditions[0].Type).To(Equal("Ready"))
			Expect(gs.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(gs.Status.Conditions[0].Reason).To(Equal("TestReason"))
			Expect(gs.Status.Conditions[0].Message).To(Equal("Test message"))
			Expect(gs.Status.Conditions[0].ObservedGeneration).To(Equal(int64(3)))
		})

		It("should update an existing condition", func() {
			gs := &gamesv1alpha1.GameServer{}
			gs.Status.ObservedGeneration = 1
			SetCondition(gs, "Ready", metav1.ConditionFalse, "Initial", "Not ready")
			Expect(gs.Status.Conditions).To(HaveLen(1))

			gs.Status.ObservedGeneration = 2
			SetCondition(gs, "Ready", metav1.ConditionTrue, "Available", "Now ready")
			Expect(gs.Status.Conditions).To(HaveLen(1))
			Expect(gs.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(gs.Status.Conditions[0].Reason).To(Equal("Available"))
			Expect(gs.Status.Conditions[0].ObservedGeneration).To(Equal(int64(2)))
		})
	})

	Describe("SetReady", func() {
		It("should set Ready condition to true", func() {
			gs := &gamesv1alpha1.GameServer{}
			SetReady(gs, true, "Available", "Server is ready")
			Expect(gs.Status.Ready).To(BeTrue())
			Expect(gs.Status.Conditions).To(HaveLen(1))
			Expect(gs.Status.Conditions[0].Type).To(Equal("Ready"))
			Expect(gs.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
		})

		It("should set Ready condition to false", func() {
			gs := &gamesv1alpha1.GameServer{}
			SetReady(gs, false, "Unavailable", "Server not ready")
			Expect(gs.Status.Ready).To(BeFalse())
			Expect(gs.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
		})
	})

	Describe("SetPhase", func() {
		It("should set the phase", func() {
			gs := &gamesv1alpha1.GameServer{}
			SetPhase(gs, gamesv1alpha1.GameServerPhaseProvisioning)
			Expect(gs.Status.Phase).To(Equal(gamesv1alpha1.GameServerPhaseProvisioning))
		})
	})

	Describe("UpdateAddress", func() {
		It("should extract LoadBalancer IP", func() {
			gs := &gamesv1alpha1.GameServer{
				Spec: gamesv1alpha1.GameServerSpec{
					Network: gamesv1alpha1.NetworkSpec{
						Ports: []gamesv1alpha1.PortSpec{
							{Name: "minecraft", Port: 25565, Protocol: corev1.ProtocolTCP},
						},
					},
				},
			}
			svc := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "minecraft", Port: 25565, Protocol: corev1.ProtocolTCP},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{IP: "203.0.113.10"},
						},
					},
				},
			}
			UpdateAddress(gs, svc)
			Expect(gs.Status.Address).To(Equal("203.0.113.10"))
			Expect(gs.Status.Endpoint).To(Equal("203.0.113.10:25565"))
			Expect(gs.Status.Ports).To(HaveLen(1))
			Expect(gs.Status.Ports[0].Name).To(Equal("minecraft"))
			Expect(gs.Status.Ports[0].Port).To(Equal(int32(25565)))
		})

		It("should fall back to hostname if no IP", func() {
			gs := &gamesv1alpha1.GameServer{
				Spec: gamesv1alpha1.GameServerSpec{
					Network: gamesv1alpha1.NetworkSpec{
						Ports: []gamesv1alpha1.PortSpec{
							{Name: "minecraft", Port: 25565, Protocol: corev1.ProtocolTCP},
						},
					},
				},
			}
			svc := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "minecraft", Port: 25565, Protocol: corev1.ProtocolTCP},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{Hostname: "my-game.example.com"},
						},
					},
				},
			}
			UpdateAddress(gs, svc)
			Expect(gs.Status.Address).To(Equal("my-game.example.com"))
			Expect(gs.Status.Endpoint).To(Equal("my-game.example.com:25565"))
		})

		It("should clear address when no ingress", func() {
			gs := &gamesv1alpha1.GameServer{}
			gs.Status.Address = "old-address"
			svc := &corev1.Service{}
			UpdateAddress(gs, svc)
			Expect(gs.Status.Address).To(BeEmpty())
			Expect(gs.Status.Endpoint).To(BeEmpty())
			Expect(gs.Status.Ports).To(BeNil())
		})
	})
})
