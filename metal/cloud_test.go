package metal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	metal "github.com/equinix/equinix-sdk-go/services/metalv1"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	clientset "k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/component-base/version"
)

const (
	token           = "12345678"
	nodeName        = "ccm-test"
	validRegionCode = "ME"
	validRegionName = "Metro"
	validZoneCode   = "ewr1"
)

type MockMetalServer struct {
	DeviceStore  map[string]*metal.Device
	ProjectStore map[string]struct {
		Devices    []*metal.Device
		BgpEnabled bool
	}

	T *testing.T
}

// mockControllerClientBuilder mock implementation of https://pkg.go.dev/k8s.io/cloud-provider#ControllerClientBuilder
// so we can pass it to cloud.Initialize()
type mockControllerClientBuilder struct{}

func (m mockControllerClientBuilder) Config(name string) (*restclient.Config, error) {
	return &restclient.Config{}, nil
}

func (m mockControllerClientBuilder) ConfigOrDie(name string) *restclient.Config {
	return &restclient.Config{}
}

func (m mockControllerClientBuilder) Client(name string) (clientset.Interface, error) {
	return k8sfake.NewSimpleClientset(), nil
}

func (m mockControllerClientBuilder) ClientOrDie(name string) clientset.Interface {
	return k8sfake.NewSimpleClientset()
}

// create a valid cloud with a client
func testGetValidCloud(t *testing.T, LoadBalancerSetting string) (*cloud, *MockMetalServer) {
	mockServer := &MockMetalServer{
		DeviceStore: map[string]*metal.Device{},
		ProjectStore: map[string]struct {
			Devices    []*metal.Device
			BgpEnabled bool
		}{},
	}
	// mock endpoint so our client can handle it
	ts := httptest.NewServer(mockServer.CreateHandler())

	url, _ := url.Parse(ts.URL)
	urlString := url.String()

	client := constructClient(token, &urlString)

	// now just need to create a client
	config := Config{
		ProjectID:           uuid.New().String(),
		LoadBalancerSetting: LoadBalancerSetting,
	}
	c, _ := newCloud(config, client)
	ccb := &mockControllerClientBuilder{}
	c.Initialize(ccb, nil)

	return c.(*cloud), mockServer
}

func TestLoadBalancerDefaultDisabled(t *testing.T) {
	vc, _ := testGetValidCloud(t, "")
	response, supported := vc.LoadBalancer()
	var (
		expectedSupported = false
		expectedResponse  = response
	)
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}
}

func TestLoadBalancerMetalLB(t *testing.T) {
	t.Skip("Test needs a k8s client to work")
	vc, _ := testGetValidCloud(t, "metallb:///metallb-system/config")
	response, supported := vc.LoadBalancer()
	var (
		expectedSupported = true
		expectedResponse  = response
	)
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}
}

func TestLoadBalancerEmpty(t *testing.T) {
	t.Skip("Test needs a k8s client to work")
	vc, _ := testGetValidCloud(t, "empty://")
	response, supported := vc.LoadBalancer()
	var (
		expectedSupported = true
		expectedResponse  = response
	)
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}
}

func TestLoadBalancerKubeVIP(t *testing.T) {
	t.Skip("Test needs a k8s client to work")
	vc, _ := testGetValidCloud(t, "kube-vip://")
	response, supported := vc.LoadBalancer()
	var (
		expectedSupported = true
		expectedResponse  = response
	)
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}
}

func TestInstances(t *testing.T) {
	vc, _ := testGetValidCloud(t, "")
	response, supported := vc.Instances()
	expectedSupported := false
	expectedResponse := cloudprovider.Instances(nil)
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}
}

func TestClusters(t *testing.T) {
	vc, _ := testGetValidCloud(t, "")
	response, supported := vc.Clusters()
	var (
		expectedSupported = false
		expectedResponse  cloudprovider.Clusters // defaults to nil
	)
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}
}

func TestRoutes(t *testing.T) {
	vc, _ := testGetValidCloud(t, "")
	response, supported := vc.Routes()
	var (
		expectedSupported = false
		expectedResponse  cloudprovider.Routes // defaults to nil
	)
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}
}

func TestProviderName(t *testing.T) {
	vc, _ := testGetValidCloud(t, "")
	name := vc.ProviderName()
	if name != ProviderName {
		t.Errorf("returned %s instead of expected %s", name, ProviderName)
	}
}

func TestHasClusterID(t *testing.T) {
	vc, _ := testGetValidCloud(t, "")
	cid := vc.HasClusterID()
	expectedCid := true
	if cid != expectedCid {
		t.Errorf("returned %v instead of expected %v", cid, expectedCid)
	}
}

// builds an Equinix Metal client
func constructClient(authToken string, baseUrl *string) *metal.APIClient {
	configuration := &metal.Configuration{
		DefaultHeader:    make(map[string]string),
		UserAgent:        "metal-go/0.29.0",
		Debug:            false,
		Servers:          metal.ServerConfigurations{},
		OperationServers: map[string]metal.ServerConfigurations{},
	}

	servers := metal.ServerConfigurations{
		{
			URL:         "https://api.equinix.com/metal/v1",
			Description: "No description provided",
		},
	}
	if baseUrl != nil {
		servers = metal.ServerConfigurations{
			{
				URL:         *baseUrl,
				Description: "No description provided",
			},
		}
	}

	configuration.Servers = servers
	configuration.AddDefaultHeader("X-Auth-Token", authToken)
	configuration.UserAgent = fmt.Sprintf("cloud-provider-equinix-metal/%s %s", version.Get(), configuration.UserAgent)
	return metal.NewAPIClient(configuration)
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
