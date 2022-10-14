package metallb

import (
	"context"
	"fmt"
)

type MetalLBCRDConfigurer struct {
	namespace string // defaults to metallb-system

	// the name of the IPAddressPool, used in metallb.universe.tf/address-pool service annotations
	ipaddresspoolName string // defaults to equinix-metal-ip-address-pool

	bgppeerPrefix string // defaults to equinix-metal-bgp-peer
}

var _ MetalLBConfigurer = (*MetalLBCRDConfigurer)(nil)

func (*MetalLBCRDConfigurer) AddPeerByService(add *Peer, svcNamespace, svcName string) bool {
	return false
}
func (*MetalLBCRDConfigurer) RemovePeersByService(svcNamespace, svcName string) bool { return false }
func (*MetalLBCRDConfigurer) RemovePeersBySelector(remove *NodeSelector) bool        { return false }
func AddAddressPool(add *AddressPool) bool                                           { return false }
func (*MetalLBCRDConfigurer) RemoveAddressPoolByAddress(addr string)                 {}
func (*MetalLBCRDConfigurer) AddAddressPool(add *AddressPool) bool                   { return false }
func (*MetalLBCRDConfigurer) Get(context.Context) error {
	return fmt.Errorf("Get is not implemented")
}

func (*MetalLBCRDConfigurer) Update(context.Context) error {
	return fmt.Errorf("Update is not implemented")
}
