#!/bin/bash
set -e
set -x

echo "Starting Promster..."
promster \
    --config-file=/prometheus.yml \
    --loglevel=$LOG_LEVEL \
    --etcd-url=$ETCD_URL \
    --scrape-etcd-path=$SCRAPE_ETCD_PATH

