package v1

import (
	"testing"
)

func TestNewAPIClient(t *testing.T) {
	cfg := NewConfiguration()
	client := NewAPIClient(cfg)

	if client == nil {
		t.Fatal("expected client to be created, got nil")
	}

	if client.LoadBalancersApi == nil {
		t.Error("expected LoadBalancersApi to be initialized")
	}

	if client.OriginsApi == nil {
		t.Error("expected OriginsApi to be initialized")
	}

	if client.PoolsApi == nil {
		t.Error("expected PoolsApi to be initialized")
	}

	if client.PortsApi == nil {
		t.Error("expected PortsApi to be initialized")
	}

	if client.ProjectsApi == nil {
		t.Error("expected ProjectsApi to be initialized")
	}
}

func TestConfiguration(t *testing.T) {
	cfg := NewConfiguration()

	if cfg == nil {
		t.Fatal("expected configuration to be created, got nil")
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"-456", -456, false},
		{"abc", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		result, err := atoi(tt.input)
		if tt.hasError && err == nil {
			t.Errorf("expected error for input '%s', got none", tt.input)
		}
		if !tt.hasError && err != nil {
			t.Errorf("unexpected error for input '%s': %v", tt.input, err)
		}
		if !tt.hasError && result != tt.expected {
			t.Errorf("expected %d for input '%s', got %d", tt.expected, tt.input, result)
		}
	}
}
