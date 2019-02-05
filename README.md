![Build Status](https://travis-ci.com/devopyio/zabbix-alertmanager.svg?branch=master)

# zabbix-alertmanager

Fully automated [Zabbix](https://www.zabbix.com/) and [Prometheus Alertmanager](https://prometheus.io/docs/alerting/alertmanager/) integration

 Project consists of 2 parts:
## 1. zal send
Sender catches the alerts from [Prometheus Alertmanager](https://prometheus.io/docs/alerting/alertmanager/) and pushes them to the [Zabbix](https://www.zabbix.com/) server by using trapper items.
 ## 2. zal prov
The provisioner will load the current [Prometheus Alerting rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) from a directory and will create hostgroup/host/application/items/triggers in [Zabbix](https://www.zabbix.com/) if needed. 
 # Howto
First of all you need to launch the zal prov which will create all the required items on [Zabbix](https://www.zabbix.com/) server and you will be able to
automatically push alerts with zal send.
 Check the config file for the possible parameters: 
```
- name: example_name
  hostGroups:
    - example_group
  tag: example_tag
  deploymentStatus: <number>
  # itemDefault* below, defines item values when not specified in a rule
  itemDefaultApplication: default
  itemDefaultHistory: 5d
  itemDefaultTrends: 5d
  itemDefaultTrapperHosts: # Hosts permitted to send data (your webhook external CIDR, default is from everywhere)
  ```
  Run the zal prov --help and you will get the instructions.
  
