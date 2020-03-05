//
// Copyright 2020 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package v1alpha1

import (
	"time"

	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The OperandRequestSpec identifies one or more specific operands (from a specific Registry) that should actually be installed
// +k8s:openapi-gen=true
type OperandRequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Requests defines a list of operand installation
	// +listType=set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Requests []Request `json:"requests"`
}

// Request identifies a operand detail
type Request struct {
	// Names the OperandRegistry entry for the operand to be deployed
	Operands          []Operand `json:"operands"`
	Registry          string    `json:"registry"`
	RegistryNamespace string    `json:"registryNamespace"`
	Description       string    `json:"description,omitempty"`
}

type Operand struct {
	Name     string    `json:"name"`
	Bindings []Binding `json:"bindings,omitempty"`
}

type Binding struct {
	Scope          scope  `json:"scope,omitempty"`
	Secret         string `json:"secret,omitempty"`
	ConfigMap      string `json:"configMap,omitempty"`
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

// ConditionType is the condition of a service
type ConditionType string

// ClusterPhase is the phase of the installation
type ClusterPhase string

// ResourceType is the type of condition use
type ResourceType string

// Constants are used for state
const (
	ConditionCreating ConditionType = "Creating"
	ConditionUpdating ConditionType = "Updating"
	ConditionDeleting ConditionType = "Deleting"

	ClusterPhaseNone     ClusterPhase = ""
	ClusterPhaseCreating ClusterPhase = "Creating"
	ClusterPhaseRunning  ClusterPhase = "Running"
	ClusterPhaseFailed   ClusterPhase = "Failed"

	ResourceTypeSub     ResourceType = "subscription"
	ResourceTypeCsv     ResourceType = "csv"
	ResourceTypeOperand ResourceType = "operand"
)

// Conditions represents the current state of the Request Service
// A condition might not show up if it is not happening.
type Condition struct {
	// Type of condition.
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

// OperandRequestStatus defines the observed state of OperandRequest
// +k8s:openapi-gen=true
type OperandRequestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Conditions represents the current state of the Request Service
	// +optional
	// +listType=set
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Conditions []Condition `json:"conditions,omitempty"`
	// Members represnets the current operand status of the set
	// +optional
	// +listType=set
	Members []MemberStatus `json:"members,omitempty"`
	// Phase is the cluster running phase
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Phase ClusterPhase `json:"phase"`
}

// MemberPhase show the phase of the operator and operator instance
type MemberPhase struct {
	OperatorPhase olmv1alpha1.ClusterServiceVersionPhase `json:"operatorPhase,omitempty"`
	OperandPhase  ServicePhase                           `json:"operandPhase,omitempty"`
}

// MemberStatus show if the Operator is ready
type MemberStatus struct {
	// The member name are the same as the subscription name
	Name string `json:"name"`
	// The operand phase include None, Creating, Running, Failed
	Phase MemberPhase `json:"phase"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandRequest is the Schema for the operandrequests API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operandrequests,shortName=opreq,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="OperandRequest"
type OperandRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperandRequestSpec   `json:"spec,omitempty"`
	Status OperandRequestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandRequestList contains a list of OperandRequest
type OperandRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperandRequest `json:"items"`
}

func (r *OperandRequest) SetCreatingCondition(name string, rt ResourceType) {
	c := newCondition(ConditionCreating, corev1.ConditionTrue, "Creating "+string(rt), "Creating "+string(rt)+" "+name)
	r.setCondition(*c)
}

func (r *OperandRequest) SetUpdatingCondition(name string, rt ResourceType) {
	c := newCondition(ConditionUpdating, corev1.ConditionTrue, "Updating "+string(rt), "Updating "+string(rt)+" "+name)
	r.setCondition(*c)
}

func (r *OperandRequest) SetDeletingCondition(name string, rt ResourceType) {
	c := newCondition(ConditionDeleting, corev1.ConditionTrue, "Deleting "+string(rt), "Deleting "+string(rt)+" "+name)
	r.setCondition(*c)
}

func (r *OperandRequest) setCondition(c Condition) {
	pos, cp := getCondition(&r.Status, c.Type, c.Message)
	if cp != nil {
		r.Status.Conditions[pos] = c
	} else {
		r.Status.Conditions = append(r.Status.Conditions, c)
	}
}

func getCondition(status *OperandRequestStatus, t ConditionType, msg string) (int, *Condition) {
	for i, c := range status.Conditions {
		if t == c.Type && msg == c.Message {
			return i, &c
		}
	}
	return -1, nil
}

func newCondition(condType ConditionType, status corev1.ConditionStatus, reason, message string) *Condition {
	now := time.Now().Format(time.RFC3339)
	return &Condition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     now,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}
}

func (r *OperandRequest) SetMemberStatus(name string, operatorPhase olmv1alpha1.ClusterServiceVersionPhase, operandPhase ServicePhase) {
	m := newMemberStatus(name, operatorPhase, operandPhase)
	pos, mp := getMemberStatus(&r.Status, name)
	if mp != nil {
		if m.Phase.OperatorPhase != mp.Phase.OperatorPhase {
			r.Status.Members[pos].Phase.OperatorPhase = m.Phase.OperatorPhase
		}
		if m.Phase.OperandPhase != mp.Phase.OperandPhase {
			r.Status.Members[pos].Phase.OperandPhase = m.Phase.OperandPhase
		}
	} else {
		r.Status.Members = append(r.Status.Members, m)
	}
}

func (r *OperandRequest) CleanMemberStatus(name string) {
	pos, _ := getMemberStatus(&r.Status, name)
	if pos != -1 {
		r.Status.Members = append(r.Status.Members[:pos], r.Status.Members[pos+1:]...)
	}
}

func getMemberStatus(status *OperandRequestStatus, name string) (int, *MemberStatus) {
	for i, m := range status.Members {
		if name == m.Name {
			return i, &m
		}
	}
	return -1, nil
}

func newMemberStatus(name string, operatorPhase olmv1alpha1.ClusterServiceVersionPhase, operandPhase ServicePhase) MemberStatus {
	return MemberStatus{
		Name: name,
		Phase: MemberPhase{
			OperatorPhase: operatorPhase,
			OperandPhase:  operandPhase,
		},
	}
}

func (r *OperandRequest) SetClusterPhase(p ClusterPhase) {
	r.Status.Phase = p
}

func init() {
	SchemeBuilder.Register(&OperandRequest{}, &OperandRequestList{})
}