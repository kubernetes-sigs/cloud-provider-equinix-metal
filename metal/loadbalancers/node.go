package loadbalancers

type Node struct {
	Name     string
	SourceIP string
	LocalASN int
	PeerASN  int
	Peers    []string
}
