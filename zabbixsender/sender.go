package zabbixsender

import (
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"

	"github.com/devopyio/zabsnd/zabbixsender/zabbixsnd"
	"github.com/devopyio/zabsnd/zabbixsender/zabbixsvc"
	log "github.com/sirupsen/logrus"
)

func Run(wordAddr string, configFileName string) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{DisableColors: true})

	cfg, err := zabbixsvc.LoadConfig(configFileName)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Configuration loaded '%s'", configFileName)

	JSONh := &zabbixsvc.JSONHandler{
		Router: httprouter.New(),
		Logger: log.New(),
	}

	JSONh.ZabbixSender.Sender = zabbixsnd.Sender{
		Host: cfg.ZabbixServerHost,
		Port: cfg.ZabbixServerPort,
	}
	JSONh.ZabbixSender.Config = *cfg

	handler := &zabbixsvc.Handler{
		JSONHandler: JSONh,
	}

	handler.JSONHandler.Router.POST("/", handler.JSONHandler.HandlePost)
	log.Fatal(http.ListenAndServe(wordAddr, handler.JSONHandler.Router))
}
