package packet

import "fmt"

// Config configuration for a provider, includes authentication token, project ID ID, and optional override URL to talk to a different packet API endpoint
type Config struct {
	AuthToken             string  `json:"apiKey"`
	ProjectID             string  `json:"projectId"`
	BaseURL               *string `json:"base-url,omitempty"`
	LoadBalancerConfigMap string  `json:"loadbalancer-configmap"`
	Facility              string  `json:"facility,omitempty"`
	PeerASN               int     `json:"peerASN,omitempty"`
	LocalASN              int     `json:"localASN,omitempty"`
	AnnotationLocalASN    string  `json:"annotationLocalASN,omitEmpty"`
	AnnotationPeerASNs    string  `json:"annotationPeerASNs,omitEmpty"`
	AnnotationPeerIPs     string  `json:"annotationPeerIPs,omitEmpty"`
	EIPTag                string  `json:"eipTag,omitEmpty"`
	APIServerPort         int     `json:"apiServerPort,omitEmpty"`
	BGPNodeSelector       string  `json:"bgpNodeSelector,omitEmpty"`
}

// String converts the Config structure to a string, while masking hidden fields.
// Is not 100% a String() conversion, as it adds some intelligence to the output,
// and masks sensitive data
func (c Config) Strings() []string {
	ret := []string{}
	if c.AuthToken == "" {
		ret = append(ret, "authToken: '<masked>'")
	} else {
		ret = append(ret, "authToken: ''")
	}
	ret = append(ret, fmt.Sprintf("projectID: '%s'", c.ProjectID))
	if c.LoadBalancerConfigMap == "" {
		ret = append(ret, "loadbalancer config: disabled")
	} else {
		ret = append(ret, "load balancer config: ''%s", c.LoadBalancerConfigMap)
	}
	ret = append(ret, fmt.Sprintf("facility: '%s'", c.Facility))
	ret = append(ret, fmt.Sprintf("peer ASN: '%d'", c.PeerASN))
	ret = append(ret, fmt.Sprintf("local ASN: '%d'", c.LocalASN))
	ret = append(ret, fmt.Sprintf("Elastic IP Tag: '%s'", c.EIPTag))
	ret = append(ret, fmt.Sprintf("API Server Port: '%d'", c.APIServerPort))
	ret = append(ret, fmt.Sprintf("BGP Node Selector: '%s'", c.BGPNodeSelector))

	return ret
}
