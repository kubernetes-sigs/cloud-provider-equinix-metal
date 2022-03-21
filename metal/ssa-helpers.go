package metal

import (
	v1 "k8s.io/api/core/v1"
	v1applyconfig "k8s.io/client-go/applyconfigurations/core/v1"
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

func EndpointSubsetApplyConfig(subset v1.EndpointSubset) *v1applyconfig.EndpointSubsetApplyConfiguration {
	applyConfig := v1applyconfig.EndpointSubset()

	for _, addr := range subset.Addresses {
		applyConfig = applyConfig.WithAddresses(EndpointAddressApplyConfig(addr))
	}

	for _, addr := range subset.NotReadyAddresses {
		applyConfig = applyConfig.WithNotReadyAddresses(EndpointAddressApplyConfig(addr))
	}

	for _, port := range subset.Ports {
		applyConfig = applyConfig.WithPorts(EndpointPortApplyConfig(port))
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
