package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration

	_ "github.com/packethost/packet-ccm/packet"
	"github.com/spf13/pflag"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	// Bogus parameter needed until https://github.com/kubernetes/kubernetes/issues/76205
	// gets resolved.
	// once we move to kubernetes-1.15.0 clients, this should be fixed
	goflag.CommandLine.String("cloud-provider-gce-lb-src-cidrs", "", "NOT USED (workaround for https://github.com/kubernetes/kubernetes/issues/76205)")

	command := app.NewCloudControllerManagerCommand()

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
