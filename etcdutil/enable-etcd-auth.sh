#! /bin/sh

ARGS="-e ETCDCTL_API=3 --network etcdutil_default -it quay.io/coreos/etcd:v3.2 /usr/local/bin/etcdctl --endpoints=https://etcd:2379 --insecure-transport=false --insecure-skip-tls-verify=true"

# Create the root user and enable auth
docker run $ARGS user add root:rootpw
docker run $ARGS auth enable

# Validate auth works
docker run $ARGS --user=root:rootpw user list
