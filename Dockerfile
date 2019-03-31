FROM golang:1-alpine AS build
LABEL Maintainer="info@devopy.io" Description="Fully automated Zabbix and Prometheus Alertmanager integration"
RUN apk update && apk add make git gcc musl-dev

ADD . /go/src/github.com/devopyio/zabbix-alertmanager

WORKDIR /go/src/github.com/devopyio/zabbix-alertmanager

ENV GO111MODULE on
RUN make build
RUN mv zal /zal

FROM alpine:latest

RUN apk add --no-cache ca-certificates && mkdir /app
RUN adduser zal -u 1001 -g 1001 -s /bin/false -D zal

COPY --from=build /zal /usr/bin
RUN chown -R zal /usr/bin/zal

USER zal
ENTRYPOINT ["/usr/bin/zal"]
