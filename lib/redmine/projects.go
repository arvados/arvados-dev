// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Somewhat inspired by https://github.com/mattn/go-redmine (MIT licensed)

package redmine

import (
	"strconv"
)

type projectWrapper struct {
	Project Project `json:"project"`
}

type projectsResult struct {
	Projects []Project `json:"projects"`
}

type Project struct {
	ID          int    `json:"id"`
	Parent      IDName `json:"parent"`
	Name        string `json:"name"`
	IDentifier  string `json:"identifier"`
	Description string `json:"description"`
	CreatedOn   string `json:"created_on"`
	UpdatedOn   string `json:"updated_on"`
}

func (c *Client) GetProject(id int) (*Project, error) {
	res, err := c.Get("/projects/" + strconv.Itoa(id) + ".json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var r projectWrapper
	err = responseHelper(res, &r, 200)
	if err != nil {
		return nil, err
	}
	return &r.Project, nil
}
