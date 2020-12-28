// empty loadbalancer that does nothing, but exists to enable bgp functionality
package empty

import (
	"context"

	"github.com/packethost/packet-ccm/packet/loadbalancers"
	"k8s.io/client-go/kubernetes"
)

type LB struct {
}

func NewLB(k8sclient kubernetes.Interface, config string) *LB {
	return &LB{}
}

func (l *LB) AddService(ctx context.Context, svc, ip string) error {
	return nil
}

func (l *LB) RemoveService(ctx context.Context, ip string) error {
	return nil
}

func (l *LB) SyncServices(ctx context.Context, ips map[string]bool) error {
	return nil
}

// AddNode add a node with the provided name, srcIP, and bgp information
func (l *LB) AddNode(ctx context.Context, nodeName string, localASN, peerASN int, srcIP string, peers ...string) error {
	return nil
}

// RemoveNode remove a node with the provided name
func (l *LB) RemoveNode(ctx context.Context, nodeName string) error {
	return nil
}

// SyncNodes ensure that the list of nodes is only those with the matched names
func (l *LB) SyncNodes(ctx context.Context, nodes map[string]loadbalancers.Node) error {
	return nil
}
