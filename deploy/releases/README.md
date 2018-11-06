# Kubernetes CCM for Packet

`packet-ccm` is the Kubernetes CCM implementation for Packet. Read more about the CCM [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

## Requirements

At the current state of Kubernetes, running the CCM requires a few things.
Please read through the requirements carefully as they are critical to running the CCM on a Kubernetes cluster.

### Version
Recommended versions of Packet CCM based on your Kubernetes version:
* Packet CCM version v0.0.4 supports Kubernetes version >=v1.10

### --cloud-provider=external
All `kubelet`s in your cluster **MUST** set the flag `--cloud-provider=external`. `kube-apiserver` and `kube-controller-manager` must **NOT** set the flag `--cloud-provider` which will default them to use no cloud provider natively.

**WARNING**: setting the kubelet flag `--cloud-provider=external` will taint all nodes in a cluster with `node.cloudprovider.kubernetes.io/uninitialized`.
The CCM will then untaint those nodes when it initializes them.
Any pod that does not tolerate that taint will be unscheduled until the CCM is running.

### Kubernetes node names must match the device name
By default, the kubelet will name nodes based on the node's hostname.
Packet's device hostnames are set based on the name of the device.
It is important that the Kubernetes node name matches the device name.

## Implementation Details
Currently `packet-ccm` implements:

* [nodecontroller](https://kubernetes.io/docs/concepts/architecture/cloud-controller/#node-controller) - updates nodes with cloud provider specific labels and addresses

## Deployment

### Token
To run `packet-ccm`, you need your Packet api key and project ID that your cluster is running in.
If you are already logged in, you can create one by clicking on your profile in the upper right then "API keys".
To get project ID click into the project that your cluster is under and select "project settings" from the header.
Under General you will see "Project ID". Once you have this information you will be able to fill in the config needed for the CCM.

#### Create config
Copy [v0.0.4/secret.yaml](v0.0.4/secret.yaml) to releases/packet-cloud-config.yaml:
```bash
cp v0.0.4/secret.yaml ./packet-cloud-config.yaml
```

Replace the placeholder in the copy with your token. When you're done, the packet-cloud-config.yaml should look something like this:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: packet-cloud-config
  namespace: kube-system
stringData:
  apiKey: "abc123abc123abc123"
  projectID: "abc123abc123abc123"
```

Then run:
```bash
kubectl apply -f packet-cloud-config.yaml`
```

You can confirm that the secret was created in the `kube-system` with the following:
```bash
$ kubectl -n kube-system get secrets packet-cloud-config
NAME                  TYPE                                  DATA      AGE
packet-cloud-config   Opaque                                1         2m
```

### CCM
You can apply the rest of the CCM by running:

```bash
kubectl apply -f v0.0.4/deployment.yaml
```
