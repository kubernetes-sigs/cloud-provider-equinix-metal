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


