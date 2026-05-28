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

package adapter

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func init() {
	Register(&MinecraftAdapter{})
}

var _ GameAdapter = (*MinecraftAdapter)(nil)

type MinecraftAdapter struct{}

func (m *MinecraftAdapter) Name() string {
	return "minecraft"
}

func (m *MinecraftAdapter) Env(gs *gamesv1alpha1.GameServer) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "EULA", Value: "TRUE"},
		{Name: "VERSION", Value: gs.Spec.Game.Version},
	}

	profileMap := map[string]string{
		"paper":  "PAPER",
		"forge":  "FORGE",
		"fabric": "FABRIC",
		"spigot": "SPIGOT",
		"bukkit": "BUKKIT",
	}

	if gs.Spec.Game.Profile != "" {
		if typ, ok := profileMap[gs.Spec.Game.Profile]; ok {
			env = append(env, corev1.EnvVar{Name: "TYPE", Value: typ})
		}
	}

	if gs.Spec.Game.Profile == "" {
		env = append(env, corev1.EnvVar{Name: "TYPE", Value: "VANILLA"})
	}

	if gs.Spec.Server != nil {
		srv := gs.Spec.Server
		if srv.MaxPlayers != nil {
			env = append(env, corev1.EnvVar{Name: "MAX_PLAYERS", Value: strconv.Itoa(int(ptr.Deref(srv.MaxPlayers, 0)))})
		}
		if srv.Motd != "" {
			env = append(env, corev1.EnvVar{Name: "MOTD", Value: srv.Motd})
		}
		if srv.LevelName != "" {
			env = append(env, corev1.EnvVar{Name: "LEVEL", Value: srv.LevelName})
		}
		if srv.Difficulty != "" {
			env = append(env, corev1.EnvVar{Name: "DIFFICULTY", Value: srv.Difficulty})
		}
		if srv.GameMode != "" {
			env = append(env, corev1.EnvVar{Name: "MODE", Value: srv.GameMode})
		}
		if srv.Pvp != nil {
			env = append(env, corev1.EnvVar{Name: "PVP", Value: strconv.FormatBool(*srv.Pvp)})
		}
		if srv.OnlineMode != nil {
			env = append(env, corev1.EnvVar{Name: "ONLINE_MODE", Value: strconv.FormatBool(*srv.OnlineMode)})
		}
	}

	return env
}

func (m *MinecraftAdapter) Command(_ *gamesv1alpha1.GameServer) []string {
	return nil
}

func (m *MinecraftAdapter) Args(_ *gamesv1alpha1.GameServer) []string {
	return nil
}

func (m *MinecraftAdapter) Probes(_ *gamesv1alpha1.GameServer) (*corev1.Probe, *corev1.Probe) {
	readiness := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.IntOrString{Type: intstr.Int, IntVal: 25565},
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		FailureThreshold:    10,
	}

	liveness := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.IntOrString{Type: intstr.Int, IntVal: 25565},
			},
		},
		InitialDelaySeconds: 120,
		PeriodSeconds:       30,
		FailureThreshold:    5,
	}

	return readiness, liveness
}

func (m *MinecraftAdapter) DataPath(_ *gamesv1alpha1.GameServer) string {
	return "/data"
}

func (m *MinecraftAdapter) DefaultSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsUser:  ptr.To(int64(1000)),
		RunAsGroup: ptr.To(int64(1000)),
		FSGroup:    ptr.To(int64(1000)),
	}
}
