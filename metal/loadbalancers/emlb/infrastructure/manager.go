package infrastructure

import (
	"context"
	"fmt"
	"net/http"
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

func (m *Manager) DeleteLoadBalancer(ctx context.Context, id string) error {
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, m.tokenExchanger)

	lb, _, err := m.client.LoadBalancersApi.GetLoadBalancer(ctx, id).Execute()

	if err != nil {
		return err
	}

	for _, poolGroups := range lb.Pools {
		for _, pool := range poolGroups {
			_, err := m.client.PoolsApi.DeleteLoadBalancerPool(ctx, pool.GetId()).Execute()
			if err != nil {
				return err
			}
		}

	}

	// TODO lb, resp, err :=
	_, err = m.client.LoadBalancersApi.DeleteLoadBalancer(ctx, id).Execute()
	return err
}

func (m *Manager) ReconcileLoadBalancer(ctx context.Context, id, name string, pools Pools) (*lbaas.LoadBalancer, error) {
	ctx = context.WithValue(ctx, lbaas.ContextOAuth2, m.tokenExchanger)

	if id == "" {
		locationId, ok := LBMetros[m.metro]
		if !ok {
			return nil, fmt.Errorf("could not determine load balancer location for metro %v; valid values are %v", m.metro, reflect.ValueOf(LBMetros).MapKeys())
		}

		lbCreateRequest := lbaas.LoadBalancerCreate{
			Name:       name,
			LocationId: locationId,
			ProviderId: ProviderID,
		}

		lbCreated, _, err := m.client.ProjectsApi.CreateLoadBalancer(ctx, m.projectID).LoadBalancerCreate(lbCreateRequest).Execute()
		if err != nil {
			return nil, err
		}

		id = lbCreated.GetId()
	}

	lb, _, err := m.client.LoadBalancersApi.GetLoadBalancer(ctx, id).Execute()
	if err != nil {
		return nil, err
	}

	existingPorts := map[int32]struct{}{}
	existingPools := lb.GetPools()
	// Update or delete existing targets
	for i, port := range lb.GetPorts() {
		portNumber := port.GetNumber()
		targets, wanted := pools[portNumber]
		if wanted {
			// We have a pool for this port and we want to keep it
			existingPorts[portNumber] = struct{}{}

			for _, existingPool := range existingPools[i] {
				existingOrigins, _, err := m.client.PoolsApi.ListLoadBalancerPoolOrigins(ctx, existingPool.GetId()).Execute()
				if err != nil {
					return nil, err
				}

				// TODO: can/should we be more granular here? figure out which to add and which to update?

				// Create new origins for all targets
				for j, target := range targets {
					_, _, err := m.createOrigin(ctx, existingPool.GetId(), existingPool.GetName(), int32(j), target)
					if err != nil {
						return nil, err
					}
				}

				// Delete old origins (some of which may be duplicates of the new ones)
				for _, origin := range existingOrigins.GetOrigins() {
					_, err := m.client.OriginsApi.DeleteLoadBalancerOrigin(ctx, origin.GetId()).Execute()
					if err != nil {
						return nil, err
					}
				}
			}
		} else {
			// We have a pool for this port and we want to get rid of it
			for _, existingPool := range existingPools[i] {
				_, err := m.client.PoolsApi.DeleteLoadBalancerPool(ctx, existingPool.GetId()).Execute()
				if err != nil {
					return nil, err
				}
			}
			_, err := m.client.PortsApi.DeleteLoadBalancerPort(ctx, port.GetId()).Execute()
			if err != nil {
				return nil, err
			}
		}
	}

	// Create ports & pools for new targets
	for externalPort, pool := range pools {
		if _, exists := existingPorts[externalPort]; !exists {
			poolID, err := m.createPool(ctx, getResourceName(lb.GetName(), "pool", externalPort), pool)
			if err != nil {
				return nil, err
			}

			createPortRequest := lbaas.LoadBalancerPortCreate{
				Name:    getResourceName(lb.GetName(), "port", externalPort),
				Number:  externalPort,
				PoolIds: []string{poolID},
			}

			// TODO do we need the port ID for something?
			_, _, err = m.client.PortsApi.CreateLoadBalancerPort(ctx, id).LoadBalancerPortCreate(createPortRequest).Execute()
			if err != nil {
				return nil, err
			}
		}
	}

	lb, _, err = m.client.LoadBalancersApi.GetLoadBalancer(ctx, id).Execute()

	return lb, err
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
		// TODO do we need the origin IDs for something?
		_, _, err := m.createOrigin(ctx, poolID, name, int32(i), target)
		if err != nil {
			return "", err
		}
	}

	return poolID, nil
}

func (m *Manager) createOrigin(ctx context.Context, poolID, poolName string, number int32, target Target) (*lbaas.ResourceCreatedResponse, *http.Response, error) {
	createOriginRequest := lbaas.LoadBalancerPoolOriginCreate{
		Name:   getResourceName(poolName, "origin", number),
		Target: target.IP,
		PortNumber: lbaas.LoadBalancerPoolOriginPortNumber{
			Int32: &target.Port,
		},
		Active: true,
		PoolId: poolID,
	}
	return m.client.PoolsApi.CreateLoadBalancerPoolOrigin(ctx, poolID).LoadBalancerPoolOriginCreate(createOriginRequest).Execute()

}

func getResourceName(loadBalancerName, resourceType string, number int32) string {
	return fmt.Sprintf("%v-%v-%v", loadBalancerName, resourceType, number)
}
