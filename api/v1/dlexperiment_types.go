/*


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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DLExperimentSpec defines the desired state of DLExperiment
type DLExperimentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of DLExperiment. Edit DLExperiment_types.go to remove/update
	User             string   `json:"user"`
	Workspace        string   `json:"workspace"`
	ExperimentID     string   `json:"experiment_id"`
	Trainer          string   `json:"trainer"`
	TrailConcurrency int      `json:"trial_concurrency"`
	MaxTrialNum      int      `json:"num"`
	Tuner            string   `json:"tuner"`
	Target           string   `json:"target"`
	GpuRequired      int      `json:"gpu_num"`
	SearchSpace      string   `json:"search_space"`
	Command          string   `json:"command"`
	TunerImage       string   `json:"tuner_image"`
	TrialImage       string   `json:"trial_image"`
	Datasets         []string `json:"datasets"`
	WorkingDir       string   `json:"working_dir"`
}

// DLExperimentStatus defines the observed state of DLExperiment
type DLExperimentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status string `json:"status,omitempty"`
	Count  *int   `json:"count,omitempty"`
}

// +kubebuilder:object:root=true

// DLExperiment is the Schema for the dlexperiments API
type DLExperiment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DLExperimentSpec   `json:"spec,omitempty"`
	Status DLExperimentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DLExperimentList contains a list of DLExperiment
type DLExperimentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DLExperiment `json:"items"`
}

type ExperimentStatus string

const (
	ExperimentCreated   = "Created"
	ExperimentRunning   = "Running"
	ExperimentFailed    = "Failed"
	ExperimentCompleted = "Completed"
)

func init() {
	SchemeBuilder.Register(&DLExperiment{}, &DLExperimentList{})
}
