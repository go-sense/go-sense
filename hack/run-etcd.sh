#!/usr/bin/env bash

export REL_ETCD_DATA_DIR="$(dirname "${0}")/etcd-data/"
export ABS_ETCD_DATA_DIR=$(realpath ${REL_ETCD_DATA_DIR})

mkdir -p ${ABS_ETCD_DATA_DIR}

docker run \
  -p 2379:2379 \
  -v ${ABS_ETCD_DATA_DIR}:/etcd-data \
  quay.io/coreos/etcd:v3.3.9 \
  /usr/local/bin/etcd \
  --data-dir=/etcd-data --name node1 \
  --initial-advertise-peer-urls http://127.0.0.1:2380 --listen-peer-urls http://0.0.0.0:2380 \
  --advertise-client-urls http://127.0.0.1:2379 --listen-client-urls http://0.0.0.0:2379 \
  --initial-cluster node1=http://127.0.0.1:2380
