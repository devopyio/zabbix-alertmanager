package zabbix

import (
	"fmt"

	reflector "github.com/devopyio/zabsnd/zabbixprovisioner/zabbixutil"
)

type (
	ItemType  int
	ValueType int
	DataType  int
	DeltaType int
)

const (
	ZabbixAgent       ItemType = 0
	SNMPv1Agent       ItemType = 1
	ZabbixTrapper     ItemType = 2
	SimpleCheck       ItemType = 3
	SNMPv2Agent       ItemType = 4
	ZabbixInternal    ItemType = 5
	SNMPv3Agent       ItemType = 6
	ZabbixAgentActive ItemType = 7
	ZabbixAggregate   ItemType = 8
	WebItem           ItemType = 9
	ExternalCheck     ItemType = 10
	DatabaseMonitor   ItemType = 11
	IPMIAgent         ItemType = 12
	SSHAgent          ItemType = 13
	TELNETAgent       ItemType = 14
	Calculated        ItemType = 15
	JMXAgent          ItemType = 16

	Float     ValueType = 0
	Character ValueType = 1
	Log       ValueType = 2
	Unsigned  ValueType = 3
	Text      ValueType = 4

	Decimal     DataType = 0
	Octal       DataType = 1
	Hexadecimal DataType = 2
	Boolean     DataType = 3

	AsIs  DeltaType = 0
	Speed DeltaType = 1
	Delta DeltaType = 2
)

// https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/definitions
type Item struct {
	ItemId       string    `json:"itemid,omitempty"`
	Delay        string    `json:"delay"`
	HostId       string    `json:"hostid"`
	InterfaceId  string    `json:"interfaceid,omitempty"`
	Key          string    `json:"key_"`
	Name         string    `json:"name"`
	Type         ItemType  `json:"type"`
	ValueType    ValueType `json:"value_type"`
	DataType     DataType  `json:"data_type"`
	Delta        DeltaType `json:"delta"`
	Description  string    `json:"description"`
	Error        string    `json:"error"`
	History      string    `json:"history,omitempty"`
	Trends       string    `json:"trends,omitempty"`
	TrapperHosts string    `json:"trapper_hosts,omitempty"`

	ApplicationIds []string `json:"applications,omitempty"`
}

type Items []Item

// Converts slice to map by key. Panics if there are duplicate keys.
func (items Items) ByKey() map[string]Item {
	res := make(map[string]Item, len(items))
	for _, i := range items {
		_, present := res[i.Key]
		if present {
			panic(fmt.Errorf("Duplicate key %s", i.Key))
		}
		res[i.Key] = i
	}
	return res
}

// Wrapper for item.get https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/get
func (api *API) ItemsGet(params Params) (Items, error) {
	var res Items
	if _, present := params["output"]; !present {
		params["output"] = "extend"
	}
	response, err := api.CallWithError("item.get", params)
	if err != nil {
		return nil, err
	}

	reflector.MapsToStructs2(response.Result.([]interface{}), &res, reflector.Strconv, "json")
	return res, nil
}

// Gets items by application Id.
func (api *API) ItemsGetByApplicationId(id string) (res Items, err error) {
	return api.ItemsGet(Params{"applicationids": id})
}

// Wrapper for item.create: https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/create
func (api *API) ItemsCreate(items Items) error {
	response, err := api.CallWithError("item.create", items)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	itemids := result["itemids"].([]interface{})
	for i, id := range itemids {
		items[i].ItemId = id.(string)
	}
	return nil
}

// Wrapper for item.update: https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/update
func (api *API) ItemsUpdate(items Items) error {
	_, err := api.CallWithError("item.update", items)
	if err != nil {
		return err
	}
	return nil
}

// Wrapper for item.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/delete
// Cleans ItemId in all items elements if call succeed.
func (api *API) ItemsDelete(items Items) error {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ItemId
	}

	err := api.ItemsDeleteByIds(ids)
	if err == nil {
		return err
	}
	for i := range items {
		items[i].ItemId = ""
	}
	return nil
}

// Wrapper for item.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/delete
func (api *API) ItemsDeleteByIds(ids []string) error {
	response, err := api.CallWithError("item.delete", ids)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	itemids1, ok := result["itemids"].([]interface{})
	l := len(itemids1)
	if !ok {
		itemids2 := result["itemids"].(map[string]interface{})
		l = len(itemids2)
	}
	if len(ids) != l {
		err = &ExpectedMore{len(ids), l}
		return err
	}
	return nil
}
