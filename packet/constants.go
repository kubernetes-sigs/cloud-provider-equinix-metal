package packet

const (
	metalLBNamespace     = "metallb-system"
	metalLBConfigMapName = "config"
	configMapResource    = "configmaps"
	localASN             = 65000
	peerASN              = 65530
	hostnameKey          = "kubernetes.io/hostname"
	packetIdentifier     = "packet-ccm-auto"
	packetTag            = "usage=" + packetIdentifier
	ccmIPDescription     = "Packet Kubernetes CCM auto-generated for Load Balancer"
)
