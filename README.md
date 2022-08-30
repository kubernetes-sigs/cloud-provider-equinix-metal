![Kubernetes Cloud Provider for Equinix Metal](docs/images/github_kubernetes-cloud-provider_share.jpeg)

# Kubernetes Cloud Controller Manager for Equinix Metal

[![GitHub release](https://img.shields.io/github/release/equinix/cloud-provider-equinix-metal/all.svg?style=flat-square)](https://github.com/equinix/cloud-provider-equinix-metal/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/equinix/cloud-provider-equinix-metal)](https://goreportcard.com/report/github.com/equinix/cloud-provider-equinix-metal)
![Continuous Integration](https://github.com/equinix/cloud-provider-equinix-metal/workflows/Continuous%20Integration/badge.svg)
[![Docker Pulls](https://img.shields.io/docker/pulls/equinix/cloud-provider-equinix-metal.svg)](https://hub.docker.com/r/equinix/cloud-provider-equinix-metal/)
[![Slack](https://slack.equinixmetal.com/badge.svg)](https://slack.equinixmetal.com/)
[![Twitter Follow](https://img.shields.io/twitter/follow/equinixmetal.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=equinixmetal&user_id=788180534543339520)
![Equinix Metal Maintained](https://img.shields.io/badge/stability-maintained-green.svg)


`cloud-provider-equinix-metal` is the Kubernetes CCM implementation for Equinix Metal. Read more about the CCM in [the official Kubernetes documentation](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

This repository is [Maintained](https://github.com/packethost/standards/blob/master/maintained-statement.md)!

## Requirements

At the current state of Kubernetes, running the CCM requires a few things.
Please read through the requirements carefully as they are critical to running the CCM on a Kubernetes cluster.

### Version
Recommended versions of Equinix Metal CCM based on your Kubernetes version:

* Equinix Metal CCM version v0.0.4 supports Kubernetes version >=v1.10
* Equinix Metal CCM version v1.0.0+ supports Kubernetes version >=1.15.0

### BGP

If you plan on using a BGP, for example for a BGP-based load balancer, you _may_ need to set static routes on your hosts.

Details about BGP can be found in [the official Equinix Metal BGP documentation](https://metal.equinix.com/developers/docs/networking/local-global-bgp/#server-host-configuration).

Equinix Metal facilities provide BGP peers at certain addresses, normally `169.254.255.1` and `169.254.255.2`. These are available
in the host configuration via the Equinix Metal API, as well as the [metadata](https://metal.equinix.com/developers/docs/servers/metadata/)
on each host.

In order for BGP peering to work, the upstream BGP peers _must_ receive the packets from your device's _private_ IP address.
If they come from the _public_ address, they will be dropped.

There are two ways to get the packets to have the correct source address:

* use BGP software that knows how to set the source address on a packet
* set static routes on your host

#### BGP Software

Some implementations of BGP software support setting a source address for BGP peering packets, including
[bird](https://bird.network.cz), [kube-vip](https://kube-vip.io) and [metallb](http://metallb.universe.tf).

CCM helps in this regard. It reads the information about the peers and the correct source address for the device from
the Equinix Metal API, and then sets those as annotations on the host, or passes them to the BGP software.
Software that knows how to read those annotations, for example, kube-vip, will do the right thing, as will those that
are configured to receive it directly, such as metallb. There will be no need to set static routes.

#### Static Routes

If your BGP software does not support using a specific source IP, then you must set static routes.

You need to retrieve the following:

* your private IPv4 upstream gateway address
* your BGP peer addresses

Before you can retrieve the information, you must enable BGP at both the Equinix Metal project level, and for each device.
You can do this in the Equinix Metal Web UI, API or CLI. CCM ensures these settings on the project and each device. However,
if you wish to retrieve the information _before_ CCM enables it, for example to run the configuration below, you may need
to enable it first.

A sample method:

```bash
GATEWAY_IP=$(curl https://metadata.platformequinix.com/metadata | jq -r '.network.addresses[] | select(.public == false and .address_family == 4) | .gateway')
PEERS=$(curl https://metadata.platformequinix.com/metadata | jq -r '.bgp_neighbors[0].peer_ips[]')
for i in ${PEERS}; do
ip route add ${i} via $GATEWAY_IP
done
```

## Deployment

**TL;DR**

1. Set kubernetes binary arguments correctly, including for VLAN IPs, if used
1. Get your Equinix Metal project and secret API token
1. Deploy your Equinix Metal project and secret API token to your cluster in a [Secret](https://kubernetes.io/docs/concepts/configuration/secret/)
1. Deploy the CCM
1. Deploy the load balancer (optional)

### Kubernetes Binary Arguments

Control plane binaries in your cluster must start with the correct flags:

* `kubelet`: All kubelets in your cluster **MUST** set the flag `--cloud-provider=external`. This must be done for _every_ kubelet. Note that [k3s](https://k3s.io) sets its own CCM by default. If you want to use the CCM with k3s, you must disable the k3s CCM and enable this one, as `--disable-cloud-controller --kubelet-arg cloud-provider=external`.
* `kube-apiserver` and `kube-controller-manager` must **NOT** set the flag `--cloud-provider`. They then will use no cloud provider natively, leaving room for the Equinix Metal CCM.

**WARNING**: setting the kubelet flag `--cloud-provider=external` will taint all nodes in a cluster with `node.cloudprovider.kubernetes.io/uninitialized`.
The CCM itself will untaint those nodes when it initializes them.
Any pod that does not tolerate that taint will be unscheduled until the CCM is running.

You **must** set the kubelet flag the first time you run the kubelet. Stopping the kubelet, adding it after,
and then restarting it will not work.

#### Kubernetes node names must match the device name

By default, the kubelet will name nodes based on the node's hostname.
Equinix Metal's device hostnames are set based on the name of the device.
It is important that the Kubernetes node name matches the device name.

#### VLANs

If using Equinix Metal [Layer 2 VLANs](https://metal.equinix.com/developers/docs/layer2-networking/overview/), then you
likely are supplying your own private IPs. As CPEM uses the Equinix Metal API to determine IPs for Kubernetes nodes,
it does not know about the IPs you manage privately and assign to the node.

In this case, you **must** assign the node the IP you want as "internal" - used to communicate between nodes in the Kubernetes cluster -
using the [kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/) option `--node-ip`. CPEM respects this kubelet option
and, when it finds it has been used, will prefer it as a node's "internal" IP address.

#### Private Elastic IP

If you are using Equinix Metal [private Elastic IPs](https://metal.equinix.com/developers/docs/networking/ip-addresses/#private-ipv4-management-subnets),
which you have assigned to a node, and wish to use that IP as the "internal" IP for the Kubernetes node, you **must** assign the node the selected IP
using the [kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/) option `--node-ip`. CPEM respects this kubelet option
and, when it finds it has been used, will prefer it as a node's "internal" IP address. CPEM is intelligent enough to recognize that this IP
was provided both via `--node-ip` and via the Equinix Metal API, and will set it only once.

### Get Equinix Metal Project ID and API Token

To run `cloud-provider-equinix-metal`, you need your Equinix Metal project ID and secret API key ID that your cluster is running in.
If you are already logged into the [Equinix Metal portal](https://console.equinix.com/), you can create one by clicking on your
profile in the upper right then "API keys".
To get your project ID click into the project that your cluster is under and select "project settings" from the header.
Under General you will see "Project ID". Once you have this information you will be able to fill in the config needed for the CCM.

### Deploy Project and API

Copy [deploy/template/secret.yaml](./deploy/template/secret.yaml) to someplace useful:

```bash
cp deploy/template/secret.yaml /tmp/secret.yaml
```

Replace the placeholder in the copy with your token. When you're done, the `yaml` should look something like this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: metal-cloud-config
  namespace: kube-system
stringData:
  cloud-sa.json: |
    {
    "apiKey": "abc123abc123abc123",
    "projectID": "abc123abc123abc123"
    }  
```


Then apply the secret, e.g.:

```bash
kubectl apply -f /tmp/secret.yaml`
```

You can confirm that the secret was created with the following:

````bash
$ kubectl -n kube-system get secrets metal-cloud-config
NAME                  TYPE                                  DATA      AGE
metal-cloud-config   Opaque                                1         2m
````

### Deploy CCM

To apply the CCM itself, select your release and apply the manifest:

```
RELEASE=v2.0.0
kubectl apply -f https://github.com/equinix/cloud-provider-equinix-metal/releases/download/${RELEASE}/deployment.yaml
```

The CCM uses multiple configuration options. See the [configuration][#Configuration] section for all of the options.

#### Deploy Load Balancer

If you want load balancing to work as well, deploy a supported load-balancer.

CCM provides the correct logic, if necessary, to manage load balancer configs for supported load-balancers.

See further in this document under loadbalancing, for details.

### Logging

By default, ccm does minimal logging, relying on the supporting infrastructure from kubernetes. However, it does support
optional additional logging levels via the `--v=<level>` flag. In general:

* `--v=2`: log most function calls for devices and facilities, when relevant logging the returned values
* `--v=3`: log additional data when logging returned values, usually entire go structs
* `--v=5`: log every function call, including those called very frequently

## Configuration

The Equinix Metal CCM has multiple configuration options. These include three different ways to set most of them, for your convenience.

1. Command-line flags, e.g. `--option value` or `--option=value`; if not set, then
1. Environment variables, e.g. `CCM_OPTION=value`; if not set, then
1. Field in the configuration [Secret](https://kubernetes.io/docs/concepts/configuration/secret/); if not set, then
1. Default, if available; if not available, then an error

This section lists each configuration option, and whether it can be set by each method.

| Purpose | CLI Flag | Env Var | Secret Field | Default |
| --- | --- | --- | --- | --- |
| Path to config secret |    |    | `cloud-config` | error |
| API Key |    | `METAL_API_KEY` | `apiKey` | error |
| Project ID |    | `METAL_PROJECT_ID` | `projectID` | error |
| Metro in which to create LoadBalancer Elastic IPs |    | `METAL_METRO_NAME` | `metro` | Service-specific annotation, else error |
| Facility in which to create LoadBalancer Elastic IPs, only if Metro is not set |    | `METAL_FACILITY_NAME` | `facility` | Service-specific annotation, else metro |
| Base URL to Equinix API |    |    | `base-url` | Official Equinix Metal API |
| Load balancer setting |   | `METAL_LOAD_BALANCER` | `loadbalancer` | none |
| BGP ASN for cluster nodes when enabling BGP on the project; if the project **already** has BGP enabled, will use the existing BGP local ASN from the project |   | `METAL_LOCAL_ASN` | `localASN` | `65000` |
| BGP passphrase to use when enabling BGP on the project; if the project **already** has BGP enabled, will use the existing BGP pass from the project |   | `METAL_BGP_PASS` | `bgpPass` | `""` |
| Kubernetes annotation to set node's BGP ASN, `{{n}}` replaced with ordinal index of peer |   | `METAL_ANNOTATION_LOCAL_ASN` | `annotationLocalASN` | `"metal.equinix.com/bgp-peers-{{n}}-node-asn"` |
| Kubernetes annotation to set BGP peer's ASN, {{n}} replaced with ordinal index of peer |   | `METAL_ANNOTATION_PEER_ASN` | `annotationPeerASN` | `"metal.equinix.com/bgp-peers-{{n}}-peer-asn"` |
| Kubernetes annotation to set BGP peer's IPs, {{n}} replaced with ordinal index of peer |   | `METAL_ANNOTATION_PEER_IP` | `annotationPeerIP` | `"metal.equinix.com/bgp-peers-{{n}}-peer-ip"` |
| Kubernetes annotation to set source IP for BGP peering, {{n}} replaced with ordinal index of peer |   | `METAL_ANNOTATION_SRC_IP` | `annotationSrcIP` | `"metal.equinix.com/bgp-peers-{{n}}-src-ip"` |
| Kubernetes annotation to set BGP MD5 password, base64-encoded (see security warning below) |   | `METAL_ANNOTATION_BGP_PASS` | `annotationBGPPass` | `"metal.equinix.com/bgp-peers-{{n}}-bgp-pass"` |
| Kubernetes annotation to set the CIDR for the network range of the private address |  | `METAL_ANNOTATION_NETWORK_IPV4_PRIVATE` |  `annotationNetworkIPv4Private` | `metal.equinix.com/network-4-private` |
| Kubernetes Service annotation to set EIP metro |   | `METAL_ANNOTATION_EIP_METRO` | `annotationEIPMetro` | `"metal.equinix.com/eip-metro"` |
| Kubernetes Service annotation to set EIP facility |   | `METAL_ANNOTATION_EIP_FACILITY` | `annotationEIPFacility` | `"metal.equinix.com/eip-facility"` |
| Tag for control plane Elastic IP |    | `METAL_EIP_TAG` | `eipTag` | No control plane Elastic IP |
| Kubernetes API server port for Elastic IP |     | `METAL_API_SERVER_PORT` | `apiServerPort` | Same as `kube-apiserver` on control plane nodes, same as `0` |
| Filter for cluster nodes on which to enable BGP |    | `METAL_BGP_NODE_SELECTOR` | `bgpNodeSelector` | All nodes |
| Use host IP for Control Plane endpoint health checks | | `METAL_EIP_HEALTH_CHECK_USE_HOST_IP` | `eipHealthCheckUseHostIP` | false |

<u>Security Warning</u>
Including your project's BGP password, even base64-encoded, may have security implications. Because Equinix Metal
only allows communication to the BGP peer from the actual node, and not from outside, and because that password already is available
form metadata on the host, this risk may be limited. We further recommend using Kubernetes
[Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) to restrict access to BGP peers solely
to system pods that have reasonable need to access them.

## How It Works

The Kubernetes CCM for Equinix Metal deploys as a `Deployment` into your cluster with a replica of `1`. It provides the following services:

* lists and retrieves instances by ID, returning Equinix Metal servers
* manages load balancers

### Load Balancers

Equinix Metal does not offer managed load balancers like [AWS ELB](https://aws.amazon.com/elasticloadbalancing/)
or [GCP Load Balancers](https://cloud.google.com/load-balancing/). Instead, if configured to do so,
Equinix Metal CCM will interface with and configure external bare-metal loadbalancers.

When a load balancer is enabled, the CCM does the following:

1. Enable BGP for the project
1. Enable BGP on each node as it comes up
1. Sets ASNs based on configuration or default
1. For each `Service` of `type=LoadBalancer`:
   * If you have specified a load balancer IP on `Service.Spec.LoadBalancerIP` (bring your own IP, or BYOIP), do nothing
   * If you have not specified a load balancer IP on `Service.Spec.LoadBalancerIP`, get an Equinix Metal Elastic IP and set it on `Service.Spec.LoadBalancerIP`, see below
1. Pass control to the specific load balancer implementation

#### Service Load Balancer IP

There are two options for getting an Elastic IP (EIP) for a Service of `type=LoadBalancer`: bring-your-own
or let CCM create one using the Equinix API.

Whether you bring your own IP or rely on CCM to request one for you, the load balancer IP will be set, and
load balancers can consume them.

##### Bring Your Own IP

Whenever a `Service` of `type=LoadBalancer` is encountered, the CCM tries to ensure that an externally accessible load balancer IP is available.
It does this in one of two ways:

If you want to use a specific IP that you have ready, either because you brought it from the outside or because you retrieved an Elastic IP
from Equinix Metal separately, you can add it to the `Service` explicitly as `Service.Spec.LoadBalancerIP`. For example:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: ip-service
spec:
  selector:
    app: MyAppIP
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9376
  type: LoadBalancer
  loadBalancerIP: 145.60.80.60
```

CCM will detect that `loadBalancerIP` already was set and not try to create a new Equinix Metal Elastic IP.

##### Equinix EIP

If the `Service.Spec.LoadBalancerIP` was *not* set, then CCM will use the Equinix Metal API to request a new,
metro- or facility-specific Elastic IP and set it to `Service.Spec.LoadBalancerIP`.

The CCM needs to determine where to request the EIP. It does not attempt to figure out where the nodes are, as that can change over time,
the nodes might not be in existence when the CCM is running or `Service` is created, and you could run a Kubernetes cluster across
multiple facilities or potentially regions, or even cloud providers.

The CCM uses the following rules to determine where to create the EIP:

1. if facility is set globally using the environment variable `METAL_FACILITY_NAME`, use it; else
1. if metro is set globally using the environment variable `METAL_METRO_NAME`, use it; else
1. if the `Service` for which the EIP is being created has the annotation indicating in which facility the EIP should be created, use it; else
1. if the `Service` for which the EIP is being created has the annotation indicating in which metro the EIP should be created, use it; else
1. Return an error, cannot set an EIP

The overrides of environment variable and config file are provided so that you can run explicitly control where the EIPs
are created at a system-wide level, ignoring the annotations.

Using these flags and annotations, you can run the CCM on a node in a different metro or facility, or even outside of Equinix Metal entirely.

#### Control Plane LoadBalancer Implementation

For the control plane nodes, the Equinix Metal CCM uses static Elastic IP assignment, via the Equinix Metal API, to tell the
Equinix Metal network which control plane node should receive the traffic. For more details on the control plane
load-balancer, see [this section](#control-plane-load-balancing).

#### Service LoadBalancer Implementations

Loadbalancing is enabled as follows.

1. If the environment variable `METAL_LOAD_BALANCER` is set, read that. Else...
1. If the config file has a key named `loadbalancer`, read that. Else...
1. Load balancing is disabled.

The value of the loadbalancing configuration is `<type>:///<detail>` where:

* `<type>` is the named supported type, of one of those listed below
* `<detail>` is any additional detail needed to configure the implementation, details in the description below

For loadbalancing for Kubernetes `Service` of `type=LoadBalancer`, the following implementations are supported:

* [kube-vip](#kube-vip)
* [MetalLB](#metallb)
* [empty](#empty)

CCM does **not** deploy _any_ load balancers for you. It limits itself to managing the Equinix Metal-specific
API calls to support a load balancer, and providing configuration for supported load balancers.

##### kube-vip

When the [kube-vip](https://kube-vip.io) option is enabled, for user-deployed Kubernetes `Service` of `type=LoadBalancer`,
the Equinix Metal CCM enables BGP on the project and nodes, assigns an EIP for each such
`Service`, and adds annotations to the nodes. These annotations are configured to be consumable
by kube-vip.

To enable it, set the configuration `METAL_LOAD_BALANCER` or config `loadbalancer` to:

```
kube-vip://
```

Directions on using configuring kube-vip in this method are available at the kube-vip [site](https://kube-vip.io/hybrid/daemonset/#equinix-metal-overview-(using-the-%5Bequinix-cloud-provider-equinix-metal%5D(https://github.com/equinix/cloud-provider-equinix-metal)))

If `kube-vip` management is enabled, then CCM does the following.

1. Enable BGP on the Equinix Metal project
1. For each node currently in the cluster or added:
   * retrieve the node's Equinix Metal ID via the node provider ID
   * retrieve the device's BGP configuration: node ASN, peer ASN, peer IPs, source IP
   * add the information to appropriate annotations on the node
1. For each service of `type=LoadBalancer` currently in the cluster or added:
   * if an Elastic IP address reservation with the appropriate tags exists, and the `Service` already has that IP address affiliated with it, it is ready; ignore
   * if an Elastic IP address reservation with the appropriate tags exists, and the `Service` does not have that IP affiliated with it, add it to the [service spec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicespec-v1-core)
   * if an Elastic IP address reservation with the appropriate tags does not exist, create it and add it to the services spec; see [Equinix EIP][Equinix EIP] to control in which metro or facility the EIP will be created.
1. For each service of `type=LoadBalancer` deleted from the cluster:
   * find the Elastic IP address from the service spec and remove it
   * delete the Elastic IP reservation from Equinix Metal

##### MetalLB

**Supported Versions**: MetalLB [version 0.11.0](https://metallb.universe.tf/release-notes/#version-0-11-0) through [version 0.12.1](https://metallb.universe.tf/release-notes/#version-0-12-1). We are in the process of adding support for version 0.13.x.

When [MetalLB](https://metallb.universe.tf) is enabled, for user-deployed Kubernetes `Service` of `type=LoadBalancer`,
the Equinix Metal CCM uses BGP and to provide the _equivalence_ of load balancing, without
requiring an additional managed service (or hop). BGP route advertisements enable Equinix Metal's network
to route traffic for your services at the Elastic IP to the correct host.

To enable it, set the configuration `METAL_LOAD_BALANCER` or config `loadbalancer` to:

```
metallb:///<configMapNamespace>/<configMapName>
```

For example:

* `metallb:///metallb-system/config` - enable `MetalLB` management and update the configmap `config` in the namespace `metallb-system`
* `metallb:///foonamespace/myconfig` -  - enable `MetalLB` management and update the configmap `myconfig` in the namespace `foonamespae`
* `metallb:///` - enable `MetalLB` management and update the default configmap, i.e. `config` in the namespace `metallb-system`

Notice the **three* slashes. In the URL, the namespace and the configmap are in the path.

When enabled, CCM controls the loadbalancer by updating the provided `ConfigMap`.

If `MetalLB` management is enabled, then CCM does the following.

1. Get the appropriate namespace and name of the `ConfigMap`, based on the rules above.
1. If the `ConfigMap` does not exist, do the rest of the behaviours, but do not update the `ConfigMap`
1. Enable BGP on the Equinix Metal project
1. For each node currently in the cluster or added:
   * retrieve the node's Equinix Metal ID via the node provider ID
   * retrieve the device's BGP configuration: node ASN, peer ASN, peer IPs, source IP
   * add them to the metallb `ConfigMap` with a kubernetes selector ensuring that the peer is only for this node
1. For each node deleted from the cluster:
   * remove the node from the MetalLB `ConfigMap`
1. For each service of `type=LoadBalancer` currently in the cluster or added:
   * if an Elastic IP address reservation with the appropriate tags exists, and the `Service` already has that IP address affiliated with it, it is ready; ignore
   * if an Elastic IP address reservation with the appropriate tags exists, and the `Service` does not have that IP affiliated with it, add it to the [service spec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicespec-v1-core) and ensure it is in the pools of the MetalLB `ConfigMap` with `auto-assign: false`
   * if an Elastic IP address reservation with the appropriate tags does not exist, create it and add it to the services spec, and ensure is in the pools of the metallb `ConfigMap` with `auto-assign: false`; see [Equinix EIP][Equinix EIP] to control in which metro or facility the EIP will be created.
1. For each service of `type=LoadBalancer` deleted from the cluster:
   * find the Elastic IP address from the service spec and remove it
   * remove the IP from the `ConfigMap`
   * delete the Elastic IP reservation from Equinix Metal

CCM itself does **not** deploy the load-balancer or any part of it, including the `ConfigMap`. It only
modifies an existing `ConfigMap`. This can be deployed by the administrator separately, using the manifest
provided in the releases page, or in any other manner.

In order to instruct metallb which IPs to announce and from where, CCM takes direct responsibility for managing the
metallb `ConfigMap`. As described above, this is normally at `metallb-system/config`. 

You **should not** attempt to modify this `ConfigMap` separately, as CCM will modify it with each loop. Modifying it
separately is likely to break metallb's functioning.

In addition to the usual entries in the `ConfigMap`, CCM adds
[nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) entries
that are specifically structured to be ignored by metallb.

For example:

```yaml
  node-selectors:
  - match-labels:
      kubernetes.io/hostname: dc-worker-1
  - match-labels:
      nomatch.metal.equinix.com/service-namespace: default
      nomatch.metal.equinix.com/service-name: nginx-deployment
  - match-labels:
      nomatch.metal.equinix.com/service-namespace: ai
      nomatch.metal.equinix.com/service-name: trainer
```

`node-selectors` are grouped together with a logical OR. The above thus means that it will match
_any_ node that has any of the 3 sets of labels. The node with the hostname `dc-worker-1` will be matched,
independent of the other selectors.

The remaining selectros are used to allow CCM to track which services are being announced by which node.
These are ignored, as long as no such labels exist on any nodes. This is why the labels are called the clearly
non-matching names of `nomatch.metal.equinix.com/service-namespace` and
`nomatch.metal.equinix.com/service-name`

##### empty

When the `empty` option is enabled, for user-deployed Kubernetes `Service` of `type=LoadBalancer`,
the Equinix Metal CCM enables BGP on the project and nodes, assigns an EIP for each such
`Service`, and adds annotations to the nodes. It does not integrate directly with any load balancer.
This is useful if you have your own implementation, but want to leverage Equinix Metal CCM's
management of BGP and EIPs.

To enable it, set the configuration `METAL_LOAD_BALANCER` or config `loadbalancer` to:

```
empty://
```

If `empty` management is enabled, then CCM does the following.

1. Enable BGP on the Equinix Metal project
1. For each node currently in the cluster or added:
   * retrieve the node's Equinix Metal ID via the node provider ID
   * retrieve the device's BGP configuration: node ASN, peer ASN, peer IPs, source IP
   * add the information to appropriate annotations on the node
1. For each service of `type=LoadBalancer` currently in the cluster or added:
   * if an Elastic IP address reservation with the appropriate tags exists, and the `Service` already has that IP address affiliated with it, it is ready; ignore
   * if an Elastic IP address reservation with the appropriate tags exists, and the `Service` does not have that IP affiliated with it, add it to the [service spec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicespec-v1-core)
   * if an Elastic IP address reservation with the appropriate tags does not exist, create it and add it to the services spec
1. For each service of `type=LoadBalancer` deleted from the cluster:
   * find the Elastic IP address from the service spec and remove it
   * delete the Elastic IP reservation from Equinix Metal

### Language

In order to ease understanding, we use several different terms for an IP address:

* Requested: A dedicated `/32` IP address has been requested for the service from Equinix Metal. It may be returned immediately, or it may need to wait for Equinix Metal intervention.
* Reserved: A dedicated `/32` IP address has been reserved for the service from Equinix Metal.
* Assigned: The dedicated IP address has been marked on the service as `Service.Spec.LoadBalancerIP` as assigned.
* Mapped: The dedicated IP address has been added to the metallb `ConfigMap` as available.

From Equinix Metal's perspective, the IP reservation is either Requested or Reserved, but not both. For the
load balancer to work, the IP address needs to be all of: Reserved, Assigned, Mapped.

## Control Plane Load Balancing

CCM implements an optional control plane load balancer using an Equinix Metal Elastic IP (EIP) and the Equinix Metal API's
ability to assign that EIP to different devices.

You have several options for control plane load-balancing:

* CCM managed
* kube-vip managed
* No control plane load-balancing (or at least, none known to CCM)

### CCM Managed

It is a common procedure to use Elastic IP as Control Plane endpoint in order to
have a static endpoint that you can use from the outside, or when configuring
the advertise address for the kubelet.

To enable CCM to manage the control plane EIP:

1. Create an Elastic IP, using the Equinix Metal API, Web UI or CLI
1. Put an arbitrary but unique tag on the EIP
1. When starting the CCM
   * set the [configuration][Configuration] for the control plane EIP tag, e.g. env var `METAL_EIP_TAG=<tag>`, where `<tag>` is whatever tag you set on the EIP
   * (optional) set the port that the EIP should listen on; by default, or when set to `0`, it will use the same port as the `kube-apiserver` on the control plane nodes. This port can also be specified with `METAL_API_SERVER_PORT=<port>.`
   * (optional) set the [configuration][Configuration] for using the host IP for control plane endpoint health checks. This is
   needed when the EIP is configured as an loopback IP address, such as the case with [CAPP](https://github.com/kubernetes-sigs/cluster-api-provider-packet)

In [CAPP](https://github.com/kubernetes-sigs/cluster-api-provider-packet) we
create one for every cluster for example. Equinix Metal does not provide an as a
service load balancer it means that in some way we have to check if the Elastic
IP is still assigned to an healthy control plane.

In order to do so CCM implements a reconciliation loop that checks if the
Control Plane Endpoint respond correctly using the `/healthz` endpoint.

When the healthcheck fails CCM looks for the other Control Planes, when it gets
a healthy one it move the Elastic IP to the new device.

This feature by default is disabled and it assumes that the ElasticIP for the
cluster is available and tagged with an arbitrary label. CAPP for example uses:

```
cluster-api-provider-packet:cluster-id:<clusterName>
```

When the tag is present CCM will filter the available elastic ip for the
specified project via tag to lookup the one used by your cluster.

It will check the correct answer, when it stops responding the IP reassign logic
will start.

The logic will circle over all the available control planes looking for an
active api server. As soon as it can find one the Elastic IP will be unassigned
and reassigned to the working node.

#### How the Elastic IP Traffic is Routed

Of course, even if the router sends traffic for your Elastic IP (EIP) to a given control
plane node, that node needs to know to process the traffic. Rather than require you to
manage the IP assignment on each node, which can lead to some complex timing issues,
the Equinix Metal CCM handles it for you.

The structure relies on the already existing `default/kubernetes` service, which
creates an `Endpoints` structure that includes all of the functioning control plane
nodes. The CCM does the following on each loop:

1. Reads the Kubernetes-created `default/kubernetes` service to discover:
   * what port `kube-apiserver` is listening on from `targetPort`
   * all of the endpoints, i.e. control plane nodes where `kube-apiserver` is running
1. Creates a service named `kube-system/cloud-provider-equinix-metal-kubernetes-external` with the following settings:
   * `type=LoadBalancer`
   * `spec.loadBalancerIP=<eip>`
   * `status.loadBalancer.ingress[0].ip=<eip>`
   * `metadata.annotations["metallb.universe.tf/address-pool"]=disabled-metallb-do-not-use-any-address-pool`
   * `spec.ports[0].targetPort=<targetPort>`
   * `spec.ports[0].port=<targetPort_or_override>`
1. Updates the service `kube-system/cloud-provider-equinix-metal-kubernetes-external` to have endpoints identical to those in `default/kubernetes`

This has the following effect:

* the annotation prevents metallb from trying to manage the IP
* the name prevents CCM from passing it to the loadbalancer provider address mapping, thus preventing any of them from managing it
* the `spec.loadBalancerIP` and `status.loadBalancer.ingress[0].ip` cause kube-proxy to set up routes on all of the nodes
* the endpoints cause the traffic to be routed to the control plane nodes

Note that we _wanted_ to just set `externalIPs` on the original `default/kubernetes`, but that would prevent traffic
from being routed to it from the control nodes, due to iptables rules. LoadBalancer types allow local traffic.

### kube-vip Managed

kube-vip has the ability to manage the Elastic IP and control plane load-balancing. To enable it:

1. Disable CCM control-plane load-balancing, by ensuring the EIP tag setting is empty via `METAL_EIP_TAG=""`
1. Enable kube-vip control plane load-balancing by following the instructions [here](https://kube-vip.io/hybrid/static/#bgp-with-equinix-metal)

## Core Control Loop

The CCM does not maintain its own control loop, instead relying on the services provided by
[cloud-provider](https://pkg.go.dev/k8s.io/cloud-provider).

On startup, the CCM:

1. Implements the [cloud-provider interface](https://pkg.go.dev/k8s.io/cloud-provider#Interface), providing primarily the following API calls:
   * `Initialize()`
   * `InstancesV2()`
   * `LoadBalancer()`
1. In `Initialize`:
   1. If BGP is configured, enable BGP on the project
   1. If EIP control plane management is enabled, create an informer for `Service`, `Node` and `Endpoints`, updating the control plane EIP as needed.

The CCM then relies on the cloud-provider control loop to call it:

* whenever a `Node` is added, to get node metadata
* whenever a `Service` of `type=LoadBalancer` is added, removed or updated
* if EIP control plane management is enabled, via shared informers:
  * whenever a control plane `Node` is added, removed or updated
  * whenever the `default/kubernetes` service is added or updated
  * whenever the endpoints behind the `default/kubernetes` service are added, updated or removed

Further, it relies on the `resync` property of the above to ensure it always is up to date, and did not miss any events.

## BGP Configuration

If a loadbalancer is enabled, the CCM enables BGP for the project and enables it by default
on all nodes as they come up. It sets the ASNs as follows:

* Node, a.k.a. local, ASN: `65000`
* Peer Router ASN: `65530`

These are the settings per Equinix Metal's BGP config, see [here](https://github.com/packet-labs/kubernetes-bgp). It is
_not_ recommended to override them. However, you can do so, using the options in [Configuration][Configuration].

Set of servers on which BGP will be enabled can be filtered as well, using the the options in [Configuration][Configuration].
Value for node selector should be a valid Kubernetes label selector (e.g. key1=value1,key2=value2).

## Node Annotations

The Equinix Metal CCM sets Kubernetes annotations on each cluster node.

* Node, or local, ASN, default annotation `metal.equinix.com/bgp-peers-{{n}}-node-asn`
* Peer ASN, default annotation `metal.equinix.com/bgp-peers-{{n}}-peer-asn`
* Peer IP, default annotation `metal.equinix.com/bgp-peers-{{n}}-peer-ip`
* Source IP to use when communicating with peer, default annotation `metal.equinix.com/bgp-peers-{{n}}-src-ip`
* BGP password for peer, default annotation `metal.equinix.com/bgp-peers-{{n}}-bgp-pass`
* CIDR of the private network range in the project which this node is part of, default annotation `metal.equinix.com/network-4-private`

These annotation names can be overridden, if you so choose, using the options in [Configuration][Configuration].

Note that the annotations for BGP peering are a _pattern_. There is one annotation per data point per peer,
following the pattern `metal.equinix.com/bgp-peers-{{n}}-<info>`, where:

* `{{n}}` is the number of the peer, **always** starting with `0`
* `<info>` is the relevant information, such as `node-asn` or `peer-ip`

For example:

* `metal.equinix.com/bgp-peers-0-peer-asn` - ASN of peer 0
* `metal.equinix.com/bgp-peers-1-peer-asn` - ASN of peer 1
* `metal.equinix.com/bgp-peers-0-peer-ip` - IP of peer 0
* `metal.equinix.com/bgp-peers-1-peer-ip` - IP of peer 1

## Elastic IP Configuration

If a loadbalancer is enabled, CCM creates an Equinix Metal Elastic IP (EIP) reservation for each `Service` of
`type=LoadBalancer`. It tags the Reservation with the following tags:

* `usage="cloud-provider-equinix-metal-auto"`
* `service="<service-hash>"` where `<service-hash>` is the sha256 hash of `<namespace>/<service-name>`. We do this so that the name of the service does not leak out to Equinix Metal itself.
* `cluster=<clusterID>` where `<clusterID>` is the UID of the immutable `kube-system` namespace. We do this so that if someone runs two clusters in the same project, and there is one `Service` in each cluster with the same namespace and name, then the two EIPs will not conflict.

IP addresses always are created `/32`.
