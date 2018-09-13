package zabbixsvc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/devopyio/zabbix-alertmanager/zabbixsender/zabbixsnd"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

type ZabbixSenderRequest struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []Alert           `json:"alerts"`
}

type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    string            `json:"startsAt,omitempty"`
	EndsAt      string            `json:"EndsAt,omitempty"`
}

type ZabbixResponse struct {
	Response string `json:"response"`
	Info     string `json:"info"`
}

//JSONHandler handles alerts
type JSONHandler struct {
	Sender       *zabbixsnd.Sender
	KeyPrefix    string
	DefaultHost  string
	DefaultHosts DefaultHosts
	//TODO: Introduce annotation to host mapping?
}

//TODO change to specific hosts and values
type DefaultHosts struct {
	Hosts []struct {
		Default  string `yaml:"default"`
		Received string `yaml:"received"`
	} `yaml:"hosts"`
}

var alertsSentStats = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "alerts_sent",
		Help: "Current number of sent alerts by status",
	},
	[]string{"alert_status"},
)

var alertsErrorsTotal = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "alerts_errors_total",
		Help: "Current number of different errors",
	},
)

func init() {
	prometheus.MustRegister(alertsSentStats)
	prometheus.MustRegister(alertsErrorsTotal)
}

func (h *JSONHandler) HandlePost(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var req ZabbixSenderRequest

	if err := dec.Decode(&req); err != nil {
		alertsErrorsTotal.Inc()

		log.Errorf("error decoding message: %v", err)
		http.Error(w, "request body is not valid json", http.StatusBadRequest)
		return
	}

	if req.Status == "" || req.CommonLabels["alertname"] == "" {
		alertsErrorsTotal.Inc()

		http.Error(w, "missing fields in request body", http.StatusBadRequest)
		return
	}

	value := "0"
	if req.Status == "firing" {
		value = "1"
	}

	var ammountOfResolved float64
	if req.Status == "resolved" {
		ammountOfResolved = float64(len(req.Alerts))
	}

	host := h.DefaultHost
	for _, hostDefault := range h.DefaultHosts.Hosts {
		if hostDefault.Received == req.Receiver {
			found := hostDefault.Default
			host = found
		}
	}

	var metrics []*zabbixsnd.Metric
	for _, alert := range req.Alerts {
		key := fmt.Sprintf("%s.%s", h.KeyPrefix, strings.ToLower(alert.Labels["alertname"]))
		m := &zabbixsnd.Metric{Host: host, Key: key, Value: value}

		m.Clock = time.Now().Unix()

		metrics = append(metrics, m)

		log.Debugf("sending zabbix metrics, host: '%s' key: '%s', value: '%s'", host, key, value)
	}

	res, err := h.zabbixSend(metrics, ammountOfResolved)
	if err != nil {
		log.Errorf("failed to send to server: %s", err)
		http.Error(w, "failed to send to server", http.StatusInternalServerError)
	}
	log.Debugf("request succesfully sent: %s", res)
}

func (h *JSONHandler) zabbixSend(metrics []*zabbixsnd.Metric, ammount float64) (*ZabbixResponse, error) {
	var zres ZabbixResponse

	packet := zabbixsnd.NewPacket(metrics)

	res, err := h.Sender.Send(packet)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(res[13:], &zres); err != nil {
		return nil, err
	}

	infoSplit := strings.Split(zres.Info, " ")

	failed, err := strconv.Atoi(strings.Trim(infoSplit[3], ";"))
	if err != nil {
		return nil, err
	}
	if failed != 0 || zres.Response != "success" {
		alertsErrorsTotal.Add(float64(failed))
		return nil, errors.Errorf("failed to fulfill the requests: %v", failed)
	}

	succes, err := strconv.Atoi(strings.Trim(infoSplit[1], ";"))
	if err != nil {
		return nil, err
	}
	alertsSentStats.WithLabelValues("resolved").Add(ammount)
	alertsSentStats.WithLabelValues("unresolved").Add(float64(succes) - ammount)

	return &zres, nil
}

func (h *JSONHandler) LoadHostsFromFile(filename string) (*DefaultHosts, error) {
	hostsFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the alerts file- %v", filename)
	}

	hostConfig := DefaultHosts{}

	err = yaml.Unmarshal(hostsFile, &hostConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the alerts file- %v", filename)
	}

	return &hostConfig, nil
}
