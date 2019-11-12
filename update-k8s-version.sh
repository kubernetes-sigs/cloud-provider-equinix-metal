#!/usr/bin/env bash
set -e

KUBERNETES_VERSION=${KUBERNETES_VERSION:?}

deps=()

# lines that start with replace
linesR=$(awk '/^\s+replace k8s.io./ {print $2}' go.mod)
# lines that are inside a replace( ... ) block
linesE=$(awk '/^\s+k8s.io.*=/ {print $1}' go.mod)

for depname in ${linesR} ${linesE}
do
  case "$depname" in
  "k8s.io/kubernetes")
    deps+=("-replace $depname=$depname@v$KUBERNETES_VERSION")
    ;;
  "k8s.io/kube-openapi")
    # kube-openapi is not properly semvered
    ;;
  *)
    deps+=("-replace $depname=$depname@kubernetes-$KUBERNETES_VERSION")
    ;;
  esac 
done

unset GOROOT GOPATH
export GO111MODULE=on

set -x
# shellcheck disable=SC2086
go mod edit ${deps[*]}
go mod tidy
go mod vendor
set +x
