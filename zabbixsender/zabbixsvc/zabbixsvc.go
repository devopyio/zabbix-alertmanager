package zabbixsvc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/devopyio/zabbix-alertmanager/zabbixsender/zabbixsnd"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
	Sender      *zabbixsnd.Sender
	KeyPrefix   string
	DefaultHost string
	Hosts       map[string]string
}

var alertsSentStats = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "alerts_sent",
		Help: "Current number of sent alerts by status",
	},
	[]string{"alert_status"},
)

var alertsErrorsTotal = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "alerts_errors_total",
		Help: "Current number of different errors",
	},
)

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

	if req.Status == "resolved" {
		alertsSentStats.WithLabelValues("resolved").Inc()
	} else {
		alertsSentStats.WithLabelValues("unresolved").Inc()
	}

	host, ok := h.Hosts[req.Receiver]
	if !ok {
		host = h.DefaultHost
	}

	var metrics []*zabbixsnd.Metric
	for _, alert := range req.Alerts {
		key := fmt.Sprintf("%s.%s", h.KeyPrefix, strings.ToLower(alert.Labels["alertname"]))
		m := &zabbixsnd.Metric{Host: host, Key: key, Value: value}

		m.Clock = time.Now().Unix()

		metrics = append(metrics, m)

		log.Debugf("sending zabbix metrics, host: '%s' key: '%s', value: '%s'", host, key, value)
	}

	res, err := h.zabbixSend(metrics)
	if err != nil {
		alertsErrorsTotal.Inc()
		log.Errorf("failed to send to server: %s", err)
		http.Error(w, "failed to send to server", http.StatusInternalServerError)
	} else {
		log.Debugf("request succesfully sent: %s", res)
	}
}

func (h *JSONHandler) zabbixSend(metrics []*zabbixsnd.Metric) (*ZabbixResponse, error) {
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

	failed := strings.Trim(infoSplit[3], ";")
	if failed != "0" || zres.Response != "success" {
		return nil, errors.Errorf("failed to fulfill the requests: %v", failed)
	}

	return &zres, nil
}

func (h *JSONHandler) LoadHostsFromFile(filename string) (map[string]string, error) {
	hostsFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the alerts file- %v", filename)
	}

	var hosts map[string]string
	err = yaml.Unmarshal(hostsFile, &hosts)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the alerts file- %v", filename)
	}

	return hosts, nil
}
