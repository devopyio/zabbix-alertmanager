package provisioner_test

import (
	"testing"

	"github.com/Dexanir/zabbix-alertmanager/zabbixprovisioner/provisioner"
)

const (
	rulesOKpath      = "./testdata/testsOK/"
	rulesErrReadFile = "./testdata/testsErr/read/"
	rulesErrOpenDir  = ""
	rulesErrSameName = "./testdata/testsErr/samename/"
)

func TestLoadPrometheusRulesFromPathOK(t *testing.T) {
	expected := 7
	rules, err := provisioner.LoadPrometheusRulesFromDir(rulesOKpath)
	if err != nil {
		t.Error("Expected to work, got :", err)
	}
	if len(rules) != expected {
		t.Errorf("Expeceted to get %d rules, but got %d", expected, len(rules))
	}
}

func TestLoadPrometheusRulesFromPathErrorReadFile(t *testing.T) {
	_, err := provisioner.LoadPrometheusRulesFromDir(rulesErrReadFile)
	if err == nil {
		t.Error("Expected to get error, got :", err)
	}
}

func TestLoadPrometheusRulesFromPathErrorOpenDir(t *testing.T) {
	_, err := provisioner.LoadPrometheusRulesFromDir(rulesErrOpenDir)
	if err == nil {
		t.Error("Expected to get error, got :", err)
	}
}

func TestLoadPrometheusRulesFromPathErrorSameName(t *testing.T) {
	_, err := provisioner.LoadPrometheusRulesFromDir(rulesErrSameName)
	if err == nil {
		t.Error("Expected to get error, got :", err)
	}
}
