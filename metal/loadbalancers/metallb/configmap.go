package metallb

/*
 contains additional funcs, not in original metallb packages
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
)

const (
	// nameJoiner character that joins names in address pools
	nameJoiner = ","
)

type CMConfigurer struct {
	namespace     string // defaults to metallb-system
	configmapName string
	config        *ConfigFile
	cmi           typedv1.ConfigMapInterface
}

var _ Configurer = (*CMConfigurer)(nil)

func (m *CMConfigurer) Get(ctx context.Context) error {
	var err error
	m.config, err = m.getConfigMap(ctx)
	return err
}

func (m *CMConfigurer) getConfigMap(ctx context.Context) (*ConfigFile, error) {
	cm, err := m.cmi.Get(ctx, m.configmapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get metallb configmap %s:%s %w", m.namespace, m.configmapName, err)
	}
	// ignore checking if it exists; if not, it gives a blank string, which ParseConfig can handle anyways
	configData := cm.Data["config"]
	return ParseConfig([]byte(configData))
}

func (m *CMConfigurer) Update(ctx context.Context) error {
	b, err := m.config.Bytes()
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
	_, err = m.cmi.Patch(ctx, m.configmapName, k8stypes.MergePatchType, mergePatch, metav1.PatchOptions{})

	return err
}

func (m *CMConfigurer) UpdatePeersByService(ctx context.Context, adds *[]Peer, svcNamespace, svcName string) (bool, error) {
	var changed bool
	for _, add := range *adds {
		// go through the peers and see if we have one that matches
		// definition of a match is:
		// - MyASN matches
		// - ASN matches
		// - Addr matches
		// - NodeSelectors all match (but order is ignored)
		var peers []Peer
		var found bool
		for _, peer := range m.config.Peers {
			if peer.EqualIgnoreService(&add) {
				found = true
				peer.AddService(svcNamespace, svcName)
			}
			peers = append(peers, peer)
		}
		m.config.Peers = peers

		if !found {
			add.AddService(svcNamespace, svcName)
			m.config.Peers = append(m.config.Peers, add)
			found = true
		}

		if !changed {
			changed = found
		}
	}
	return changed, nil
}

// RemovePeersByService remove peers from a particular service.
// For any peers that have this services in the special MatchLabel, remove
// the service from the label. If there are no services left on a peer, remove the
// peer entirely.
func (m *CMConfigurer) RemovePeersByService(ctx context.Context, svcNamespace, svcName string) (bool, error) {
	var changed bool
	// go through the peers and see if we have a match
	peers := make([]Peer, 0)
	// remove that one, keep all others
	for _, peer := range m.config.Peers {
		// get the services for which this peer works
		peerChanged, size := peer.RemoveService(svcNamespace, svcName)

		// if not changed, or it has at least one service left, we can keep this node
		if !peerChanged || size >= 1 {
			peers = append(peers, peer)
		}
		if peerChanged || size <= 0 {
			changed = true
		}
	}
	m.config.Peers = peers
	return changed, nil
}

// AddAddressPool adds an address pool. If a matching pool already exists, do not change anything.
// Returns if anything changed
func (m *CMConfigurer) AddAddressPool(ctx context.Context, add *AddressPool, svcNamespace, svcName string) (bool, error) {
	// ignore empty pool; nothing to add
	if add == nil {
		return false, nil
	}
	// go through the pools and see if we have one that matches
	for i, pool := range m.config.Pools {
		// MetalLB cannot handle two pools with everything the same
		// except for the name. So if we have two pools that are identical except for the name:
		// - if the name is the same, do nothing
		// - if the name is different, modify the name on the first to encompass both
		if pool.Equal(add) {
			// they were equal, so we found a matcher
			return false, nil
		}
		if pool.EqualIgnoreName(add) {
			// they were not equal, so the names must be different. We need to modify
			// the name of the first one to cover both.
			existing := strings.Split(pool.Name, nameJoiner)
			for _, name := range existing {
				// if it already has it, no need to add anything
				if name == add.Name {
					return false, nil
				}
			}
			// we made it here, so the name does not exist; add it
			existing = append(existing, add.Name)
			sort.Strings(existing)
			pool.Name = strings.Join(existing, nameJoiner)
			m.config.Pools[i] = pool
			return true, nil
		}
	}

	// if we got here, none matched exactly, so add it
	m.config.Pools = append(m.config.Pools, *add)
	return true, nil
}

// RemoveAddressPooByAddress remove a pool by an address alone. If the matching pool does not exist, do not change anything
func (m *CMConfigurer) RemoveAddressPoolByAddress(ctx context.Context, addr string) error {
	if addr == "" {
		return nil
	}
	// go through the pools and see if we have a match
	pools := make([]AddressPool, 0)
	// go through the pools and see if we have one with our hostname
	for _, pool := range m.config.Pools {
		var found bool
		for _, ipaddr := range pool.Addresses {
			if addr == ipaddr {
				found = true
			}
		}
		if !found {
			pools = append(pools, pool)
		}
	}
	m.config.Pools = pools
	return nil
}

// RemoveFromAddressPool remove service from a pool by name. If the matching pool is not found, do not change anything
func (m *CMConfigurer) RemoveFromAddressPool(ctx context.Context, svcNamespace, svcName string) error {
	return nil
}

// RemoveAddressPool remove a pool by name. If the matching pool does not exist, do not change anything
func (m *CMConfigurer) RemoveAddressPool(ctx context.Context, pool string) error { return nil }
