package packet

import (
	"fmt"

	"github.com/packethost/packngo"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

type bgp struct {
	project string
	client  *packngo.Client
}

func newBGP(client *packngo.Client, project string) *bgp {
	return &bgp{
		project: project,
		client:  client,
	}
}

func (b *bgp) name() string {
	return "bgp"
}
func (b *bgp) init(k8sclient kubernetes.Interface) error {
	// enable BGP
	klog.V(2).Info("bgp.init(): enabling BGP on project")
	if err := b.enableBGP(); err != nil {
		return fmt.Errorf("failed to enable BGP on project %s: %v", b.project, err)
	}
	klog.V(2).Info("bgp.init(): BGP enabled")
	return nil
}
func (b *bgp) nodeReconciler() nodeReconciler {
	return b.reconcileNodes
}
func (b *bgp) serviceReconciler() serviceReconciler {
	return nil
}

func (b *bgp) reconcileNodes(nodes []*v1.Node, remove bool) error {
	for _, node := range nodes {
		// are we adding or removing the node?
		if !remove {
			// get the node provider ID
			id := node.Spec.ProviderID
			if id == "" {
				return fmt.Errorf("no provider ID given")
			}
			klog.V(2).Infof("bgp.reconcileNodes(): enabling BGP on node %s", node.Name)
			// ensure BGP is enabled for the node
			if err := ensureNodeBGPEnabled(id, b.client); err != nil {
				klog.Errorf("could not ensure BGP enabled for node %s: %v", node.Name, err)
			}
			klog.V(2).Infof("bgp.reconcileNodes(): bgp enabled on node %s", node.Name)
		}
	}
	return nil
}

// enableBGP enable bgp on the project
func (b *bgp) enableBGP() error {
	req := packngo.CreateBGPConfigRequest{
		Asn:            asn,
		DeploymentType: "local",
		UseCase:        "kubernetes-load-balancer",
	}
	_, err := b.client.BGPConfig.Create(b.project, req)
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
	_, _, err = client.BGPSessions.Create(id, req)
	return err
}
