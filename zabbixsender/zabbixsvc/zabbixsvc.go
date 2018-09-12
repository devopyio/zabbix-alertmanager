package zabbixsvc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devopyio/zabbix-alertmanager/zabbixsender/zabbixsnd"
	log "github.com/sirupsen/logrus"
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
	//TODO: Introduce annotation to host mapping?
}

func (h *JSONHandler) HandlePost(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var req ZabbixSenderRequest

	if err := dec.Decode(&req); err != nil {
		log.Errorf("error decoding message: %v", err)
		http.Error(w, "request body is not valid json", http.StatusBadRequest)
		return
	}

	if req.Status == "" || req.CommonLabels["alertname"] == "" {
		http.Error(w, "missing fields in request body", http.StatusBadRequest)
		return
	}

	value := "0"
	if req.Status == "firing" {
		value = "1"
	}

	var metrics []*zabbixsnd.Metric
	for _, alert := range req.Alerts {
		host := h.DefaultHost

		key := fmt.Sprintf("%s.%s", h.KeyPrefix, strings.ToLower(alert.Labels["alertname"]))
		m := &zabbixsnd.Metric{Host: host, Key: key, Value: value}

		m.Clock = time.Now().Unix()

		metrics = append(metrics, m)

		log.Debugf("sending zabbix metrics, host: '%s' key: '%s', value: '%s'", host, key, value)
	}

	res, err := h.zabbixSend(metrics)
	if err != nil {
		log.Errorf("failed to send to server: %s", err)
		http.Error(w, "failed to send to server", http.StatusInternalServerError)
	}

	infoSplit := strings.Split(res.Info, " ")
	if strings.Trim(infoSplit[3], ";") != "0" || res.Response != "success" {
		log.Errorf("failed to fulfill the requests: %s", strings.Trim(infoSplit[5], ";"))
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

	return &zres, nil
}
