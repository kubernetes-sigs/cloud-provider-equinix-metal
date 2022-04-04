package loadbalancers

import (
	"context"
)

type LB interface {
	// AddService add a service with the provided name and IP
	AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []Node) error
	// RemoveService remove service with the given IP
	RemoveService(ctx context.Context, svcNamespace, svcName, ip string) error
	// UpdateService ensure that the nodes handled by the service are correct
	UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []Node) error
}
