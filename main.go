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

	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"

	"github.com/packethost/packet-ccm/packet"
	"github.com/spf13/pflag"
)

const (
	apiKeyName               = "PACKET_API_KEY"
	projectIDName            = "PACKET_PROJECT_ID"
	facilityName             = "PACKET_FACILITY_NAME"
	disableLoadBalancerName  = "PACKET_DISABLE_LB"
	envVarLocalASN           = "PACKET_LOCAL_ASN"
	envVarPeerASN            = "PACKET_PEER_ASN"
	envVarAnnotationLocalASN = "PACKET_ANNOTATION_LOCAL_ASN"
	envVarAnnotationPeerASNs = "PACKET_ANNOTATION_PEER_ASNS"
	envVarAnnotationPeerIPs  = "PACKET_ANNOTATION_PEER_IPS"
	// fixedDisableLoadBalancers disabled until the packet API is ready with IP management
	fixedDisableLoadBalancers = true
)

var (
	providerConfig           string
	loadBalancerManifestPath string
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	command := app.NewCloudControllerManagerCommand()

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	// add our config
	command.PersistentFlags().StringVar(&providerConfig, "provider-config", "", "path to provider config file")
	command.PersistentFlags().StringVar(&loadBalancerManifestPath, "load-balancer-manifest", "", "path to load-balancer manifests")

	logs.InitLogs()
	defer logs.FlushLogs()

	// parse our flags so we get the providerConfig
	command.ParseFlags(os.Args[1:])

	// register the provider
	config, err := getPacketConfig(providerConfig, loadBalancerManifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider config error: %v\n", err)
		os.Exit(1)
	}
	// register the provider
	if err := packet.InitializeProvider(config); err != nil {
		fmt.Fprintf(os.Stderr, "provider initialization error: %v\n", err)
		os.Exit(1)
	}

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func getPacketConfig(providerConfig, loadBalancerManifestPath string) (packet.Config, error) {
	// get our token and project
	var config, rawConfig packet.Config
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

	disableLoadBalancer := os.Getenv(disableLoadBalancerName)
	config.DisableLoadBalancer = rawConfig.DisableLoadBalancer
	if disableLoadBalancer == "true" || fixedDisableLoadBalancers {
		config.DisableLoadBalancer = true
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

	// try to read the load balancer manifest
	if !config.DisableLoadBalancer {
		b, err := ioutil.ReadFile(loadBalancerManifestPath)
		if err != nil {
			return config, fmt.Errorf("error loading the load balancer manifest from %s: %v", loadBalancerManifestPath, err)
		}
		config.LoadBalancerManifest = b
	}

	// if facility was not defined, retrieve it from our metadata
	if facility == "" {
		metadata, err := packet.GetAndParseMetadata("")
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
		config.LocalASN = packet.DefaultLocalASN
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
		config.PeerASN = packet.DefaultPeerASN
	}

	// set the annotations
	config.AnnotationLocalASN = packet.DefaultAnnotationNodeASN
	annotationLocalASN := os.Getenv(envVarAnnotationLocalASN)
	if annotationLocalASN != "" {
		config.AnnotationLocalASN = annotationLocalASN
	}
	config.AnnotationPeerASNs = packet.DefaultAnnotationPeerASNs
	annotationPeerASNs := os.Getenv(envVarAnnotationPeerASNs)
	if annotationPeerASNs != "" {
		config.AnnotationPeerASNs = annotationPeerASNs
	}
	config.AnnotationPeerIPs = packet.DefaultAnnotationPeerIPs
	annotationPeerIPs := os.Getenv(envVarAnnotationPeerIPs)
	if annotationPeerIPs != "" {
		config.AnnotationPeerIPs = annotationPeerIPs
	}

	return config, nil
}
