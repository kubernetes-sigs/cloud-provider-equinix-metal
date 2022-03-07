package metal

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	emServer "github.com/packethost/packet-api-server/pkg/server"
	"github.com/packethost/packet-api-server/pkg/store"
	"github.com/packethost/packngo"

	clientset "k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
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
type mockControllerClientBuilder struct {
}

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
func constructClient(authToken string, baseURL *string) *packngo.Client {
	client := retryablehttp.NewClient()

	// client.Transport = logging.NewTransport("EquinixMetal", client.Transport)
	if baseURL != nil {
		// really should handle error, but packngo does not distinguish now or handle errors, so ignoring for now
		client, _ := packngo.NewClientWithBaseURL(ConsumerToken, authToken, client.StandardClient(), *baseURL)
		return client
	}
	return packngo.NewClientWithAuth(ConsumerToken, authToken, client.StandardClient())
}
