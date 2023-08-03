// kubevip loadbalancer that does nothing, but exists to enable bgp functionality
package kubevip

import (
	"context"

	"github.com/kubernetes-sigs/cloud-provider-equinix-metal/metal/loadbalancers"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type LB struct{}

var _ loadbalancers.LB = (*LB)(nil)

func NewLB(k8sclient kubernetes.Interface, config string) *LB {
	return &LB{}
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node, svc *v1.Service, n []*v1.Node, loadBalancerName string) error {
	return nil
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string, svc *v1.Service) error {
	return nil
}

func (l *LB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node, svc *v1.Service, n []*v1.Node) error {
	return nil
}

func (l *LB) GetLoadBalancer(ctx context.Context, clusterName string, svc *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	// TODO
	return nil, false, nil
}
