# zabbix-alertmanager

Project consists of 2 parts:
## 1. zabbix-sender
Sender catches the alerts from Prometheus alertmanager and pushes them to the Zabbix server by using trapper items.

## 2. zabbix-provisioner
The provisioner will load the current configured rules from a directory and will create hostgroup/host/application/items/triggers accordingly, 
considering the state(Old/Equal/New/Update).

# Howto
First of all you need to launch the zabbix-provisioner which will create all the required tools on Zabbix server and you will be able to
automatically push alerts with zabbix-sender.

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
  Run the main.go file and you will get the instructions.
  
