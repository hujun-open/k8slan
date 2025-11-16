package interfaces

import (
	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/vishvananda/netlink"
)

func Remove(lan *v1beta1.LAN) {
	//veth
	for _, spoke := range lan.Spec.SpokeList {
		vethLink, err := netlink.LinkByName(getPeerVethName(spoke))
		if err == nil {
			netlink.LinkDel(vethLink)
		}
	}
	//vxlan
	vxlink, err := netlink.LinkByName(*lan.Spec.VxLANName)
	if err == nil {
		netlink.LinkDel(vxlink)
	}

	//bridge
	brlink, err := netlink.LinkByName(*lan.Spec.BridgeName)
	if err == nil {
		netlink.LinkDel(brlink)
	}

}
