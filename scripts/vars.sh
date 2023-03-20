k8s_version="v1.23.4"
submrepo="quay.io/submariner"
submver="$(go list -m github.com/submariner-io/submariner | cut -d\  -f2 | cut -c2-)"
