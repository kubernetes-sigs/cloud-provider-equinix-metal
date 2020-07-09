package packet

import (
	"github.com/packethost/packngo"
	v1 "k8s.io/api/core/v1"
)

/*
 controlPlaneEndpointManager checks the availability of an elastic IP for
 the control plane and if it exists the reconciliation guarantees that it is
 attached to a healthy control plane.

 The general steps are:
 1. Check if the passed ElasticIP tags returns a valid Elastic IP via Packet API.
 2. If there is NOT an ElasticIP with those tags just end the reconciliation
 3. If there is an ElasticIP use the kubernetes client-go to check if it
 returns a valid response
 4. If the response returned via client-go is good we do not need to do anything
 5. If the response if wrong or it terminated it means that the device behind
 the ElasticIP is not working correctly and we have to find a new one.
 6. Ping the other control plane available in the cluster, if one of them work
 assign the ElasticIP to that device.
 7. If NO Control Planes succeed, the cluster is unhealthy and the
 reconciliation terminates without changing the current state of the system.
*/
type controlPlaneEndpointManager struct {
	eipTag    string
	instances cloudInstances
}

func (m *controlPlaneEndpointManager) Reconciler() func(nodes []*v1.Node, remove bool) {
	return func(nodes []*v1.Node, remove bool) {
		return
	}
}

func newControlPlaneEndpointManager(eipTag string, ipSrv packngo.DeviceIPService, i cloudInstances) *controlPlaneEndpointManager {
	return &controlPlaneEndpointManager{}
}
