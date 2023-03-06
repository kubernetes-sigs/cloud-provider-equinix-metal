package metal

import (
	"encoding/json"
	"fmt"
	"io"
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
	envVarEIPHealthCheckUseHostIP      = "METAL_EIP_HEALTH_CHECK_USE_HOST_IP"
	envVarUseCRDForMetalLB             = "METAL_USE_CRD_FOR_METALLB"
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
	AnnotationLocalASN           string  `json:"annotationLocalASN,omitempty"`
	AnnotationPeerASN            string  `json:"annotationPeerASN,omitempty"`
	AnnotationPeerIP             string  `json:"annotationPeerIP,omitempty"`
	AnnotationSrcIP              string  `json:"annotationSrcIP,omitempty"`
	AnnotationBGPPass            string  `json:"annotationBGPPass,omitempty"`
	AnnotationNetworkIPv4Private string  `json:"annotationNetworkIPv4Private,omitempty"`
	AnnotationEIPMetro           string  `json:"annotationEIPMetro,omitempty"`
	AnnotationEIPFacility        string  `json:"annotationEIPFacility,omitempty"`
	EIPTag                       string  `json:"eipTag,omitempty"`
	APIServerPort                int32   `json:"apiServerPort,omitempty"`
	BGPNodeSelector              string  `json:"bgpNodeSelector,omitempty"`
	EIPHealthCheckUseHostIP      bool    `json:"eipHealthCheckUseHostIP,omitempty"`
	UseCRDForMetalLB             bool    `json:"useCRDForMetalLB,omitempty"`
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
		ret = append(ret, fmt.Sprintf("load balancer config: '%s'", c.LoadBalancerSetting))
	}
	ret = append(ret, fmt.Sprintf("metro: '%s'", c.Metro))
	ret = append(ret, fmt.Sprintf("facility: '%s'", c.Facility))
	ret = append(ret, fmt.Sprintf("local ASN: '%d'", c.LocalASN))
	ret = append(ret, fmt.Sprintf("Elastic IP Tag: '%s'", c.EIPTag))
	ret = append(ret, fmt.Sprintf("API Server Port: '%d'", c.APIServerPort))
	ret = append(ret, fmt.Sprintf("BGP Node Selector: '%s'", c.BGPNodeSelector))

	return ret
}

func override(options ...string) string {
	for _, val := range options {
		if val != "" {
			return val
		}
	}

	return ""
}

// getMetalConfig returns a Config struct from a cloud config JSON file or environment variables
func getMetalConfig(providerConfig io.Reader) (Config, error) {
	// get our token and project
	var config, rawConfig Config

	// providerConfig may be nil if no --cloud-config is provided
	if providerConfig != nil {
		configBytes, err := io.ReadAll(providerConfig)
		if err != nil {
			return config, fmt.Errorf("failed to read configuration : %w", err)
		}
		err = json.Unmarshal(configBytes, &rawConfig)
		if err != nil {
			return config, fmt.Errorf("failed to process json of configuration file at path %s: %w", providerConfig, err)
		}
	}

	// read env vars; if not set, use rawConfig
	config.AuthToken = override(os.Getenv(apiKeyName), rawConfig.AuthToken)

	config.ProjectID = override(os.Getenv(projectIDName), rawConfig.ProjectID)

	config.LoadBalancerSetting = override(os.Getenv(loadBalancerSettingName), rawConfig.LoadBalancerSetting)

	config.Facility = override(os.Getenv(facilityName), rawConfig.Facility)

	config.Metro = override(os.Getenv(metroName), rawConfig.Metro)

	if config.AuthToken == "" {
		return config, fmt.Errorf("environment variable %q is required if not in config file", apiKeyName)
	}

	if config.ProjectID == "" {
		return config, fmt.Errorf("environment variable %q is required if not in config file", projectIDName)
	}

	// get the local ASN
	localASN := os.Getenv(envVarLocalASN)
	switch {
	case localASN != "":
		localASNNo, err := strconv.Atoi(localASN)
		if err != nil {
			return config, fmt.Errorf("env var %s must be a number, was %s: %w", envVarLocalASN, localASN, err)
		}
		config.LocalASN = localASNNo
	case rawConfig.LocalASN != 0:
		config.LocalASN = rawConfig.LocalASN
	default:
		config.LocalASN = DefaultLocalASN
	}

	config.BGPPass = override(os.Getenv(envVarBGPPass), rawConfig.BGPPass)

	// set the annotations
	config.AnnotationLocalASN = override(os.Getenv(envVarAnnotationLocalASN), rawConfig.AnnotationLocalASN, DefaultAnnotationNodeASN)

	config.AnnotationPeerASN = override(os.Getenv(envVarAnnotationPeerASN), rawConfig.AnnotationPeerASN, DefaultAnnotationPeerASN)

	config.AnnotationPeerIP = override(os.Getenv(envVarAnnotationPeerIP), rawConfig.AnnotationPeerIP, DefaultAnnotationPeerIP)

	config.AnnotationSrcIP = override(os.Getenv(envVarAnnotationSrcIP), rawConfig.AnnotationSrcIP, DefaultAnnotationSrcIP)

	config.AnnotationBGPPass = override(os.Getenv(envVarAnnotationBGPPass), rawConfig.AnnotationBGPPass, DefaultAnnotationBGPPass)

	config.AnnotationNetworkIPv4Private = override(os.Getenv(envVarAnnotationNetworkIPv4Private), rawConfig.AnnotationNetworkIPv4Private, DefaultAnnotationNetworkIPv4Private)

	config.AnnotationEIPMetro = override(os.Getenv(envVarAnnotationEIPMetro), rawConfig.AnnotationEIPMetro, DefaultAnnotationEIPMetro)

	config.AnnotationEIPFacility = override(os.Getenv(envVarAnnotationEIPFacility), rawConfig.AnnotationEIPFacility, DefaultAnnotationEIPFacility)

	config.EIPTag = override(os.Getenv(envVarEIPTag), rawConfig.EIPTag)

	apiServer := os.Getenv(envVarAPIServerPort)
	switch {
	case apiServer != "":
		apiServerNo, err := strconv.Atoi(apiServer)
		if err != nil {
			return config, fmt.Errorf("env var %s must be a number, was %s: %w", envVarAPIServerPort, apiServer, err)
		}
		config.APIServerPort = int32(apiServerNo)
	case rawConfig.APIServerPort != 0:
		config.APIServerPort = rawConfig.APIServerPort
	default:
		// if nothing else set it, we set it to 0, to indicate that it should use whatever the kube-apiserver port is
		config.APIServerPort = 0
	}

	config.BGPNodeSelector = override(os.Getenv(envVarBGPNodeSelector), rawConfig.BGPNodeSelector)

	if _, err := labels.Parse(config.BGPNodeSelector); err != nil {
		return config, fmt.Errorf("BGP Node Selector must be valid Kubernetes selector: %w", err)
	}

	config.EIPHealthCheckUseHostIP = rawConfig.EIPHealthCheckUseHostIP
	if v := os.Getenv(envVarEIPHealthCheckUseHostIP); v != "" {
		useHostIP, err := strconv.ParseBool(v)
		if err != nil {
			return config, fmt.Errorf("env var %s must be a boolean, was %s: %w", envVarEIPHealthCheckUseHostIP, v, err)
		}
		config.EIPHealthCheckUseHostIP = useHostIP
	}

	config.UseCRDForMetalLB = rawConfig.UseCRDForMetalLB
	if v := os.Getenv(envVarUseCRDForMetalLB); v != "" {
		useCRDForMetalLB, err := strconv.ParseBool(v)
		if err != nil {
			return config, fmt.Errorf("env var %s must be a boolean, was %s: %w", envVarUseCRDForMetalLB, v, err)
		}
		config.UseCRDForMetalLB = useCRDForMetalLB
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
