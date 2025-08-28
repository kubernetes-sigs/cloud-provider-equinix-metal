package loadbalancers

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestNodeStruct(t *testing.T) {
	node := Node{
		Name:     "test-node",
		SourceIP: "192.168.1.1",
		LocalASN: 65000,
		PeerASN:  65001,
		Password: "test-password",
		Peers:    []string{"peer1", "peer2"},
	}

	if node.Name != "test-node" {
		t.Errorf("expected Name to be 'test-node', got '%s'", node.Name)
	}

	if node.SourceIP != "192.168.1.1" {
		t.Errorf("expected SourceIP to be '192.168.1.1', got '%s'", node.SourceIP)
	}

	if node.LocalASN != 65000 {
		t.Errorf("expected LocalASN to be 65000, got %d", node.LocalASN)
	}

	if node.PeerASN != 65001 {
		t.Errorf("expected PeerASN to be 65001, got %d", node.PeerASN)
	}

	if node.Password != "test-password" {
		t.Errorf("expected Password to be 'test-password', got '%s'", node.Password)
	}

	if len(node.Peers) != 2 {
		t.Errorf("expected 2 peers, got %d", len(node.Peers))
	}

	if node.Peers[0] != "peer1" {
		t.Errorf("expected first peer to be 'peer1', got '%s'", node.Peers[0])
	}

	if node.Peers[1] != "peer2" {
		t.Errorf("expected second peer to be 'peer2', got '%s'", node.Peers[1])
	}
}

func TestNodeEmptyStruct(t *testing.T) {
	node := Node{}

	if node.Name != "" {
		t.Errorf("expected empty Name, got '%s'", node.Name)
	}

	if node.SourceIP != "" {
		t.Errorf("expected empty SourceIP, got '%s'", node.SourceIP)
	}

	if node.LocalASN != 0 {
		t.Errorf("expected LocalASN to be 0, got %d", node.LocalASN)
	}

	if node.PeerASN != 0 {
		t.Errorf("expected PeerASN to be 0, got %d", node.PeerASN)
	}

	if node.Password != "" {
		t.Errorf("expected empty Password, got '%s'", node.Password)
	}

	if node.Peers != nil {
		t.Errorf("expected nil Peers, got %v", node.Peers)
	}
}

func TestLBInterface(t *testing.T) {
	// This test verifies that the LB interface is properly defined
	// The actual implementations are tested in their respective packages
	var _ LB = (*mockLB)(nil)
}

type mockLB struct{}

func (m *mockLB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []Node, svc *v1.Service, n []*v1.Node, loadBalancerName string) error {
	return nil
}

func (m *mockLB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string, svc *v1.Service) error {
	return nil
}

func (m *mockLB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []Node, svc *v1.Service, n []*v1.Node) error {
	return nil
}

func (m *mockLB) GetLoadBalancer(ctx context.Context, clusterName string, svc *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	return nil, false, nil
}
