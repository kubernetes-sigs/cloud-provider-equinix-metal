package kubevip

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewLB(t *testing.T) {
	k8sclient := fake.NewSimpleClientset()
	lb := NewLB(k8sclient, "test-config")

	if lb == nil {
		t.Fatal("expected LB to be created, got nil")
	}
}

func TestKubeVIPGetLoadBalancer(t *testing.T) {
	k8sclient := fake.NewSimpleClientset()
	lb := NewLB(k8sclient, "test-config")

	ctx := context.Background()
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	status, exists, err := lb.GetLoadBalancer(ctx, "test-cluster", service)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected load balancer to not exist")
	}
	if status != nil {
		t.Error("expected status to be nil when load balancer doesn't exist")
	}
}

func TestKubeVIPAddService(t *testing.T) {
	k8sclient := fake.NewSimpleClientset()
	lb := NewLB(k8sclient, "test-config")

	ctx := context.Background()
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	err := lb.AddService(ctx, "default", "test-service", "192.168.1.1", nil, service, nil, "test-lb")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKubeVIPRemoveService(t *testing.T) {
	k8sclient := fake.NewSimpleClientset()
	lb := NewLB(k8sclient, "test-config")

	ctx := context.Background()
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	err := lb.RemoveService(ctx, "default", "test-service", "192.168.1.1", service)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKubeVIPUpdateService(t *testing.T) {
	k8sclient := fake.NewSimpleClientset()
	lb := NewLB(k8sclient, "test-config")

	ctx := context.Background()
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	err := lb.UpdateService(ctx, "default", "test-service", nil, service, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

