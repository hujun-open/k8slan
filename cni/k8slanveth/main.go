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
	"errors"
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/vishvananda/netlink"
)

// PluginConf is whatever you expect your configuration json to be. This is whatever
// is passed in on stdin. Your plugin may wish to expose its functionality via
// runtime args, see CONVENTIONS.md in the CNI spec.
type PluginConf struct {
	// This embeds the standard NetConf structure which allows your plugin
	// to more easily parse standard fields like Name, Type, CNIVersion,
	// and PrevResult.
	types.NetConf
	VethName  string `json:"veth"`
	EnableDad bool   `json:"enableDad"`
}

// MacEnvArgs represents CNI_ARGS
type MacEnvArgs struct {
	types.CommonArgs
	MAC types.UnmarshallableString `json:"mac,omitempty"`
}

type BridgeArgs struct {
	Mac string `json:"mac,omitempty"`
}

// parseConfig parses the supplied configuration (and prevResult) from stdin.
func parseConfig(stdin []byte, envArgs string) (*PluginConf, error) {
	conf := PluginConf{}

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

	return &conf, nil
}

// cmdAdd is called for ADD requests
func cmdAdd(args *skel.CmdArgs) error {
	success := false
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

	//IPAM
	if conf.IPAM.Type != "" {
		//assign IP
		// run the IPAM plugin and get back the config to apply
		r, err := ipam.ExecAdd(conf.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}

		// release IP in case of failure
		defer func() {
			if !success {
				ipam.ExecDel(conf.IPAM.Type, args.StdinData)
			}
		}()

		// Convert whatever the IPAM result was into the current Result type
		ipamResult, err := current.NewResultFromResult(r)
		if err != nil {
			return err
		}

		result.IPs = ipamResult.IPs
		result.Routes = ipamResult.Routes
		result.DNS = ipamResult.DNS

		if len(result.IPs) == 0 {
			return errors.New("IPAM plugin returned missing IP config")
		}

		// Configure the container hardware address and IP address(es)
		if err := podNS.Do(func(_ ns.NetNS) error {
			if conf.EnableDad {
				_, _ = sysctl.Sysctl(fmt.Sprintf("/net/ipv6/conf/%s/enhanced_dad", args.IfName), "1")
				_, _ = sysctl.Sysctl(fmt.Sprintf("net/ipv6/conf/%s/accept_dad", args.IfName), "1")
			} else {
				_, _ = sysctl.Sysctl(fmt.Sprintf("net/ipv6/conf/%s/accept_dad", args.IfName), "0")
			}
			_, _ = sysctl.Sysctl(fmt.Sprintf("net/ipv4/conf/%s/arp_notify", args.IfName), "1")

			// Add the IP to the interface
			return ipam.ConfigureIface(args.IfName, result)
		}); err != nil {
			return err
		}
	}
	// Use incoming DNS settings if provided, otherwise use the
	// settings that were already configured by the IPAM plugin
	if dnsConfSet(conf.DNS) {
		result.DNS = conf.DNS
	}

	success = true
	return types.PrintResult(result, conf.CNIVersion)

}

func dnsConfSet(dnsConf types.DNS) bool {
	return dnsConf.Nameservers != nil ||
		dnsConf.Search != nil ||
		dnsConf.Options != nil ||
		dnsConf.Domain != ""
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
