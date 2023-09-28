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
	/*
		1. Gather the properties we need: Metal API key, port number(s), cluster name(?), target IP(s?)
		What we need here is:
			- The NodePort (for first pass we will use the same port on the LB, so if NodePort is 8000 we use 8000 on LB)
			- The public IPs of the nodes on which the service is running
	*/
	additionalProperties := map[string]string{}

	// 2. Create the infrastructure (what do we need to return here?  lb name and/or ID? anything else?)
	_, err := l.controller.createLoadBalancer(ctx, additionalProperties)

	if err != nil {
		return err
	}

	/*
		3. Add the annotations
			- ID of the load balancer
			- Name of the load balancer
			- Metro of the load balancer
			- IP address of the load balancer
			- Listener port that this service is using
	*/
	return nil
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string) error {
	// 1. Gather the properties we need: ID of load balancer
	loadBalancerId := "TODO"
	additionalProperties := map[string]string{}

	// 2. Delete the infrastructure (do we need to return anything here?)
	_, err := l.controller.deleteLoadBalancer(ctx, loadBalancerId, additionalProperties)

	if err != nil {
		return err
	}

	// 3. No need to remove the annotations because the annotated object was deleted

	return nil
}

func (l *LB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node) error {
	/*
		1. Gather the properties we need:
			- load balancer ID
			- NodePort
			- Public IP addresses of the nodes on which the target pods are running
	*/
	loadBalancerId := "TODO"
	additionalProperties := map[string]string{}

	// 2. Update infrastructure change (do we need to return anything here? or are all changes reflected by properties from [1]?)
	_, err := l.controller.updateLoadBalancer(ctx, loadBalancerId, additionalProperties)

	if err != nil {
		return err
	}

	/*
		3. Update the annotations
			- Listener port that this service is using
	*/

	return nil
}
