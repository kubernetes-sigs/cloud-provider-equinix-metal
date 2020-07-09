package packet

import (
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
	"k8s.io/klog"
)

const (
	packetAuthTokenEnvVar string = "PACKET_AUTH_TOKEN"
	packetProjectIDEnvVar string = "PACKET_PROJECT_ID"
	providerName          string = "packet"
	// ConsumerToken token for packet consumer
	ConsumerToken         string = "packet-ccm"
	checkLoopTimerSeconds        = 60
)

type nodeReconciler func(nodes []*v1.Node, remove bool) error
type serviceReconciler func(services []*v1.Service, remove bool) error

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

// Config configuration for a provider, includes authentication token, project ID ID, and optional override URL to talk to a different packet API endpoint
type Config struct {
	AuthToken            string  `json:"apiKey"`
	ProjectID            string  `json:"projectId"`
	BaseURL              *string `json:"base-url,omitempty"`
	DisableLoadBalancer  bool    `json:"disableLoadBalancer,omitempty"`
	Facility             string  `json:"facility,omitempty"`
	LoadBalancerManifest []byte
	PeerASN              int    `json:"peerASN,omitempty"`
	LocalASN             int    `json:"localASN,omitempty"`
	AnnotationLocalASN   string `json:"annotationLocalASN,omitEmpty"`
	AnnotationPeerASNs   string `json:"annotationPeerASNs,omitEmpty"`
	AnnotationPeerIPs    string `json:"annotationPeerIPs,omitEmpty"`
	EIPTag               string `json:"eipTag,omitEmpty"`
}

func newCloud(packetConfig Config, client *packngo.Client) (cloudprovider.Interface, error) {
	c := &cloud{
		client:       client,
		facility:     packetConfig.Facility,
		instances:    newInstances(client, packetConfig.ProjectID),
		zones:        newZones(client, packetConfig.ProjectID),
		loadBalancer: newLoadBalancers(client, packetConfig.ProjectID, packetConfig.Facility, packetConfig.DisableLoadBalancer, packetConfig.LoadBalancerManifest, packetConfig.LocalASN, packetConfig.PeerASN),
		bgp:          newBGP(client, packetConfig.ProjectID, packetConfig.LocalASN, packetConfig.PeerASN, packetConfig.AnnotationLocalASN, packetConfig.AnnotationPeerASNs, packetConfig.AnnotationPeerIPs),
	}
	if packetConfig.EIPTag != "" {
		c.controlPlaneEndpointManager = newControlPlaneEndpointManager(packetConfig.EIPTag, client.DeviceIPs, c.instances)
	}
	return c, nil
}

func InitializeProvider(packetConfig Config) error {
	// set up our client and create the cloud interface
	client := packngo.NewClientWithAuth("", packetConfig.AuthToken, nil)
	client.UserAgent = fmt.Sprintf("packet-ccm/%s %s", VERSION, client.UserAgent)
	cloud, err := newCloud(packetConfig, client)
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
	return []cloudService{c.loadBalancer, c.instances, c.zones, c.bgp}
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.V(5).Info("called Initialize")
	clientset := clientBuilder.ClientOrDie("packet-shared-informers")
	sharedInformer := informers.NewSharedInformerFactory(clientset, 0)
	// if we have services that want to reconcile, we will start node loop
	nodeReconcilers := []nodeReconciler{}
	serviceReconcilers := []serviceReconciler{}
	for _, elm := range c.services() {
		if err := elm.init(clientset); err != nil {
			klog.Errorf("could not initialize %s: %v", elm.name(), err)
			return
		}
		if n := elm.nodeReconciler(); n != nil {
			nodeReconcilers = append(nodeReconcilers, n)
		}
		if s := elm.serviceReconciler(); s != nil {
			serviceReconcilers = append(serviceReconcilers, s)
		}
	}

	if c.controlPlaneEndpointManager != nil {
		nodeReconcilers = append(nodeReconcilers, c.controlPlaneEndpointManager.instances.nodeReconciler())
	}

	if err := startNodesWatcher(sharedInformer, nodeReconcilers, stop); err != nil {
		klog.Errorf("nodes watcher initialization failed: %v", err)
	}
	if err := startServicesWatcher(sharedInformer, serviceReconcilers, stop); err != nil {
		klog.Errorf("services watcher initialization failed: %v", err)
	}
	go timerLoop(sharedInformer, nodeReconcilers, serviceReconcilers, stop)
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
	return false
}

// startNodesWatcher start a goroutine that watches k8s for nodes and calls any handlers
func startNodesWatcher(informer informers.SharedInformerFactory, handlers []nodeReconciler, stop <-chan struct{}) error {
	klog.V(5).Info("called startNodesWatcher")
	if len(handlers) == 0 {
		klog.V(5).Info("no service handlers to process")
		return nil
	}
	nodes := informer.Core().V1().Nodes()
	nodesLister := nodes.Lister()

	// next make sure existing nodes have it set
	nodeList, err := nodesLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}
	klog.V(2).Infof("existing nodes: %v", nodeList)
	for _, h := range handlers {
		if err := h(nodeList, false); err != nil {
			klog.Errorf("failed to update and sync nodes for handler: %v", err)
		}
	}

	klog.V(5).Info("startNodesWatcher(): creating nodesInformer")
	nodesInformer := nodes.Informer()
	klog.V(5).Info("startNodesWatcher(): adding event handlers")
	nodesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			n := obj.(*v1.Node)
			for _, h := range handlers {
				if err := h([]*v1.Node{n}, false); err != nil {
					klog.Errorf("failed to update and sync node for add %s for handler: %v", n.Name, err)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			n := obj.(*v1.Node)
			for _, h := range handlers {
				if err := h([]*v1.Node{n}, false); err != nil {
					klog.Errorf("failed to update and sync node for remove %s for handler: %v", n.Name, err)
				}
			}
		},
	})
	klog.V(5).Info("startNodesWatcher(): nodesInformer.Run()")
	go nodesInformer.Run(stop)
	klog.Info("nodes watcher started")
	return nil
}

// startServicesWatcher start a goroutine that watches k8s for services and calls
// any handlers
func startServicesWatcher(informer informers.SharedInformerFactory, handlers []serviceReconciler, stop <-chan struct{}) error {
	klog.V(5).Info("called startServicesWatcher")
	if len(handlers) == 0 {
		klog.V(5).Info("no service handlers to process")
		return nil
	}
	services := informer.Core().V1().Services()
	servicesLister := services.Lister()

	// next make sure existing nodes have it set
	servicesList, err := servicesLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list services: %v", err)
	}

	for _, h := range handlers {
		if err := h(servicesList, false); err != nil {
			klog.Errorf("failed to update and sync services: %v", err)
		}
	}

	// register to capture all new services
	servicesInformer := services.Informer()
	servicesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*v1.Service)
			for _, h := range handlers {
				if err := h([]*v1.Service{svc}, false); err != nil {
					klog.Errorf("failed to update and sync service for add %s/%s: %v", svc.Namespace, svc.Name, err)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			svc := obj.(*v1.Service)
			for _, h := range handlers {
				if err := h([]*v1.Service{svc}, true); err != nil {
					klog.Errorf("failed to update and sync service for remove %s/%s: %v", svc.Namespace, svc.Name, err)
				}
			}
		},
	})
	go servicesInformer.Run(stop)
	klog.Info("services watcher started")

	return nil
}

func timerLoop(informer informers.SharedInformerFactory, nodesHandlers []nodeReconciler, servicesHandlers []serviceReconciler, stop <-chan struct{}) {
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
				if err := h(servicesList, false); err != nil {
					klog.Errorf("failed to update and sync services: %v", err)
				}
			}
			nodesList, err := nodesLister.List(labels.Everything())
			if err != nil {
				klog.Errorf("timed reservations watcher: failed to list nodes: %v", err)
			}
			for _, h := range nodesHandlers {
				if err := h(nodesList, false); err != nil {
					klog.Errorf("failed to update and sync nodes: %v", err)
				}
			}
		case <-stop:
			return
		}
	}
}
