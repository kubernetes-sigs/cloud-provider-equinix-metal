package main

import (
	"encoding/json"
	goflag "flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration

	"github.com/packethost/packet-ccm/packet"
	"github.com/spf13/pflag"
)

const (
	apiKeyName    = "PACKET_API_KEY"
	projectIDName = "PACKET_PROJECT_ID"
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
	config, err := getPacketConfig(providerConfig)
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

func getPacketConfig(providerConfig string) (packet.Config, error) {
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

	if apiToken == "" {
		return config, fmt.Errorf("environment variable %q is required", apiKeyName)
	}

	if projectID == "" {
		return config, fmt.Errorf("environment variable %q is required", projectIDName)
	}
	return config, nil
}
