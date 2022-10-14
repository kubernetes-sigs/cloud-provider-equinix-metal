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
		return nil, fmt.Errorf("unable to get metallb configmap %s: %w", m.configmapName, err)
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

// AddPeerByService adds a peer for a specific service.
// If a matching peer already exists with the service, do not change anything.
// If a matching peer already exists but does not have the service, add it.
// Returns if anything changed.
func (m *CMConfigurer) AddPeerByService(add *Peer, svcNamespace, svcName string) bool {
	var found bool
	// ignore empty peer; nothing to add
	if add == nil {
		return false
	}

	// go through the peers and see if we have one that matches
	// definition of a match is:
	// - MyASN matches
	// - ASN matches
	// - Addr matches
	// - NodeSelectors all match (but order is ignored)
	var peers []Peer
	for _, peer := range m.config.Peers {
		if peer.EqualIgnoreService(add) {
			found = true
			peer.AddService(svcNamespace, svcName)
		}
		peers = append(peers, peer)
	}
	m.config.Peers = peers
	if found {
		return true
	}
	add.AddService(svcNamespace, svcName)
	m.config.Peers = append(m.config.Peers, *add)
	return true
}

// RemovePeersByService remove peers from a particular service.
// For any peers that have this services in the special MatchLabel, remove
// the service from the label. If there are no services left on a peer, remove the
// peer entirely.
func (m *CMConfigurer) RemovePeersByService(svcNamespace, svcName string) bool {
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
	return changed
}

// RemovePeersBySelector remove a peer by selector. If the matching peer does not exist, do not change anything.
// Returns if anything changed.
func (m *CMConfigurer) RemovePeersBySelector(remove *NodeSelector) bool {
	if remove == nil {
		return false
	}
	originalCount := len(m.config.Peers)
	// go through the peers and see if we have a match
	peers := make([]Peer, 0)
	for _, peer := range m.config.Peers {
		if !peer.MatchSelector(remove) {
			peers = append(peers, peer)
		}
	}
	m.config.Peers = peers
	return len(m.config.Peers) != originalCount
}

// AddAddressPool adds an address pool. If a matching pool already exists, do not change anything.
// Returns if anything changed
func (m *CMConfigurer) AddAddressPool(add *AddressPool) bool {
	// ignore empty peer; nothing to add
	if add == nil {
		return false
	}
	// go through the pools and see if we have one that matches
	for i, pool := range m.config.Pools {
		// MetalLB cannot handle two pools with everything the same
		// except for the name. So if we have two pools that are identical except for the name:
		// - if the name is the same, do nothing
		// - if the name is different, modify the name on the first to encompass both
		if pool.Equal(add) {
			// they were equal, so we found a matcher
			return false
		}
		if pool.EqualIgnoreName(add) {
			// they were not equal, so the names must be different. We need to modify
			// the name of the first one to cover both.
			existing := strings.Split(pool.Name, nameJoiner)
			for _, name := range existing {
				// if it already has it, no need to add anything
				if name == add.Name {
					return false
				}
			}
			// we made it here, so the name does not exist; add it
			existing = append(existing, add.Name)
			sort.Strings(existing)
			pool.Name = strings.Join(existing, nameJoiner)
			m.config.Pools[i] = pool
			return true
		}
	}

	// if we got here, none matched exactly, so add it
	m.config.Pools = append(m.config.Pools, *add)
	return true
}

// RemoveAddressPool remove a pool. If the matching pool does not exist, do not change anything
func (m *CMConfigurer) RemoveAddressPool(remove *AddressPool) {
	if remove == nil {
		return
	}
	// go through the pools and see if we have a match
	pools := make([]AddressPool, 0)
	// remove that one, keep all others
	for _, pool := range m.config.Pools {
		// if an exact match, continue
		if pool.Equal(remove) {
			continue
		}
		// if an exact match except for name, see if the name is in the list
		if pool.EqualIgnoreName(remove) {
			// they were not equal, so the names must be different.
			// check if it is in teh list
			existing := strings.Split(pool.Name, nameJoiner)
			var newNames []string
			for _, name := range existing {
				// if it already has it, no need to add anything
				if name == remove.Name {
					continue
				}
				newNames = append(newNames, name)
			}
			sort.Strings(newNames)
			pool.Name = strings.Join(newNames, nameJoiner)
		}
		pools = append(pools, pool)
	}
	m.config.Pools = pools
}

// RemoveAddressPooByAddress remove a pool by an address alone. If the matching pool does not exist, do not change anything
func (m *CMConfigurer) RemoveAddressPoolByAddress(addr string) {
	if addr == "" {
		return
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
}
