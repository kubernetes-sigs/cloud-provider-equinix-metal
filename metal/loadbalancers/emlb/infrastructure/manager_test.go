package infrastructure

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	manager := NewManager("test-token", "test-project", "da")

	if manager == nil {
		t.Fatal("expected manager to be created, got nil")
	}

	if manager.metro != "da" {
		t.Errorf("expected metro to be 'da', got '%s'", manager.metro)
	}

	if manager.projectID != "test-project" {
		t.Errorf("expected project ID to be 'test-project', got '%s'", manager.projectID)
	}
}

func TestManagerFields(t *testing.T) {
	manager := &Manager{
		metro:     "da",
		projectID: "test-project",
	}

	if manager.metro != "da" {
		t.Errorf("expected metro to be 'da', got '%s'", manager.metro)
	}

	if manager.projectID != "test-project" {
		t.Errorf("expected project ID to be 'test-project', got '%s'", manager.projectID)
	}
}

func TestGetMetro(t *testing.T) {
	manager := &Manager{
		metro: "ny",
	}

	metro := manager.GetMetro()
	if metro != "ny" {
		t.Errorf("expected metro to be 'ny', got '%s'", metro)
	}
}

func TestLBMetros(t *testing.T) {
	if len(LBMetros) == 0 {
		t.Error("expected LBMetros to contain values")
	}

	if LBMetros["da"] == "" {
		t.Error("expected da metro to have a location ID")
	}

	if LBMetros["ny"] == "" {
		t.Error("expected ny metro to have a location ID")
	}

	if LBMetros["sv"] == "" {
		t.Error("expected sv metro to have a location ID")
	}
}
