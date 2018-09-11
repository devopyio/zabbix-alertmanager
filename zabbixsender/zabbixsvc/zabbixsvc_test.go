package zabbixsvc_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devopyio/zabbix-alertmanager/zabbixsender/zabbixsnd"
	"github.com/devopyio/zabbix-alertmanager/zabbixsender/zabbixsvc"
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

func TestJSONHandlerOK(t *testing.T) {
	const expectedMsg = "ZBXD\x01}\x00\x00\x00\x00\x00\x00\x00{\"request\":\"sender data\",\"data\":[{\"host\":\"Testing\",\"key\":\".instancedown\",\"value\":\"0\",\"clock\""
	l, err := net.Listen("tcp", "127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			buf := make([]byte, 141)
			_, err = conn.Read(buf)
			if err != nil {
				t.Fatal(err)
			}
			log.Info(string(buf))

			if msg := string(buf[:105]); msg != expectedMsg {
				t.Fatalf("Unexpected message:\nGot:\t\t%s\nExpected:\t%s\n", msg, expectedMsg)
			}
			_, err = conn.Write([]byte("ZBXD\x01Z\x00\x00\x00\x00\x00\x00\x00{\"response\":\"success\",\"info\":\"processed: 1; failed: 0; total: 1; seconds spent: 0.000041\"}"))
			if err != nil {
				t.Fatal(err)
			}

			return
		}
	}()

	s, err := zabbixsnd.New("127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}

	h := &zabbixsvc.JSONHandler{
		Sender:      s,
		DefaultHost: "Testing",
	}

	rr := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/", strings.NewReader(alertOK))
	if err != nil {
		t.Fatal(err)
	}

	h.HandlePost(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatal("Expected working, got error:", rr.Code)
	}
}

func TestJSONHandlerStatusBadRequest(t *testing.T) {
	s, err := zabbixsnd.New("127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}

	h := &zabbixsvc.JSONHandler{
		Sender:      s,
		DefaultHost: "host",
	}

	rr := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/", strings.NewReader(alertBadReqErr))
	if err != nil {
		t.Fatal(err)
	}

	h.HandlePost(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatal("Expected error, got:", rr.Code)
	}

}
func TestJSONHandlerMissingFields(t *testing.T) {
	s, err := zabbixsnd.New("127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}

	h := &zabbixsvc.JSONHandler{
		Sender:      s,
		DefaultHost: "host",
	}

	rr := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/", strings.NewReader(alertMissingFields))
	if err != nil {
		t.Fatal(err)
	}

	h.HandlePost(rr, req)

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

	s, err := zabbixsnd.New("127.0.0.1:3000")
	if err != nil {
		t.Fatal(err)
	}

	h := &zabbixsvc.JSONHandler{
		Sender:      s,
		DefaultHost: "host",
	}

	rr := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/", strings.NewReader(alertInternal))
	if err != nil {
		t.Fatal(err)
	}

	h.HandlePost(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatal("Expected error, got:", rr.Code)
	}

}
