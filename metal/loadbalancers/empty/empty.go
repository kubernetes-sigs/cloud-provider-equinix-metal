// empty loadbalancer that does nothing, but exists to enable bgp functionality
package empty

import (
	"context"

	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	"k8s.io/client-go/kubernetes"
)

type LB struct {
}

func NewLB(k8sclient kubernetes.Interface, config string) *LB {
	return &LB{}
}

func (l *LB) AddService(ctx context.Context, svc, ip string, nodes []loadbalancers.Node) error {
	return nil
}

func (l *LB) RemoveService(ctx context.Context, svc, ip string) error {
	return nil
}

func (l *LB) UpdateService(ctx context.Context, svc string, nodes []loadbalancers.Node) error {
	return nil
}
