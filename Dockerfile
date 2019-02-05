FROM alpine:latest

RUN adduser zal -s /bin/false -D zal

COPY zal /usr/bin

USER zal

ENTRYPOINT ["/usr/bin/zal"]
