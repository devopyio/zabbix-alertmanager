package zabbix

import (
	reflector "github.com/devopyio/zabsnd/alertmanager-zabbix-provisioner/zabbixutil"
)

type (
	PriorityType int
)

const (
	NotClassified PriorityType = 0
	Information   PriorityType = 1
	Warning       PriorityType = 2
	Average       PriorityType = 3
	High          PriorityType = 4
	Critical      PriorityType = 5

	Enabled  StatusType = 0
	Disabled StatusType = 1

	OK      ValueType = 0
	Problem ValueType = 1
)

// https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/definitions
type Trigger struct {
	TriggerId   string `json:"triggerid,omitempty"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
	Comments    string `json:"comments"`
	//TemplateId  string    `json:"templateid"`
	Value ValueType `json:""`

	Priority PriorityType `json:"priority"`
	Status   StatusType   `json:"status"`
}

type Triggers []Trigger

// Wrapper for item.get https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/get
func (api *API) TriggersGet(params Params) (Triggers, error) {
	var res Triggers
	if _, present := params["output"]; !present {
		params["output"] = "extend"
	}
	response, err := api.CallWithError("trigger.get", params)
	if err != nil {
		return nil, err
	}

	reflector.MapsToStructs2(response.Result.([]interface{}), &res, reflector.Strconv, "json")
	return res, nil
}

// Wrapper for item.create: https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/create
func (api *API) TriggersCreate(triggers Triggers) error {
	response, err := api.CallWithError("trigger.create", triggers)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	triggerids := result["triggerids"].([]interface{})
	for i, id := range triggerids {
		triggers[i].TriggerId = id.(string)
	}
	return nil
}

// Wrapper for item.update: https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/update
func (api *API) TriggersUpdate(triggers Triggers) error {
	_, err := api.CallWithError("trigger.update", triggers)
	if err != nil {
		return err
	}
	return nil
}

// Wrapper for item.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/delete
// Cleans ItemId in all items elements if call succeed.
func (api *API) TriggersDelete(triggers Triggers) error {
	ids := make([]string, len(triggers))
	for i, trigger := range triggers {
		ids[i] = trigger.TriggerId
	}

	err := api.TriggersDeleteByIds(ids)
	if err != nil {
		return err
	}

	for i := range triggers {
		triggers[i].TriggerId = ""
	}

	return nil
}

// Wrapper for item.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/delete
func (api *API) TriggersDeleteByIds(ids []string) error {
	response, err := api.CallWithError("trigger.delete", ids)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	triggerids1, ok := result["triggerids"].([]interface{})
	l := len(triggerids1)
	if !ok {
		// some versions actually return map there
		triggerids2 := result["triggerids"].(map[string]interface{})
		l = len(triggerids2)
	}
	if len(ids) != l {
		err = &ExpectedMore{len(ids), l}
		return err
	}
	return nil
}
