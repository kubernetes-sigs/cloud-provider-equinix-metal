package metal

import "fmt"

// Config configuration for a provider, includes authentication token, project ID ID, and optional override URL to talk to a different Equinix Metal API endpoint
type Config struct {
	AuthToken                    string  `json:"apiKey"`
	ProjectID                    string  `json:"projectId"`
	BaseURL                      *string `json:"base-url,omitempty"`
	LoadBalancerSetting          string  `json:"loadbalancer"`
	Facility                     string  `json:"facility,omitempty"`
	LocalASN                     int     `json:"localASN,omitempty"`
	BGPPass                      string  `json:"bgpPass,omitempty"`
	AnnotationLocalASN           string  `json:"annotationLocalASN,omitEmpty"`
	AnnotationPeerASNs           string  `json:"annotationPeerASNs,omitEmpty"`
	AnnotationPeerIPs            string  `json:"annotationPeerIPs,omitEmpty"`
	AnnotationSrcIP              string  `json:"annotationSrcIP,omitEmpty"`
	AnnotationBGPPass            string  `json:"annotationBGPPass,omitEmpty"`
	AnnotationNetworkIPv4Private string  `json:"annotationNetworkIPv4Private,omitEmpty"`
	EIPTag                       string  `json:"eipTag,omitEmpty"`
	APIServerPort                int32   `json:"apiServerPort,omitEmpty"`
	BGPNodeSelector              string  `json:"bgpNodeSelector,omitEmpty"`
}

// String converts the Config structure to a string, while masking hidden fields.
// Is not 100% a String() conversion, as it adds some intelligence to the output,
// and masks sensitive data
func (c Config) Strings() []string {
	ret := []string{}
	if c.AuthToken != "" {
		ret = append(ret, "authToken: '<masked>'")
	} else {
		ret = append(ret, "authToken: ''")
	}
	ret = append(ret, fmt.Sprintf("projectID: '%s'", c.ProjectID))
	if c.LoadBalancerSetting == "" {
		ret = append(ret, "loadbalancer config: disabled")
	} else {
		ret = append(ret, "load balancer config: ''%s", c.LoadBalancerSetting)
	}
	ret = append(ret, fmt.Sprintf("facility: '%s'", c.Facility))
	ret = append(ret, fmt.Sprintf("local ASN: '%d'", c.LocalASN))
	ret = append(ret, fmt.Sprintf("Elastic IP Tag: '%s'", c.EIPTag))
	ret = append(ret, fmt.Sprintf("API Server Port: '%d'", c.APIServerPort))
	ret = append(ret, fmt.Sprintf("BGP Node Selector: '%s'", c.BGPNodeSelector))

	return ret
}
