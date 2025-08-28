package testing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metal "github.com/equinix/equinix-sdk-go/services/metalv1"
	"github.com/google/uuid"
)

func TestNewMockMetalServer(t *testing.T) {
	server := NewMockMetalServer(t)

	if server == nil {
		t.Fatal("expected server to be created, got nil")
	}

	if server.DeviceStore == nil {
		t.Error("expected DeviceStore to be initialized")
	}

	if server.ProjectStore == nil {
		t.Error("expected ProjectStore to be initialized")
	}
}

func TestMockMetalServerCreateHandler(t *testing.T) {
	server := NewMockMetalServer(t)
	handler := server.CreateHandler()

	if handler == nil {
		t.Fatal("expected handler to be created, got nil")
	}
}

func TestMockMetalServerListDevicesHandler(t *testing.T) {
	server := NewMockMetalServer(t)
	projectID := uuid.New().String()
	deviceID := uuid.New().String()

	device := &metal.Device{
		Id:       &deviceID,
		Hostname: stringPtr("test-device"),
	}

	server.ProjectStore[projectID] = struct {
		Devices    []*metal.Device
		BgpEnabled bool
	}{
		Devices:    []*metal.Device{device},
		BgpEnabled: false,
	}

	handler := server.CreateHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/projects/" + projectID + "/devices")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var response struct {
		Devices []*metal.Device `json:"devices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(response.Devices))
		return
	}

	if response.Devices[0].GetHostname() != "test-device" {
		t.Errorf("expected hostname 'test-device', got '%s'", response.Devices[0].GetHostname())
	}
}

func TestMockMetalServerGetDeviceHandler(t *testing.T) {
	server := NewMockMetalServer(t)
	deviceID := uuid.New().String()

	device := &metal.Device{
		Id:       &deviceID,
		Hostname: stringPtr("test-device"),
	}

	server.DeviceStore[deviceID] = device

	handler := server.CreateHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/devices/" + deviceID)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var response metal.Device
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.GetHostname() != "test-device" {
		t.Errorf("expected hostname 'test-device', got '%s'", response.GetHostname())
	}
}

func TestMockMetalServerGetDeviceHandlerNotFound(t *testing.T) {
	server := NewMockMetalServer(t)
	deviceID := uuid.New().String()

	handler := server.CreateHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/devices/" + deviceID)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestMockMetalServerCreateBGPHandler(t *testing.T) {
	server := NewMockMetalServer(t)
	projectID := uuid.New().String()

	server.ProjectStore[projectID] = struct {
		Devices    []*metal.Device
		BgpEnabled bool
	}{
		Devices:    []*metal.Device{},
		BgpEnabled: false,
	}

	handler := server.CreateHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/projects/"+projectID+"/bgp-configs", "application/json", nil)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	projectData := server.ProjectStore[projectID]
	if !projectData.BgpEnabled {
		t.Error("expected BGP to be enabled")
	}
}

func stringPtr(s string) *string {
	return &s
}
