package main

import (
	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/vishvananda/netlink"
)

func (r *LANReconciler) remove(lan *v1beta1.LAN) {
	//vxlan
	vxlink, err := netlink.LinkByName(*lan.Spec.VxLANName)
	if err == nil {
		netlink.LinkDel(vxlink)
	}
	//vlans
	for _, vlanName := range lan.Spec.VlanNameList {
		vlink, err := netlink.LinkByName(vlanName)
		if err == nil {
			netlink.LinkDel(vlink)
		}
	}
	//bridge
	brlink, err := netlink.LinkByName(*lan.Spec.BridgeName)
	if err == nil {
		netlink.LinkDel(brlink)
	}
}
