package main

import (
	"fmt"
	"net/netip"

	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/vishvananda/netlink"
)

// ensure creates all objs to match lan's spec
func (r *LANReconciler) ensure(lan *v1beta1.LAN) error {
	var err error
	//get underly link
	vxDevName := *lan.Spec.DefaultVxDev
	if _, ok := lan.Spec.VxDevMap[r.hostName]; ok {
		vxDevName = lan.Spec.VxDevMap[r.hostName]
	}
	var vxDevLink netlink.Link
	if vxDevName == v1beta1.DefaultVxLANDevAuto {
		//auto detect dev, using default route egress interface
		vxDevLink, err = getDefaultRouteInterface()
		if err != nil {
			return fmt.Errorf("auto determine vxlan dev failed, %w", err)
		}
		vxDevName = vxDevLink.Attrs().Name
	} else {
		vxDevLink, err = netlink.LinkByName(vxDevName)
		if err != nil {
			return fmt.Errorf("vxlan dev %v not found, %w", vxDevName, err)
		}
	}
	mtu := vxDevLink.Attrs().MTU - maxVxLANEncapOverhead

	//bridge
	needToAdd := false
	removeOld := false
	var br netlink.Link
	br, err = netlink.LinkByName(*lan.Spec.BridgeName)
	if err != nil {
		needToAdd = true
	} else {
		if br.Type() != "bridge" {
			if !lan.Spec.Force {
				return fmt.Errorf("interface %v already exists but not a bridge", lan.Spec.BridgeName)
			} else {
				needToAdd = true
				removeOld = true
			}
		}
	}
	if needToAdd {
		//remove existing one, if any
		if removeOld {
			netlink.LinkDel(br)
		}
		//create bridge interface
		la := netlink.NewLinkAttrs()
		la.Name = *lan.Spec.BridgeName
		la.MTU = mtu
		br = &netlink.Bridge{
			LinkAttrs: la,
		}
		if err := netlink.LinkAdd(br); err != nil {
			return fmt.Errorf("failed to create bridge %v: %v", lan.Spec.BridgeName, err)
		}
		// Bring the bridge up
		if err := netlink.LinkSetUp(br); err != nil {
			return fmt.Errorf("failed to bring bridge %v up, %w", lan.Spec.BridgeName, err)
		}
		br, _ = netlink.LinkByName(*lan.Spec.BridgeName)
	}
	//vxlan
	var vxLink netlink.Link
	needToAdd = false
	removeOld = false
	grpSpec := netip.MustParseAddr(*lan.Spec.VxLANGrp)

	vxLink, err = netlink.LinkByName(*lan.Spec.VxLANName)
	if err != nil {
		needToAdd = true
	} else {
		if vxLink.Type() != "vxlan" {
			if !lan.Spec.Force {
				return fmt.Errorf("interface %v already exists but not a vxlink", lan.Spec.VxLANName)
			} else {
				needToAdd = true
				removeOld = true
			}
		} else {
			//check vxlink config
			notEqualFunc := func(err error) error {
				if !lan.Spec.Force {
					return err
				} else {
					needToAdd = true
					removeOld = true
					return nil
				}
			}
			vx := vxLink.(*netlink.Vxlan)
			if grpSpec.Compare(netip.MustParseAddr(vx.Group.String())) != 0 {
				err = fmt.Errorf("existing vxlan interface has a different group addr: %v", vx.Group.String())
				if err = notEqualFunc(err); err != nil {
					return err
				}
			}
			if vx.VxlanId != int(*lan.Spec.VNI) {
				err = fmt.Errorf("existing vxlan interface has a different vni: %v", vx.VxlanId)
				if err = notEqualFunc(err); err != nil {
					return err
				}
			}
			if vx.VtepDevIndex != vxDevLink.Attrs().Index {
				err = fmt.Errorf("existing vxlan interface has a different dev index: %v", vx.VtepDevIndex)
				if err = notEqualFunc(err); err != nil {
					return err
				}
			}

		}
	}

	if needToAdd {
		if removeOld {
			netlink.LinkDel(vxLink)
		}
		vxLink, err = CreateVXLANIF(lan, vxDevName, mtu, int(*lan.Spec.VxPort))
		if err != nil {
			return fmt.Errorf("failed to create vxlan interface, %w", err)
		}

	}
	//attach vxlan to br
	err = netlink.LinkSetMaster(vxLink, br)
	if err != nil {

		return fmt.Errorf("failed to set master of vxlan interface, %w", err)
	}
	//set grp_fwd_mask
	err = netlink.LinkSetBRSlaveGroupFwdMask(vxLink, BRSlaveGrpFwdMask)
	if err != nil {
		return fmt.Errorf("failed to set brg slave grp fwd mask, %w", err)
	}
	//creating vlan interfaces
	for i, vlanName := range lan.Spec.VlanNameList {
		la := netlink.LinkAttrs{
			ParentIndex: br.Attrs().Index,
			MTU:         mtu - 4,
			Name:        vlanName,
		}
		vlink := &netlink.Vlan{
			VlanId:    i + 1,
			LinkAttrs: la,
		}
		if err := netlink.LinkAdd(vlink); err != nil {
			return fmt.Errorf("failed to create vlan interface %v: %v", vlanName, err)
		}
		if err := netlink.LinkSetUp(vlink); err != nil {
			return fmt.Errorf("failed to bring vlan %v up, %w", vlanName, err)
		}
	}

	return nil

}

func getDefaultRouteInterface() (netlink.Link, error) {
	// Get all routes from the system
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}

	// Find the default route (destination 0.0.0.0/0 or ::/0)
	for _, route := range routes {
		// Check if this is a default route
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" || route.Dst.String() == "::/0" {
			// Get the interface by index
			link, err := netlink.LinkByIndex(route.LinkIndex)
			if err != nil {
				return nil, fmt.Errorf("failed to get link by index %d: %w", route.LinkIndex, err)
			}

			return link, nil
		}
	}

	return nil, fmt.Errorf("no default route found")
}
