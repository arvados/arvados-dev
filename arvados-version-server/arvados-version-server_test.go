// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"
	"time"
)

// Hook gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

// Gocheck boilerplate
var _ = Suite(&ServerRequiredSuite{})
var _ = Suite(&ServerNotRequiredSuite{})

type ServerRequiredSuite struct{}
type ServerNotRequiredSuite struct{}

var tmpConfigFileName string

func closeListener() {
	if listener != nil {
		listener.Close()
	}
}

func (s *ServerNotRequiredSuite) TestConfig(c *C) {
	var config Config

	// A specified but non-existing config path needs to result in an error
	err := readConfig(&config, "/nosuchdir89j7879/8hjwr7ojgyy7", defaultConfigPath)
	c.Assert(err, NotNil)

	// No configuration file but default configuration path specified
	// should result in the default config being used
	err = readConfig(&config, "/nosuchdir89j7879/8hjwr7ojgyy7", "/nosuchdir89j7879/8hjwr7ojgyy7")
	c.Assert(err, IsNil)

	c.Check(config.DirPath, Equals, "")
	c.Check(config.CacheDirPath, Equals, "")
	c.Check(config.GitExecutablePath, Equals, "")
	c.Check(config.ListenPort, Equals, "")

	// Test parsing of config data
	tmpfile, err := ioutil.TempFile(os.TempDir(), "config")
	c.Check(err, IsNil)
	defer os.Remove(tmpfile.Name())

	argsS := `{"DirPath": "/x/y", "CacheDirPath": "/x/z", "GitExecutablePath": "/usr/local/bin/gitexecutable", "ListenPort": "12345"}`
	_, err = tmpfile.Write([]byte(argsS))
	c.Check(err, IsNil)

	err = readConfig(&config, tmpfile.Name(), defaultConfigPath)
	c.Assert(err, IsNil)

	c.Check(config.DirPath, Equals, "/x/y")
	c.Check(config.CacheDirPath, Equals, "/x/z")
	c.Check(config.GitExecutablePath, Equals, "/usr/local/bin/gitexecutable")
	c.Check(config.ListenPort, Equals, "12345")

}

func (s *ServerNotRequiredSuite) TestFlags(c *C) {

	args := []string{"arvados-version-server"}
	os.Args = append(args)
	//go main()

}

func runServer(c *C) {
	tmpfile, err := ioutil.TempFile(os.TempDir(), "config")
	c.Check(err, IsNil)

	tmpConfigFileName = tmpfile.Name()

	argsS := `{"DirPath": "", "CacheDirPath": "", "GitExecutablePath": "", "ListenPort": "12345"}`
	_, err = tmpfile.Write([]byte(argsS))
	c.Check(err, IsNil)

	args := []string{"arvados-version-server"}
	os.Args = append(args, "-config", tmpfile.Name())
	listener = nil
	go main()
	waitForListener()
}

func clearCache(c *C) {
	err := os.RemoveAll(theConfig.CacheDirPath)
	c.Check(err, IsNil)
}

func waitForListener() {
	const (
		ms = 5
	)
	for i := 0; listener == nil && i < 10000; i += ms {
		time.Sleep(ms * time.Millisecond)
	}
	if listener == nil {
		log.Fatalf("Timed out waiting for listener to start")
	}
}

func (s *ServerRequiredSuite) SetUpTest(c *C) {
	//arvadostest.ResetEnv()
}

func (s *ServerRequiredSuite) TearDownSuite(c *C) {
	//arvadostest.StopKeep(2)
}

func (s *ServerRequiredSuite) TestResults(c *C) {
	runServer(c)
	clearCache(c)
	defer closeListener()
	defer os.Remove(tmpConfigFileName)

	// Test the about handler
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/about"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		body, err := ioutil.ReadAll(resp.Body)
		c.Check(string(body), Matches, ".*\"Name\":\"Arvados Version Server\".*")
	}

	// Test the help handler
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/help"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		body, err := ioutil.ReadAll(resp.Body)
		c.Check(string(body), Matches, ".*\"Usage\":\"GET /v1/commit/ or GET /v1/commit/git-commit or GET /v1/about or GET /v1/help\".*")
	}

	// Check the arvados-src version string for the first commit
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/commit/155848c15844554a5d5fd50f9577aa2e19767d9e"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		body, err := ioutil.ReadAll(resp.Body)
		c.Check(string(body), Matches, ".*\"arvados-src\":\"0.1.20130104011935.155848c\".*")
	}

	// Check the arvados-src version string for a more recent commit
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/commit/9c1a28719df89a68b83cee07e3e0ab87c1712f69"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		body, err := ioutil.ReadAll(resp.Body)
		c.Check(string(body), Matches, ".*\"arvados-src\":\"0.1.20161208152419.9c1a287\".*")
	}

	// Check the arvados-src version string for a weirdly truncated commit
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/commit/9c1a28719df89"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		body, err := ioutil.ReadAll(resp.Body)
		c.Check(string(body), Matches, ".*\"arvados-src\":\"0.1.20161208152419.9c1a287\".*")
	}

	// Check an invalid request hash
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/commit/____"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		body, err := ioutil.ReadAll(resp.Body)
		c.Check(string(body), Matches, ".*\"Type\":\"Error\".*")
		c.Check(string(body), Matches, ".*\"Msg\":\"Invalid RequestHash\".*")
	}

	// Check an invalid request hash of improper length
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/commit/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		body, err := ioutil.ReadAll(resp.Body)
		c.Check(string(body), Matches, ".*\"Type\":\"Error\".*")
		c.Check(string(body), Matches, ".*\"Msg\":\"There was an error checking out the requested revision\".*")
	}

	// Check the python-arvados-cwl-runner version string for a *merge* commit where the python sdk version takes precedence
	// This does not test the "if pythonSDKTimestamp > packageTimestamp" conditional block in func pythonSDKVersionCheck
	// which appears to be a consequence of the --first-parent argument we pass to git log (exactly why, I don't understand yet)
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/commit/965565ddc62635928a6b043158fd683738961c8c"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		body, err := ioutil.ReadAll(resp.Body)
		c.Check(string(body), Matches, ".*\"python-arvados-cwl-runner\":\"1.0.20161216221537\".*")
	}

	// Check the python-arvados-cwl-runner version string for a non-merge commit where the python sdk version takes precedence
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/commit/697e73b0605b6c182f1051e97ed370d5afa7d954"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		body, err := ioutil.ReadAll(resp.Body)
		c.Check(string(body), Matches, ".*\"python-arvados-cwl-runner\":\"1.0.20161216215418\".*")
	}

	// Check passing 'master' as revision
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/commit/master"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		_, err = ioutil.ReadAll(resp.Body)
		//		c.Check(string(body), Matches, ".*\"python-arvados-cwl-runner\":\"1.0.20161216215418\".*")
	}

	// Check passing '' as revision
	{
		client := http.Client{}
		req, err := http.NewRequest("GET",
			fmt.Sprintf("http://%s/%s", listener.Addr().String(), "v1/commit/"),
			nil)
		resp, err := client.Do(req)
		c.Check(err, Equals, nil)
		c.Check(resp.StatusCode, Equals, 200)
		_, err = ioutil.ReadAll(resp.Body)
		//		c.Check(string(body), Matches, ".*\"python-arvados-cwl-runner\":\"1.0.20161216215418\".*")
	}

}
