package provisioner

import (
	"os"

	"github.com/devopyio/zabsnd/zabbixprovisioner/provisioner"
	log "github.com/sirupsen/logrus"
)

func Run(configFileName string, alertsFileName string) {

	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{DisableColors: true})

	log.Infof("loading configuration at '%s'", configFileName)
	cfg, err := provisioner.LoadFromFile(configFileName)
	if err != nil {
		log.Fatal(err)
	}

	provisioner.New(cfg).Start(alertsFileName)
}
