package v1beta1

import (
	"net/netip"

	"github.com/containernetworking/cni/pkg/types"
)

// +kubebuilder:object:generate=false
// +kubebuilder:object:root=false
type K8sLANVethPluginConf struct {
	types.NetConf
	VethName      string         `json:"veth"`
	Addresses     []netip.Prefix `json:"addrs,omitempty"`
	Routes        []Route        `json:"routes,omitempty"`
	TxChecksumOff bool           `json:"disableTxChecksum"`
	PeerNS        string         `json:"peerNS"`
}

func DefK8sLANVethPluginConf() *K8sLANVethPluginConf {
	return &K8sLANVethPluginConf{
		NetConf: types.NetConf{
			CNIVersion: "0.3.1",
			Type:       "k8slanveth",
		},
		TxChecksumOff: true,
	}
}

// +kubebuilder:object:generate=false
// +kubebuilder:object:root=false
type MACVTAPPluginConf struct {
	types.NetConf
	//following is from MACVTAP pkg/cni/plugin.go
	DeviceID      string `json:"deviceID"`
	MTU           int    `json:"mtu,omitempty"`
	IsPromiscuous bool   `json:"promiscMode,omitempty"`
	Owner         int    `json:"owner,omitempty"`
	Group         int    `json:"group,omitempty"`
}

func GetMACVTAPPluginConf(name string) *MACVTAPPluginConf {
	return &MACVTAPPluginConf{
		NetConf: types.NetConf{
			CNIVersion: "0.3.1",
			Type:       "macvtap",
			Name:       name,
		},
		IsPromiscuous: true,
	}
}
