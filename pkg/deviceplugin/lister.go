package deviceplugin

import (
	"fmt"
	"os"
	"sync"

	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	ctrl "sigs.k8s.io/controller-runtime"
)

type macvtapLister struct {
	DeviceList         map[string]*v1beta1.LAN //key is the res name in the LAN
	deviceListLock     *sync.RWMutex
	AddChan            chan v1beta1.AddRequest
	RemovChan          chan *v1beta1.LAN
	ExistingNSList     []string
	existingNSListLock *sync.RWMutex
}

func (ml *macvtapLister) getCurrentPlugins() dpm.PluginNameList {
	r := make(dpm.PluginNameList, 0)
	ml.deviceListLock.RLock()
	defer ml.deviceListLock.RUnlock()
	for name := range ml.DeviceList {
		r = append(r, name)
	}
	return r
}

func NewMacvtapLister(netNsPath string, add chan v1beta1.AddRequest, remove chan *v1beta1.LAN) *macvtapLister {
	return &macvtapLister{
		AddChan:            add,
		RemovChan:          remove,
		DeviceList:         make(map[string]*v1beta1.LAN),
		deviceListLock:     new(sync.RWMutex),
		existingNSListLock: new(sync.RWMutex),
	}
}

func (ml macvtapLister) GetResourceNamespace() string {
	return v1beta1.ResourceNamespace
}
func (ml *macvtapLister) report(pluginListCh chan dpm.PluginNameList) {
	curList := ml.getCurrentPlugins()
	if len(curList) > 0 {
		pluginListCh <- curList
	}
}

func (ml *macvtapLister) Discover(pluginListCh chan dpm.PluginNameList) {
	log := ctrl.Log.WithName("Discover")
	for {
		select {
		case req := <-ml.AddChan:

			lan := req.NewLan
			log.Info("got a new lan", "name", lan.Name)
			ml.existingNSListLock.Lock()
			ml.ExistingNSList = req.ExistingNSNames
			ml.existingNSListLock.Unlock()
			ml.deviceListLock.Lock()
			for _, spokeName := range lan.Spec.SpokeList {
				ml.DeviceList[v1beta1.GetDPResouceName(lan.Name, spokeName, true)] = lan
				ml.DeviceList[v1beta1.GetDPResouceName(lan.Name, spokeName, false)] = lan
			}
			ml.deviceListLock.Unlock()
			ml.report(pluginListCh)
			log.Info("report the new lan", "name", lan.Name)

		case lan := <-ml.RemovChan:
			log.Info("got a  lan to remove", "name", lan.Name)
			ml.deviceListLock.Lock()
			for _, spokeName := range lan.Spec.SpokeList {
				delete(ml.DeviceList, v1beta1.GetDPResouceName(lan.Name, spokeName, true))
				delete(ml.DeviceList, v1beta1.GetDPResouceName(lan.Name, spokeName, false))
			}
			ml.deviceListLock.Unlock()
			ml.report(pluginListCh)
			log.Info("report new dev list after removal", "name", lan.Name)

		}
	}
}

// name here is the "dataplane" of "k8s.v1.cni.cncf.io/resourceName: macvtap.network.kubevirt.io/dataplane"
// also vlanName in k8slan case
func (ml *macvtapLister) NewPlugin(name string) dpm.PluginInterface {
	log := ctrl.Log.WithName("deviceplugin")
	ml.deviceListLock.RLock()
	lan, ok := ml.DeviceList[name]
	ml.deviceListLock.RUnlock()
	if !ok {
		return nil
	}

	log.Info("Creating device plugin", "name", name, "config", lan)
	return NewMacvtapDevicePlugin(name, lan, ml)
}

// GetMainThreadNetNsPath returns the path of the main thread's namespace
func GetMainThreadNetNsPath() string {
	return fmt.Sprintf("/proc/%d/ns/net", os.Getpid())
}
