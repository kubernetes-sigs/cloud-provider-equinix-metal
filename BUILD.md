# CCM for Equinix Metal Build and Design

The Cloud Controller Manager (CCM) Equinix Metal plugin enables a Kubernetes cluster to interface directly with
Equinix Metal cloud services.

## Deploy

Read how to deploy the Kubernetes CCM for Equinix Metal in the [README.md](./README.md)!

## Building

To build the binary, run:

```
make build
```

It will deposit the binary for your local architecture as `dist/bin/cloud-provider-equinix-metal-$(OS)-$(ARCH)`

By default `make build` builds the binary using your locally installed go toolchain.
To build it using a docker container, do:

```
make build DOCKERBUILD=true
```

By default, it will build for your local operating system and CPU architecture. You can build for alternate architectures or operating systems via the `OS` and `ARCH` parameters:

```
make build OS=darwin
make build OS=linux ARCH=arm64
```

## Docker Image

To build a docker image, run:

```
make image
```

The image will be tagged with `:latest`

## CI/CD/Release pipeline

The CI/CD/Release pipeline is run via the following steps:

- `make ci`: builds the binary, runs all tests, builds the OCI image
- `make cd`: takes the image from the prior stage, tags it with the name of the branch and git hash from the commit, and pushes to the docker registry
- `make release`: takes the image from the `ci` stage, tags it with the git tag, and pushes to the docker registry

The assumptions about workflow are as follows:

- `make ci` can be run anywhere. The built binaries and OCI image will be named and tagged as per `make build` and `make image` above.
- `make cd` should be run only on a merge into `main`. It generally will be run only in a CI system, e.g. [drone](https://drone.io) or [github actions](https://github.com/features/actions). It requires passing both `CONFIRM=true` to tell it that it is ok to push, and `BRANCH_NAME=${BRANCH_NAME}` to tell it what tag should be used in addition to the git hash. For example, to push out the current commit as main: `make cd CONFIRM=true BRANCH_NAME=main`
- `make release` should be run only on applying a tag to `main`, although it can run elsewhere. It generally will be run only in a CI system. It requires passing both `CONFIRM=true` to tell it that it is ok to push, and `RELEASE_TAG=${RELEASE_TAG}` to tell it what tag this release should be. For example, to push out a tagged version `v1.2.3` on the current commit: `make release CONFIRM=true RELEASE_TAG=v1.2.3`.

For both `make cd` and `make release`, if you wish to push out a _different_ commit, then check that one out first.

The flow to make changes normally should be:

1. `main` is untouched, a protected branch.
2. In your local copy, create a new working branch.
3. Make your changes in your working branch, commit and push.
4. Open a Pull Request or Merge Request from the branch to `main`. This will cause `make ci` to run.
5. When CI passes and maintainers approve, merge the PR/MR into `main`. This will cause `make ci` and `make cd CONFIRM=true BRANCH_NAME=main` to run, pushing out images tagged with `:main` and `:${GIT_HASH}`
6. When a particular commit is ready to cut a release, **on main** add a git tag and push. This will cause `make release CONFIRM=true RELEASE_TAG=<applied git tag>` to run, pushing out an image tagged with `:${RELEASE_TAG}`

## Design

The Equinix Metal CCM follows the standard design principles for external cloud controller managers.

The main entrypoint command is in [main.go](./main.go), and provides fairly standard boilerplate for CCM.

1. import the Equinix Metal implementation as `import _ "github.com/kubernetes-sigs/cloud-provider-equinix-metal/metal"`:
   1. calls `init()`, which..
   1. registers the Equinix Metal provider
1. import the main app from [k8s.io/kubernetes/cmd/cloud-controller-manager/app](https://godoc.org/k8s.io/kubernetes/cmd/cloud-controller-manager/app)
1. `main()`:
   1. initialize the command
   1. call `command.Execute()`

The Equinix Metal-specific logic is in [github.com/kubernetes-sigs/cloud-provider-equinix-metal/metal](./metal/), which, as described before,
is imported into `main.go`. The blank `import _` is used solely for the side-effects, i.e. to cause the `init()`
function in [metal/cloud.go](./metal/cloud.go) to run before executing the command. This `init()`
registers the Equinix Metal cloud provider via `cloudprovider.RegisterCloudProvider`, where `cloudprovider` is
aliased to [k8s.io/cloud-provider](https://godoc.org/k8s.io/cloud-provider).

The `init()` step does the following registers the Equinix Metal provider with the name `"equinixmetal"` and an initializer
`func`, which:

1. retrieves the Equinix Metal project ID and Equinix Metal secret API token
1. creates a new [packngo.Client](https://godoc.org/github.com/packethost/packngo#Client)
1. creates a new [metal.cloud](./metal/cloud.go), passing it the client, so it can interact with the Equinix Metal API
1. returns the `metal.cloud`, as it complies with [cloudprovider.Interface](https://godoc.org/k8s.io/cloud-provider#Interface)

The `cloudprovider` now has a functioning `struct` that can perform the CCM functionality.

### File Structure

The primary entrypoint to the Equinix Metal provider is in [cloud.go](./metal/cloud.go). This file contains
the initialization functions, as above, as well as registers the Equinix Metal provider and sets it up.

The `cloud struct` itself is created via `newCloud()`, also in [cloud.go](./metal/cloud.go). This
initializes the `struct` with the `packngo.Client`, as well as `struct`s for each of the sub-components
that are supported: [LoadBalancer](https://pkg.go.dev/k8s.io/cloud-provider#LoadBalancer) and [InstancesV2](https://pkg.go.dev/k8s.io/cloud-provider#InstancesV2). The specific logic for each of these is contained in its own file:

- `InstancesV2`: [devices.go](./metal/devices.go)
- `LoadBalancer`: [loadbalancers.go](./metal/loadbalancers.go)

The other calls to `cloud` return `nil`, indicating they are not supported.

### Adding Functionality

To add support for additional elements of [cloudprovider.Interface](https://godoc.org/k8s.io/cloud-provider#Interface)

1. Modify `newCloud()` in [cloud.go](./metal/cloud.go) to populate the `cloud struct`, using `newX()`, e.g. to support `Routes()`, populate with `newRoutes`.
1. Create a file to support the new functionality, with the name of the file matching the functionality, e.g. for `Routes`, name the file `routes.go`.
1. In the new file:
   - Create a `type <functionality> struct` with at least the `client` and `project` properties, as well as any others required, e.g. `type routes struct`
   - Create a `func newX()` to create and populate the `struct`
   - Add necessary `func` with the correct receiver to implement the new functionality, per [cloudprovider.Interface](https://godoc.org/k8s.io/cloud-provider#Interface)
   - Create a test file for the new file, testing each functionality, e.g. `routes_test.go`. See the section below on testing.

### Testing

By definition, a cloud controller manager is intended to interface with a cloud provider. It would be difficult
and expensive to test the CCM against the true Equinix Metal API each time.

To simplify matters, the Equinix Metal CCM leverages [packet-api-server](https://github.com/packethost/packet-api-server)
to simulate a true Equinix Metal API. All of the tests leverage `testGetValidCloud()` from
[cloud_test.go](./metal/cloud_test.go) to:

- launch a simulated Equinix Metal API server that will terminate at the end of tests
- create an instance of `cloud` that is configured to connect to the simulated API server
- create an instance of [store.Memory](https://godoc.org/github.com/packethost/packet-api-server/pkg/store#Memory) so you can manipulate the "backend data" that the API server returns

To run any test:

1. `vc, backend := testGetValidCloud(t)`
1. use `backend` to input the seed data you want
1. call the function under test
1. check the results, either as the return from the function under test, or as the modified data in the `backend`

For examples, see [devices_test.go](./metal/devices_test.go) or [facilities_test.go](./metal/facilities_test.go).
