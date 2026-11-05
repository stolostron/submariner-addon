FROM registry.redhat.io/openshift/golang-builder:golang-builder-v1.24-rhel9 AS builder
WORKDIR /go/src/github.com/stolostron/submariner-addon
COPY . .
ENV GO_PACKAGE github.com/stolostron/submariner-addon
RUN make build --warn-undefined-variables

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
COPY --from=builder /go/src/github.com/stolostron/submariner-addon/submariner /
RUN microdnf update -y && microdnf clean all
