// Implementation of Equinix Metal Load Balancer
package emlb

import (
	"context"
	"fmt"
	"reflect"

	lbaas "github.com/equinix/cloud-provider-equinix-metal/internal/lbaas/v1"
	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers"
	"k8s.io/client-go/kubernetes"
)

const ProviderID = "loadpvd-gOB_-byp5ebFo7A3LHv2B"

var LBMetros = map[string]string{
	"da": "lctnloc--uxs0GLeAELHKV8GxO_AI",
	"ny": "lctnloc-Vy-1Qpw31mPi6RJQwVf9A",
	"sv": "lctnloc-H5rl2M2VL5dcFmdxhbEKx",
}

type LB struct {
	client               *lbaas.APIClient
	loadBalancerLocation *lbaas.LoadBalancerLocation
}

func NewLB(k8sclient kubernetes.Interface, config string) *LB {
	// Parse config for Equinix Metal Load Balancer
	// An example config using Dallas as the location would look like
	// The format is emlb://<location>
	// it may have an extra slash at the beginning or end, so get rid of it

	lb := &LB{}
	emlbConfig := lbaas.NewConfiguration()
	lb.client = lbaas.NewAPIClient(emlbConfig)
	lb.loadBalancerLocation.Id = &config
	return lb
}

func (l *LB) AddService(ctx context.Context, svcNamespace, svcName, ip string, nodes []loadbalancers.Node) error {
	tokenExchanger := &MetalTokenExchanger{
		metalAPIKey: "", // TODO pass this in somehow (maybe add it to the context?)
		client:      l.client.GetConfig().HTTPClient,
	}
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, tokenExchanger)

	metro := "da" // TODO get this from somewhere else

	locationId, ok := LBMetros[metro]
	if !ok {
		return fmt.Errorf("could not determine load balancer location for metro %v; valid values are %v", metro, reflect.ValueOf(LBMetros).Keys())
	}
	lbCreateRequest := lbaas.LoadBalancerCreate{
		Name:       "", // TODO generate from service definition.  Maybe "svcNamespace:svcName"?  Do we need to know the cluster name here?
		LocationId: locationId,
		ProviderId: ProviderID,
	}

	// TODO lb, resp, err :=
	_, _, err := l.client.ProjectsApi.CreateLoadBalancer(ctx, "TODO: project ID").LoadBalancerCreate(lbCreateRequest).Execute()
	if err != nil {
		return err
	}

	// TODO create other resources

	return nil
}

func (l *LB) RemoveService(ctx context.Context, svcNamespace, svcName, ip string) error {
	tokenExchanger := &MetalTokenExchanger{
		metalAPIKey: "TODO",
		client:      l.client.GetConfig().HTTPClient,
	}
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, tokenExchanger)

	loadBalancerId := "TODO"

	// TODO delete other resources

	// TODO lb, resp, err :=
	_, err := l.client.LoadBalancersApi.DeleteLoadBalancer(ctx, loadBalancerId).Execute()
	if err != nil {
		return err
	}

	return nil
}

func (l *LB) UpdateService(ctx context.Context, svcNamespace, svcName string, nodes []loadbalancers.Node) error {
	return nil
}
