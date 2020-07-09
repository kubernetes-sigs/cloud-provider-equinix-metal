FROM alpine:3.8 as certs

RUN apk --update add ca-certificates

# Create Docker image of just the binary
FROM scratch as runner

ARG BINARY=packet-cloud-controller-manager
ARG ARCH
ARG OS=linux

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY dist/bin/${BINARY}-${OS}-${ARCH} ${BINARY}
COPY lb/manifests.yaml lb/manifests.yaml

# because you cannot use ARG or ENV in CMD when in [] mode, and with "FROM scratch", we have no shell
ENTRYPOINT ["./packet-cloud-controller-manager"]
