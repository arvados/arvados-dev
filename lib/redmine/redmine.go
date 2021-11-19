// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Somewhat inspired by https://github.com/mattn/go-redmine (MIT licensed)

package redmine

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Client struct {
	endpoint string
	apikey   string
	*http.Client
}

type errorsResult struct {
	Errors []string `json:"errors"`
}

type IDName struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ID struct {
	ID int `json:"id"`
}

func NewClient(endpoint, apikey string) *Client {
	return &Client{endpoint, apikey, http.DefaultClient}
}

func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.endpoint+url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-Redmine-API-Key", c.apikey)
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return res, err
}

func (c *Client) Post(url string, payload string) (*http.Response, error) {
	req, err := http.NewRequest("POST", c.endpoint+url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("X-Redmine-API-Key", c.apikey)
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return res, err
}

func (c *Client) Put(url string, payload string) (*http.Response, error) {
	req, err := http.NewRequest("PUT", c.endpoint+url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("X-Redmine-API-Key", c.apikey)
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return res, err
}

func responseHelper(res *http.Response, r interface{}, okCode int) error {
	var err error
	decoder := json.NewDecoder(res.Body)
	if res.StatusCode != okCode {
		var result errorsResult
		err = decoder.Decode(&result)
		if err == nil {
			err = errors.New(strings.Join(result.Errors, "\n"))
		} else if err.Error() == "EOF" {
			// The body is empty, just return the status code as an error. This is
			// an error because res.StatusCode != okCode.
			err = fmt.Errorf("[error] %s", res.Status)
		}
	} else if r != nil {
		// When r is nil, the API call is not expected to return a result (empty res.Body)
		err = decoder.Decode(&r)
	}
	return err
}
