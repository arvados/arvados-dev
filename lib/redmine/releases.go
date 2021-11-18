// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package redmine

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type Release struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	Sharing          string  `json:"sharing"`
	ReleaseStartDate string  `json:"release_start_date"`
	ReleaseEndDate   string  `json:"release_end_date"`
	PlannedVelocity  string  `json:"planned_velocity"`
	Status           string  `json:"status"`
	ProjectID        int     `json:"-"`
	Project          *IDName `json:"-"`
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
	err = responseHelper(res, r, 200)
	if err != nil {
		return nil, err
	}
	return &r.Release, nil
}
