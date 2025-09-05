package metal

import (
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	v1applyconfig "k8s.io/client-go/applyconfigurations/core/v1"
	discoveryv1applyconfig "k8s.io/client-go/applyconfigurations/discovery/v1"
)

func ObjectReferenceApplyConfiguration(ref *v1.ObjectReference) *v1applyconfig.ObjectReferenceApplyConfiguration {
	if ref == nil {
		return nil
	}

	applyConfig := v1applyconfig.ObjectReference().WithAPIVersion(ref.APIVersion).
		WithFieldPath(ref.FieldPath).WithKind(ref.Kind).WithName(ref.Name).
		WithNamespace(ref.Namespace).WithResourceVersion(ref.ResourceVersion).
		WithUID(ref.UID)

	return applyConfig
}

func EndpointAddressApplyConfig(addr v1.EndpointAddress) *v1applyconfig.EndpointAddressApplyConfiguration {
	applyConfig := v1applyconfig.EndpointAddress().WithHostname(addr.Hostname).WithIP(addr.IP).
		WithTargetRef(ObjectReferenceApplyConfiguration(addr.TargetRef))

	if addr.NodeName != nil {
		applyConfig = applyConfig.WithNodeName(*addr.NodeName)
	}

	return applyConfig
}

func EndpointPortApplyConfig(port v1.EndpointPort) *v1applyconfig.EndpointPortApplyConfiguration {
	applyConfig := v1applyconfig.EndpointPort().WithName(port.Name).WithPort(port.Port).WithProtocol(port.Protocol)

	if port.AppProtocol != nil {
		applyConfig.WithAppProtocol(*port.AppProtocol)
	}

	return applyConfig
}

func EndpointSubsetApplyConfig(subset discoveryv1.EndpointSlice) *discoveryv1applyconfig.EndpointSliceApplyConfiguration {
	applyConfig := discoveryv1applyconfig.EndpointSlice(subset.Name, subset.Namespace)

	applyConfig = applyConfig.WithAddressType(subset.AddressType)

	for _, port := range subset.Ports {
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

	for _, endpoint := range subset.Endpoints {
		if len(endpoint.Addresses) == 0 {
			continue
		}

		endpointConfig := discoveryv1applyconfig.Endpoint()

		for _, addr := range endpoint.Addresses {
			endpointConfig = endpointConfig.WithAddresses(addr)
		}

		conditionsConfig := discoveryv1applyconfig.EndpointConditions()

		if endpoint.Conditions.Ready != nil {
			conditionsConfig = conditionsConfig.WithReady(*endpoint.Conditions.Ready)
		}
		if endpoint.Conditions.Serving != nil {
			conditionsConfig = conditionsConfig.WithServing(*endpoint.Conditions.Serving)
		}
		if endpoint.Conditions.Terminating != nil {
			conditionsConfig = conditionsConfig.WithTerminating(*endpoint.Conditions.Terminating)
		}

		endpointConfig = endpointConfig.WithConditions(conditionsConfig)

		applyConfig = applyConfig.WithEndpoints(endpointConfig)
	}

	return applyConfig
}

func ServicePortApplyConfig(port v1.ServicePort) *v1applyconfig.ServicePortApplyConfiguration {
	applyConfig := v1applyconfig.ServicePort().WithName(port.Name).WithNodePort(port.NodePort).
		WithPort(port.Port).WithProtocol(port.Protocol).WithTargetPort(port.TargetPort)

	if port.AppProtocol != nil {
		applyConfig = applyConfig.WithAppProtocol(*port.AppProtocol)
	}

	return applyConfig
}

func ServiceSpecApplyConfig(eip string, spec v1.ServiceSpec) *v1applyconfig.ServiceSpecApplyConfiguration {
	applyConfig := v1applyconfig.ServiceSpec().WithType(v1.ServiceTypeLoadBalancer).WithLoadBalancerIP(eip)

	for _, port := range spec.Ports {
		applyConfig = applyConfig.WithPorts(ServicePortApplyConfig(port))
	}

	return applyConfig
}
