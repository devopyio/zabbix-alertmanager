package main

import (
	"flag"
	"os"

	"github.com/devopyio/zabsnd/alertmanager-zabbix-provisioner/provisioner"
	log "github.com/sirupsen/logrus"
)

func main() {

	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{DisableColors: true})

	configFileName := flag.String("config", "./config.yaml", "path to the configuration file")
	alertsFileName := flag.String("alerts", "foo", "path to the alerts file")
	flag.Parse()

	log.Infof("loading configuration at '%s'", *configFileName)
	cfg, err := provisioner.LoadFromFile(*configFileName)
	if err != nil {
		log.Fatal(err)
	}

	provisioner.New(cfg).Start(*alertsFileName)
}
