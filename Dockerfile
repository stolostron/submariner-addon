FROM registry.ci.openshift.org/stolostron/builder:go1.18-linux AS builder
WORKDIR /go/src/github.com/stolostron/submariner-addon
COPY . .
ENV GO_PACKAGE github.com/stolostron/submariner-addon
RUN make build --warn-undefined-variables

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
COPY --from=builder /go/src/github.com/stolostron/submariner-addon/submariner /
RUN microdnf update && microdnf clean all
