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

// AlertmanageRequest this is request received from Alertmanager.
type AlertmanagerRequest struct {
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

// Alert is alert received from alertmanager.
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

// JSONHandler handles alerts
type JSONHandler struct {
	Sender      *zabbixsnd.Sender
	KeyPrefix   string
	DefaultHost string
	Hosts       map[string]string
}

var (
	alertsSentStats = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alerts_sent_total",
			Help: "Current number of sent alerts by status",
		},
		[]string{"alert_status", "host"},
	)

	alertsErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alerts_errors_total",
			Help: "Current number of different errors",
		},
		[]string{"alert_status", "host"},
	)
)

func (h *JSONHandler) HandlePost(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var req AlertmanagerRequest
	if err := dec.Decode(&req); err != nil {
		alertsErrorsTotal.WithLabelValues("", "").Inc()

		log.Errorf("error decoding message: %v", err)
		http.Error(w, "request body is not valid json", http.StatusBadRequest)
		return
	}

	if req.Status == ""  {
		alertsErrorsTotal.WithLabelValues(req.Status, req.Receiver).Inc()
		http.Error(w, "missing fields in request body", http.StatusBadRequest)
		return
	}

	value := "0"
	if req.Status == "firing" {
		value = "1"
	}

	host, ok := h.Hosts[req.Receiver]
	if !ok {
		host = h.DefaultHost
		log.Warnf("using default host %s, receiver not found: %s", host, req.Receiver)
	}

	alertsSentStats.WithLabelValues(req.Status, host).Inc()

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
		alertsErrorsTotal.WithLabelValues(req.Status, host).Add(float64(len(req.Alerts)))
		log.Errorf("failed to send to server, metrics: %v, error: %s, raw request: %v", metrics, err, req)
		http.Error(w, "failed to send to server", http.StatusInternalServerError)
		return
	}

	log.Debugf("request succesfully sent: %s", res)
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
		return nil, errors.Errorf("failed to fulfill the requests: %v, info: %v, Data: %v", failed, zres.Info, zres.Response)
	}

	return &zres, nil
}

func LoadHostsFromFile(filename string) (map[string]string, error) {
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
