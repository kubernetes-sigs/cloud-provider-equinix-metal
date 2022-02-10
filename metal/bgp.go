package metal

import (
	"fmt"
	"strings"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type bgp struct {
	project   string
	client    *packngo.Client
	k8sclient kubernetes.Interface
	localASN  int
	bgpPass   string
}

func newBGP(client *packngo.Client, project string, localASN int, bgpPass string) *bgp {

	return &bgp{
		project:  project,
		client:   client,
		localASN: localASN,
		bgpPass:  bgpPass,
	}
}

func (b *bgp) name() string {
	return "bgp"
}
func (b *bgp) init(k8sclient kubernetes.Interface) error {
	b.k8sclient = k8sclient
	// enable BGP
	klog.V(2).Info("bgp.init(): enabling BGP on project")
	if err := b.enableBGP(); err != nil {
		return fmt.Errorf("failed to enable BGP on project %s: %v", b.project, err)
	}
	klog.V(2).Info("bgp.init(): BGP enabled")
	return nil
}
func (b *bgp) nodeReconciler() nodeReconciler {
	return nil
}
func (b *bgp) serviceReconciler() serviceReconciler {
	return nil
}

// enableBGP enable bgp on the project
func (b *bgp) enableBGP() error {
	// first check if it is enabled before trying to create it
	bgpConfig, _, err := b.client.BGPConfig.Get(b.project, &packngo.GetOptions{})
	// if we already have a config, just return
	// we need some extra handling logic because the API always returns 200, even if
	// not BGP config is in place.
	// We treat it as valid config already exists only if ALL of the above is true:
	// - no error
	// - bgpConfig struct exists
	// - bgpConfig struct has non-blank ID
	// - bgpConfig struct does not have Status=="disabled"
	if err == nil && bgpConfig != nil && bgpConfig.ID != "" && strings.ToLower(bgpConfig.Status) != "disabled" {
		return nil
	}

	// we did not have a valid one, so create it
	req := packngo.CreateBGPConfigRequest{
		Asn:            b.localASN,
		Md5:            b.bgpPass,
		DeploymentType: "local",
		UseCase:        "kubernetes-load-balancer",
	}
	_, err = b.client.BGPConfig.Create(b.project, req)
	return err
}

// ensureNodeBGPEnabled check if the node has bgp enabled, and set it if it does not
func ensureNodeBGPEnabled(id string, client *packngo.Client) error {
	// if we are rnning ccm properly, then the provider ID will be on the node object
	id, err := deviceIDFromProviderID(id)
	if err != nil {
		return err
	}
	// fortunately, this is idempotent, so just create
	req := packngo.CreateBGPSessionRequest{
		AddressFamily: "ipv4",
	}
	_, response, err := client.BGPSessions.Create(id, req)
	// if we already had one, then we can ignore the error
	// this really should be a 409, but 422 is what is returned
	if response.StatusCode == 422 && strings.Contains(fmt.Sprintf("%s", err), "already has session") {
		err = nil
	}
	return err
}

// getNodeBGPConfig get the BGP config for a specific node
func getNodeBGPConfig(providerID string, client *packngo.Client) (peer *packngo.BGPNeighbor, err error) {
	id, err := deviceIDFromProviderID(providerID)
	if err != nil {
		return nil, err
	}
	neighbours, _, err := client.Devices.ListBGPNeighbors(id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get device neighbours for device %s: %v", id, err)
	}
	// we need the ipv4 neighbour
	for _, n := range neighbours {
		if n.AddressFamily == 4 {
			return &n, nil
		}
	}
	return nil, errors.New("no matching ipv4 neighbour found")
}
