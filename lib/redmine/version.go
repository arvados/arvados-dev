// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package redmine

import (
	"errors"
	"strconv"
)

type versionWrapper struct {
	Version Version `json:"version"`
}

type versionsResult struct {
	Versions []Version `json:"versions"`
}

type Version struct {
	ID          int    `json:"id"`
	Project     IDName `json:"project"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	DueDate     string `json:"due_date"`
	CreatedOn   string `json:"created_on"`
	UpdatedOn   string `json:"updated_on"`
}

func (c *Client) Version(id int) (*Version, error) {
	res, err := c.Get("/versions/" + strconv.Itoa(id) + ".json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, errors.New("Not Found")
	}

	var r versionWrapper
	err = responseHelper(res, &r, 200)
	if err != nil {
		return nil, err
	}
	return &r.Version, nil
}

func (c *Client) Versions(projectId int) ([]Version, error) {
	res, err := c.Get("/projects/" + strconv.Itoa(projectId) + "/versions.json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var r versionsResult
	err = responseHelper(res, &r, 200)
	if err != nil {
		return nil, err
	}
	return r.Versions, nil
}
