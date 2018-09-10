package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"

	"github.com/devopyio/zabsnd/alertmanager-zabbix/zabbixsnd"
	"github.com/devopyio/zabsnd/alertmanager-zabbix/zabbixsvc"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{DisableColors: true})

	wordAddr := flag.String("addr", "foo", "a string")
	configFileName := flag.String("config", "./config.yaml", "path to the configuration file")
	flag.Parse()

	cfg, err := zabbixsvc.LoadConfig(*configFileName)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Configuration loaded '%s'", *configFileName)

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
	log.Fatal(http.ListenAndServe(*wordAddr, handler.JSONHandler.Router))
}
