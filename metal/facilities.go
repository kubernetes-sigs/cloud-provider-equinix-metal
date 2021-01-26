package metal

import (
	"context"

	"github.com/packethost/packngo"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

type zones struct {
	client  *packngo.Client
	project string
}

func newZones(client *packngo.Client, projectID string) zones {
	return zones{client, projectID}
}

// cloudService implementation
func (z zones) name() string {
	return "zones"
}
func (z zones) init(k8sclient kubernetes.Interface) error {
	return nil
}
func (z zones) nodeReconciler() nodeReconciler {
	return nil
}

func (z zones) serviceReconciler() serviceReconciler {
	return nil
}

// cloudprovider.Zones implementation

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in
// In most cases, this method is called from the kubelet querying a local metadata service to acquire its zone.
// For the case of external cloud providers, use GetZoneByProviderID or GetZoneByNodeName since GetZone
// can no longer be called from the kubelets.
func (z zones) GetZone(_ context.Context) (cloudprovider.Zone, error) {
	klog.V(2).Info("called GetZones")
	return cloudprovider.Zone{}, cloudprovider.NotImplemented
}

// GetZoneByProviderID returns the Zone containing the current zone and locality region of the node specified by providerId
// This method is particularly used in the context of external cloud providers where node initialization must be down
// outside the kubelets.
func (z zones) GetZoneByProviderID(_ context.Context, providerID string) (cloudprovider.Zone, error) {
	klog.V(2).Infof("called GetZoneByProviderID with providerID %s", providerID)
	id, err := deviceIDFromProviderID(providerID)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	device, err := deviceByID(z.client, id)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return cloudprovider.Zone{Region: device.Facility.Code}, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node name
// This method is particularly used in the context of external cloud providers where node initialization must be down
// outside the kubelets.
func (z zones) GetZoneByNodeName(_ context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	klog.V(2).Infof("called GetZoneByNodeName with nodeName %s", nodeName)
	device, err := deviceByName(z.client, z.project, nodeName)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return cloudprovider.Zone{Region: device.Facility.Code}, nil
}
