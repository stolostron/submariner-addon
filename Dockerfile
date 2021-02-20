FROM docker.io/openshift/origin-release:golang-1.15 AS builder
WORKDIR /go/src/github.com/open-cluster-management/submariner-addon
COPY . .
ENV GO_PACKAGE github.com/open-cluster-management/submariner-addon
RUN make build --warn-undefined-variables

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
COPY --from=builder /go/src/github.com/open-cluster-management/submariner-addon/submariner /
RUN microdnf update && microdnf clean all
