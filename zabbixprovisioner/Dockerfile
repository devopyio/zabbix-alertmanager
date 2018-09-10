FROM alpine:latest

RUN adduser provisioner -s /bin/false -D provisioner

RUN mkdir -p /etc/provisioner
COPY config.yaml /etc/provisioner

COPY alertmanager-zabbix-provisioner /usr/bin
RUN chmod +x /usr/bin/alertmanager-zabbix-provisioner

USER provisioner

ENTRYPOINT ["/usr/bin/alertmanager-zabbix-provisioner"]
CMD ["-config", "/etc/provisioner/config.yaml", "-alerts", "/etc/prometheus/alerts.yml"]
