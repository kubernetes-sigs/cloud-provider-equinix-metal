package metal

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

const (
	apiKeyName                         = "METAL_API_KEY"
	projectIDName                      = "METAL_PROJECT_ID"
	facilityName                       = "METAL_FACILITY_NAME"
	metroName                          = "METAL_METRO_NAME"
	loadBalancerSettingName            = "METAL_LOAD_BALANCER"
	envVarLocalASN                     = "METAL_LOCAL_ASN"
	envVarBGPPass                      = "METAL_BGP_PASS"
	envVarAnnotationLocalASN           = "METAL_ANNOTATION_LOCAL_ASN"
	envVarAnnotationPeerASN            = "METAL_ANNOTATION_PEER_ASN"
	envVarAnnotationPeerIP             = "METAL_ANNOTATION_PEER_IP"
	envVarAnnotationSrcIP              = "METAL_ANNOTATION_SRC_IP"
	envVarAnnotationBGPPass            = "METAL_ANNOTATION_BGP_PASS"
	envVarAnnotationNetworkIPv4Private = "METAL_ANNOTATION_NETWORK_IPV4_PRIVATE"
	envVarAnnotationEIPMetro           = "METAL_ANNOTATION_EIP_METRO"
	envVarAnnotationEIPFacility        = "METAL_ANNOTATION_EIP_FACILITY"
	envVarEIPTag                       = "METAL_EIP_TAG"
	envVarAPIServerPort                = "METAL_API_SERVER_PORT"
	envVarBGPNodeSelector              = "METAL_BGP_NODE_SELECTOR"
	defaultLoadBalancerConfigMap       = "metallb-system:config"
)

// Config configuration for a provider, includes authentication token, project ID ID, and optional override URL to talk to a different Equinix Metal API endpoint
type Config struct {
	AuthToken                    string  `json:"apiKey"`
	ProjectID                    string  `json:"projectId"`
	BaseURL                      *string `json:"base-url,omitempty"`
	LoadBalancerSetting          string  `json:"loadbalancer"`
	Metro                        string  `json:"metro,omitempty"`
	Facility                     string  `json:"facility,omitempty"`
	LocalASN                     int     `json:"localASN,omitempty"`
	BGPPass                      string  `json:"bgpPass,omitempty"`
	AnnotationLocalASN           string  `json:"annotationLocalASN,omitEmpty"`
	AnnotationPeerASN            string  `json:"annotationPeerASN,omitEmpty"`
	AnnotationPeerIP             string  `json:"annotationPeerIP,omitEmpty"`
	AnnotationSrcIP              string  `json:"annotationSrcIP,omitEmpty"`
	AnnotationBGPPass            string  `json:"annotationBGPPass,omitEmpty"`
	AnnotationNetworkIPv4Private string  `json:"annotationNetworkIPv4Private,omitEmpty"`
	AnnotationEIPMetro           string  `json:"annotationEIPMetro,omitEmpty"`
	AnnotationEIPFacility        string  `json:"annotationEIPFacility,omitEmpty"`
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
		ret = append(ret, fmt.Sprintf("load balancer config: ''%s", c.LoadBalancerSetting))
	}
	ret = append(ret, fmt.Sprintf("metro: '%s'", c.Metro))
	ret = append(ret, fmt.Sprintf("facility: '%s'", c.Facility))
	ret = append(ret, fmt.Sprintf("local ASN: '%d'", c.LocalASN))
	ret = append(ret, fmt.Sprintf("Elastic IP Tag: '%s'", c.EIPTag))
	ret = append(ret, fmt.Sprintf("API Server Port: '%d'", c.APIServerPort))
	ret = append(ret, fmt.Sprintf("BGP Node Selector: '%s'", c.BGPNodeSelector))

	return ret
}

func getMetalConfig(providerConfig io.Reader) (Config, error) {
	// get our token and project
	var config, rawConfig Config
	configBytes, err := ioutil.ReadAll(providerConfig)
	if err != nil {
		return config, fmt.Errorf("failed to read configuration : %v", err)
	}
	err = json.Unmarshal(configBytes, &rawConfig)
	if err != nil {
		return config, fmt.Errorf("failed to process json of configuration file at path %s: %v", providerConfig, err)
	}

	// read env vars; if not set, use rawConfig
	apiToken := os.Getenv(apiKeyName)
	if apiToken == "" {
		apiToken = rawConfig.AuthToken
	}
	config.AuthToken = apiToken

	projectID := os.Getenv(projectIDName)
	if projectID == "" {
		projectID = rawConfig.ProjectID
	}
	config.ProjectID = projectID

	loadBalancerSetting := os.Getenv(loadBalancerSettingName)
	config.LoadBalancerSetting = rawConfig.LoadBalancerSetting
	// rule for processing: any setting in env var overrides setting from file
	if loadBalancerSetting != "" {
		config.LoadBalancerSetting = loadBalancerSetting
	}
	// and set for default
	if config.LoadBalancerSetting == "" {
		config.LoadBalancerSetting = defaultLoadBalancerConfigMap
	}

	facility := os.Getenv(facilityName)
	if facility == "" {
		facility = rawConfig.Facility
	}

	metro := os.Getenv(metroName)
	if metro == "" {
		metro = rawConfig.Metro
	}

	if apiToken == "" {
		return config, fmt.Errorf("environment variable %q is required", apiKeyName)
	}

	if projectID == "" {
		return config, fmt.Errorf("environment variable %q is required", projectIDName)
	}

	config.Facility = facility
	config.Metro = metro

	// get the local ASN
	localASN := os.Getenv(envVarLocalASN)
	switch {
	case localASN != "":
		localASNNo, err := strconv.Atoi(localASN)
		if err != nil {
			return config, fmt.Errorf("env var %s must be a number, was %s: %v", envVarLocalASN, localASN, err)
		}
		config.LocalASN = localASNNo
	case rawConfig.LocalASN != 0:
		config.LocalASN = rawConfig.LocalASN
	default:
		config.LocalASN = DefaultLocalASN
	}

	bgpPass := os.Getenv(envVarBGPPass)
	if bgpPass != "" {
		config.BGPPass = bgpPass
	}

	// set the annotations
	config.AnnotationLocalASN = DefaultAnnotationNodeASN
	annotationLocalASN := os.Getenv(envVarAnnotationLocalASN)
	if annotationLocalASN != "" {
		config.AnnotationLocalASN = annotationLocalASN
	}
	config.AnnotationPeerASN = DefaultAnnotationPeerASN
	annotationPeerASN := os.Getenv(envVarAnnotationPeerASN)
	if annotationPeerASN != "" {
		config.AnnotationPeerASN = annotationPeerASN
	}
	config.AnnotationPeerIP = DefaultAnnotationPeerIP
	annotationPeerIP := os.Getenv(envVarAnnotationPeerIP)
	if annotationPeerIP != "" {
		config.AnnotationPeerIP = annotationPeerIP
	}
	config.AnnotationSrcIP = DefaultAnnotationSrcIP
	annotationSrcIP := os.Getenv(envVarAnnotationSrcIP)
	if annotationSrcIP != "" {
		config.AnnotationSrcIP = annotationSrcIP
	}

	config.AnnotationBGPPass = DefaultAnnotationBGPPass
	annotationBGPPass := os.Getenv(envVarAnnotationBGPPass)
	if annotationBGPPass != "" {
		config.AnnotationBGPPass = annotationBGPPass
	}

	config.AnnotationNetworkIPv4Private = DefaultAnnotationNetworkIPv4Private
	annotationNetworkIPv4Private := os.Getenv(envVarAnnotationNetworkIPv4Private)
	if annotationNetworkIPv4Private != "" {
		config.AnnotationNetworkIPv4Private = annotationNetworkIPv4Private
	}

	config.AnnotationEIPMetro = DefaultAnnotationEIPMetro
	annotationEIPMetro := os.Getenv(envVarAnnotationEIPMetro)
	if annotationEIPMetro != "" {
		config.AnnotationEIPMetro = annotationEIPMetro
	}

	config.AnnotationEIPFacility = DefaultAnnotationEIPFacility
	annotationEIPFacility := os.Getenv(envVarAnnotationEIPFacility)
	if annotationEIPFacility != "" {
		config.AnnotationEIPFacility = annotationEIPFacility
	}

	if rawConfig.EIPTag != "" {
		config.EIPTag = rawConfig.EIPTag
	}
	eipTag := os.Getenv(envVarEIPTag)
	if eipTag != "" {
		config.EIPTag = eipTag
	}

	apiServer := os.Getenv(envVarAPIServerPort)
	switch {
	case apiServer != "":
		apiServerNo, err := strconv.Atoi(apiServer)
		if err != nil {
			return config, fmt.Errorf("env var %s must be a number, was %s: %v", envVarAPIServerPort, apiServer, err)
		}
		config.APIServerPort = int32(apiServerNo)
	case rawConfig.APIServerPort != 0:
		config.APIServerPort = rawConfig.APIServerPort
	default:
		// if nothing else set it, we set it to 0, to indicate that it should use whatever the kube-apiserver port is
		config.APIServerPort = 0
	}

	config.BGPNodeSelector = rawConfig.BGPNodeSelector
	if v := os.Getenv(envVarBGPNodeSelector); v != "" {
		config.BGPNodeSelector = v
	}

	if _, err := labels.Parse(config.BGPNodeSelector); err != nil {
		return config, fmt.Errorf("BGP Node Selector must be valid Kubernetes selector: %w", err)
	}

	return config, nil
}

// printMetalConfig report the config to startup logs
func printMetalConfig(config Config) {
	lines := config.Strings()
	for _, l := range lines {
		klog.Infof(l)
	}
}
