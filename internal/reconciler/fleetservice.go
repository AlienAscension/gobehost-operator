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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func BuildFleetService(fleet *gamesv1alpha1.GameServerFleet, gs *gamesv1alpha1.GameServer) *corev1.Service {
	ports := make([]corev1.ServicePort, 0, len(gs.Spec.Network.Ports))
	for _, p := range gs.Spec.Network.Ports {
		sp := corev1.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: intstr.FromInt(int(p.Port)),
			Protocol:   p.Protocol,
		}
		if p.TargetPort != nil {
			sp.TargetPort = intstr.FromInt(int(*p.TargetPort))
		}
		ports = append(ports, sp)
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fleet.Name,
			Namespace:   fleet.Namespace,
			Labels:      FleetLabels(fleet),
			Annotations: gs.Spec.Network.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     gs.Spec.Network.ServiceType,
			Selector: GameServerLabels(gs),
			Ports:    ports,
		},
	}
}

func FleetLabels(fleet *gamesv1alpha1.GameServerFleet) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":        "gameserverfleet",
		"app.kubernetes.io/instance":    fleet.Name,
		"app.kubernetes.io/managed-by":  "gobehost-operator",
		"games.gobehost.com/fleet-name": fleet.Name,
	}
}
