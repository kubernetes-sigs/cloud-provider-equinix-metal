package metal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/packethost/packngo"
	"github.com/packethost/packngo/metadata"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

type instances struct {
	client  *packngo.Client
	project string
}

var (
	_ cloudprovider.InstancesV2 = (*instances)(nil)
)

func newInstances(client *packngo.Client, projectID string) *instances {
	return &instances{client: client, project: projectID}
}

// InstanceShutdown returns true if the node is shutdown in cloudprovider
func (i *instances) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	klog.V(2).Infof("called InstanceShutdown for node %s with providerID %s", node.GetName(), node.Spec.ProviderID)
	device, err := i.deviceFromProviderID(node.Spec.ProviderID)
	if err != nil {
		return false, err
	}

	return device.State == "inactive", nil
}

// InstanceExists returns true if the node exists in cloudprovider
func (i *instances) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	klog.V(2).Infof("called InstanceExists for node %s with providerID %s", node.GetName(), node.Spec.ProviderID)
	_, err := i.deviceFromProviderID(node.Spec.ProviderID)

	switch {
	case errors.Is(err, cloudprovider.InstanceNotFound):
		return false, nil
	case err != nil:
		return false, err
	}

	return true, nil
}

// InstanceMetadata returns instancemetadata for the node according to the cloudprovider
func (i *instances) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	device, err := i.deviceByNode(node)
	if err != nil {
		return nil, err
	}
	nodeAddresses, err := nodeAddresses(device)
	if err != nil {
		// TODO(displague) we error on missing private and public ip. is that restrictive?

		// TODO(displague) should we return the public addresses DNS name as the Type=Hostname NodeAddress type too?
		return nil, err
	}
	var p, r, z string
	if device.Plan != nil {
		p = device.Plan.Slug
	}

	// "A zone represents a logical failure domain"
	// "A region represents a larger domain, made up of one or more zones"
	//
	// Equinix Metal metros are made up of one or more facilities, so we treat
	// metros as K8s topology regions. EM facilities are then equated to zones.
	//
	// https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone

	if device.Facility != nil {
		z = device.Facility.Code
	}
	if device.Metro != nil {
		r = device.Metro.Code
	}

	return &cloudprovider.InstanceMetadata{
		ProviderID:    providerIDFromDevice(device),
		InstanceType:  p,
		NodeAddresses: nodeAddresses,
		Zone:          z,
		Region:        r,
	}, nil
}

func nodeAddresses(device *packngo.Device) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: device.Hostname})

	var privateIP, publicIP string
	for _, address := range device.Network {
		if address.AddressFamily == int(metadata.IPv4) {
			var addrType v1.NodeAddressType
			if address.Public {
				publicIP = address.Address
				addrType = v1.NodeExternalIP
			} else {
				privateIP = address.Address
				addrType = v1.NodeInternalIP
			}
			addresses = append(addresses, v1.NodeAddress{Type: addrType, Address: address.Address})
		}
	}

	if privateIP == "" {
		return nil, errors.New("could not get at least one private ip")
	}

	if publicIP == "" {
		return nil, errors.New("could not get at least one public ip")
	}

	return addresses, nil
}

func (i *instances) deviceByNode(node *v1.Node) (*packngo.Device, error) {
	if node.Spec.ProviderID != "" {
		return i.deviceFromProviderID(node.Spec.ProviderID)
	}

	return deviceByName(i.client, i.project, types.NodeName(node.GetName()))
}

func deviceByID(client *packngo.Client, id string) (*packngo.Device, error) {
	klog.V(2).Infof("called deviceByID with ID %s", id)
	device, _, err := client.Devices.Get(id, nil)
	if isNotFound(err) {
		return nil, cloudprovider.InstanceNotFound
	}
	return device, err
}

// deviceByName returns an instance whose hostname matches the kubernetes node.Name
func deviceByName(client *packngo.Client, projectID string, nodeName types.NodeName) (*packngo.Device, error) {
	klog.V(2).Infof("called deviceByName with projectID %s nodeName %s", projectID, nodeName)
	if string(nodeName) == "" {
		return nil, errors.New("node name cannot be empty string")
	}
	devices, _, err := client.Devices.List(projectID, nil)
	if err != nil {
		klog.V(2).Infof("error listing devices for project %s: %v", projectID, err)
		return nil, err
	}

	for _, device := range devices {
		if device.Hostname == string(nodeName) {
			klog.V(2).Infof("Found device %s for nodeName %s", device.ID, nodeName)
			return &device, nil
		}
	}

	klog.V(2).Infof("No device found for nodeName %s", nodeName)
	return nil, cloudprovider.InstanceNotFound
}

// deviceIDFromProviderID returns a device's ID from providerID.
//
// The providerID spec should be retrievable from the Kubernetes
// node object. The expected format is: equinixmetal://device-id or just device-id
func deviceIDFromProviderID(providerID string) (string, error) {
	klog.V(2).Infof("called deviceIDFromProviderID with providerID %s", providerID)
	if providerID == "" {
		return "", errors.New("providerID cannot be empty string")
	}

	split := strings.Split(providerID, "://")
	var deviceID string
	switch len(split) {
	case 2:
		deviceID = split[1]
		if split[0] != ProviderName && split[0] != deprecatedProviderName {
			return "", fmt.Errorf("provider name from providerID should be %s, was %s", ProviderName, split[0])
		}
	case 1:
		deviceID = providerID
	default:
		return "", fmt.Errorf("unexpected providerID format: %s, format should be: 'device-id' or 'equinixmetal://device-id'", providerID)
	}

	return deviceID, nil
}

// deviceFromProviderID uses providerID to get the device id and return the device
func (i *instances) deviceFromProviderID(providerID string) (*packngo.Device, error) {
	klog.V(2).Infof("called deviceFromProviderID with providerID %s", providerID)
	id, err := deviceIDFromProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return deviceByID(i.client, id)
}

// providerIDFromDevice returns a providerID from a device
func providerIDFromDevice(device *packngo.Device) string {
	return fmt.Sprintf("%s://%s", ProviderName, device.ID)
}
