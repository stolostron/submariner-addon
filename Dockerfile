FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:v1.24.6-202510150934.g4284440.el9 AS builder
WORKDIR /go/src/github.com/stolostron/submariner-addon
COPY . .
ENV GO_PACKAGE github.com/stolostron/submariner-addon
RUN make build --warn-undefined-variables

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
COPY --from=builder /go/src/github.com/stolostron/submariner-addon/submariner /
RUN microdnf update -y && microdnf clean all
