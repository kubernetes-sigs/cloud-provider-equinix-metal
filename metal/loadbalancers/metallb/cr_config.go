package metallb

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func poolName(svcNamespace, svcName string) string {
	ns := regexp.MustCompile(`[^a-zA-Z0-9-]+`).ReplaceAllString(svcNamespace, "")
	svc := regexp.MustCompile(`[^a-zA-Z0-9-]+`).ReplaceAllString(svcName, "")
	return fmt.Sprintf("%s.%s", ns, svc)
}

func serviceLabelKey(svcName string) string {
	return svcLabelKeyPrefix + svcName
}

func serviceLabelValue(svcNamespace string) string {
	return svcLabelValuePrefix + svcNamespace
}

func sharedAnnotationKey(sharedKey string) string {
	return svcAnnotationSharedPrefix + sharedKey
}

func sharedServiceName(svcNamespace, svcName string) string {
	return fmt.Sprintf("%s.%s", svcNamespace, svcName)
}

func containsSharedService(poolAnnotationValue, svcNamespace, svcName string) bool {
	svcList := strings.Split(poolAnnotationValue, ",")
	return slices.Contains(svcList, sharedServiceName(svcNamespace, svcName))
}

func convertToIPAddr(addr AddressPool, namespace, svcNamespace, svcName string) metallbv1beta1.IPAddressPool {
	ip := metallbv1beta1.IPAddressPool{
		Spec: metallbv1beta1.IPAddressPoolSpec{
			Addresses:     addr.Addresses,
			AutoAssign:    addr.AutoAssign,
			AvoidBuggyIPs: addr.AvoidBuggyIPs,
		},
	}
	ip.SetLabels(map[string]string{
		cpemLabelKey:             cpemLabelValue,
		serviceLabelKey(svcName): serviceLabelValue(svcNamespace),
	})
	ip.SetName(addr.Name)
	ip.SetNamespace(namespace)
	return ip
}

func convertToBGPPeer(peer Peer, namespace, svc string) metallbv1beta1.BGPPeer {
	time, _ := time.ParseDuration(peer.HoldTime)
	bgpPeer := metallbv1beta1.BGPPeer{
		Spec: metallbv1beta1.BGPPeerSpec{
			MyASN:      peer.MyASN,
			ASN:        peer.ASN,
			Address:    peer.Addr,
			SrcAddress: peer.SrcAddr,
			Port:       peer.Port,
			HoldTime:   metav1.Duration{Duration: time},
			// KeepaliveTime: ,
			// RouterID: peer.RouterID,
			NodeSelectors: convertToNodeSelectors(peer.NodeSelectors),
			Password:      peer.Password,
			// BFDProfile:
			// EBGPMultiHop:
		},
	}
	bgpPeer.SetLabels(map[string]string{cpemLabelKey: cpemLabelValue})
	bgpPeer.SetName(peer.Name)
	bgpPeer.SetNamespace(namespace)
	return bgpPeer
}

func convertToNodeSelectors(legacy NodeSelectors) []metallbv1beta1.NodeSelector {
	nodeSelectors := make([]metallbv1beta1.NodeSelector, 0)
	for _, l := range legacy {
		nodeSelectors = append(nodeSelectors, convertToNodeSelector(l))
	}
	return nodeSelectors
}

func convertToNodeSelector(legacy NodeSelector) metallbv1beta1.NodeSelector {
	return metallbv1beta1.NodeSelector{
		MatchLabels:      legacy.MatchLabels,
		MatchExpressions: convertToMatchExpressions(legacy.MatchExpressions),
	}
}

func convertToMatchExpressions(legacy []SelectorRequirements) []metallbv1beta1.MatchExpression {
	matchExpressions := make([]metallbv1beta1.MatchExpression, 0)
	for _, l := range legacy {
		new := metallbv1beta1.MatchExpression{
			Key:      l.Key,
			Operator: l.Operator,
			Values:   l.Values,
		}
		matchExpressions = append(matchExpressions, new)
	}
	return matchExpressions
}

// peerSpecEqual return true if a peer is identical.
// Will only check for it in the current Peer p, and not the "other" peer in the parameter.
func peerSpecEqual(p, o metallbv1beta1.BGPPeerSpec) bool {
	// not matched if any field is mismatched except for NodeSelectors
	if p.MyASN != o.MyASN || p.ASN != o.ASN || p.Address != o.Address || p.Port != o.Port || p.HoldTime != o.HoldTime ||
		p.Password != o.Password || p.RouterID != o.RouterID {
		return false
	}
	return true
}

// AddService ensures that the provided service is in the list of linked services.
func peerAddService(p *metallbv1beta1.BGPPeer, svcNamespace, svcName string) bool {
	var (
		services = []Resource{
			{Namespace: svcNamespace, Name: svcName},
		}
		selectors []metallbv1beta1.NodeSelector
	)
	for _, ns := range p.Spec.NodeSelectors {
		var namespace, name string
		for k, v := range ns.MatchLabels {
			switch k {
			case serviceNameKey:
				name = v
			case serviceNamespaceKey:
				namespace = v
			}
		}
		// if this was not a service namespace/name selector, just add it
		if name == "" && namespace == "" {
			selectors = append(selectors, ns)
		}
		if name != "" && namespace != "" {
			// if it already had it, nothing to do, nothing change
			if svcNamespace == namespace && svcName == name {
				return false
			}
			services = append(services, Resource{Namespace: namespace, Name: name})
		}
	}
	// replace the NodeSelectors with everything except for the services
	p.Spec.NodeSelectors = selectors

	// now add the services
	sort.Sort(Resources(services))

	// if we did not find it, add it
	for _, svc := range services {
		p.Spec.NodeSelectors = append(p.Spec.NodeSelectors, metallbv1beta1.NodeSelector{
			MatchLabels: map[string]string{
				serviceNamespaceKey: svc.Namespace,
				serviceNameKey:      svc.Name,
			},
		})
	}
	return true
}

// RemoveService removes a given service from the peer. Returns whether or not it was
// changed, and how many services are left for this peer.
func peerRemoveService(p *metallbv1beta1.BGPPeer, svcNamespace, svcName string) (bool, int) {
	var (
		found     bool
		size      int
		services  = []Resource{}
		selectors []metallbv1beta1.NodeSelector
	)
	for _, ns := range p.Spec.NodeSelectors {
		var name, namespace string
		for k, v := range ns.MatchLabels {
			switch k {
			case serviceNameKey:
				name = v
			case serviceNamespaceKey:
				namespace = v
			}
		}
		switch {
		case name == "" && namespace == "":
			selectors = append(selectors, ns)
		case name == svcName && namespace == svcNamespace:
			found = true
		case name != "" && namespace != "" && (name != svcName || namespace != svcNamespace):
			services = append(services, Resource{Namespace: namespace, Name: name})
		}
	}
	// first put back all of the previous selectors except for the services
	p.Spec.NodeSelectors = selectors
	// then add all of the services
	sort.Sort(Resources(services))
	size = len(services)
	for _, svc := range services {
		p.Spec.NodeSelectors = append(p.Spec.NodeSelectors, metallbv1beta1.NodeSelector{
			MatchLabels: map[string]string{
				serviceNamespaceKey: svc.Namespace,
				serviceNameKey:      svc.Name,
			},
		})
	}
	return found, size
}
