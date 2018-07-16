FROM golang:1.10.1-alpine3.7 as builder

# Add build tools
RUN apk update && \
    apk add --no-cache git gcc musl-dev mercurial && \
    wget -O dep-install.sh https://raw.githubusercontent.com/golang/dep/master/install.sh && \
    sh dep-install.sh

ENV SRC_DIR=/go/src/github.com/packethost/packet-ccm/

WORKDIR /bin

# Dep ensure before adding rest of source so we can cache the resulting
# vendor dir
COPY Gopkg.toml Gopkg.lock $SRC_DIR

RUN cd $SRC_DIR && \
        dep ensure -vendor-only

# Add the source code:
COPY . $SRC_DIR

# Build it:
RUN cd $SRC_DIR && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build \
        -o packet-cloud-controller-manager ./ && \
    cp packet-cloud-controller-manager /bin/

# Create Docker image of just the binary
FROM scratch as runner
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /bin/packet-cloud-controller-manager .

CMD ["./packet-cloud-controller-manager"]
