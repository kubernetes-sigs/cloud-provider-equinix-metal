package metal

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/equinix/cloud-provider-equinix-metal/metal/loadbalancers/emlb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	v1applyconfig "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
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

	if _, err := sharedInformer.Core().V1().Endpoints().Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				e, _ := obj.(*v1.Endpoints)
				if e.Namespace != metav1.NamespaceDefault && e.Name != "kubernetes" {
					return false
				}

				return true
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					k8sEndpoints, _ := obj.(*v1.Endpoints)
					klog.Infof("handling add, endpoints: %s/%s", k8sEndpoints.Namespace, k8sEndpoints.Name)

					if err := m.syncEndpoints(ctx, k8sEndpoints); err != nil {
						klog.Errorf("failed to sync endpoints from default/kubernetes to %s/%s: %v", externalServiceNamespace, externalServiceName, err)
						return
					}
				},
				UpdateFunc: func(_, obj interface{}) {
					k8sEndpoints, _ := obj.(*v1.Endpoints)
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

func (m *controlPlaneLoadBalancerManager) syncEndpoints(ctx context.Context, k8sEndpoints *v1.Endpoints) error {
	m.endpointsMutex.Lock()
	defer m.endpointsMutex.Unlock()

	applyConfig := v1applyconfig.Endpoints(externalServiceName, externalServiceNamespace)
	for _, subset := range k8sEndpoints.Subsets {
		applyConfig = applyConfig.WithSubsets(EndpointSubsetApplyConfig(subset))
	}

	if _, err := m.k8sclient.CoreV1().Endpoints(externalServiceNamespace).Apply(
		ctx,
		applyConfig,
		metav1.ApplyOptions{FieldManager: emIdentifier},
	); err != nil {
		return fmt.Errorf("failed to apply endpoint %s/%s: %w", externalServiceNamespace, externalServiceName, err)
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
