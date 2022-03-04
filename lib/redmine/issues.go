// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Somewhat inspired by https://github.com/mattn/go-redmine (MIT licensed)

package redmine

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// In read operations the Redmine API returns ID fields like ProjectID.
// When updating or creating an object, it wants a Project field.
// This struct represents both for convenience.
type Issue struct {
	ID             int                `json:"id"`
	Subject        string             `json:"subject"`
	Description    string             `json:"description,omitempty"`
	ProjectID      int                `json:"project_id,omitempty"`
	Project        *IDName            `json:"project,omitempty"`
	ParentIssueID  int                `json:"parent_issue_id,omitempty"`
	Parent         *ID                `json:"parent,omitempty"`
	StatusID       int                `json:"status_id,omitempty"`
	Status         *IDName            `json:"status,omitempty"`
	FixedVersionID int                `json:"fixed_version_id,omitempty"`
	FixedVersion   *IDName            `json:"fixed_version,omitempty"`
	ReleaseID      int                `json:"release_id,omitempty"`
	Release        map[string]*IDName `json:"release,omitempty"`
	TrackerID      int                `json:"tracker_id,omitempty"`
	Tracker        *IDName            `json:"tracker,omitempty"`
	PriorityID     int                `json:"priority_id,omitempty"`
	Priority       *IDName            `json:"priority,omitempty"`
	CategoryID     int                `json:"category_id,omitempty"`
	Category       *IDName            `json:"category,omitempty"`
	AssignedToID   int                `json:"assigned_to_id,omitempty"`
	AssignedTo     *IDName            `json:"assigned_to,omitempty"`
	WatcherUserIDs []int              `json:"watcher_user_ids,omitempty"`
	Watchers       []*IDName          `json:"watchers,omitempty"`
	IsPrivate      bool               `json:"is_private,omitempty"`
	EstimatedHours float64            `json:"estimated_hours,omitempty"`
	Notes          string             `json:"notes,omitempty"`
}

type IssueFilter struct {
	ProjectID string
	StatusID  string
	Subject   string
	ParentID  string
	VersionID string
}

type issuesResult struct {
	Issues     []Issue `json:"issues"`
	TotalCount uint    `json:"total_count"`
	Offset     uint    `json:"offset"`
	Limit      uint    `json:"limit"`
}

type issueWrapper struct {
	Issue Issue `json:"issue"`
}

// issueFilters converts an *IssueFilter into a slice of filter strings
func issueFilters(issueFilter *IssueFilter) []string {
	var filterParameters []string

	if issueFilter == nil {
		return filterParameters
	}

	if len(issueFilter.ProjectID) > 0 {
		filterParameters = append(filterParameters, fmt.Sprintf("project_id=%v", issueFilter.ProjectID))
	}
	if len(issueFilter.StatusID) > 0 {
		filterParameters = append(filterParameters, fmt.Sprintf("status_id=%v", issueFilter.StatusID))
	}
	if len(issueFilter.ParentID) > 0 {
		filterParameters = append(filterParameters, fmt.Sprintf("parent_id=%v", issueFilter.ParentID))
	}
	if len(issueFilter.Subject) > 0 {
		filterParameters = append(filterParameters, fmt.Sprintf("subject=~%v", issueFilter.Subject))
	}
	if len(issueFilter.VersionID) > 0 {
		filterParameters = append(filterParameters, fmt.Sprintf("fixed_version=~%v", issueFilter.VersionID))
	}

	if len(filterParameters) > 0 {
		return filterParameters[1:]
	}

	return filterParameters
}

// FilteredIssues returns a slice of issues that matches the f criteria
func (c *Client) FilteredIssues(f *IssueFilter) ([]Issue, error) {
	s := issueFilters(f)

	res, err := c.Get("/issues.json?" + strings.Join(s, "&"))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var r issuesResult
	err = responseHelper(res, &r, 200)
	if err != nil {
		return nil, err
	}
	return r.Issues, nil
}

// CreateIssue creates a redmine issue
func (c *Client) CreateIssue(issue Issue) (*Issue, error) {
	var ir issueWrapper
	ir.Issue = issue
	s, err := json.Marshal(ir)
	if err != nil {
		return nil, err
	}
	res, err := c.Post("/issues.json", string(s))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var r issueWrapper
	err = responseHelper(res, &r, 201)
	if err != nil {
		return nil, err
	}
	return &r.Issue, nil
}

// GetIssue retrieves a redmine Issue object by id
func (c *Client) GetIssue(ID int) (*Issue, error) {
	res, err := c.Get("/issues/" + strconv.Itoa(ID) + ".json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, fmt.Errorf("Issue with id %d not found", ID)
	}
	var r issueWrapper
	err = responseHelper(res, &r, 200)
	if err != nil {
		return nil, err
	}
	return &r.Issue, nil
}

// UpdateIssue updates a redmine issue
func (c *Client) UpdateIssue(issue Issue) error {
	var ir issueWrapper
	issue.ProjectID = issue.Project.ID
	ir.Issue = issue
	s, err := json.Marshal(ir)
	if err != nil {
		return err
	}
	res, err := c.Put("/issues/"+strconv.Itoa(issue.ID)+".json", string(s))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return fmt.Errorf("Issue with id %d not found", issue.ID)
	}

	return responseHelper(res, nil, 200)
}

// FindOrCreateIssue finds or creates an issue with a given subject, parentID, versionID and projectID
func (c *Client) FindOrCreateIssue(subject string, parentID int, versionID int, projectID int) (Issue, error) {
	var f IssueFilter
	var issue Issue
	f.Subject = url.QueryEscape(subject)
	if parentID != 0 {
		f.ParentID = strconv.Itoa(parentID)
	}
	if projectID != 0 {
		f.ProjectID = strconv.Itoa(projectID)
	}
	f.StatusID = "*"
	issues, err := c.FilteredIssues(&f)
	if err != nil {
		return issue, err
	}
	if len(issues) > 0 {
		// Issue found, return it
		return issues[0], err
	}

	// Create new issue
	issue.ProjectID = projectID
	issue.FixedVersionID = versionID
	issue.Subject = subject
	if parentID != 0 {
		issue.ParentIssueID = parentID
	}

	i, err := c.CreateIssue(issue)
	if err != nil {
		return Issue{}, err
	}
	return *i, err
}

// SetRelease updates the release for an issue
func (c *Client) SetRelease(issue Issue, release int) error {
	issue.ReleaseID = release
	issue.Release = nil
	return c.UpdateIssue(issue)
}
