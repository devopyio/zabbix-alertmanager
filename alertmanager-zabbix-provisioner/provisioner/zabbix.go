package provisioner

import (
	"strings"

	zabbix "github.com/devopyio/zabsnd/alertmanager-zabbix-provisioner/zabbixclient"
	log "github.com/sirupsen/logrus"
)

type State int

const (
	StateNew State = iota
	StateUpdated
	StateEqual
	StateOld
)

var StateName = map[State]string{
	StateNew:     "New",
	StateUpdated: "Updated",
	StateEqual:   "Equal",
	StateOld:     "Old",
}

type CustomApplication struct {
	State State
	zabbix.Application
}

type CustomTrigger struct {
	State State
	zabbix.Trigger
}

type CustomHostGroup struct {
	State State
	zabbix.HostGroup
}

type CustomItem struct {
	State State
	zabbix.Item
	Applications map[string]struct{}
}

type CustomHost struct {
	State State
	zabbix.Host
	HostGroups   map[string]struct{}
	Applications map[string]*CustomApplication
	Items        map[string]*CustomItem
	Triggers     map[string]*CustomTrigger
}

type CustomZabbix struct {
	Hosts      map[string]*CustomHost
	HostGroups map[string]*CustomHostGroup
}

func (z *CustomZabbix) AddHost(host *CustomHost) (updatedHost *CustomHost) {

	updatedHost = host

	if existing, ok := z.Hosts[host.Name]; ok {
		if existing.Equal(host) {
			if host.State == StateOld {
				existing.HostId = host.HostId
				existing.State = StateEqual
				updatedHost = existing
			}
		} else {
			if host.State == StateOld {
				existing.HostId = host.HostId
			}
			existing.State = StateUpdated
			updatedHost = existing
		}
	}

	z.Hosts[host.Name] = updatedHost
	return updatedHost
}

func (host *CustomHost) AddItem(item *CustomItem) {

	updatedItem := item

	if existing, ok := host.Items[item.Key]; ok {
		if existing.Equal(item) {
			if item.State == StateOld {
				existing.ItemId = item.ItemId
				existing.State = StateEqual
				updatedItem = existing
			}
		} else {
			if item.State == StateOld {
				existing.ItemId = item.ItemId
			}
			existing.State = StateUpdated
			updatedItem = existing
		}
	}

	host.Items[item.Key] = updatedItem
}

func (host *CustomHost) AddTrigger(trigger *CustomTrigger) {

	updatedTrigger := trigger

	if existing, ok := host.Triggers[trigger.Expression]; ok {
		if existing.Equal(trigger) {
			if trigger.State == StateOld {
				existing.TriggerId = trigger.TriggerId
				existing.State = StateEqual
				updatedTrigger = existing
			}
		} else {
			if trigger.State == StateOld {
				existing.TriggerId = trigger.TriggerId
			}
			existing.State = StateUpdated
			updatedTrigger = existing
		}
	}

	host.Triggers[trigger.Expression] = updatedTrigger
}

func (host *CustomHost) AddApplication(application *CustomApplication) {

	if _, ok := host.Applications[application.Name]; ok {
		if application.State == StateOld {
			application.State = StateEqual
		}
	}
	host.Applications[application.Name] = application
}

func (z *CustomZabbix) AddHostGroup(hostGroup *CustomHostGroup) {

	if _, ok := z.HostGroups[hostGroup.Name]; ok {
		if hostGroup.State == StateOld {
			hostGroup.State = StateEqual
		}
	}
	z.HostGroups[hostGroup.Name] = hostGroup
}

func (i *CustomHost) Equal(j *CustomHost) bool {
	if i.Name != j.Name {
		return false
	}

	if len(i.HostGroups) != len(j.HostGroups) {
		return false
	}

	for hostGroupName, _ := range i.HostGroups {
		if _, ok := j.HostGroups[hostGroupName]; !ok {
			return false
		}
	}

	if len(i.Inventory) != len(j.Inventory) {
		return false
	}

	for key, valueI := range i.Inventory {
		if valueJ, ok := j.Inventory[key]; !ok {
			return false
		} else if valueJ != valueI {
			return false
		}
	}

	return true
}

func (i *CustomItem) Equal(j *CustomItem) bool {
	if i.Name != j.Name {
		return false
	}

	if i.Description != j.Description {
		return false
	}

	if i.Trends != j.Trends {
		return false
	}

	if i.History != j.History {
		return false
	}

	if i.TrapperHosts != j.TrapperHosts {
		return false
	}

	if len(i.Applications) != len(j.Applications) {
		return false
	}

	for appName, _ := range i.Applications {
		if _, ok := j.Applications[appName]; !ok {
			return false
		}
	}

	return true
}

func (i *CustomTrigger) Equal(j *CustomTrigger) bool {
	if i.Expression != j.Expression {
		return false
	}

	if i.Description != j.Description {
		return false
	}

	if i.Priority != j.Priority {
		return false
	}

	if i.Comments != j.Comments {
		return false
	}

	return true
}

func (z *CustomZabbix) GetHostsByState() (hostByState map[State]zabbix.Hosts) {

	hostByState = map[State]zabbix.Hosts{
		StateNew:     zabbix.Hosts{},
		StateOld:     zabbix.Hosts{},
		StateUpdated: zabbix.Hosts{},
		StateEqual:   zabbix.Hosts{},
	}

	for _, host := range z.Hosts {
		for hostGroupName, _ := range host.HostGroups {
			host.GroupIds = append(host.GroupIds, zabbix.HostGroupId{GroupId: z.HostGroups[hostGroupName].GroupId})
		}
		hostByState[host.State] = append(hostByState[host.State], host.Host)
		log.Infof("GetHostByState = State: %s, Name: %s", StateName[host.State], host.Name)
	}

	return hostByState
}

func (z *CustomZabbix) GetHostGroupsByState() (hostGroupsByState map[State]zabbix.HostGroups) {

	hostGroupsByState = map[State]zabbix.HostGroups{
		StateNew:     zabbix.HostGroups{},
		StateOld:     zabbix.HostGroups{},
		StateUpdated: zabbix.HostGroups{},
		StateEqual:   zabbix.HostGroups{},
	}

	for _, hostGroup := range z.HostGroups {
		hostGroupsByState[hostGroup.State] = append(hostGroupsByState[hostGroup.State], hostGroup.HostGroup)
		log.Infof("GetHostGroupsByState = State: %s, Name: %s", StateName[hostGroup.State], hostGroup.Name)
	}

	return hostGroupsByState
}

func (zabbix *CustomZabbix) PropagateCreatedHosts(hosts zabbix.Hosts) {
	for _, newHost := range hosts {
		if host, ok := zabbix.Hosts[newHost.Name]; ok {
			host.HostId = newHost.HostId
		}
	}
}

func (zabbix *CustomZabbix) PropagateCreatedHostGroups(hostGroups zabbix.HostGroups) {
	for _, newHostGroup := range hostGroups {
		if hostGroup, ok := zabbix.HostGroups[newHostGroup.Name]; ok {
			hostGroup.GroupId = newHostGroup.GroupId
		}
	}
}

func (host *CustomHost) PropagateCreatedApplications(applications zabbix.Applications) {

	for _, application := range applications {
		host.Applications[application.Name].ApplicationId = application.ApplicationId
	}
}

func (host *CustomHost) GetItemsByState() (itemsByState map[State]zabbix.Items) {

	itemsByState = map[State]zabbix.Items{
		StateNew:     zabbix.Items{},
		StateOld:     zabbix.Items{},
		StateUpdated: zabbix.Items{},
		StateEqual:   zabbix.Items{},
	}

	for _, item := range host.Items {
		item.HostId = host.HostId
		item.Item.ApplicationIds = []string{}
		for appName, _ := range item.Applications {
			item.Item.ApplicationIds = append(item.Item.ApplicationIds, host.Applications[appName].ApplicationId)
		}
		itemsByState[item.State] = append(itemsByState[item.State], item.Item)
		log.Infof("GetItemsByState = State: %s, Key: %s, Applications: %+v", StateName[item.State], item.Key, item.Applications)
	}

	return itemsByState
}

func (host *CustomHost) GetTriggersByState() (triggersByState map[State]zabbix.Triggers) {

	triggersByState = map[State]zabbix.Triggers{
		StateNew:     zabbix.Triggers{},
		StateOld:     zabbix.Triggers{},
		StateUpdated: zabbix.Triggers{},
		StateEqual:   zabbix.Triggers{},
	}

	for _, trigger := range host.Triggers {
		triggersByState[trigger.State] = append(triggersByState[trigger.State], trigger.Trigger)
		log.Infof("GetTriggersByState = State: %s, Expression: %s", StateName[trigger.State], trigger.Expression)
	}

	return triggersByState
}

func (host *CustomHost) GetApplicationsByState() (applicationsByState map[State]zabbix.Applications) {

	applicationsByState = map[State]zabbix.Applications{
		StateNew:     zabbix.Applications{},
		StateOld:     zabbix.Applications{},
		StateUpdated: zabbix.Applications{},
		StateEqual:   zabbix.Applications{},
	}

	for _, application := range host.Applications {
		application.Application.HostId = host.HostId
		applicationsByState[application.State] = append(applicationsByState[application.State], application.Application)
		log.Infof("GetApplicationsByState = State: %s, Name: %s", StateName[application.State], application.Name)
	}

	return applicationsByState
}

func GetZabbixPriority(severity string) zabbix.PriorityType {

	switch strings.ToLower(severity) {
	case "information":
		return zabbix.Information
	case "warning":
		return zabbix.Warning
	case "average":
		return zabbix.Average
	case "high":
		return zabbix.High
	case "critical":
		return zabbix.Critical
	default:
		return zabbix.NotClassified
	}
}
