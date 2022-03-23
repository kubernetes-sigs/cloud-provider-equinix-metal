# End-to-End Tests

What end-to-end tests are required to test CPEM?

Currently, these are performed manually, but executing each of the steps in each section of this document.

In the next phase, we should create a single `make` target that runs them all. These require an Equinix Metal account
and token, deploying several real devices.

Eventually, these should be part of the CI flow.

The user flow is expected to be:

```console
export PACKET_AUTH_TOKEN=<my auth token>
make integration-tests
```

## Prerequisites

In order to test the CPEM, we need a Kubernetes cluster with at least some of the nodes - control plane or worker - on
Equinix Metal, depending on the test. For simplicity sake, all tests will use a cluster, all of whose nodes, both
control plane and worker, are on Equinix Metal.

It is not necessary to deploy the CPEM onto the cluster, as the CPEM can run in standalone mode.
This makes the testing part of the lifecycle simpler.

## Tests

The functionality that needs to be tested is as follows:

* node management - provider ID, deletion, metadata
* load balancer service - addition, update, deletion
* control plane EIP

### Node Management

* add new EQXM device to cluster, check that:
  * node receives provider ID
  * provider ID aligns with device UUID in Equinix Metal
  * node receives appropriate metadata as labels, specifically: type, addresses, zone (facility), region (metro)
* remove EQXM device by deleting, check that:
  * Kubernetes deletes the node

### LoadBalancer Services

As there are different loadbalancer implementations, these require different tests:

#### kube-vip, bgp

1. Start with 0 services of `type=LoadBalancer`
1. add 2 services of `type=LoadBalancer`, check that:
  * EIPs are created, have correct tags
  * BGP is enabled for project
  * BGP is enabled on each node with the services
  * annotations are created on each node, specifically, multiple of: local ASN, peer ASN, local IP, peer IP
1. delete 1 service of `type=LoadBalancer`, check that:
  * EIP is removed
  * annotations remain
  * EIP for 2nd service remains
1. delete 2nd service of `type=LoadBalancer`, check that:
  * EIP is removed
  * annotations are removed

#### metallb

1. Start with 0 services of `type=LoadBalancer`
1. add 2 services of `type=LoadBalancer`, check that:
  * EIPs are created, have correct tags
  * BGP is enabled for project
  * BGP is enabled on each node with the services
  * metallb configmap is modified to include:
    * an addresses entry for each service
    * a node entry for each combination of node and upstream peer
1. delete 1 service of `type=LoadBalancer`, check that:
  * EIP is removed
  * configmap remains, with address entry for 1st service removed
  * EIP for 2nd service remains
1. delete 2nd service of `type=LoadBalancer`, check that:
  * EIP is removed
  * configmap is emptied

### Control Plane EIP

This requires a control plane of 3 nodes; workers are not necessary.

1. Create the control plane nodes.
1. Create the EIP, tagged with a unique tag.
1. Start CPEM with the tag passed.
1. Check that the EIP has been attached to one of the control plane nodes.
1. Delete one of the non-EIP-attached nodes; check that the EIP remains attached to the same node.
1. Disable one of the EIP-attached nodes, but do not delete from EQXM API; check that the EIP is detached from that node and attached to the remaining node.
