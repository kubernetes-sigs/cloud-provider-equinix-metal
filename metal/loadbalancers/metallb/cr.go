package metallb

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"time"

	// "fmt"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CRDConfigurer struct {
	namespace string // defaults to metallb-system

	// // the name of the IPAddressPool, used in metallb.universe.tf/address-pool service annotations
	// ipaddresspoolName string // defaults to equinix-metal-ip-address-pool

	// bgppeerPrefix string // defaults to equinix-metal-bgp-peer

	advertisementNames []string // defaults to equinix-metal-bgp-adv

	client client.Client

	// bgpadvconfig BGPAdvConfigFile

	// config CRDConfigFile
	// ctx context.Context
}

var _ Configurer = (*CRDConfigurer)(nil)

func (m *CRDConfigurer) AddPeerByService(ctx context.Context, add *Peer, svcNamespace, svcName string) bool {
	var found bool
	// ignore empty peer; nothing to add
	if add == nil {
		return false
	}

	// go through the pools and see if we have one that matches
	newBGPPeer := convertToBGPPeer(*add)

	// go through the peers and see if we have one that matches
	// definition of a match is:
	// - MyASN matches
	// - ASN matches
	// - Addr matches
	// - NodeSelectors all match (but order is ignored)
	// var peers []Peer
	peers, err := m.listBGPPeers(ctx)
	if err != nil {
		// TODO return error
		klog.V(2).ErrorS(err, "unable to retrieve a list of BGPPeers")
		return false
	}

	for _, peer := range peers.Items {
		if peerEqualIgnoreService(peer.Spec, newBGPPeer.Spec) {
			found = true
			if ns, peerAdded := peerAppendServiceToNodeSelectors(svcNamespace, svcName, peer.Spec.NodeSelectors); peerAdded {
				if err := m.UpdateBGPPeerNodeSelectors(ctx, peer, ns); err != nil {
					// TODO return error
					klog.V(2).ErrorS(err, "unable to update BGPPeer NodeSelectors")
					return false
				}
			}
		}
	}
	if found {
		return true
	}
	if ns, peerAdded := peerAppendServiceToNodeSelectors(svcNamespace, svcName, newBGPPeer.Spec.NodeSelectors); peerAdded {
		if err := m.UpdateBGPPeerNodeSelectors(ctx, newBGPPeer, ns); err != nil {
			// TODO return error
			klog.V(2).ErrorS(err, "unable to update BGPPeer NodeSelectors")
			return false
		}
	}
	return true
}

func (m *CRDConfigurer) RemovePeersByService(ctx context.Context, svcNamespace, svcName string) bool {
	var changed bool
	// go through the peers and see if we have a match
	peers, err := m.listBGPPeers(ctx)
	if err != nil {
		// TODO return error
		klog.V(2).ErrorS(err, "unable to retrieve a list of BGPPeers")
		return false
	}

	// remove that one, keep all others
	for _, peer := range peers.Items {
		// get the services for which this peer works
		ns, peerChanged, size := peerRemoveServiceFromNodeSelectors(svcNamespace, svcName, peer.Spec.NodeSelectors)

		// if changed, or there is no service left, we can remove this Peer
		if peerChanged {
			changed = true
			if size <= 0 {
				// remove BGP Peer
				if err := m.client.Delete(ctx, &peer); err != nil {
					// TODO return error
					klog.V(2).ErrorS(err, "unable to update BGPPeer NodeSelectors")
					return false
				}
			} else {
				// update
				if err := m.UpdateBGPPeerNodeSelectors(ctx, peer, ns); err != nil {
					// TODO return error
					klog.V(2).ErrorS(err, "unable to update BGPPeer NodeSelectors")
					return false
				}
			}
		}
	}
	return changed
}

func (m *CRDConfigurer) RemovePeersBySelector(ctx context.Context, remove *NodeSelector) bool {
	if remove == nil {
		return false
	}

	// go through the peers and see if we have a match
	peers, err := m.listBGPPeers(ctx)
	if err != nil {
		// TODO return error
		klog.V(2).ErrorS(err, "unable to retrieve a list of BGPPeers")
		return false
	}

	nsToRemove := convertToNodeSelector(*remove)
	// go through the peers and see if we have a match
	// peers := make([]Peer, 0)
	var removed bool
	for _, peer := range peers.Items {
		for _, ns := range peer.Spec.NodeSelectors {
			if reflect.DeepEqual(ns, nsToRemove) {
				// remove BGP Peer
				if err := m.client.Delete(ctx, &peer); err != nil {
					// TODO return error
					klog.V(2).ErrorS(err, "unable to update BGPPeer NodeSelectors")
					return false
				}
				removed = true
			}
		}
	}
	return removed
}

func (m *CRDConfigurer) AddAddressPool(ctx context.Context, add *AddressPool) bool {
	// ignore empty peer; nothing to add
	if add == nil {
		return false
	}
	// go through the pools and see if we have one that matches
	ipAddr := convertToIPAddr(*add)

	pools, err := m.listIPAddressPools(ctx)
	if err != nil {
		// TODO return error
		klog.V(2).ErrorS(err, "unable to retrieve a list of IPAddressPool")
		return false
	}

	// go through the pools and see if we have one that matches
	for _, pool := range pools.Items {
		// MetalLB cannot handle two pools with everything the same
		// except for the name. So if we have two pools that are identical except for the name:
		// - if the name is the same, do nothing
		// - if the name is different, modify the name on the first to encompass both
		if reflect.DeepEqual(pool.Spec, ipAddr.Spec) {
			if pool.Name == ipAddr.Name {
				// they were equal, so we found a matcher
				return false
			}
		}
	}

	// if we got here, none matched exactly, so add it
	err = m.client.Create(ctx, &ipAddr)
	if err != nil {
		klog.V(2).ErrorS(err, "unable to add new IPAddressPool")
		return false
	}

	// update BGP Advertisements IPAddressPools list
	bgpList, err := m.listBGPAdvertisements(ctx)
	if err != nil {
		klog.V(2).ErrorS(err, "unable to retrieve bgp advs")
		return false
	}

	for _, bgp := range bgpList.Items {
		pools := bgp.Spec.IPAddressPools
		if !containsAddress(pools, ipAddr.Name) {
			pools = append(pools, ipAddr.Name)
		}

		if err := m.updateBGPAdvertisementPools(ctx, bgp, pools); err != nil {
			klog.V(2).Infof("error updating BGPAdvertisement pool list: %v", err)
			// return fmt.Errorf("failed to update bgp adv: %w", err)
			return false
		}
	}

	return true
}

func (m *CRDConfigurer) RemoveAddressPoolByAddress(ctx context.Context, addr string) {
	if addr == "" {
		return
	}

	// go through the pools and see if we have a match
	// pools := make([]metallbv1beta1.IPAddressPool, 0)
	// go through the pools and see if we have one with our hostname
	pools, err := m.listIPAddressPools(ctx)
	if err != nil {
		// TODO return error
		klog.V(2).ErrorS(err, "unable to retrieve a list of IPAddressPool")
		return
	}
	for _, pool := range pools.Items {
		if containsAddress(pool.Spec.Addresses, addr) {
			if err := m.client.Delete(ctx, &pool); err != nil {
				// TODO return error
				klog.V(2).ErrorS(err, "unable to delete pool %s", pool.Name)
			}
		}
	}
}

func (m *CRDConfigurer) Get(ctx context.Context) error { return nil }

func (m *CRDConfigurer) Update(ctx context.Context) error { return nil }

func (m *CRDConfigurer) listBGPPeers(ctx context.Context) (metallbv1beta1.BGPPeerList, error) {
	var err error
	peerList := metallbv1beta1.BGPPeerList{}
	m.client.List(ctx, &peerList)
	return peerList, err
}

func (m *CRDConfigurer) UpdateBGPPeerNodeSelectors(ctx context.Context, peer metallbv1beta1.BGPPeer, ns []metallbv1beta1.NodeSelector) error {
	patch, _ := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"nodeSelectors": ns,
		},
	})

	return m.client.Patch(ctx, &peer, client.RawPatch(k8stypes.MergePatchType, patch))
}

func (m *CRDConfigurer) listIPAddressPools(ctx context.Context) (metallbv1beta1.IPAddressPoolList, error) {
	var err error
	poolList := metallbv1beta1.IPAddressPoolList{}
	m.client.List(ctx, &poolList, client.InNamespace(m.namespace))
	return poolList, err
}

func (m *CRDConfigurer) updateBGPAdvertisementPools(ctx context.Context, bgp metallbv1beta1.BGPAdvertisement, pools []string) error {
	patch, _ := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"ipAddressPools": pools,
		},
	})

	err := m.client.Patch(ctx, &bgp, client.RawPatch(k8stypes.MergePatchType, patch))
	return err
}

func (m *CRDConfigurer) listBGPAdvertisements(ctx context.Context) (metallbv1beta1.BGPAdvertisementList, error) {
	var err error
	bgpAdvList := metallbv1beta1.BGPAdvertisementList{}
	m.client.List(ctx, &bgpAdvList, client.InNamespace(m.namespace))
	return bgpAdvList, err
}

func containsAddress(p []string, a string) bool {
	for i := range p {
		if p[i] == a {
			return true
		}
	}
	return false
}

func convertToIPAddr(addr AddressPool) metallbv1beta1.IPAddressPool {
	return metallbv1beta1.IPAddressPool{
		Spec: metallbv1beta1.IPAddressPoolSpec{
			Addresses:     addr.Addresses,
			AutoAssign:    addr.AutoAssign,
			AvoidBuggyIPs: addr.AvoidBuggyIPs,
		},
	}
}

func convertToBGPPeer(peer Peer) metallbv1beta1.BGPPeer {
	time, _ := time.ParseDuration(peer.HoldTime)
	return metallbv1beta1.BGPPeer{
		Spec: metallbv1beta1.BGPPeerSpec{
			MyASN:      peer.MyASN,
			ASN:        peer.ASN,
			Address:    peer.Addr,
			SrcAddress: peer.SrcAddr,
			Port:       peer.Port,
			HoldTime:   metav1.Duration{time},
			// KeepaliveTime: ,
			// RouterID: peer.RouterID,
			NodeSelectors: convertToNodeSelectors(peer.NodeSelectors),
			Password:      peer.Password,
			// BFDProfile:
			// EBGPMultiHop:
		},
	}
}

func convertToNodeSelectors(legacy NodeSelectors) []metallbv1beta1.NodeSelector {
	nodeSelectors := make([]metallbv1beta1.NodeSelector, 0)
	for _, l := range legacy {
		nodeSelectors = append(nodeSelectors, convertToNodeSelector(l))
	}
	return nodeSelectors
}

func convertToNodeSelector(legacy NodeSelector) metallbv1beta1.NodeSelector {
	return metallbv1beta1.NodeSelector{
		MatchLabels:      legacy.MatchLabels,
		MatchExpressions: convertToMatchExpressions(legacy.MatchExpressions),
	}
}

func convertToMatchExpressions(legacy []SelectorRequirements) []metallbv1beta1.MatchExpression {
	matchExpressions := make([]metallbv1beta1.MatchExpression, 0)
	for _, l := range legacy {
		new := metallbv1beta1.MatchExpression{
			Key:      l.Key,
			Operator: l.Operator,
			Values:   l.Values,
		}
		matchExpressions = append(matchExpressions, new)
	}
	return matchExpressions
}

// EqualIgnoreService return true if a peer is identical except
// for the special service label. Will only check for it in the current Peer
// p, and not the "other" peer in the parameter.
func peerEqualIgnoreService(p, o metallbv1beta1.BGPPeerSpec) bool {
	// not matched if any field is mismatched
	if p.MyASN != o.MyASN || p.ASN != o.ASN || p.Address != o.Address || p.Port != o.Port || p.HoldTime != o.HoldTime ||
		p.Password != o.Password || p.RouterID != o.RouterID {
		return false
	}

	var pns, ons []metallbv1beta1.NodeSelector = p.NodeSelectors, o.NodeSelectors
	return peerNsEqualIgnoreService(pns, ons)
}

// EqualIgnoreService return true if two sets of NodeSelectors are identical,
// except that the NodeSelector containing the special service label is ignored
// in the first one.
func peerNsEqualIgnoreService(pns, ons []metallbv1beta1.NodeSelector) bool {
	// create a new NodeSelectors that ignores a NodeSelector
	// whose sole entry is a MatchLabels for the special service one.
	var ns1, os1 []metallbv1beta1.NodeSelector
	for _, ns := range pns {
		if len(ns.MatchLabels) <= 2 && len(ns.MatchExpressions) == 0 && (ns.MatchLabels[serviceNameKey] != "" || ns.MatchLabels[serviceNamespaceKey] != "") {
			continue
		}
		ns1 = append(ns1, ns)
	}
	for _, ns := range ons {
		if len(ns.MatchLabels) <= 2 && len(ns.MatchExpressions) == 0 && (ns.MatchLabels[serviceNameKey] != "" || ns.MatchLabels[serviceNamespaceKey] != "") {
			continue
		}
		os1 = append(os1, ns)
	}
	// not matched if the node selectors are of the wrong length
	if len(ns1) != len(os1) {
		return false
	}

	return reflect.DeepEqual(ns1, os1)
}

func peerAppendServiceToNodeSelectors(svcNamespace, svcName string, nodeSelectors []metallbv1beta1.NodeSelector) ([]metallbv1beta1.NodeSelector, bool) {
	var (
		services = []Resource{
			{Namespace: svcNamespace, Name: svcName},
		}
		selectors []metallbv1beta1.NodeSelector
	)
	for _, ns := range nodeSelectors {
		var namespace, name string
		for k, v := range ns.MatchLabels {
			switch k {
			case serviceNameKey:
				name = v
			case serviceNamespaceKey:
				namespace = v
			}
		}
		// if this was not a service namespace/name selector, just add it
		if name == "" && namespace == "" {
			selectors = append(selectors, ns)
		}
		if name != "" && namespace != "" {
			// if it already had it, nothing to do, nothing change
			if svcNamespace == namespace && svcName == name {
				return nodeSelectors, false
			}
			services = append(services, Resource{Namespace: namespace, Name: name})
		}
	}
	// replace the NodeSelectors with everything except for the services
	nodeSelectors = selectors

	// now add the services
	sort.Sort(Resources(services))

	// if we did not find it, add it
	for _, svc := range services {
		nodeSelectors = append(nodeSelectors, metallbv1beta1.NodeSelector{
			MatchLabels: map[string]string{
				serviceNamespaceKey: svc.Namespace,
				serviceNameKey:      svc.Name,
			},
		})
	}

	// update peer
	return nodeSelectors, true
}

func peerRemoveServiceFromNodeSelectors(svcNamespace, svcName string, nodeSelectors []metallbv1beta1.NodeSelector) ([]metallbv1beta1.NodeSelector, bool, int) {
	var (
		found     bool
		size      int
		services  = []Resource{}
		selectors []metallbv1beta1.NodeSelector
	)
	for _, ns := range nodeSelectors {
		var name, namespace string
		for k, v := range ns.MatchLabels {
			switch k {
			case serviceNameKey:
				name = v
			case serviceNamespaceKey:
				namespace = v
			}
		}
		switch {
		case name == "" && namespace == "":
			selectors = append(selectors, ns)
		case name == svcName && namespace == svcNamespace:
			found = true
		case name != "" && namespace != "" && (name != svcName || namespace != svcNamespace):
			services = append(services, Resource{Namespace: namespace, Name: name})
		}
	}
	// first put back all of the previous selectors except for the services
	nodeSelectors = selectors
	// then add all of the services
	sort.Sort(Resources(services))
	size = len(services)
	for _, svc := range services {
		nodeSelectors = append(nodeSelectors, metallbv1beta1.NodeSelector{
			MatchLabels: map[string]string{
				serviceNamespaceKey: svc.Namespace,
				serviceNameKey:      svc.Name,
			},
		})
	}
	return nodeSelectors, found, size
}
