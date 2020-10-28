package packet

const (
	configMapResource         = "configmaps"
	hostnameKey               = "kubernetes.io/hostname"
	packetIdentifier          = "packet-ccm-auto"
	packetTag                 = "usage=" + packetIdentifier
	ccmIPDescription          = "Packet Kubernetes CCM auto-generated for Load Balancer"
	DefaultAnnotationNodeASN  = "packet.com/node.asn"
	DefaultAnnotationPeerASNs = "packet.com/peer.asns"
	DefaultAnnotationPeerIPs  = "packet.com/peer.ips"
	DefaultLocalASN           = 65000
	DefaultPeerASN            = 65530
	DefaultAPIServerPort      = 6443
)
