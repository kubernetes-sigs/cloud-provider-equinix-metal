package metal

const (
	configMapResource                   = "configmaps"
	hostnameKey                         = "kubernetes.io/hostname"
	emIdentifier                        = "cloud-provider-equinix-metal-auto"
	emTag                               = "usage=" + emIdentifier
	ccmIPDescription                    = "Equinix Metal Kubernetes CCM auto-generated for Load Balancer"
	DefaultAnnotationNodeASN            = "metal.equinix.com/node-asn"
	DefaultAnnotationPeerASNs           = "metal.equinix.com/peer-asn"
	DefaultAnnotationPeerIPs            = "metal.equinix.com/peer-ip"
	DefaultAnnotationSrcIP              = "metal.equinix.com/src-ip"
	DefaultAnnotationBGPPass            = "metal.equinix.com/bgp-pass"
	DefaultAnnotationNetworkIPv4Private = "metal.equinix.com/network/4/private"
	DefaultLocalASN                     = 65000
	DefaultPeerASN                      = 65530
)
