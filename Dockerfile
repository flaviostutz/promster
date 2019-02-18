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
ENV SCRAPE_LABELS ""
ENV SCRAPE_ENDPOINTS /metrics
ENV SCRAPE_INTERVAL 15s

ENV REGISTRY_ETCD_URL ""
ENV REGISTRY_ETCD_BASE "/registry"
ENV REGISTRY_SERVICE ""
ENV REGISTRY_TTL "60"

ENV RECORD_RULE_1_NAME ""
ENV RECORD_RULE_1_EXPR ""

USER root

COPY --from=BUILD /go/bin/* /bin/
ADD startup.sh /
ADD prometheus.yml /
ADD servers.json /

ENTRYPOINT [ "/bin/sh" ]
CMD [ "-C", "/startup.sh" ]

