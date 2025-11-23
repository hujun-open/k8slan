package main

import (
	"fmt"
	"log"
	"time"

	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/hujun-open/k8slan/pkg/interfaces"
)

func main() {
	lanspec := &v1beta1.LANSpec{
		NS:           new(string),
		BridgeName:   new(string),
		VxLANName:    new(string),
		VNI:          new(int32),
		VxLANGrp:     new(string),
		DefaultVxDev: "eth0.10",
		VxPort:       new(int32),
		SpokeList:    []string{"spoke1"},
	}
	*lanspec.NS = "ns1"
	*lanspec.BridgeName = "br1"
	*lanspec.VxLANName = "vxlan1"
	*lanspec.VNI = 999
	*lanspec.VxLANGrp = "ff02::14"
	*lanspec.VxPort = 4789
	log.Printf("del existing ns...")
	interfaces.Remove(*lanspec.NS)
	// err := netns.DeleteNamed(*lanspec.NS)
	// if err != nil {
	// 	log.Printf("failed to del ns, %v", err)
	// }
	time.Sleep(time.Second)
	_, err := interfaces.Ensure("macvtap1", "spoke1", lanspec, "hjlaptop", "passthru")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("done")

}
