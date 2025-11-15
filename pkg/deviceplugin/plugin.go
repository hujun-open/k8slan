package deviceplugin

import (
	"fmt"
	"os"
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
	suffix = "Mvp" // if lower device does not exist.
	// DefaultCapacity is the default when no capacity is provided
	DefaultCapacity = 1
	// DefaultMode is the default when no mode is provided
	DefaultMode = "passthru"
)

type macvtapDevicePlugin struct {
	Name        string
	hostName    string
	lan         *v1beta1.LANSpec
	Capacity    int
	Mode        string
	stopWatcher chan struct{}
	pluginapi.UnimplementedDevicePluginServer
}

func NewMacvtapDevicePlugin(name string, lan *v1beta1.LANSpec) *macvtapDevicePlugin {
	hname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return &macvtapDevicePlugin{
		Name:        name,
		Mode:        DefaultMode,
		Capacity:    DefaultCapacity,
		lan:         lan,
		stopWatcher: make(chan struct{}),
		hostName:    hname,
	}
}

func (mdp *macvtapDevicePlugin) generateMacvtapDevices() []*pluginapi.Device {
	var macvtapDevs []*pluginapi.Device

	var capacity = mdp.Capacity
	if capacity <= 0 {
		capacity = DefaultCapacity
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

func (mdp *macvtapDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse
	for _, req := range r.ContainerRequests {
		var devices []*pluginapi.DeviceSpec
		for _, macVtapName := range req.DevicesIds {

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
			index, err = interfaces.Ensure(macVtapName, mdp.Name, mdp.lan, mdp.hostName, mdp.Mode)
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
