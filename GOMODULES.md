# Go Modules

Getting go modules to work can be painful; with kubernetes and its many packages and interdependencies, it can be downright
brutal.

The section below is the base path that was used to move this to go modules.

1. Be sure to use go 1.13 or higher; it does a _much_ better job reporting dependency problems.
1. Create `go.mod` with the following contents

```go
module github.com/equinix/cloud-provider-equinix-metal

go 1.13

require (
        github.com/packethost/packngo v0.1.0
        k8s.io/kubernetes v1.15.0
)

replace (
        k8s.io/kube-aggregator => k8s.io/kube-aggregator kubernetes-1.15.0
        k8s.io/api => k8s.io/api kubernetes-1.15.0
        k8s.io/apimachinery => k8s.io/apimachinery kubernetes-1.15.0
        k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver kubernetes-1.15.0
        k8s.io/apiserver => k8s.io/apiserver kubernetes-1.15.0
        k8s.io/cli-runtime => k8s.io/cli-runtime kubernetes-1.15.0
        k8s.io/client-go => k8s.io/client-go kubernetes-1.15.0
        k8s.io/cloud-provider => k8s.io/cloud-provider kubernetes-1.15.0
        k8s.io/cli-runtime => k8s.io/cli-runtime kubernetes-1.15.0
        k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap kubernetes-1.15.0
        k8s.io/code-generator => k8s.io/code-generator kubernetes-1.15.0
        k8s.io/component-base => k8s.io/component-base kubernetes-1.15.0
        k8s.io/cri-api => k8s.io/cri-api kubernetes-1.15.0
        k8s.io/csi-translation-lib => k8s.io/csi-translation-lib kubernetes-1.15.0
        k8s.io/kube-controller-manager => k8s.io/kube-controller-manager kubernetes-1.15.0
        k8s.io/kube-proxy => k8s.io/kube-proxy kubernetes-1.15.0
        k8s.io/kube-scheduler => k8s.io/kube-scheduler kubernetes-1.15.0
        k8s.io/kubelet => k8s.io/kubelet kubernetes-1.15.0
        k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers kubernetes-1.15.0
        k8s.io/metrics => k8s.io/metrics kubernetes-1.15.0
        k8s.io/sample-apiserver => k8s.io/sample-apiserver kubernetes-1.15.0
        k8s.io/kubernetes => k8s.io/kubernetes v1.15.0
)
```

1. Run `go mod download`

## Upgrading

When upgrading to a new version of kubernetes:

1. Generate a new go.mod to reference the new version, changing anywhere in the above file from the current version to the new version
1. `rm go.sum`
1. Run `go mod download`
1. Loop through for any errors on mismatched versions, and add `replace` dependencies. These are almost entirely due to `k8s.io/kubernetes`
1. PRAY
