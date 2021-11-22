# Integration tests

What integration tests should be performed for the CPEM?

In the first phase, these will be executed manually, as a single command. These require an Equinix Metal account
and token, deploying several real devices. Eventually, these may be part of the CI flow.

The user flow is expected to be:

```console
export PACKET_AUTH_TOKEN=<my auth token>
make integration-tests
```

## Prerequisites

In order to test the CPEM, we need a single Kubernetes cluster whose worker node(s) are on Equinix Metal. Some of
the tests do not exercise the control plane, and thus do not require that the control plane nodes be on Equinix
Metal as well, while others do.

## Tests

To keep things simple, we separate the tests into those that do not require testing of the control plane, and those
that do.

### Non-Control Plane

Requirements: cluster with single control plane node and two worker nodes, deployed and in the `Ready` state.

* Node annotations - run CPEM and check that the worker nodes receive appropriate annotations
* Load balancer - run CPEM in each of the load balancer modes, and see if BGP is enabled for the project as well as on each node, if it should be.
* Load balancer - run CPEM in each of the load balancer modes
  * create a Service of `type=LoadBalancer` and see if the EIP is created and attached. For all modes, check annotations; for metallb, check `ConfigMap`
  * create a Service whose type is _not_ `LoadBalancer`, and see that the EIP is _not_ created, and that, for metallb, the `ConfigMap` is not updated
  * remove the Service of `type=LoadBalancer` and see that the EIP is detached and deleted. For all modes, check annotations; for metallb, check `ConfigMap`

### Control Plane

Requirements: clusters with three control plane nodes; worker nodes are not needed.

* Control plane load balancer setup - run CPEM with the correct configured EIP tag, see that the EIP is attached to one of the control plane nodes.
* Control plane load balancer failover - delete the node to which it was attached, and see that it gets attached to the alternate node.
