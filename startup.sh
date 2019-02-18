#!/bin/bash
set -e
set -x

echo "Starting Promster..."
promster \
    --loglevel=$LOG_LEVEL \
    --etcd-url-scrape=$SCRAPE_ETCD_URL \
    --scrape-etcd-path=$SCRAPE_ETCD_PATH \
    --etcd-url=$REGISTRY_ETCD_URL \
    --etcd-base=$REGISTRY_ETCD_BASE \
    --etcd-service-name=$REGISTRY_SERVICE \
    --etcd-node-ttl=$REGISTRY_TTL&

echo "Starting Prometheus..."
prometheus --config.file=/prometheus.yml --web.enable-lifecycle





