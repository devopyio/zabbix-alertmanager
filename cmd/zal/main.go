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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ver "github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	//version variable populated on build time
	version, revision, branch, user, date string
)

func main() {
	ver.Version = version
	ver.Revision = revision
	ver.Branch = branch
	ver.BuildUser = user
	ver.BuildDate = date

	app := kingpin.New("zal", "A zabbix and prometheus integration.")

	app.Version(ver.Print("zal"))
	app.HelpFlag.Short('h')

	send := app.Command("send", "Start zabbix sender.")
	senderAddr := send.Flag("addr", "Server address which will receive alerts from alertmanager.").Default("0.0.0.0:9095").String()
	zabbixAddr := send.Flag("zabbix-addr", "Zabbix address.").Envar("ZABBIX_URL").Required().String()
	hostsFile := send.Flag("hosts-path", "Path to resolver to host mapping file.").String()
	keyPrefix := send.Flag("key-prefix", "Prefix to add to the trapper item key").Default("prometheus").String()
	defaultHost := send.Flag("default-host", "default host to send alerts to").Default("prometheus").String()

	prov := app.Command("prov", "Start zabbix provisioner.")
	provConfig := prov.Flag("config-path", "Path to provisioner hosts config file.").Required().String()
	provUser := prov.Flag("user", "Zabbix json rpc user.").Envar("ZABBIX_USER").Required().String()
	provPassword := prov.Flag("password", "Zabbix json rpc password.").Envar("ZABBIX_PASSWORD").Required().String()
	provURL := prov.Flag("url", "Zabbix json rpc url.").Envar("ZABBIX_URL").Default("http://127.0.0.1/zabbix/api_jsonrpc.php").String()
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

	prometheus.MustRegister(ver.NewCollector(send.FullCommand()))

	switch cmd {
	case send.FullCommand():
		s, err := zabbixsnd.New(*zabbixAddr)
		if err != nil {
			log.Fatalf("error could not create zabbix sender: %v", err)
		}

		hosts := make(map[string]string)

		if hostsFile != nil && *hostsFile != "" {
			hosts, err = zabbixsvc.LoadHostsFromFile(*hostsFile)
			if err != nil {
				log.Errorf("cant load the default hosts file: %v", err)
			}
		}

		h := &zabbixsvc.JSONHandler{
			Sender:      s,
			KeyPrefix:   *keyPrefix,
			DefaultHost: *defaultHost,
			Hosts:       hosts,
		}

		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/alerts", h.HandlePost)

		log.Info("Zabbix sender started, listening on ", *senderAddr)
		if err := http.ListenAndServe(*senderAddr, nil); err != nil {
			log.Fatal(err)
		}

	case prov.FullCommand():
		cfg, err := provisioner.LoadHostConfigFromFile(*provConfig)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("loaded hosts configuration from '%s'", *provConfig)

		prov, err := provisioner.New(*prometheusURL, *provKeyPrefix, *provURL, *provUser, *provPassword, cfg)
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
