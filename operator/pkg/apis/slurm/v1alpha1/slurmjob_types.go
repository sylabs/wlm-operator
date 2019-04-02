package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SlurmJobSpec defines the desired state of SlurmJob
// +k8s:openapi-gen=true
type SlurmJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html.

	// Batch is a script that will be submitted to a Slurm cluster as a batch job.
	// +kubebuilder:validation:MinLength=1
	Batch string `json:"batch"`

	// NodeSelector is a selector which must be true for the SlurmJob to fit on a node.
	// Selector which must match a node's labels for the SlurmJob to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Compute resources required by this SlurmJob.
	// Cannot be updated. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources.
	Resources map[string]int64 `json:"resources,omitempty"`

	// SSH contains configuration to be used when running jobs on a remote cluster via ssh.
	SSH *SlurmJobSSH `json:"ssh,omitempty"`

	// Results may be specified for an optional results collection step.
	// When specified, after job is completed all results will be downloaded from Slurm
	// cluster with respect to this configuration.
	Results *SlurmJobResults `json:"results,omitempty"`
}

// SlurmJobStatus defines the observed state of SlurmJob
// +k8s:openapi-gen=true
type SlurmJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	Status  string `json:"status"`
	Message string `json:"message"`
}

// SlurmJobSSH is a Schema for required data to execute job via ssh
// +k8s:openapi-gen=true
type SlurmJobSSH struct {
	// User to be used when establishing ssh connection to a Slurm cluster.
	// +kubebuilder:validation:MinLength=1
	User string `json:"user,omitempty"`

	// Key to use to connect to a remote Slurm cluster.
	Key *v1.EnvVarSource `json:"key,omitempty"`

	// Password to use when logging in as a User.
	Password *v1.EnvVarSource `json:"password,omitempty"`
}

// +k8s:openapi-gen=true
type SlurmJobResults struct {
	// Mount is a directory where job results will be stored.
	// After results collection all job generated files can be found in Mount/<SlurmJob.Name> directory.
	// +kubebuilder:validation:MinLength=1
	Mount v1.Volume `json:"mount"`

	// From is a path to the results to be collected from a Slurm cluster.
	From string `json:"from"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SlurmJob is the Schema for the slurmjobs API
// +k8s:openapi-gen=true
type SlurmJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SlurmJobSpec   `json:"spec,omitempty"`
	Status SlurmJobStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SlurmJobList contains a list of SlurmJob
type SlurmJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SlurmJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SlurmJob{}, &SlurmJobList{})
}
