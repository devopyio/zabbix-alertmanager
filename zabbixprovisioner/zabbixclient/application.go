package zabbix

import (
	reflector "github.com/Dexanir/zabbix-alertmanager/zabbixprovisioner/zabbixutil"
)

// https://www.zabbix.com/documentation/2.2/manual/appendix/api/application/definitions
type Application struct {
	ApplicationId string `json:"applicationid,omitempty"`
	HostId        string `json:"hostid"`
	Name          string `json:"name"`
	TemplateId    string `json:"templateid,omitempty"`
}

type Applications []Application

// Wrapper for application.get: https://www.zabbix.com/documentation/2.2/manual/appendix/api/application/get
func (api *API) ApplicationsGet(params Params) (Applications, error) {
	var res Applications
	if _, present := params["output"]; !present {
		params["output"] = "extend"
	}
	response, err := api.CallWithError("application.get", params)
	if err != nil {
		return nil, err
	}

	reflector.MapsToStructs2(response.Result.([]interface{}), &res, reflector.Strconv, "json")
	return res, nil
}

// Gets application by Id only if there is exactly 1 matching application.
func (api *API) ApplicationGetById(id string) (*Application, error) {
	var res *Application
	apps, err := api.ApplicationsGet(Params{"applicationids": id})
	if err != nil {
		return nil, err
	}

	if len(apps) == 1 {
		res = &apps[0]
	} else {
		e := ExpectedOneResult(len(apps))
		err = &e
		return nil, err
	}
	return res, nil
}

// Gets application by host Id and name only if there is exactly 1 matching application.
func (api *API) ApplicationGetByHostIdAndName(hostId, name string) (*Application, error) {
	var res *Application
	apps, err := api.ApplicationsGet(Params{"hostids": hostId, "filter": map[string]string{"name": name}})
	if err != nil {
		return nil, err
	}

	if len(apps) == 1 {
		res = &apps[0]
	} else {
		e := ExpectedOneResult(len(apps))
		err = &e
		return nil, err
	}
	return res, nil
}

// Wrapper for application.create: https://www.zabbix.com/documentation/2.2/manual/appendix/api/application/create
func (api *API) ApplicationsCreate(apps Applications) error {
	response, err := api.CallWithError("application.create", apps)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	applicationids := result["applicationids"].([]interface{})
	for i, id := range applicationids {
		apps[i].ApplicationId = id.(string)
	}
	return nil
}

// Wrapper for application.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/application/delete
// Cleans ApplicationId in all apps elements if call succeed.
func (api *API) ApplicationsDelete(apps Applications) error {
	ids := make([]string, len(apps))
	for i, app := range apps {
		ids[i] = app.ApplicationId
	}

	err := api.ApplicationsDeleteByIds(ids)
	if err != nil {
		return err
	}

	for i := range apps {
		apps[i].ApplicationId = ""
	}
	return nil

}

// Wrapper for application.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/application/delete
func (api *API) ApplicationsDeleteByIds(ids []string) error {
	response, err := api.CallWithError("application.delete", ids)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	applicationids := result["applicationids"].([]interface{})
	if len(ids) != len(applicationids) {
		err = &ExpectedMore{len(ids), len(applicationids)}
		return err
	}
	return nil
}
