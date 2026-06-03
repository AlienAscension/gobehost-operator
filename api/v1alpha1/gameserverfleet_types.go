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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateStrategyType string

const (
	RollingUpdateStrategyType UpdateStrategyType = "RollingUpdate"
	RecreateStrategyType      UpdateStrategyType = "Recreate"
)

type UpdateStrategy struct {
	// +kubebuilder:validation:Enum=RollingUpdate;Recreate
	// +kubebuilder:default=RollingUpdate
	Type UpdateStrategyType `json:"type,omitempty"`
}

type GameServerTemplate struct {
	// Standard object metadata for the GameServer (labels, annotations).
	// Name is derived from the fleet name + suffix; do not set manually.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is a full GameServerSpec embedded verbatim.
	Spec GameServerSpec `json:"spec"`
}

// GameServerFleetSpec defines the desired state of GameServerFleet.
type GameServerFleetSpec struct {
	// Replicas is hardcoded to 1 for SaaS provisioning.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1
	Replicas int32 `json:"replicas,omitempty"`

	// Strategy controls how updates are applied.
	// +kubebuilder:default={type:"RollingUpdate"}
	Strategy UpdateStrategy `json:"strategy,omitempty"`

	// GracefulShutdown sends RCON countdown warnings to players before an update.
	// Disabled by default. Uses RCON_PASSWORD from the GameServer runtime env.
	// +optional
	GracefulShutdown *GracefulShutdownSpec `json:"gracefulShutdown,omitempty"`

	// Template is the GameServer spec to stamp out.
	Template GameServerTemplate `json:"template"`
}

// GracefulShutdownSpec configures player warnings before fleet updates.
type GracefulShutdownSpec struct {
	// Enabled enables RCON-based countdown warnings.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// CountdownSeconds is the total warning period in seconds.
	// Warnings are sent at each second. Minimum 3s.
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=3
	CountdownSeconds int32 `json:"countdownSeconds,omitempty"`

	// RCONPort is the Minecraft server's RCON port.
	// +kubebuilder:default=25575
	RCONPort int32 `json:"rconPort,omitempty"`
}

type FleetPhase string

const (
	FleetProgressing FleetPhase = "Progressing"
	FleetAvailable   FleetPhase = "Available"
	FleetDegraded    FleetPhase = "Degraded"
	FleetFailed      FleetPhase = "Failed"
)

type RolloutRecord struct {
	// StartedAt is when the rollout began.
	StartedAt metav1.Time `json:"startedAt"`

	// CompletedAt is when the rollout finished (success or failure).
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// FromVersion is the previous game version.
	// +optional
	FromVersion string `json:"fromVersion,omitempty"`

	// ToVersion is the target game version.
	// +optional
	ToVersion string `json:"toVersion,omitempty"`

	// Result is "Success" or "Failed".
	// +kubebuilder:validation:Enum=Success;Failed
	// +optional
	Result string `json:"result,omitempty"`

	// Message is a human-readable summary.
	// +optional
	Message string `json:"message,omitempty"`
}

// GameServerFleetStatus defines the observed state of GameServerFleet.
type GameServerFleetStatus struct {
	// Phase summarises the fleet's overall state.
	// +kubebuilder:validation:Enum=Progressing;Available;Degraded;Failed
	// +optional
	Phase FleetPhase `json:"phase,omitempty"`

	// ObservedGeneration is the last generation the controller reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ReadyReplicas is the count of GameServers in Ready=true state (0 or 1).
	ReadyReplicas int32 `json:"readyReplicas"`

	// Conditions holds detailed status conditions.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// CurrentGameServer is the name of the active GameServer owned by this fleet.
	// +optional
	CurrentGameServer string `json:"currentGameServer,omitempty"`

	// UpdatedGameServer is the name of the incoming GameServer during a rolling update.
	// Empty when no update is in progress.
	// +optional
	UpdatedGameServer string `json:"updatedGameServer,omitempty"`

	// History holds the last 10 completed rollouts.
	// +kubebuilder:validation:MaxItems=10
	// +optional
	History []RolloutRecord `json:"history,omitempty"`
}

const (
	FleetAvailableCondition   = "Available"
	FleetProgressingCondition = "Progressing"
	FleetDegradedCondition    = "Degraded"
)

const GameServerFleetFinalizer = "games.gobehost.com/fleet-finalizer"

const (
	UpdatePhaseAnnotation      = "games.gobehost.com/update-phase"
	UpdatePhaseWaitingForReady = "waiting-for-ready"
	UpdatePhaseDrainingOld     = "draining-old"
	TemplateHashAnnotation     = "games.gobehost.com/template-hash"
	FleetNameLabel             = "games.gobehost.com/fleet-name"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=gsf,categories=gobehost
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="GameServer",type=string,JSONPath=`.status.currentGameServer`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GameServerFleet is the Schema for the gameserverfleets API.
type GameServerFleet struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of GameServerFleet.
	// +required
	Spec GameServerFleetSpec `json:"spec"`

	// status defines the observed state of GameServerFleet.
	// +optional
	Status GameServerFleetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// GameServerFleetList contains a list of GameServerFleet.
type GameServerFleetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []GameServerFleet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameServerFleet{}, &GameServerFleetList{})
}
