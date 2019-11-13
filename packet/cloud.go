package packet

import (
	"io"
	"os"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

const (
	packetAuthTokenEnvVar string = "PACKET_AUTH_TOKEN"
	packetProjectIDEnvVar string = "PACKET_PROJECT_ID"
	providerName          string = "packet"
	// ConsumerToken token for packet consumer
	ConsumerToken string = "packet-ccm"
)

type cloud struct {
	client    *packngo.Client
	instances cloudprovider.Instances
	zones     cloudprovider.Zones
}

func readEnvVars() (string, string, error) {
	token := os.Getenv(packetAuthTokenEnvVar)
	project := os.Getenv(packetProjectIDEnvVar)

	if token == "" {
		return "", "", errors.Errorf("environment variable %q is required", packetAuthTokenEnvVar)
	}

	if project == "" {
		return "", "", errors.Errorf("environment variable %q is required", packetProjectIDEnvVar)
	}
	return token, project, nil
}
func newCloud(config io.Reader, token, project string, client *packngo.Client) (cloudprovider.Interface, error) {
	return &cloud{
		client:    client,
		instances: newInstances(client, project),
		zones:     newZones(client, project),
	}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(config io.Reader) (cloudprovider.Interface, error) {
		token, project, err := readEnvVars()
		if err != nil {
			return nil, err
		}
		client := packngo.NewClientWithAuth("", token, nil)

		return newCloud(config, token, project, client)
	})
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (c *cloud) Initialize(_ cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.V(5).Info("called Initialize")
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
// TODO unimplemented
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	klog.V(5).Info("called LoadBalancer")
	return nil, false
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	klog.V(5).Info("called Instances")
	return c.instances, true
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	klog.V(5).Info("called Zones")
	return c.zones, true
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	klog.V(5).Info("called Clusters")
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	klog.V(5).Info("called Routes")
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *cloud) ProviderName() string {
	klog.V(2).Infof("called ProviderName, returning %s", providerName)
	return providerName
}

// HasClusterID returns true if a ClusterID is required and set
func (c *cloud) HasClusterID() bool {
	klog.V(5).Info("called HasClusterID")
	return false
}
