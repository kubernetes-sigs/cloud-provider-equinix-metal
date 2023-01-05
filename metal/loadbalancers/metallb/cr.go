package metallb

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/exp/slices"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultBgpAdvertisement = "equinix-metal-bgp-adv"
	servicesLabelKey        = "services"
)

type CRDConfigurer struct {
	namespace string // defaults to metallb-system

	bgpAdvertisements []string // defaults to equinix-metal-bgp-adv

	client client.Client
}

var _ Configurer = (*CRDConfigurer)(nil)

//AddPeer
func (m *CRDConfigurer) AddPeer(ctx context.Context, add *Peer) bool {
	// ignore empty peer; nothing to add
	if add == nil {
		return false
	}

	// go through the pools and see if we have one that matches
	newBGPPeer := convertToBGPPeer(*add, m.namespace)

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
		if peerEqual(peer.Spec, newBGPPeer.Spec) {
			return false
		}
	}

	// if we got here, none matched exactly, so add it
	err = m.client.Create(ctx, &newBGPPeer)
	if err != nil {
		klog.V(2).ErrorS(err, "unable to add new BGPPeer")
		return false
	}
	return true
}

func (m *CRDConfigurer) RemovePeersBySelector(ctx context.Context, remove *NodeSelector) (bool, error) {
	if remove == nil {
		return false, nil
	}

	// go through the peers and see if we have a match
	peers, err := m.listBGPPeers(ctx)
	if err != nil {
		return false, fmt.Errorf("unable to retrieve a list of BGPPeers: %w", err)
	}

	nsToRemove := convertToNodeSelector(*remove)
	// go through the peers and see if we have a match
	// peers := make([]Peer, 0)
	var removed bool
	for _, peer := range peers.Items {
		for _, ns := range peer.Spec.NodeSelectors {
			// TODO (ocobleseqx) it seems deepEqual is not working properly
			// may need to be replaced with a field by field comparison
			if reflect.DeepEqual(ns, nsToRemove) {
				// remove BGPPeer
				if err := m.client.Delete(ctx, &peer); err != nil {
					return false, fmt.Errorf("nable to update BGPPeer NodeSelectorss: %w", err)
				}
				removed = true
			}
		}
	}
	return removed, nil
}

func (m *CRDConfigurer) AddAddressPool(ctx context.Context, add *AddressPool) (bool, error) {
	// ignore empty add; nothing to add
	if add == nil {
		return false, nil
	}

	addIpAddr := convertToIPAddr(*add, m.namespace)

	// check if already exists
	pools, err := m.listIPAddressPools(ctx)
	if err != nil {
		return false, fmt.Errorf("unable to retrieve a list of IPAddressPool: %w", err)
	}


	// - if same service name return err
	// - if one of the new addresses exists in a pool, add all them to that pool
	// - update label `services`
	for _, pool := range pools.Items {
		if pool.GetName() == addIpAddr.GetName() {
			//already exists
			return false, nil
		}
		for _, addr := range addIpAddr.Spec.Addresses {
			if slices.Contains(pool.Spec.Addresses, addr) {
				//update addreses
				addresses := appendUnique(pool.Spec.Addresses, addIpAddr.Spec.Addresses...)
				//update labels
				labels := pool.Labels
				if labels[servicesLabelKey] == "" {
					labels[servicesLabelKey] = addIpAddr.Labels[servicesLabelKey]
				} else {
					svcs := strings.Split(labels[servicesLabelKey], ",")
					addIpAddrSvcs := strings.Split(addIpAddr.Labels[servicesLabelKey], ",")
					labels[servicesLabelKey] = strings.Join(appendUnique(svcs, addIpAddrSvcs...), ",")
				}
				//update pool
				patch, _ := json.Marshal(map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": labels,
					},
					"spec": map[string]interface{}{
						"addresses": addresses,
					},
				})

				if err := m.updateIPAddressPool(ctx, pool, patch); err != nil {
					return false, err
				}
				return true, nil
			}
		}
	}

	// if we got here, none matched exactly, so add it
	err = m.client.Create(ctx, &addIpAddr)
	if err != nil {
		return false, fmt.Errorf("unable to add new IPAddressPool: %w", err)
	}

	// update BGPAdvertisement
	advs, err := m.listBGPAdvertisements(ctx)
	if err != nil {
		return false, fmt.Errorf("unable to retrieve a list of BGPAdvertisement: %w", err)
	}

	// get only those specified in the m.bgpAdvertisements list
	filteredBgpAdvs := advs.DeepCopy()
	filteredBgpAdvs.Items = make([]metallbv1beta1.BGPAdvertisement, 0)
	if len(m.bgpAdvertisements) > 0 {
		for _, adv := range advs.Items {
			if !slices.Contains(m.bgpAdvertisements, adv.GetName()) { continue }
			filteredBgpAdvs.Items = append(filteredBgpAdvs.Items, adv)
		}
	}

	if len(filteredBgpAdvs.Items) == 0 {
		// there's no BGPAdvertisement, let's create the default BGPAdvertisement without specifying ipAddressPools
		bgpAdv := metallbv1beta1.BGPAdvertisement{}
		bgpAdv.SetName("equinix-metal-bgp-adv")
		bgpAdv.SetNamespace(m.namespace)
		bgpAdv.Spec.IPAddressPools = []string{ addIpAddr.Name }

		err = m.client.Create(ctx, &bgpAdv)
		if err != nil {
			return false, fmt.Errorf("unable to create default BGPAdvertisement: %w", err)
		}
		// ensure default BGPAdvertisement name is in the bgpAdvertisements list
		if !slices.Contains(m.bgpAdvertisements, bgpAdv.GetName()) {
			m.bgpAdvertisements = append(m.bgpAdvertisements, bgpAdv.GetName())
		}
		return true, nil
	}

	// update existing bgpAdvertisements
	for _, adv := range filteredBgpAdvs.Items {
		if !slices.Contains(adv.Spec.IPAddressPools, addIpAddr.GetName()){
			adv.Spec.IPAddressPools = append(adv.Spec.IPAddressPools, addIpAddr.GetName())
			if err := m.updateBGPAdvertisementPools(ctx, adv, adv.Spec.IPAddressPools); err != nil {
				return false, fmt.Errorf("error updating BGPAdvertisement pool list: %w", err)
			}
		}
	}

	return true, nil
}

func (m *CRDConfigurer) RemoveAddressPoolByAddress(ctx context.Context, addrName string) error {
	if addrName == "" {
		return nil
	}

	// go through the pools and see if we have a match
	// pools := make([]metallbv1beta1.IPAddressPool, 0)
	// go through the pools and see if we have one with our hostname
	pools, err := m.listIPAddressPools(ctx)
	if err != nil {
		return fmt.Errorf("unable to retrieve a list of IPAddressPool: %w", err)
	}
	for _, pool := range pools.Items {
		if pool.GetName() == addrName {
			if err := m.client.Delete(ctx, &pool); err != nil {
				return fmt.Errorf("unable to delete pool: %w", err)
			}
			klog.V(2).Info("config changed, addressPool removed")
			return nil
		}
	}
	return nil
}

func (m *CRDConfigurer) listBGPPeers(ctx context.Context) (metallbv1beta1.BGPPeerList, error) {
	var err error
	peerList := metallbv1beta1.BGPPeerList{}
	m.client.List(ctx, &peerList)
	return peerList, err
}

func (m *CRDConfigurer) listIPAddressPools(ctx context.Context) (metallbv1beta1.IPAddressPoolList, error) {
	var err error
	poolList := metallbv1beta1.IPAddressPoolList{}
	m.client.List(ctx, &poolList, client.InNamespace(m.namespace))
	return poolList, err
}

func (m *CRDConfigurer) updateIPAddressPool(ctx context.Context, addr metallbv1beta1.IPAddressPool, patch []byte) error {
	err := m.client.Patch(ctx, &addr, client.RawPatch(k8stypes.MergePatchType, patch))
	if err != nil {
		return fmt.Errorf("unable to update IPAddressPool %s: %w", addr.GetName(), err)
	}
	return nil
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

// AddPeerByService not longer required
func (m *CRDConfigurer) Get(ctx context.Context) error { return nil }

// AddPeerByService not longer required
func (m *CRDConfigurer) Update(ctx context.Context) error { return nil }

// AddPeerByService not longer required
func (m *CRDConfigurer) AddPeerByService(ctx context.Context, add *Peer, svcNamespace, svcName string) bool { return false }

// RemovePeersByService not longer required
func (m *CRDConfigurer) RemovePeersByService(ctx context.Context, svcNamespace, svcName string) bool { return false }
