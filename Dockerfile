FROM golang:1.10 AS BUILD

#doing dependency build separated from source build optimizes time for developer, but is not required
#install external dependencies first
ADD /main.dep $GOPATH/src/promster/main.go
RUN go get -v promster

#now build source code
ADD promster $GOPATH/src/promster
# RUN go get -v promster
#embed C libs
RUN CGO_ENABLED=0 GOOS=linux go get promster

#RUN go test -v promster


FROM prom/prometheus:v2.4.0

ENV LOG_LEVEL 'info'

ENV SCRAPE_ETCD_URL ""
ENV SCRAPE_ETCD_PATH ""
ENV SCRAPE_PATHS /metrics
ENV SCRAPE_MATCH_REGEX ""
ENV SCRAPE_INTERVAL 30s
ENV SCRAPE_TIMEOUT 30s

ENV EVALUATION_INTERVAL 30s
ENV RETENTION_TIME 1d

ENV REGISTRY_ETCD_URL ""
ENV REGISTRY_ETCD_BASE "/registry"
ENV REGISTRY_SERVICE ""
ENV REGISTRY_TTL "60"

# ENV RECORD_RULE_1_NAME ""
# ENV RECORD_RULE_1_EXPR ""

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

