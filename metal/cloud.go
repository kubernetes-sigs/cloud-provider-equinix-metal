package metal

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/packethost/packngo"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	providerName string = "equinixmetal"

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
	cloudprovider.Instances
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
	facility                    string
	controlPlaneEndpointManager *controlPlaneEndpointManager
	// holds our bgp service handler
	bgp *bgp
}

func newCloud(metalConfig Config, client *packngo.Client) (cloudprovider.Interface, error) {
	i := newInstances(client, metalConfig.ProjectID, metalConfig.AnnotationNetworkIPv4Private)
	return &cloud{
		client:                      client,
		facility:                    metalConfig.Facility,
		instances:                   i,
		zones:                       newZones(client, metalConfig.ProjectID),
		loadBalancer:                newLoadBalancers(client, metalConfig.ProjectID, metalConfig.Facility, metalConfig.LoadBalancerSetting),
		bgp:                         newBGP(client, metalConfig.ProjectID, metalConfig.LocalASN, metalConfig.BGPPass, metalConfig.AnnotationLocalASN, metalConfig.AnnotationPeerASNs, metalConfig.AnnotationPeerIPs, metalConfig.AnnotationSrcIP, metalConfig.AnnotationBGPPass, metalConfig.BGPNodeSelector),
		controlPlaneEndpointManager: newControlPlaneEndpointManager(metalConfig.EIPTag, metalConfig.ProjectID, client.DeviceIPs, client.ProjectIPs, i, metalConfig.APIServerPort),
	}, nil
}

func InitializeProvider(metalConfig Config) error {
	// set up our client and create the cloud interface
	client := packngo.NewClientWithAuth("", metalConfig.AuthToken, nil)
	client.UserAgent = fmt.Sprintf("cloud-provider-equinix-metal/%s %s", VERSION, client.UserAgent)
	cloud, err := newCloud(metalConfig, client)
	if err != nil {
		return fmt.Errorf("failed to create new cloud handler: %v", err)
	}

	// finally, register
	cloudprovider.RegisterCloudProvider(providerName, func(config io.Reader) (cloudprovider.Interface, error) {
		// by the time we get here, there is no error, as it would have been handled earlier
		return cloud, nil
	})

	return nil
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
	nodeReconcilers := []nodeReconciler{}
	serviceReconcilers := []serviceReconciler{}
	for _, elm := range c.services() {
		if err := elm.init(clientset); err != nil {
			klog.Fatalf("could not initialize %s: %v", elm.name(), err)
		}
		if n := elm.nodeReconciler(); n != nil {
			nodeReconcilers = append(nodeReconcilers, n)
		}
		if s := elm.serviceReconciler(); s != nil {
			serviceReconcilers = append(serviceReconcilers, s)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stop
		cancel()
	}()

	if err := startNodesWatcher(ctx, sharedInformer, nodeReconcilers); err != nil {
		klog.Errorf("nodes watcher initialization failed: %v", err)
	}
	if err := startServicesWatcher(ctx, sharedInformer, serviceReconcilers); err != nil {
		klog.Errorf("services watcher initialization failed: %v", err)
	}
	go timerLoop(ctx, sharedInformer, nodeReconcilers, serviceReconcilers)
	klog.V(5).Info("Initialize complete")
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
	return c.instances, true
}

// InstancesV2 returns an implementation of cloudprovider.InstancesV2.
func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	klog.Warning("The Equinix Metal cloud provider does not support InstancesV2")
	return nil, false
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
	klog.V(2).Infof("called ProviderName, returning %s", providerName)
	return providerName
}

// HasClusterID returns true if a ClusterID is required and set
func (c *cloud) HasClusterID() bool {
	klog.V(5).Info("called HasClusterID")
	return true
}

// startNodesWatcher start a goroutine that watches k8s for nodes and calls any handlers
func startNodesWatcher(ctx context.Context, informer informers.SharedInformerFactory, handlers []nodeReconciler) error {
	klog.V(5).Info("called startNodesWatcher")
	if len(handlers) == 0 {
		klog.V(5).Info("no node handlers to process")
		return nil
	}

	klog.V(5).Info("startNodesWatcher(): creating nodesInformer")
	nodesInformer := informer.Core().V1().Nodes().Informer()
	klog.V(5).Info("startNodesWatcher(): adding event handlers")
	nodesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			n := obj.(*v1.Node)
			for _, h := range handlers {
				if err := h(ctx, []*v1.Node{n}, ModeAdd); err != nil {
					klog.Errorf("failed to update and sync node for add %s for handler: %v", n.Name, err)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			n := obj.(*v1.Node)
			for _, h := range handlers {
				if err := h(ctx, []*v1.Node{n}, ModeRemove); err != nil {
					klog.Errorf("failed to update and sync node for remove %s for handler: %v", n.Name, err)
				}
			}
		},
	})

	// what this does:
	// when you create an informer, you start it by calling informer.Run()
	// however, it can take some time for the local state to sync up. If you use any methods before
	// it is completely synced, especially get or list, you can end up missing data. In order to
	// avoid the issue, you run it in the following order:
	//
	// 1. create your informer
	// 2. informer.Run()
	// 3. create a slice of sync functions []cache.InformerSynced. The function on each informer is informer.HasSynced
	// 4. use the utility function cache.WaitForCacheSync(), passing it your sync function slice
	// 5. when the utility function returns, the cache is synced and you are ready to use it
	//
	// for a good overview of controllers and their lifecycle, see https://engineering.bitnami.com/articles/a-deep-dive-into-kubernetes-controllers.html
	klog.V(5).Info("startNodesWatcher(): nodesInformer.Run()")
	go nodesInformer.Run(ctx.Done())
	syncFuncs := []cache.InformerSynced{
		nodesInformer.HasSynced,
	}
	klog.V(4).Infof("startNodesWatcher(): waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), syncFuncs...) {
		return fmt.Errorf("syncing caches failed")
	}
	klog.Info("nodes watcher started")
	return nil
}

// startServicesWatcher start a goroutine that watches k8s for services and calls
// any handlers
func startServicesWatcher(ctx context.Context, informer informers.SharedInformerFactory, handlers []serviceReconciler) error {
	klog.V(5).Info("called startServicesWatcher")
	if len(handlers) == 0 {
		klog.V(5).Info("no service handlers to process")
		return nil
	}

	// register to capture all new services
	servicesInformer := informer.Core().V1().Services().Informer()
	servicesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*v1.Service)
			for _, h := range handlers {
				if err := h(ctx, []*v1.Service{svc}, ModeAdd); err != nil {
					klog.Errorf("failed to update and sync service for add %s/%s: %v", svc.Namespace, svc.Name, err)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			svc := obj.(*v1.Service)
			for _, h := range handlers {
				if err := h(ctx, []*v1.Service{svc}, ModeRemove); err != nil {
					klog.Errorf("failed to update and sync service for remove %s/%s: %v", svc.Namespace, svc.Name, err)
				}
			}
		},
	})
	// what this does:
	// when you create an informer, you start it by calling informer.Run()
	// however, it can take some time for the local state to sync up. If you use any methods before
	// it is completely synced, especially get or list, you can end up missing data. In order to
	// avoid the issue, you run it in the following order:
	//
	// 1. create your informer
	// 2. informer.Run()
	// 3. create a slice of sync functions []cache.InformerSynced. The function on each informer is informer.HasSynced
	// 4. use the utility function cache.WaitForCacheSync(), passing it your sync function slice
	// 5. when the utility function returns, the cache is synced and you are ready to use it
	//
	// for a good overview of controllers and their lifecycle, see https://engineering.bitnami.com/articles/a-deep-dive-into-kubernetes-controllers.html
	klog.V(5).Info("startServicesWatcher(): servicesInformer.Run()")
	go servicesInformer.Run(ctx.Done())
	syncFuncs := []cache.InformerSynced{
		servicesInformer.HasSynced,
	}
	klog.V(4).Infof("startServicesWatcher(): waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), syncFuncs...) {
		return fmt.Errorf("syncing caches failed")
	}
	klog.Info("services watcher started")

	return nil
}

func timerLoop(ctx context.Context, informer informers.SharedInformerFactory, nodesHandlers []nodeReconciler, servicesHandlers []serviceReconciler) {
	servicesLister := informer.Core().V1().Services().Lister()
	nodesLister := informer.Core().V1().Nodes().Lister()
	for {
		select {
		case <-time.After(checkLoopTimerSeconds * time.Second):
			servicesList, err := servicesLister.List(labels.Everything())
			if err != nil {
				klog.Errorf("timed reservations watcher: failed to list services: %v", err)
			}
			for _, h := range servicesHandlers {
				if err := h(ctx, servicesList, ModeSync); err != nil {
					klog.Errorf("failed to update and sync services: %v", err)
				}
			}
			nodesList, err := nodesLister.List(labels.Everything())
			if err != nil {
				klog.Errorf("timed reservations watcher: failed to list nodes: %v", err)
			}
			for _, h := range nodesHandlers {
				if err := h(ctx, nodesList, ModeSync); err != nil {
					klog.Errorf("failed to update and sync nodes: %v", err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
