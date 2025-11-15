package deviceplugin

import (
	"fmt"
	"os"

	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	resourceNamespace = "macvtap.k8slan.io"
)

type macvtapLister struct {
	DeviceList map[string]*v1beta1.LANSpec //key is the vlan name in the LAN
	// lock   *sync.RWMutex
	// NetNsPath is the path to the network namespace the lister operates in.
	AddChan   chan *v1beta1.LANSpec
	RemovChan chan *v1beta1.LANSpec
}

func (ml *macvtapLister) getCurrentPlugins() dpm.PluginNameList {
	r := make(dpm.PluginNameList, 0)
	for name := range ml.DeviceList {
		r = append(r, name)
	}
	return r
}

func NewMacvtapLister(netNsPath string, add, remove chan *v1beta1.LANSpec) *macvtapLister {
	return &macvtapLister{
		AddChan:    add,
		RemovChan:  remove,
		DeviceList: make(map[string]*v1beta1.LANSpec),
	}
}

func (ml macvtapLister) GetResourceNamespace() string {
	return resourceNamespace
}
func (ml *macvtapLister) report(pluginListCh chan dpm.PluginNameList) {
	curList := ml.getCurrentPlugins()
	if len(curList) > 0 {
		pluginListCh <- curList
	}
}

func (ml *macvtapLister) Discover(pluginListCh chan dpm.PluginNameList) {
	for {
		select {
		case lan := <-ml.AddChan:
			for _, vlanName := range lan.VlanNameList {
				ml.DeviceList[vlanName] = lan
			}
			ml.report(pluginListCh)

		case lan := <-ml.RemovChan:
			for _, vlanName := range lan.VlanNameList {
				delete(ml.DeviceList, vlanName)
			}
			ml.report(pluginListCh)

		}
	}
}

// name here is the "dataplane" of "k8s.v1.cni.cncf.io/resourceName: macvtap.network.kubevirt.io/dataplane"
// also vlanName in k8slan case
func (ml *macvtapLister) NewPlugin(name string) dpm.PluginInterface {
	log := ctrl.Log.WithName("deviceplugin")
	lan, ok := ml.DeviceList[name]
	if !ok {
		return nil
	}

	log.Info("Creating device plugin", "name", name, "config", lan)
	return NewMacvtapDevicePlugin(name, lan)
}

// GetMainThreadNetNsPath returns the path of the main thread's namespace
func GetMainThreadNetNsPath() string {
	return fmt.Sprintf("/proc/%d/ns/net", os.Getpid())
}
