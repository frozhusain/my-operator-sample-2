/*
Copyright 2025.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ConfigMapSyncSpec defines the desired state of ConfigMapSync
type ConfigMapSyncSpec struct {
	// SourceNamespace is the namespace where the source ConfigMap is located
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SourceNamespace string `json:"sourceNamespace"`

	// DestinationNamespace is the namespace where the ConfigMap should be synced to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	DestinationNamespace string `json:"destinationNamespace"`

	// ConfigMapName is the name of the ConfigMap to sync
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ConfigMapName string `json:"configMapName"`

	// SyncInterval defines how often to check for changes in minutes
	// +optional
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1440
	SyncInterval int32 `json:"syncInterval,omitempty"`

	// SyncLabels determines whether to sync labels from the source ConfigMap
	// +optional
	// +kubebuilder:default=false
	SyncLabels bool `json:"syncLabels,omitempty"`

	// SyncAnnotations determines whether to sync annotations from the source ConfigMap
	// +optional
	// +kubebuilder:default=false
	SyncAnnotations bool `json:"syncAnnotations,omitempty"`
}

// ConfigMapSyncStatus defines the observed state of ConfigMapSync
type ConfigMapSyncStatus struct {
	// SyncStatus represents the current sync state of the ConfigMap
	// +optional
	SyncStatus string `json:"syncStatus,omitempty"`

	// LastSyncTime is the last time the ConfigMap was successfully synced
	// +optional
	LastSyncTime metav1.Time `json:"lastSyncTime,omitempty"`

	// SourceGeneration is the observed generation of the source ConfigMap
	// that was last synced
	// +optional
	SourceGeneration int64 `json:"sourceGeneration,omitempty"`

	// Message provides additional information about the current sync status
	// +optional
	Message string `json:"message,omitempty"`

	// Reason is a brief description of why the ConfigMap sync is in the current state
	// +optional
	Reason string `json:"reason,omitempty"`

	// Conditions represent the latest available observations of the ConfigMapSync state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// ConfigMapSync is the Schema for the configmapsyncs API
type ConfigMapSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ConfigMapSyncSpec   `json:"spec,omitempty"`
	Status            ConfigMapSyncStatus `json:"status,omitempty"`
}

func (c ConfigMapSync) DeepCopyObject() runtime.Object {
	return nil
}

// +kubebuilder:object:root=true
// ConfigMapSyncList contains a list of ConfigMapSync
type ConfigMapSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigMapSync `json:"items"`
}

func (c ConfigMapSyncList) DeepCopyObject() runtime.Object {
	return nil
}

func init() {
	SchemeBuilder.Register(&ConfigMapSync{}, &ConfigMapSyncList{})
}
