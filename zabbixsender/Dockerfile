FROM alpine:latest

RUN adduser sender -s /bin/false -D sender

RUN mkdir -p /etc/sender
COPY config.yaml /etc/sender

COPY alertmanager-zabbix  /usr/bin
RUN chmod +x /usr/bin/alertmanager-zabbix

EXPOSE 8080
USER sender

ENTRYPOINT ["/usr/bin/alertmanager-zabbix"]