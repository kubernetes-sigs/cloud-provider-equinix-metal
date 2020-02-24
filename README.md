# Kubernetes Cloud Controller Manager for Packet

`packet-ccm` is the Kubernetes CCM implementation for Packet. Read more about the CCM in [the official Kubernetes documentation](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

## Requirements

At the current state of Kubernetes, running the CCM requires a few things.
Please read through the requirements carefully as they are critical to running the CCM on a Kubernetes cluster.

### Version
Recommended versions of Packet CCM based on your Kubernetes version:
* Packet CCM version v0.0.4 supports Kubernetes version >=v1.10
* Packet CCM version v1.0.0 support Kubernetes version >=1.15.0

## Deployment

**TL;DR**

1. Set kubernetes binary arguments correctly
1. Get your Packet project and secret API token
1. Deploy your Packet project and secret API token to your cluster
1. Deploy the CCM

### Kubernetes Binary Arguments

Control plane binaries in your cluster must start with the correct flags:

* `kubelet`: All kubelets in your cluster **MUST** set the flag `--cloud-provider=external`. This must be done for _every_ kubelet. Note that [k3s](https://k3s.io) sets its own CCM by default. If you want to use the CCM with k3s, you must disable the k3s CCM and enable this one, as `--disable-cloud-controller --kubelet-arg cloud-provider=external`.
* `kube-apiserver` and `kube-controller-manager` must **NOT** set the flag `--cloud-provider`. They then will use no cloud provider natively, leaving room for the Packet CCM.

**WARNING**: setting the kubelet flag `--cloud-provider=external` will taint all nodes in a cluster with `node.cloudprovider.kubernetes.io/uninitialized`.
The CCM itself will untaint those nodes when it initializes them.
Any pod that does not tolerate that taint will be unscheduled until the CCM is running.

#### Kubernetes node names must match the device name

By default, the kubelet will name nodes based on the node's hostname.
Packet's device hostnames are set based on the name of the device.
It is important that the Kubernetes node name matches the device name.

### Get Packet Project ID and API Token

To run `packet-ccm`, you need your Packet project ID and secret API key ID that your cluster is running in.
If you are already logged into the [Packet portal](https://app.packet.net), you can create one by clicking on your
profile in the upper right then "API keys".
To get your project ID click into the project that your cluster is under and select "project settings" from the header.
Under General you will see "Project ID". Once you have this information you will be able to fill in the config needed for the CCM.

#### Deploy Project and API
Copy [deploy/template/secret.yaml](./deploy/template/secret.yaml) to someplace useful:

```bash
cp deploy/template/secret.yaml /tmp/secret.yaml
```

Replace the placeholder in the copy with your token. When you're done, the `yaml` should look something like this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: packet-cloud-config
  namespace: kube-system
stringData:
  cloud-sa.json: |
    {
    "apiKey": "abc123abc123abc123",
    "projectID": "abc123abc123abc123"
    }  
```

Then apply the file via `kubectl`, e.g.:

```bash
kubectl apply -f /tmp/secret.yaml`
```

You can confirm that the secret was created with the following:

````bash
$ kubectl -n kube-system get secrets packet-cloud-config
NAME                  TYPE                                  DATA      AGE
packet-cloud-config   Opaque                                1         2m
````

##

You can apply the rest of the CCM by using `kubectl` to apply `deploy/releases/<version>/deployment.yaml`, e.g:

```bash
kubectl apply -f deploy/releases/v1.0.0/deployment.yaml
```

### Logging

By default, ccm does minimal logging, relying on the supporting infrastructure from kubernetes. However, it does support
optional additional logging levels via the `--v=<level>` flag. In general:

* `--v=2`: log most function calls for devices and facilities, when relevant logging the returned values
* `--v=3`: log additional data when logging returned values, usually entire go structs
* `--v=5`: log every function call, including those called very frequently
