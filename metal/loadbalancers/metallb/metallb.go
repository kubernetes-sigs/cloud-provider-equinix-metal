package metallb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
)

const (
	hostnameKey         = "kubernetes.io/hostname"
	serviceNameKey      = "nomatch.metal.equinix.com/service-name"
	serviceNamespaceKey = "nomatch.metal.equinix.com/service-namespace"
	defaultNamespace    = "metallb-system"
	defaultName         = "config"
)

type LB struct {
	configMapInterface typedv1.ConfigMapInterface
	configMapNamespace string
	configMapName      string
}

func NewLB(k8sclient kubernetes.Interface, config string) *LB {
	var configmapnamespace, configmapname string
	// it may have an extra slash at the beginning or end, so get rid of it
	config = strings.TrimPrefix(config, "/")
	config = strings.TrimSuffix(config, "/")
	cmparts := strings.SplitN(config, "/", 2)
	if len(cmparts) >= 2 {
		configmapnamespace, configmapname = cmparts[0], cmparts[1]
	}
	// defaults
	if configmapname == "" {
		configmapname = defaultName
	}
	if configmapnamespace == "" {
		configmapnamespace = defaultNamespace
	}

	// get the configmap
	cmInterface := k8sclient.CoreV1().ConfigMaps(configmapnamespace)
	return &LB{
		configMapInterface: cmInterface,
		configMapNamespace: configmapnamespace,
		configMapName:      configmapname,
	}
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node) error {
	config, err := l.getConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %w", l.configMapNamespace, l.configMapName, err)
	}

	// Update the service and configmap and save them
	err = mapIP(ctx, config, ip, svcNamespace, svcName, l.configMapName, l.configMapInterface)
	if err != nil {
		return fmt.Errorf("unable to map IP to service: %w", err)
	}
	if err := l.addNodes(ctx, svcNamespace, svcName, nodes); err != nil {
		return fmt.Errorf("failed to add nodes: %w", err)
	}
	return nil
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string) error {
	config, err := l.getConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %w", l.configMapNamespace, l.configMapName, err)
	}

	// unmap the EIP
	if err := unmapIP(ctx, config, ip, l.configMapName, l.configMapInterface); err != nil {
		return fmt.Errorf("failed to remove IP: %w", err)
	}

	// remove any node entries for this service
	// go through the peers and see if we have one with our hostname.
	if config.RemovePeersByService(svcNamespace, svcName) {
		return saveUpdatedConfigMap(ctx, l.configMapInterface, l.configMapName, config)
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
	config, err := l.getConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %w", l.configMapNamespace, l.configMapName, err)
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
			if config.AddPeerByService(&p, svcNamespace, svcName) {
				changed = true
			}
		}
	}
	if changed {
		return saveUpdatedConfigMap(ctx, l.configMapInterface, l.configMapName, config)
	}
	return nil
}

// RemoveNode remove a node with the provided name
func (l *LB) RemoveNode(ctx context.Context, nodeName string) error {
	config, err := l.getConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %w", l.configMapNamespace, l.configMapName, err)
	}
	// go through the peers and see if we have one with our hostname.
	selector := NodeSelector{
		MatchLabels: map[string]string{
			hostnameKey: nodeName,
		},
	}
	var changed bool
	if config.RemovePeersBySelector(&selector) {
		changed = true
	}
	if changed {
		return saveUpdatedConfigMap(ctx, l.configMapInterface, l.configMapName, config)
	}
	return nil
}

func (l *LB) getConfigMap(ctx context.Context) (*ConfigFile, error) {
	cm, err := l.configMapInterface.Get(ctx, l.configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get metallb configmap %s: %w", l.configMapName, err)
	}
	// ignore checking if it exists; if not, it gives a blank string, which ParseConfig can handle anyways
	configData := cm.Data["config"]
	return ParseConfig([]byte(configData))
}

// mapIP add a given ip address to the metallb configmap
func mapIP(ctx context.Context, config *ConfigFile, addr, svcNamespace, svcName, configmapname string, cmInterface typedv1.ConfigMapInterface) error {
	klog.V(2).Infof("mapping IP %s", addr)
	return updateMapIP(ctx, config, addr, svcNamespace, svcName, configmapname, cmInterface, true)
}

// unmapIP remove a given IP address from the metalllb config map
func unmapIP(ctx context.Context, config *ConfigFile, addr, configmapname string, cmInterface typedv1.ConfigMapInterface) error {
	klog.V(2).Infof("unmapping IP %s", addr)
	return updateMapIP(ctx, config, addr, "", "", configmapname, cmInterface, false)
}

func updateMapIP(ctx context.Context, config *ConfigFile, addr, svcNamespace, svcName, configmapname string, cmInterface typedv1.ConfigMapInterface, add bool) error {
	if config == nil {
		klog.V(2).Info("config unchanged, not updating")
		return nil
	}
	// update the configmap and save it
	if add {
		autoAssign := false
		if !config.AddAddressPool(&AddressPool{
			Protocol:   "bgp",
			Name:       svcName,
			Addresses:  []string{addr},
			AutoAssign: &autoAssign,
		}) {
			klog.V(2).Info("address already on ConfigMap, unchanged")
			return nil
		}
	} else {
		config.RemoveAddressPoolByAddress(addr)
	}
	klog.V(2).Info("config changed, updating")
	if err := saveUpdatedConfigMap(ctx, cmInterface, configmapname, config); err != nil {
		klog.V(2).Infof("error updating configmap: %v", err)
		return fmt.Errorf("failed to update configmap: %w", err)
	}
	return nil
}

func saveUpdatedConfigMap(ctx context.Context, cmi typedv1.ConfigMapInterface, name string, cfg *ConfigFile) error {
	b, err := cfg.Bytes()
	if err != nil {
		return fmt.Errorf("error converting configfile data to bytes: %w", err)
	}

	mergePatch, _ := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{
			"config": string(b),
		},
	})

	klog.V(2).Infof("patching configmap:\n%s", mergePatch)
	// save to k8s
	_, err = cmi.Patch(ctx, name, k8stypes.MergePatchType, mergePatch, metav1.PatchOptions{})

	return err
}
