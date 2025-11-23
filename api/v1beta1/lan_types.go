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

package v1beta1

import (
	"fmt"
	"net/netip"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	DefaultVxPort = 4789
	DefaultVxGrp  = "FF02:0:0:0:0:0:0:14"
)

// LANSpec defines the desired state of LAN
type LANSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// +required
	NS *string `json:"ns,omitempty"`
	// +required
	BridgeName *string `json:"bridge,omitempty"`
	// +required
	VxLANName *string `json:"vxlan,omitempty"`
	// +required
	VNI *int32 `json:"vni,omitempty"`
	// +required
	VxLANGrp *string `json:"vxlanGrp,omitempty"`
	// +optional
	DefaultVxDev string `json:"defaultVxlanDev,omitempty"`
	// +optional
	VxDevMap map[string]string `json:"vxlanDevMap,omitempty"`
	// +optional
	VxPort *int32 `json:"vxlanPort,omitempty"`
	// +required
	SpokeList []string `json:"spokes,omitempty"`
}

const (
	maxLinuxIfNameLen = 13
)

func checkInterfaceName(ifname string) error {
	nlen := len(ifname)
	if nlen == 0 || nlen > maxLinuxIfNameLen {
		return fmt.Errorf("invalid length of interface name %v, must be 1..%d", ifname, maxLinuxIfNameLen)
	}
	return nil
}

func (spec *LANSpec) Validate() error {

	if err := checkInterfaceName(*spec.BridgeName); err != nil {
		return err
	}
	if err := checkInterfaceName(*spec.VxLANName); err != nil {
		return err
	}

	if strings.TrimSpace(*spec.NS) == "" {
		return fmt.Errorf("ns is not specified")
	}

	if *spec.VNI <= 0 || *spec.VNI > 0xFFFFFF {
		return fmt.Errorf("invalid vni %d, must be 1..16777215", *spec.VNI)
	}
	addr, err := netip.ParseAddr(*spec.VxLANGrp)
	if err != nil {
		return fmt.Errorf("%v is not valid address. %w", *spec.VxLANGrp, err)
	}
	if !addr.IsMulticast() {
		return fmt.Errorf("%v is not a multicast address", *spec.VxLANGrp)
	}
	if len(spec.SpokeList) == 0 || len(spec.SpokeList) > 4095 {
		return fmt.Errorf("the number of vlan names must be in range of 1..4095")
	}
	for _, ifname := range spec.SpokeList {
		if err := checkInterfaceName(ifname); err != nil {
			return err
		}
	}
	return nil
}

// LANStatus defines the observed state of LAN.
type LANStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the LAN resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LAN is the Schema for the lans API
type LAN struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of LAN
	// +required
	Spec LANSpec `json:"spec"`

	// status defines the observed state of LAN
	// +optional
	Status LANStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// LANList contains a list of LAN
type LANList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LAN `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LAN{}, &LANList{})
}
