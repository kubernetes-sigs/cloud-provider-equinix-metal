package metal

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/empty"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/kubevip"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/metallb"
	"github.com/packethost/packngo"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	bufferSize = 4096
)

type loadBalancers struct {
	client            *packngo.Client
	k8sclient         kubernetes.Interface
	project           string
	facility          string
	clusterID         string
	implementor       loadbalancers.LB
	implementorConfig string
}

func newLoadBalancers(client *packngo.Client, projectID, facility string, config string) *loadBalancers {
	return &loadBalancers{client, nil, projectID, facility, "", nil, config}
}

func (l *loadBalancers) name() string {
	return "loadbalancer"
}
func (l *loadBalancers) init(k8sclient kubernetes.Interface) error {
	klog.V(2).Info("loadBalancers.init(): started")
	// parse the implementor config and see what kind it is - allow for no config
	if l.implementorConfig == "" {
		klog.V(2).Info("loadBalancers.init(): no loadbalancer implementation config, skipping")
		return nil
	}

	l.k8sclient = k8sclient
	// get the UID of the kube-system namespace
	systemNamespace, err := k8sclient.CoreV1().Namespaces().Get(context.Background(), "kube-system", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get kube-system namespace: %v", err)
	}
	if systemNamespace == nil {
		return fmt.Errorf("kube-system namespace is missing unexplainably")
	}

	u, err := url.Parse(l.implementorConfig)
	if err != nil {
		return fmt.Errorf("invalid config: %v", err)
	}
	config := u.Path
	var impl loadbalancers.LB
	switch u.Scheme {
	case "kube-vip":
		klog.Info("loadbalancer implementation enabled: kube-vip")
		impl = kubevip.NewLB(k8sclient, config)
	case "metallb":
		klog.Info("loadbalancer implementation enabled: metallb")
		impl = metallb.NewLB(k8sclient, config)
	case "empty":
		klog.Info("loadbalancer implementation enabled: empty, bgp only")
		impl = empty.NewLB(k8sclient, config)
	default:
		klog.Info("loadbalancer implementation disabled")
		impl = nil
	}

	l.clusterID = string(systemNamespace.UID)
	l.implementor = impl
	klog.V(2).Info("loadBalancers.init(): complete")
	return nil
}

// implementation of cloudprovider.LoadBalancer
// we do this via metallb, not directly, so none of this works... for now.

func (l *loadBalancers) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	return nil, false, nil
}
func (l *loadBalancers) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	return ""
}
func (l *loadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	return nil, nil
}
func (l *loadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	return nil
}
func (l *loadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	return nil
}

// utility funcs

func (l *loadBalancers) nodeReconciler() nodeReconciler {
	if l.implementor == nil {
		klog.V(2).Info("loadBalancers disabled, not enabling nodeReconciler")
		return nil
	}
	return l.reconcileNodes
}

func (l *loadBalancers) serviceReconciler() serviceReconciler {
	if l.implementor == nil {
		klog.V(2).Info("loadBalancers disabled, not enabling serviceReconciler")
		return nil
	}
	return l.reconcileServices
}

// reconcileNodes given a node, update the metallb load balancer by
// by adding it to or removing it from the known metallb configmap
func (l *loadBalancers) reconcileNodes(ctx context.Context, nodes []*v1.Node, mode UpdateMode) error {
	var (
		peer *packngo.BGPNeighbor
		err  error
	)
	klog.V(2).Infof("loadbalancers.reconcileNodes(): called for nodes %v", nodes)

	// are we adding, removing or syncing the node?
	switch mode {
	case ModeRemove:
		for _, node := range nodes {
			klog.V(2).Infof("loadbalancers.reconcileNodes(): reconciling remove node %s", node.Name)
			if err := l.implementor.RemoveNode(ctx, node.Name); err != nil {
				klog.V(2).Infof("loadbalancers.reconcileNodes(): error removing node %s: %v", node.Name, err)
				continue
			}
		}
	case ModeAdd:
		for _, node := range nodes {
			klog.V(2).Infof("loadbalancers.reconcileNodes(): reconciling add node %s", node.Name)
			// get the node provider ID
			id := node.Spec.ProviderID
			if id == "" {
				return fmt.Errorf("no provider ID given for node %s", node.Name)
			}
			if peer, err = getNodeBGPConfig(id, l.client); err != nil || peer == nil {
				klog.Errorf("loadbalancers.reconcileNodes(): could not add metallb node peer address for node %s: %v", node.Name, err)
				continue
			}
			if err := l.implementor.AddNode(ctx, node.Name, peer.CustomerAs, peer.PeerAs, peer.Md5Password, peer.CustomerIP, peer.PeerIps...); err != nil {
				klog.V(2).Infof("loadbalancers.reconcileNodes(): error adding node %s: %v", node.Name, err)
				continue
			}
		}
	case ModeSync:
		// make sure the list of nodes exactly matches between the provided nodes and the ones in the configmap
		goodMap := map[string]loadbalancers.Node{}
		for _, node := range nodes {
			// get the node provider ID
			id := node.Spec.ProviderID
			if id == "" {
				return fmt.Errorf("no provider ID given for node %s", node.Name)
			}
			if peer, err = getNodeBGPConfig(id, l.client); err != nil || peer == nil {
				klog.Errorf("loadbalancers.reconcileNodes(): could not get node peer address for node %s: %v", node.Name, err)
				continue
			}
			goodMap[node.Name] = loadbalancers.Node{
				Name:     node.Name,
				LocalASN: peer.CustomerAs,
				PeerASN:  peer.PeerAs,
				SourceIP: peer.CustomerIP,
				Peers:    peer.PeerIps,
				Password: peer.Md5Password,
			}
		}
		if err := l.implementor.SyncNodes(ctx, goodMap); err != nil {
			return fmt.Errorf("error syncing nodes: %v", err)
		}
	}
	klog.V(2).Infof("loadbalancers.reconcileNodes(): config changed, done")
	return nil
}

// reconcileServices add or remove services to have loadbalancers. If it adds a
// service, then it requests a new IP reservation, with "fast-fail", i.e. if it
// cannot create the IP reservation immediately, then it fails, rather than
// waiting for human support. It tags the IP reservation so it can find it later.
// Before trying to create one, it tries to find an IP reservation with the right tags.
func (l *loadBalancers) reconcileServices(ctx context.Context, svcs []*v1.Service, mode UpdateMode) error {
	klog.V(2).Infof("loadbalancer.reconcileServices(): %v starting", mode)
	klog.V(5).Infof("loadbalancer.reconcileServices(): services %#v", svcs)

	var err error
	// get IP address reservations and check if they any exists for this svc
	ips, _, err := l.client.ProjectIPs.List(l.project, &packngo.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to retrieve IP reservations for project %s: %v", l.project, err)
	}

	validSvcs := []*v1.Service{}
	for _, svc := range svcs {
		// filter on type: only take those that are of type=LoadBalancer
		if svc.Spec.Type == v1.ServiceTypeLoadBalancer {
			validSvcs = append(validSvcs, svc)
		}
	}
	klog.V(5).Infof("loadbalancer.reconcileServices(): valid services %#v", validSvcs)

	switch mode {
	case ModeAdd:
		// ADDITION
		for _, svc := range validSvcs {
			klog.V(2).Infof("loadbalancer.reconcileServices(): add: service %s", svc.Name)
			if err := l.addService(ctx, svc, ips); err != nil {
				return err
			}
		}
	case ModeRemove:
		// REMOVAL
		for _, svc := range validSvcs {
			svcName := serviceRep(svc)
			svcTag := serviceTag(svc)
			clsTag := clusterTag(l.clusterID)
			svcIP := svc.Spec.LoadBalancerIP

			var svcIPCidr string
			ipReservation := ipReservationByAllTags([]string{svcTag, emTag, clsTag}, ips)

			klog.V(2).Infof("loadbalancer.reconcileServices(): remove: %s with existing IP assignment %s", svcName, svcIP)

			// get the IPs and see if there is anything to clean up
			if ipReservation == nil {
				klog.V(2).Infof("loadbalancer.reconcileServices(): remove: no IP reservation found for %s, nothing to delete", svcName)
				continue
			}
			// delete the reservation
			klog.V(2).Infof("loadbalancer.reconcileServices(): remove: for %s EIP ID %s", svcName, ipReservation.ID)
			_, err = l.client.ProjectIPs.Remove(ipReservation.ID)
			if err != nil {
				return fmt.Errorf("failed to remove IP address reservation %s from project: %v", ipReservation.String(), err)
			}
			// remove it from the configmap
			svcIPCidr = fmt.Sprintf("%s/%d", ipReservation.Address, ipReservation.CIDR)
			klog.V(2).Infof("loadbalancer.reconcileServices(): remove: for %s entry %s", svcName, svcIPCidr)
			if err := l.implementor.RemoveService(ctx, svcIPCidr); err != nil {
				return fmt.Errorf("error removing IP from configmap for %s: %v", svcName, err)
			}
			klog.V(2).Infof("loadbalancer.reconcileServices(): remove: removed service %s from implementation", svcName)
		}
	case ModeSync:
		// what we have to do:
		// 1. get all of the services that are of type=LoadBalancer
		// 2. for each service, get its eip, if available. if it does not have one, create one for it.
		// 3. for each EIP, ensure it exists in the configmap
		// 4. get each EIP in the configmap, check if it is in our list; if not, delete

		// add each service that is in the known list
		for _, svc := range validSvcs {
			klog.V(2).Infof("loadbalancer.reconcileServices(): sync: service %s", svc.Name)
			if err := l.addService(ctx, svc, ips); err != nil {
				return err
			}
		}

		// remove any service that is not in the known list

		// we need to get the addresses again, because we might have changed them
		klog.V(5).Info("loadbalancer.reconcileServices(): sync: getting all IP reservations")
		ips, _, err = l.client.ProjectIPs.List(l.project, &packngo.ListOptions{})
		if err != nil {
			return fmt.Errorf("unable to retrieve IP reservations for project %s: %v", l.project, err)
		}
		// get all EIP that have the equinix metal tag and are allocated to this cluster
		ipReservations := ipReservationsByAllTags([]string{emTag, clusterTag(l.clusterID)}, ips)
		// create a map of EIP to svcIP so we can get the CIDR
		ipCidr := map[string]int{}
		for _, ipr := range ipReservations {
			ipCidr[ipr.Address] = ipr.CIDR
		}

		// create a map of all valid IPs
		validTags := map[string]bool{}
		validIPs := map[string]bool{}

		for _, svc := range validSvcs {
			validTags[serviceTag(svc)] = true
			svcIP := svc.Spec.LoadBalancerIP
			if svcIP != "" {
				if cidr, ok := ipCidr[svcIP]; ok {
					validIPs[fmt.Sprintf("%s/%d", svcIP, cidr)] = true
				}
			}
		}

		klog.V(2).Infof("loadbalancer.reconcileServices(): sync: valid tags %v", validTags)
		klog.V(2).Infof("loadbalancer.reconcileServices(): sync: valid svc IPs %v", validIPs)

		if err := l.implementor.SyncServices(ctx, validIPs); err != nil {
			return err
		}

		// remove any EIPs that do not have a reservation

		klog.V(5).Infof("loadbalancer.reconcileServices(): sync: all reservations with emTag %#v", ipReservations)
		for _, ipReservation := range ipReservations {
			var foundTag bool
			for _, tag := range ipReservation.Tags {
				if _, ok := validTags[tag]; ok {
					foundTag = true
				}
			}
			// did we find a valid tag?
			if !foundTag {
				klog.V(2).Infof("loadbalancer.reconcileServices(): sync: removing reservation with service= tag but not in validTags list %#v", ipReservation)
				// delete the reservation
				_, err = l.client.ProjectIPs.Remove(ipReservation.ID)
				if err != nil {
					return fmt.Errorf("failed to remove IP address reservation %s from project: %v", ipReservation.String(), err)
				}
			}
		}
	}
	return nil
}

// addService add a single service; wraps the implementation
func (l *loadBalancers) addService(ctx context.Context, svc *v1.Service, ips []packngo.IPAddressReservation) error {
	svcName := serviceRep(svc)
	svcTag := serviceTag(svc)
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
			facility := l.facility
			req := packngo.IPReservationRequest{
				Type:        "public_ipv4",
				Quantity:    1,
				Description: ccmIPDescription,
				Facility:    &facility,
				Tags: []string{
					emTag,
					svcTag,
					clsTag,
				},
				FailOnApprovalRequired: true,
			}

			ipReservation, _, err = l.client.ProjectIPs.Request(l.project, &req)
			if err != nil {
				return fmt.Errorf("failed to request an IP for the load balancer: %v", err)
			}
		}

		// if we have no IP from existing or a new reservation, log it and return
		if ipReservation == nil {
			klog.V(2).Infof("no IP to assign to service %s, will need to wait until it is allocated", svcName)
			return nil
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
			return fmt.Errorf("failed to get latest for service %s: %v", svcName, err)
		}
		existing.Spec.LoadBalancerIP = svcIP

		_, err = intf.Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			klog.V(2).Infof("failed to update service %s: %v", svcName, err)
			return fmt.Errorf("failed to update service %s: %v", svcName, err)
		}
		klog.V(2).Infof("successfully assigned %s update service %s", svcIP, svcName)
	}
	// our default CIDR for each address is 32
	cidr := 32
	if ipReservation != nil {
		cidr = ipReservation.CIDR
	}
	svcIPCidr = fmt.Sprintf("%s/%d", svcIP, cidr)
	return l.implementor.AddService(ctx, svcName, svcIPCidr)
}

func serviceRep(svc *v1.Service) string {
	if svc == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
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
