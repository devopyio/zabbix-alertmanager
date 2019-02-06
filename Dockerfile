FROM golang:1-alpine AS build

RUN apk update && apk add make git gcc musl-dev

ADD . /go/src/github.com/devopyio/zabbix-alertmanager

WORKDIR /go/src/github.com/devopyio/zabbix-alertmanager

RUN make go-deps
RUN make go-build

RUN mv zal /zal

FROM alpine:latest

RUN apk add --no-cache ca-certificates && mkdir /app
RUN adduser zal -s /bin/false -D zal

COPY --from=build /zal /usr/bin

USER zal
ENTRYPOINT ["/usr/bin/zal"]

ENTRYPOINT exec /app/${APP}
