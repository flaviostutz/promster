# promster
Prometheus with dynamic clustering and scrape sharding capabilities based on ETCD

# ENV parameters
ETCD_URL=http://etcd0:2379

SCRAPE_ETCD_PATH=/webservers
SCRAPE_LABELS=server=webservers,env=test
SCRAPE_ENDPOINTS=/metrics,/metrics2
SCRAPE_INTERVAL=15s

REGISTRATION_ETCD_PATH=/L1

RECORD_RULE_1_NAME=instance_path:requests:rate5m
RECORD_RULE_1_EXPR=rate(requests_total{job="myjob"}[5m])
RECORD_RULE_2_NAME=requests_total:sum
RECORD_RULE_2_EXPR=sum(requests_total)

