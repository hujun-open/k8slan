package interfaces

import (
	"fmt"
	"net/netip"

	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/vishvananda/netlink"
)

// ensure creates all objs to match lan's spec
func Ensure(macName, vlanName string, lan *v1beta1.LANSpec, hostname, macvtapMode string) (int, error) {
	var err error
	//get underly link
	vxDevName := *lan.DefaultVxDev
	if _, ok := lan.VxDevMap[hostname]; ok {
		vxDevName = lan.VxDevMap[hostname]
	}
	var vxDevLink netlink.Link
	if vxDevName == v1beta1.DefaultVxLANDevAuto {
		//auto detect dev, using default route egress interface
		vxDevLink, err = getDefaultRouteInterface()
		if err != nil {
			return -1, fmt.Errorf("auto determine vxlan dev failed, %w", err)
		}
		vxDevName = vxDevLink.Attrs().Name
	} else {
		vxDevLink, err = netlink.LinkByName(vxDevName)
		if err != nil {
			return -1, fmt.Errorf("vxlan dev %v not found, %w", vxDevName, err)
		}
	}
	mtu := vxDevLink.Attrs().MTU - maxVxLANEncapOverhead

	//bridge
	needToAdd := false
	removeOld := false
	var br netlink.Link
	br, err = netlink.LinkByName(*lan.BridgeName)
	if err != nil {
		needToAdd = true
	} else {
		if br.Type() != "bridge" {
			if !lan.Force {
				return -1, fmt.Errorf("interface %v already exists but not a bridge", lan.BridgeName)
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
		la.Name = *lan.BridgeName
		la.MTU = mtu
		br = &netlink.Bridge{
			LinkAttrs: la,
		}
		if err := netlink.LinkAdd(br); err != nil {
			return -1, fmt.Errorf("failed to create bridge %v: %v", lan.BridgeName, err)
		}
		// Bring the bridge up
		if err := netlink.LinkSetUp(br); err != nil {
			return -1, fmt.Errorf("failed to bring bridge %v up, %w", lan.BridgeName, err)
		}
		br, _ = netlink.LinkByName(*lan.BridgeName)
	}
	//vxlan
	var vxLink netlink.Link
	needToAdd = false
	removeOld = false
	grpSpec := netip.MustParseAddr(*lan.VxLANGrp)

	vxLink, err = netlink.LinkByName(*lan.VxLANName)
	if err != nil {
		needToAdd = true
	} else {
		if vxLink.Type() != "vxlan" {
			if !lan.Force {
				return -1, fmt.Errorf("interface %v already exists but not a vxlink", lan.VxLANName)
			} else {
				needToAdd = true
				removeOld = true
			}
		} else {
			//check vxlink config
			notEqualFunc := func(err error) error {
				if !lan.Force {
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
					return -1, err
				}
			}
			if vx.VxlanId != int(*lan.VNI) {
				err = fmt.Errorf("existing vxlan interface has a different vni: %v", vx.VxlanId)
				if err = notEqualFunc(err); err != nil {
					return -1, err
				}
			}
			if vx.VtepDevIndex != vxDevLink.Attrs().Index {
				err = fmt.Errorf("existing vxlan interface has a different dev index: %v", vx.VtepDevIndex)
				if err = notEqualFunc(err); err != nil {
					return -1, err
				}
			}

		}
	}

	if needToAdd {
		if removeOld {
			netlink.LinkDel(vxLink)
		}
		vxLink, err = CreateVXLANIF(lan, vxDevName, mtu, int(*lan.VxPort))
		if err != nil {
			return -1, fmt.Errorf("failed to create vxlan interface, %w", err)
		}

	}
	//attach vxlan to br
	err = netlink.LinkSetMaster(vxLink, br)
	if err != nil {

		return -1, fmt.Errorf("failed to set master of vxlan interface, %w", err)
	}
	//set grp_fwd_mask
	err = netlink.LinkSetBRSlaveGroupFwdMask(vxLink, BRSlaveGrpFwdMask)
	if err != nil {
		return -1, fmt.Errorf("failed to set brg slave grp fwd mask, %w", err)
	}
	//creating vlan interfaces
	//remove existing vlan interface with same name
	vlanLink, err := netlink.LinkByName(vlanName)
	if err == nil {
		if !lan.Force {
			return -1, fmt.Errorf("vlan interface %v already exists", vlanName)
		} else {
			netlink.LinkDel(vlanLink)
		}
	}
	vlanID := -1
	for i, name := range lan.VlanNameList {
		if name == vlanName {
			vlanID = i + 1
			break
		}
	}
	la := netlink.LinkAttrs{
		ParentIndex: br.Attrs().Index,
		MTU:         mtu - 4,
		Name:        vlanName,
	}
	vlink := &netlink.Vlan{
		VlanId:    vlanID,
		LinkAttrs: la,
	}
	if err := netlink.LinkAdd(vlink); err != nil {
		return -1, fmt.Errorf("failed to create vlan interface %v: %v", vlanName, err)
	}
	if err := netlink.LinkSetUp(vlink); err != nil {
		return -1, fmt.Errorf("failed to bring vlan %v up, %w", vlanName, err)
	}
	//create macvtap interface
	return RecreateMacvtap(macName, vlanName, macvtapMode)
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

func ModeFromString(s string) (netlink.MacvlanMode, error) {
	switch s {
	case "", "bridge":
		return netlink.MACVLAN_MODE_BRIDGE, nil
	case "private":
		return netlink.MACVLAN_MODE_PRIVATE, nil
	case "vepa":
		return netlink.MACVLAN_MODE_VEPA, nil
	case "passthru":
		return netlink.MACVLAN_MODE_PASSTHRU, nil
	default:
		return 0, fmt.Errorf("unknown macvtap mode: %q", s)
	}
}

func CreateMacvtap(name string, lowerDevice string, mode string) (int, error) {
	ifindex := 0

	m, err := netlink.LinkByName(lowerDevice)
	if err != nil {
		return ifindex, fmt.Errorf("failed to lookup lowerDevice %q: %v", lowerDevice, err)
	}

	nlmode, err := ModeFromString(mode)
	if err != nil {
		return ifindex, err
	}

	mv := &netlink.Macvtap{
		Macvlan: netlink.Macvlan{
			LinkAttrs: netlink.LinkAttrs{
				Name:        name,
				ParentIndex: m.Attrs().Index,
				// we had crashes if we did not set txqlen to some value
				TxQLen: m.Attrs().TxQLen,
			},
			Mode: nlmode,
		},
	}

	if err := netlink.LinkAdd(mv); err != nil {
		return ifindex, fmt.Errorf("failed to create macvtap: %v", err)
	}

	if err := netlink.LinkSetUp(mv); err != nil {
		return ifindex, fmt.Errorf("failed to set %q UP: %v", name, err)
	}

	ifindex = mv.Attrs().Index
	return ifindex, nil
}

func RecreateMacvtap(name string, lowerDevice string, mode string) (int, error) {
	err := LinkDelete(name)
	if err != nil {
		return 0, err
	}
	return CreateMacvtap(name, lowerDevice, mode)
}
func LinkDelete(link string) error {
	l, err := netlink.LinkByName(link)
	if _, ok := err.(netlink.LinkNotFoundError); ok {
		return nil
	}
	if err != nil {
		return err
	}
	err = netlink.LinkDel(l)
	return err
}
