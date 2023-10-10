package infrastructure

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

type Pools map[int32][]Target

type Target struct {
	IP   string
	Port int32
}

type Manager struct {
	client         *lbaas.APIClient
	metro          string
	projectID      string
	tokenExchanger *TokenExchanger
}

func NewManager(metalAPIKey, projectID, metro string) *Manager {
	manager := &Manager{}
	emlbConfig := lbaas.NewConfiguration()

	manager.client = lbaas.NewAPIClient(emlbConfig)
	manager.tokenExchanger = &TokenExchanger{
		metalAPIKey: metalAPIKey,
		client:      manager.client.GetConfig().HTTPClient,
	}
	manager.projectID = projectID
	manager.metro = metro

	return manager
}

func (m *Manager) GetMetro() string {
	return m.metro
}

// Returns a Load Balancer object given an id
func (m *Manager) GetLoadBalancer(ctx context.Context, id string) (*lbaas.LoadBalancer, error) {
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, m.tokenExchanger)

	LoadBalancer, _, err := m.client.LoadBalancersApi.GetLoadBalancer(ctx, id).Execute()
	return LoadBalancer, err
}

// Returns a list of Load Balancer objects in the project
func (m *Manager) GetLoadBalancers(ctx context.Context) (*lbaas.LoadBalancerCollection, error) {
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, m.tokenExchanger)

	LoadBalancers, _, err := m.client.ProjectsApi.ListLoadBalancers(ctx, m.projectID).Execute()
	return LoadBalancers, err
}

func (m *Manager) GetPools(ctx context.Context) (*lbaas.LoadBalancerPoolCollection, error) {
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, m.tokenExchanger)

	LoadBalancerPools, _, err := m.client.ProjectsApi.ListPools(ctx, m.projectID).Execute()
	return LoadBalancerPools, err
}

func (m *Manager) CreateLoadBalancer(ctx context.Context, name string, pools Pools) (*lbaas.LoadBalancer, error) {
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, m.tokenExchanger)

	locationId, ok := LBMetros[m.metro]
	if !ok {
		return nil, fmt.Errorf("could not determine load balancer location for metro %v; valid values are %v", m.metro, reflect.ValueOf(LBMetros).MapKeys())
	}

	lbCreateRequest := lbaas.LoadBalancerCreate{
		Name:       name,
		LocationId: locationId,
		ProviderId: ProviderID,
	}

	// TODO lb, resp, err :=
	lbCreated, _, err := m.client.ProjectsApi.CreateLoadBalancer(ctx, m.projectID).LoadBalancerCreate(lbCreateRequest).Execute()
	if err != nil {
		return nil, err
	}

	loadBalancerID := lbCreated.GetId()

	for externalPort, pool := range pools {
		poolName := fmt.Sprintf("%v-pool-%v", name, externalPort)
		poolID, err := m.createPool(ctx, poolName, pool)
		if err != nil {
			return nil, err
		}

		createPortRequest := lbaas.LoadBalancerPortCreate{
			Name:    fmt.Sprintf("%v-port-%v", name, externalPort),
			Number:  externalPort,
			PoolIds: []string{poolID},
		}

		// TODO do we need the port ID for something?
		_, _, err = m.client.PortsApi.CreateLoadBalancerPort(ctx, loadBalancerID).LoadBalancerPortCreate(createPortRequest).Execute()
		if err != nil {
			return nil, err
		}
	}

	lb, _, err := m.client.LoadBalancersApi.GetLoadBalancer(ctx, loadBalancerID).Execute()
	if err != nil {
		return nil, err
	}

	return lb, nil
}

func (m *Manager) UpdateLoadBalancer(ctx context.Context, id string, config map[string]string) (map[string]string, error) {
	outputProperties := map[string]string{}

	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, m.tokenExchanger)

	// TODO delete other resources

	// TODO lb, resp, err :=
	_, _, err := m.client.LoadBalancersApi.UpdateLoadBalancer(ctx, id).Execute()
	if err != nil {
		return nil, err
	}

	return outputProperties, nil
}

func (m *Manager) DeleteLoadBalancer(ctx context.Context, id string) error {
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, m.tokenExchanger)

	lb, _, err := m.client.LoadBalancersApi.GetLoadBalancer(ctx, id).Execute()

	if err != nil {
		return err
	}

	for _, poolGroups := range lb.Pools {
		for _, pool := range poolGroups {
			_, err := m.client.PoolsApi.DeleteLoadBalancerPool(ctx, pool.GetId().(string)).Execute()
			if err != nil {
				return err
			}
		}

	}

	// TODO lb, resp, err :=
	_, err = m.client.LoadBalancersApi.DeleteLoadBalancer(ctx, id).Execute()
	return err
}

func (m *Manager) createPool(ctx context.Context, name string, targets []Target) (string, error) {
	createPoolRequest := lbaas.LoadBalancerPoolCreate{
		Name: name,
		Protocol: lbaas.LoadBalancerPoolCreateProtocol{
			LoadBalancerPoolProtocol: lbaas.LOADBALANCERPOOLPROTOCOL_TCP.Ptr(),
		},
	}

	poolCreated, _, err := m.client.ProjectsApi.CreatePool(ctx, m.projectID).LoadBalancerPoolCreate(createPoolRequest).Execute()

	if err != nil {
		return "", err
	}

	poolID := poolCreated.GetId()

	for i, target := range targets {
		createOriginRequest := lbaas.LoadBalancerPoolOriginCreate{
			Name:   fmt.Sprintf("%v-origin-%v", name, i),
			Target: target.IP,
			PortNumber: lbaas.LoadBalancerPoolOriginPortNumber{
				Int32: &target.Port,
			},
			Active: true,
			PoolId: poolID,
		}
		// TODO do we need the origin IDs for something?
		_, _, err := m.client.PoolsApi.CreateLoadBalancerPoolOrigin(ctx, poolID).LoadBalancerPoolOriginCreate(createOriginRequest).Execute()
		if err != nil {
			return "", err
		}
	}

	return poolID, nil
}
