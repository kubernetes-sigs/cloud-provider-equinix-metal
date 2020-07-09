package packet

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"errors"

	"github.com/packethost/packngo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
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
	inProcess     bool
	apiServerPort int
	eipTag        string
	instances     cloudInstances
	deviceIPSrv   packngo.DeviceIPService
	ipResSvr      packngo.ProjectIPService
	projectID     string
	httpClient    *http.Client
	k8sclient     kubernetes.Interface
}

func (m *controlPlaneEndpointManager) name() string {
	return "controlPlaneEndpointManager"
}

func (m *controlPlaneEndpointManager) init(k8sclient kubernetes.Interface) error {
	m.k8sclient = k8sclient
	klog.V(2).Info("controlPlaneEndpointManager.init(): enabling BGP on project")
	return nil
}

func (m *controlPlaneEndpointManager) nodeReconciler() nodeReconciler {
	return m.reconciler()
}
func (m *controlPlaneEndpointManager) serviceReconciler() serviceReconciler {
	return nil
}

func (m *controlPlaneEndpointManager) reconciler() func(nodes []*v1.Node, remove bool) error {
	return func(nodes []*v1.Node, remove bool) error {
		klog.V(2).Info("controlPlaneEndpoint.reconcile: new reconciliation")
		if m.inProcess {
			klog.V(2).Info("controlPlaneEndpoint.reconcileNodes: already in process, not starting a new one")
			return nil
		}
		m.inProcess = true
		defer func() {
			m.inProcess = false
		}()
		if m.eipTag == "" {
			return errors.New("elastic ip tag is empty. Nothing to do")
		}
		ipList, _, err := m.ipResSvr.List(m.projectID)
		if err != nil {
			return err
		}
		controlPlaneEndpoint := ipReservationByTags([]string{m.eipTag}, ipList)
		if controlPlaneEndpoint == nil {
			// IP NOT FOUND nothing to do here.
			klog.Errorf("elastic IP not found. Please verify you have one with the expected tag: %s", m.eipTag)
			return err
		}
		klog.Infof("healthcheck elastic ip %s", fmt.Sprintf("https://%s:%d/healthz", controlPlaneEndpoint.Address, m.apiServerPort))
		req, err := http.NewRequest("GET", fmt.Sprintf("https://%s:%d/healthz", controlPlaneEndpoint.Address, m.apiServerPort), nil)
		if err != nil {
			return err
		}
		resp, err := m.httpClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if err != nil {
				klog.Errorf("http client error during healthcheck. err \"%s\"", err)
			}
			if err := m.reassign(context.Background(), nodes, controlPlaneEndpoint); err != nil {
				klog.Errorf("error reassigning control plane endpoint to a different device. err \"%s\"", err)
				return err
			}
		}
		defer resp.Body.Close()
		return nil
	}
}

func (m *controlPlaneEndpointManager) reassign(ctx context.Context, nodes []*v1.Node, ip *packngo.IPAddressReservation) error {
	klog.V(2).Info("controlPlaneEndpoint.reassign")
	for _, node := range nodes {
		addresses, err := m.instances.NodeAddresses(ctx, types.NodeName(node.Name))
		if err != nil {
			return err
		}

		// I decided to iterate over all the addresses assigned to the node to avoid network missconfiguration
		// The first one for example is the node name, and if the hostname is not well configured it will never work.
		for _, a := range addresses {
			klog.Infof("healthcheck node %s", fmt.Sprintf("https://%s:%d/healthz", a.Address, m.apiServerPort))
			req, err := http.NewRequest("GET", fmt.Sprintf("https://%s:%d/healthz", a.Address, m.apiServerPort), nil)
			if err != nil {
				klog.Errorf("healthcheck failed for node %s. err \"%s\"", node.Name, err)
				continue
			}
			resp, err := m.httpClient.Do(req)

			if err != nil {
				if err != nil {
					klog.Errorf("http client error during healthcheck. err \"%s\"", err)
				}
				continue
			}

			// We have a healthy node, this is the candidate to receive the EIP
			if resp.StatusCode == http.StatusOK {
				deviceID, err := m.instances.InstanceID(ctx, types.NodeName(node.Name))
				if err != nil {
					return err
				}
				if _, err := m.deviceIPSrv.Unassign(ip.ID); err != nil {
					return err
				}
				if _, _, err := m.deviceIPSrv.Assign(deviceID, &packngo.AddressStruct{
					Address: ip.Address,
				}); err != nil {
					return err
				}
				klog.V(2).Infof("control plane endpoint assigned to new device %s", node.Name)
			}
		}

	}
	return errors.New("ccm didn't find a good candidate for IP allocation. Cluster is unhealthy")
}

func newControlPlaneEndpointManager(eipTag, projectID string, deviceIPSrv packngo.DeviceIPService, ipResSvr packngo.ProjectIPService, i cloudInstances, apiServerPort int) *controlPlaneEndpointManager {
	return &controlPlaneEndpointManager{
		httpClient: &http.Client{
			Timeout: time.Second * 5,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}},
		eipTag:        eipTag,
		projectID:     projectID,
		instances:     i,
		ipResSvr:      ipResSvr,
		deviceIPSrv:   deviceIPSrv,
		apiServerPort: apiServerPort,
	}
}
