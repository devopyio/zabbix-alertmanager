package zabbixsvc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/devopyio/zabsnd/zabbixsender/zabbixsnd"
	"github.com/pkg/errors"

	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

type ZabbixSender struct {
	Sender zabbixsnd.Sender
	Config ZabbixSenderConfig
}

type ZabbixSenderConfig struct {
	Port                 int    `yaml:"port"`
	QueueCapacity        int    `yaml:"queueCapacity"`
	ZabbixServerHost     string `yaml:"zabbixServerHost"`
	ZabbixServerPort     int    `yaml:"zabbixServerPort"`
	ZabbixHostDefault    string `yaml:"zabbixHostDefault"`
	ZabbixHostAnnotation string `yaml:"zabbixHostAnnotation"`
	ZabbixKeyPrefix      string `yaml:"zabbixKeyPrefix"`
}

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

//JSONHandler handles alerts
type JSONHandler struct {
	*httprouter.Router
	Logger       *log.Logger
	ZabbixSender ZabbixSender
}

func (h *JSONHandler) HandlePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Info("Post request is being handled")
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

	var metrics []*zabbixsnd.Metric

	host := h.ZabbixSender.Config.ZabbixHostAnnotation

	value := "0"
	if req.Status == "firing" {
		value = "1"
	}

	for _, alert := range req.Alerts {
		key := fmt.Sprintf("%s.%s", h.ZabbixSender.Config.ZabbixKeyPrefix, strings.ToLower(alert.Labels["alertname"]))

		log.Infof("added Zabbix metrics, host: '%s' key: '%s', value: '%s'", host, key, value)
		m := &zabbixsnd.Metric{Host: host, Key: key, Value: value}
		// use current time, if `clock` is not specified
		if m.Clock = time.Now().Unix(); len(value) > 0 {
			m.Clock = int64(value[0])
		}
		metrics = append(metrics, m)
	}

	res, err := h.zabbixSend(metrics)
	if err != nil {
		log.Errorf("Failed to send to server: %s", err)
		http.Error(w, "Failed to send to server", http.StatusInternalServerError)
	}
	log.Debug("Request succesfully sent! ", string(res))

}

//LoadConfig reads the config.yaml
func LoadConfig(filename string) (cfg *ZabbixSenderConfig, err error) {
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the config file")
	}
	var config ZabbixSenderConfig

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the config file: %s")
	}

	return &config, nil
}

func (h *JSONHandler) zabbixSend(metrics []*zabbixsnd.Metric) ([]byte, error) {
	packet := zabbixsnd.NewPacket(metrics)

	res, err := h.ZabbixSender.Sender.Send(packet)
	if err != nil {
		return nil, err
	}

	return res, nil

}
