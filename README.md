![zabbix-alertmanager](http://devopy.io/wp-content/uploads/2019/02/zal-200.png)

# zabbix-alertmanager

![Build Status](https://travis-ci.com/devopyio/zabbix-alertmanager.svg?branch=master)
![Go Report Card](https://goreportcard.com/badge/github.com/devopyio/zabbix-alertmanager)
[![Docker Repository on Quay](https://quay.io/repository/devopyio/zabbix-alertmanager/status "Docker Repository on Quay")](https://quay.io/repository/devopyio/zabbix-alertmanager)

Fully automated [Zabbix](https://www.zabbix.com/) and [Prometheus Alertmanager](https://prometheus.io/docs/alerting/alertmanager/) integration. 

## Tutorials

[Introducing ZAL - Zabbix Alertmanager Integration](https://devopy.io/zabbix-alertmanager-integration/)

[Setting Up Zabbix Alertmanager integration](http://devopy.io/setting-up-zabbix-alertmanager-integration/)

[Running Zabbix Alertmanager integration](http://devopy.io/)


Project consists of 2 components:

## 1. zal send

`zal send` command, which listens for Alert requests from Alertmanager and sends them to Zabbix.

Run `zal send --help` to see possible options. Consult [Setting Up Zabbix Alertmanager integration](http://devopy.io/setting-up-zabbix-alertmanager-integration/) for step by step tutorial.

## 2. zal prov

`zal prov` command, which reads Prometheus Alerting rules and converts them into Zabbix Triggers.

Run the `zal prov --help` to get the instructions.
  
