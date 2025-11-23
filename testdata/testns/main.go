package main

import (
	"flag"
	"fmt"
	"log"
	"os/exec"

	"github.com/containernetworking/plugins/pkg/ns"
)

func main() {
	nspath := flag.String("n", "", "nspath")
	cmd := flag.String("c", "", "cmd")
	flag.Parse()
	netns, err := ns.GetNS(*nspath)
	if err != nil {
		log.Fatalf("failed to open nspath %v, %v", *nspath, err)
	}
	err = netns.Do(func(_ ns.NetNS) error {
		cmd := exec.Command("sh", "-c", *cmd)
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", stdoutStderr)
		return nil
	})
	if err != nil {
		log.Fatalf("failed to exec command %v, %v", *cmd, err)
	}

}
