// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package redmine

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

//map[string]interface {}{
/*  "release":map[string]interface {}{
	"description":interface {}(nil),
	"id":64,
	"name":"Arvados 2.3.7",
	"planned_velocity":interface {}(nil),
	"project":map[string]interface {}{
		"id":36,
		"name":"Arvados"},
	"release_end_date":"2021-12-24",
	"release_start_date":"2021-11-26",
	"sharing":"hierarchy",
	"status":"open"}
} */
type Release struct {
	ID               int     `json:"id,omitempty"`
	Name             string  `json:"name,omitempty"`
	Description      string  `json:"description,omitempty"`
	Sharing          string  `json:"sharing,omitempty"`
	ReleaseStartDate string  `json:"release_start_date,omitempty"`
	ReleaseEndDate   string  `json:"release_end_date,omitempty"`
	PlannedVelocity  string  `json:"planned_velocity,omitempty"`
	Status           string  `json:"status,omitempty"`
	ProjectID        int     `json:"project_id,omitempty"`
	Project          *IDName `json:"project,omitempty"`
}

type releaseWrapper struct {
	Release Release `json:"release"`
}

// FindReleaseByName retrieves a redmine Release object by name
func (c *Client) FindReleaseByName(project, name string) (*Release, error) {
	// This api call only returns the first matching release object. There is no unique index on release names.
	res, err := c.Get("/rb/release/" + strings.ToLower(project) + "/find_by_name.json?name=" + url.QueryEscape(name))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, fmt.Errorf("Missing API call /rb/release/project_id/find_by_name.json")
	}
	var r releaseWrapper
	err = responseHelper(res, &r, 200)
	if err != nil {
		return nil, err
	}
	if r.Release.ID == 0 {
		return nil, nil
	}
	return &r.Release, nil
}

// FindReleaseByName retrieves a redmine Release object by name
func (c *Client) GetRelease(ID int) (*Release, error) {
	// This api call only returns the first matching release object. There is no unique index on release names.
	res, err := c.Get("/rb/release/" + strconv.Itoa(ID) + ".json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, fmt.Errorf("Not found")
	}
	var r releaseWrapper
	err = responseHelper(res, &r, 200)
	if err != nil {
		return nil, err
	}
	if r.Release.ID == 0 {
		return nil, nil
	}
	return &r.Release, nil
}

func (c *Client) CreateRelease(release Release) (*Release, error) {
	var rr releaseWrapper
	rr.Release = release
	s, err := json.Marshal(rr)
	if err != nil {
		return nil, err
	}
	res, err := c.Post("/rb/release/"+strings.ToLower(release.Project.Name)+"/new.json", string(s))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var r releaseWrapper
	err = responseHelper(res, &r, 201)
	if err != nil {
		return nil, err
	}
	return &r.Release, nil
}
