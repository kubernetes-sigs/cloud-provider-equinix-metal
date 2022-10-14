package metallb

import (
	"context"
	"fmt"
)

type CRDConfigurer struct {
	namespace string // defaults to metallb-system

	// the name of the IPAddressPool, used in metallb.universe.tf/address-pool service annotations
	ipaddresspoolName string // defaults to equinix-metal-ip-address-pool

	bgppeerPrefix string // defaults to equinix-metal-bgp-peer
}

var _ Configurer = (*CRDConfigurer)(nil)

func (*CRDConfigurer) AddPeerByService(add *Peer, svcNamespace, svcName string) bool {
	return false
}
func (*CRDConfigurer) RemovePeersByService(svcNamespace, svcName string) bool { return false }
func (*CRDConfigurer) RemovePeersBySelector(remove *NodeSelector) bool        { return false }
func AddAddressPool(add *AddressPool) bool                                    { return false }
func (*CRDConfigurer) RemoveAddressPoolByAddress(addr string)                 {}
func (*CRDConfigurer) AddAddressPool(add *AddressPool) bool                   { return false }
func (*CRDConfigurer) Get(context.Context) error {
	return fmt.Errorf("Get is not implemented")
}

func (*CRDConfigurer) Update(context.Context) error {
	return fmt.Errorf("Update is not implemented")
}
