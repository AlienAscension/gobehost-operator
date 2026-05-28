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
	corev1 "k8s.io/api/core/v1"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

type GameAdapter interface {
	Name() string
	Env(gs *gamesv1alpha1.GameServer) []corev1.EnvVar
	Command(gs *gamesv1alpha1.GameServer) []string
	Args(gs *gamesv1alpha1.GameServer) []string
	Probes(gs *gamesv1alpha1.GameServer) (*corev1.Probe, *corev1.Probe)
	DataPath(gs *gamesv1alpha1.GameServer) string
}
