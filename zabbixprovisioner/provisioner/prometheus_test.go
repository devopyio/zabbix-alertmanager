package provisioner_test

import (
	"testing"

	"github.com/devopyio/zabbix-alertmanager/zabbixprovisioner/provisioner"
)

const (
	rulesOKpath      = "/home/edas/go/src/github.com/devopyio/zabbix-alertmanager/zabbixprovisioner/provisioner/testfiles/rulesOK_test.yml"
	rulesOKcyclepath = "/home/edas/go/src/github.com/devopyio/zabbix-alertmanager/zabbixprovisioner/provisioner/testfiles/rulesOKcycle_test.yml"
	rulesOpenErr     = ""
	rulesReadErr     = "/home/edas/go/src/github.com/devopyio/zabbix-alertmanager/zabbixprovisioner/provisioner/testfiles/rulesErr_test.yml"
)

func TestLoadPrometheusRulesFromFileOK(t *testing.T) {
	expected := 4
	rules, err := provisioner.LoadPrometheusRulesFromFile(rulesOKpath)

	if err != nil {
		t.Fatal("Expected to work, got :", err)
	}
	t.Log(rules)
	if len(rules) != expected {
		t.Fatalf("Expeceted to get %d rules, but got %d", expected, len(rules))
	}
}

func TestLoadPrometheusRulesFromFileOK2(t *testing.T) {
	expected := 5
	rules, err := provisioner.LoadPrometheusRulesFromFile(rulesOKcyclepath)

	if err != nil {
		t.Fatal("Expected to work, got :", err)
	}
	t.Log(rules)
	if len(rules) != expected {
		t.Fatalf("Expeceted to get %d rules, but got %d", expected, len(rules))
	}
}

func TestLoadPrometheusRulesFromFileOpenErr(t *testing.T) {
	_, err := provisioner.LoadPrometheusRulesFromFile(rulesOpenErr)

	if err == nil {
		t.Fatal("Expected error, got", err)
	}

	_, err = provisioner.LoadPrometheusRulesFromFile(rulesReadErr)

	if err == nil {
		t.Fatal("Expected error, got", err)
	}
}
