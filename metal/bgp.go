package metal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	metal "github.com/equinix/equinix-sdk-go/services/metalv1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type bgp struct {
	project   string
	client    *metal.BGPApiService
	k8sclient kubernetes.Interface
	localASN  int
	bgpPass   string
}

func newBGP(client *metal.BGPApiService, k8sclient kubernetes.Interface, metalConfig Config) (*bgp, error) {
	b := &bgp{
		client:    client,
		k8sclient: k8sclient,
		project:   metalConfig.ProjectID,
		localASN:  metalConfig.LocalASN,
		bgpPass:   metalConfig.BGPPass,
	}
	// enable BGP
	klog.V(2).Info("bgp.init(): enabling BGP on project")
	if err := b.enableBGP(); err != nil {
		return nil, fmt.Errorf("failed to enable BGP on project %s: %w", b.project, err)
	}
	klog.V(2).Info("bgp.init(): BGP enabled")
	return b, nil
}

// enableBGP enable bgp on the project
func (b *bgp) enableBGP() error {
	// first check if it is enabled before trying to create it
	bgpConfig, _, err := b.client.
		FindBgpConfigByProject(context.Background(), b.project).
		Execute()
	// if we already have a config, just return
	// we need some extra handling logic because the API always returns 200, even if
	// not BGP config is in place.
	// We treat it as valid config already exists only if ALL of the above is true:
	// - no error
	// - bgpConfig struct exists
	// - bgpConfig struct has non-blank ID
	// - bgpConfig struct does not have Status=="disabled"
	if err == nil && bgpConfig != nil && bgpConfig.GetId() != "" && bgpConfig.GetStatus() != metal.BGPCONFIGSTATUS_DISABLED {
		b.localASN = int(bgpConfig.GetAsn())
		b.bgpPass = bgpConfig.GetMd5()
		return nil
	}

	// we did not have a valid one, so create it
	req := metal.BgpConfigRequestInput{
		Asn:            int64(b.localASN),
		Md5:            &b.bgpPass,
		DeploymentType: "local",
		UseCase:        metal.PtrString("kubernetes-load-balancer"),
	}
	_, err = b.client.
		RequestBgpConfig(context.Background(), b.project).
		BgpConfigRequestInput(req).
		Execute()
	return err
}

// ensureNodeBGPEnabled check if the node has bgp enabled, and set it if it does not
func ensureNodeBGPEnabled(id string, client *metal.APIClient) error {
	// if we are rnning ccm properly, then the provider ID will be on the node object
	id, err := deviceIDFromProviderID(id)
	if err != nil {
		return err
	}
	// fortunately, this is idempotent, so just create
	req := metal.BGPSessionInput{
		AddressFamily: metal.BGPSESSIONINPUTADDRESSFAMILY_IPV4.Ptr(),
	}
	_, response, err := client.DevicesApi.
		CreateBgpSession(context.Background(), id).
		BGPSessionInput(req).
		Execute()

	// if we already had one, then we can ignore the error
	// this really should be a 409, but 422 is what is returned
	if response.StatusCode == 422 && strings.Contains(fmt.Sprintf("%s", err), "already has session") {
		err = nil
	}
	return err
}

// getNodeBGPConfig get the BGP config for a specific node
func getNodeBGPConfig(providerID string, client *metal.APIClient) (peer *metal.BgpNeighborData, err error) {
	id, err := deviceIDFromProviderID(providerID)
	if err != nil {
		return nil, err
	}
	bgpSessions, _, err := client.DevicesApi.
		GetBgpNeighborData(context.Background(), id).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get device neighbours for device %s: %w", id, err)
	}

	bgpNeighbours := bgpSessions.GetBgpNeighbors()
	// we need the ipv4 neighbour
	for _, n := range bgpNeighbours {
		if n.GetAddressFamily() == 4 {
			return &n, nil
		}
	}
	return nil, errors.New("no matching ipv4 neighbour found")
}
