/*
Copyright 2020.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SosreportSpec defines the desired state of Sosreport
type SosreportSpec struct {
	// Select nodes to run sosreports on.
	// For example, in order to generate Sosreports on all master nodes, use
	// node-role.kubernetes.io/master: ""
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Sosreport jobs will respect Node Taints. One can work around this by configuring tolerations.
	Tolerations []corev1.Toleration `json:"tolerations,omitempty" protobuf:"bytes,22,opt,name=tolerations"`
}

// SosreportStatus defines the observed state of Sosreport
type SosreportStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Finished              bool     `json:"finished,omitempty"`
	InProgress            bool     `json:"inprogress,omitempty"`
	CurrentlyRunningNodes []string `json:"currentlyrunningnodes,omitempty"`
	OutstandingNodes      []string `json:"outstandingnodes,omitempty"`
}

// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Finished",type=boolean,JSONPath=`.status.finished`
// +kubebuilder:printcolumn:name="In Progress",type=boolean,JSONPath=`.status.inprogress`
// +kubebuilder:printcolumn:name="Currently Running Nodes",type=string,JSONPath=`.status.currentlyrunningnodes`

// Sosreport is the Schema for the sosreports API
type Sosreport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SosreportSpec   `json:"spec,omitempty"`
	Status SosreportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SosreportList contains a list of Sosreport
type SosreportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sosreport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Sosreport{}, &SosreportList{})
}
