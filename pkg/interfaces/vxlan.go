package interfaces

import (
	"errors"
	"fmt"
	"net/netip"
	"syscall"

	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/vishvananda/netlink"
)

type Config struct {
	BRLink             netlink.Link
	IfName             string
	MulticastGrpPrefix netip.Prefix
	MTU                int
	VNIAllocatorFQDN   string
}

const (
	BRSlaveGrpFwdMask     = 65533
	maxVxLANEncapOverhead = 74
)

func ensureVXLANIf(name string, devFD, netns int, vni int, grp netip.Addr, mtu uint32, port int) error {
	// log.Printf("ensure vxlanif, %v, %v, %v, %v, %v", name, egressifname, vni, grp, mtu)
	// var err error
	if !grp.IsMulticast() {
		return fmt.Errorf("%s is not a multicast address", grp)
	}

	newif := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        name,
			MTU:         int(mtu),
			TxQLen:      1024,
			NumTxQueues: 1,
			NumRxQueues: 1,
			// Namespace:   netlink.NsFd(int(mnetns.NsHandle(netns))),
			Namespace: netlink.NsFd(netns),
		},
		VxlanId:      vni,
		VtepDevIndex: devFD,
		Group:        grp.AsSlice(),
		Learning:     true,  //learn MAC address dynamically from data packet
		Proxy:        false, //arp proxy
		Age:          3600,  //leaned MAC lifetime, in seconds
		Port:         port,  //IANA value, not the linux default
	}
	//remove exisitng interface first
	// err = removeLinkByName(name)
	// err = util.LinkDelete(name)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to remove old vxlan if %v, %w", name, err)
	// }
	return netlink.LinkAdd(newif)
	// var existLink netlink.Link
	// sameVXLink := func(link1, link2 *netlink.Vxlan) bool {
	// 	if link1.MTU != link2.MTU {
	// 		return false
	// 	}
	// 	if link1.VxlanId != link2.VxlanId {
	// 		return false
	// 	}
	// 	if link1.VtepDevIndex != link2.VtepDevIndex {
	// 		return false
	// 	}
	// 	if !link1.Group.Equal(link2.Group) {
	// 		return false
	// 	}

	// 	return true

	// }
	// if err != nil {
	// 	if err != syscall.EEXIST {
	// 		return fmt.Errorf("failed to create vxlan if %v, %w", name, err)
	// 	} else {
	// 		//already exist, remove it if it doesn't have parent, or doesn't match the expected spec

	// 		log.Printf("%v already exists", name)
	// 		existLink, err = netlink.LinkByName(name)
	// 		if err == nil {
	// 			rebuild := false
	// 			if existLink.(*netlink.Vxlan).LinkAttrs.MasterIndex == 0 || !(sameVXLink(existLink.(*netlink.Vxlan), newif)) {
	// 				rebuild = true
	// 			}

	// 			if rebuild {
	// 				log.Printf("delete existing parentless interface %v", name)
	// 				err = netlink.LinkDel(existLink)
	// 				if err == nil {
	// 					//add new link again
	// 					err = netlink.LinkAdd(newif)
	// 					log.Printf("recreate interaface %v", name)
	// 				}
	// 			}

	// 		}

	// 	}
	// }
	// if err == nil {
	// 	err = netlink.LinkSetUp(newif)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to bring up %v, %w", name, err)
	// 	}
	// } else {
	// 	return fmt.Errorf("failed to recreate vxlan if %v, %w", name, err)
	// }
	// rif, err := netlink.LinkByName(name)
	// if err != nil {
	// 	return fmt.Errorf("failed get newly created if %v, %w", name, err)
	// }
	// return nil
}

func CreateVXLANIF(lan *v1beta1.LANSpec, devFD, netns int, mtu, port int) error {
	grpAddr := netip.MustParseAddr(*lan.VxLANGrp)
	//create vxlan
	err := ensureVXLANIf(*lan.VxLANName,
		devFD, netns, int(*lan.VNI),
		grpAddr, uint32(mtu), port)
	if err != nil {
		if !errors.Is(err, syscall.EEXIST) {
			return err
		}
	}
	return nil

}
