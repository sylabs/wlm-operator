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

// WlmJobSpec defines the desired state of WlmJob.
// +k8s:openapi-gen=true
type WlmJobSpec struct {
	// Image name to start as a job.
	Image string `json:"image"`

	// Options singularity run options.
	Options SingularityOptions `json:"options,omitempty"`

	// Resources describes required resources for a job.
	Resources WlmResources `json:"resources,omitempty"`

	// NodeSelector is a selector which must be true for the WlmJob to fit on a node.
	// Selector which must match a node's labels for the WlmJob to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Results may be specified for an optional results collection step.
	// When specified, after job is completed all results will be downloaded from WLM
	// cluster with respect to this configuration.
	Results *JobResults `json:"results,omitempty"`
}

// SingularityOptions singularity run options.
// +k8s:openapi-gen=true
type SingularityOptions struct {
	// Allow to pull and run unsigned images.
	AllowUnsigned bool `json:"allowUnsigned,omitempty"`
	// Clean environment before running container.
	CleanEnv bool `json:"cleanEnv,omitempty"`
	// Run container in new user namespace as uid 0.
	FakeRoot bool `json:"fakeRoot,omitempty"`
	// Run container in a new IPC namespace.
	IPC bool `json:"ipc,omitempty"`
	// Run container in a new PID namespace.
	PID bool `json:"pid,omitempty"`
	// Drop all privileges from root user in container.
	NoPrivs bool `json:"noPrivs,omitempty"`
	// By default all Singularity containers are
	// available as read only. This option makes
	// the file system accessible as read/write.
	Writable bool `json:"writable,omitempty"`
	// Set an application to run inside a container.
	App string `json:"app,omitempty"`
	// Set container hostname.
	HostName string `json:"hostName,omitempty"`
	// Binds a user-bind path specification. Spec has
	// the format src[:dest[:opts]], where src and
	// dest are outside and inside paths.  If dest
	// is not given, it is set equal to src.
	// Mount options ('opts') may be specified as
	// 'ro' (read-only) or 'rw' (read/write, which
	// is the default). Multiple bind paths can be
	// given by a comma separated list.
	Binds []string `json:"binds,omitempty"`
}

// WlmResources is a schema for wlm resources.
// +k8s:openapi-gen=true
type WlmResources struct {
	Nodes      int64 `json:"nodes,omitempty"`
	CPUPerNode int64 `json:"cpuPerNode,omitempty"`
	MemPerNode int64 `json:"memPerNode,omitempty"`
	// WallTime in seconds.
	WallTime int64 `json:"wallTime,omitempty"`
}
