package interfaces

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/vishvananda/netlink"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ensure creates all objs to match lan's spec
func Ensure(macName, spokeName string, lan *v1beta1.LANSpec, hostname, macvtapMode string) (int, error) {
	log := ctrl.Log.WithName("deviceplugin")
	var err error
	var lanNS ns.NetNS
	//make sure NS exists
	nsPath := filepath.Join(getNsRunDir(), *lan.NS)
	_, err = os.Stat(nsPath)
	if err != nil {
		//no exists
		lanNS, err = NewNS(*lan.NS)
		if err != nil {
			return -1, fmt.Errorf("failed to create ns %v, %w", *lan.NS, err)
		}
	} else {
		//exists
		lanNS, err = ns.GetNS(nsPath)
		if err != nil {
			return -1, fmt.Errorf("failed to open ns %v, %w", *lan.NS, err)
		}
	}
	//bring lo interface in NS up
	err = lanNS.Do(func(hostNs ns.NetNS) error {
		l, ierr := netlink.LinkByName("lo")
		if ierr != nil {
			return ierr
		}
		return netlink.LinkSetUp(l)
	})
	if err != nil {
		return -1, fmt.Errorf("failed to bring lo up in ns, %w", err)
	}

	//get underly link name
	vxDevName := lan.DefaultVxDev
	if _, ok := lan.VxDevMap[hostname]; ok {
		vxDevName = lan.VxDevMap[hostname]
	}

	//check it exists
	var vxDevLink netlink.Link
	vxDevLink, err = netlink.LinkByName(vxDevName)
	if err != nil {
		return -1, fmt.Errorf("vxlan dev %v not found, %w", vxDevName, err)
	}
	needToAdd := false
	mtu := vxDevLink.Attrs().MTU - maxVxLANEncapOverhead
	err = lanNS.Do(func(hostNs ns.NetNS) error {
		//bridge
		var br netlink.Link
		br, err = netlink.LinkByName(*lan.BridgeName)
		if err != nil {
			needToAdd = true
		} else {
			if br.Type() != "bridge" {
				return fmt.Errorf("interface %v already exists but not a bridge", lan.BridgeName)
			}
		}
		if needToAdd {
			//create bridge interface
			la := netlink.NewLinkAttrs()
			la.Name = *lan.BridgeName
			la.MTU = mtu
			la.TxQLen = -1 //this is important, otherwise the interface only accept broadcast traffic
			br = &netlink.Bridge{
				LinkAttrs: la,
			}
			if err := netlink.LinkAdd(br); err != nil {
				return fmt.Errorf("failed to create bridge %v: %v", lan.BridgeName, err)
			}
			// Bring the bridge up
			if err := netlink.LinkSetUp(br); err != nil {
				return fmt.Errorf("failed to bring bridge %v up, %w", lan.BridgeName, err)
			}
			br, _ = netlink.LinkByName(*lan.BridgeName)
		}
		//vxlan
		var vxLink netlink.Link
		needToAdd = false
		grpSpec := netip.MustParseAddr(*lan.VxLANGrp)

		vxLink, err = netlink.LinkByName(*lan.VxLANName)
		if err != nil {
			needToAdd = true
		} else {
			if vxLink.Type() != "vxlan" {
				return fmt.Errorf("interface %v already exists but not a vxlink", lan.VxLANName)
			} else {
				//check vxlink config
				vx := vxLink.(*netlink.Vxlan)
				if grpSpec.Compare(netip.MustParseAddr(vx.Group.String())) != 0 {
					log.Error(fmt.Errorf("existing vxlan interface has a different group addr: %v", vx.Group.String()), fmt.Sprintf("existing vlan interface %v has different config", *lan.VxLANName))
					needToAdd = true

				}
				if vx.VxlanId != int(*lan.VNI) {
					// err = fmt.Errorf("existing vxlan interface has a different vni: %v", vx.VxlanId)
					log.Error(fmt.Errorf("existing vxlan interface has a different vni: %v", vx.VxlanId), fmt.Sprintf("existing vlan interface %v has different config", *lan.VxLANName))
					needToAdd = true
				}
				if vx.VtepDevIndex != vxDevLink.Attrs().Index {
					// err = fmt.Errorf("existing vxlan interface has a different dev index: %v", vx.VtepDevIndex)
					log.Error(fmt.Errorf("existing vxlan interface has a different dev index: %v", vx.VtepDevIndex), fmt.Sprintf("existing vlan interface %v has different config", *lan.VxLANName))
					needToAdd = true
				}
				if vx.Port != int(*lan.VxPort) {
					log.Error(fmt.Errorf("existing vxlan interface has a different port: %v", vx.Port), fmt.Sprintf("existing vlan interface %v has different config", *lan.VxLANName))
					needToAdd = true
				}

			}
		}

		if needToAdd {
			LinkDelete(*lan.VxLANName)

			// vxLink, err = CreateVXLANIF(lan, vxDevLink.Attrs().Index, mtu, int(*lan.VxPort))
			// if err != nil {
			// 	return fmt.Errorf("failed to create vxlan interface, %w", err)
			// }

		}
		// //attach vxlan to br
		// err = netlink.LinkSetMaster(vxLink, br)
		// if err != nil {

		// 	return fmt.Errorf("failed to set master of vxlan interface, %w", err)
		// }
		// //set grp_fwd_mask
		// err = netlink.LinkSetBRSlaveGroupFwdMask(vxLink, BRSlaveGrpFwdMask)
		// if err != nil {
		// 	return fmt.Errorf("failed to set vxlan slave grp fwd mask, %w", err)
		// }
		//creating veth interfaces
		//remove existing vlan interface with same name
		peerName := getPeerVethName(spokeName)
		LinkDelete(peerName)
		time.Sleep(time.Second)
		la := netlink.LinkAttrs{
			ParentIndex: br.Attrs().Index,
			Name:        spokeName,
			TxQLen:      -1,
			MTU:         mtu,
		}

		vlink := &netlink.Veth{
			PeerName:  peerName,
			LinkAttrs: la,
		}
		if err := netlink.LinkAdd(vlink); err != nil {
			return fmt.Errorf("failed to create veth interface %v: %v", spokeName, err)
		}
		if err := netlink.LinkSetUp(vlink); err != nil {
			return fmt.Errorf("failed to veth %v up, %w", spokeName, err)
		}
		peerLink, err := netlink.LinkByName(peerName)
		if err != nil {
			return fmt.Errorf("failed to find peer veth %v, %w", peerName, err)
		}
		if err = netlink.LinkSetMaster(peerLink, br); err != nil {
			return fmt.Errorf("failed to set veth %v to master %v, %w", peerName, br.Attrs().Name, err)
		}
		//set grp_fwd_mask
		err = netlink.LinkSetBRSlaveGroupFwdMask(peerLink, BRSlaveGrpFwdMask)
		if err != nil {
			return fmt.Errorf("failed to set veth slave grp fwd mask, %w", err)
		}
		if err := netlink.LinkSetUp(peerLink); err != nil {
			return fmt.Errorf("failed to peer veth %v up, %w", peerName, err)
		}
		//move spoke link back to host ns
		return netlink.LinkSetNsFd(vlink, int(hostNs.Fd()))
	})
	if err != nil {
		return -1, err
	}
	//add vxlan interface if needed
	if needToAdd {
		err := CreateVXLANIF(lan, vxDevLink.Attrs().Index, int(lanNS.Fd()), mtu, int(*lan.VxPort))
		if err != nil {
			return -1, fmt.Errorf("failed to create vxlan interface, %w", err)
		}
		//bring it up
		err = lanNS.Do(func(hostNs ns.NetNS) error {
			vxLink, err := netlink.LinkByName(*lan.VxLANName)
			if err != nil {
				return err
			}
			br, err := netlink.LinkByName(*lan.BridgeName)
			if err != nil {
				return err
			}
			//attach vxlan to br
			err = netlink.LinkSetMaster(vxLink, br)
			if err != nil {
				return fmt.Errorf("failed to set master of vxlan interface, %w", err)
			}
			//set grp_fwd_mask
			err = netlink.LinkSetBRSlaveGroupFwdMask(vxLink, BRSlaveGrpFwdMask)
			if err != nil {
				return fmt.Errorf("failed to set vxlan slave grp fwd mask, %w", err)
			}
			//bring link up
			return netlink.LinkSetUp(vxLink)
		})
	}
	if err != nil {
		return -1, fmt.Errorf("failed to create vxlink interface in the ns, %w", err)
	}

	//bring up spoke link in host ns
	vlink, err := netlink.LinkByName(spokeName)
	if err != nil {
		return -1, fmt.Errorf("failed to get the created spoke link %v in host ns, %w", spokeName, err)
	}
	err = netlink.LinkSetUp(vlink)
	if err != nil {
		return -1, fmt.Errorf("failed to bring up spoke link %v in host ns, %w", spokeName, err)
	}
	//create macvtap interface
	return RecreateMacvtap(macName, spokeName, macvtapMode)

}

func getPeerVethName(name string) string {
	return name + "p"
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
