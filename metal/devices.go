package metal

import (
	"context"
	"fmt"
	"strings"

	"github.com/packethost/packngo"
	"github.com/packethost/packngo/metadata"
	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

type instances struct {
	client  *packngo.Client
	project string
}

var (
	_ cloudprovider.Instances   = (*instances)(nil)
	_ cloudprovider.InstancesV2 = (*instances)(nil)
)

func newInstances(client *packngo.Client, projectID string) *instances {
	return &instances{client: client, project: projectID}
}

// cloudService implementation
func (i *instances) name() string {
	return "instances"
}
func (i *instances) init(k8sclient kubernetes.Interface) error {
	return nil
}
func (i *instances) nodeReconciler() nodeReconciler {
	return nil
}
func (i *instances) serviceReconciler() serviceReconciler {
	return nil
}

// cloudprovider.Instances interface implementation

// NodeAddresses returns the addresses of the specified instance.
func (i *instances) NodeAddresses(_ context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	klog.V(2).Infof("called NodeAddresses with node name %s", name)
	device, err := deviceByName(i.client, i.project, name)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(device)
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose nodeaddresses are being queried. i.e. local metadata
// services cannot be used in this method to obtain nodeaddresses.
func (i *instances) NodeAddressesByProviderID(_ context.Context, providerID string) ([]v1.NodeAddress, error) {
	klog.V(2).Infof("called NodeAddressesByProviderID with providerID %s", providerID)
	device, err := i.deviceFromProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(device)
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

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist or is no longer running, we must return ("", cloudprovider.InstanceNotFound)
func (i *instances) InstanceID(_ context.Context, nodeName types.NodeName) (string, error) {
	klog.V(2).Infof("called InstanceID with node name %s", nodeName)
	device, err := deviceByName(i.client, i.project, nodeName)
	if err != nil {
		return "", err
	}

	// safely handle if it already is structured as equinixmetal://<id>
	split := strings.Split(device.ID, "://")
	var devID string
	switch len(split) {
	case 2:
		devID = split[1]
	case 1:
		devID = device.ID
	default:
		return "", fmt.Errorf("unknown format for deviceID: %s", device.ID)
	}

	klog.V(2).Infof("InstanceID for %s: %s", nodeName, devID)
	return devID, nil
}

// InstanceType returns the type of the specified instance.
func (i *instances) InstanceType(_ context.Context, nodeName types.NodeName) (string, error) {
	klog.V(2).Infof("called InstanceType with node name %s", nodeName)
	device, err := deviceByName(i.client, i.project, nodeName)
	if err != nil {
		return "", err
	}

	return device.Plan.Name, nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *instances) InstanceTypeByProviderID(_ context.Context, providerID string) (string, error) {
	klog.V(2).Infof("called InstanceTypeByProviderID with providerID %s", providerID)
	device, err := i.deviceFromProviderID(providerID)
	if err != nil {
		return "", err
	}

	return device.Plan.Name, nil
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i *instances) AddSSHKeyToAllInstances(_ context.Context, user string, keyData []byte) error {
	klog.V(2).Info("called AddSSHKeyToAllInstances")
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i *instances) CurrentNodeName(_ context.Context, nodeName string) (types.NodeName, error) {
	klog.V(2).Infof("called CurrentNodeName with nodeName %s", nodeName)
	return types.NodeName(nodeName), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider id still is running.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
func (i *instances) InstanceExistsByProviderID(_ context.Context, providerID string) (bool, error) {
	klog.V(2).Infof("called InstanceExistsByProviderID with providerID %s", providerID)
	_, err := i.deviceFromProviderID(providerID)
	switch {
	case err != nil && err == cloudprovider.InstanceNotFound:
		return false, nil
	case err != nil:
		return false, err
	}

	return true, nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i *instances) InstanceShutdownByProviderID(_ context.Context, providerID string) (bool, error) {
	klog.V(2).Infof("called InstanceShutdownByProviderID with providerID %s", providerID)
	device, err := i.deviceFromProviderID(providerID)
	if err != nil {
		return false, err
	}

	return device.State == "inactive", nil
}

// InstanceShutdown returns true if the node is shutdown in cloudprovider
func (i *instances) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	return i.InstanceShutdownByProviderID(ctx, node.Spec.ProviderID)
}

// InstanceExists returns true if the node exists in cloudprovider
func (i *instances) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	return i.InstanceExistsByProviderID(ctx, node.Spec.ProviderID)
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
			return "", errors.Errorf("provider name from providerID should be %s, was %s", ProviderName, split[0])
		}
	case 1:
		deviceID = providerID
	default:
		return "", errors.Errorf("unexpected providerID format: %s, format should be: 'device-id' or 'equinixmetal://device-id'", providerID)
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
