package metallb

import (
	"context"
	"encoding/json"
	"errors"
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
	hostnameKey      = "kubernetes.io/hostname"
	defaultNamespace = "metallb-system"
	defaultName      = "config"
)

type LB struct {
	configMapInterface typedv1.ConfigMapInterface
	configMapNamespace string
	configMapName      string
}

func NewLB(k8sclient kubernetes.Interface, config string) *LB {
	var configmapnamespace, configmapname string
	cmparts := strings.SplitN(config, ":", 2)
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

func (l *LB) AddService(ctx context.Context, svc, ip string) error {
	config, err := l.getConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %v", l.configMapNamespace, l.configMapName, err)
	}

	// Update the service and configmap and save them
	return mapIP(ctx, config, ip, svc, l.configMapName, l.configMapInterface)
}

func (l *LB) RemoveService(ctx context.Context, ip string) error {
	config, err := l.getConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %v", l.configMapNamespace, l.configMapName, err)
	}

	return unmapIP(ctx, config, ip, l.configMapName, l.configMapInterface)
}

func (l *LB) SyncServices(ctx context.Context, ips map[string]bool) error {
	config, err := l.getConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %v", l.configMapNamespace, l.configMapName, err)
	}

	// get all IPs registered in the configmap; remove those not in our valid list
	configIPs := getServiceAddresses(config)
	klog.V(2).Infof("metallb.SyncServices(): actual configmap IPs %v", configIPs)
	for _, ip := range configIPs {
		if _, ok := ips[ip]; !ok {
			klog.V(2).Infof("metallb.SyncServices(): removing from configmap ip %s not in valid list", ip)
			if err := unmapIP(ctx, config, ip, l.configMapName, l.configMapInterface); err != nil {
				return fmt.Errorf("error removing IP from configmap %s: %v", ip, err)
			}
		}
	}
	return nil
}

// AddNode add a node with the provided name, srcIP, and bgp information
func (l *LB) AddNode(ctx context.Context, nodeName string, localASN, peerASN int, srcIP string, peers ...string) error {
	config, err := l.getConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %v", l.configMapNamespace, l.configMapName, err)
	}

	ns := NodeSelector{
		MatchLabels: map[string]string{
			hostnameKey: nodeName,
		},
	}
	var changed bool
	for _, peer := range peers {
		p := Peer{
			MyASN:         uint32(localASN),
			ASN:           uint32(peerASN),
			Addr:          peer,
			NodeSelectors: []NodeSelector{ns},
		}
		if config.AddPeer(&p) {
			changed = true
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
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %v", l.configMapNamespace, l.configMapName, err)
	}
	// go through the peers and see if we have one with our hostname.
	selector := NodeSelector{
		MatchLabels: map[string]string{
			hostnameKey: nodeName,
		},
	}
	var changed bool
	if config.RemovePeerBySelector(&selector) {
		changed = true
	}
	if changed {
		return saveUpdatedConfigMap(ctx, l.configMapInterface, l.configMapName, config)
	}
	return nil
}

// SyncNodes ensure that the list of nodes is only those with the matched names
func (l *LB) SyncNodes(ctx context.Context, nodes map[string]loadbalancers.Node) error {
	config, err := l.getConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map %s:%s : %v", l.configMapNamespace, l.configMapName, err)
	}

	// first remove every node from the configmap that is not in the provided nodes
	configNodes := getNodes(config)
	for _, node := range configNodes {
		if _, ok := nodes[node]; !ok {
			klog.V(2).Infof("metallb.SyncNodes(): removing node from configmap: %s", node)
			if err := l.RemoveNode(ctx, node); err != nil {
				klog.V(2).Infof("metallb.SyncNodes(): error removing node %s: %v", node, err)
				continue
			}
		}
	}
	// now see if any nodes are missing
	// get the list of nodes afresh
	configNodes = getNodes(config)
	configMap := map[string]bool{}
	for _, node := range configNodes {
		configMap[node] = true
	}
	for _, node := range nodes {
		if _, ok := configMap[node.Name]; !ok {
			if err := l.AddNode(ctx, node.Name, node.LocalASN, node.PeerASN, node.SourceIP, node.Peers...); err != nil {
				klog.V(2).Infof("loadbalancers.reconcileNodes(): error adding node %s: %v", node.Name, err)
				continue
			}
		}
	}
	return nil
}

func (l *LB) getConfigMap(ctx context.Context) (*ConfigFile, error) {
	cm, err := l.configMapInterface.Get(ctx, l.configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get metallb configmap %s: %v", l.configMapName, err)
	}
	var (
		configData string
		ok         bool
	)
	if configData, ok = cm.Data["config"]; !ok {
		return nil, errors.New("configmap data has no property 'config'")
	}
	return ParseConfig([]byte(configData))
}

// mapIP add a given ip address to the metallb configmap
func mapIP(ctx context.Context, config *ConfigFile, addr, svcName, configmapname string, cmInterface typedv1.ConfigMapInterface) error {
	klog.V(2).Infof("mapping IP %s", addr)
	return updateMapIP(ctx, config, addr, svcName, configmapname, cmInterface, true)
}

// unmapIP remove a given IP address from the metalllb config map
func unmapIP(ctx context.Context, config *ConfigFile, addr, configmapname string, cmInterface typedv1.ConfigMapInterface) error {
	klog.V(2).Infof("unmapping IP %s", addr)
	return updateMapIP(ctx, config, addr, "", configmapname, cmInterface, false)
}

func updateMapIP(ctx context.Context, config *ConfigFile, addr, svcName, configmapname string, cmInterface typedv1.ConfigMapInterface, add bool) error {
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
		return fmt.Errorf("failed to update configmap: %v", err)
	}
	return nil
}

func saveUpdatedConfigMap(ctx context.Context, cmi typedv1.ConfigMapInterface, name string, cfg *ConfigFile) error {
	b, err := cfg.Bytes()
	if err != nil {
		return fmt.Errorf("error converting configfile data to bytes: %v", err)
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

// getServiceAddresses get the IPs of services in the metallb configmap
func getServiceAddresses(config *ConfigFile) []string {
	ips := []string{}
	pools := config.Pools
	for _, p := range pools {
		ips = append(ips, p.Addresses...)
	}
	return ips
}

// getNodes get the names of nodes in the metallb configmap
func getNodes(config *ConfigFile) []string {
	nodes := []string{}
	peers := config.Peers
	for _, p := range peers {
		for _, selector := range p.NodeSelectors {
			for k, v := range selector.MatchLabels {
				if k == hostnameKey {
					nodes = append(nodes, v)
				}
			}
		}
	}
	return nodes
}
