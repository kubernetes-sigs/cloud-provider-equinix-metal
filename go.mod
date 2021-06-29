module github.com/equinix/cloud-provider-equinix-metal

go 1.15

require (
	github.com/google/uuid v1.1.1 // indirect
	github.com/hashicorp/go-hclog v0.12.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.6
	github.com/packethost/packet-api-server v0.0.0-20200706140707-f0f79ef89944
	github.com/packethost/packngo v0.19.2-0.20211119121357-05157869e685
	github.com/pallinder/go-randomdata v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/cloud-provider v0.21.2
	k8s.io/component-base v0.21.2
	k8s.io/klog/v2 v2.8.0
)
