FROM alpine:3.15 as certs

RUN apk --update add ca-certificates

# builder
FROM golang:1.17-alpine3.15 as build

WORKDIR /go/src/app
RUN apk --update add make git

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN make build

# Create Docker image of just the binary
FROM scratch as runner

ARG BINARY=cloud-provider-equinix-metal
ARG TARGETARCH
ARG OS=linux

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /go/src/app/dist/bin/${BINARY}-${OS}-${TARGETARCH} ${BINARY}

# because you cannot use ARG or ENV in CMD when in [] mode, and with "FROM scratch", we have no shell
ENTRYPOINT ["./cloud-provider-equinix-metal"]
