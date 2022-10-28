package metallb

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	// k8sapiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	hostnameKey         = "kubernetes.io/hostname"
	serviceNameKey      = "nomatch.metal.equinix.com/service-name"
	serviceNamespaceKey = "nomatch.metal.equinix.com/service-namespace"
	defaultNamespace    = "metallb-system"
	defaultName         = "config"
)

type Configurer interface {
	// AddPeerByService adds a peer for a specific service.
	// If a matching peer already exists with the service, do not change anything.
	// If a matching peer already exists but does not have the service, add it.
	// Returns if anything changed.
	AddPeerByService(ctx context.Context, add *Peer, svcNamespace, svcName string) bool

	// RemovePeersByService remove peers from a particular service.
	// For any peers that have this services in the special MatchLabel, remove
	// the service from the label. If there are no services left on a peer, remove the
	// peer entirely.
	RemovePeersByService(ctx context.Context, svcNamespace, svcName string) bool

	// RemovePeersBySelector remove a peer by selector. If the matching peer does not exist, do not change anything.
	// Returns if anything changed.
	RemovePeersBySelector(ctx context.Context, remove *NodeSelector) bool

	// AddAddressPool adds an address pool. If a matching pool already exists, do not change anything.
	// Returns if anything changed
	AddAddressPool(ctx context.Context, add *AddressPool) bool

	// RemoveAddressPooByAddress remove a pool by an address alone. If the matching pool does not exist, do not change anything
	RemoveAddressPoolByAddress(ctx context.Context, addr string)

	Get(context.Context) error
	Update(context.Context) error
}

type LB struct {
	configurer Configurer
}

var _ loadbalancers.LB = (*LB)(nil)

// func NewLB(k8sclient kubernetes.Interface, k8sApiextensionsClientset *k8sapiextensionsclient.Clientset, config string) *LB {
func NewLB(k8sclient kubernetes.Interface, config string, extraConfig map[string][]string) *LB {
	var namespace, configmapname string

	// it may have an extra slash at the beginning or end, so get rid of it
	config = strings.TrimPrefix(config, "/")
	config = strings.TrimSuffix(config, "/")
	cmparts := strings.SplitN(config, "/", 2)
	if len(cmparts) >= 2 {
		namespace, configmapname = cmparts[0], cmparts[1]
	}
	// defaults
	if configmapname == "" {
		configmapname = defaultName
	}
	if namespace == "" {
		namespace = defaultNamespace
	}

	crdConfiguration := false

	lb := &LB{}
	if crdConfiguration {

		scheme := runtime.NewScheme()
		_ = metallbv1beta1.AddToScheme(scheme)
		cl, err := client.New(clientconfig.GetConfigOrDie(), client.Options{Scheme: scheme})
		if err != nil {
			panic(err)
		}

		// lb.configurer = &CRDConfigurer{namespace: namespace, crdi: k8sApiextensionsClientset.ApiextensionsV1beta1().CustomResourceDefinitions()}
		lb.configurer = &CRDConfigurer{namespace: namespace, client: cl, advertisementNames: extraConfig["advertisment-names"]}
	} else {
		// get the configmapinterface scoped to the namespace
		cmInterface := k8sclient.CoreV1().ConfigMaps(namespace)
		lb.configurer = &CMConfigurer{namespace: namespace, configmapName: configmapname, cmi: cmInterface}
	}

	return lb
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node) error {
	config := l.configurer
	if err := config.Get(ctx); err != nil {
		return fmt.Errorf("unable to add service: %w", err)
	}

	// Update the service and configmap and save them
	if err := mapIP(ctx, config, ip, svcNamespace, svcName); err != nil {
		return fmt.Errorf("unable to map IP to service: %w", err)
	}
	if err := l.addNodes(ctx, svcNamespace, svcName, nodes); err != nil {
		return fmt.Errorf("unable to add service: %w", err)
	}
	return nil
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string) error {
	config := l.configurer
	if err := config.Get(ctx); err != nil {
		return fmt.Errorf("unable to remove service: %w", err)
	}

	// unmap the EIP
	if err := unmapIP(ctx, config, ip); err != nil {
		return fmt.Errorf("failed to remove IP: %w", err)
	}

	// remove any node entries for this service
	// go through the peers and see if we have one with our hostname.
	if config.RemovePeersByService(ctx, svcNamespace, svcName) {
		if err := config.Update(ctx); err != nil {
			return fmt.Errorf("unable to remove service: %w", err)
		}
	}
	return nil
}

func (l *LB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node) error {
	// find the service whose name matches the requested svc

	// ensure nodes are correct
	if err := l.addNodes(ctx, svcNamespace, svcName, nodes); err != nil {
		return fmt.Errorf("failed to add nodes: %w", err)
	}
	return nil
}

// addNodes add one or more nodes with the provided name, srcIP, and bgp information
func (l *LB) addNodes(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node) error {
	config := l.configurer
	if err := config.Get(ctx); err != nil {
		return fmt.Errorf("unable to add nodes: %w", err)
	}

	var changed bool
	for _, node := range nodes {
		ns := []NodeSelector{
			{MatchLabels: map[string]string{
				hostnameKey: node.Name,
			}},
		}
		for _, peer := range node.Peers {
			p := Peer{
				MyASN:         uint32(node.LocalASN),
				ASN:           uint32(node.PeerASN),
				Password:      node.Password,
				Addr:          peer,
				SrcAddr:       node.SourceIP,
				NodeSelectors: ns,
			}
			if config.AddPeerByService(ctx, &p, svcNamespace, svcName) {
				changed = true
			}
		}
	}
	if changed { // and type configmap
		if err := config.Update(ctx); err != nil {
			return fmt.Errorf("unable to add nodes: %w", err)
		}
	}
	return nil
}

// RemoveNode remove a node with the provided name
func (l *LB) RemoveNode(ctx context.Context, nodeName string) error {
	config := l.configurer
	if err := config.Get(ctx); err != nil {
		return fmt.Errorf("unable to remove node : %w", err)
	}
	// go through the peers and see if we have one with our hostname.
	selector := NodeSelector{
		MatchLabels: map[string]string{
			hostnameKey: nodeName,
		},
	}
	var changed bool
	if config.RemovePeersBySelector(ctx, &selector) {
		changed = true
	}
	if changed {
		if err := config.Update(ctx); err != nil {
			return fmt.Errorf("unable to remove node: %w", err)
		}
	}
	return nil
}

// mapIP add a given ip address to the metallb configmap
func mapIP(ctx context.Context, config Configurer, addr, svcNamespace, svcName string) error {
	klog.V(2).Infof("mapping IP %s", addr)
	return updateMapIP(ctx, config, addr, svcNamespace, svcName, true)
}

// unmapIP remove a given IP address from the metalllb config map
func unmapIP(ctx context.Context, config Configurer, addr string) error {
	klog.V(2).Infof("unmapping IP %s", addr)
	return updateMapIP(ctx, config, addr, "", "", false)
}

func updateMapIP(ctx context.Context, config Configurer, addr, svcNamespace, svcName string, add bool) error {
	if config == nil {
		klog.V(2).Info("config unchanged, not updating")
		return nil
	}
	// update the configmap and save it
	if add {
		autoAssign := false
		if !config.AddAddressPool(ctx, &AddressPool{
			Protocol:   "bgp",
			Name:       fmt.Sprintf("%s/%s", svcNamespace, svcName),
			Addresses:  []string{addr},
			AutoAssign: &autoAssign,
		}) {
			klog.V(2).Info("address already on ConfigMap, unchanged")
			return nil
		}
	} else {
		config.RemoveAddressPoolByAddress(ctx, addr)
	}
	klog.V(2).Info("config changed, updating")
	if err := config.Update(ctx); err != nil {
		klog.V(2).Infof("error updating configmap: %v", err)
		return fmt.Errorf("failed to update configmap: %w", err)
	}
	return nil
}
