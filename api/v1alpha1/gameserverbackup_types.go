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

const GameServerBackupFinalizer = "games.gobehost.com/backup-finalizer"

// Condition types for GameServerBackup.
const (
	BackupReady     = "Ready"
	BackupSucceeded = "LastBackupSucceeded"
)

// Condition reasons for GameServerBackup.
const (
	BackupCronJobCreated = "CronJobCreated"
	BackupTargetNotFound = "TargetNotFound"
	BackupStorageMissing = "StorageConfigMissing"
	BackupCronJobFailed  = "CronJobFailed"
	BackupPVCNotReady    = "PVCNotReady"
	BackupInvalidCreds   = "InvalidCredentials"
	BackupStorageUnavail = "StorageUnavailable"
)

// BackupStorageSpec defines S3 storage configuration for backups.
type BackupStorageSpec struct {
	// endpoint is the S3-compatible endpoint URL. Uses the platform default if empty.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// bucket is the S3 bucket name. Uses the platform default if empty.
	// +optional
	Bucket string `json:"bucket,omitempty"`

	// path is the prefix within the bucket. Defaults to <namespace>/<target-name>.
	// +optional
	Path string `json:"path,omitempty"`

	// secretRef references a Secret with S3 credentials.
	// +optional
	SecretRef *BackupSecretRef `json:"secretRef,omitempty"`
}

// BackupSecretRef references a Secret containing S3 credentials.
type BackupSecretRef struct {
	// name is the Secret name in the same namespace.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// TargetReference points to a GameServer or GameServerFleet.
type TargetReference struct {
	// kind is the target resource kind.
	// +kubebuilder:validation:Enum=GameServer;GameServerFleet
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// name is the target resource name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// GameServerBackupSpec defines the desired state of GameServerBackup.
type GameServerBackupSpec struct {
	// targetRef specifies what to back up.
	// +kubebuilder:validation:Required
	TargetRef TargetReference `json:"targetRef"`

	// schedule is the cron schedule for backups (e.g., "0 */6 * * *").
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`

	// retention is the number of backups to keep.
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +optional
	Retention *int32 `json:"retention,omitempty"`

	// storage configures where backups are stored. Uses platform defaults if empty.
	// +optional
	Storage *BackupStorageSpec `json:"storage,omitempty"`

	// includeMetadata controls whether CRD YAML and secrets are included in the backup.
	// +kubebuilder:default=true
	// +optional
	IncludeMetadata *bool `json:"includeMetadata,omitempty"`

	// backupOnDelete triggers a final backup before the target is deleted.
	// +kubebuilder:default=true
	// +optional
	BackupOnDelete *bool `json:"backupOnDelete,omitempty"`
}

// GameServerBackupStatus defines the observed state of GameServerBackup.
type GameServerBackupStatus struct {
	// lastBackupTime is the timestamp of the most recent completed backup.
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// lastBackupStatus is "Success" or "Failed" for the most recent backup.
	// +optional
	LastBackupStatus string `json:"lastBackupStatus,omitempty"`

	// nextBackupTime is when the next scheduled backup will run.
	// +optional
	NextBackupTime *metav1.Time `json:"nextBackupTime,omitempty"`

	// observedGeneration reflects the metadata.generation last processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions represent the current state of the GameServerBackup resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Schedule,type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name=LastBackup,type=date,JSONPath=`.status.lastBackupTime`
// +kubebuilder:printcolumn:name=Status,type=string,JSONPath=`.status.lastBackupStatus`
// +kubebuilder:printcolumn:name=Age,type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:scope=Namespaced,shortName=gsb

// GameServerBackup is the Schema for the gameserverbackups API
type GameServerBackup struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of GameServerBackup
	// +required
	Spec GameServerBackupSpec `json:"spec"`

	// status defines the observed state of GameServerBackup
	// +optional
	Status GameServerBackupStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// GameServerBackupList contains a list of GameServerBackup
type GameServerBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []GameServerBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameServerBackup{}, &GameServerBackupList{})
}
