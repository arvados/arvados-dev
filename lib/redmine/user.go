// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package redmine

import (
	"errors"
	"strconv"
)

type userWrapper struct {
	User User `json:"user"`
}

type User struct {
	ID          int    `json:"id"`
	FirstName   string `json:"firstname"`
	LastName    string `json:"lastname"`
	Mail        string `json:"mail"`
	CreatedOn   string `json:"created_on"`
	LastLoginOn string `json:"last_login_on"`
}

func (c *Client) User(id int) (*User, error) {
	res, err := c.Get("/users/" + strconv.Itoa(id) + ".json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, errors.New("Not Found")
	}

	var r userWrapper
	err = responseHelper(res, &r, 200)
	if err != nil {
		return nil, err
	}
	return &r.User, nil
}
