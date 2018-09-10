package zabbixsvc_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devopyio/zabsnd/zabbixsender/zabbixsnd"

	"github.com/julienschmidt/httprouter"

	"net"

	"github.com/devopyio/zabsnd/zabbixsender/zabbixsvc"
	log "github.com/sirupsen/logrus"
)

const (
	alertOK = `{  
		"version":"4",
		"groupKey":"{}:{alertname=\"InstanceDown\"}",
		"status":"resolved",
		"receiver":"testing",
		"groupLabels":{  
		   "alertname":"InstanceDown"
		},
		"commonLabels":{  
		   "alertname":"InstanceDown",
		   "instance":"localhost:9100",
		   "job":"node_exporter",
		   "severity":"critical"
		},
		"commonAnnotations":{  
		   "description":"localhost:9100 of job node_exporter has been down for more than 1 minute.",
		   "summary":"Instance localhost:9100 down"
		},
		"externalURL":"http://edas-GE72-6QC:9093",
		"alerts":[  
		   {  
			  "labels":{  
				 "alertname":"InstanceDown",
				 "instance":"localhost:9100",
				 "job":"node_exporter",
				 "severity":"critical"
			  },
			  "annotations":{  
				 "description":"localhost:9100 of job node_exporter has been down for more than 1 minute.",
				 "summary":"Instance localhost:9100 down"
			  },
			  "startsAt":"2018-08-30T16:59:09.653872838+03:00",
			  "EndsAt":"2018-08-30T17:01:09.656110177+03:00"
		   }
		]
	 }`

	alertBadReqErr = `{  
		"status": BadRequest
	 }`

	alertMissingFields = `{  
		"version":"4",
		"groupKey":"{}:{alertname=\"InstanceDown\"}",
		"status":"",
		"receiver":"testing",
		"groupLabels":{  
		   "alertname":"InstanceDown"
		},
		"commonLabels":{  
		   "alertname":"",
		   "instance":"localhost:9100",
		   "job":"node_exporter",
		   "severity":"critical"
		},
		"commonAnnotations":{  
		   "description":"localhost:9100 of job node_exporter has been down for more than 1 minute.",
		   "summary":"Instance localhost:9100 down"
		},
		"externalURL":"http://edas-GE72-6QC:9093",
		"alerts":[  
		   {  
			  "labels":{  
				 "alertname":"InstanceDown",
				 "instance":"localhost:9100",
				 "job":"node_exporter",
				 "severity":"critical"
			  },
			  "annotations":{  
				 "description":"localhost:9100 of job node_exporter has been down for more than 1 minute.",
				 "summary":"Instance localhost:9100 down"
			  },
			  "startsAt":"2018-08-30T16:59:09.653872838+03:00",
			  "EndsAt":"2018-08-30T17:01:09.656110177+03:00"
		   }
		]
	 }`

	alertInternal = `{  
		"version":"4",
		"groupKey":"{}:{alertname=\"InstanceDown\"}",
		"status":"firing",
		"receiver":"testing",
		"groupLabels":{  
		   "alertname":"InstanceDown"
		},
		"commonLabels":{  
		   "alertname":"InstanceDown",
		   "instance":"localhost:9100",
		   "job":"node_exporter",
		   "severity":"critical"
		},
		"commonAnnotations":{  
		   "description":"localhost:9100 of job node_exporter has been down for more than 1 minute.",
		   "summary":"Instance localhost:9100 down"
		},
		"externalURL":"http://edas-GE72-6QC:9093",
		"alerts":[  
		   {  
			  "labels":{  
				 "alertname":"InstanceDown",
				 "instance":"localhost:9100",
				 "job":"node_exporter",
				 "severity":"critical"
			  },
			  "annotations":{  
				 "description":"localhost:9100 of job node_exporter has been down for more than 1 minute.",
				 "summary":"Instance localhost:9100 down"
			  },
			  "startsAt":"2018-08-30T16:59:09.653872838+03:00",
			  "EndsAt":"2018-08-30T17:01:09.656110177+03:00"
		   }
		]
	 }`
)

var cfg zabbixsvc.ZabbixSenderConfig

func TestConfigFromFile(t *testing.T) {
	_, err := zabbixsvc.LoadConfig("")
	if err == nil {
		t.Fatalf("Wanted error, got %s", err)
	}
}

func TestJSONHandlerOK(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	go func() {
		for {
			log.Info("here")
			conn, err := l.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			buf := make([]byte, 112)
			_, err = conn.Read(buf)
			if err != nil {
				t.Fatal(err)
			}
			log.Info(string(buf))

			if msg := string(buf); msg != "ZBXD\x01x\x00\x00\x00\x00\x00\x00\x00{\"request\":\"sender data\",\"data\":[{\"host\":\"Testing\",\"key\":\"any.instancedown\",\"value\":\"0\",\"clock\":48}" {
				t.Fatalf("Unexpected message:\nGot:\t\t%s\nExpected:\t%s\n", msg, "Request sent")
			}
			_, err = conn.Write([]byte("ZBXD\x01Z\x00\x00\x00\x00\x00\x00\x00{\"response\":\"success\",\"info\":\"processed: 1; failed: 0; total: 1; seconds spent: 0.000041\"}"))
			if err != nil {
				t.Fatal(err)
			}

			conn.Read(buf)

			return
		}
	}()

	h := &zabbixsvc.JSONHandler{
		Router: httprouter.New(),
		Logger: log.New(),
	}
	h.ZabbixSender.Sender = zabbixsnd.Sender{
		Host: "127.0.0.1",
		Port: 3000,
	}
	h.ZabbixSender.Config = zabbixsvc.ZabbixSenderConfig{
		ZabbixServerHost:     "127.0.0.1",
		ZabbixServerPort:     3000,
		ZabbixHostAnnotation: "Testing",
		ZabbixKeyPrefix:      "any",
	}

	h.Router.POST("/", h.HandlePost)
	rr := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/", strings.NewReader(alertOK))
	if err != nil {
		t.Fatal(err)
	}

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatal("Expected working, got error:", rr.Code)
	}

}

func TestJSONHandlerStatusBadRequest(t *testing.T) {
	h := &zabbixsvc.JSONHandler{
		Router: httprouter.New(),
		Logger: log.New(),
	}
	h.ZabbixSender.Sender = zabbixsnd.Sender{
		Host: "127.0.0.1",
		Port: 3000,
	}
	h.ZabbixSender.Config = zabbixsvc.ZabbixSenderConfig{
		ZabbixServerHost:     "127.0.0.1",
		ZabbixServerPort:     3000,
		ZabbixHostAnnotation: "Testing",
		ZabbixKeyPrefix:      "any",
	}

	h.Router.POST("/", h.HandlePost)
	rr := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/", strings.NewReader(alertBadReqErr))
	if err != nil {
		t.Fatal(err)
	}

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatal("Expected error, got:", rr.Code)
	}

}
func TestJSONHandlerMissingFields(t *testing.T) {
	h := &zabbixsvc.JSONHandler{
		Router: httprouter.New(),
		Logger: log.New(),
	}
	h.ZabbixSender.Sender = zabbixsnd.Sender{
		Host: "127.0.0.1",
		Port: 3000,
	}
	h.ZabbixSender.Config = zabbixsvc.ZabbixSenderConfig{
		ZabbixServerHost:     "127.0.0.1",
		ZabbixServerPort:     3000,
		ZabbixHostAnnotation: "Testing",
		ZabbixKeyPrefix:      "any",
	}

	h.Router.POST("/", h.HandlePost)
	rr := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/", strings.NewReader(alertMissingFields))
	if err != nil {
		t.Fatal(err)
	}

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatal("Expected error, got:", rr.Code)
	}

}

func TestJSONHandlerInternal(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	go func() {
		for {
			log.Info("here")
			conn, err := l.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			buf := make([]byte, 112)
			_, err = conn.Read(buf)
			if err != nil {
				t.Fatal(err)
			}
			return
		}
	}()

	h := &zabbixsvc.JSONHandler{
		Router: httprouter.New(),
		Logger: log.New(),
	}
	h.ZabbixSender.Sender = zabbixsnd.Sender{
		Host: "127.0.0.1",
		Port: 3000,
	}
	h.ZabbixSender.Config = zabbixsvc.ZabbixSenderConfig{
		ZabbixServerHost:     "127.0.0.1",
		ZabbixServerPort:     3000,
		ZabbixHostAnnotation: "Testing",
		ZabbixKeyPrefix:      "any",
	}

	h.Router.POST("/", h.HandlePost)
	rr := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/", strings.NewReader(alertInternal))
	if err != nil {
		t.Fatal(err)
	}

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatal("Expected error, got:", rr.Code)
	}

}
