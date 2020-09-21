FROM golang:1.12.4-stretch AS BUILD

RUN mkdir /promster
WORKDIR /promster

ADD go.mod .
ADD go.sum .
RUN go mod download

#now build source code
ADD . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/promster


FROM prom/prometheus:v2.15.1

ENV LOG_LEVEL 'info'

ENV SCHEME http
ENV TLS_INSECURE false

ENV SCRAPE_ETCD_URL ""
ENV SCRAPE_ETCD_PATH ""
ENV SCRAPE_PATHS /metrics
ENV SCRAPE_MATCH_REGEX ""
ENV SCRAPE_SHARD_ENABLE true
ENV SCRAPE_INTERVAL 30s
ENV SCRAPE_TIMEOUT 30s

ENV EVALUATION_INTERVAL 30s
ENV RETENTION_TIME 1d

ENV REGISTRY_ETCD_URL ""
ENV REGISTRY_ETCD_BASE "/registry"
ENV REGISTRY_SERVICE ""
ENV REGISTRY_TTL "60"
ENV METRICS_RELABEL ""

# ENV RECORD_RULE_1_NAME ""
# ENV RECORD_RULE_1_EXPR ""
# ENV RECORD_RULE_1_LABELS ""

USER root

COPY --from=BUILD /go/bin/* /bin/
ADD startup.sh /
ADD prometheus.yml /
ADD prometheus.yml.tmpl /

RUN touch /rules.yml
RUN touch /servers.json

RUN chmod -R 777 /startup.sh
RUN chmod -R 777 /prometheus.yml
RUN chmod -R 777 /prometheus.yml.tmpl
RUN chmod -R 777 /rules.yml
RUN chmod -R 777 /servers.json

ENTRYPOINT [ "/bin/sh" ]
CMD [ "-C", "/startup.sh" ]

