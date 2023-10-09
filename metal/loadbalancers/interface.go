package loadbalancers

import (
	"context"

	v1 "k8s.io/api/core/v1"
)

type LB interface {
	// AddService add a service with the provided name and IP
	AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []Node, svc *v1.Service, n []*v1.Node, loadBalancerName string) error
	// RemoveService remove service with the given IP
	RemoveService(ctx context.Context, svcNamespace, svcName, ip string, svc *v1.Service) error
	// UpdateService ensure that the nodes handled by the service are correct
	UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []Node) error
	// GetLoadBalancerList returns the load balancer objects
	GetLoadBalancerList(ctx context.Context) ([]string, error)
}
