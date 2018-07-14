package packet

import (
	"io"
	"os"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"

	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	packetAuthTokenEnvVar string = "PACKET_AUTH_TOKEN"
	packetProjectIDEnvVar string = "PACKET_PROJECT_ID"
	providerName          string = "packet"
)

type cloud struct {
	client    *packngo.Client
	instances cloudprovider.Instances
	zones     cloudprovider.Zones
}

func newCloud(config io.Reader) (cloudprovider.Interface, error) {
	token := os.Getenv(packetAuthTokenEnvVar)
	project := os.Getenv(packetProjectIDEnvVar)

	if token == "" {
		return nil, errors.Errorf("environment variable %q is required", packetAuthTokenEnvVar)
	}

	if project == "" {
		return nil, errors.Errorf("environment variable %q is required", packetProjectIDEnvVar)
	}

	client := packngo.NewClient("", token, nil)

	facility, err := deviceFacility()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get facility from device metadata")
	}

	return &cloud{
		client:    client,
		instances: newInstances(client, project),
		zones:     newZones(client, project, facility),
	}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(config io.Reader) (cloudprovider.Interface, error) {
		return newCloud(config)
	})
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (c *cloud) Initialize(_ controller.ControllerClientBuilder) {
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
// TODO unimplemented
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	return c.zones, true
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *cloud) ProviderName() string {
	return providerName
}

// HasClusterID returns true if a ClusterID is required and set
func (c *cloud) HasClusterID() bool {
	return false
}
