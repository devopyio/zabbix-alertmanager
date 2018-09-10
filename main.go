package main

import (
	"os"

	provisioner "github.com/devopyio/zabsnd/zabbixprovisioner"
	"github.com/devopyio/zabsnd/zabbixsender"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app   = kingpin.New("zabsnd", "A zabbix and prometheus integration.")
	debug = app.Flag("debug", "Enable debug mode.").Bool()

	send         = app.Command("send", "Start zabbix sender.")
	senderAddr   = send.Arg("addr", "Server address which will receive alerts from alertmanager.").Required().String()
	senderConfig = send.Arg("sconfig", "Zabbix sender config file path.").Required().String()

	prov       = app.Command("prov", "Start zabbix provisioner.")
	provConfig = prov.Arg("pconfig", "Zabbix provisioner config file path.").Required().String()
	provAlerts = prov.Arg("alerts", "Path to the prometheus alerts file.").Required().String()
)

func main() {
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {

	case send.FullCommand():
		zabbixsender.Run(*senderAddr, *senderConfig)
	case prov.FullCommand():
		provisioner.Run(*provConfig, *provAlerts)
	}
}
