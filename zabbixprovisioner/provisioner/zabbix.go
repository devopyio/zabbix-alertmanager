package provisioner

import (
	"strings"

	zabbix "github.com/Dexanir/zabbix-alertmanager/zabbixprovisioner/zabbixclient"
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
}

type CustomHost struct {
	State State
	zabbix.Host
	HostGroups map[string]struct{}
	Items      map[string]*CustomItem
	Triggers   map[string]*CustomTrigger
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

	if i.URL != j.URL {
		return false
	}

	if i.ManualClose != j.ManualClose {
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

	newHostAmmount := 0
	for _, host := range z.Hosts {
		for hostGroupName, _ := range host.HostGroups {
			host.GroupIds = append(host.GroupIds, zabbix.HostGroupId{GroupId: z.HostGroups[hostGroupName].GroupId})
		}
		hostByState[host.State] = append(hostByState[host.State], host.Host)
		if StateName[host.State] == "New" || StateName[host.State] == "Updated" {
			newHostAmmount++
			log.Infof("GetHostByState = State: %s, Name: %s", StateName[host.State], host.Name)
		} else {
			log.Debugf("GetHostByState = State: %s, Name: %s", StateName[host.State], host.Name)
		}
	}

	log.Infof("HOSTS, total: %v, new or updated: %v", len(z.Hosts), newHostAmmount)
	return hostByState
}

func (z *CustomZabbix) GetHostGroupsByState() (hostGroupsByState map[State]zabbix.HostGroups) {

	hostGroupsByState = map[State]zabbix.HostGroups{
		StateNew:     zabbix.HostGroups{},
		StateOld:     zabbix.HostGroups{},
		StateUpdated: zabbix.HostGroups{},
		StateEqual:   zabbix.HostGroups{},
	}

	newHostGroupAmmount := 0
	for _, hostGroup := range z.HostGroups {
		hostGroupsByState[hostGroup.State] = append(hostGroupsByState[hostGroup.State], hostGroup.HostGroup)
		if StateName[hostGroup.State] == "New" || StateName[hostGroup.State] == "Updated" {
			newHostGroupAmmount++
			log.Infof("GetHostGroupsByState = State: %s, Name: %s", StateName[hostGroup.State], hostGroup.Name)
		} else {
			log.Debugf("GetHostGroupsByState = State: %s, Name: %s", StateName[hostGroup.State], hostGroup.Name)
		}
	}

	log.Infof("HOSTGROUPS, total: %v, new or updated: %v", len(z.HostGroups), newHostGroupAmmount)

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

func (host *CustomHost) GetItemsByState() (itemsByState map[State]zabbix.Items) {

	itemsByState = map[State]zabbix.Items{
		StateNew:     zabbix.Items{},
		StateOld:     zabbix.Items{},
		StateUpdated: zabbix.Items{},
		StateEqual:   zabbix.Items{},
	}

	newItemAmmount := 0
	for _, item := range host.Items {
		item.HostId = host.HostId
		itemsByState[item.State] = append(itemsByState[item.State], item.Item)
		if StateName[item.State] == "New" || StateName[item.State] == "Updated" {
			newItemAmmount++
			log.Infof("GetItemsByState = State: %s, Key: %s", StateName[item.State], item.Key)
		} else {
			log.Debugf("GetItemsByState = State: %s, Key: %s", StateName[item.State], item.Key)
		}
	}

	log.Infof("ITEMS, total: %v, new or updated: %v", len(host.Items), newItemAmmount)
	return itemsByState
}

func (host *CustomHost) GetTriggersByState() (triggersByState map[State]zabbix.Triggers) {

	triggersByState = map[State]zabbix.Triggers{
		StateNew:     zabbix.Triggers{},
		StateOld:     zabbix.Triggers{},
		StateUpdated: zabbix.Triggers{},
		StateEqual:   zabbix.Triggers{},
	}

	newTriggerAmmount := 0
	for _, trigger := range host.Triggers {
		triggersByState[trigger.State] = append(triggersByState[trigger.State], trigger.Trigger)
		if StateName[trigger.State] == "New" || StateName[trigger.State] == "Updated" {
			newTriggerAmmount++
			log.Infof("GetTriggersByState = State: %s, Expression: %s", StateName[trigger.State], trigger.Expression)
		} else {
			log.Debugf("GetTriggersByState = State: %s, Expression: %s", StateName[trigger.State], trigger.Expression)
		}
	}

	log.Infof("TRIGGERS, total: %v, new or updated: %v", len(host.Triggers), newTriggerAmmount)
	return triggersByState
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
