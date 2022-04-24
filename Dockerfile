# syntax=docker/dockerfile:1.1-experimental

# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the manager binary
ARG GOVER=1.17.8
FROM --platform=$BUILDPLATFORM golang:${GOVER} as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM

WORKDIR /workspace

# Run this with docker build --build_arg $(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
ENV GOPROXY=$goproxy

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

# Build
ARG TARGETARCH
ARG LDFLAGS
ARG BINARY=cloud-provider-equinix-metal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -a -ldflags "${LDFLAGS} -extldflags '-static'" \
    -o "${BINARY}" .

# because you cannot use ARG or ENV in CMD when in [] mode

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
ARG BINARY=cloud-provider-equinix-metal
COPY --from=builder /workspace/${BINARY} ./cloud-provider-equinix-metal
USER nonroot:nonroot
ENTRYPOINT ["./cloud-provider-equinix-metal"]
