package metal

import (
	"context"
	"fmt"
	"io"

	"github.com/packethost/packngo"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
)

const (
	ProviderName string = "equinixmetal"

	// deprecatedProviderName is used to provide backward compatibility support
	// with previous versions
	deprecatedProviderName string = "packet"

	// ConsumerToken token for metal consumer
	ConsumerToken         string = "cloud-provider-equinix-metal"
	checkLoopTimerSeconds        = 60
)

type nodeReconciler func(ctx context.Context, nodes []*v1.Node, mode UpdateMode) error
type serviceReconciler func(ctx context.Context, services []*v1.Service, mode UpdateMode) error

// cloudService an internal service that can be initialize and report a name
type cloudService interface {
	name() string
	init(k8sclient kubernetes.Interface) error
	nodeReconciler() nodeReconciler
	serviceReconciler() serviceReconciler
}

type cloudInstances interface {
	cloudprovider.InstancesV2
	cloudService
}
type cloudLoadBalancers interface {
	cloudprovider.LoadBalancer
	cloudService
}
type cloudZones interface {
	cloudprovider.Zones
	cloudService
}

// cloud implements cloudprovider.Interface
type cloud struct {
	client                      *packngo.Client
	instances                   cloudInstances
	zones                       cloudZones
	loadBalancer                cloudLoadBalancers
	metro                       string
	facility                    string
	controlPlaneEndpointManager *controlPlaneEndpointManager
	// holds our bgp service handler
	bgp *bgp
}

var _ cloudprovider.Interface = (*cloud)(nil)

func newCloud(metalConfig Config, client *packngo.Client) (cloudprovider.Interface, error) {
	i := newInstances(client, metalConfig.ProjectID, metalConfig.AnnotationNetworkIPv4Private)
	return &cloud{
		client:                      client,
		metro:                       metalConfig.Metro,
		facility:                    metalConfig.Facility,
		instances:                   i,
		zones:                       newZones(client, metalConfig.ProjectID),
		loadBalancer:                newLoadBalancers(client, metalConfig.ProjectID, metalConfig.Metro, metalConfig.Facility, metalConfig.LoadBalancerSetting),
		bgp:                         newBGP(client, metalConfig.ProjectID, metalConfig.LocalASN, metalConfig.BGPPass, metalConfig.AnnotationLocalASN, metalConfig.AnnotationPeerASN, metalConfig.AnnotationPeerIP, metalConfig.AnnotationSrcIP, metalConfig.AnnotationBGPPass, metalConfig.BGPNodeSelector),
		controlPlaneEndpointManager: newControlPlaneEndpointManager(metalConfig.EIPTag, metalConfig.ProjectID, client.DeviceIPs, client.ProjectIPs, i, metalConfig.APIServerPort),
	}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		// by the time we get here, there is no error, as it would have been handled earlier
		metalConfig, err := getMetalConfig(config)
		// register the provider
		if err != nil {
			return nil, fmt.Errorf("provider config error: %v", err)
		}

		// report the config
		printMetalConfig(metalConfig)

		// set up our client and create the cloud interface
		client := packngo.NewClientWithAuth("cloud-provider-equinix-metal", metalConfig.AuthToken, nil)
		client.UserAgent = fmt.Sprintf("cloud-provider-equinix-metal/%s %s", version.Get(), client.UserAgent)
		cloud, err := newCloud(metalConfig, client)
		if err != nil {
			return nil, fmt.Errorf("failed to create new cloud handler: %v", err)
		}

		return cloud, nil
	})
}

// services get those elements that are initializable
func (c *cloud) services() []cloudService {
	return []cloudService{c.loadBalancer, c.instances, c.zones, c.bgp, c.controlPlaneEndpointManager}
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.V(5).Info("called Initialize")
	clientset := clientBuilder.ClientOrDie("cloud-provider-equinix-metal-shared-informers")
	sharedInformer := informers.NewSharedInformerFactory(clientset, 0)
	// if we have services that want to reconcile, we will start node loop
	services := c.services()
	for _, elm := range services {
		if err := elm.init(clientset); err != nil {
			klog.Fatalf("could not initialize %s: %v", elm.name(), err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stop
		cancel()
	}()

	var watchers []cache.SharedIndexInformer
	nodesWatcher, err := createNodesWatcher(ctx, sharedInformer, services)
	if err != nil {
		klog.Fatalf("nodes watcher initialization failed: %v", err)
	}
	if nodesWatcher != nil {
		watchers = append(watchers, nodesWatcher)
	}
	servicesWatcher, err := createServicesWatcher(ctx, sharedInformer, services)
	if err != nil {
		klog.Fatalf("services watcher initialization failed: %v", err)
	}
	if servicesWatcher != nil {
		watchers = append(watchers, servicesWatcher)
	}
	if err := startWatchers(ctx, watchers); err != nil {
		klog.Fatalf("watchers initialization failed: %v", err)
	}
	go timerLoop(ctx, sharedInformer, services)
	klog.Info("Initialize of cloud provider complete")
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
// TODO unimplemented
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	klog.V(5).Info("called LoadBalancer")
	return nil, false
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	klog.V(5).Info("called Instances")
	return nil, false
}

// InstancesV2 returns an implementation of cloudprovider.InstancesV2.
func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	klog.V(5).Info("called InstancesV2")
	return c.instances, true
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	klog.V(5).Info("called Zones")
	return c.zones, true
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	klog.V(5).Info("called Clusters")
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	klog.V(5).Info("called Routes")
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *cloud) ProviderName() string {
	klog.V(2).Infof("called ProviderName, returning %s", ProviderName)
	return ProviderName
}

// HasClusterID returns true if a ClusterID is required and set
func (c *cloud) HasClusterID() bool {
	klog.V(5).Info("called HasClusterID")
	return true
}
