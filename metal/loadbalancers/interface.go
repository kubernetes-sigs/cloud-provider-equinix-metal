package loadbalancers

import (
	"context"
)

type LB interface {
	// AddNode add a node with the provided name, srcIP, and bgp information
	AddNode(ctx context.Context, nodeName string, localASN, peerASN int, pass string, srcIP string, peers ...string) error
	// RemoveNode remove a node with the provided name
	RemoveNode(ctx context.Context, nodeName string) error
	// SyncNodes ensure that the list of nodes is only those with the matched names
	SyncNodes(ctx context.Context, nodes map[string]Node) error
	// AddService add a service with the provided name and IP
	AddService(ctx context.Context, svc, ip string) error
	// RemoveService remove service with the given IP
	RemoveService(ctx context.Context, ip string) error
	// SyncServices ensure that the list of services is only those with the matched IPs
	SyncServices(ctx context.Context, ips map[string]bool) error
}
