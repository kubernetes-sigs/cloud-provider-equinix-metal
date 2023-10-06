// Implementation of Equinix Metal Load Balancer
package emlb

import (
	"context"
	"errors"
	"strings"

	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/emlb/infrastructure"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type LB struct {
	manager   *infrastructure.Manager
	k8sclient kubernetes.Interface
}

var _ loadbalancers.LB = (*LB)(nil)

func NewLB(k8sclient kubernetes.Interface, config, metalAPIKey, projectID string) *LB {
	// Parse config for Equinix Metal Load Balancer
	// The format is emlb:///<location>
	// An example config using Dallas as the location would look like emlb:///da
	// it may have an extra slash at the beginning or end, so get rid of it
	metro := strings.TrimPrefix(config, "/")

	lb := &LB{}
	lb.manager = infrastructure.NewManager(metalAPIKey, projectID, metro)
	lb.k8sclient = k8sclient

	return lb
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node, svc *v1.Service, n []*v1.Node, loadBalancerName string) error {

	if len(svc.Spec.Ports) < 1 {
		return errors.New("cannot add loadbalancer service; no ports assigned")
	}

	pools := l.convertToPools(svc, n)

	loadBalancer, err := l.manager.CreateLoadBalancer(ctx, loadBalancerName, pools)

	if err != nil {
		return err
	}

	var ingress []v1.LoadBalancerIngress
	for _, ip := range loadBalancer.GetIps() {
		ingress = append(ingress, v1.LoadBalancerIngress{
			IP: ip,
		})
		// TODO: this is here for backwards compatibility and should be removed ASAP
		svc.Spec.LoadBalancerIP = ip
	}

	// TODO: THIS DOES NOT ACTUALLY UPDATE THE SERVICE!
	svc.Status.LoadBalancer.Ingress = ingress

	svc.Annotations["equinix.com/loadbalancerID"] = loadBalancer.GetId()
	svc.Annotations["equinix.com/loadbalancerMetro"] = l.manager.GetMetro()

	return nil
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string) error {
	// 1. Gather the properties we need: ID of load balancer
	loadBalancerId := "TODO"
	additionalProperties := map[string]string{}

	// 2. Delete the infrastructure (do we need to return anything here?)
	_, err := l.manager.DeleteLoadBalancer(ctx, loadBalancerId, additionalProperties)

	if err != nil {
		return err
	}

	// 3. No need to remove the annotations because the annotated object was deleted

	return nil
}

func (l *LB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node) error {
	/*
		1. Gather the properties we need:
			- load balancer ID
			- NodePort
			- Public IP addresses of the nodes on which the target pods are running
	*/
	loadBalancerId := "TODO"
	additionalProperties := map[string]string{}

	// 2. Update infrastructure change (do we need to return anything here? or are all changes reflected by properties from [1]?)
	_, err := l.manager.UpdateLoadBalancer(ctx, loadBalancerId, additionalProperties)

	if err != nil {
		return err
	}

	/*
		3. Update the annotations
			- Listener port that this service is using
	*/

	return nil
}

func (l *LB) convertToPools(svc *v1.Service, nodes []*v1.Node) infrastructure.Pools {
	pools := infrastructure.Pools{}
	for _, svcPort := range svc.Spec.Ports {
		targets := []infrastructure.Target{}
		for _, node := range nodes {
			for _, address := range node.Status.Addresses {
				if address.Type == v1.NodeExternalIP {
					targets = append(targets, infrastructure.Target{
						IP:   address.Address,
						Port: svcPort.NodePort,
					})
				}
			}
		}
		pools[svcPort.Port] = targets
	}

	return pools
}
