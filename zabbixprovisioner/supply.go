package provisioner

import (
	"github.com/devopyio/zabbix-alertmanager/zabbixprovisioner/provisioner"
	log "github.com/sirupsen/logrus"
)

func Run(configFileName string, alertsFileName string) {
	cfg, err := provisioner.LoadFromFile(configFileName)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("loading configuration at '%s'", configFileName)

	provisioner.New(cfg).Start(alertsFileName)
}
