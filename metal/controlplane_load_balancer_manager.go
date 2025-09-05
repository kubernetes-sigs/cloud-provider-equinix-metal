package metal

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	v1applyconfig "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cloud-provider-equinix-metal/metal/loadbalancers/emlb"
	discoveryv1 "k8s.io/api/discovery/v1"
	discoveryv1applyconfig "k8s.io/client-go/applyconfigurations/discovery/v1"
)

type controlPlaneLoadBalancerManager struct {
	apiServerPort         int32 // node on which the external load balancer should listen
	nodeAPIServerPort     int32 // port on which the api server is listening on the control plane nodes
	projectID             string
	loadBalancerID        string
	httpClient            *http.Client
	k8sclient             kubernetes.Interface
	serviceMutex          sync.Mutex
	endpointsMutex        sync.Mutex
	controlPlaneSelectors []labels.Selector
	useHostIP             bool
}

func newControlPlaneLoadBalancerManager(k8sclient kubernetes.Interface, stop <-chan struct{}, projectID string, loadBalancerID string, apiServerPort int32, useHostIP bool) (*controlPlaneLoadBalancerManager, error) {
	klog.V(2).Info("newControlPlaneLoadBalancerManager()")

	if loadBalancerID == "" {
		klog.Info("Load balancer ID is not configured, skipping control plane load balancer management")
		return nil, nil
	}

	m := &controlPlaneLoadBalancerManager{
		httpClient: &http.Client{
			Timeout: time.Second * 5,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		apiServerPort:  apiServerPort,
		projectID:      projectID,
		loadBalancerID: loadBalancerID,
		k8sclient:      k8sclient,
		useHostIP:      useHostIP,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stop
		cancel()
	}()

	for _, label := range controlPlaneLabels {
		req, err := labels.NewRequirement(label, selection.Exists, nil)
		if err != nil {
			return m, err
		}

		m.controlPlaneSelectors = append(m.controlPlaneSelectors, labels.NewSelector().Add(*req))
	}

	sharedInformer := informers.NewSharedInformerFactory(k8sclient, checkLoopTimerSeconds*time.Second)

	if _, err := sharedInformer.Discovery().V1().EndpointSlices().Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				e, _ := obj.(*discoveryv1.EndpointSlice)
				if e.Namespace != metav1.NamespaceDefault && e.Name != "kubernetes" {
					return false
				}

				return true
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					k8sEndpoints, _ := obj.(*discoveryv1.EndpointSlice)
					klog.Infof("handling add, endpoints: %s/%s", k8sEndpoints.Namespace, k8sEndpoints.Name)

					if err := m.syncEndpoints(ctx, k8sEndpoints); err != nil {
						klog.Errorf("failed to sync endpoints from default/kubernetes to %s/%s: %v", externalServiceNamespace, externalServiceName, err)
						return
					}
				},
				UpdateFunc: func(_, obj interface{}) {
					k8sEndpoints, _ := obj.(*discoveryv1.EndpointSlice)
					klog.Infof("handling update, endpoints: %s/%s", k8sEndpoints.Namespace, k8sEndpoints.Name)

					if err := m.syncEndpoints(ctx, k8sEndpoints); err != nil {
						klog.Errorf("failed to sync endpoints from default/kubernetes to %s/%s: %v", externalServiceNamespace, externalServiceName, err)
						return
					}
				},
			},
		},
	); err != nil {
		return m, err
	}

	if _, err := sharedInformer.Core().V1().Services().Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				s, _ := obj.(*v1.Service)
				// Filter only service default/kubernetes
				if s.Namespace == metav1.NamespaceDefault && s.Name == "kubernetes" {
					return true
				}
				//else
				return false
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					k8sService, _ := obj.(*v1.Service)
					klog.Infof("handling add, service: %s/%s", k8sService.Namespace, k8sService.Name)

					if err := m.syncService(ctx, k8sService); err != nil {
						klog.Errorf("failed to sync service from default/kubernetes to %s/%s: %v", externalServiceNamespace, externalServiceName, err)
						return
					}
				},
				UpdateFunc: func(_, obj interface{}) {
					k8sService, _ := obj.(*v1.Service)
					klog.Infof("handling update, service: %s/%s", k8sService.Namespace, k8sService.Name)

					if err := m.syncService(ctx, k8sService); err != nil {
						klog.Errorf("failed to sync service from default/kubernetes to %s/%s: %v", externalServiceNamespace, externalServiceName, err)
						return
					}
				},
			},
		},
	); err != nil {
		return m, err
	}

	sharedInformer.Start(stop)
	sharedInformer.WaitForCacheSync(stop)

	return m, nil
}

func (m *controlPlaneLoadBalancerManager) syncEndpoints(ctx context.Context, k8sEndpoints *discoveryv1.EndpointSlice) error {
	m.endpointsMutex.Lock()
	defer m.endpointsMutex.Unlock()

	applyConfig := discoveryv1applyconfig.EndpointSlice(externalServiceName, externalServiceNamespace)
	applyConfig = applyConfig.WithAddressType(k8sEndpoints.AddressType).WithLabels(map[string]string{
		discoveryv1.LabelServiceName: externalServiceName,
	})

	for _, port := range k8sEndpoints.Ports {
        portConfig := discoveryv1applyconfig.EndpointPort()
        
        if port.Port != nil {
            portConfig = portConfig.WithPort(*port.Port)
        }
        if port.Protocol != nil {
            portConfig = portConfig.WithProtocol(*port.Protocol)
        }
        if port.Name != nil {
            portConfig = portConfig.WithName(*port.Name)
        }
        if port.AppProtocol != nil {
            portConfig = portConfig.WithAppProtocol(*port.AppProtocol)
        }
        
        applyConfig = applyConfig.WithPorts(portConfig)
    }
    
    for _, endpoint := range k8sEndpoints.Endpoints {
        if len(endpoint.Addresses) == 0 {
            continue
        }
        
        endpointConfig := discoveryv1applyconfig.Endpoint()
        
        for _, addr := range endpoint.Addresses {
            endpointConfig = endpointConfig.WithAddresses(addr)
        }
        
        if endpoint.Conditions.Ready != nil {
            conditionsConfig := discoveryv1applyconfig.EndpointConditions().WithReady(*endpoint.Conditions.Ready)
            
            if endpoint.Conditions.Serving != nil {
                conditionsConfig = conditionsConfig.WithServing(*endpoint.Conditions.Serving)
            }
            if endpoint.Conditions.Terminating != nil {
                conditionsConfig = conditionsConfig.WithTerminating(*endpoint.Conditions.Terminating)
            }
            
            endpointConfig = endpointConfig.WithConditions(conditionsConfig)
        }
        
        applyConfig = applyConfig.WithEndpoints(endpointConfig)
    }

	if _, err := m.k8sclient.DiscoveryV1().EndpointSlices(externalServiceNamespace).Apply(
		ctx,
		applyConfig,
		metav1.ApplyOptions{FieldManager: emIdentifier},
	); err != nil {
		return fmt.Errorf("failed to apply endpointslice %s/%s: %w", externalServiceNamespace, externalServiceName, err)
	}

	return nil
}

func (m *controlPlaneLoadBalancerManager) syncService(ctx context.Context, k8sService *v1.Service) error {
	m.serviceMutex.Lock()
	defer m.serviceMutex.Unlock()

	existingService := k8sService.DeepCopy()

	// get the target port
	existingPorts := existingService.Spec.Ports
	if len(existingPorts) < 1 {
		return errors.New("default/kubernetes service does not have any ports defined")
	}

	// track which port the kube-apiserver actually is listening on
	m.nodeAPIServerPort = existingPorts[0].TargetPort.IntVal
	// if a specific port was requested, use that instead of the one from the original service
	if m.apiServerPort != 0 {
		existingPorts[0].Port = m.apiServerPort
	} else {
		existingPorts[0].Port = m.nodeAPIServerPort
	}

	annotations := map[string]string{}
	annotations[emlb.LoadBalancerIDAnnotation] = m.loadBalancerID

	specApplyConfig := v1applyconfig.ServiceSpec().WithType(v1.ServiceTypeLoadBalancer)

	for _, port := range existingPorts {
		specApplyConfig = specApplyConfig.WithPorts(ServicePortApplyConfig(port))
	}

	applyConfig := v1applyconfig.Service(externalServiceName, externalServiceNamespace).
		WithAnnotations(annotations).
		WithSpec(specApplyConfig)

	if _, err := m.k8sclient.CoreV1().Services(externalServiceNamespace).Apply(
		ctx,
		applyConfig,
		metav1.ApplyOptions{FieldManager: emIdentifier},
	); err != nil {
		return fmt.Errorf("failed to apply service %s/%s: %w", externalServiceNamespace, externalServiceName, err)
	}

	return nil
}
