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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func SetCondition(gs *gamesv1alpha1.GameServer, condType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	cond := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: gs.Status.ObservedGeneration,
		LastTransitionTime: now,
	}

	for i, existing := range gs.Status.Conditions {
		if existing.Type == condType {
			gs.Status.Conditions[i] = cond
			return
		}
	}

	gs.Status.Conditions = append(gs.Status.Conditions, cond)
}

func SetReady(gs *gamesv1alpha1.GameServer, ready bool, reason, message string) {
	status := metav1.ConditionFalse
	if ready {
		status = metav1.ConditionTrue
	}
	SetCondition(gs, "Ready", status, reason, message)
	gs.Status.Ready = ready
}

func SetPhase(gs *gamesv1alpha1.GameServer, phase gamesv1alpha1.GameServerPhase) {
	gs.Status.Phase = phase
}

func UpdateAddress(gs *gamesv1alpha1.GameServer, svc *corev1.Service) {
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		gs.Status.Address = ""
		gs.Status.Endpoint = ""
		gs.Status.Ports = nil
		return
	}

	ing := svc.Status.LoadBalancer.Ingress[0]
	address := ""
	if ing.IP != "" {
		address = ing.IP
	} else if ing.Hostname != "" {
		address = ing.Hostname
	}

	gs.Status.Address = address

	if address != "" && len(gs.Spec.Network.Ports) > 0 {
		gs.Status.Endpoint = fmt.Sprintf("%s:%d", address, gs.Spec.Network.Ports[0].Port)
	} else {
		gs.Status.Endpoint = ""
	}

	ports := make([]gamesv1alpha1.PortInfo, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		ports = append(ports, gamesv1alpha1.PortInfo{
			Name:     p.Name,
			Port:     p.Port,
			Protocol: p.Protocol,
		})
	}
	gs.Status.Ports = ports
}
