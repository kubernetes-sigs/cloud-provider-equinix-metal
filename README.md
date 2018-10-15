# packet-ccm
Cloud Controller Manager for Packet

## Building
To build the binary, run:

```
make build
```

It will deposit the binary for your local architecture as `dist/bin/packet-cloud-controller-manager-$(ARCH)`

By default `make build` builds the binary using your locally installed go toolchain. If you want to build it using docker, do:

```
make build DOCKERBUILD=true
```

## Docker Image
To build a docker image, run:

```
make image
```

The image will be tagged with `:latest`

## CI/CD/Release pipeline
The CI/CD/Release pipeline is run via the following steps:

* `make ci`: builds the binary, runs all tests, builds the OCI image
* `make cd`: takes the image from the prior stage, tags it with the name of the branch and git hash from the commit, and pushes to the docker registry
* `make release`: takes the image from the `ci` stage, tags it with the git tag, and pushes to the docker registry

The assumptions about workflow are as follows:

* `make ci` can be run anywhere. The built binaries and OCI image will be named and tagged as per `make build` and `make image` above.
* `make cd` should be run only on a merge into `master`. It generally will be run only in a CI system, e.g. travis or drone. It requires passing both `CONFIRM=true` to tell it that it is ok to push, and `BRANCH_NAME=${BRANCH_NAME}` to tell it what tag should be used in addition to the git hash. For example, to push out the current commit as master: `make cd CONFIRM=true BRANCH_NAME=master` 
* `make release` should be run only on applying a tag to `master`, although it can run elsewhere. It generally will be run only in a CI system. It requires passing both `CONFIRM=true` to tell it that it is ok to push, and `RELEASE_TAG=${RELEASE_TAG}` to tell it what tag this release should be. For example, to push out a tagged version `v1.2.3` on the current commit: `make release CONFIRM=true RELEASE_TAG=v1.2.3`. 

For both `make cd` and `make release`, if you wish to push out a _different_ commit, then check that one out first.

The flow to make changes normally should be:

1. `master` is untouched, a protected branch.
2. In your local copy, create a new working branch.
3. Make your changes in your working branch, commit and push.
4. Open a Pull Request or Merge Request from the branch to `master`. This will cause `make ci` to run.
5. When CI passes and maintainers approve, merge the PR/MR into `master`. This will cause `make ci` and `make cd CONFIRM=true BRANCH_NAME=master` to run, pushing out images tagged with `:master` and `:${GIT_HASH}`
6. When a particular commit is ready to cut a release, **on master** add a git tag and push. This will cause `make release CONFIRM=true RELEASE_TAG=<applied git tag>` to run, pushing out an image tagged with `:${RELEASE_TAG}`


