package provisioner

import (
	"io/ioutil"
	"strings"

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
}

func LoadPrometheusRulesFromDir(dir string) ([]PrometheusRule, error) {
	filesInDir, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the alerts files directory")
	}

	var rules []PrometheusRule

	for _, file := range filesInDir {
		if strings.HasSuffix(file.Name(), ".yml") {
			alertsFile, err := ioutil.ReadFile(dir + file.Name())
			if err != nil {
				return nil, errors.Wrapf(err, "can't open the alerts file-", file.Name())
			}

			ruleConfig := PrometheusAlertRules{}

			err = yaml.Unmarshal(alertsFile, &ruleConfig)
			if err != nil {
				return nil, errors.Wrapf(err, "can't read the alerts file-", file.Name())
			}

			for _, rule := range ruleConfig.Groups {
				rules = append(rules, rule.Rules...)
			}

		}
	}
	return rules, nil
}
