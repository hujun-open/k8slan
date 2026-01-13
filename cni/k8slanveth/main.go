//This plugin move the specified veth interface (which already exists) into pod namespace

// Copyright 2017 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This is a sample chained plugin that supports multiple CNI versions. It
// parses prevResult according to the cniVersion
package main

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/safchain/ethtool"
	"github.com/vishvananda/netlink"
)

// PluginConf is whatever you expect your configuration json to be. This is whatever
// is passed in on stdin. Your plugin may wish to expose its functionality via
// runtime args, see CONVENTIONS.md in the CNI spec.
// type PluginConf struct {
// 	// This embeds the standard NetConf structure which allows your plugin
// 	// to more easily parse standard fields like Name, Type, CNIVersion,
// 	// and PrevResult.
// 	types.NetConf
// 	VethName      string          `json:"veth"`
// 	Addresses     []netip.Prefix  `json:"addrs,omitempty"`
// 	Routes        []v1beta1.Route `json:"routes,omitempty"`
// 	EnableDad     bool            `json:"enableDad"`
// 	TxChecksumOff bool            `json:"disableTxChecksum"`
// 	PeerNS        string          `json:"peerNS"`
// }

// MacEnvArgs represents CNI_ARGS
type MacEnvArgs struct {
	types.CommonArgs
	MAC types.UnmarshallableString `json:"mac,omitempty"`
}

type BridgeArgs struct {
	Mac string `json:"mac,omitempty"`
}

// parseConfig parses the supplied configuration (and prevResult) from stdin.
func parseConfig(stdin []byte, envArgs string) (*v1beta1.K8sLANVethPluginConf, error) {
	conf := v1beta1.K8sLANVethPluginConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	// Parse previous result. This will parse, validate, and place the
	// previous result object into conf.PrevResult. If you need to modify
	// or inspect the PrevResult you will need to convert it to a concrete
	// versioned Result struct.
	if err := version.ParsePrevResult(&conf.NetConf); err != nil {
		return nil, fmt.Errorf("could not parse prevResult: %v", err)
	}
	// End previous result parsing
	for i, addr := range conf.Addresses {
		if !addr.IsValid() {
			return nil, fmt.Errorf("#%d is invalid ip prefix", i+1)
		}
		if _, err := netlink.ParseAddr(addr.String()); err != nil {
			return nil, fmt.Errorf("#%d is not a valid ip prefix", i+1)
		}
	}
	if len(conf.Routes) > 0 && len(conf.Addresses) == 0 {
		return nil, fmt.Errorf("routes can't be specified without interface address")
	}
	for i, r := range conf.Routes {
		if !r.To.IsValid() {
			return nil, fmt.Errorf("route #%d has invalid dest", i+1)
		}
		if !r.Via.IsValid() {
			return nil, fmt.Errorf("route #%d has invalid nexthop", i+1)
		}
		if r.Via.Is4() != r.To.Addr().Is4() {
			return nil, fmt.Errorf("route #%d has inconsistent address family", i+1)
		}
	}
	return &conf, nil
}

func turnOffIPTxChecksum(ifname string) error {
	eth, err := ethtool.NewEthtool()
	if err != nil {
		return fmt.Errorf("Failed to initialize ethtool: %w", err)
	}
	defer eth.Close()
	targetFeature := "tx-checksum-ip-generic"
	changeRequest := map[string]bool{
		targetFeature: false,
	}
	return eth.Change(ifname, changeRequest)
}

// cmdAdd is called for ADD requests
func cmdAdd(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}
	podNS, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open pod netns %q: %v", args.Netns, err)
	}
	//locate the veth
	vlink, err := netlink.LinkByName(conf.VethName)
	if err != nil {
		return fmt.Errorf("failed to locate veth interface %v, %w", conf.VethName, err)
	}
	//turn of TX IP checksum on the peer interface, this is needed for SRSIM
	if conf.TxChecksumOff {
		peerNS, err := ns.GetNS(conf.PeerNS)
		if err != nil {
			return fmt.Errorf("failed to get peer NS at %v, %w", conf.PeerNS, err)
		}
		err = peerNS.Do(func(_ ns.NetNS) error {
			peerLink, err := netlink.LinkByIndex(vlink.Attrs().ParentIndex)
			if err != nil {
				return fmt.Errorf("failed to locate peer link in the peer ns, %w", err)
			}
			err = turnOffIPTxChecksum(peerLink.Attrs().Name)
			if err != nil {
				return fmt.Errorf("failed to turn of tx-checksum-ip-generic on interface %v, %w", peerLink.Attrs().Name, err)
			}
			return nil
		})
		if err != nil {
			return err
		}

	}
	//move interface to pod NS
	err = netlink.LinkSetNsFd(vlink, int(podNS.Fd()))
	if err != nil {
		return fmt.Errorf("failed to move veth interface %v into pod NS, %w", conf.VethName, err)
	}
	//rename it
	err = podNS.Do(func(_ ns.NetNS) error {
		err := netlink.LinkSetName(vlink, args.IfName)
		if err != nil {
			return err
		}
		vlink, err = netlink.LinkByName(args.IfName)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to rename veth interface from %v -> %v, %w", conf.VethName, args.IfName, err)
	}

	//assign address

	err = podNS.Do(func(_ ns.NetNS) error {
		vlink, err = netlink.LinkByName(args.IfName)
		if err != nil {
			return err
		}
		for _, addr := range conf.Addresses {
			naddr, _ := netlink.ParseAddr(addr.String())
			err = netlink.AddrReplace(vlink, naddr)
			if err != nil {
				return fmt.Errorf("failed to assign addr %v, %w", addr.String(), err)
			}
		}
		//bring up the interface
		return netlink.LinkSetUp(vlink)

	})

	if err != nil {
		return err
	}
	//add routes

	err = podNS.Do(func(_ ns.NetNS) error {
		vlink, err = netlink.LinkByName(args.IfName)
		if err != nil {
			return err
		}
		for _, route := range conf.Routes {
			nRoute := &netlink.Route{
				LinkIndex: vlink.Attrs().Index,
				Dst: &net.IPNet{
					IP:   route.To.Addr().AsSlice(),
					Mask: net.CIDRMask(route.To.Bits(), route.To.Addr().BitLen()),
				},
				Gw: route.Via.AsSlice(),
			}
			err = netlink.RouteAdd(nRoute)
			if err != nil {
				return fmt.Errorf("failed to add route %v, %w", nRoute.String(), err)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}
	podIface := &current.Interface{}
	podIface.Name = vlink.Attrs().Name
	podIface.Mac = vlink.Attrs().HardwareAddr.String()
	podIface.Sandbox = podNS.Path()
	result := &current.Result{
		CNIVersion: current.ImplementedSpecVersion,
		Interfaces: []*current.Interface{
			podIface,
		},
	}

	return types.PrintResult(result, conf.CNIVersion)
}

// cmdDel is called for DELETE requests
func cmdDel(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}
	_ = conf

	// Do your delete here

	return nil
}

func main() {
	// replace TODO with your plugin name
	skel.PluginMainFuncs(skel.CNIFuncs{
		Add:    cmdAdd,
		Check:  cmdCheck,
		Del:    cmdDel,
		Status: cmdStatus,
		/* FIXME GC */
	}, version.All, bv.BuildString("TODO"))
}

func cmdCheck(_ *skel.CmdArgs) error {
	// TODO: implement
	return fmt.Errorf("not implemented")
}

// cmdStatus implements the STATUS command, which indicates whether or not
// this plugin is able to accept ADD requests.
//
// If the plugin has external dependencies, such as a daemon
// or chained ipam plugin, it should determine their status. If all is well,
// and an ADD can be successfully processed, return nil
func cmdStatus(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}
	_ = conf

	// If this plugins delegates IPAM, ensure that IPAM is also running
	if err := ipam.ExecStatus(conf.IPAM.Type, args.StdinData); err != nil {
		return err
	}

	// TODO: implement STATUS here
	// e.g. querying an external deamon, or delegating STATUS to an IPAM plugin

	return nil
}
