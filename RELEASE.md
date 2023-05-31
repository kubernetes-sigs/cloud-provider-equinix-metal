# Release

This document describes how to release the Equinix Metal Cloud Provider.

This is _not_ intended for regular users.

This is normally performed by our CI system. However, there are important steps to take first.

## How to Cut a Release

In order to cut a release, you must:

1. Commit the changes.
1. Push out your branch, open a PR and merge the changes
1. Wait for the Continuous Integration github action to finish running
1. Tag the release with `git tag -a vX.Y.z -m "Message"
1. Push out the tag

## How A Release Happens

* GitHub Actions detects a new tag has been pushed
* CI builds docker images for each supported architecture as well as a multi-arch manifest, and tags it with the semver tag of the release, e.g. `v0.4.0`
* CI creates the release in `out/release`, the equivalent of `make release`
* CI copies the artifacts in `out/release/*` to the github releases