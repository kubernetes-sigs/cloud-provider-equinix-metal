package metal

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/emlb"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/empty"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/kubevip"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/metallb"
	"github.com/packethost/packngo"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type loadBalancers struct {
	client                *packngo.Client
	k8sclient             kubernetes.Interface
	project               string
	metro                 string
	facility              string
	clusterID             string
	implementor           loadbalancers.LB
	implementorConfig     string
	localASN              int
	bgpPass               string
	annotationNetwork     string
	annotationLocalASN    string
	annotationPeerASN     string
	annotationPeerIP      string
	annotationSrcIP       string
	annotationBgpPass     string
	eipMetroAnnotation    string
	eipFacilityAnnotation string
	nodeSelector          labels.Selector
	eipTag                string
}

func newLoadBalancers(client *packngo.Client, k8sclient kubernetes.Interface, projectID, metro, facility, config string, localASN int, bgpPass, annotationNetwork, annotationLocalASN, annotationPeerASN, annotationPeerIP, annotationSrcIP, annotationBgpPass, eipMetroAnnotation, eipFacilityAnnotation, nodeSelector, eipTag string) (*loadBalancers, error) {
	selector := labels.Everything()
	if nodeSelector != "" {
		selector, _ = labels.Parse(nodeSelector)
	}

	l := &loadBalancers{client, k8sclient, projectID, metro, facility, "", nil, config, localASN, bgpPass, annotationNetwork, annotationLocalASN, annotationPeerASN, annotationPeerIP, annotationSrcIP, annotationBgpPass, eipMetroAnnotation, eipFacilityAnnotation, selector, eipTag}

	// parse the implementor config and see what kind it is - allow for no config
	if l.implementorConfig == "" {
		klog.V(2).Info("loadBalancers.init(): no loadbalancer implementation config, skipping")
		return nil, nil
	}

	// get the UID of the kube-system namespace
	systemNamespace, err := k8sclient.CoreV1().Namespaces().Get(context.Background(), "kube-system", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get kube-system namespace: %w", err)
	}
	if systemNamespace == nil {
		return nil, fmt.Errorf("kube-system namespace is missing unexplainably")
	}

	u, err := url.Parse(l.implementorConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	lbconfig := u.Path
	lbflags := u.Query()
	var impl loadbalancers.LB
	switch u.Scheme {
	case "kube-vip":
		klog.Info("loadbalancer implementation enabled: kube-vip")
		impl = kubevip.NewLB(k8sclient, lbconfig)
	case "metallb":
		klog.Info("loadbalancer implementation enabled: metallb")
		impl = metallb.NewLB(k8sclient, lbconfig, lbflags)
	case "empty":
		klog.Info("loadbalancer implementation enabled: empty, bgp only")
		impl = empty.NewLB(k8sclient, lbconfig)
	case "emlb":
		klog.Info("loadbalancer implementation enabled: emlb")
		impl = emlb.NewLB(k8sclient, lbconfig, client.APIKey, projectID)
	default:
		klog.Info("loadbalancer implementation disabled")
		impl = nil
	}

	l.clusterID = string(systemNamespace.UID)
	l.implementor = impl
	klog.V(2).Info("loadBalancers.init(): complete")
	return l, nil
}

// implementation of cloudprovider.LoadBalancer

// GetLoadBalancer returns whether the specified load balancer exists, and
// if so, what its status is.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancers) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	svcName := serviceRep(service)
	svcTag := serviceTag(service)
	clsTag := clusterTag(l.clusterID)
	svcIP := service.Spec.LoadBalancerIP

	var svcIPCidr string

	// get IP address reservations and check if they any exists for this svc
	ips, _, err := l.client.ProjectIPs.List(l.project, &packngo.ListOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("unable to retrieve IP reservations for project %s: %w", l.project, err)
	}

	ipReservation := ipReservationByAllTags([]string{svcTag, emTag, clsTag}, ips)

	klog.V(2).Infof("GetLoadBalancer(): remove: %s with existing IP assignment %s", svcName, svcIP)

	// get the IPs and see if there is anything to clean up
	if ipReservation == nil {
		return nil, false, nil
	}
	svcIPCidr = fmt.Sprintf("%s/%d", ipReservation.Address, ipReservation.CIDR)
	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{IP: svcIPCidr},
		},
	}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *loadBalancers) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	svcTag := serviceTag(service)
	clsTag := clusterTag(l.clusterID)
	return fmt.Sprintf("%s:%s:%s", emTag, svcTag, clsTag)
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	klog.V(2).Infof("EnsureLoadBalancer(): add: service %s/%s", service.Namespace, service.Name)
	// get IP address reservations and check if they any exists for this svc
	ips, _, err := l.client.ProjectIPs.List(l.project, &packngo.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve IP reservations for project %s: %w", l.project, err)
	}
	var ipCidr string
	// handling is completely different if it is the control plane vs a regular service of type=LoadBalancer
	if service.Name == externalServiceName && service.Namespace == externalServiceNamespace {
		ipCidr, err = l.retrieveIPByTag(ctx, service, ips, l.eipTag)
		if err != nil {
			return nil, fmt.Errorf("failed to add service %s: %w", service.Name, err)
		}
	} else {
		ipCidr, err = l.addService(ctx, service, ips, filterNodes(nodes, l.nodeSelector))
		if err != nil {
			return nil, fmt.Errorf("failed to add service %s: %w", service.Name, err)
		}
	}
	// get the IP only
	ip := strings.SplitN(ipCidr, "/", 2)

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{IP: ip[0]},
		},
	}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.V(2).Infof("UpdateLoadBalancer(): service %s", service.Name)

	var n []loadbalancers.Node
	for _, node := range filterNodes(nodes, l.nodeSelector) {
		klog.V(2).Infof("UpdateLoadBalancer(): %s", node.Name)
		// get the node provider ID
		id := node.Spec.ProviderID
		if id == "" {
			return fmt.Errorf("no provider ID given for node %s, skipping", node.Name)
		}
		// ensure BGP is enabled for the node
		if err := ensureNodeBGPEnabled(id, l.client); err != nil {
			klog.Errorf("could not ensure BGP enabled for node %s: %w", node.Name, err)
			continue
		}
		klog.V(2).Infof("bgp enabled on node %s", node.Name)
		// ensure the node has the correct annotations
		if err := l.annotateNode(ctx, node); err != nil {
			return fmt.Errorf("failed to annotate node %s: %w", node.Name, err)
		}
		var (
			peer *packngo.BGPNeighbor
			err  error
		)
		if peer, err = getNodeBGPConfig(id, l.client); err != nil || peer == nil {
			return fmt.Errorf("could not add metallb node peer address for node %s: %w", node.Name, err)
		}
		n = append(n, loadbalancers.Node{
			Name:     node.Name,
			LocalASN: peer.CustomerAs,
			PeerASN:  peer.PeerAs,
			SourceIP: peer.CustomerIP,
			Peers:    peer.PeerIps,
			Password: peer.Md5Password,
		})
	}
	return l.implementor.UpdateService(ctx, service.Namespace, service.Name, n)
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	// REMOVAL
	svcName := serviceRep(service)
	svcTag := serviceTag(service)
	clsTag := clusterTag(l.clusterID)
	svcIP := service.Spec.LoadBalancerIP

	var svcIPCidr string

	// get IP address reservations and check if they any exists for this svc
	ips, _, err := l.client.ProjectIPs.List(l.project, &packngo.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to retrieve IP reservations for project %s: %w", l.project, err)
	}

	ipReservation := ipReservationByAllTags([]string{svcTag, emTag, clsTag}, ips)

	klog.V(2).Infof("EnsureLoadBalancerDeleted(): remove: %s with existing IP assignment %s", svcName, svcIP)

	// get the IPs and see if there is anything to clean up
	if ipReservation == nil {
		klog.V(2).Infof("EnsureLoadBalancerDeleted(): remove: no IP reservation found for %s, nothing to delete", svcName)
		return nil
	}
	// delete the reservation
	klog.V(2).Infof("EnsureLoadBalancerDeleted(): remove: for %s EIP ID %s", svcName, ipReservation.ID)
	if _, err := l.client.ProjectIPs.Remove(ipReservation.ID); err != nil {
		return fmt.Errorf("failed to remove IP address reservation %s from project: %w", ipReservation.String(), err)
	}
	// remove it from any implementation-specific parts
	svcIPCidr = fmt.Sprintf("%s/%d", ipReservation.Address, ipReservation.CIDR)
	klog.V(2).Infof("EnsureLoadBalancerDeleted(): remove: for %s entry %s", svcName, svcIPCidr)
	if err := l.implementor.RemoveService(ctx, service.Namespace, service.Name, svcIPCidr); err != nil {
		return fmt.Errorf("error removing IP from configmap for %s: %w", svcName, err)
	}
	klog.V(2).Infof("EnsureLoadBalancerDeleted(): remove: removed service %s from implementation", svcName)
	return nil
}

// utility funcs

// annotateNode ensure a node has the correct annotations.
func (l *loadBalancers) annotateNode(ctx context.Context, node *v1.Node) error {
	klog.V(2).Infof("annotateNode: %s", node.Name)
	// get the node provider ID
	id, err := deviceIDFromProviderID(node.Spec.ProviderID)
	if err != nil {
		return fmt.Errorf("unable to get device ID from providerID: %w", err)
	}

	// add annotations
	// if it already has them, nothing needs to be done
	annotations := node.Annotations
	if annotations != nil {
		if val, ok := annotations[l.annotationNetwork]; ok {
			klog.V(2).Infof("annotateNode %s: already has annotation %s=%s", node.Name, l.annotationNetwork, val)
			return nil
		}
	}
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// get the network info
	network, err := getNodePrivateNetwork(id, l.client)
	if err != nil || network == "" {
		return fmt.Errorf("could not get private network info for node %s: %w", node.Name, err)
	}
	annotations[l.annotationNetwork] = network

	// get the bgp info
	peer, err := getNodeBGPConfig(id, l.client)
	switch {
	case err != nil || peer == nil:
		return fmt.Errorf("could not get BGP info for node %s: %w", node.Name, err)
	case len(peer.PeerIps) == 0:
		klog.Errorf("got BGP info for node %s but it had no peer IPs", node.Name)
	default:
		// the localASN and peerASN are the same across peers
		localASN := strconv.Itoa(peer.CustomerAs)
		peerASN := strconv.Itoa(peer.PeerAs)
		bgpPass := base64.StdEncoding.EncodeToString([]byte(peer.Md5Password))

		// we always set the peer IPs as a sorted list, so that 0, 1, n are
		// consistent in ordering
		pips := peer.PeerIps
		sort.Strings(pips)
		var (
			i  int
			ip string
		)

		// ensure all of the data we have is in the annotations, either
		// adding or replacing
		for i, ip = range pips {
			annotationLocalASN := strings.Replace(l.annotationLocalASN, "{{n}}", strconv.Itoa(i), 1)
			annotationPeerASN := strings.Replace(l.annotationPeerASN, "{{n}}", strconv.Itoa(i), 1)
			annotationPeerIP := strings.Replace(l.annotationPeerIP, "{{n}}", strconv.Itoa(i), 1)
			annotationSrcIP := strings.Replace(l.annotationSrcIP, "{{n}}", strconv.Itoa(i), 1)
			annotationBgpPass := strings.Replace(l.annotationBgpPass, "{{n}}", strconv.Itoa(i), 1)

			annotations[annotationLocalASN] = localASN
			annotations[annotationPeerASN] = peerASN
			annotations[annotationPeerIP] = ip
			annotations[annotationSrcIP] = peer.CustomerIP
			annotations[annotationBgpPass] = bgpPass
		}
	}

	// TODO: ensure that any old ones that are not in the new data are removed
	// for now, since there are consistently two upstream nodes, we will not bother
	// it gets complex, because we need to match patterns. It is not worth the effort for now.

	// patch the node with the new annotations
	klog.V(2).Infof("annotateNode %s: %v", node.Name, annotations)

	mergePatch, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	})

	if _, err := l.k8sclient.CoreV1().Nodes().Patch(ctx, node.Name, k8stypes.MergePatchType, mergePatch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("failed to patch node with annotations %s: %w", node.Name, err)
	}
	klog.V(2).Infof("annotateNode %s: complete", node.Name)
	return nil
}

// addService add a single service; wraps the implementation
func (l *loadBalancers) addService(ctx context.Context, svc *v1.Service, ips []packngo.IPAddressReservation, nodes []*v1.Node) (string, error) {
	svcName := serviceRep(svc)
	svcTag := serviceTag(svc)
	svcRegion := serviceAnnotation(svc, l.eipMetroAnnotation)
	svcZone := serviceAnnotation(svc, l.eipFacilityAnnotation)
	clsTag := clusterTag(l.clusterID)
	svcIP := svc.Spec.LoadBalancerIP

	var (
		svcIPCidr string
		err       error
	)
	ipReservation := ipReservationByAllTags([]string{svcTag, emTag, clsTag}, ips)

	klog.V(2).Infof("processing %s with existing IP assignment %s", svcName, svcIP)
	// if it already has an IP, no need to get it one
	if svcIP == "" {
		klog.V(2).Infof("no IP assigned for service %s; searching reservations", svcName)

		// if no IP found, request a new one
		if ipReservation == nil {

			// if we did not find an IP reserved, create a request
			klog.V(2).Infof("no IP assignment found for %s, requesting", svcName)
			// create a request
			// our logic as to where to create the IP:
			// 1. if metro is set globally, use it; else
			// 2. if facility is set globally, use it; else
			// 3. if Service.Metadata.Labels["topology.kubernetes.io/region"] is set, use it; else
			// 4. if Service.Metadata.Labels["topology.kubernetes.io/zone"] is set, use it; else
			// 5. Return error, cannot set an EIP
			facility := l.facility
			metro := l.metro
			req := packngo.IPReservationRequest{
				Type:        "public_ipv4",
				Quantity:    1,
				Description: ccmIPDescription,
				Tags: []string{
					emTag,
					svcTag,
					clsTag,
				},
				FailOnApprovalRequired: true,
			}
			switch {
			case svcRegion != "":
				req.Metro = &svcRegion
			case svcZone != "":
				req.Facility = &svcZone
			case metro != "":
				req.Metro = &metro
			case facility != "":
				req.Facility = &facility
			default:
				return "", errors.New("unable to create load balancer when no IP, region or zone specified, either globally or on service")
			}

			ipReservation, _, err = l.client.ProjectIPs.Request(l.project, &req)
			if err != nil {
				return "", fmt.Errorf("failed to request an IP for the load balancer: %w", err)
			}
		}

		// if we have no IP from existing or a new reservation, log it and return
		if ipReservation == nil {
			klog.V(2).Infof("no IP to assign to service %s, will need to wait until it is allocated", svcName)
			return "", nil
		}

		// we have an IP, either found from existing reservations or a new reservation.
		// map and assign it
		svcIP = ipReservation.Address

		// assign the IP and save it
		klog.V(2).Infof("assigning IP %s to %s", svcIP, svcName)
		intf := l.k8sclient.CoreV1().Services(svc.Namespace)
		existing, err := intf.Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil || existing == nil {
			klog.V(2).Infof("failed to get latest for service %s: %v", svcName, err)
			return "", fmt.Errorf("failed to get latest for service %s: %w", svcName, err)
		}
		existing.Spec.LoadBalancerIP = svcIP

		_, err = intf.Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			klog.V(2).Infof("failed to update service %s: %v", svcName, err)
			return "", fmt.Errorf("failed to update service %s: %w", svcName, err)
		}
		klog.V(2).Infof("successfully assigned %s update service %s", svcIP, svcName)
	}
	// our default CIDR for each address is 32
	cidr := 32
	if ipReservation != nil {
		cidr = ipReservation.CIDR
	}
	svcIPCidr = fmt.Sprintf("%s/%d", svcIP, cidr)
	// now need to pass it the nodes

	var n []loadbalancers.Node
	for _, node := range nodes {
		// get the node provider ID
		id := node.Spec.ProviderID
		if id == "" {
			klog.Errorf("no provider ID given for node %s, skipping", node.Name)
			continue
		}
		// ensure BGP is enabled for the node
		if err := ensureNodeBGPEnabled(id, l.client); err != nil {
			klog.Errorf("could not ensure BGP enabled for node %s: %w", node.Name, err)
			continue
		}
		klog.V(2).Infof("bgp enabled on node %s", node.Name)
		// ensure the node has the correct annotations
		if err := l.annotateNode(ctx, node); err != nil {
			klog.Errorf("failed to annotate node %s: %w", node.Name, err)
			continue
		}
		peer, err := getNodeBGPConfig(id, l.client)
		if err != nil || peer == nil {
			klog.Errorf("loadbalancers.addService(): could not get node peer address for node %s: %w", node.Name, err)
			continue
		}
		n = append(n, loadbalancers.Node{
			Name:     node.Name,
			LocalASN: peer.CustomerAs,
			PeerASN:  peer.PeerAs,
			SourceIP: peer.CustomerIP,
			Peers:    peer.PeerIps,
			Password: peer.Md5Password,
		})
	}

	return svcIPCidr, l.implementor.AddService(ctx, svc.Namespace, svc.Name, svcIPCidr, n)
}

func (l *loadBalancers) retrieveIPByTag(ctx context.Context, svc *v1.Service, ips []packngo.IPAddressReservation, tag string) (string, error) {
	svcName := serviceRep(svc)
	svcIP := svc.Spec.LoadBalancerIP
	cidr := 32

	var svcIPCidr string
	ipReservation := ipReservationByAllTags([]string{tag}, ips)

	klog.V(2).Infof("processing %s with existing IP assignment %s", svcName, svcIP)
	// if it already has an IP, no need to get it one
	if svcIP == "" {
		klog.V(2).Infof("no IP assigned for service %s; searching reservations", svcName)

		if ipReservation == nil {
			// if we did not find an IP reserved, create a request
			klog.V(2).Infof("no IP assignment found for %s, returning none", svcName)
			return "", fmt.Errorf("no IP found with tag '%s", tag)
		}

		// we have an IP, map and assign it
		svcIP = ipReservation.Address

		// assign the IP and save it
		klog.V(2).Infof("assigning IP %s to %s", svcIP, svcName)
		intf := l.k8sclient.CoreV1().Services(svc.Namespace)
		existing, err := intf.Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil || existing == nil {
			klog.V(2).Infof("failed to get latest for service %s: %v", svcName, err)
			return "", fmt.Errorf("failed to get latest for service %s: %w", svcName, err)
		}
		existing.Spec.LoadBalancerIP = svcIP

		_, err = intf.Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			klog.V(2).Infof("failed to update service %s: %v", svcName, err)
			return "", fmt.Errorf("failed to update service %s: %w", svcName, err)
		}
		klog.V(2).Infof("successfully assigned %s update service %s", svcIP, svcName)
	}
	if ipReservation != nil {
		cidr = ipReservation.CIDR
	}
	svcIPCidr = fmt.Sprintf("%s/%d", svcIP, cidr)

	return svcIPCidr, nil
}

func serviceRep(svc *v1.Service) string {
	if svc == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
}

func serviceAnnotation(svc *v1.Service, annotation string) string {
	if svc == nil {
		return ""
	}
	if svc.ObjectMeta.Annotations == nil {
		return ""
	}
	return svc.ObjectMeta.Annotations[annotation]
}

func serviceTag(svc *v1.Service) string {
	if svc == nil {
		return ""
	}
	hash := sha256.Sum256([]byte(serviceRep(svc)))
	return fmt.Sprintf("service=%s", base64.StdEncoding.EncodeToString(hash[:]))
}

func clusterTag(clusterID string) string {
	return fmt.Sprintf("cluster=%s", clusterID)
}

// getNodePrivateNetwork use the Equinix Metal API to get the CIDR of the private network given a providerID.
func getNodePrivateNetwork(deviceID string, client *packngo.Client) (string, error) {
	device, _, err := client.Devices.Get(deviceID, &packngo.GetOptions{Includes: []string{"ip_addresses.parent_block,parent_block"}})
	if err != nil {
		return "", err
	}
	for _, net := range device.Network {
		// we only want the private, management, ipv4 network
		if net.Public || !net.Management || net.AddressFamily != 4 {
			continue
		}
		parent := net.ParentBlock
		if parent == nil || parent.Network == "" || parent.CIDR == 0 {
			return "", fmt.Errorf("no network information provided for private address %s", net.String())
		}
		return fmt.Sprintf("%s/%d", parent.Network, parent.CIDR), nil
	}
	return "", nil
}

func filterNodes(nodes []*v1.Node, nodeSelector labels.Selector) []*v1.Node {
	filteredNodes := []*v1.Node{}

	for _, node := range nodes {
		if nodeSelector.Matches(labels.Set(node.Labels)) {
			filteredNodes = append(filteredNodes, node)
		}
	}
	return filteredNodes
}
