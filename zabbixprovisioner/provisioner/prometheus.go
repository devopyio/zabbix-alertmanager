package provisioner

import (
	"io/ioutil"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type PrometheusAlertRules struct {
	Groups []struct {
		Rules []struct {
			Name        string            `yaml:"alert"`
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

type PrometheusRule struct {
	Name        string            `yaml:"name"`
	Annotations map[string]string `yaml:"annotations"`
}

type PrometheusResponse struct {
	Rules []PrometheusRule `json:"rules"`
}

func LoadPrometheusRulesFromFile(filename string) ([]PrometheusRule, error) {
	alertsFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the alerts file")
	}

	rule := PrometheusAlertRules{}

	err = yaml.Unmarshal(alertsFile, &rule)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the alerts file")
	}
	rules := []PrometheusRule{}
	temp := PrometheusRule{}
	temp.Annotations = make(map[string]string)
	for _, groups := range rule.Groups {
		for _, grrules := range groups.Rules {
			temp.Name = grrules.Name
			for key, annotation := range grrules.Annotations {
				temp.Annotations[key] = annotation

			}
			rules = append(rules, temp)
		}
	}

	return rules, nil
}
