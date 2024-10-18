package metallb

import (
	"context"
	"fmt"
	"strings"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultBgpAdvertisement   = "equinix-metal-bgp-adv"
	cpemLabelKey              = "cloud-provider"
	cpemLabelValue            = "equinix-metal"
	svcLabelKeyPrefix         = "service-"
	svcLabelValuePrefix       = "namespace-"
	svcAnnotationSharedPrefix = "shared-"
	metallbAnnotationSharedIP = "metallb.universe.tf/allow-shared-ip" // Not exported as a const from metallb package :(
)

type CRDConfigurer struct {
	namespace string // defaults to metallb-system

	client client.Client
}

var _ Configurer = (*CRDConfigurer)(nil)

func (m *CRDConfigurer) UpdatePeersByService(ctx context.Context, adds *[]Peer, svcNamespace, svcName string) (bool, error) {
	olds, err := m.listBGPPeers(ctx)
	if err != nil {
		return false, err
	}

	news := []metallbv1beta1.BGPPeer{}
	toAdd := make(map[string]metallbv1beta1.BGPPeer)
	for _, add := range *adds {
		peer := convertToBGPPeer(add, m.namespace, svcName)
		news = append(news, peer)
		toAdd[peer.Name] = peer
	}

	// if there is no Peers, add all the new ones
	if len(olds.Items) == 0 {
		for _, n := range news {
			err = m.client.Create(ctx, &n)
			if err != nil {
				return false, fmt.Errorf("unable to add BGPPeer %s: %w", n.GetName(), err)
			}
		}
		return true, nil
	}

	var changed bool
	for _, o := range olds.Items {
		found := false
		for _, n := range news {
			if n.Name == o.GetName() {
				found = true
				// remove from toAdd list
				delete(toAdd, n.Name)
				// update
				patch := client.MergeFrom(o.DeepCopy())
				var update bool
				// update services in node selectors
				if peerAddService(&o, svcNamespace, svcName) {
					update = true
				}
				// check specs
				if !peerSpecEqual(o.Spec, n.Spec) {
					o.Spec = n.Spec
					update = true
				}
				if update {
					err := m.client.Patch(ctx, &o, patch)
					if err != nil {
						return false, fmt.Errorf("unable to update IPAddressPool %s: %w", o.GetName(), err)
					}
					if !changed {
						changed = true
					}
				}
				break
			}
		}
		// if a peer in the config no longer exists for a service,
		// execute RemovePeersByService to update config
		if !found {
			updatedOrRemoved, err := m.updateOrDeletePeerByService(ctx, o, svcNamespace, svcName)
			if err != nil {
				return false, err
			}
			if !changed {
				changed = updatedOrRemoved
			}
		}
	}

	for _, n := range toAdd {
		peerAddService(&n, svcNamespace, svcName)
		err = m.client.Create(ctx, &n)
		if err != nil {
			return false, fmt.Errorf("unable to add BGPPeer %s: %w", n.GetName(), err)
		}
		changed = true
	}

	return changed, nil
}

// RemovePeersByService remove peers from a particular service.
// For any peers that have this service in the services Label, remove
// the service from the label. If there are no services left, remove the
// peer entirely.
func (m *CRDConfigurer) RemovePeersByService(ctx context.Context, svcNamespace, svcName string) (bool, error) {
	var changed bool

	olds, err := m.listBGPPeers(ctx)
	if err != nil {
		return false, err
	}

	for _, o := range olds.Items {
		removed, err := m.updateOrDeletePeerByService(ctx, o, svcNamespace, svcName)
		if err != nil {
			return false, err
		}
		if !changed {
			changed = removed
		}
	}
	return true, nil
}

// AddAddressPool adds an address pool. If a matching pool already exists, do not change anything.
// Returns if anything changed.
func (m *CRDConfigurer) AddAddressPool(ctx context.Context, add *AddressPool, svcNamespace, svcName string) (bool, error) {
	// ignore empty pool; nothing to add
	if add == nil {
		return false, nil
	}

	olds, err := m.listIPAddressPools(ctx)
	if err != nil {
		return false, fmt.Errorf("retrieve a list of IPAddressPools %s %w", m.namespace, err)
	}

	addIPAddr := convertToIPAddr(*add, m.namespace, svcNamespace, svcName)

	svc := corev1.Service{}
	if err = m.client.Get(ctx, client.ObjectKey{Namespace: svcNamespace, Name: svcName}, &svc); err != nil {
		return false, fmt.Errorf("unable to retrieve service: %w", err)
	}

	// go through the pools and see if we have one that matches
	// - if same service name return false
	for _, o := range olds.Items {
		var updateLabels, updateAddresses, updateAnnotations bool
		// if same name check services labels
		if o.GetName() == addIPAddr.GetName() {
			// if service label and key matches
			klog.V(2).Info("found matching address pool")
			if o.Labels[serviceLabelKey(svcName)] == serviceLabelValue(svcNamespace) {
				// if is shared and service exsits in shared annotation
				if k, ok := svc.Annotations[metallbAnnotationSharedIP]; ok {
					if containsSharedService(o.Annotations[sharedAnnotationKey(k)], svcNamespace, svcName) {
						// already exists, and in shared annotation
						return false, nil
					} else {
						updateAnnotations = true
					}
				} else {
					// already exists, and not shared
					return false, nil
				}
			}
			// if we got here, none matched exactly, update labels
			updateLabels = true
		}

		// If we already need to update the annotations, then this is the owning service and it's just adding a shared-ip annotation
		if !updateAnnotations {
			// Otherwise we need to check that the IP is new or can be shared
			for _, addr := range addIPAddr.Spec.Addresses {
				if slices.Contains(o.Spec.Addresses, addr) {
					klog.V(2).Info("found matching ip in other address pool, checking if it can be shared")
					// Check the Service is configured to share the IP
					sharedIpKey, ok := svc.Annotations[metallbAnnotationSharedIP]
					if !ok {
						return false, fmt.Errorf("unable to configure IPAddressPool: requested ip %s already in use and no %s annotation found", addr, metallbAnnotationSharedIP)
					}

					// Check the shared IP key matches the pool annotation
					if _, ok := o.Annotations[sharedAnnotationKey(sharedIpKey)]; !ok {
						return false, fmt.Errorf("unable to configure IPAddressPool: requested ip %s already in use and %s annotation does not match", addr, metallbAnnotationSharedIP)
					}

					updateAnnotations = true
					updateAddresses = true
					break
				}
			}
		}

		if updateLabels || updateAddresses || updateAnnotations {
			// update pool
			patch := client.MergeFrom(o.DeepCopy())
			if updateLabels {
				o.Labels[serviceLabelKey(svcName)] = serviceLabelValue(svcNamespace)
			}
			if updateAddresses {
				// update addreses and remove duplicates
				addresses := append(o.Spec.Addresses, addIPAddr.Spec.Addresses...)
				slices.Sort(addresses)
				o.Spec.Addresses = slices.Compact(addresses)
			}
			if updateAnnotations {
				sharedIpKey := sharedAnnotationKey(svc.Annotations[metallbAnnotationSharedIP])
				if sharedSvcs, ok := o.Annotations[sharedIpKey]; !ok {
					// Safer way to set annotations in case the annotation map itself is nil
					o.SetAnnotations(map[string]string{sharedIpKey: sharedServiceName(svcNamespace, svcName)})
				} else {
					sharedSvcs := strings.Split(sharedSvcs, ",")
					sharedSvcs = append(sharedSvcs, sharedServiceName(svcNamespace, svcName))
					slices.Sort(sharedSvcs)
					sharedSvcs = slices.Compact(sharedSvcs)
					o.Annotations[sharedIpKey] = strings.Join(sharedSvcs, ",")
				}
			}
			err := m.client.Patch(ctx, &o, patch)
			if err != nil {
				return false, fmt.Errorf("unable to update IPAddressPool %s: %w", o.GetName(), err)
			}
			return true, nil
		}
	}

	// if we got here, none matched exactly, so add it
	err = m.client.Create(ctx, &addIPAddr)
	if err != nil {
		return false, fmt.Errorf("unable to add IPAddressPool %s: %w", addIPAddr.GetName(), err)
	}
	
	// - if there's no BGPAdvertisement, create the default BGPAdvertisement
	// - if default BGPAdvertisement exists, update IPAddressPools
	advs, err := m.listBGPAdvertisements(ctx)
	if err != nil {
		return false, err
	}
	if len(advs.Items) == 0 {
		adv := metallbv1beta1.BGPAdvertisement{}
		adv.SetName(defaultBgpAdvertisement)
		adv.SetNamespace(m.namespace)
		adv.SetLabels(map[string]string{cpemLabelKey: cpemLabelValue})
		adv.Spec.IPAddressPools = []string{addIPAddr.Name}
		err = m.client.Create(ctx, &adv)
		if err != nil {
			return false, fmt.Errorf("unable to add default BGPAdvertisement %s: %w", adv.GetName(), err)
		}
	} else {
		for _, adv := range advs.Items {
			if adv.Name == defaultBgpAdvertisement {
				patch := client.MergeFrom(adv.DeepCopy())
				adv.Spec.IPAddressPools = append(adv.Spec.IPAddressPools, addIPAddr.Name)
				err := m.client.Patch(ctx, &adv, patch)
				if err != nil {
					return false, fmt.Errorf("unable to update BGPAdvertisement %s: %w", adv.GetName(), err)
				}
			}
		}
	}
	return true, nil
}

// RemoveFromAddressPool removes a service from a pool by name. If the matching pool is not found, do not change anything
func (m *CRDConfigurer) RemoveFromAddressPool(ctx context.Context, svcNamespace, svcName string) error {
	if svcNamespace == "" || svcName == "" {
		return nil
	}

	olds, err := m.listIPAddressPools(ctx)
	if err != nil {
		return err
	}

	// go through the pools and see if we have a match
	pool := poolName(svcNamespace, svcName)
	for _, o := range olds.Items {
		if slices.ContainsFunc(maps.Keys(o.GetAnnotations()), func(s string) bool {
			return strings.HasPrefix(s, svcAnnotationSharedPrefix) && containsSharedService(o.Annotations[s], svcNamespace, svcName)
		}) {
			// If there are more services sharing this pool, we only need to remove this service from the annotation
			for k, v := range o.GetAnnotations() {
				if strings.HasPrefix(k, svcAnnotationSharedPrefix) && containsSharedService(v, svcNamespace, svcName) {
					svcList := slices.DeleteFunc(strings.Split(v, ","), func(s string) bool {
						return s == sharedServiceName(svcNamespace, svcName)
					})
					if len(svcList) == 0 {
						// No other shared services with this key
						return m.RemoveAddressPool(ctx, o.GetName())
					} else {
						patch := client.MergeFrom(o.DeepCopy())
						delete(o.Labels, serviceLabelKey(svcName))
						o.Annotations[k] = strings.Join(svcList, ",")
						if err = m.client.Patch(ctx, &o, patch); err != nil {
							return fmt.Errorf("unable to update IPAddressPool %s: %w", o.GetName(), err)
						}
						// Other Services still use this IP
						return ErrIPStillInUse
					}

				}
			}
		} else if o.GetName() == pool {
			// Not shared, so just delete the pool
			return m.RemoveAddressPool(ctx, pool)
		}
	}
	return nil
}

// RemoveAddressPool removes a pool by name. If the matching pool does not exist, do not change anything
func (m *CRDConfigurer) RemoveAddressPool(ctx context.Context, pool string) error {
	if pool == "" {
		return nil
	}

	olds, err := m.listIPAddressPools(ctx)
	if err != nil {
		return err
	}

	// go through the pools and see if we have a match
	for _, o := range olds.Items {
		if o.GetName() == pool {
			if err := m.client.Delete(ctx, &o); err != nil {
				return fmt.Errorf("unable to delete pool: %w", err)
			}
			klog.V(2).Info("addressPool removed")
		}
	}

	// TODO (ocobleseqx) if we manage bgpAdvertisements created by users
	// we will also want to check/update/remove pools specified in them
	advs, err := m.listBGPAdvertisements(ctx)
	if err != nil {
		return err
	}
	for _, adv := range advs.Items {
		if adv.Name == defaultBgpAdvertisement {
			for i, p := range adv.Spec.IPAddressPools {
				if p == pool {
					if len(adv.Spec.IPAddressPools) > 1 {
						// there are more pools, just remove pool from the default bgpAdv IPAddressPools list
						patch := client.MergeFrom(adv.DeepCopy())
						adv.Spec.IPAddressPools = slices.Delete(adv.Spec.IPAddressPools, i, i+1)
						err := m.client.Patch(ctx, &adv, patch)
						if err != nil {
							return fmt.Errorf("unable to update BGPAdvertisement %s: %w", adv.GetName(), err)
						}
					} else {
						// no pools left, delete default bgpAdv
						err = m.client.Delete(ctx, &adv)
						if err != nil {
							return fmt.Errorf("unable to delete BGPPeer %s: %w", adv.GetName(), err)
						}
					}
				}
			}
			break
		}
	}
	return nil
}

func (m *CRDConfigurer) listBGPPeers(ctx context.Context) (metallbv1beta1.BGPPeerList, error) {
	var err error
	peerList := metallbv1beta1.BGPPeerList{}
	err = m.client.List(ctx, &peerList, client.MatchingLabels{cpemLabelKey: cpemLabelValue}, client.InNamespace(m.namespace))
	if err != nil {
		err = fmt.Errorf("unable to retrieve a list of BGPPeers: %w", err)
	}
	return peerList, err
}

func (m *CRDConfigurer) listBGPAdvertisements(ctx context.Context) (metallbv1beta1.BGPAdvertisementList, error) {
	var err error
	bgpAdvList := metallbv1beta1.BGPAdvertisementList{}
	err = m.client.List(ctx, &bgpAdvList, client.MatchingLabels{cpemLabelKey: cpemLabelValue}, client.InNamespace(m.namespace))
	if err != nil {
		err = fmt.Errorf("unable to retrieve a list of BGPAdvertisements: %w", err)
	}
	return bgpAdvList, err
}

func (m *CRDConfigurer) listIPAddressPools(ctx context.Context) (metallbv1beta1.IPAddressPoolList, error) {
	var err error
	poolList := metallbv1beta1.IPAddressPoolList{}
	err = m.client.List(ctx, &poolList, client.MatchingLabels{cpemLabelKey: cpemLabelValue}, client.InNamespace(m.namespace))
	if err != nil {
		err = fmt.Errorf("unable to retrieve a list of IPAddressPools: %w", err)
	}
	return poolList, err
}

func (m *CRDConfigurer) updateOrDeletePeerByService(ctx context.Context, o metallbv1beta1.BGPPeer, svcNamespace, svcName string) (bool, error) {
	original := o.DeepCopy()
	// get the services for which this peer works
	peerChanged, size := peerRemoveService(&o, svcNamespace, svcName)

	// if service left update it, otherwise delete peer
	if peerChanged {
		if size > 0 {
			err := m.client.Patch(ctx, &o, client.MergeFrom(original))
			if err != nil {
				return false, fmt.Errorf("unable to update BGPPeer %s: %w", o.GetName(), err)
			}
		} else {
			err := m.client.Delete(ctx, &o)
			if err != nil {
				return false, fmt.Errorf("unable to delete BGPPeer %s: %w", o.GetName(), err)
			}
		}
		return true, nil
	}
	return false, nil
}

func (m *CRDConfigurer) Get(ctx context.Context) error { return nil }

func (m *CRDConfigurer) Update(ctx context.Context) error { return nil }

// RemoveAddressPooByAddress remove a pool by an address name. If the matching pool does not exist, do not change anything
func (m *CRDConfigurer) RemoveAddressPoolByAddress(ctx context.Context, addrName string) error {
	return nil
}
