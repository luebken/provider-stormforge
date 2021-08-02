/*
Copyright 2020 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// MyTypeParameters are the configurable fields of a MyType.
type TestCaseParameters struct {
	ConfigurableField string `json:"configurableField"`
}

// MyTypeObservation are the observable fields of a MyType.
type TestCaseObservation struct {
	ObservableField string `json:"observableField,omitempty"`
}

// A TestCaseSpec defines the desired state of a MyType.
type TestCaseSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       TestCaseParameters `json:"forProvider"`
}

// A TestCaseStatus represents the observed state of a MyType.
type TestCaseStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          TestCaseObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A MyType is an example API type.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// Please replace `PROVIDER-NAME` with your actual provider name, like `aws`, `azure`, `gcp`, `alibaba`
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,stormforge}
type TestCase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestCaseSpec   `json:"spec"`
	Status TestCaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MyTypeList contains a list of MyType
type TestCaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TestCase `json:"items"`
}
