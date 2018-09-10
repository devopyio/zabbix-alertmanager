package zabbix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync/atomic"
)

type (
	Params map[string]interface{}
)

type request struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Auth    string      `json:"auth,omitempty"`
	Id      int32       `json:"id"`
}

type Response struct {
	Jsonrpc string      `json:"jsonrpc"`
	Error   *Error      `json:"error"`
	Result  interface{} `json:"result"`
	Id      int32       `json:"id"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d (%s): %s", e.Code, e.Message, e.Data)
}

type ExpectedOneResult int

func (e *ExpectedOneResult) Error() string {
	return fmt.Sprintf("Expected exactly one result, got %d.", *e)
}

type ExpectedMore struct {
	Expected int
	Got      int
}

func (e *ExpectedMore) Error() string {
	return fmt.Sprintf("Expected %d, got %d.", e.Expected, e.Got)
}

type API struct {
	Auth   string      // auth token, filled by Login()
	Logger *log.Logger // request/response logger, nil by default
	url    string
	c      http.Client
	id     int32
}

// Creates new API access object.
// Typical URL is http://host/api_jsonrpc.php or http://host/zabbix/api_jsonrpc.php.
// It also may contain HTTP basic auth username and password like
// http://username:password@host/api_jsonrpc.php.
func NewAPI(url string) (api *API) {
	return &API{url: url, c: http.Client{}}
}

// Allows one to use specific http.Client, for example with InsecureSkipVerify transport.
func (api *API) SetClient(c *http.Client) {
	api.c = *c
}

func (api *API) printf(format string, v ...interface{}) {
	if api.Logger != nil {
		api.Logger.Printf(format, v...)
	}
}

func (api *API) callBytes(method string, params interface{}) ([]byte, error) {
	id := atomic.AddInt32(&api.id, 1)
	jsonobj := request{"2.0", method, params, api.Auth, id}
	b, err := json.Marshal(jsonobj)
	if err != nil {
		return nil, err
	}
	api.printf("Request (POST): %s", b)

	req, err := http.NewRequest("POST", api.url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(b))
	req.Header.Add("Content-Type", "application/json-rpc")

	res, err := api.c.Do(req)
	if err != nil {
		api.printf("Error   : %s", err)
		return nil, err
	}
	defer res.Body.Close()

	b, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	api.printf("Response (%d): %s", res.StatusCode, b)
	return b, nil
}

// Calls specified API method. Uses api.Auth if not empty.
// err is something network or marshaling related. Caller should inspect response.Error to get API error.
func (api *API) Call(method string, params interface{}) (Response, error) {
	var response Response
	b, err := api.callBytes(method, params)
	if err != nil {
		return response, err
	}
	err = json.Unmarshal(b, &response)
	if err != nil {
		return response, err
	}
	return response, nil
}

// Uses Call() and then sets err to response.Error if former is nil and latter is not.
func (api *API) CallWithError(method string, params interface{}) (Response, error) {
	response, err := api.Call(method, params)
	if err == nil && response.Error != nil {
		err = response.Error
	}
	return response, err
}

// Calls "user.login" API method and fills api.Auth field.
// This method modifies API structure and should not be called concurrently with other methods.
func (api *API) Login(user, password string) (string, error) {
	params := map[string]string{"user": user, "password": password}
	response, err := api.CallWithError("user.login", params)
	if err != nil {
		return "", err
	}

	auth := response.Result.(string)
	api.Auth = auth
	return auth, nil
}

// Calls "APIInfo.version" API method.
// This method temporary modifies API structure and should not be called concurrently with other methods.
func (api *API) Version() (string, error) {
	// temporary remove auth for this method to succeed
	// https://www.zabbix.com/documentation/2.2/manual/appendix/api/apiinfo/version
	auth := api.Auth
	api.Auth = ""
	response, err := api.CallWithError("APIInfo.version", Params{})
	if err != nil {
		return "", err
	}
	api.Auth = auth

	// despite what documentation says, Zabbix 2.2 requires auth, so we try again
	if e, ok := err.(*Error); ok && e.Code == -32602 {
		response, err = api.CallWithError("APIInfo.version", Params{})
	}
	if err != nil {
		return "", err
	}

	v := response.Result.(string)
	return v, nil
}
