package metallb

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

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
	metallbCrdMinVersion      = "0.13.2"
)

type Configurer interface {
	// AddPeer adds a peer.
	// If a matching peer already exists with same settings and NodeSelectors, do not change anything.
	// If a matching peer already exists but does not have same NodeSelectors, add new Peer.
	// Returns if a new Peer is added.
	AddPeer(ctx context.Context, add *Peer) bool

	// AddPeerByService adds a peer for a specific service.
	// If a matching peer already exists with the service, do not change anything.
	// If a matching peer already exists but does not have the service, add it.
	// Returns if anything changed.
	AddPeerByService(ctx context.Context, add *Peer, svcNamespace, svcName string) bool

	// RemovePeersByService remove peers from a particular service.
	// For any peers that have this services in the special MatchLabel, remove
	// the service from the label. If there are no services left on a peer, remove the
	// peer entirely. Not applicable with CRD
	RemovePeersByService(ctx context.Context, svcNamespace, svcName string) bool

	// RemovePeersBySelector remove a peer by selector. If the matching peer does not exist, do not change anything.
	// Returns if anything changed.
	RemovePeersBySelector(ctx context.Context, remove *NodeSelector) (bool, error)

	// AddAddressPool adds an address pool. If a matching pool already exists, do not change anything.
	// Returns if anything changed
	AddAddressPool(ctx context.Context, add *AddressPool) (bool, error)

	// RemoveAddressPooByAddress remove a pool by an address alone. If the matching pool does not exist, do not change anything
	RemoveAddressPoolByAddress(ctx context.Context, addr string) error

	Get(context.Context) error
	Update(context.Context) error
}

type LB struct {
	configurer Configurer
	configurerType string
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

	//check metallb version
	crdConfiguration := false
	version, _ := metallbVersion(k8sclient, namespace)
	currentVersion, _ := semver.Make(version)
	crdMinVersion, _ := semver.Make(metallbCrdMinVersion)

	// if current ver is >= min ver supporting CRD; then crdConfiguration = true
	if currentVersion.Compare(crdMinVersion) != -1 {
		crdConfiguration = true
	}
	lb := &LB{}
	if crdConfiguration {

		scheme := runtime.NewScheme()
		_ = metallbv1beta1.AddToScheme(scheme)
		cl, err := client.New(clientconfig.GetConfigOrDie(), client.Options{Scheme: scheme})
		if err != nil {
			panic(err)
		}

		// defaults
		bgpAdvs := extraConfig[bgpAdvertisementConfigKey]
		if len(bgpAdvs) == 0 {
			bgpAdvs = []string{ defaultBgpAdvertisement }
		}

		// lb.configurer = &CRDConfigurer{namespace: namespace, crdi: k8sApiextensionsClientset.ApiextensionsV1beta1().CustomResourceDefinitions()}
		lb.configurer = &CRDConfigurer{namespace: namespace, client: cl, bgpAdvertisements: bgpAdvs}
		lb.configurerType = "CRD"
	} else {
		// get the configmapinterface scoped to the namespace
		cmInterface := k8sclient.CoreV1().ConfigMaps(namespace)
		lb.configurer = &CMConfigurer{namespace: namespace, configmapName: configmapname, cmi: cmInterface}
		lb.configurerType = "CM"
	}

	return lb
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node) error {
	config := l.configurer
	if err := config.Get(ctx); err != nil {
		return fmt.Errorf("unable to add service: %w", err)
	}

	// Update the service and configmap/IpAddressPool and save them
	if err := addIP(ctx, config, ip, svcNamespace, svcName, l.configurerType); err != nil {
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

	// remove the EIP
	if err := removeIP(ctx, config, ip, l.configurerType); err != nil {
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
		for i, peer := range node.Peers {
			p := Peer{
				MyASN:         uint32(node.LocalASN),
				ASN:           uint32(node.PeerASN),
				Password:      node.Password,
				Addr:          peer,
				SrcAddr:       node.SourceIP,
				NodeSelectors: ns,
			}
			if l.configurerType == "CM" && config.AddPeerByService(ctx, &p, svcNamespace, svcName) {
				changed = true
			} else if l.configurerType == "CRD" {
				p.Name = fmt.Sprintf("%s-%d", node.Name, i)
				// TODO (ocobleseqx) could it be another port num?
				p.Port = 179
				config.AddPeer(ctx, &p)
			}
		}
	}
	if l.configurerType == "CM" && changed {
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

	changed, err := config.RemovePeersBySelector(ctx, &selector)

	if err != nil {
		return err
	}

	if l.configurerType == "CM" && changed {
		if err := config.Update(ctx); err != nil {
			return fmt.Errorf("unable to remove node: %w", err)
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
func removeIP(ctx context.Context, config Configurer, addr, configurerType string) error {
	klog.V(2).Infof("unmapping IP %s", addr)
	return updateIP(ctx, config, addr, "", "", configurerType, false)
}

func updateIP(ctx context.Context, config Configurer, addr, svcNamespace, svcName, configurerType string, add bool) error {
	if config == nil {
		klog.V(2).Info("config unchanged, not updating")
		return nil
	}
	// < v0.13: update the ConfigMap and save it
	// > v0.13: update/create new AddressPool
	var addrName string
	if configurerType == "CM" {
		addrName = fmt.Sprintf("%s/%s", svcNamespace, svcName)
	} else {
		addrName = svcName
	}

	if add {
		autoAssign := false

		added, err := config.AddAddressPool(ctx, &AddressPool{
			Protocol:   "bgp",
			Name:       addrName,
			Addresses:  []string{addr},
			AutoAssign: &autoAssign,
		})

		if err != nil {
			return fmt.Errorf("failed to add new address pool: %w", err)
		}

		if !added {
			klog.V(2).Info("address pool already exists, unchanged")
			return nil
		}
	} else {
		if configurerType == "CM" {
			config.RemoveAddressPoolByAddress(ctx, addr)
		} else {
			config.RemoveAddressPoolByAddress(ctx, addrName)
		}
	}

	// > v0.13: Update not required, will return nil
	if err := config.Update(ctx); err != nil {
		klog.V(2).Infof("error updating configmap: %v", err)
		return fmt.Errorf("failed to update configmap: %w", err)
	}
	return nil
}

func metallbVersion(k8sclient kubernetes.Interface, namespace string) (string, error) {
	listOptions := metav1.ListOptions{
        LabelSelector: "app=metallb,component=controller",
    }
	deploys, err := k8sclient.AppsV1().Deployments(namespace).List(context.Background(), listOptions )

	if err != nil {
		return "", fmt.Errorf("unable to get metallb controller deployment %s:controller %w", namespace, err)
	}

	if len(deploys.Items) > 0 {
		for _, c := range deploys.Items[0].Spec.Template.Spec.Containers {
			img := strings.Split(c.Image, ":v")
			if len(img) > 1 {
				if img[0] == "quay.io/metallb/controller" {
					return img[1], nil
				}

			}
		}
	}
	return "", fmt.Errorf("unable to get metallb installed version in %s", namespace)
}
