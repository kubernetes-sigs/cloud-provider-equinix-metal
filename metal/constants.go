package metal

const (
	configMapResource                   = "configmaps"
	hostnameKey                         = "kubernetes.io/hostname"
	emIdentifier                        = "cloud-provider-equinix-metal-auto"
	emTag                               = "usage=" + emIdentifier
	ccmIPDescription                    = "Equinix Metal Kubernetes CCM auto-generated for Load Balancer"
	DefaultAnnotationNodeASN            = "metal.equinix.com/bgp-peers-{{n}}-node-asn"
	DefaultAnnotationPeerASN            = "metal.equinix.com/bgp-peers-{{n}}-peer-asn"
	DefaultAnnotationPeerIP             = "metal.equinix.com/bgp-peers-{{n}}-peer-ip"
	DefaultAnnotationSrcIP              = "metal.equinix.com/bgp-peers-{{n}}-src-ip"
	DefaultAnnotationBGPPass            = "metal.equinix.com/bgp-peers-{{n}}-bgp-pass"
	DefaultAnnotationNetworkIPv4Private = "metal.equinix.com/network-4-private"
	DefaultAnnotationEIPMetro           = "metal.equinix.com/eip-metro"
	DefaultAnnotationEIPFacility        = "metal.equinix.com/eip-facility"
	DefaultLocalASN                     = 65000
	DefaultPeerASN                      = 65530
)
