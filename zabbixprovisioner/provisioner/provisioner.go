package provisioner

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	zabbix "github.com/Dexanir/zabbix-alertmanager/zabbixprovisioner/zabbixclient"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

type HostConfig struct {
	Name                    string            `yaml:"name"`
	HostGroups              []string          `yaml:"hostGroups"`
	Tag                     string            `yaml:"tag"`
	DeploymentStatus        string            `yaml:"deploymentStatus"`
	ItemDefaultHistory      string            `yaml:"itemDefaultHistory"`
	ItemDefaultTrends       string            `yaml:"itemDefaultTrends"`
	ItemDefaultTrapperHosts string            `yaml:"itemDefaultTrapperHosts"`
	HostAlertsDir           string            `yaml:"alertsDir"`
	TriggerTags             map[string]string `yaml:"triggerTags"`
}

type Provisioner struct {
	api           *zabbix.API
	keyPrefix     string
	hosts         []HostConfig
	prometheusUrl string
	*CustomZabbix
}

func New(prometheusUrl, keyPrefix, url, user, password string, hosts []HostConfig) (*Provisioner, error) {
	transport := http.DefaultTransport

	api := zabbix.NewAPI(url)
	api.SetClient(&http.Client{
		Transport: transport,
	})

	_, err := api.Login(user, password)
	if err != nil {
		return nil, errors.Wrap(err, "error while login to zabbix api")
	}

	return &Provisioner{
		api:           api,
		keyPrefix:     keyPrefix,
		hosts:         hosts,
		prometheusUrl: prometheusUrl,
	}, nil
}

func LoadHostConfigFromFile(filename string) ([]HostConfig, error) {
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the config file: %s", filename)
	}

	hosts := []HostConfig{}

	err = yaml.Unmarshal(configFile, &hosts)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the config file: %s", filename)
	}

	return hosts, nil
}

func (p *Provisioner) Run() error {
	p.CustomZabbix = &CustomZabbix{
		Hosts:      map[string]*CustomHost{},
		HostGroups: map[string]*CustomHostGroup{},
	}

	//All hosts will have the rules which were only written for them
	for _, host := range p.hosts {
		if err := p.LoadRulesFromPrometheus(host); err != nil {
			return errors.Wrapf(err, "error loading prometheus rules, file: %s", host.HostAlertsDir)
		}
	}

	if err := p.LoadDataFromZabbix(); err != nil {
		return errors.Wrap(err, "error loading zabbix rules")
	}

	if err := p.ApplyChanges(); err != nil {
		return errors.Wrap(err, "error applying changes")
	}

	return nil
}

// Create hosts structures and populate them from Prometheus rules
func (p *Provisioner) LoadRulesFromPrometheus(hostConfig HostConfig) error {
	rules, err := LoadPrometheusRulesFromDir(hostConfig.HostAlertsDir)
	if err != nil {
		return errors.Wrap(err, "error loading rules")
	}

	log.Infof("Prometheus Rules for host - %v loaded: %v", hostConfig.Name, len(rules))

	newHost := &CustomHost{
		State: StateNew,
		Host: zabbix.Host{
			Host:          hostConfig.Name,
			Available:     1,
			Name:          hostConfig.Name,
			Status:        0,
			InventoryMode: zabbix.InventoryManual,
			Inventory: map[string]string{
				"deployment_status": hostConfig.DeploymentStatus,
				"tag":               hostConfig.Tag,
			},
			Interfaces: zabbix.HostInterfaces{
				zabbix.HostInterface{
					DNS:   "",
					IP:    "127.0.0.1",
					Main:  1,
					Port:  "10050",
					Type:  1,
					UseIP: 1,
				},
			},
		},
		HostGroups: make(map[string]struct{}, len(hostConfig.HostGroups)),
		Items:      map[string]*CustomItem{},
		Triggers:   map[string]*CustomTrigger{},
	}

	for _, hostGroupName := range hostConfig.HostGroups {
		p.AddHostGroup(&CustomHostGroup{
			State: StateNew,
			HostGroup: zabbix.HostGroup{
				Name: hostGroupName,
			},
		})

		newHost.HostGroups[hostGroupName] = struct{}{}
	}

	// Parse Prometheus rules and create corresponding items/triggers and applications for this host
	for _, rule := range rules {
		key := fmt.Sprintf("%s.%s", strings.ToLower(p.keyPrefix), strings.ToLower(rule.Name))

		var triggerTags []zabbix.Tag
		for k, v := range hostConfig.TriggerTags {
			triggerTags = append(triggerTags, zabbix.Tag{Tag: k, Value: v})
		}

		newItem := &CustomItem{
			State: StateNew,
			Item: zabbix.Item{
				Name:         rule.Name,
				Key:          key,
				HostId:       "", //To be filled when the host will be created
				Type:         2,  //Trapper
				ValueType:    3,
				History:      hostConfig.ItemDefaultHistory,
				Trends:       hostConfig.ItemDefaultTrends,
				TrapperHosts: hostConfig.ItemDefaultTrapperHosts,
			},
		}

		newTrigger := &CustomTrigger{
			State: StateNew,
			Trigger: zabbix.Trigger{
				Description: rule.Name,
				Expression:  fmt.Sprintf("last(/%s/%s)<>0", newHost.Name, key),
				ManualClose: 1,
				Tags:        triggerTags,
			},
		}

		if p.prometheusUrl != "" {
			newTrigger.URL = p.prometheusUrl + "/alerts"

			url := p.prometheusUrl + "/graph?g0.expr=" + url.QueryEscape(rule.Expression)
			if len(url) < 255 {
				newTrigger.URL = url
			}
		}

		if v, ok := rule.Annotations["summary"]; ok {
			newTrigger.Comments = v
		} else if v, ok := rule.Annotations["message"]; ok {
			newTrigger.Comments = v
		} else if v, ok := rule.Annotations["description"]; ok {
			newTrigger.Comments = v
		}

		if v, ok := rule.Labels["severity"]; ok {
			newTrigger.Priority = GetZabbixPriority(v)
		}

		// Add the special "No Data" trigger if requested
		if delay, ok := rule.Annotations["zabbix_trigger_nodata"]; ok {
			newTrigger.Trigger.Description = fmt.Sprintf("%s - no data for the last %s seconds", newTrigger.Trigger.Description, delay)
			newTrigger.Trigger.Expression = fmt.Sprintf("nodata(/%s/%s,%s)", newHost.Name, key, delay)
		}

		log.Debugf("Loading item from Prometheus: %+v", newItem)
		newHost.AddItem(newItem)

		log.Debugf("Loading trigger from Prometheus: %+v", newTrigger)
		newHost.AddTrigger(newTrigger)

	}
	log.Debugf("Host from Prometheus: %+v", newHost)
	p.AddHost(newHost)

	return nil
}

// Update created hosts with the current state in Zabbix
func (p *Provisioner) LoadDataFromZabbix() error {
	hostNames := make([]string, len(p.hosts))
	hostGroupNames := []string{}
	for i, _ := range p.hosts {
		hostNames[i] = p.hosts[i].Name
		hostGroupNames = append(hostGroupNames, p.hosts[i].HostGroups...)
	}

	if len(hostNames) == 0 {
		return errors.Errorf("error no hosts are defined")
	}

	zabbixHostGroups, err := p.api.HostGroupsGet(zabbix.Params{
		"output": "extend",
		"filter": map[string][]string{
			"name": hostGroupNames,
		},
	})
	if err != nil {
		return errors.Wrapf(err, "error getting hostgroups: %v", hostGroupNames)
	}

	for _, zabbixHostGroup := range zabbixHostGroups {
		p.AddHostGroup(&CustomHostGroup{
			State:     StateOld,
			HostGroup: zabbixHostGroup,
		})
	}

	zabbixHosts, err := p.api.HostsGet(zabbix.Params{
		"output": "extend",
		"selectInventory": []string{
			"tag",
			"deployment_status",
		},
		"filter": map[string][]string{
			"host": hostNames,
		},
	})
	if err != nil {
		return errors.Wrapf(err, "error getting hosts: %v", hostNames)
	}

	for _, zabbixHost := range zabbixHosts {
		zabbixHostGroups, err := p.api.HostGroupsGet(zabbix.Params{
			"output":  "extend",
			"hostids": zabbixHost.HostId,
		})
		if err != nil {
			return errors.Wrapf(err, "error getting hostgroup, hostid: %v", zabbixHost.HostId)
		}

		hostGroups := make(map[string]struct{}, len(zabbixHostGroups))
		for _, zabbixHostGroup := range zabbixHostGroups {
			hostGroups[zabbixHostGroup.Name] = struct{}{}
		}

		// Remove hostid because the Zabbix api add it automatically and it breaks the comparison between new/old hosts
		delete(zabbixHost.Inventory, "hostid")

		oldHost := p.AddHost(&CustomHost{
			State:      StateOld,
			Host:       zabbixHost,
			HostGroups: hostGroups,
			Items:      map[string]*CustomItem{},
			Triggers:   map[string]*CustomTrigger{},
		})
		log.Debugf("Load host from Zabbix: %+v", oldHost)

		zabbixItems, err := p.api.ItemsGet(zabbix.Params{
			"output":  "extend",
			"hostids": oldHost.Host.HostId,
		})
		if err != nil {
			return errors.Wrapf(err, "error getting item, hostid: %v", oldHost.Host.HostId)
		}

		for _, zabbixItem := range zabbixItems {
			newItem := &CustomItem{
				State: StateOld,
				Item:  zabbixItem,
			}

			log.Debugf("Loading item from Zabbix: %+v", newItem)
			oldHost.AddItem(newItem)
		}

		zabbixTriggers, err := p.api.TriggersGet(zabbix.Params{
			"output":           "extend",
			"hostids":          oldHost.Host.HostId,
			"expandExpression": true,
		})
		if err != nil {
			return errors.Wrapf(err, "error getting zabbix triggers, hostids: %v", oldHost.Host.HostId)
		}

		for _, zabbixTrigger := range zabbixTriggers {
			newTrigger := &CustomTrigger{
				State:   StateOld,
				Trigger: zabbixTrigger,
			}

			log.Debugf("Loading trigger from Zabbix: %+v", newTrigger)
			oldHost.AddTrigger(newTrigger)
		}
	}
	return nil
}

func (p *Provisioner) ApplyChanges() error {
	hostGroupsByState := p.GetHostGroupsByState()
	if len(hostGroupsByState[StateNew]) != 0 {
		log.Debugf("Creating HostGroups: %+v\n", hostGroupsByState[StateNew])
		err := p.api.HostGroupsCreate(hostGroupsByState[StateNew])
		if err != nil {
			return errors.Wrap(err, "Failed in creating hostgroups")
		}
	}

	// Make sure we update ids for the newly created host groups
	p.PropagateCreatedHostGroups(hostGroupsByState[StateNew])

	hostsByState := p.GetHostsByState()
	if len(hostsByState[StateNew]) != 0 {
		log.Debugf("Creating Hosts: %+v\n", hostsByState[StateNew])
		err := p.api.HostsCreate(hostsByState[StateNew])
		if err != nil {
			return errors.Wrap(err, "Failed in creating host")
		}
	}

	// Make sure we update ids for the newly created hosts
	p.PropagateCreatedHosts(hostsByState[StateNew])

	if len(hostsByState[StateUpdated]) != 0 {
		log.Debugf("Updating Hosts: %+v\n", hostsByState[StateUpdated])
		err := p.api.HostsUpdate(hostsByState[StateUpdated])
		if err != nil {
			return errors.Wrap(err, "Failed in updating host")
		}
	}

	for _, host := range p.Hosts {
		log.Debugf("Updating host, hostName: %s", host.Name)

		itemsByState := host.GetItemsByState()
		triggersByState := host.GetTriggersByState()

		if len(triggersByState[StateOld]) != 0 {
			log.Debugf("Deleting triggers: %+v\n", triggersByState[StateOld])
			err := p.api.TriggersDelete(triggersByState[StateOld])
			if err != nil {
				return errors.Wrap(err, "Failed in deleting triggers")
			}
		}

		if len(itemsByState[StateOld]) != 0 {
			log.Debugf("Deleting items: %+v\n", itemsByState[StateOld])
			err := p.api.ItemsDelete(itemsByState[StateOld])
			if err != nil {
				return errors.Wrap(err, "Failed in deleting items")
			}
		}

		if len(itemsByState[StateUpdated]) != 0 {
			log.Debugf("Updating items: %+v\n", itemsByState[StateUpdated])
			err := p.api.ItemsUpdate(itemsByState[StateUpdated])
			if err != nil {
				return errors.Wrap(err, "Failed in updating items")
			}
		}

		if len(triggersByState[StateUpdated]) != 0 {
			log.Debugf("Updating triggers: %+v\n", triggersByState[StateUpdated])
			err := p.api.TriggersUpdate(triggersByState[StateUpdated])
			if err != nil {
				return errors.Wrap(err, "Failed in updating triggers")
			}
		}

		if len(itemsByState[StateNew]) != 0 {
			log.Debugf("Creating items: %+v\n", itemsByState[StateNew])
			err := p.api.ItemsCreate(itemsByState[StateNew])
			if err != nil {
				return errors.Wrap(err, "Failed in creating items")
			}
		}

		if len(triggersByState[StateNew]) != 0 {
			log.Debugf("Creating triggers: %+v\n", triggersByState[StateNew])
			err := p.api.TriggersCreate(triggersByState[StateNew])
			if err != nil {
				return errors.Wrap(err, "Failed in creating triggers")
			}
		}
	}
	return nil
}
