package interfaces

func Remove(nsname string) {
	DeleteNamed(nsname)
	// nsPath := filepath.Join(getNsRunDir(), *lan.Spec.NS)

	// //exists
	// lanNS, err := ns.GetNS(nsPath)
	// if err != nil {
	// 	//can't open the ns, return
	// 	return
	// }
	// lanNS.Do(func(hostNs ns.NetNS) error {
	// 	//veth
	// 	for _, spoke := range lan.Spec.SpokeList {
	// 		vethLink, err := netlink.LinkByName(getPeerVethName(spoke))
	// 		if err == nil {
	// 			netlink.LinkDel(vethLink)
	// 		}
	// 	}
	// 	//vxlan
	// 	vxlink, err := netlink.LinkByName(*lan.Spec.VxLANName)
	// 	if err == nil {
	// 		netlink.LinkDel(vxlink)
	// 	}

	// 	//bridge
	// 	brlink, err := netlink.LinkByName(*lan.Spec.BridgeName)
	// 	if err == nil {
	// 		netlink.LinkDel(brlink)
	// 	}
	// 	return nil
	// })

}
