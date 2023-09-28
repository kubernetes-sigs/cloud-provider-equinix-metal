package emlb

import (
	"context"
	"fmt"
	"reflect"

	lbaas "github.com/equinix/cloud-provider-equinix-metal/internal/lbaas/v1"
)

const ProviderID = "loadpvd-gOB_-byp5ebFo7A3LHv2B"

var LBMetros = map[string]string{
	"da": "lctnloc--uxs0GLeAELHKV8GxO_AI",
	"ny": "lctnloc-Vy-1Qpw31mPi6RJQwVf9A",
	"sv": "lctnloc-H5rl2M2VL5dcFmdxhbEKx",
}

type controller struct {
	client         *lbaas.APIClient
	tokenExchanger *MetalTokenExchanger
}

func NewController(metalAPIKey string) *controller {
	controller := &controller{}
	emlbConfig := lbaas.NewConfiguration()

	controller.client = lbaas.NewAPIClient(emlbConfig)
	controller.tokenExchanger = &MetalTokenExchanger{
		metalAPIKey: metalAPIKey,
		client:      controller.client.GetConfig().HTTPClient,
	}

	return controller
}

func (c *controller) createLoadBalancer(ctx context.Context, config map[string]string) (map[string]string, error) {
	outputProperties := map[string]string{}

	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, c.tokenExchanger)

	metro := config["metro"]

	locationId, ok := LBMetros[metro]
	if !ok {
		return nil, fmt.Errorf("could not determine load balancer location for metro %v; valid values are %v", metro, reflect.ValueOf(LBMetros).Keys())
	}
	lbCreateRequest := lbaas.LoadBalancerCreate{
		Name:       "", // TODO generate from service definition.  Maybe "svcNamespace:svcName"?  Do we need to know the cluster name here?
		LocationId: locationId,
		ProviderId: ProviderID,
	}

	// TODO lb, resp, err :=
	_, _, err := c.client.ProjectsApi.CreateLoadBalancer(ctx, "TODO: project ID").LoadBalancerCreate(lbCreateRequest).Execute()
	if err != nil {
		return nil, err
	}

	// TODO create other resources
	return outputProperties, nil
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

	tokenExchanger := &MetalTokenExchanger{
		metalAPIKey: "TODO",
		client:      c.client.GetConfig().HTTPClient,
	}
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, tokenExchanger)

	// TODO delete other resources

	// TODO lb, resp, err :=
	_, err := c.client.LoadBalancersApi.DeleteLoadBalancer(ctx, id).Execute()
	if err != nil {
		return nil, err
	}

	return outputProperties, nil
}
