package testing

import (
	"encoding/json"
	"net/http"
	"testing"

	metal "github.com/equinix/equinix-sdk-go/services/metalv1"
	"github.com/gorilla/mux"
)

type MockMetalServer struct {
	DeviceStore  map[string]*metal.Device
	ProjectStore map[string]struct {
		Devices    []*metal.Device
		BgpEnabled bool
	}

	T *testing.T
}

func NewMockMetalServer(t *testing.T) *MockMetalServer {
	return &MockMetalServer{
		DeviceStore: map[string]*metal.Device{},
		ProjectStore: map[string]struct {
			Devices    []*metal.Device
			BgpEnabled bool
		}{},
		T: t,
	}
}

func (s *MockMetalServer) CreateHandler() http.Handler {
	r := mux.NewRouter()
	// create a BGP config for a project
	r.HandleFunc("/projects/{projectID}/bgp-configs", s.createBGPHandler).Methods("POST")
	// get all devices for a project
	r.HandleFunc("/projects/{projectID}/devices", s.listDevicesHandler).Methods("GET")
	// get a single device
	r.HandleFunc("/devices/{deviceID}", s.getDeviceHandler).Methods("GET")
	// handle metadata requests
	return r
}

func (s *MockMetalServer) listDevicesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["projectID"]

	data := s.ProjectStore[projectID]
	devices := data.Devices
	var resp = struct {
		Devices []*metal.Device `json:"devices"`
	}{
		Devices: devices,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		s.T.Fatal(err.Error())
	}
}

// get information about a specific device
func (s *MockMetalServer) getDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	volID := vars["deviceID"]
	dev := s.DeviceStore[volID]
	w.Header().Set("Content-Type", "application/json")
	if dev != nil {
		err := json.NewEncoder(w).Encode(dev)
		if err != nil {
			s.T.Fatal(err)
		}
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

// createBGPHandler enable BGP for a project
func (s *MockMetalServer) createBGPHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["projectID"]
	projectData := s.ProjectStore[projectID]
	projectData.BgpEnabled = true
	s.ProjectStore[projectID] = projectData
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
}
