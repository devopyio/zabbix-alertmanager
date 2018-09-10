package provisioner

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	zabbix "github.com/devopyio/zabsnd/zabbixprovisioner/zabbixclient"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

const (
	rulePollingInterval = 3600
)

type Provisioner struct {
	Api    *zabbix.API
	Config ProvisionerConfig
	*CustomZabbix
}

type ProvisionerConfig struct {
	ZabbixApiUrl      string       `yaml:"zabbixApiUrl"`
	ZabbixApiUser     string       `yaml:"zabbixApiUser"`
	ZabbixApiPassword string       `yaml:"zabbixApiPassword"`
	ZabbixKeyPrefix   string       `yaml:"zabbixKeyPrefix"`
	ZabbixHosts       []HostConfig `yaml:"zabbixHosts"`
}

type HostConfig struct {
	Name                    string            `yaml:"name"`
	Selector                map[string]string `yaml:"selector"`
	HostGroups              []string          `yaml:"hostGroups"`
	Tag                     string            `yaml:"tag"`
	DeploymentStatus        string            `yaml:"deploymentStatus"`
	ItemDefaultApplication  string            `yaml:"itemDefaultApplication"`
	ItemDefaultHistory      string            `yaml:"itemDefaultHistory"`
	ItemDefaultTrends       string            `yaml:"itemDefaultTrends"`
	ItemDefaultTrapperHosts string            `yaml:"itemDefaultTrapperHosts"`
}

func New(cfg *ProvisionerConfig) *Provisioner {
	transport := http.DefaultTransport

	api := zabbix.NewAPI(cfg.ZabbixApiUrl)
	api.SetClient(&http.Client{
		Transport: transport,
	})

	auth, err := api.Login(cfg.ZabbixApiUser, cfg.ZabbixApiPassword)
	if err != nil {
		log.Fatalln("error while login to Zabbix:", err)
	}
	log.Info(auth)

	return &Provisioner{
		Api:    api,
		Config: *cfg,
	}

}

func LoadFromFile(filename string) (cfg *ProvisionerConfig, err error) {
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the config file: %s")
	}

	// Default values
	config := ProvisionerConfig{
		ZabbixApiUrl:      "https://127.0.0.1/zabbix/api_jsonrpc.php",
		ZabbixApiUser:     "Admin",
		ZabbixApiPassword: "zabbix",
		ZabbixKeyPrefix:   "prometheus",
		ZabbixHosts:       []HostConfig{},
	}

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the config file: %s")
	}

	log.Info("configuration loaded")

	// If Environment variables are set for zabbix user and password, use those instead
	zabbixApiUser, ok := os.LookupEnv("ZABBIX_API_USER")
	if ok {
		config.ZabbixApiUser = zabbixApiUser
	}

	zabbixApiPassword, ok := os.LookupEnv("ZABBIX_API_PASSWORD")
	if ok {
		config.ZabbixApiPassword = zabbixApiPassword
	}

	return &config, nil
}

func (p *Provisioner) Start(filename string) {
	for {
		p.CustomZabbix = &CustomZabbix{
			Hosts:      map[string]*CustomHost{},
			HostGroups: map[string]*CustomHostGroup{},
		}

		p.FillFromPrometheus(filename)
		p.FillFromZabbix()
		p.ApplyChanges()

		time.Sleep(rulePollingInterval * time.Second)
	}
}

// Create hosts structures and populate them from Prometheus rules
func (p *Provisioner) FillFromPrometheus(filename string) error {
	rules, err := GetRulesFromFile(filename)
	if err != nil {
		return err
	}

	for _, hostConfig := range p.Config.ZabbixHosts {

		// Create an internal host object
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

		// Create host groups from the configuration file and link them to this host
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

			key := fmt.Sprintf("prometheus.%s", strings.ToLower(rule.Name))

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
					// Note that trigger "description" are called "comments" in the Zabbix API
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
					// Note that trigger "name" is called "description" in the Zabbix API
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

			log.Debugf("Item from Prometheus: %+v", newItem)
			newHost.AddItem(newItem)

			log.Debugf("Trigger from Prometheus: %+v", newTrigger)
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
func (p *Provisioner) FillFromZabbix() error {
	hostNames := make([]string, len(p.Config.ZabbixHosts))
	hostGroupNames := []string{}
	for i, _ := range p.Config.ZabbixHosts {
		hostNames[i] = p.Config.ZabbixHosts[i].Name
		hostGroupNames = append(hostGroupNames, p.Config.ZabbixHosts[i].HostGroups...)
	}

	zabbixHostGroups, err := p.Api.HostGroupsGet(zabbix.Params{
		"output": "extend",
		"filter": map[string][]string{
			"name": hostGroupNames,
		},
	})
	if err != nil {
		return err
	}

	for _, zabbixHostGroup := range zabbixHostGroups {
		p.AddHostGroup(&CustomHostGroup{
			State:     StateOld,
			HostGroup: zabbixHostGroup,
		})
	}

	zabbixHosts, err := p.Api.HostsGet(zabbix.Params{
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
		return err
	}

	for _, zabbixHost := range zabbixHosts {
		zabbixHostGroups, err := p.Api.HostGroupsGet(zabbix.Params{
			"output":  "extend",
			"hostids": zabbixHost.HostId,
		})
		if err != nil {
			return err
		}

		hostGroups := make(map[string]struct{}, len(zabbixHostGroups))
		for _, zabbixHostGroup := range zabbixHostGroups {
			hostGroups[zabbixHostGroup.Name] = struct{}{}
		}

		// Remove hostid because the Zabbix API add it automatically and it breaks the comparison between new/old hosts
		delete(zabbixHost.Inventory, "hostid")

		oldHost := p.AddHost(&CustomHost{
			State:        StateOld,
			Host:         zabbixHost,
			HostGroups:   hostGroups,
			Items:        map[string]*CustomItem{},
			Applications: map[string]*CustomApplication{},
			Triggers:     map[string]*CustomTrigger{},
		})
		log.Debugf("Host from Zabbix: %+v", oldHost)

		zabbixApplications, err := p.Api.ApplicationsGet(zabbix.Params{
			"output":  "extend",
			"hostids": oldHost.HostId,
		})
		if err != nil {
			return err
		}

		for _, zabbixApplication := range zabbixApplications {
			oldHost.AddApplication(&CustomApplication{
				State:       StateOld,
				Application: zabbixApplication,
			})
		}

		zabbixItems, err := p.Api.ItemsGet(zabbix.Params{
			"output":  "extend",
			"hostids": oldHost.Host.HostId,
		})
		if err != nil {
			return err
		}

		for _, zabbixItem := range zabbixItems {

			newItem := &CustomItem{
				State: StateOld,
				Item:  zabbixItem,
			}

			zabbixApplications, err := p.Api.ApplicationsGet(zabbix.Params{
				"output":  "extend",
				"itemids": zabbixItem.ItemId,
			})
			if err != nil {
				return err
			}

			newItem.Applications = make(map[string]struct{}, len(zabbixApplications))
			for _, zabbixApplication := range zabbixApplications {
				newItem.Applications[zabbixApplication.Name] = struct{}{}
			}

			log.Debugf("Item from Zabbix: %+v", newItem)
			oldHost.AddItem(newItem)
		}

		zabbixTriggers, err := p.Api.TriggersGet(zabbix.Params{
			"output":           "extend",
			"hostids":          oldHost.Host.HostId,
			"expandExpression": true,
		})
		if err != nil {
			return err
		}

		for _, zabbixTrigger := range zabbixTriggers {
			newTrigger := &CustomTrigger{
				State:   StateOld,
				Trigger: zabbixTrigger,
			}

			log.Debugf("Triggers from Zabbix: %+v", newTrigger)
			oldHost.AddTrigger(newTrigger)
		}
	}
	return nil
}

func (p *Provisioner) ApplyChanges() error {
	hostGroupsByState := p.GetHostGroupsByState()
	if len(hostGroupsByState[StateNew]) != 0 {
		log.Debugf("Creating HostGroups: %+v\n", hostGroupsByState[StateNew])
		err := p.Api.HostGroupsCreate(hostGroupsByState[StateNew])
		if err != nil {
			return errors.Wrap(err, "Failed in creating hostgroups")
		}
	}

	// Make sure we update ids for the newly created host groups
	p.PropagateCreatedHostGroups(hostGroupsByState[StateNew])

	hostsByState := p.GetHostsByState()
	if len(hostsByState[StateNew]) != 0 {
		log.Debugf("Creating Hosts: %+v\n", hostsByState[StateNew])
		err := p.Api.HostsCreate(hostsByState[StateNew])
		if err != nil {
			return errors.Wrap(err, "Failed in creating host")
		}
	}

	// Make sure we update ids for the newly created hosts
	p.PropagateCreatedHosts(hostsByState[StateNew])

	if len(hostsByState[StateUpdated]) != 0 {
		log.Debugf("Updating Hosts: %+v\n", hostsByState[StateUpdated])
		err := p.Api.HostsUpdate(hostsByState[StateUpdated])
		if err != nil {
			return errors.Wrap(err, "Failed in updating host")
		}
	}

	for _, host := range p.Hosts {
		log.Info("Updating host:", host.Name)

		applicationsByState := host.GetApplicationsByState()
		if len(applicationsByState[StateOld]) != 0 {
			log.Debugf("Deleting applications: %+v\n", applicationsByState[StateOld])
			err := p.Api.ApplicationsDelete(applicationsByState[StateOld])
			if err != nil {
				return errors.Wrap(err, "Failed in deleting applications")
			}
		}

		if len(applicationsByState[StateNew]) != 0 {
			log.Debugf("Creating applications: %+v\n", applicationsByState[StateNew])
			err := p.Api.ApplicationsCreate(applicationsByState[StateNew])
			if err != nil {
				return errors.Wrap(err, "Failed in creating applications")
			}
		}
		host.PropagateCreatedApplications(applicationsByState[StateNew])

		itemsByState := host.GetItemsByState()
		triggersByState := host.GetTriggersByState()

		if len(triggersByState[StateOld]) != 0 {
			log.Debugf("Deleting triggers: %+v\n", triggersByState[StateOld])
			err := p.Api.TriggersDelete(triggersByState[StateOld])
			if err != nil {
				return errors.Wrap(err, "Failed in deleting triggers")
			}
		}

		if len(itemsByState[StateOld]) != 0 {
			log.Debugf("Deleting items: %+v\n", itemsByState[StateOld])
			err := p.Api.ItemsDelete(itemsByState[StateOld])
			if err != nil {
				return errors.Wrap(err, "Failed in deleting items")
			}
		}

		if len(itemsByState[StateUpdated]) != 0 {
			log.Debugf("Updating items: %+v\n", itemsByState[StateUpdated])
			err := p.Api.ItemsUpdate(itemsByState[StateUpdated])
			if err != nil {
				return errors.Wrap(err, "Failed in updating items")
			}
		}

		if len(triggersByState[StateUpdated]) != 0 {
			log.Debugf("Updating triggers: %+v\n", triggersByState[StateUpdated])
			err := p.Api.TriggersUpdate(triggersByState[StateUpdated])
			if err != nil {
				return errors.Wrap(err, "Failed in updating triggers")
			}
		}

		if len(itemsByState[StateNew]) != 0 {
			log.Debugf("Creating items: %+v\n", itemsByState[StateNew])
			err := p.Api.ItemsCreate(itemsByState[StateNew])
			if err != nil {
				return errors.Wrap(err, "Failed in creating items")
			}
		}

		if len(triggersByState[StateNew]) != 0 {
			log.Debugf("Creating triggers: %+v\n", triggersByState[StateNew])
			err := p.Api.TriggersCreate(triggersByState[StateNew])
			if err != nil {
				return errors.Wrap(err, "Failed in creating triggers")
			}
		}
	}
	return nil

}
