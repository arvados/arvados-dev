// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package redmine

import (
	"errors"
	"strconv"
)

type sprintWrapper struct {
	Sprint Sprint `json:"sprint"`
}

type sprintsResult struct {
	Sprints []Sprint `json:"sprints"`
}

// The backlogs plugin overlays the redmine Version object as a Sprint, which
// has a few more fields.

type Sprint struct {
	ID            int     `json:"id"`
	ProjectID     int     `json:"project_id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Status        string  `json:"status"`
	Sharing       string  `json:"sharing"`
	DueDate       string  `json:"effective_date"`
	StartDate     string  `json:"sprint_start_date"`
	CreatedOn     string  `json:"created_on"`
	UpdatedOn     string  `json:"updated_on"`
	StoryPoints   float32 `json:"story_points"`
	TeamID        int     `json:"rb_team_id"`
	WikiPageTitle string  `json:"wiki_page_title"`
}

func (c *Client) Sprint(id int) (*Sprint, error) {
	res, err := c.Get("/rb/sprint/" + strconv.Itoa(id) + ".json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, errors.New("Not Found")
	}

	var r sprintWrapper
	err = responseHelper(res, &r, 200)
	if err != nil {
		return nil, err
	}
	return &r.Sprint, nil
}
