// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/smtp"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"git.arvados.org/arvados-dev.git/lib/redmine"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

//go:embed emailTemplate.txt
var emailTemplate string
var debug, send bool

type ReviewTask struct {
	IssueID      string
	IssueSubject string
	IssueURL     string
	ID           string
	Subject      string
	URL          string
	Status       string
}

type Report struct {
	Developer             string
	Email                 string
	Subject               string
	Body                  bytes.Buffer
	SprintName            string
	SprintURL             string
	SprintStartDate       string
	SprintDueDate         string
	ReviewTasksInProgress []ReviewTask
	UnassignedReviewTasks []ReviewTask
	NewReviewTasks        []ReviewTask
}

var (
	conf config
)

type config struct {
	Endpoint string `json:"endpoint"` // https://dev-dev.arvados.org
	Apikey   string `json:"apikey"`   // abcde...
}

func loadConfig() config {
	var c config

	Viper := viper.New()
	Viper.SetEnvPrefix("redmine") // will be uppercased automatically
	Viper.BindEnv("endpoint")
	Viper.BindEnv("apikey")

	c.Endpoint = Viper.GetString("endpoint")
	c.Apikey = Viper.GetString("apikey")

	return c
}

func init() {
	rootCmd.PersistentFlags().StringP("output", "o", "", "Output format. Empty for human-readable, 'json' or 'json-line'")
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Print help")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Print debug output")
	rootCmd.PersistentFlags().BoolP("send", "s", false, "Send reports via e-mail (if false, print them to stdout)")
	rootCmd.Flags().StringP("project", "p", "", "Redmine project name")
	err := rootCmd.MarkFlagRequired("project")
	if err != nil {
		log.Fatalf(err.Error())
	}

}

var rootCmd = &cobra.Command{
	Use:   "review-task-reminder",
	Short: "review-task-reminder - Send e-mail reminders with the list of review tasks in progress",
	Long: `
review-task-reminder looks at the current sprint and e-mails a reminder to all
people with assigned review tasks.

https://git.arvados.org/arvados-dev.git/cmd/review-task-reminder` +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if conf.Endpoint == "" {
			cmd.Help()
			fmt.Println()
			fmt.Println("Error: the REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server")
			os.Exit(1)
		}
		if conf.Apikey == "" {
			cmd.Help()
			fmt.Println()
			fmt.Println("Error: the REDMINE_APIKEY environment variable must be set to your redmine API key")
			os.Exit(1)
		}
		var err error
		debug, err = cmd.Flags().GetBool("debug")
		if err != nil {
			log.Fatalf(err.Error())
		}
		if debug {
			// parse string, this is built-in feature of logrus
			ll, err := log.ParseLevel("debug")
			if err != nil {
				ll = log.DebugLevel
			}
			// set global log level
			log.SetLevel(ll)
			log.Debug("Enabled debug log level")
		}

		send, err = cmd.Flags().GetBool("send")
		if err != nil {
			log.Fatalf(err.Error())
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug("Creating redmine object")
		rm := redmine.NewClient(conf.Endpoint, conf.Apikey)

		log.Debug("Getting project object")
		project, err := cmd.Flags().GetString("project")
		if err != nil {
			log.Fatalf(err.Error())
		}
		p, err := rm.GetProjectByName(project)
		if err != nil {
			log.Fatalf(err.Error())
		}

		log.Debugf("Project: %s ID: %d", project, p.ID)

		log.Debug("Getting versions")
		versions, err := rm.Versions(p.ID)
		if err != nil {
			log.Fatalf(err.Error())
		}
		now := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)
		// Find any current sprint(s)
		for _, v := range versions {
			// It must be "open"
			if v.Status != "open" {
				continue
			}
			// The due date must be in the future
			if v.DueDate == "" {
				continue
			}
			dueDate, err := time.Parse("2006-01-02", v.DueDate)
			if err != nil {
				log.Fatalf(err.Error())
			}
			if dueDate.Before(now) {
				continue
			}
			// The start date must be in the past (have to look up the Sprint object!)
			log.Debugf("Getting sprint with id %d", v.ID)
			s, err := rm.Sprint(v.ID)
			if err != nil {
				log.Fatalf(err.Error())
			}
			if s.StartDate == "" {
				continue
			}
			startDate, err := time.Parse("2006-01-02", s.StartDate)
			if err != nil {
				log.Fatalf(err.Error())
			}
			if startDate.After(now) {
				continue
			}
			// Found a current sprint
			log.Debugf("Current sprint: %+#v", s)

			// Get the issues from this sprint
			var issueFilter redmine.IssueFilter
			issueFilter.VersionID = strconv.Itoa(v.ID)
			log.Debugf("Getting issues with version ID %d", v.ID)
			issues, err := rm.FilteredIssues(&issueFilter)
			if err != nil {
				log.Fatalf(err.Error())
			}

			log.Debugf("Retrieved %d issues", len(issues))
			reviewTasksByDeveloper := make(map[int][]ReviewTask)
			var UnassignedReviewTasks []ReviewTask
			for _, i := range issues {
				log.Debugf("Considering issue (%s): %+#v, \"%s\"", i.Tracker.Name, i.ID, i.Subject)
				// Filter for tasks
				if i.Tracker.Name != "Task" {
					continue
				}
				// Filter for review tasks (issue subject must start with 'review')
				reviewRE := regexp.MustCompile(`^(Review|review)`)
				if !reviewRE.MatchString(i.Subject) {
					continue
				}
				// Is the task assigned?
				if i.AssignedTo == nil {
					log.Debugf("Found unassigned review task: %+#v, \"%s\"", i.ID, i.Subject)
					log.Debugf("Getting parent issue with ID %d", i.Parent.ID)
					parent, err := rm.GetIssue(i.Parent.ID)
					if err != nil {
						log.Fatalf(err.Error())
					}
					rt := ReviewTask{
						IssueID:      "#" + strconv.Itoa(parent.ID),
						IssueSubject: limit(parent.Subject, 58),
						IssueURL:     conf.Endpoint + "/issues/" + strconv.Itoa(parent.ID),
						ID:           "#" + strconv.Itoa(i.ID),
						Subject:      limit(i.Subject, 58),
						URL:          conf.Endpoint + "/issues/" + strconv.Itoa(i.ID),
						Status:       i.Status.Name,
					}
					UnassignedReviewTasks = append(UnassignedReviewTasks, rt)
					continue
				}
				// Found an assigned review task
				log.Debugf("Found assigned review task: %+#v, \"%s\", assigned to %+#v", i.ID, i.Subject, i.AssignedTo.ID)
				if _, ok := reviewTasksByDeveloper[i.AssignedTo.ID]; !ok {
					reviewTasksByDeveloper[i.AssignedTo.ID] = []ReviewTask{}
				}
				log.Debugf("Getting parent issue with ID %d", i.Parent.ID)
				parent, err := rm.GetIssue(i.Parent.ID)
				if err != nil {
					log.Fatalf(err.Error())
				}
				rt := ReviewTask{
					IssueID:      "#" + strconv.Itoa(parent.ID),
					IssueSubject: limit(parent.Subject, 58),
					IssueURL:     conf.Endpoint + "/issues/" + strconv.Itoa(parent.ID),
					ID:           "#" + strconv.Itoa(i.ID),
					Subject:      limit(i.Subject, 58),
					URL:          conf.Endpoint + "/issues/" + strconv.Itoa(i.ID),
					Status:       i.Status.Name,
				}
				reviewTasksByDeveloper[i.AssignedTo.ID] = append(reviewTasksByDeveloper[i.AssignedTo.ID], rt)
			}

			// Create the report(s)
			log.Debug("Creating reports")
			for developerID, rt := range reviewTasksByDeveloper {
				log.Debugf("Getting user with ID %d", developerID)
				u, err := rm.User(developerID)
				if err != nil {
					log.Fatalf(err.Error())
				}
				var report Report
				report.Developer = u.FirstName + " " + u.LastName + " <" + u.Mail + ">"
				report.Email = u.Mail
				report.SprintName = s.Name
				report.SprintURL = conf.Endpoint + "/rb/taskboards/" + strconv.Itoa(s.ID)
				report.SprintStartDate = s.StartDate
				report.SprintDueDate = s.DueDate
				report.UnassignedReviewTasks = UnassignedReviewTasks
				for _, r := range rt {
					log.Debugf("rt status %s", r.Status)
					if r.Status == "In Progress" {
						report.ReviewTasksInProgress = append(report.ReviewTasksInProgress, r)
					} else if r.Status == "New" {
						report.NewReviewTasks = append(report.NewReviewTasks, r)
					}
				}
				if len(report.ReviewTasksInProgress) == 1 {
					report.Subject = strings.Title(project) + ": you have 1 review to finish"
				} else if len(report.ReviewTasksInProgress) > 0 {
					report.Subject = strings.Title(project) + ": you have " + strconv.Itoa(len(report.ReviewTasksInProgress)) + " reviews to finish"
				} else if len(report.UnassignedReviewTasks) > 0 {
					report.Subject = strings.Title(project) + ": nobody is waiting on a review from you (but take an unassigned review, please?)"
				} else {
					report.Subject = strings.Title(project) + ": nobody is waiting on a review from you"
				}
				t, err := template.New("report").Parse(emailTemplate)
				if err != nil {
					log.Fatalf(err.Error())
				}
				err = t.Execute(&report.Body, report)
				if err != nil {
					log.Fatalf(err.Error())
				}
				if send && !debug {
					log.Info("Sending e-mail report to " + report.Email)
					_, err = report.SendEmail()
					if err != nil {
						log.Fatalf(err.Error())
					}
				} else if !debug {
					log.Info("Not sending e-mail (--send option not enabled), report follows:")
					log.Info(report.Body.String())
				} else {
					log.Debug("Not sending e-mail (debug mode), report follows:")
					log.Debug(report.Body.String())
				}
			}
		}
	},
}

func (r *Report) SendEmail() (bool, error) {
	if err := smtp.SendMail("localhost:25", nil, "sysadmin@curii.com", []string{r.Email}, r.Body.Bytes()); err != nil {
		return false, err
	}
	return true, nil
}

func limit(text string, limit int) string {
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}

func Execute() {
	conf = loadConfig()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
