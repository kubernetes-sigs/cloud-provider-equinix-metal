# Kubernetes Cloud Controller Manager for Equinix Metal

[![GitHub release](https://img.shields.io/github/release/packethost/metal-ccm/all.svg?style=flat-square)](https://github.com/packethost/metal-ccm/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/packethost/metal-ccm)](https://goreportcard.com/report/github.com/packethost/metal-ccm)
![Continuous Integration](https://github.com/packethost/metal-ccm/workflows/Continuous%20Integration/badge.svg)
[![Docker Pulls](https://img.shields.io/docker/pulls/packethost/metal-ccm.svg)](https://hub.docker.com/r/packethost/metal-ccm/)
[![Slack](https://slack.equinixmetal.com/badge.svg)](https://slack.equinixmetal.com/)
[![Twitter Follow](https://img.shields.io/twitter/follow/equinixmetal.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=equinixmetal&user_id=788180534543339520)
![Equinix Metal Maintained](https://img.shields.io/badge/stability-maintained-green.svg)


`metal-ccm` is the Kubernetes CCM implementation for Equinix Metal. Read more about the CCM in [the official Kubernetes documentation](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

This repository is [Maintained](https://github.com/packethost/standards/blob/master/maintained-statement.md)!

## Requirements

At the current state of Kubernetes, running the CCM requires a few things.
Please read through the requirements carefully as they are critical to running the CCM on a Kubernetes cluster.

### Version
Recommended versions of Equinix Metal CCM based on your Kubernetes version:

* Equinix Metal CCM version v0.0.4 supports Kubernetes version >=v1.10
* Equinix Metal CCM version v1.0.0+ supports Kubernetes version >=1.15.0

## Deployment

**TL;DR**

1. Set kubernetes binary arguments correctly
1. Get your Equinix Metal project and secret API token
1. Deploy your Equinix Metal project and secret API token to your cluster
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

### Get Equinix Metal Project ID and API Token

To run `metal-ccm`, you need your Equinix Metal project ID and secret API key ID that your cluster is running in.
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


Then apply the file via `kubectl`, e.g.:

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
kubectl apply -f https://github.com/packethost/metal-ccm/releases/download/${RELEASE}/deployment.yaml
```

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

## How It Works

The Kubernetes CCM for Equinix Metal deploys as a `Deployment` into your cluster with a replica of `1`. It provides the following services:

* lists available zones, returning Equinix Metal regions
* lists and retrieves instances by ID, returning Equinix Metal servers
* manages load balancers

### Facility

The Equinix Metal CCM works in one facility at a time. You can control which facility it works with as follows:

1. If the environment variable `METAL_FACILITY_NAME` is set, use that value. Else..
1. If the config file has a field named `facility`, use that value. Else..
1. Read the facility from the Equinix Metal metadata of the host where the CCM is running. Else..
1. Fail.

The overrides of environment variable and config file are provided so that you can run the CCM
on a node in a different facility, or even outside of Equinix Metal entirely.

### Load Balancers

Equinix Metal does not offer managed load balancers like [AWS ELB](https://aws.amazon.com/elasticloadbalancing/)
or [GCP Load Balancers](https://cloud.google.com/load-balancing/). Instead, if configured to do so,
Equinix Metal CCM will interface with and configure external bare-metal loadbalancers.

When a load balancer is enabled, the CCM does the following:

1. Enable BGP for the project
1. Enable BGP on each node as it comes up
1. Sets ASNs based on configuration or default
1. Get an Equinix Metal Elasic IP for each `Service` of `type=LoadBalancer`
1. Set the `Spec.LoadBalancerIP` on the `Service`
1. Pass control to the specific load balancer implementation

#### Control Plane LoadBalancer Implementation

For the control plane nodes, the Equinix Metal CCM uses static Elastic IP assignment, via the Equinix Metal API, to tell the
Equinix Metal network which control plane node should receive the traffic. For more details on the control plane
load-balancer, see [this section](#Elastic_IP_as_Control_Plane_Endpoint).

#### Service LoadBalancer Implementations

Loadbalancing is enabled as follows.

1. If the environment variable `METAL_LB` is set, read that. Else...
1. If the config file has a key named `metalLB`, read that. Else...
1. Load balancing is disabled.

The value of the loadbalancing configuration is `<type>://<detail>` where:

* `<type>` is the named supported type, of one of those listed below
* `<detail>` is any additional detail needed to configure the implementation, details in the description below

For loadbalancing for Kubernetes `Service` of `type=LoadBalancer`, the following implementations are supported:

* [kube-vip](#kube-vip)
* [metallb](#metallb)
* [empty](#empty)

CCM does **not** deploy _any_ load balancers for you. It limits itself to managing the Equinix Metal-specific
API calls to support a load balancer, and providing configuration for supported load balancers.

##### kube-vip

When the [kube-vip](https://kube-vip.io) option is enabled, for user-deployed Kubernetes `Service` of `type=LoadBalancer`,
the Equinix Metal CCM enables BGP on the project and nodes, assigns an EIP for each such
`Service`, and adds annotations to the nodes. These annotations are configured to be consumable
by kube-vip.

To enable it, set the configuration `METAL_LB` or config `metalLB` to:

```
kube-vip://
```

Directions on using configuring kube-vip in this method are available at the kube-vip [site](https://kube-vip.io/hybrid/daemonset/#equinix-metal-overview-(using-the-%5Bequinix-metal-ccm%5D(https://github.com/packethost/metal-ccm)))

If `kube-vip` management is enabled, then CCM does the following.

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

##### metallb

When [metallb](https://metallb.universe.tf) is enabled, for user-deployed Kubernetes `Service` of `type=LoadBalancer`,
the Equinix Metal CCM uses BGP and to provide the _equivalence_ of load balancing, without
requiring an additional managed service (or hop). BGP route advertisements enable Equinix Metal's network
to route traffic for your services at the Elastic IP to the correct host.

To enable it, set the configuration `METAL_LB` or config `metalLB` to:

```
metallb://<configMapNamespace>:<configMapName>
```

For example:

* `metallb://metallb-system:config` - enable `metallb` management and update the configmap `config` in the namespace `metallb-system`
* `metallb://foonamespace:myconfig` -  - enable `metallb` management and update the configmap `myconfig` in the namespace `foonamespae`
* `metallb://` - enable `metallb` management and update the default configmap, i.e. `config` in the namespace `metallb-system`

When enabled, CCM controls the loadbalancer by updating the provided `ConfigMap`.

If `metallb` management is enabled, then CCM does the following.

1. Get the appropriate namespace and name of the `ConfigMap`, based on the rules above.
1. If the `ConfigMap` does not exist, do the rest of the behaviours, but do not update the `ConfigMap`
1. Enable BGP on the Equinix Metal project
1. For each node currently in the cluster or added:
   * retrieve the node's Equinix Metal ID via the node provider ID
   * retrieve the device's BGP configuration: node ASN, peer ASN, peer IPs, source IP
   * add them to the metallb `ConfigMap` with a kubernetes selector ensuring that the peer is only for this node
1. For each node deleted from the cluster:
   * remove the node from the metallb `ConfigMap`
1. For each service of `type=LoadBalancer` currently in the cluster or added:
   * if an Elastic IP address reservation with the appropriate tags exists, and the `Service` already has that IP address affiliated with it, it is ready; ignore
   * if an Elastic IP address reservation with the appropriate tags exists, and the `Service` does not have that IP affiliated with it, add it to the [service spec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicespec-v1-core) and ensure it is in the pools of the metallb `ConfigMap` with `auto-assign: false`
   * if an Elastic IP address reservation with the appropriate tags does not exist, create it and add it to the services spec, and ensure is in the pools of the metallb `ConfigMap` with `auto-assign: false`
1. For each service of `type=LoadBalancer` deleted from the cluster:
   * find the Elastic IP address from the service spec and remove it
   * remove the IP from the `ConfigMap`
   * delete the Elastic IP reservation from Equinix Metal

CCM itself does **not** deploy the load-balancer or any part of it, including the `ConfigMap`. It only
modifies an existing `ConfigMap`. This can be deployed by the administrator separately, using the manifest
provided in the releases page, or in any other manner.

##### empty

When the `empty` option is enabled, for user-deployed Kubernetes `Service` of `type=LoadBalancer`,
the Equinix Metal CCM enables BGP on the project and nodes, assigns an EIP for each such
`Service`, and adds annotations to the nodes. It does not integrate directly with any load balancer.
This is useful if you have your own implementation, but want to leverage Equinix Metal CCM's
management of BGP and EIPs.

To enable it, set the configuration `METAL_LB` or config `metalLB` to:

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
1. When starting the CCM, set the env var `METAL_EIP_TAG=<tag>`, where `<tag>` is whatever tag you set on the EIP

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

1. Finds all of the endpoints for `default/kubernetes` and creates or updates parallel endpoints in `kube-system/metal-ccm-kubernetes-external`
1. Creates a service named `kube-system/metal-ccm-kubernetes-external` with the following settings:
   * `type=LoadBalancer`
   * `spec.loadBalancerIP=<eip>`
   * `status.loadBalancer.ingress[0].ip=<eip>`
   * `metadata.annotations["metallb.universe.tf/address-pool"]=disabled-metallb-do-not-use-any-address-pool`

This has the following effect:

* the annotation prevents metallb from trying to manage it
* the `spec.loadBalancerIP` and `status.loadBalancer.ingress[0].ip` cause kube-proxy to set up routes on all of the nodes
* the endpoints cause the traffic to be routed to the control plane nodes

Note that we _wanted_ to just set `externalIPs` on the original `default/kubernetes`, but that would prevent traffic
from being routed to it from the control nodes, due to iptables rules. LoadBalancer types allow local traffic.

### kube-vip Managed

kube-vip has the ability to manage the Elastic IP and control plane load-balancing. To enable it:

1. Disable CCM control-plane load-balancing, by ensuring the EIP tag setting is empty via `METAL_EIP_TAG=""`
1. Enable kube-vip control plane load-balancing by following the instructions [here](https://kube-vip.io/hybrid/static/#bgp-with-equinix-metal)

## Core Control Loop

On startup, the CCM executes the following core control loop:

1. List each node in the cluster using a kubernetes node lister
   * for each node, call the node processing function in "sync" mode on each area, such as loadbalancers, bgp, devices, etc.
1. List each service in the cluster using a kubernetes service lister
   * for each node, if it is of `type=LoadBalancer`, call the service processing function in "sync" mode on each area, such as loadbalancers, bgp, devices, etc.
1. Start a kubernetes informer for node changes, responding to node addition and removals
   * for each node added, call the node processing function in "add" mode on each area
   * for each node removed, call the node processing function in "remove" mode on each area
1. Start a kubernetes informer for service changes, responding to service addition and removals of `type=LoadBalancer`
   * for each service added, call the service processing function in "add" mode on each area
   * for each service removed, call the service processing function in "remove" mode on each area
1. Start an independent loop that checks every 30 seconds (configurable) for the following:
   * list all nodes in the cluster using a kubernetes node lister, and call the node processing function in "sync" mode on each area
   * list all services in the cluster of `type=LoadBalancer`, and call the service processing function in "sync" mode on each area

## BGP Configuration

If a loadbalancer is enabled, the CCM enables BGP for the project and enables it by default
on all nodes as they come up. It sets the ASNs as follows:

* Node, a.k.a. local, ASN: 65000
* Peer Router ASN: 65530

These are the settings per Equinix Metal's BGP config, see [here](https://github.com/packet-labs/kubernetes-bgp). It is
_not_ recommended to override them. However, the settings are set as follows:

1. If the environment variables `METAL_LOCAL_ASN` and `METAL_PEER_ASN` are set. Else...
1. If the config file has fields named `localASN` and `peerASN`. Else...
1. Use the above defaults.

Set of servers on which BGP will be enabled can be filtered using the following settings:
1. If the environment variable `METAL_BGP_NODE_SELECTOR` is set. Else...
1. If the config file has field named `bgpNodeSelector` set. Else...
1. Select all nodes.

Value for node selector should be a valid Kubernetes label selector (e.g. key1=value1,key2=value2).

In addition to enabling BGP and setting ASNs, the Equinix Metal CCM sets Kubernetes annotations on the nodes. It sets the
following information:

* `metal.equinix.com/node-asn` - Node, or local, ASN
* `metal.equinix.com/peer-asns` - Peer ASNs, comma-separated if multiple
* `metal.equinix.com/peer-ips` - Peer IPs, comma-separated if multiple
* `metal.equinix.com/src-ip` - Source IP to use

These annotation names can be overridden, if you so choose. The settings are as follows:

1. If the environment variables `METAL_ANNOTATION_LOCAL_ASN`, `METAL_ANNOTATION_PEER_ASNS`, `METAL_ANNOTATION_PEER_IPS`, `METAL_ANNOTATION_SRC_IP` are set. Else...
1. If the config file has files named `annotationLocalASN`, `annotationPeerASNs`, `annotationPeerIPs`, `annotationSrcIP`. Else...
1. Use the above defaults.

## Elastic IP Configuration

If a loadbalancer is enabled, CCM creates an Equinix Metal Elastic IP (EIP) reservation for each `Service` of
`type=LoadBalancer`. It tags the Reservation with the following tags:

* `usage="metal-ccm-auto"`
* `service="<service-hash>"` where `<service-hash>` is the sha256 hash of `<namespace>/<service-name>`. We do this so that the name of the service does not leak out to Equinix Metal itself.
* `cluster=<clusterID>` where `<clusterID>` is the UID of the immutable `kube-system` namespace. We do this so that if someone runs two clusters in the same project, and there is one `Service` in each cluster with the same namespace and name, then the two EIPs will not conflict.

IP addresses always are created `/32`.

## Running Locally

You can run the CCM locally on your laptop or VM, i.e. not in the cluster. This _dramatically_ speeds up development. To do so:

1. Deploy everything except for the `Deployment` and, optionally, the `Secret`
1. Build it for your local platform `make build`
1. Set the environment variable `CCM_SECRET` to a file with the secret contents as a json, i.e. the content of the secret's `stringData`, e.g. `CCM_SECRET=ccm-secret.yaml`
1. Set the environment variable `KUBECONFIG` to a kubeconfig file with sufficient access to the cluster, e.g. `KUBECONFIG=mykubeconfig`
1. Set the environment variable `METAL_FACILITY_NAME` to the correct facility where the cluster is running, e.g. `METAL_FACILITY_NAME=ewr1`
1. If you want to run the loadbalancer, and it is not yet deployed, run `kubectl apply -f deploy/loadbalancer.yaml`
1. Enable the loadbalancer by setting the environment variable `METAL_LB=metallb://`
1. If you want to use a managed Elastic IP for the control plane, create one using the Equinix Metal API or Web UI, tag it uniquely, and set the environment variable `METAL_EIP_TAG=<tag>`
1. Run the command, e.g.:

```
METAL_FACILITY_NAME=${METAL_FACILITY_NAME} METAL_LB=metallb:// dist/bin/metal-cloud-controller-manager-darwin-amd64 --cloud-provider=metal --leader-elect=false --authentication-skip-lookup=true --provider-config=$CCM_SECRET --kubeconfig=$KUBECONFIG
```

For lots of extra debugging, add `--v=2` or even higher levels, e.g. `--v=5`.
