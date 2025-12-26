package deviceplugin

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/hujun-open/k8slan/api/v1beta1"
	"github.com/hujun-open/k8slan/pkg/interfaces"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	tapPath = "/dev/tap"
	// Interfaces will be named as <Name><suffix>[0-<Capacity>]
	macvtapSuffix = "M"
	vethSuffix    = "V"
	// DefaultCapacity is the default when no capacity is provided
	// DefaultCapacity = 1
	// DefaultMode is the default when no mode is provided
	DefaultMode = "passthru"
)

type macvtapDevicePlugin struct {
	Name         string //spoke name
	hostName     string
	lan          *v1beta1.LAN
	Capacity     int
	Mode         string
	stopWatcher  chan struct{}
	dummyMACVTAP bool
	lister       *macvtapLister
	pluginapi.UnimplementedDevicePluginServer
}

func NewMacvtapDevicePlugin(name string, lan *v1beta1.LAN, lister *macvtapLister) *macvtapDevicePlugin {
	hname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return &macvtapDevicePlugin{
		Name:         v1beta1.GetSpokeNameFromResourceName(name, lan.Name),
		Mode:         DefaultMode,
		lan:          lan,
		stopWatcher:  make(chan struct{}),
		hostName:     hname,
		lister:       lister,
		dummyMACVTAP: strings.HasPrefix(name, v1beta1.VETHPreffix),
	}
}

func (mdp *macvtapDevicePlugin) generateMacvtapDevices() []*pluginapi.Device {
	var macvtapDevs []*pluginapi.Device

	var capacity = 1
	suffix := macvtapSuffix
	if mdp.dummyMACVTAP {
		suffix = vethSuffix
	}
	for i := 0; i < capacity; i++ {
		name := fmt.Sprint(mdp.Name, suffix, i)
		macvtapDevs = append(macvtapDevs, &pluginapi.Device{
			ID:     name,
			Health: pluginapi.Healthy,
		})
	}

	return macvtapDevs
}

func (mdp *macvtapDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	//always advertise devices regardless if they exist
	log := ctrl.Log.WithName("deviceplugin")
	allocatableDevs := mdp.generateMacvtapDevices()
	log.Info("LowerDevice exists, sending ListAndWatch response with available devices", "name", mdp.Name)
	for {
		s.Send(&pluginapi.ListAndWatchResponse{Devices: allocatableDevs})
		time.Sleep(time.Second)
	}
}

func (mdp *macvtapDevicePlugin) clearUnused() error {
	mdp.lister.ExistingNSListLock.RLock()
	crNSList := slices.Clone(mdp.lister.ExistingNSList)
	mdp.lister.ExistingNSListLock.RUnlock()
	existList, err := interfaces.GetExistingNSPaths()
	if err != nil {
		return fmt.Errorf("failed to list existing NS path, %w", err)
	}
	for _, existOne := range existList {
		if !slices.Contains(crNSList, existOne) {
			err = interfaces.DeleteNamed(existOne)
			if err != nil {
				return fmt.Errorf("failed to create stale ns %v, %w", existOne, err)
			}
		}
	}
	return nil
}

func (mdp *macvtapDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse
	//clear stale ns
	err := mdp.clearUnused()
	if err != nil {
		return nil, err
	}
	for _, req := range r.ContainerRequests {
		var devices []*pluginapi.DeviceSpec
		for _, devID := range req.DevicesIds {

			dev := new(pluginapi.DeviceSpec)

			// There is a possibility the interface already exists from a
			// previous allocation. In a typical scenario, macvtap interfaces
			// would be deleted by the CNI when healthy pod sandbox is
			// terminated. But on occasions, sandbox allocations may fail and
			// the interface is left lingering. The device plugin framework has
			// no de-allocate flow to clean up. So we attempt to delete a
			// possibly existing existing interface before creating it to reset
			// its state.
			var index int
			var err error
			// index, err = util.RecreateMacvtap(name, mdp.LowerDevice, mdp.Mode)
			index, err = interfaces.Ensure(devID, mdp.Name, &mdp.lan.Spec, mdp.hostName, mdp.Mode, mdp.dummyMACVTAP)
			if err != nil {
				return nil, err
			}

			devPath := fmt.Sprint(tapPath, index)
			dev.HostPath = devPath
			dev.ContainerPath = devPath
			dev.Permissions = "rw"
			devices = append(devices, dev)
		}

		response.ContainerResponses = append(response.ContainerResponses, &pluginapi.ContainerAllocateResponse{
			Devices: devices,
		})
	}

	return &response, nil
}

func (mdp *macvtapDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return nil, nil
}

func (mdp *macvtapDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (mdp *macvtapDevicePlugin) GetPreferredAllocation(context.Context, *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return nil, nil
}

func (mdp *macvtapDevicePlugin) Stop() error {
	close(mdp.stopWatcher)
	return nil
}
