package metal

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"

	metal "github.com/equinix/equinix-sdk-go/services/metalv1"
	"github.com/google/uuid"
	emServer "github.com/packethost/packet-api-server/pkg/server"
	"github.com/packethost/packet-api-server/pkg/store"
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
	validZoneName   = "Parsippany, NJ"
	validPlanSlug   = "hourly"
	validPlanName   = "Bill by the hour"
)

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

type apiServerError struct {
	t *testing.T
}

func (a *apiServerError) Error(err error) {
	a.t.Fatal(err)
}

// create a valid cloud with a client
func testGetValidCloud(t *testing.T, LoadBalancerSetting string) (*cloud, *store.Memory) {
	// mock endpoint so our client can handle it
	backend := store.NewMemory()
	fake := emServer.PacketServer{
		Store: backend,
		ErrorHandler: &apiServerError{
			t: t,
		},
	}
	// ensure we have a single region
	_, _ = backend.CreateFacility(validZoneName, validZoneCode)
	ts := httptest.NewServer(fake.CreateHandler())

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

	return c.(*cloud), backend
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
