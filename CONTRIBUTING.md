# Contributing

Thanks for your interest in improving this project! Before we get technical,
make sure you have reviewed the [code of conduct](code-of-conduct.md),
[Developer Certificate of Origin](DCO), and [OWNERS](OWNERS.md) files. Code will
be licensed according to [LICENSE](LICENSE).

## Pull Requests

When creating a pull request, please refer to an open issue. If there is no
issue open for the pull request you are creating, please create one. Frequently,
pull requests may be merged or closed while the underlying issue being addressed
is not fully addressed. Issues are a place to discuss the problem in need of a
solution. Pull requests are a place to discuss an implementation of one
particular answer to that problem.  A pull request may not address all (or any)
of the problems expressed in the issue, so it is important to track these
separately.

## Code Quality

### Documentation

All public functions and variables should include at least a short description
of the functionality they provide. Comments should be formatted according to
<https://golang.org/doc/effective_go.html#commentary>.

Documentation at <https://godoc.org/github.com/equinix/cloud-provider-equinix-metal> will be
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

The [Makefile](./Makefile) contains the targets to build, lint, vet and test:

```sh
make build lint vet test
```

These normally will be run using your locally installed golang tools. If you do not have them
installed, or do not want to use local ones for any other reason, you can run it in a docker
image by setting the var `DOCKERBUILD=true`:

```sh
make build lint vet test DOCKERBUILD=true
```

If you want to see HTTP requests, set the `PACKNGO_DEBUG` env var to non-empty
string, for example:

```sh
PACKNGO_DEBUG=1 make test
```

### Automation (CI/CD)

All CI/CD is performed via github actions, see the files in [.github/workflows/](./.github/workflows).
