// Implementation of Equinix Metal Load Balancer
package emlb

import (
	"context"
	"errors"
	"strings"

	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/emlb/infrastructure"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

type LB struct {
	manager   *infrastructure.Manager
	k8sclient kubernetes.Interface
	client    client.Client
}

const (
	LoadBalancerIDAnnotation = "equinix.com/loadbalancerID"
)

var _ loadbalancers.LB = (*LB)(nil)

func NewLB(k8sclient kubernetes.Interface, config, metalAPIKey, projectID string) *LB {
	// Parse config for Equinix Metal Load Balancer
	// The format is emlb:///<location>
	// An example config using Dallas as the location would look like emlb:///da
	// it may have an extra slash at the beginning or end, so get rid of it
	metro := strings.TrimPrefix(config, "/")

	// Create a new LB object.
	lb := &LB{}

	// Set the manager subobject to have the API key and project id and metro.
	lb.manager = infrastructure.NewManager(metalAPIKey, projectID, metro)

	// Pass the k8sclient into the LB object.
	lb.k8sclient = k8sclient

	// Set up a new controller-runtime k8s client for LB object.
	scheme := runtime.NewScheme()
	err := v1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	newClient, err := client.New(clientconfig.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		panic(err)
	}
	lb.client = newClient

	return lb
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node, svc *v1.Service, n []*v1.Node, loadBalancerName string) error {
	if svc.Annotations[LoadBalancerIDAnnotation] != "" {
		// TODO UpdateService will be updated later to accept *v1.Service and []*v1.Nodes
		return l.UpdateService(ctx, svcNamespace, svcName, nodes)
	}
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

	svc.Annotations[LoadBalancerIDAnnotation] = loadBalancer.GetId()
	svc.Annotations["equinix.com/loadbalancerMetro"] = l.manager.GetMetro()

	return l.client.Update(ctx, svc)
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string, svc *v1.Service) error {
	// 1. Gather the properties we need: ID of load balancer
	loadBalancerId := svc.Annotations[LoadBalancerIDAnnotation]

	// 2. Delete the infrastructure (do we need to return anything here?)
	err := l.manager.DeleteLoadBalancer(ctx, loadBalancerId)

	return err
}

func (l *LB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node) error {
	/*
		1. Gather the properties we need:
			- load balancer ID
			- NodePort
			- Public IP addresses of the nodes on which the target pods are running
	*/

	// 2. Update infrastructure change (do we need to return anything here? or are all changes reflected by properties from [1]?)

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
func (l *LB) GetLoadBalancerList(ctx context.Context) ([]string, error) {
	var lbList []string

	lbCollection, err := l.manager.GetLoadBalancers(ctx)

	for _, lb := range lbCollection.Loadbalancers {
		lbList = append(lbList, lb.Name)
	}

	return lbList, err
}
