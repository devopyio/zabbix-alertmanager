package main

import (
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/devopyio/zabbix-alertmanager/zabbixprovisioner/provisioner"
	"github.com/devopyio/zabbix-alertmanager/zabbixsender/zabbixsnd"
	"github.com/devopyio/zabbix-alertmanager/zabbixsender/zabbixsvc"
	"github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	app := kingpin.New("zal", "A zabbix and prometheus integration.")

	app.Version(version.Print("zal"))
	app.HelpFlag.Short('h')

	send := app.Command("send", "Start zabbix sender.")
	senderAddr := send.Arg("addr", "Server address which will receive alerts from alertmanager.").Required().String()
	zabbixAddr := send.Arg("zabbix-addr", "Zabbix address.").Required().String()
	keyPrefix := send.Arg("key-prefix", "Prefix to add to the trapper item key").Default("prometheus").String()
	defaultHost := send.Arg("default-host", "default host-name").Default("prometheus").String()

	prov := app.Command("prov", "Start zabbix provisioner.")
	provConfig := prov.Flag("config-path", "Path to provisioner hosts config file.").Required().String()
	provAlerts := prov.Flag("alert-path", "Path to the prometheus alerts files.").Required().String()
	provUser := prov.Flag("user", "Zabbix json rpc user.").Envar("ZABBIX_USER").Required().String()
	provPassword := prov.Flag("password", "Zabbix json rpc password.").Envar("ZABBIX_PASSWORD").Required().String()
	provURL := prov.Flag("url", "Zabbix json rpc url.").Envar("ZABBIX_URL").Default("https://127.0.0.1/zabbix/api_jsonrpc.php").String()
	provKeyPrefix := prov.Flag("key-prefix", "Prefix to add to the trapper item key.").Default("prometheus").String()
	prometheusURL := prov.Flag("prometheus-url", "Prometheus URL.").Default("").String()

	logLevel := app.Flag("log.level", "Log level.").
		Default("info").Enum("error", "warn", "info", "debug")
	logFormat := app.Flag("log.format", "Log format.").
		Default("text").Enum("text", "json")

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	switch strings.ToLower(*logLevel) {
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	}

	switch strings.ToLower(*logFormat) {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	case "text":
		log.SetFormatter(&log.TextFormatter{DisableColors: true})
	}
	log.SetOutput(os.Stdout)

	switch cmd {
	case send.FullCommand():
		s, err := zabbixsnd.New(*zabbixAddr)
		if err != nil {
			log.Fatalf("error could not create zabbix sender: %v", err)
		}

		h := &zabbixsvc.JSONHandler{
			Sender:      s,
			KeyPrefix:   *keyPrefix,
			DefaultHost: *defaultHost,
		}

		http.HandleFunc("/", h.HandlePost)

		if err := http.ListenAndServe(*senderAddr, nil); err != nil {
			log.Fatal(err)
		}

	case prov.FullCommand():
		cfg, err := provisioner.LoadHostConfigFromFile(*provConfig)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("loaded hosts configuration from '%s'", *provConfig)

		prov, err := provisioner.New(*provAlerts, *prometheusURL, *provKeyPrefix, *provURL, *provUser, *provPassword, cfg)
		if err != nil {
			log.Fatalf("error failed to create provisioner: %s", err)
		}

		if err := prov.Run(); err != nil {
			log.Fatalf("error provisioning zabbix items: %s", err)
		}
	}
}

func interrupt(logger log.Logger, cancel <-chan struct{}) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-c:
		log.Info("caught signal. Exiting.", "signal", s)
		return nil
	case <-cancel:
		return errors.New("canceled")
	}
}
