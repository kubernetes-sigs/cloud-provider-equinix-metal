# Kubernetes Cloud Controller Manager for Equinix Metal

[![GitHub release](https://img.shields.io/github/release/packethost/packet-ccm/all.svg?style=flat-square)](https://github.com/packethost/packet-ccm/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/packethost/packet-ccm)](https://goreportcard.com/report/github.com/packethost/packet-ccm)
![Continuous Integration](https://github.com/packethost/packet-ccm/workflows/Continuous%20Integration/badge.svg)
[![Docker Pulls](https://img.shields.io/docker/pulls/packethost/packet-ccm.svg)](https://hub.docker.com/r/packethost/packet-ccm/)
[![Slack](https://slack.equinixmetal.com/badge.svg)](https://slack.equinixmetal.com/)
[![Twitter Follow](https://img.shields.io/twitter/follow/equinixmetal.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=equinixmetal&user_id=788180534543339520)
![Equinix Metal Maintained](https://img.shields.io/badge/stability-maintained-green.svg)


`packet-ccm` is the Kubernetes CCM implementation for Equinix Metal. Read more about the CCM in [the official Kubernetes documentation](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

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

To run `packet-ccm`, you need your Equinix Metal project ID and secret API key ID that your cluster is running in.
If you are already logged into the [Equinix Metal portal](ttps://console.equinix.com/), you can create one by clicking on your
profile in the upper right then "API keys".
To get your project ID click into the project that your cluster is under and select "project settings" from the header.
Under General you will see "Project ID". Once you have this information you will be able to fill in the config needed for the CCM.

#### Deploy Project and API

Copy [deploy/template/secret.yaml](./deploy/template/secret.yaml) to someplace useful:

```bash
cp deploy/template/secret.yaml /tmp/secret.yaml
```

Replace the placeholder in the copy with your token. When you're done, the `yaml` should look something like this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: packet-cloud-config
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
$ kubectl -n kube-system get secrets packet-cloud-config
NAME                  TYPE                                  DATA      AGE
packet-cloud-config   Opaque                                1         2m
````

#### Deploy CCM

To apply the CCM itself, select your release and apply the manifest:

```
RELEASE=v2.0.0
kubectl apply -f https://github.com/packethost/packet-ccm/releases/download/${RELEASE}/deployment.yaml
```

#### Deploy Load Balancer

If you want load balancing to work as well, deploy the load balancer manifest:

```
kubectl apply -f https://github.com/packethost/packet-ccm/releases/download/${RELEASE}/loadbalancer.yaml
```

As of this writing, the load balancer is metallb. CCM provides the correct logic to manage the load balancer
config.

You can deploy metallb on your own, and simply direct CCM to control a different configuration map, or
none at all. On startup and on each create or delete of a `Service` that is of `type=LoadBalancer`,
CCM checks for the existence of a `ConfigMap` in the correct namespace.

* if loadbalancer management is disabled, do nothing
* if it finds no configmap, do nothing
* if it finds a configmap, configure it

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

1. If the environment variable `PACKET_FACILITY_NAME` is set, use that value. Else..
1. If the config file has a field named `facility`, use that value. Else..
1. Read the facility from the Equinix Metal metadata of the host where the CCM is running. Else..
1. Fail.

The overrides of environment variable and config file are provided so that you can run the CCM
on a node in a different facility, or even outside of Equinix Metal entirely.

### BGP

The Equinix Metal CCM enables BGP for the project and enables it by default on all nodes as they come up. It sets the ASNs as follows:

* Node, a.k.a. local, ASN: 65000
* Peer Router ASN: 65530

These are the settings per Equinix Metal's BGP config, see [here](https://github.com/packet-labs/kubernetes-bgp). It is
_not_ recommended to override them. However, the settings are set as follows:

1. If the environment variables `PACKET_LOCAL_ASN` and `PACKET_PEER_ASN` are set. Else...
1. If the config file has fields named `localASN` and `peerASN`. Else...
1. Use the above defaults.

Set of servers on which BGP will be enabled can be filtered using the following settings:
1. If the environment variable `PACKET_BGP_NODE_SELECTOR` is set. Else...
1. If the config file has field named `bgpNodeSelector` set. Else...
1. Select all nodes.

Value for node selector should be a valid Kubernetes label selector (e.g. key1=value1,key2=value2).

In addition to enabling BGP and setting ASNs, the Equinix Metal CCM sets Kubernetes annotations on the nodes. It sets the
following information:

* `packet.com/node-asn` - Node, or local, ASN
* `packet.com/peer-asns` - Peer ASNs, comma-separated if multiple
* `packet.com/peer-ips` - Peer IPs, comma-separated if multiple
* `packet.com/src-ip` - Source IP to use


These annotation names can be overridden, if you so choose. The settings are as follows:

1. If the environment variables `PACKET_ANNOTATION_LOCAL_ASN`, `PACKET_ANNOTATION_PEER_ASNS`, `PACKET_ANNOTATION_PEER_IPS`, `PACKET_ANNOTATION_SRC_IP` are set. Else...
1. If the config file has files named `annotationLocalASN`, `annotationPeerASNs`, `annotationPeerIPs`, `annotationSrcIP`. Else...
1. Use the above defaults.

### Load Balancers

Equinix Metal does not offer managed load balancers like [AWS ELB](https://aws.amazon.com/elasticloadbalancing/)
or [GCP Load Balancers](https://cloud.google.com/load-balancing/). Instead, Equinix Metal provides the following
load-balancing options, both using Equinix Metal's Elastic IP.

For user-deployed Kubernetes `Service` of `type=LoadBalancer`, the Equinix Metal CCM uses BGP and
[metallb](https://metallb.universe.tf) to provide the _equivalence_ of load balancing, without
requiring an additional managed service (or hop). BGP route advertisements enable Equinix Metal's network
to route traffic for your services at the Elastic IP to the correct host. This section describes how to use it, and how to
disable it.

For the control plane nodes, the Equinix Metal CCM uses static Elastic IP assignment, via the Equinix Metal API, to tell the
Equinix Metal network which control plane node should receive the traffic. For more details on the control plane
load-balancer, see [this section](#Elastic_IP_as_Control_Plane_Endpoint).

By default, CCM controls the loadbalancer by updating the `ConfigMap` named `metallb-system:config`,
i.e. a `ConfigMap` named `config` in the `metallb-system` namespace. This behaviour is controlled
in one of two ways:

* by the environment variable `PACKET_LB_CONFIGMAP`
* by the property `loadbalancer-configmap` in the kubernetes secret `packet-cloud-config`

For each, CCM's behaviour is as follows:

* if unset, default to `metallb-system:config`
* if set to `<namespace>:<name>`, look for and manage a `ConfigMap` with name `<name>` in namespace `<namespace>`.You _must_ include a namespace; there is no default namespace when providing a specific `ConfigMap` name.
* if set to `disabled`, do not attempt to manage any config

Environment variable overrides setting in kubernetes secret; the secret will be checked only if
the environment variable is unset. Only if _both_ are unset will it default.

If the `ConfigMap` is not disabled, then upon start, CCM does the following. EIP tag descriptions follow this list.

1. Get the appropriate namespace and name of the `ConfigMap`, based on the rules above.
1. If the `ConfigMap` does not exist, do nothing more.
1. List each node in the cluster using a kubernetes node lister; for each:
   * retrieve its BGP node ASN, peer IPs and peer ASNs
   * add them to the metallb `ConfigMap` with a kubernetes selector ensuring that the peer is only for this node
1. Start a kubernetes informer for node changes, responding to node addition and removals
   * Node addition: ensure the node is in the metallb `ConfigMap` as above
   * Node deletion: remove the node from the metallb `ConfigMap`
1. List all of the services in the cluster using a kubernetes service lister; for each:
   * If the service is not of `type=LoadBalancer`, ignore it
   * If a facility-specific `/32` IP address reservation tagged with the appropriate tags exists, and it already has that IP address affiliated with it, it is ready; ignore
   * If the service does not have that IP affiliated with it, add it to the [service spec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicespec-v1-core) and ensure it is in the pools of the metallb `ConfigMap` with `auto-assign: false`
1. Start a kubernetes informer for service changes, responding to service addition and removals
   * Service addition: create a facility-specific `/32` IP address reservation tagged with appropriate tags; if it is ready immediately, add it to the [service spec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicespec-v1-core) and ensure is in the pools of the metallb `ConfigMap` with `auto-assign: false`
   * Service deletion: find the `/32` IP address on the service spec and remove it; remove from the `ConfigMap`
1. Start an independent loop that checks every 30 seconds (configurable) for IP address reservations that are tagged with appropriate tags but not on any services. If it finds one:
   * If a service exists that matches the `<service-hash>`, that is an indication that an IP address reservation request was made, not completed at request time, and now is available. Add the IP to the [service spec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicespec-v1-core) and ensure is in the pools of the metallb `ConfigMap` with `auto-assign: false`
   * If no service exists that is missing an IP, or none with a matching hash, delete the IP reservation

At **no** point does CCM itself deploy the load-balancer or any part of it, including the `ConfigMap`. It only
modifies an existing `ConfigMap`. This can be deployed by the administrator separately, using the manifest
provided in the releases page, or by any other manner.

In all cases of tagging the IP address reservation, we tag the IP reservation with the following tags:

* `usage="packet-ccm-auto"`
* `service="<service-hash>"` where `<service-hash>` is the sha256 hash of `<namespace>/<service-name>`. We do this so that the name of the service does not leak out to Equinix Metal itself.
* `cluster=<clusterID>` where `<clusterID>` is the UID of the immutable `kube-system` namespace. We do this so that if someone runs two clusters in the same project, and there is one `Service` in each cluster with the same namespace and name, then the two EIPs will not conflict.

IP addresses always are created `/32`.

### Language

In order to ease understanding, we use several different terms for an IP address:

* Requested: A dedicated `/32` IP address has been requested for the service from Equinix Metal. It may be returned immediately, or it may need to wait for Equinix Metal intervention.
* Reserved: A dedicated `/32` IP address has been reserved for the service from Equinix Metal.
* Assigned: The dedicated IP address has been marked on the service as `Service.Spec.LoadBalancerIP` as assigned.
* Mapped: The dedicated IP address has been added to the metallb `ConfigMap` as available.

From Equinix Metal's perspective, the IP reservation is either Requested or Reserved, but not both. For the
load balancer to work, the IP address needs to be all of: Reserved, Assigned, Mapped.

## Elastic IP as Control Plane Endpoint

It is a common procedure to use Elastic IP as Control Plane endpoint in order to
have a static endpoint that you can use from the outside, or when configuring
the advertise address for the kubelet.

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

### How the Elastic IP Traffic is Routed

Of course, even if the router sends traffic for your Elastic IP (EIP) to a given control
plane node, that node needs to know to process the traffic. Rather than require you to
manage the IP assignment on each node, which can lead to some complex timing issues,
the Equinix Metal CCM handles it for you.

The structure relies on the already existing `default/kubernetes` service, which
creates an `Endpoints` structure that includes all of the functioning control plane
nodes. The CCM does the following on each loop:

1. Finds all of the endpoints for `default/kubernetes` and creates or updates parallel endpoints in `kube-system/packet-ccm-kubernetes-external`
1. Creates a service named `kube-system/packet-ccm-kubernetes-external` with the following settings:
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

## Running Locally

You can run the CCM locally on your laptop or VM, i.e. not in the cluster. This _dramatically_ speeds up development. To do so:

1. Deploy everything except for the `Deployment` and, optionally, the `Secret`
1. Build it for your local platform `make build`
1. Set the environment variable `CCM_SECRET` to a file with the secret contents as a json, i.e. the content of the secret's `stringData`, e.g. `CCM_SECRET=ccm-secret.yaml`
1. Set the environment variable `KUBECONFIG` to a kubeconfig file with sufficient access to the cluster, e.g. `KUBECONFIG=mykubeconfig`
1. Set the environment variable `PACKET_FACILITY_NAME` to the correct facility where the cluster is running, e.g. `PACKET_FACILITY_NAME=ewr1`
1. If you want to run the loadbalancer, and it is not yet deployed, run `kubectl apply -f deploy/loadbalancer.yaml`
1. If you want to use a managed Elastic IP for the control plane, create one using the Equinix Metal API or Web UI, tag it uniquely, and set the environment variable `PACKET_EIP_TAG=<tag>`
1. Run the command, e.g.:

```
PACKET_FACILITY_NAME=${PACKET_FACILITY_NAME} dist/bin/packet-cloud-controller-manager-darwin-amd64 --cloud-provider=packet --leader-elect=false --allow-untagged-cloud=true --authentication-skip-lookup=true --provider-config=$CCM_SECRET --kubeconfig=$KUBECONFIG
```

For lots of extra debugging, add `--v=2` or even higher levels, e.g. `--v=5`.
