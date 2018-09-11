package provisioner

import (
	"io/ioutil"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type PrometheusAlertRules struct {
	Groups []struct {
		Rules []PrometheusRule `yaml:"rules"`
	} `yaml:"groups"`
}

type PrometheusRule struct {
	Name        string            `yaml:"alert"`
	Annotations map[string]string `yaml:"annotations"`
	Expression  string            `yaml:"expr"`
	Labels      map[string]string `yaml:"labels"`
}

func LoadPrometheusRulesFromFile(filename string) ([]PrometheusRule, error) {
	alertsFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the alerts file")
	}

	ruleConfig := PrometheusAlertRules{}

	err = yaml.Unmarshal(alertsFile, &ruleConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the alerts file")
	}
	var rules []PrometheusRule

	for _, rule := range ruleConfig.Groups {
		rules = append(rules, rule.Rules...)
	}
	return rules, nil
}
