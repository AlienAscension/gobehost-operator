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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GameServerPhase enumerates the phases of a GameServer.
type GameServerPhase string

const (
	GameServerPhasePending          GameServerPhase = "Pending"
	GameServerPhaseProvisioning     GameServerPhase = "Provisioning"
	GameServerPhaseRunning          GameServerPhase = "Running"
	GameServerPhaseStopping         GameServerPhase = "Stopping"
	GameServerPhaseStopped          GameServerPhase = "Stopped"
	GameServerPhaseFailed           GameServerPhase = "Failed"
	GameServerPhaseBackupInProgress GameServerPhase = "BackupInProgress"
)

const GameServerFinalizer = "games.gobehost.com/finalizer"

// GameSpec defines the game type and configuration.
type GameSpec struct {
	// type is the game type (e.g., "minecraft", "valheim", "terraria").
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// version is the game version (e.g., "1.21", "0.218.15").
	// +kubebuilder:validation:Required
	Version string `json:"version"`

	// profile is an optional game configuration profile name.
	// +optional
	Profile string `json:"profile,omitempty"`

	// mode is an optional game mode (e.g., "survival", "creative").
	// +optional
	Mode string `json:"mode,omitempty"`
}

// RuntimeSpec defines the container runtime configuration.
type RuntimeSpec struct {
	// image is the container image to run.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// imagePullPolicy is the image pull policy.
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// command is an optional container entrypoint override.
	// +optional
	Command []string `json:"command,omitempty"`

	// args are optional arguments to the entrypoint.
	// +optional
	Args []string `json:"args,omitempty"`

	// env are environment variables.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// envFrom lists sources for environment variables.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
}

// StorageSpec defines persistent volume storage.
type StorageSpec struct {
	// size is the requested storage size (e.g., "10Gi").
	// +kubebuilder:validation:Required
	Size resource.Quantity `json:"size"`

	// storageClass is the StorageClass for the PVC.
	// +optional
	StorageClass *string `json:"storageClass,omitempty"`

	// accessModes for the PVC.
	// +kubebuilder:default={ReadWriteOnce}
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// NetworkSpec defines network configuration.
type NetworkSpec struct {
	// ports are the ports to expose.
	// +kubebuilder:validation:MinItems=1
	Ports []PortSpec `json:"ports"`

	// serviceType is the Service type for exposure.
	// +kubebuilder:default=LoadBalancer
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// hostname is an optional DNS hostname for the server.
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// annotations are optional Service annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PortSpec defines a port to expose.
type PortSpec struct {
	// name is the port name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// port is the port number.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// targetPort is the target port on the container. Defaults to port if nil.
	// +optional
	TargetPort *int32 `json:"targetPort,omitempty"`

	// protocol is the network protocol.
	// +kubebuilder:default=TCP
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`
}

// ServerSpec defines game-specific server configuration.
type ServerSpec struct {
	// maxPlayers is the maximum number of players.
	// +optional
	MaxPlayers *int32 `json:"maxPlayers,omitempty"`

	// motd is the message of the day.
	// +optional
	Motd string `json:"motd,omitempty"`

	// levelName is the world/level name.
	// +optional
	LevelName string `json:"levelName,omitempty"`

	// whitelist is a list of allowed player names/UUIDs.
	// +optional
	Whitelist []string `json:"whitelist,omitempty"`

	// ops is a list of operator player names/UUIDs.
	// +optional
	Ops []string `json:"ops,omitempty"`

	// difficulty is the game difficulty setting.
	// +optional
	Difficulty string `json:"difficulty,omitempty"`

	// gameMode is the game mode (e.g., "survival", "creative", "adventure").
	// +optional
	GameMode string `json:"gameMode,omitempty"`

	// pvp enables player-vs-player combat.
	// +optional
	Pvp *bool `json:"pvp,omitempty"`

	// onlineMode enables online authentication.
	// +optional
	OnlineMode *bool `json:"onlineMode,omitempty"`
}

// BackupSpec defines backup configuration.
type BackupSpec struct {
	// schedule is the cron schedule for backups (e.g., "0 */6 * * *").
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`

	// retention is the number of backups to retain.
	// +kubebuilder:default=3
	// +optional
	Retention *int32 `json:"retention,omitempty"`

	// storageClass is the StorageClass for backup volumes.
	// +optional
	StorageClass *string `json:"storageClass,omitempty"`
}

// SchedulingSpec defines scheduling constraints.
type SchedulingSpec struct {
	// nodeSelector for pod placement.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// affinity for pod placement.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// tolerations for pod placement.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// SecuritySpec defines security configuration.
type SecuritySpec struct {
	// runAsNonRoot enforces running as non-root.
	// +kubebuilder:default=true
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty"`

	// readOnlyRootFilesystem enforces a read-only root filesystem.
	// +optional
	ReadOnlyRootFilesystem *bool `json:"readOnlyRootFilesystem,omitempty"`

	// dropAllCapabilities drops all Linux capabilities.
	// +kubebuilder:default=true
	// +optional
	DropAllCapabilities *bool `json:"dropAllCapabilities,omitempty"`

	// seccompProfile type to use.
	// +kubebuilder:default=RuntimeDefault
	// +optional
	SeccompProfile corev1.SeccompProfileType `json:"seccompProfile,omitempty"`
}

// PortInfo provides status information about an exposed port.
type PortInfo struct {
	Name     string          `json:"name"`
	Port     int32           `json:"port"`
	Protocol corev1.Protocol `json:"protocol"`
}

// GameServerSpec defines the desired state of GameServer
type GameServerSpec struct {
	Game       GameSpec                    `json:"game"`
	Runtime    RuntimeSpec                 `json:"runtime"`
	Resources  corev1.ResourceRequirements `json:"resources,omitempty"`
	Storage    StorageSpec                 `json:"storage"`
	Network    NetworkSpec                 `json:"network"`
	Server     *ServerSpec                 `json:"server,omitempty"`
	Backup     *BackupSpec                 `json:"backup,omitempty"`
	Scheduling *SchedulingSpec             `json:"scheduling,omitempty"`
	Security   *SecuritySpec               `json:"security,omitempty"`
}

// GameServerStatus defines the observed state of GameServer.
type GameServerStatus struct {
	// conditions represent the current state of the GameServer resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// phase is the current lifecycle phase.
	// +optional
	Phase GameServerPhase `json:"phase,omitempty"`

	// ready indicates the server is operational.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// endpoint is the connection endpoint.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// address is the server IP address or hostname.
	// +optional
	Address string `json:"address,omitempty"`

	// ports lists exposed port info.
	// +optional
	Ports []PortInfo `json:"ports,omitempty"`

	// playerCount is the current number of connected players.
	// +optional
	PlayerCount *int32 `json:"playerCount,omitempty"`

	// observedGeneration reflects the metadata.generation last processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Game",type=string,JSONPath=`.spec.game.type`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.game.version`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Address",type=string,JSONPath=`.status.address`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:scope=Namespaced,shortName=gs;gsv

// GameServer is the Schema for the gameservers API
type GameServer struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of GameServer
	// +required
	Spec GameServerSpec `json:"spec"`

	// status defines the observed state of GameServer
	// +optional
	Status GameServerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// GameServerList contains a list of GameServer
type GameServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []GameServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameServer{}, &GameServerList{})
}
