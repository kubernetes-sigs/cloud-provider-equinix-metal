package packet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/packethost/packet-ccm/packet/metallb"
	"github.com/packethost/packngo"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"
)

const (
	asn                   = 65000
	bufferSize            = 4096
	checkLoopTimerSeconds = 60
)

type loadBalancers struct {
	client    *packngo.Client
	k8sclient kubernetes.Interface
	project   string
	disabled  bool
	facility  string
	manifest  []byte
}

func newLoadBalancers(client *packngo.Client, projectID, facility string, disabled bool, manifest []byte) *loadBalancers {
	return &loadBalancers{client, nil, projectID, disabled, facility, manifest}
}

func (l *loadBalancers) name() string {
	return "loadbalancer"
}
func (l *loadBalancers) init(k8sclient kubernetes.Interface) error {
	klog.V(2).Info("loadBalancers.init(): started")
	l.k8sclient = k8sclient
	if l.disabled {
		klog.V(2).Info("loadBalancers disabled, not deploying metallb")
		return nil
	}
	// deploy metallb
	klog.V(2).Info("loadBalancers.init(): deploying metallb")
	if err := l.deployMetalLB(); err != nil {
		return fmt.Errorf("failed to deploy metallb: %v", err)
	}
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

// deployMetalLB deploy the metallb config to the kubernetes cluster
func (l *loadBalancers) deployMetalLB() error {
	// read each item in the manifest and deploy it
	// we could use gopkg.in/yaml.v2 or even k8s.io/apimachinery/pkg/util/yaml
	// but we do not need to marshal these into objects, as the server handles it.
	// We just want to split them via --- per https://yaml.org/spec/1.2/spec.html

	// save to k8s
	manifests := bytes.Split(l.manifest, []byte("---"))
	for i, m := range manifests {
		klog.V(2).Infof("applying manifest %s", m)
		_, err := l.k8sclient.CoreV1().RESTClient().Patch(k8stypes.ApplyPatchType).
			Namespace(metalLBNamespace).
			Resource(configMapResource).
			Name(metalLBConfigMapName).
			Body(m).
			Do().
			Get()
		if err != nil {
			return fmt.Errorf("error applying document %d: %v", i, err)
		}
	}
	klog.V(2).Info("all manifests applied")
	return nil
}

func (l *loadBalancers) nodeReconciler() nodeReconciler {
	if l.disabled {
		klog.V(2).Info("loadBalancers disabled, not enabling nodeReconciler")
		return nil
	}
	return l.reconcileNodes
}

func (l *loadBalancers) serviceReconciler() serviceReconciler {
	if l.disabled {
		klog.V(2).Info("loadBalancers disabled, not enabling serviceReconciler")
		return nil
	}
	return l.reconcileServices
}

func (l *loadBalancers) reconcileNodes(nodes []*v1.Node, remove bool) error {
	var (
		peer string
		err  error
	)
	// get the configmap
	cmInterface := l.k8sclient.CoreV1().ConfigMaps(metalLBNamespace)

	config, err := getMetalConfigMap(cmInterface)
	if err != nil {
		return fmt.Errorf("failed to get metallb config map: %v", err)
	}
	for _, node := range nodes {
		// are we adding or removing the node?
		if remove {
			// go through the peers and see if we have one with our hostname.
			selector := metallb.NodeSelector{
				MatchLabels: map[string]string{
					hostnameKey: node.Name,
				},
			}
			config.RemovePeerBySelector(&selector)
		} else {
			// get the node provider ID
			id := node.Spec.ProviderID
			if id == "" {
				return fmt.Errorf("no provider ID given")
			}
			if peer, err = getNodePeerAddress(id, l.client); err != nil {
				klog.Errorf("could not add metallb node peer address for node %s: %v", node.Name, err)
			}
			config = addNodePeer(config, node.Name, peer)
			if config == nil {
				klog.V(2).Info("config unchanged, not updating")
				return nil
			}
		}
		klog.V(2).Info("config changed, updating")
		err = saveUpdatedConfigMap(config, l.k8sclient)
	}
	return nil
}

func (l *loadBalancers) reconcileServices(svcs []*v1.Service, remove bool) error {
	var err error
	// get IP address reservations and check if they any exists for this svc
	ips, _, err := l.client.ProjectIPs.List(l.project)
	if err != nil {
		return fmt.Errorf("unable to retrieve IP reservations for project %s: %v", l.project, err)
	}
	// get the configmap
	cmInterface := l.k8sclient.CoreV1().ConfigMaps(metalLBNamespace)

	config, err := getMetalConfigMap(cmInterface)
	if err != nil {
		return fmt.Errorf("unable to retrieve metallb config map: %v", err)
	}

	for _, svc := range svcs {
		// filter on type: only take those that are of type=LoadBalancer
		if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
			continue
		}

		svcName := serviceRep(svc)
		svcTag := serviceTag(svc)
		svcIP := svc.Spec.LoadBalancerIP
		ipReservation := ipReservationByTags([]string{svcTag, packetTag}, ips)

		klog.V(2).Infof("processing %s with existing IP assignment %s", svcName, svcIP)

		if remove {
			// REMOVAL
			// get the IPs and see if there is anything to clean up
			if ipReservation == nil {
				klog.V(2).Infof("no IP reservation found for %s, nothing to delete", svcName)
				return nil
			}
			// delete the reservation
			_, err = l.client.ProjectIPs.Remove(ipReservation.ID)
			if err != nil {
				return fmt.Errorf("failed to remove IP address reservation %s from project: %v", ipReservation.String(), err)
			}
			// remove it from the configmap
			if err = unmapIP(config, ipReservation.String(), l.k8sclient); err != nil {
				return fmt.Errorf("error mapping IP %s: %v", svcName, err)
			}
		} else {
			// ADDITION
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
							packetTag,
							svcTag,
						},
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
				svcIP = ipReservation.String()
				klog.V(2).Infof("service %s has reserved IP %s", svcName, svcIP)

				// assign the IP and save it
				klog.V(2).Infof("assigning IP %s to %s", svcIP, svcName)
				svc.Spec.LoadBalancerIP = svcIP
				_, err = l.k8sclient.CoreV1().Services(svc.Namespace).Update(svc)
				if err != nil {
					klog.V(2).Infof("failed to update service %s: %v", svcName, err)
					return fmt.Errorf("failed to update service %s: %v", svcName, err)
				}
				klog.V(2).Infof("successfully assigned %s update service %s", svcIP, svcName)
			}
			// Update the service and configmap and save them
			if err = mapIP(config, svcIP, svcName, l.k8sclient); err != nil {
				return fmt.Errorf("error mapping IP %s: %v", svcName, err)
			}
		}
	}
	return nil
}

// getNodePeerAddress get the BGP peer address for a specific node
func getNodePeerAddress(device string, client *packngo.Client) (address string, err error) {
	ips, _, err := client.DeviceIPs.List(device, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get device IPs for device %s: %v", device, err)
	}
	// we need to get the ip address that is all of:
	// - AddressFamily == 4
	// - Public == false
	// - Management == true
	var addr string
	for _, ip := range ips {
		if ip.AddressFamily == 4 && !ip.Public && ip.Management {
			addr = ip.Network
			break
		}
	}
	if addr == "" {
		return addr, errors.New("no matching IP address found that is private+ipv4+management")
	}
	return addr, nil
}

// addNodePeer update the configmap to ensure that the given node has the given peer
func addNodePeer(config *metallb.ConfigFile, nodeName string, peer string) *metallb.ConfigFile {
	ns := metallb.NodeSelector{
		MatchLabels: map[string]string{
			hostnameKey: nodeName,
		},
	}
	p := metallb.Peer{
		MyASN:         localASN,
		ASN:           peerASN,
		Addr:          peer,
		NodeSelectors: []metallb.NodeSelector{ns},
	}
	config.AddPeer(&p)
	return config
}

func saveUpdatedConfigMap(cfg *metallb.ConfigFile, client kubernetes.Interface) error {
	b, err := cfg.Bytes()
	if err != nil {
		return fmt.Errorf("error converting configmap to bytes: %v", err)
	}

	// save to k8s
	_, err = client.CoreV1().RESTClient().Patch(k8stypes.ApplyPatchType).
		Namespace(metalLBNamespace).
		Resource(configMapResource).
		Name(metalLBConfigMapName).
		Body(b).
		Do().
		Get()

	return err
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
	return fmt.Sprintf("service=%s", hash)
}

func getMetalConfigMap(getter typedv1.ConfigMapInterface) (*metallb.ConfigFile, error) {
	cm, err := getter.Get(metalLBConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get metallb configmap %s: %v", metalLBConfigMapName, err)
	}
	var (
		configData []byte
		ok         bool
	)
	if configData, ok = cm.BinaryData["config"]; !ok {
		return nil, errors.New("configmap data has no property 'config'")
	}
	return metallb.ParseConfig(configData)
}

// unmapIP remove a given IP address from the metalllb config map
func unmapIP(config *metallb.ConfigFile, addr string, k8sclient kubernetes.Interface) error {
	klog.V(2).Infof("unmapping IP %s", addr)
	return updateMapIP(config, addr, "", k8sclient, false)
}

// mapIP add a given ip address to the metallb configmap
func mapIP(config *metallb.ConfigFile, addr, svcName string, k8sclient kubernetes.Interface) error {
	klog.V(2).Infof("mapping IP %s", addr)
	return updateMapIP(config, addr, svcName, k8sclient, true)
}
func updateMapIP(config *metallb.ConfigFile, addr, svcName string, k8sclient kubernetes.Interface, add bool) error {
	// update the configmap and save it
	if add {
		autoAssign := false
		config.AddAddressPool(&metallb.AddressPool{
			Protocol:   "bgp",
			Name:       svcName,
			Addresses:  []string{addr},
			AutoAssign: &autoAssign,
		})
	} else {
		config.RemoveAddressPoolByAddress(addr)
	}
	if config == nil {
		klog.V(2).Info("config unchanged, not updating")
		return nil
	}
	klog.V(2).Info("config changed, updating")
	if err := saveUpdatedConfigMap(config, k8sclient); err != nil {
		klog.V(2).Infof("error updating configmap: %v", err)
		return fmt.Errorf("failed to update configmap: %v", err)
	}
	return nil
}
