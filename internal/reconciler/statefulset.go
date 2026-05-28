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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
	"github.com/gobehost/operator/internal/adapter"
)

func BuildStatefulSet(gs *gamesv1alpha1.GameServer) (*appsv1.StatefulSet, error) {
	a, err := adapter.Get(gs)
	if err != nil {
		return nil, err
	}

	pvc := BuildPVC(gs)
	pvc.Name = "data"
	pvc.Namespace = ""

	env := a.Env(gs)
	env = append(env, gs.Spec.Runtime.Env...)

	command := gs.Spec.Runtime.Command
	if len(command) == 0 {
		command = a.Command(gs)
	}

	args := gs.Spec.Runtime.Args
	if len(args) == 0 {
		args = a.Args(gs)
	}

	ports := make([]corev1.ContainerPort, 0, len(gs.Spec.Network.Ports))
	for _, p := range gs.Spec.Network.Ports {
		ports = append(ports, corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		})
	}

	readiness, liveness := a.Probes(gs)

	container := corev1.Container{
		Name:            "server",
		Image:           gs.Spec.Runtime.Image,
		ImagePullPolicy: gs.Spec.Runtime.ImagePullPolicy,
		Env:             env,
		Command:         command,
		Args:            args,
		Ports:           ports,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: a.DataPath(gs),
			},
		},
		Resources:       gs.Spec.Resources,
		SecurityContext: buildSecurityContext(gs),
		EnvFrom:         gs.Spec.Runtime.EnvFrom,
	}

	if readiness != nil {
		container.ReadinessProbe = readiness
	}
	if liveness != nil {
		container.LivenessProbe = liveness
	}

	podSpec := corev1.PodSpec{
		TerminationGracePeriodSeconds: ptr.To(int64(120)),
		Containers:                    []corev1.Container{container},
		SecurityContext:               buildPodSecurityContext(gs, a),
	}

	applyScheduling(gs, &podSpec)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name,
			Namespace: gs.Namespace,
			Labels:    GameServerLabels(gs),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptr.To(int32(1)),
			Selector:    &metav1.LabelSelector{MatchLabels: GameServerLabels(gs)},
			ServiceName: gs.Name + "-headless",
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: GameServerLabels(gs),
				},
				Spec: podSpec,
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				*pvc,
			},
		},
	}

	return sts, nil
}

func buildSecurityContext(gs *gamesv1alpha1.GameServer) *corev1.SecurityContext {
	sec := gs.Spec.Security
	if sec == nil {
		sec = &gamesv1alpha1.SecuritySpec{}
	}

	sc := &corev1.SecurityContext{
		RunAsNonRoot: sec.RunAsNonRoot,
	}

	if sec.RunAsUser != nil {
		sc.RunAsUser = sec.RunAsUser
	}

	if sec.DropAllCapabilities == nil || *sec.DropAllCapabilities {
		sc.Capabilities = &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}
	}

	if sec.ReadOnlyRootFilesystem != nil {
		sc.ReadOnlyRootFilesystem = sec.ReadOnlyRootFilesystem
	}

	if sec.SeccompProfile != "" {
		sc.SeccompProfile = &corev1.SeccompProfile{
			Type: sec.SeccompProfile,
		}
	}

	return sc
}

func buildPodSecurityContext(gs *gamesv1alpha1.GameServer, a adapter.GameAdapter) *corev1.PodSecurityContext {
	defaultSC := a.DefaultSecurityContext()
	if defaultSC == nil {
		defaultSC = &corev1.PodSecurityContext{}
	}

	podSC := &corev1.PodSecurityContext{
		RunAsUser:  defaultSC.RunAsUser,
		RunAsGroup: defaultSC.RunAsGroup,
		FSGroup:    defaultSC.FSGroup,
	}

	if gs.Spec.Security != nil {
		if gs.Spec.Security.RunAsUser != nil {
			podSC.RunAsUser = gs.Spec.Security.RunAsUser
		}
		if gs.Spec.Security.RunAsGroup != nil {
			podSC.RunAsGroup = gs.Spec.Security.RunAsGroup
		}
		if gs.Spec.Security.FSGroup != nil {
			podSC.FSGroup = gs.Spec.Security.FSGroup
		}
	}

	return podSC
}

func applyScheduling(gs *gamesv1alpha1.GameServer, podSpec *corev1.PodSpec) {
	if gs.Spec.Scheduling == nil {
		return
	}

	podSpec.NodeSelector = gs.Spec.Scheduling.NodeSelector
	podSpec.Affinity = gs.Spec.Scheduling.Affinity
	podSpec.Tolerations = gs.Spec.Scheduling.Tolerations
}
