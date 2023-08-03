# Contributing

Thanks for your interest in improving this project! Before we get technical,
make sure you have reviewed the [code of conduct](code-of-conduct.md),
[Developer Certificate of Origin](https://developercertificate.org/), and [OWNERS](OWNERS.md) files. Code will
be licensed according to [LICENSE](LICENSE).

## Pull Requests

When creating a pull request, please refer to an open issue. If there is no
issue open for the pull request you are creating, please create one. Frequently,
pull requests may be merged or closed while the underlying issue being addressed
is not fully addressed. Issues are a place to discuss the problem in need of a
solution. Pull requests are a place to discuss an implementation of one
particular answer to that problem. A pull request may not address all (or any)
of the problems expressed in the issue, so it is important to track these
separately.

## Code Quality

### Documentation

All public functions and variables should include at least a short description
of the functionality they provide. Comments should be formatted according to
<https://golang.org/doc/effective_go.html#commentary>.

Documentation at <https://godoc.org/github.com/kubernetes-sigs/cloud-provider-equinix-metal> will be
generated from these comments.

Although the Equinix Metal CCM is intended more as a standalone runtime container than
a library, following the generally accepted golang standards will make it
easier to maintain this project, and easier for others to contribute.

### Linters

golangci-lint is used to verify that the style of the code remains consistent.

`make lint` can be used to verify style before creating a pull request.

Before committing, it's a good idea to run `goimports -w .`.
([goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports?tab=doc))

## Building and Testing

The [Makefile](./Makefile) contains the targets to build, lint, and test:

```sh
make build lint test
```

These normally will be run using your locally installed golang tools. If you do not have them
installed, or do not want to use local ones for any other reason, you can run it in a docker
image by setting the var `DOCKERBUILD=true`:

```sh
make build lint test DOCKERBUILD=true
```

If you want to see HTTP requests, set the `PACKNGO_DEBUG` env var to non-empty
string, for example:

```sh
PACKNGO_DEBUG=1 make test
```

In addition, the [Makefile](./Makefile) contains targets to build docker images.

`make image` builds the image locally for your local OS and architecture.
It will be named and tagged `equinix/cloud-provider-equinix-metal:latest-${ARCH}`.

You can override any of the above as follows:

- `BUILD_IMAGE`: name of the image, instead of the default `equinix/cloud-provider-equinix-metal`
- `BUILD_TAG`: base tag for the image, instead of the default `latest`
- `ARCH`: architecture to build for, and extension to tag
- `OS`: OS to build for, not included in the tag, defaults to `linux`
- `TAGGED_ARCH_IMAGE`: to replace the entire tag, defaults to `$(BUILD_IMAGE):$(BUILD_TAG)-$(ARCH)`

### Automation (CI/CD)

All CI/CD is performed via github actions, see the files in [.github/workflows/](./.github/workflows).

It is possible to test the github actions using your own fork of this repository, just make sure
you have github actions support enabled in your repository settings.

If you want to test publishing container images to quay.io, you will need to set the following secrets:

- QUAY_ORG
- QUAY_USERNAME
- QUAY_PASSWORD

If you want to test publishing container images to dockerhub, you will need to set the following secrets:

- DOCKER_ORG
- DOCKER_USERNAME
- DOCKER_PASSWORD

## Running Locally

You can run the CCM locally on your laptop or VM, i.e. not in the cluster. This _dramatically_ speeds up development. To do so:

1. Deploy everything except for the `Deployment` and, optionally, the `Secret`
1. Build it for your local platform `make build`
1. Set the environment variable `CCM_SECRET` to a file with the secret contents as a json, i.e. the content of the secret's `stringData`, e.g. `CCM_SECRET=ccm-secret.yaml`
1. Set the environment variable `KUBECONFIG` to a kubeconfig file with sufficient access to the cluster, e.g. `KUBECONFIG=mykubeconfig`
1. Set the environment variable `METAL_METRO_NAME` to the correct metro where the cluster is running, e.g. `METAL_METRO_NAME=ny` _OR_ set the environment variable `METAL_FACILITY_NAME` to the correct facility where the cluster is running, e.g. `METAL_FACILITY_NAME=ewr1`
1. If you want to run the loadbalancer, and it is not yet deployed, run `kubectl apply -f deploy/loadbalancer.yaml`
1. Enable the loadbalancer by setting the environment variable `METAL_LOAD_BALANCER=metallb://`
1. If you want to use a managed Elastic IP for the control plane, create one using the Equinix Metal API or Web UI, tag it uniquely, and set the environment variable `METAL_EIP_TAG=<tag>`
1. Run the command, e.g.:

```
METAL_METRO_NAME=${METAL_METRO_NAME} METAL_LOAD_BALANCER=metallb:// dist/bin/cloud-provider-equinix-metal-darwin-amd64 --cloud-provider=equinixmetal --leader-elect=false --authentication-skip-lookup=true --cloud-config=$CCM_SECRET --kubeconfig=$KUBECONFIG
```

For lots of extra debugging, add `--v=2` or even higher levels, e.g. `--v=5`.
