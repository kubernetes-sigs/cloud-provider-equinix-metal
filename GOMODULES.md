# Go Modules

Getting go modules to work can be painful; with kubernetes and its many packages and interdependencies, it can be downright
brutal.

The section below is the base path that was used to move this to go modules.

1. Create `go.mod` with the following contents

```go
module github.com/packethost/packet-ccm

go 1.12

require (
	github.com/packethost/packngo v0.1.0
	k8s.io/api kubernetes-1.14.1
	k8s.io/apimachinery kubernetes-1.14.1
	k8s.io/apiserver kubernetes-1.14.1
	k8s.io/apiextensions-apiserver kubernetes-1.14.1
	k8s.io/client-go kubernetes-1.14.1
	k8s.io/cloud-provider kubernetes-1.14.1
	k8s.io/component-base kubernetes-1.14.1
	k8s.io/kubernetes v1.14.1
)

replace (
	k8s.io/api => k8s.io/api kubernetes-1.14.1
	k8s.io/apimachinery => k8s.io/apimachinery kubernetes-1.14.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver kubernetes-1.14.1
	k8s.io/apiserver => k8s.io/apiserver kubernetes-1.14.1
	k8s.io/cli-runtime => k8s.io/cli-runtime kubernetes-1.14.1
	k8s.io/client-go => k8s.io/client-go kubernetes-1.14.1
	k8s.io/cloud-provider => k8s.io/cloud-provider kubernetes-1.14.1
	k8s.io/cli-runtime => k8s.io/cli-runtime kubernetes-1.14.1
	k8s.io/component-base => k8s.io/component-base kubernetes-1.14.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager kubernetes-1.14.1
	k8s.io/kubernetes => k8s.io/kubernetes v1.14.1
)
```

1. Look in github to see specific hashes of dependencies for the various k8s.io/ elements. Go to the individual projects, select the tag that matches the above release, and look in its dependency trees (`Godeps`, `Gopkg.toml`, etc.) to see what versions of packages it uses.
1. Run `GO111MODULE=on go get <dependency>@<hash>`

The specific packages that had dependencies that we discovered are:

* `k8s.io/apiserver`

The dependencies we installed were:

```sh
go get sigs.k8s.io/structured-merge-diff@e85c7b244fd2cc57bb829d73a061f93a441e63ce
go get k8s.io/kube-openapi@b3a7cee44a305be0a69e1b9ac03018307287e1b0
``` 

## Upgrading

When upgrading to a new version of kubernetes:

1. Generate a new go.mod to reference the new version
1. Look up the above dependencies
1. Run `make build`
1. Loop through for any errors on mismatched versions
1. PRAY

