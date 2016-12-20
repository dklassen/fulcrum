package main

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	goquery "github.com/google/go-querystring/query"
)

const (
	contentType     = "Content-Type"
	jsonContentType = "application/json"
	formContentType = "application/x-www-form-urlencoded"
)

type Requester interface {
	Do(*http.Request) (*http.Response, error)
}

type API struct {
	baseURL      *url.URL
	HTTPMethod   string
	client       Requester
	header       http.Header
	jsonDecoder  interface{}
	queryStructs []interface{}
}

type APIClient struct {
	http.Client
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (client *APIClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	return resp, err
}

func JSONDecoder(api *API) (jsonDecoder interface{}) {
	if jsonDecoder != nil {
		api.jsonDecoder = jsonDecoder
		api.Set(contentType, jsonContentType)
	}
	return api
}

func NewAPI() *API {
	return &API{
		client: &http.Client{},
	}
}

func (api *API) Receive(success, failure interface{}) (*http.Response, error) {
	req, err := api.Request()
	if err != nil {
		return nil, err
	}
	return api.Do(req, success, failure)
}

func (api *API) ReceiveSuccess(success interface{}) (*http.Response, error) {
	return api.Receive(success, nil)
}

func (api *API) Do(request *http.Request, success, failure interface{}) (*http.Response, error) {
	response, err := api.client.Do(request)
	if err != nil {
		return response, err
	}
	defer response.Body.Close()

	if strings.Contains(response.Header.Get(contentType), jsonContentType) {
		err = decodeResponseJSON(response, success, failure)
	}
	return response, err
}

func decodeResponseJSON(resp *http.Response, success, failure interface{}) error {
	if code := resp.StatusCode; 200 <= code && code <= 299 {
		if success != nil {
			return decodeResponseBodyJSON(resp, success)
		}
	} else {
		if failure != nil {
			return decodeResponseBodyJSON(resp, failure)
		}
	}
	return nil
}

func decodeResponseBodyJSON(resp *http.Response, v interface{}) error {
	return json.NewDecoder(resp.Body).Decode(v)
}

func (api *API) GET(path string) *API {
	api.HTTPMethod = "GET"
	api, err := api.Path(path)
	if err != nil {
		logrus.Fatal(err)
	}
	return api
}

func (api *API) Duplicate() *API {
	header := make(http.Header)
	for k, v := range api.header {
		header[k] = v
	}

	return &API{
		client:      api.client,
		header:      header,
		baseURL:     api.baseURL,
		jsonDecoder: api.jsonDecoder,
	}
}

func (api *API) BaseURL(path string) *API {
	var err error
	api.baseURL, err = url.Parse(path)
	if err != nil {
		log.Fatal(err)
	}
	return api
}

func (api *API) Set(key, value string) *API {
	api.header.Set(key, value)
	return api
}

func (api *API) Path(path string) (*API, error) {
	pathURL, pathErr := url.Parse(path)
	if pathURL != nil {
		return nil, pathErr
	}

	api.baseURL = api.baseURL.ResolveReference(pathURL)
	return api, nil
}

func (api *API) QueryStruct(queryStruct interface{}) *API {
	if queryStruct != nil {
		api.queryStructs = append(api.queryStructs, queryStruct)
	}
	return api
}

func addQueryStructs(reqURL *url.URL, queryStructs []interface{}) error {
	urlValues, err := url.ParseQuery(reqURL.RawQuery)
	if err != nil {
		return err
	}

	for _, queryStruct := range queryStructs {
		queryValues, err := goquery.Values(queryStruct)
		if err != nil {
			return err
		}
		for key, values := range queryValues {
			for _, value := range values {
				urlValues.Add(key, value)
			}
		}
	}

	reqURL.RawQuery = urlValues.Encode()
	return nil
}

func (api *API) Request() (*http.Request, error) {
	err := addQueryStructs(api.baseURL, api.queryStructs)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(api.HTTPMethod, api.baseURL.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header = api.header
	return req, err
}

func (api *API) SetBasicAuth(username, password string) *API {
	return api.Set("Authorization", "Basic "+basicAuth(username, password))
}
