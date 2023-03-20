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

go mod tidy
go mod vendor
