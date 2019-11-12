# go issues

**This is a temporary file! Do not commit this to any PR!**

There are two approaches I have taken to modifying go.mod to get k8s 1.15.0. Each is reflected in a different `go.mod` file:

* `go.mod.update`: this is the result of taking the existing `go.mod` based on `1.14.1` and running `KUBERNETES_VERSION=1.15.0 ./update-k8s-version.sh`
* `go.mod.manual`: this is the result of taking the pristine `go.mod` based on `1.14.1` that is embedded in the `GOMODULES.md` file and changing every reference to `1.14.1` to `1.15.0`.

Both of these fail, albeit for different reasons.

