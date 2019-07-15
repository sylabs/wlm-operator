// Copyright (c) 2019 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&WlmJob{}, &WlmJobList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WlmJob is the Schema for the wlm jobs API.
// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName=wj
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status",description="status of the kind"
type WlmJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WlmJobSpec   `json:"spec,omitempty"`
	Status WlmJobStatus `json:"status,omitempty"`
}

// WlmJobStatus defines the observed state of a WlmJob.
// +k8s:openapi-gen=true
type WlmJobStatus struct {
	// Status reflects job status, e.g running, succeeded.
	Status string `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WlmJobList contains a list of WlmJob.
type WlmJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WlmJob `json:"items"`
}

// WlmJobSpec defines the desired state of WlmJob
// +k8s:openapi-gen=true
type WlmJobSpec struct {
	Image     string       `json:"image"`
	Resources WlmResources `json:"resources,omitempty"`
	// Results may be specified for an optional results collection step.
	// When specified, after job is completed all results will be downloaded from WLM
	// cluster with respect to this configuration.
	Results *JobResults `json:"results,omitempty"`
}

// WlmResources is a schema for wlm resources.
// +k8s:openapi-gen=true
type WlmResources struct {
	Nodes      int64         `json:"nodes,omitempty"`
	CpuPerNode int64         `json:"cpuPerNode,omitempty"`
	MemPerNode int64         `json:"memPerNode,omitempty"`
	WallTime   time.Duration `json:"wallTime,omitempty"`
}
