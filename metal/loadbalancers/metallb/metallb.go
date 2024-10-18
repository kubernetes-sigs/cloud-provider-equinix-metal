package metallb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cloud-provider-equinix-metal/metal/loadbalancers"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	hostnameKey               = "kubernetes.io/hostname"
	serviceNameKey            = "nomatch.metal.equinix.com/service-name"
	serviceNamespaceKey       = "nomatch.metal.equinix.com/service-namespace"
	defaultNamespace          = "metallb-system"
	defaultName               = "config"
	bgpAdvertisementConfigKey = "bgp_advertisments"
)

type Configurer interface {
	// AddPeerByService adds a peer for a specific service.
	// If a matching peer already exists with the service, do not change anything.
	// If a matching peer already exists but does not have the service, add it.
	// Returns if anything changed.
	UpdatePeersByService(ctx context.Context, adds *[]Peer, svcNamespace, svcName string) (bool, error)

	// RemovePeersByService remove peers from a particular service.
	// For any peers that have this services in the special MatchLabel, remove
	// the service from the label. If there are no services left on a peer, remove the
	// peer entirely. Not applicable with CRD
	RemovePeersByService(ctx context.Context, svcNamespace, svcName string) (bool, error)

	// AddAddressPool adds an address pool. If a matching pool already exists, do not change anything.
	// Returns if anything changed
	AddAddressPool(ctx context.Context, add *AddressPool, svcNamespace, svcName string) (bool, error)

	// RemoveFromAddressPool remove service from a pool by name. If the matching pool if not found, do not change anything
	RemoveFromAddressPool(ctx context.Context, svcNamespace, svcName string) error

	// RemoveAddressPool remove a pool by name. If the matching pool does not exist, do not change anything
	RemoveAddressPool(ctx context.Context, pool string) error

	// RemoveAddressPoolByAddress remove a pool by an address alone. If the matching pool does not exist, do not change anything
	RemoveAddressPoolByAddress(ctx context.Context, addr string) error

	Get(context.Context) error
	Update(context.Context) error
}

type LB struct {
	configurer     Configurer
	configurerType string
}

var (
	_                loadbalancers.LB = (*LB)(nil)
	crdConfiguration                  = false

	ErrIPStillInUse = errors.New("ip address still in use")
)

// func NewLB(k8sclient kubernetes.Interface, k8sApiextensionsClientset *k8sapiextensionsclient.Clientset, config string) *LB {
func NewLB(k8sclient kubernetes.Interface, config string, featureFlags url.Values) *LB {
	var namespace, configmapname string

	// it may have an extra slash at the beginning or end, so get rid of it
	config = strings.TrimPrefix(config, "/")
	config = strings.TrimSuffix(config, "/")
	cmparts := strings.SplitN(config, "/", 2)

	if len(cmparts) >= 1 {
		namespace = cmparts[0]
	}

	if len(cmparts) >= 2 {
		configmapname = cmparts[1]
	}

	// defaults
	if configmapname == "" {
		configmapname = defaultName
	}
	if namespace == "" {
		namespace = defaultNamespace
	}

	if featureFlags.Has("crdConfiguration") {
		rawCrdConfiguration := featureFlags.Get("crdConfiguration")
		parsedCrdConfiguration, err := strconv.ParseBool(rawCrdConfiguration)
		if err != nil {
			panic(fmt.Errorf("crdConfiguration must be a boolean, was %s: %w", rawCrdConfiguration, err))
		}
		crdConfiguration = parsedCrdConfiguration
	}

	lb := &LB{}
	if crdConfiguration {
		scheme := runtime.NewScheme()
		_ = metallbv1beta1.AddToScheme(scheme)
		_ = v1.AddToScheme(scheme)
		cl, err := client.New(clientconfig.GetConfigOrDie(), client.Options{Scheme: scheme})
		if err != nil {
			panic(err)
		}
		lb.configurer = &CRDConfigurer{namespace: namespace, client: cl}
	} else {
		// get the configmapinterface scoped to the namespace
		cmInterface := k8sclient.CoreV1().ConfigMaps(namespace)
		lb.configurer = &CMConfigurer{namespace: namespace, configmapName: configmapname, cmi: cmInterface}
	}

	return lb
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node, svc *v1.Service, n []*v1.Node, loadBalancerName string) error {
	config := l.configurer
	if err := config.Get(ctx); err != nil {
		return fmt.Errorf("unable to add service: %w", err)
	}

	// Update the service and configmap/IpAddressPool and save them
	if err := addIP(ctx, config, ip, svcNamespace, svcName, l.configurerType); err != nil {
		return fmt.Errorf("unable to map IP to service: %w", err)
	}
	if err := l.updateNodes(ctx, svcNamespace, svcName, nodes); err != nil {
		return fmt.Errorf("unable to add service: %w", err)
	}
	return nil
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string, svc *v1.Service) error {
	config := l.configurer
	if err := config.Get(ctx); err != nil {
		return fmt.Errorf("unable to remove service: %w", err)
	}

	// remove the EIP
	if err := removeIP(ctx, config, ip, svcNamespace, svcName, l.configurerType); err != nil {
		return fmt.Errorf("failed to remove IP: %w", err)
	}

	// remove any node entries for this service
	// go through the peers and see if we have one with our hostname.
	removed, err := config.RemovePeersByService(ctx, svcNamespace, svcName)
	if err != nil {
		return fmt.Errorf("unable to remove service: %w", err)
	}
	if removed {
		if err := config.Update(ctx); err != nil {
			return fmt.Errorf("unable to remove service: %w", err)
		}
	}
	return nil
}

func (l *LB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node, svc *v1.Service, n []*v1.Node) error {
	// ensure nodes are correct
	if err := l.updateNodes(ctx, svcNamespace, svcName, nodes); err != nil {
		return fmt.Errorf("failed to add nodes: %w", err)
	}
	return nil
}

func (l *LB) GetLoadBalancer(ctx context.Context, clusterName string, svc *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	// TODO
	return nil, false, nil
}

// updateNodes add/delete one or more nodes with the provided name, srcIP, and bgp information
func (l *LB) updateNodes(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node) error {
	config := l.configurer
	if err := config.Get(ctx); err != nil {
		return fmt.Errorf("unable to add nodes: %w", err)
	}

	var changed bool
	var peersToUpdate []Peer
	for _, node := range nodes {
		ns := []NodeSelector{
			{MatchLabels: map[string]string{
				hostnameKey: node.Name,
			}},
		}
		for i, peer := range node.Peers {
			p := Peer{
				MyASN:         uint32(node.LocalASN),
				ASN:           uint32(node.PeerASN),
				Password:      node.Password,
				Addr:          peer,
				SrcAddr:       node.SourceIP,
				NodeSelectors: ns,
			}
			if crdConfiguration {
				p.Name = fmt.Sprintf("%s-%d", node.Name, i)
				// TODO (ocobleseqx) could it be another port num?
				p.Port = 179
			}
			peersToUpdate = append(peersToUpdate, p)
		}
	}
	// to ensure that the nodes are correct, we need to check the nodes specified
	// for these services against the whole list of nodes/peers saved in the configuration
	changed, err := config.UpdatePeersByService(ctx, &peersToUpdate, svcNamespace, svcName)
	if err != nil {
		return fmt.Errorf("unable to update nodes: %w", err)
	}

	if changed {
		if err := config.Update(ctx); err != nil {
			return fmt.Errorf("unable to update nodes: %w", err)
		}
	}
	return nil
}

// addIP add a given ip address to the metallb ConfigMap or IPAddressPool
func addIP(ctx context.Context, config Configurer, addr, svcNamespace, svcName, configurerType string) error {
	klog.V(2).Infof("mapping IP %s", addr)
	return updateIP(ctx, config, addr, svcNamespace, svcName, configurerType, true)
}

// removeIP remove a given IP address from the metalllb ConfigMap or IPAddressPool
func removeIP(ctx context.Context, config Configurer, addr, svcNamespace, svcName, configurerType string) error {
	klog.V(2).Infof("unmapping IP %s", addr)
	return updateIP(ctx, config, addr, svcNamespace, svcName, configurerType, false)
}

func updateIP(ctx context.Context, config Configurer, addr, svcNamespace, svcName, configurerType string, add bool) error {
	if config == nil {
		klog.V(2).Info("config unchanged, not updating")
		return nil
	}
	// < v0.13: update the ConfigMap and save it
	// > v0.13: update/create new AddressPool
	var name string
	if !crdConfiguration {
		name = fmt.Sprintf("%s/%s", svcNamespace, svcName)
	} else {
		name = poolName(svcNamespace, svcName)
	}

	if add {
		autoAssign := false

		added, err := config.AddAddressPool(ctx, &AddressPool{
			Protocol:   "bgp",
			Name:       name,
			Addresses:  []string{addr},
			AutoAssign: &autoAssign,
		}, svcNamespace, svcName)
		if err != nil {
			klog.V(2).Infof("error adding IP: %v", err)
			return fmt.Errorf("error adding IP: %w", err)
		}
		if !added {
			klog.V(2).Info("address pool already exists, unchanged")
			return nil
		}
	} else {
		if !crdConfiguration {
			if err := config.RemoveAddressPoolByAddress(ctx, addr); err != nil {
				klog.V(2).Infof("error removing IP: %v", err)
				return fmt.Errorf("error removing IP: %w", err)
			}
		} else {
			if err := config.RemoveFromAddressPool(ctx, svcNamespace, svcName); err != nil {
				klog.V(2).Infof("error removing from IPAddressPool: %v", err)
				return fmt.Errorf("error removing from IPAddressPool: %w", err)
			}
		}
	}

	if !crdConfiguration {
		if err := config.Update(ctx); err != nil {
			klog.V(2).Infof("error updating configmap: %v", err)
			return fmt.Errorf("failed to update configmap: %w", err)
		}
	}
	return nil
}
