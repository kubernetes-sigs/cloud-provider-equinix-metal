// Implementation of Equinix Metal Load Balancer
package emlb

import (
	"context"

	lbaas "github.com/equinix/cloud-provider-equinix-metal/internal/lbaas/v1"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	"k8s.io/client-go/kubernetes"
)

type LB struct {
	controller           *controller
	loadBalancerLocation *lbaas.LoadBalancerLocation
}

func NewLB(k8sclient kubernetes.Interface, config string) *LB {
	// Parse config for Equinix Metal Load Balancer
	// An example config using Dallas as the location would look like
	// The format is emlb://<location>
	// it may have an extra slash at the beginning or end, so get rid of it

	metalAPIKey := "TODO"

	lb := &LB{}
	lb.loadBalancerLocation.Id = &config

	lb.controller = NewController(metalAPIKey)
	return lb
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node) error {
	// 1. Gather the properties we need: Metal API key, port number(s), cluster name(?), target IP(s?)
	additionalProperties := map[string]string{}

	// 2. Create the infrastructure (what do we need to return here?  lb name and/or ID? anything else?)
	_, err := l.controller.createLoadBalancer(ctx, additionalProperties)

	if err != nil {
		return err
	}

	// 3. Add the annotations
	return nil
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string) error {
	// 1. Gather the properties we need: Metal API key, port number(s), cluster name(?), target IP(s?)
	loadBalancerId := "TODO"
	additionalProperties := map[string]string{}

	// 2. Delete the infrastructure (do we need to return anything here?)
	_, err := l.controller.deleteLoadBalancer(ctx, loadBalancerId, additionalProperties)

	if err != nil {
		return err
	}

	// 3. Remove the annotations

	return nil
}

func (l *LB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node) error {
	// 1. Gather the properties we need: Metal API key, port number(s), cluster name(?), target IP(s?)
	loadBalancerId := "TODO"
	additionalProperties := map[string]string{}

	// 2. Update infrastructure change (do we need to return anything here? or are all changes reflected by properties from [1]?)
	_, err := l.controller.updateLoadBalancer(ctx, loadBalancerId, additionalProperties)

	if err != nil {
		return err
	}

	// 3. Update the annotations

	return nil
}
