// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"git.curoverse.com/arvados.git/sdk/go/config"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var listener net.Listener

type logStruct struct {
	Type string
	Msg  string
}

type packageStruct struct {
	sourceDir            string
	packageName          string
	packageType          string
	packageVersionType   string
	packageVersionPrefix string
}

type returnStruct struct {
	RequestHash string
	GitHash     string
	Versions    map[string]map[string]string
	Cached      bool
	Elapsed     string
}

type aboutStruct struct {
	Name    string
	Version string
	URL     string
}

type helpStruct struct {
	Usage string
}

// Config structure
type Config struct {
	DirPath           string
	CacheDirPath      string
	GitExecutablePath string
	ListenPort        string

	Packages []packageStruct
}

var theConfig Config

const defaultConfigPath = "/etc/arvados/version-server/version-server.yml"

func loadPackages() (packages []packageStruct) {
	packages = []packageStruct{
		{
			sourceDir:            ".",
			packageName:          "arvados-src",
			packageType:          "distribution",
			packageVersionType:   "git",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "apps/workbench",
			packageName:          "arvados-workbench",
			packageType:          "distribution",
			packageVersionType:   "git",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "sdk/cwl",
			packageName:          "python-arvados-cwl-runner",
			packageType:          "distribution",
			packageVersionType:   "python",
			packageVersionPrefix: "1.0",
		},
		{
			sourceDir:            "sdk/cwl",
			packageName:          "arvados-cwl-runner",
			packageType:          "python",
			packageVersionType:   "python",
			packageVersionPrefix: "1.0",
		},
		{
			sourceDir:            "sdk/cwl",
			packageName:          "arvados/jobs",
			packageType:          "docker",
			packageVersionType:   "docker",
			packageVersionPrefix: "",
		},
		{
			sourceDir:            "sdk/go/crunchrunner",
			packageName:          "crunchrunner",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "sdk/pam",
			packageName:          "libpam-arvados",
			packageType:          "distribution",
			packageVersionType:   "python",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "sdk/pam",
			packageName:          "arvados-pam",
			packageType:          "python",
			packageVersionType:   "python",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "sdk/python",
			packageName:          "python-arvados-python-client",
			packageType:          "distribution",
			packageVersionType:   "python",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "sdk/python",
			packageName:          "arvados-python-client",
			packageType:          "python",
			packageVersionType:   "python",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/api",
			packageName:          "arvados-api-server",
			packageType:          "distribution",
			packageVersionType:   "git",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/arv-git-httpd",
			packageName:          "arvados-git-httpd",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/crunch-dispatch-local",
			packageName:          "crunch-dispatch-local",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/crunch-dispatch-slurm",
			packageName:          "crunch-dispatch-slurm",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/crunch-run",
			packageName:          "crunch-run",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/crunchstat",
			packageName:          "crunchstat",
			packageType:          "distribution",
			packageVersionType:   "git",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/dockercleaner",
			packageName:          "arvados-docker-cleaner",
			packageType:          "distribution",
			packageVersionType:   "python",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/fuse",
			packageName:          "python-arvados-fuse",
			packageType:          "distribution",
			packageVersionType:   "python",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/fuse",
			packageName:          "arvados_fuse",
			packageType:          "python",
			packageVersionType:   "python",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/keep-balance",
			packageName:          "keep-balance",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/keepproxy",
			packageName:          "keepproxy",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/keepstore",
			packageName:          "keepstore",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/keep-web",
			packageName:          "keep-web",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/nodemanager",
			packageName:          "arvados-node-manager",
			packageType:          "distribution",
			packageVersionType:   "python",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/nodemanager",
			packageName:          "arvados-node-manager",
			packageType:          "python",
			packageVersionType:   "python",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/ws",
			packageName:          "arvados-ws",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "tools/crunchstat-summary",
			packageName:          "crunchstat-summary",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "tools/keep-block-check",
			packageName:          "keep-block-check",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "tools/keep-exercise",
			packageName:          "keep-exercise",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "tools/keep-rsync",
			packageName:          "keep-rsync",
			packageType:          "distribution",
			packageVersionType:   "go",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "sdk/ruby",
			packageName:          "arvados",
			packageType:          "gem",
			packageVersionType:   "ruby",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "sdk/cli",
			packageName:          "arvados-cli",
			packageType:          "gem",
			packageVersionType:   "ruby",
			packageVersionPrefix: "0.1",
		},
		{
			sourceDir:            "services/login-sync",
			packageName:          "arvados-login-sync",
			packageType:          "gem",
			packageVersionType:   "ruby",
			packageVersionPrefix: "0.1",
		},
	}
	return
}

func lookupInCache(hash string) (returnStruct, error) {
	statData, err := os.Stat(theConfig.CacheDirPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(theConfig.CacheDirPath, 0700)
		if err != nil {
			logError([]string{"Error creating directory", theConfig.CacheDirPath, ":", err.Error()})
		}
	} else {
		if !statData.IsDir() {
			logError([]string{"The path", theConfig.CacheDirPath, "is not a directory"})
			return returnStruct{}, fmt.Errorf("The path %s is not a directory", theConfig.CacheDirPath)
		}
	}
	file, e := ioutil.ReadFile(theConfig.CacheDirPath + "/" + hash)
	if e != nil {
		return returnStruct{}, fmt.Errorf("File error: %v\n", e)
	}
	var m returnStruct
	err = json.Unmarshal(file, &m)
	return m, err
}

func writeToCache(hash string, data returnStruct) (err error) {
	statData, err := os.Stat(theConfig.CacheDirPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(theConfig.CacheDirPath, 0700)
		if err != nil {
			logError([]string{"Error creating directory", theConfig.CacheDirPath, ":", err.Error()})
		}
	} else {
		if !statData.IsDir() {
			logError([]string{"The path", theConfig.CacheDirPath, "is not a directory"})
			return fmt.Errorf("The path %s is not a directory", theConfig.CacheDirPath)
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(theConfig.CacheDirPath+"/"+hash, jsonData, 0644)
	return
}

func prepareGitPath(hash string) error {
	statData, err := os.Stat(theConfig.DirPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(theConfig.DirPath, 0700)
		if err != nil {
			logError([]string{"Error creating directory", theConfig.DirPath, ":", err.Error()})
			return fmt.Errorf("Error creating directory %s", theConfig.DirPath)
		}
		cmdArgs := []string{"clone", "https://github.com/curoverse/arvados.git", theConfig.DirPath}
		if _, err = exec.Command(theConfig.GitExecutablePath, cmdArgs...).Output(); err != nil {
			logError([]string{"There was an error running the command ", theConfig.GitExecutablePath, strings.Join(cmdArgs, " "), err.Error()})
			return fmt.Errorf("There was an error cloning the repository")
		}
	} else {
		if !statData.IsDir() {
			logError([]string{"The path", theConfig.DirPath, "is not a directory"})
			return fmt.Errorf("The path %s is not a directory", theConfig.DirPath)
		}
	}
	return nil
}

func prepareGitCheckout(hash string) (string, error) {
	err := prepareGitPath(hash)
	if err != nil {
		return "", err
	}
	err = os.Chdir(theConfig.DirPath)
	if err != nil {
		logError([]string{"Error changing directory to", theConfig.DirPath})
		return "", fmt.Errorf("Error changing directory to %s", theConfig.DirPath)
	}
	cmdArgs := []string{"fetch", "--all"}
	if _, err := exec.Command(theConfig.GitExecutablePath, cmdArgs...).Output(); err != nil {
		logError([]string{"There was an error running the command ", theConfig.GitExecutablePath, strings.Join(cmdArgs, " "), err.Error()})
		return "", fmt.Errorf("There was an error fetching all remotes")
	}
	if hash == "" {
		hash = "master"
	}
	cmdArgs = []string{"checkout", hash}
	if _, err := exec.Command(theConfig.GitExecutablePath, cmdArgs...).Output(); err != nil {
		logError([]string{"There was an error running the command ", theConfig.GitExecutablePath, strings.Join(cmdArgs, " "), err.Error()})
		return "", fmt.Errorf("There was an error checking out the requested revision")
	}
	if hash == "master" {
		cmdArgs := []string{"reset", "--hard", "origin/master"}
		if _, err := exec.Command(theConfig.GitExecutablePath, cmdArgs...).Output(); err != nil {
			logError([]string{"There was an error running the command ", theConfig.GitExecutablePath, strings.Join(cmdArgs, " "), err.Error()})
			return "", fmt.Errorf("There was an error fetching all remotes")
		}
	}
	return "", nil
}

// Generates the hash for the latest git commit for the current working directory
func gitHashFull() (string, error) {
	cmdArgs := []string{"log", "-n1", "--first-parent", "--max-count=1", "--format=format:%H", "."}
	cmdOut, err := exec.Command(theConfig.GitExecutablePath, cmdArgs...).Output()
	if err != nil {
		logError([]string{"There was an error running the command ", theConfig.GitExecutablePath, strings.Join(cmdArgs, " "), err.Error()})
		return "", fmt.Errorf("There was an error getting the git hash for this revision")
	}
	return string(cmdOut), nil
}

// Generates a version number from the git log for the current working directory
func versionFromGit(prefix string) (string, error) {
	gitTs, err := getGitTs()
	if err != nil {
		return "", err
	}
	cmdArgs := []string{"log", "-n1", "--first-parent", "--max-count=1", "--format=format:%h", "."}
	gitHash, err := exec.Command(theConfig.GitExecutablePath, cmdArgs...).Output()
	if err != nil {
		logError([]string{"There was an error running the command ", theConfig.GitExecutablePath, strings.Join(cmdArgs, " "), err.Error()})
		return "", fmt.Errorf("There was an error getting the git hash for this revision")
	}
	cmdName := "/bin/date"
	cmdArgs = []string{"-ud", "@" + string(gitTs), "+%Y%m%d%H%M%S"}
	date, err := exec.Command(cmdName, cmdArgs...).Output()
	if err != nil {
		logError([]string{"There was an error running the command ", cmdName, strings.Join(cmdArgs, " "), err.Error()})
		return "", fmt.Errorf("There was an error converting the datestamp for this revision")
	}

	return fmt.Sprintf("%s.%s.%s", strings.TrimSpace(prefix), strings.TrimSpace(string(date)), strings.TrimSpace(string(gitHash))), nil
}

// Generates a python package version number from the git log for the current working directory
func rubyVersionFromGit(prefix string) (string, error) {
	gitTs, err := getGitTs()
	if err != nil {
		return "", err
	}
	cmdName := "/bin/date"
	cmdArgs := []string{"-ud", "@" + string(gitTs), "+%Y%m%d%H%M%S"}
	date, err := exec.Command(cmdName, cmdArgs...).Output()
	if err != nil {
		logError([]string{"There was an error running the command ", cmdName, strings.Join(cmdArgs, " "), err.Error()})
		return "", fmt.Errorf("There was an error converting the datestamp for this revision")
	}

	return fmt.Sprintf("%s.%s", strings.TrimSpace(prefix), strings.TrimSpace(string(date))), nil
}

// Generates a python package version number from the git log for the current working directory
func pythonVersionFromGit(prefix string) (string, error) {
	rv, err := rubyVersionFromGit(prefix)
	if err != nil {
		return "", err
	}
	return rv, nil
}

// Generates a docker image version number from the git log for the current working directory
func dockerVersionFromGit() (string, error) {
	rv, err := gitHashFull()
	if err != nil {
		return "", err
	}
	return rv, nil
}

func getGitTs() (gitTs []byte, err error) {
	cmdArgs := []string{"log", "-n1", "--first-parent", "--max-count=1", "--format=format:%ct", "."}
	gitTs, err = exec.Command(theConfig.GitExecutablePath, cmdArgs...).Output()
	if err != nil {
		logError([]string{"There was an error running the command ", theConfig.GitExecutablePath, strings.Join(cmdArgs, " "), err.Error()})
		return nil, fmt.Errorf("There was an error getting the git hash for this revision")
	}
	return
}

// Generates a timestamp from the git log for the current working directory
func timestampFromGit() (string, error) {
	gitTs, err := getGitTs()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", strings.TrimSpace(string(gitTs))), nil
}

func normalizeRequestedHash(hash string) (string, error) {
	_, err := prepareGitCheckout(hash)
	if err != nil {
		return "", err
	}

	// Get the git hash for the tree
	var gitHash string
	gitHash, err = gitHashFull()
	if err != nil {
		return "", err
	}

	return gitHash, nil
}

func getPackageVersionsWorker(hash string) (gitHash string, goSDKTimestamp string, goSDKVersionWithoutPrefix string, pythonSDKTimestamp string, err error) {
	_, err = prepareGitCheckout(hash)
	if err != nil {
		return "", "", "", "", err
	}

	// Get the git hash for the tree
	gitHash, err = gitHashFull()
	if err != nil {
		return "", "", "", "", err
	}

	// Get the git timestamp and version string for the sdk/go directory
	err = os.Chdir(theConfig.DirPath + "/sdk/go")
	if err != nil {
		goSDKTimestamp = ""
		goSDKVersionWithoutPrefix = ""
		err = nil
	} else {
		goSDKTimestamp, err = timestampFromGit()
		if err != nil {
			return "", "", "", "", err
		}
		goSDKVersionWithoutPrefix, err = versionFromGit("")
		if err != nil {
			return "", "", "", "", err
		}
	}

	// Get the git timestamp and version string for the sdk/python directory
	err = os.Chdir(theConfig.DirPath + "/sdk/python")
	if err != nil {
		pythonSDKTimestamp = ""
		err = nil
	} else {
		pythonSDKTimestamp, err = timestampFromGit()
		if err != nil {
			return "", "", "", "", err
		}
	}

	return
}

func pythonSDKVersionCheck(pythonSDKTimestamp string) (err error) {
	var packageTimestamp string
	packageTimestamp, err = timestampFromGit()
	if err != nil {
		return
	}

	if pythonSDKTimestamp > packageTimestamp {
		err = os.Chdir(theConfig.DirPath + "/sdk/python")
		if err != nil {
			return
		}
	}
	return
}

func getPackageVersions(hash string) (versions map[string]map[string]string, gitHash string, err error) {
	versions = make(map[string]map[string]string)

	gitHash, goSDKTimestamp, goSDKVersionWithoutPrefix, pythonSDKTimestamp, err := getPackageVersionsWorker(hash)
	if err != nil {
		return nil, "", err
	}

	for _, p := range theConfig.Packages {
		err = os.Chdir(theConfig.DirPath + "/" + p.sourceDir)
		if err != nil {
			// Skip those packages for which the source directory doesn't exist
			// in this revision of the source tree.
			err = nil
			continue
		}
		packageName := p.packageName

		var packageVersion string

		if (p.packageVersionType == "git") || (p.packageVersionType == "go") {
			packageVersion, err = versionFromGit(p.packageVersionPrefix)
			if err != nil {
				return nil, "", err
			}
		}
		if p.packageVersionType == "go" {
			var packageTimestamp string
			packageTimestamp, err = timestampFromGit()
			if err != nil {
				return nil, "", err
			}

			if goSDKTimestamp > packageTimestamp {
				packageVersion = p.packageVersionPrefix + goSDKVersionWithoutPrefix
			}
		} else if p.packageVersionType == "python" {
			// Not all of our packages that use our python sdk are automatically
			// getting rebuilt when sdk/python changes. Yet.
			if p.packageName == "python-arvados-cwl-runner" {
				err = pythonSDKVersionCheck(pythonSDKTimestamp)
				if err != nil {
					return nil, "", err
				}
			}

			packageVersion, err = pythonVersionFromGit(p.packageVersionPrefix)
			if err != nil {
				return nil, "", err
			}
		} else if p.packageVersionType == "ruby" {
			packageVersion, err = rubyVersionFromGit(p.packageVersionPrefix)
			if err != nil {
				return nil, "", err
			}
		} else if p.packageVersionType == "docker" {
			// the arvados/jobs image version is always the latest of the
			// sdk/python and the sdk/cwl version
			if p.packageName == "arvados/jobs" {
				err = pythonSDKVersionCheck(pythonSDKTimestamp)
				if err != nil {
					return nil, "", err
				}
			}
			packageVersion, err = dockerVersionFromGit()
			if err != nil {
				return nil, "", err
			}
		}

		if versions[strings.Title(p.packageType)] == nil {
			versions[strings.Title(p.packageType)] = make(map[string]string)
		}
		versions[strings.Title(p.packageType)][packageName] = packageVersion
	}

	return
}

func logError(m []string) {
	fmt.Fprintln(os.Stderr, string(marshal(logStruct{"Error", strings.Join(m, " ")})))
}

func logNotice(m []string) {
	fmt.Fprintln(os.Stderr, string(marshal(logStruct{"Notice", strings.Join(m, " ")})))
}

func marshal(message interface{}) (encoded []byte) {
	encoded, err := json.Marshal(message)
	if err != nil {
		// do not call logError here because that would create an infinite loop
		fmt.Fprintln(os.Stderr, "{\"Error\": \"Unable to marshal message into json:", message, "\"}")
		return nil
	}
	return
}

func marshalAndWrite(w io.Writer, message interface{}) {
	b := marshal(message)
	if b == nil {
		errorMessage := "{\n\"Error\": \"Unspecified error\"\n}"
		_, err := io.WriteString(w, errorMessage)
		if err != nil {
			// do not call logError (it calls marshal and that function has already failed at this point)
			fmt.Fprintln(os.Stderr, "{\"Error\": \"Unable to write message to client\"}")
		}
	} else {
		_, err := w.Write(b)
		if err != nil {
			logError([]string{"Unable to write message to client:", string(b)})
		}
	}
}

func packageVersionHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var packageVersions map[string]map[string]string
	var cached bool

	// Sanity check the input RequestHash
	match, err := regexp.MatchString("^([a-z0-9]+|)$", r.URL.Path[11:])
	if err != nil {
		m := logStruct{"Error", "Error matching RequestHash"}
		marshalAndWrite(w, m)
		return
	}
	if !match {
		m := logStruct{"Error", "Invalid RequestHash"}
		marshalAndWrite(w, m)
		return
	}

	hash := r.URL.Path[11:]

	// Empty hash or non-standard hash length? Normalize it.
	if len(hash) != 7 && len(hash) != 40 {
		hash, err = normalizeRequestedHash(hash)
		if err != nil {
			m := logStruct{"Error", err.Error()}
			marshalAndWrite(w, m)
			return
		}
	}

	var gitHash string
	rs, err := lookupInCache(hash)
	if err == nil {
		packageVersions = rs.Versions
		gitHash = rs.GitHash
		cached = true
	} else {
		packageVersions, gitHash, err = getPackageVersions(hash)
		if err != nil {
			m := logStruct{"Error", err.Error()}
			marshalAndWrite(w, m)
			return
		}
		m := returnStruct{"", gitHash, packageVersions, true, ""}
		err = writeToCache(hash, m)
		if err != nil {
			logError([]string{"Unable to save entry in cache directory", theConfig.CacheDirPath})
		}
		cached = false
	}

	m := returnStruct{hash, gitHash, packageVersions, cached, fmt.Sprintf("%v", time.Since(start))}
	marshalAndWrite(w, m)
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	m := aboutStruct{"Arvados Version Server", "0.1", "https://arvados.org"}
	marshalAndWrite(w, m)
}

func helpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	m := helpStruct{"GET /v1/commit/ or GET /v1/commit/git-commit or GET /v1/about or GET /v1/help"}
	marshalAndWrite(w, m)
}

func parseFlags() (configPath *string) {

	flags := flag.NewFlagSet("arvados-version-server", flag.ExitOnError)
	flags.Usage = func() { usage(flags) }

	configPath = flags.String(
		"config",
		defaultConfigPath,
		"`path` to YAML configuration file")

	// Parse args; omit the first arg which is the command name
	err := flags.Parse(os.Args[1:])
	if err != nil {
		logError([]string{"Unable to parse command line arguments:", err.Error()})
		os.Exit(1)
	}

	return
}

func main() {
	err := os.Setenv("TZ", "UTC")
	if err != nil {
		logError([]string{"Error setting environment variable:", err.Error()})
		os.Exit(1)
	}

	configPath := parseFlags()

	err = readConfig(&theConfig, *configPath, defaultConfigPath)
	if err != nil {
		logError([]string{"Unable to start Arvados Version Server:", err.Error()})
		os.Exit(1)
	}

	theConfig.Packages = loadPackages()

	if theConfig.DirPath == "" {
		theConfig.DirPath = "/tmp/arvados-version-server-checkout"
	}

	if theConfig.CacheDirPath == "" {
		theConfig.CacheDirPath = "/tmp/arvados-version-server-cache"
	}

	if theConfig.GitExecutablePath == "" {
		theConfig.GitExecutablePath = "/usr/bin/git"
	}

	if theConfig.ListenPort == "" {
		theConfig.ListenPort = "80"
	}

	http.HandleFunc("/v1/commit/", packageVersionHandler)
	http.HandleFunc("/v1/about", aboutHandler)
	http.HandleFunc("/v1/help", helpHandler)
	http.HandleFunc("/v1", helpHandler)
	http.HandleFunc("/", helpHandler)
	logNotice([]string{"Arvados Version Server listening on port", theConfig.ListenPort})

	listener, err = net.Listen("tcp", ":"+theConfig.ListenPort)

	if err != nil {
		logError([]string{"Unable to start Arvados Version Server:", err.Error()})
		os.Exit(1)
	}

	// Shut down the server gracefully (by closing the listener)
	// if SIGTERM is received.
	term := make(chan os.Signal, 1)
	go func(sig <-chan os.Signal) {
		<-sig
		logError([]string{"caught signal"})
		_ = listener.Close()
	}(term)
	signal.Notify(term, syscall.SIGTERM)
	signal.Notify(term, syscall.SIGINT)

	// Start serving requests.
	_ = http.Serve(listener, nil)
	// http.Serve returns an error when it gets the term or int signal

	logNotice([]string{"Arvados Version Server shutting down"})

}

func readConfig(cfg interface{}, path string, defaultConfigPath string) error {
	err := config.LoadFile(cfg, path)
	if err != nil && os.IsNotExist(err) && path == defaultConfigPath {
		logNotice([]string{"Config not specified. Continue with default configuration."})
		err = nil
	}
	return err
}
