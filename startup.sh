#!/bin/bash
set -e
set -x

echo "Starting Promster..."
promster \
    --loglevel=$LOG_LEVEL \
    --scheme=$SCHEME \
    --tls-insecure=$TLS_INSECURE \
    --evaluation-interval=$EVALUATION_INTERVAL \
    --scrape-interval=$SCRAPE_INTERVAL \
    --scrape-timeout=$SCRAPE_TIMEOUT \
    --scrape-paths=$SCRAPE_PATHS \
    --scrape-match=$SCRAPE_MATCH_REGEX \
    --scrape-shard-enable=$SCRAPE_SHARD_ENABLE \
    --scrape-etcd-url=$SCRAPE_ETCD_URL \
    --scrape-etcd-path=$SCRAPE_ETCD_PATH \
    --registry-etcd-url=$REGISTRY_ETCD_URL \
    --registry-etcd-base=$REGISTRY_ETCD_BASE \
    --registry-service-name=$REGISTRY_SERVICE \
    --registry-node-ttl=$REGISTRY_TTL \
    --metrics-relabel-file=$METRICS_RELABEL&

echo "Starting Prometheus..."
prometheus --config.file=/prometheus.yml --web.enable-lifecycle --storage.tsdb.retention.time=$RETENTION_TIME



