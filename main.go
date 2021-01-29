package main

import (
	"encoding/json"
	goflag "flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"

	"github.com/equinix/cloud-provider-equinix-metal/metal"
	"github.com/spf13/pflag"
)

const (
	apiKeyName                   = "METAL_API_KEY"
	projectIDName                = "METAL_PROJECT_ID"
	facilityName                 = "METAL_FACILITY_NAME"
	loadBalancerSettingName      = "METAL_LB"
	envVarLocalASN               = "METAL_LOCAL_ASN"
	envVarPeerASN                = "METAL_PEER_ASN"
	envVarAnnotationLocalASN     = "METAL_ANNOTATION_LOCAL_ASN"
	envVarAnnotationPeerASNs     = "METAL_ANNOTATION_PEER_ASNS"
	envVarAnnotationPeerIPs      = "METAL_ANNOTATION_PEER_IPS"
	envVarAnnotationSrcIP        = "METAL_ANNOTATION_SRC_IP"
	envVarEIPTag                 = "METAL_EIP_TAG"
	envVarAPIServerPort          = "METAL_API_SERVER_PORT"
	envVarBGPNodeSelector        = "METAL_BGP_NODE_SELECTOR"
	defaultLoadBalancerConfigMap = "metallb-system:config"
)

var (
	providerConfig string
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	command := app.NewCloudControllerManagerCommand()

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	// add our config
	command.PersistentFlags().StringVar(&providerConfig, "provider-config", "", "path to provider config file")

	logs.InitLogs()
	defer logs.FlushLogs()

	// parse our flags so we get the providerConfig
	command.ParseFlags(os.Args[1:])

	// register the provider
	config, err := getMetalConfig(providerConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider config error: %v\n", err)
		os.Exit(1)
	}
	// report the config
	printMetalConfig(config)

	// register the provider
	if err := metal.InitializeProvider(config); err != nil {
		fmt.Fprintf(os.Stderr, "provider initialization error: %v\n", err)
		os.Exit(1)
	}

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func getMetalConfig(providerConfig string) (metal.Config, error) {
	// get our token and project
	var config, rawConfig metal.Config
	if providerConfig != "" {
		configBytes, err := ioutil.ReadFile(providerConfig)
		if err != nil {
			return config, fmt.Errorf("failed to get read configuration file at path %s: %v", providerConfig, err)
		}
		err = json.Unmarshal(configBytes, &rawConfig)
		if err != nil {
			return config, fmt.Errorf("failed to process json of configuration file at path %s: %v", providerConfig, err)
		}
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

	if apiToken == "" {
		return config, fmt.Errorf("environment variable %q is required", apiKeyName)
	}

	if projectID == "" {
		return config, fmt.Errorf("environment variable %q is required", projectIDName)
	}

	// if facility was not defined, retrieve it from our metadata
	if facility == "" {
		metadata, err := metal.GetAndParseMetadata("")
		if err != nil {
			return config, fmt.Errorf("facility not set in environment variable %q or config file, and error reading metadata: %v", facilityName, err)
		}
		facility = metadata.Facility
	}
	config.Facility = facility

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
		config.LocalASN = metal.DefaultLocalASN
	}

	// get the peer ASN
	peerASN := os.Getenv(envVarPeerASN)
	switch {
	case peerASN != "":
		peerASNNo, err := strconv.Atoi(peerASN)
		if err != nil {
			return config, fmt.Errorf("env var %s must be a number, was %s: %v", envVarPeerASN, peerASN, err)
		}
		config.PeerASN = peerASNNo
	case rawConfig.PeerASN != 0:
		config.PeerASN = rawConfig.PeerASN
	default:
		config.PeerASN = metal.DefaultPeerASN
	}

	// set the annotations
	config.AnnotationLocalASN = metal.DefaultAnnotationNodeASN
	annotationLocalASN := os.Getenv(envVarAnnotationLocalASN)
	if annotationLocalASN != "" {
		config.AnnotationLocalASN = annotationLocalASN
	}
	config.AnnotationPeerASNs = metal.DefaultAnnotationPeerASNs
	annotationPeerASNs := os.Getenv(envVarAnnotationPeerASNs)
	if annotationPeerASNs != "" {
		config.AnnotationPeerASNs = annotationPeerASNs
	}
	config.AnnotationPeerIPs = metal.DefaultAnnotationPeerIPs
	annotationPeerIPs := os.Getenv(envVarAnnotationPeerIPs)
	if annotationPeerIPs != "" {
		config.AnnotationPeerIPs = annotationPeerIPs
	}
	config.AnnotationSrcIP = metal.DefaultAnnotationSrcIP
	annotationSrcIP := os.Getenv(envVarAnnotationSrcIP)
	if annotationSrcIP != "" {
		config.AnnotationSrcIP = annotationSrcIP
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
		config.APIServerPort = apiServerNo
	case rawConfig.APIServerPort != 0:
		config.APIServerPort = rawConfig.APIServerPort
	default:
		config.APIServerPort = metal.DefaultAPIServerPort
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
func printMetalConfig(config metal.Config) {
	lines := config.Strings()
	for _, l := range lines {
		klog.Infof(l)
	}
}
