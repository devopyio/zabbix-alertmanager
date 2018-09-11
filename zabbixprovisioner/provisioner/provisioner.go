package provisioner

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	zabbix "github.com/devopyio/zabbix-alertmanager/zabbixprovisioner/zabbixclient"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

type HostConfig struct {
	Name                    string   `yaml:"name"`
	HostGroups              []string `yaml:"hostGroups"`
	Tag                     string   `yaml:"tag"`
	DeploymentStatus        string   `yaml:"deploymentStatus"`
	ItemDefaultApplication  string   `yaml:"itemDefaultApplication"`
	ItemDefaultHistory      string   `yaml:"itemDefaultHistory"`
	ItemDefaultTrends       string   `yaml:"itemDefaultTrends"`
	ItemDefaultTrapperHosts string   `yaml:"itemDefaultTrapperHosts"`
}

type Provisioner struct {
	api                 *zabbix.API
	keyPrefix           string
	prometheusAlertPath string
	hosts               []HostConfig
	*CustomZabbix
}

func New(prometheusAlertPath, keyPrefix, url, user, password string, hosts []HostConfig) (*Provisioner, error) {
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
		api:                 api,
		keyPrefix:           keyPrefix,
		prometheusAlertPath: prometheusAlertPath,
		hosts:               hosts,
	}, nil
}

func LoadHostConfigFromFile(filename string) (cfg []HostConfig, err error) {
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the config file: %s", filename)
	}

	hosts := HostConfig{}

	err = yaml.Unmarshal(configFile, &hosts)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the config file: %s", filename)
	}

	return cfg, nil
}

func (p *Provisioner) Run() error {
	p.CustomZabbix = &CustomZabbix{
		Hosts:      map[string]*CustomHost{},
		HostGroups: map[string]*CustomHostGroup{},
	}

	if err := p.LoadRulesFromPrometheus(p.prometheusAlertPath); err != nil {
		return errors.Wrapf(err, "error loading prometheus rules, file: %s", p.prometheusAlertPath)
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
func (p *Provisioner) LoadRulesFromPrometheus(filename string) error {
	rules, err := LoadPrometheusRulesFromFile(filename)
	if err != nil {
		return errors.Wrap(err, "error loading rules")
	}

	for _, hostConfig := range p.hosts {
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
			HostGroups:   make(map[string]struct{}, len(hostConfig.HostGroups)),
			Items:        map[string]*CustomItem{},
			Applications: map[string]*CustomApplication{},
			Triggers:     map[string]*CustomTrigger{},
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
				Applications: map[string]struct{}{},
			}

			newTrigger := &CustomTrigger{
				State: StateNew,
				Trigger: zabbix.Trigger{
					Description: rule.Name,
					Expression:  fmt.Sprintf("{%s:%s.last()}<>0", newHost.Name, key),
				},
			}

			for k, v := range rule.Annotations {
				switch k {
				case "zabbix_applications":

					// List of applications separated by comma
					applicationNames := strings.Split(v, ",")
					for _, applicationName := range applicationNames {
						newApplication := &CustomApplication{
							State: StateNew,
							Application: zabbix.Application{
								Name: applicationName,
							},
						}

						newHost.AddApplication(newApplication)

						if _, ok := newItem.Applications[applicationName]; !ok {
							newItem.Applications[applicationName] = struct{}{}
						}
					}
				case "description":
					// If a specific description for this item is not present use the default prometheus description
					if _, ok := rule.Annotations["zabbix_description"]; !ok {
						newItem.Description = v
					}

					// If a specific description for this trigger is not present use the default prometheus description
					// Note that trigger "description" are called "comments" in the Zabbix api
					if _, ok := rule.Annotations["zabbix_trigger_description"]; !ok {
						newTrigger.Comments = v
					}
				case "zabbix_description":
					newItem.Description = v
				case "zabbix_history":
					newItem.History = v
				case "zabbix_trend":
					newItem.Trends = v
				case "zabbix_trapper_hosts":
					newItem.TrapperHosts = v
				case "summary":
					// Note that trigger "name" is called "description" in the Zabbix api
					if _, ok := rule.Annotations["zabbix_trigger_name"]; !ok {
						newTrigger.Description = v
					}
				case "zabbix_trigger_name":
					newTrigger.Description = v
				case "zabbix_trigger_description":
					newTrigger.Comments = v
				case "zabbix_trigger_severity":
					newTrigger.Priority = GetZabbixPriority(v)
				default:
					continue
				}
			}

			// If no applications are found in the rule, add the default application declared in the configuration
			if len(newItem.Applications) == 0 {
				newHost.AddApplication(&CustomApplication{
					State: StateNew,
					Application: zabbix.Application{
						Name: hostConfig.ItemDefaultApplication,
					},
				})
				newItem.Applications[hostConfig.ItemDefaultApplication] = struct{}{}
			}

			log.Debugf("Loading item from Prometheus: %+v", newItem)
			newHost.AddItem(newItem)

			log.Debugf("Loading trigger from Prometheus: %+v", newTrigger)
			newHost.AddTrigger(newTrigger)

			// Add the special "No Data" trigger if requested
			if delay, ok := rule.Annotations["zabbix_trigger_nodata"]; ok {
				noDataTrigger := &CustomTrigger{
					State:   StateNew,
					Trigger: newTrigger.Trigger,
				}

				noDataTrigger.Trigger.Description = fmt.Sprintf("%s - no data for the last %s seconds", newTrigger.Trigger.Description, delay)
				noDataTrigger.Trigger.Expression = fmt.Sprintf("{%s:%s.nodata(%s)}", newHost.Name, key, delay)
				log.Debugf("Trigger from Prometheus: %+v", noDataTrigger)
				newHost.AddTrigger(noDataTrigger)
			}
		}
		log.Debugf("Host from Prometheus: %+v", newHost)
		p.AddHost(newHost)
	}
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
			State:        StateOld,
			Host:         zabbixHost,
			HostGroups:   hostGroups,
			Items:        map[string]*CustomItem{},
			Applications: map[string]*CustomApplication{},
			Triggers:     map[string]*CustomTrigger{},
		})
		log.Debugf("Load host from Zabbix: %+v", oldHost)

		zabbixApplications, err := p.api.ApplicationsGet(zabbix.Params{
			"output":  "extend",
			"hostids": oldHost.HostId,
		})
		if err != nil {
			return errors.Wrapf(err, "error getting application, hostid: %v", oldHost.HostId)
		}

		for _, zabbixApplication := range zabbixApplications {
			oldHost.AddApplication(&CustomApplication{
				State:       StateOld,
				Application: zabbixApplication,
			})
		}

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

			zabbixApplications, err := p.api.ApplicationsGet(zabbix.Params{
				"output":  "extend",
				"itemids": zabbixItem.ItemId,
			})
			if err != nil {
				return errors.Wrapf(err, "error getting item, itemid: %v", oldHost.Host.HostId)
			}

			newItem.Applications = make(map[string]struct{}, len(zabbixApplications))
			for _, zabbixApplication := range zabbixApplications {
				newItem.Applications[zabbixApplication.Name] = struct{}{}
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

		applicationsByState := host.GetApplicationsByState()
		if len(applicationsByState[StateOld]) != 0 {
			log.Debugf("Deleting applications: %+v\n", applicationsByState[StateOld])
			err := p.api.ApplicationsDelete(applicationsByState[StateOld])
			if err != nil {
				return errors.Wrap(err, "Failed in deleting applications")
			}
		}

		if len(applicationsByState[StateNew]) != 0 {
			log.Debugf("Creating applications: %+v\n", applicationsByState[StateNew])
			err := p.api.ApplicationsCreate(applicationsByState[StateNew])
			if err != nil {
				return errors.Wrap(err, "Failed in creating applications")
			}
		}
		host.PropagateCreatedApplications(applicationsByState[StateNew])

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
