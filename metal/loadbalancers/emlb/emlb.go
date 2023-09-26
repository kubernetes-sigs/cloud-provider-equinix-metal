// Implementation of Equinix Metal Load Balancer
package emlb

import (
	"context"

	lbaas "github.com/equinix/cloud-provider-equinix-metal/internal/lbaas/v1"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	"k8s.io/client-go/kubernetes"
)

type LB struct {
	client               *lbaas.APIClient
	loadBalancerLocation *lbaas.LoadBalancerLocation
}

func NewLB(k8sclient kubernetes.Interface, config string) *LB {
	// Parse config for Equinix Metal Load Balancer
	// An example config using Dallas as the location would look like
	// The format is emlb://<location>
	// it may have an extra slash at the beginning or end, so get rid of it

	lb := &LB{}
	emlbConfig := lbaas.NewConfiguration()
	lb.client = lbaas.NewAPIClient(emlbConfig)
	lb.loadBalancerLocation.Id = &config
	return lb
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node) error {
	return nil
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string) error {
	return nil
}

func (l *LB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node) error {
	return nil
}
