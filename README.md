## Basic configuration
Original article:

[Setting Up Zabbix Alertmanager integration](http://devopy.io/setting-up-zabbix-alertmanager-integration/)
Configuration for hosts are incorrect in the article. Working one is located [here](./zabbixprovisioner/config.yaml)

## Deployment

[grafana.json](./grafana.json) for Grafana dashboard.

[alerts.yaml](./alerts.yaml) for Prometheus alerts.

## General Info

Project consists of 2 components [send](#1-zal-send) and [prov](#2-zal-prov):

## 1. zal send

`zal send` command, which listens for Alert requests from Alertmanager and sends them to Zabbix.

Run `zal send --help` to see possible options. Consult [Setting Up Zabbix Alertmanager integration](http://devopy.io/setting-up-zabbix-alertmanager-integration/) for step by step tutorial.

## 2. zal prov

`zal prov` command, which reads Prometheus Alerting rules and converts them into Zabbix Triggers.

Run the `zal prov --help` to get the instructions.
 
## Usage

```
usage: zal [<flags>] <command> [<args> ...]

Zabbix and Prometheus integration.

Flags:
  -h, --help             Show context-sensitive help (also try --help-long and --help-man).
      --version          Show application version.
      --log.level=info   Log level.
      --log.format=text  Log format.

Commands:
  help [<command>...]
    Show help.

  send --zabbix-addr=ZABBIX-ADDR [<flags>]
    Listens for Alert requests from Alertmanager and sends them to Zabbix.

  prov --config-path=CONFIG-PATH --user=USER --password=PASSWORD [<flags>]
    Reads Prometheus Alerting rules and converts them into Zabbix Triggers.
```

## Zal send

```
usage: zal send --zabbix-addr=ZABBIX-ADDR [<flags>]

Listens for Alert requests from Alertmanager and sends them to Zabbix.

Flags:
  -h, --help                     Show context-sensitive help (also try --help-long and --help-man).
      --version                  Show application version.
      --log.level=info           Log level.
      --log.format=text          Log format.
      --addr="0.0.0.0:9095"      Server address which will receive alerts from alertmanager.
      --zabbix-addr=ZABBIX-ADDR  Zabbix address.
      --hosts-path=HOSTS-PATH    Path to resolver to host mapping file.
      --key-prefix="prometheus"  Prefix to add to the trapper item key
      --default-host="prometheus"
                                 default host to send alerts to

```

## Zal prov
```
usage: zal prov --config-path=CONFIG-PATH --user=USER --password=PASSWORD [<flags>]

Reads Prometheus Alerting rules and converts them into Zabbix Triggers.

Flags:
  -h, --help                     Show context-sensitive help (also try --help-long and --help-man).
      --version                  Show application version.
      --log.level=info           Log level.
      --log.format=text          Log format.
      --config-path=CONFIG-PATH  Path to provisioner hosts config file.
      --user=USER                Zabbix json rpc user.
      --password=PASSWORD        Zabbix json rpc password.
      --url="http://127.0.0.1/zabbix/api_jsonrpc.php"
                                 Zabbix json rpc url.
      --key-prefix="prometheus"  Prefix to add to the trapper item key.
      --prometheus-url=""        Prometheus URL.
```

Example of the alert's configuration is [here](./alerts.yaml)


Original repository: https://github.com/devopyio/zabbix-alertmanager