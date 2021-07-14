package metal

import (
	"net/http/httptest"
	"net/url"
	"testing"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	emServer "github.com/packethost/packet-api-server/pkg/server"
	"github.com/packethost/packet-api-server/pkg/store"
	"github.com/packethost/packngo"

	cloudprovider "k8s.io/cloud-provider"
)

const (
	projectID       = "abcdef-123456"
	token           = "12345678"
	nodeName        = "ccm-test"
	validRegionCode = "ewr1"
	validRegionName = "Parsippany, NJ"
	validPlanSlug   = "hourly"
	validPlanName   = "Bill by the hour"
)

var (
	validCloud *cloud
	backend    *store.Memory
)

type apiServerError struct {
	t *testing.T
}

func (a *apiServerError) Error(err error) {
	a.t.Fatal(err)
}

// create a valid cloud with a client
func testGetValidCloud(t *testing.T) (*cloud, *store.Memory) {
	// if we already have it, it is fine
	if validCloud != nil {
		return validCloud, backend
	}
	// mock endpoint so our client can handle it
	backend = store.NewMemory()
	fake := emServer.PacketServer{
		Store: backend,
		ErrorHandler: &apiServerError{
			t: t,
		},
	}
	// ensure we have a single region
	backend.CreateFacility(validRegionName, validRegionCode)
	ts := httptest.NewServer(fake.CreateHandler())

	url, _ := url.Parse(ts.URL)
	urlString := url.String()

	client := constructClient(token, &urlString)

	// now just need to create a client
	config := Config{
		ProjectID: projectID,
	}
	c, _ := newCloud(config, client)
	validCloud = c.(*cloud)
	return validCloud, backend
}

func TestLoadBalancer(t *testing.T) {
	vc, _ := testGetValidCloud(t)
	response, supported := vc.LoadBalancer()
	var (
		expectedSupported = false
		expectedResponse  cloudprovider.LoadBalancer // defaults to nil
	)
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}
}

func TestInstances(t *testing.T) {
	vc, _ := testGetValidCloud(t)
	response, supported := vc.Instances()
	expectedSupported := true
	expectedResponse := vc.instances
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}
}

func TestZones(t *testing.T) {
	vc, _ := testGetValidCloud(t)
	response, supported := vc.Zones()
	expectedSupported := true
	expectedResponse := vc.zones
	if supported != expectedSupported {
		t.Errorf("supported returned %v instead of expected %v", supported, expectedSupported)
	}
	if response != expectedResponse {
		t.Errorf("value returned %v instead of expected %v", response, expectedResponse)
	}

}

func TestClusters(t *testing.T) {
	vc, _ := testGetValidCloud(t)
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
	vc, _ := testGetValidCloud(t)
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
	vc, _ := testGetValidCloud(t)
	name := vc.ProviderName()
	if name != providerName {
		t.Errorf("returned %s instead of expected %s", name, providerName)
	}
}

func TestHasClusterID(t *testing.T) {
	vc, _ := testGetValidCloud(t)
	cid := vc.HasClusterID()
	expectedCid := true
	if cid != expectedCid {
		t.Errorf("returned %v instead of expected %v", cid, expectedCid)
	}

}

// builds an Equinix Metal client
func constructClient(authToken string, baseURL *string) *packngo.Client {
	/*
		tr := &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
		}
	*/
	client := retryablehttp.NewClient()

	// client.Transport = logging.NewTransport("EquinixMetal", client.Transport)
	if baseURL != nil {
		// really should handle error, but packngo does not distinguish now or handle errors, so ignoring for now
		client, _ := packngo.NewClientWithBaseURL(ConsumerToken, authToken, client.StandardClient(), *baseURL)
		return client
	}
	return packngo.NewClientWithAuth(ConsumerToken, authToken, client.StandardClient())
}
