#!/bin/sh

# Update all our Submariner dependencies to the version given
# as an argument

if [ -z "$1" ]; then
    echo Please specify the version of Submariner, e.g.
    echo $0 v0.13.0
    exit 1
fi

for project in admiral cloud-prepare submariner submariner-operator; do
    go get github.com/submariner-io/${project}@$1
done

sed -i "s/version: .*$/version: $1/" pkg/hub/submarineragent/manifests/operator/submariner.io-submariners-cr.yaml
sed -i "s/submver=.*$/submver=${1#v}/" scripts/deploy.sh

go mod tidy
go mod vendor
