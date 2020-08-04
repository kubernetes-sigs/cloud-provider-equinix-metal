package packet

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

type bgp struct {
	project                                                   string
	client                                                    *packngo.Client
	k8sclient                                                 kubernetes.Interface
	peerASN, localASN                                         int
	annotationLocalASN, annotationPeerASNs, annotationPeerIPs string
}

func newBGP(client *packngo.Client, project string, localASN, peerASN int, annotationLocalASN, annotationPeerASNs, annotationPeerIPs string) *bgp {
	return &bgp{
		project:            project,
		client:             client,
		localASN:           localASN,
		peerASN:            peerASN,
		annotationLocalASN: annotationLocalASN,
		annotationPeerASNs: annotationPeerASNs,
		annotationPeerIPs:  annotationPeerIPs,
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
	return b.reconcileNodes
}
func (b *bgp) serviceReconciler() serviceReconciler {
	return nil
}

// reconcileNodes ensures each node has the annotations showing the peer address
// and ASN. MetalLB currently does not use this, although it is in process, see
// http://github.com/metallb/metallb/pull/593 . Once that is in, we will not
// need to update the configmap.
func (b *bgp) reconcileNodes(nodes []*v1.Node, remove bool) error {
	nodeNames := []string{}
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}
	klog.V(2).Infof("bgp.reconcileNodes(): called for nodes %v", nodeNames)
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

			// add annotations for bgp
			klog.V(2).Infof("bgp.reconcileNodes(): setting annotations on node %s", node.Name)
			// get the bgp info
			peers, err := getNodePeerAddress(id, b.client)
			if err != nil || len(peers) < 1 {
				klog.Errorf("bgp.reconcileNodes(): could not get BGP info for node %s: %v", node.Name, err)
			} else {
				localASN := strconv.Itoa(b.localASN)
				peerASNs := strconv.Itoa(b.peerASN)
				newAnnotations := make(map[string]string)
				oldAnnotations := node.Annotations
				if oldAnnotations == nil {
					oldAnnotations = make(map[string]string)
				}
				val, ok := oldAnnotations[b.annotationLocalASN]
				if !ok || val != localASN {
					newAnnotations[b.annotationLocalASN] = localASN
				}

				val, ok = oldAnnotations[b.annotationPeerASNs]
				if !ok || val != peerASNs {
					newAnnotations[b.annotationPeerASNs] = peerASNs
				}

				val, ok = oldAnnotations[b.annotationPeerIPs]
				if !ok || val != peers[0] {
					newAnnotations[b.annotationPeerIPs] = peers[0]
				}

				// patch the node with the new annotations
				if len(newAnnotations) > 0 {
					mergePatch, _ := json.Marshal(map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": newAnnotations,
						},
					})

					if err := patchUpdatedNode(node.Name, mergePatch, b.k8sclient); err != nil {
						klog.Errorf("bgp.reconcileNodes(): failed to save updated node with annotations %s: %v", node.Name, err)
					} else {
						klog.V(2).Infof("bgp.reconcileNodes(): annotations set on node %s", node.Name)
					}
				} else {
					klog.V(2).Infof("bgp.reconcileNodes(): no change to annotations for %s", node.Name)
				}
			}
		}
	}
	klog.V(2).Info("bgp.reconcileNodes(): complete")
	return nil
}

// enableBGP enable bgp on the project
func (b *bgp) enableBGP() error {
	// first check if it is enabled before trying to create it
	bgpConfig, _, err := b.client.BGPConfig.Get(b.project, &packngo.GetOptions{})
	// if we already have a config, just return
	if err == nil || bgpConfig != nil {
		return nil
	}

	req := packngo.CreateBGPConfigRequest{
		Asn:            b.localASN,
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

// getNodePeerAddress get the BGP peer address for a specific node
func getNodePeerAddress(providerID string, client *packngo.Client) (address []string, err error) {
	id, err := deviceIDFromProviderID(providerID)
	if err != nil {
		return nil, err
	}
	neighbours, _, err := client.Devices.ListBGPNeighbors(id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get device neighbours for device %s: %v", id, err)
	}
	// we need the ipv4 neighbour
	var addrs []string
	for _, n := range neighbours {
		if n.AddressFamily != 4 {
			continue
		}
		addrs = n.PeerIps
		break
	}
	if len(addrs) == 0 {
		return addrs, errors.New("no matching ipv4 neighbour found")
	}
	return addrs, nil
}

// patchUpdatedNode apply a patch to the node
func patchUpdatedNode(name string, patch []byte, client kubernetes.Interface) error {
	if _, err := client.CoreV1().Nodes().Patch(name, k8stypes.MergePatchType, patch); err != nil {
		return fmt.Errorf("Failed to patch node %s: %v", name, err)
	}
	return nil
}
