package emlb

import (
	"context"
	"fmt"
	"reflect"

	lbaas "github.com/equinix/cloud-provider-equinix-metal/internal/lbaas/v1"
	"k8s.io/client-go/kubernetes"
)

const ProviderID = "loadpvd-gOB_-byp5ebFo7A3LHv2B"

var LBMetros = map[string]string{
	"da": "lctnloc--uxs0GLeAELHKV8GxO_AI",
	"ny": "lctnloc-Vy-1Qpw31mPi6RJQwVf9A",
	"sv": "lctnloc-H5rl2M2VL5dcFmdxhbEKx",
}

type controller struct {
	client         *lbaas.APIClient
	k8sclient      kubernetes.Interface
	metro          string
	projectID      string
	tokenExchanger *MetalTokenExchanger
}

func NewController(k8sclient kubernetes.Interface, metalAPIKey, projectID, metro string) *controller {
	controller := &controller{}
	emlbConfig := lbaas.NewConfiguration()

	controller.client = lbaas.NewAPIClient(emlbConfig)
	controller.tokenExchanger = &MetalTokenExchanger{
		metalAPIKey: metalAPIKey,
		client:      controller.client.GetConfig().HTTPClient,
	}
	controller.projectID = projectID
	controller.metro = metro

	return controller
}

func (c *controller) createLoadBalancer(ctx context.Context, name string, port int32, nodePort int32, ips []string) (*lbaas.LoadBalancer, error) {
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, c.tokenExchanger)

	locationId, ok := LBMetros[c.metro]
	if !ok {
		return nil, fmt.Errorf("could not determine load balancer location for metro %v; valid values are %v", c.metro, reflect.ValueOf(LBMetros).MapKeys())
	}

	lbCreateRequest := lbaas.LoadBalancerCreate{
		Name:       name,
		LocationId: locationId,
		ProviderId: ProviderID,
	}

	// TODO lb, resp, err :=
	lbCreated, _, err := c.client.ProjectsApi.CreateLoadBalancer(ctx, c.projectID).LoadBalancerCreate(lbCreateRequest).Execute()
	if err != nil {
		return nil, err
	}

	loadBalancerID := lbCreated.GetId()

	createPoolRequest := lbaas.LoadBalancerPoolCreate{
		Name: fmt.Sprintf("%v-pool", name),
		Protocol: lbaas.LoadBalancerPoolCreateProtocol{
			LoadBalancerPoolProtocol: lbaas.LOADBALANCERPOOLPROTOCOL_TCP.Ptr(),
		},
	}

	poolCreated, _, err := c.client.ProjectsApi.CreatePool(ctx, c.projectID).LoadBalancerPoolCreate(createPoolRequest).Execute()
	if err != nil {
		return nil, err
	}

	poolID := poolCreated.GetId()

	for i, ip := range ips {
		createOriginRequest := lbaas.LoadBalancerPoolOriginCreate{
			Name:   fmt.Sprintf("%v-origin-%v", name, i),
			Target: ip,
			PortNumber: lbaas.LoadBalancerPoolOriginPortNumber{
				Int32: &nodePort,
			},
			Active: true,
			PoolId: poolID,
		}
		// TODO do we need the origin IDs for something?
		_, _, err := c.client.PoolsApi.CreateLoadBalancerPoolOrigin(ctx, poolID).LoadBalancerPoolOriginCreate(createOriginRequest).Execute()
		if err != nil {
			return nil, err
		}
	}

	createPortRequest := lbaas.LoadBalancerPortCreate{
		Name:    fmt.Sprintf("%v-port-%v", name, port),
		Number:  port,
		PoolIds: []string{poolID},
	}

	// TODO do we need the port ID for something?
	_, _, err = c.client.PortsApi.CreateLoadBalancerPort(ctx, loadBalancerID).LoadBalancerPortCreate(createPortRequest).Execute()
	if err != nil {
		return nil, err
	}

	lb, _, err := c.client.LoadBalancersApi.GetLoadBalancer(ctx, loadBalancerID).Execute()
	if err != nil {
		return nil, err
	}

	return lb, nil
}

func (c *controller) updateLoadBalancer(ctx context.Context, id string, config map[string]string) (map[string]string, error) {
	outputProperties := map[string]string{}

	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, c.tokenExchanger)

	// TODO delete other resources

	// TODO lb, resp, err :=
	_, err := c.client.LoadBalancersApi.DeleteLoadBalancer(ctx, id).Execute()
	if err != nil {
		return nil, err
	}

	return outputProperties, nil
}

func (c *controller) deleteLoadBalancer(ctx context.Context, id string, config map[string]string) (map[string]string, error) {
	outputProperties := map[string]string{}
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, c.tokenExchanger)

	// TODO delete other resources

	// TODO lb, resp, err :=
	_, err := c.client.LoadBalancersApi.DeleteLoadBalancer(ctx, id).Execute()
	if err != nil {
		return nil, err
	}

	return outputProperties, nil
}
