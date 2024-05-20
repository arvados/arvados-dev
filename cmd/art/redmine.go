// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"git.arvados.org/arvados-dev.git/lib/redmine"
	survey "github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/semver"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(redmineCmd)
	redmineCmd.AddCommand(issuesCmd)
	redmineCmd.AddCommand(releasesCmd)

	associateIssueCmd.Flags().IntP("release", "r", 0, "Redmine release ID")
	err := associateIssueCmd.MarkFlagRequired("release")
	if err != nil {
		log.Fatalf(err.Error())
	}
	associateIssueCmd.Flags().IntP("issue", "i", 0, "Redmine issue ID")
	err = associateIssueCmd.MarkFlagRequired("issue")
	if err != nil {
		log.Fatalf(err.Error())
	}
	issuesCmd.AddCommand(associateIssueCmd)


	setIssueSprintCmd.Flags().IntP("sprint", "r", 0, "Redmine sprint ID")
	err = setIssueSprintCmd.MarkFlagRequired("sprint")
	if err != nil {
		log.Fatalf(err.Error())
	}
	setIssueSprintCmd.Flags().IntP("issue", "i", 0, "Redmine issue ID")
	err = setIssueSprintCmd.MarkFlagRequired("issue")
	if err != nil {
		log.Fatalf(err.Error())
	}
	issuesCmd.AddCommand(setIssueSprintCmd)

	associateOrphans.Flags().IntP("release", "r", 0, "Redmine release ID")
	err = associateOrphans.MarkFlagRequired("release")
	if err != nil {
		log.Fatalf(err.Error())
	}
	associateOrphans.Flags().StringP("project", "p", "", "Redmine project name")
	err = associateOrphans.MarkFlagRequired("project")
	if err != nil {
		log.Fatalf(err.Error())
	}
	associateOrphans.Flags().BoolP("dry-run", "", false, "Only report what will happen without making any change")
	issuesCmd.AddCommand(associateOrphans)

	findAndAssociateIssuesCmd.Flags().IntP("release", "r", 0, "Redmine release ID")
	err = findAndAssociateIssuesCmd.MarkFlagRequired("release")
	if err != nil {
		log.Fatalf(err.Error())
	}
	findAndAssociateIssuesCmd.Flags().StringP("previous-release-tag", "p", "", "Semantic version number of the previous release")
	err = findAndAssociateIssuesCmd.MarkFlagRequired("previous-release-tag")
	if err != nil {
		log.Fatalf(err.Error())
	}
	findAndAssociateIssuesCmd.Flags().StringP("new-release-commit", "n", "", "Git commit for the new release")
	err = findAndAssociateIssuesCmd.MarkFlagRequired("new-release-commit")
	if err != nil {
		log.Fatalf(err.Error())
	}
	findAndAssociateIssuesCmd.Flags().BoolP("auto-set", "a", false, "Associate issues without existing release without prompting")
	findAndAssociateIssuesCmd.Flags().BoolP("skip-release-change", "s", false, "Skip issues already assigned to another release (do not prompt)")
	findAndAssociateIssuesCmd.Flags().StringP("source-repo", "", "https://github.com/arvados/arvados.git", "Source repository to clone from")
	if err != nil {
		log.Fatalf(err.Error())
	}

	issuesCmd.AddCommand(findAndAssociateIssuesCmd)

	createReleaseIssueCmd.Flags().StringP("new-release-version", "n", "", "Semantic version number of the new release")
	err = createReleaseIssueCmd.MarkFlagRequired("new-release-version")
	if err != nil {
		log.Fatalf(err.Error())
	}
	createReleaseIssueCmd.Flags().IntP("sprint", "s", 0, "Redmine sprint (aka Version) ID")
	err = createReleaseIssueCmd.MarkFlagRequired("sprint")
	if err != nil {
		log.Fatalf(err.Error())
	}
	createReleaseIssueCmd.Flags().StringP("project", "p", "", "Redmine project name")
	err = createReleaseIssueCmd.MarkFlagRequired("project")
	if err != nil {
		log.Fatalf(err.Error())
	}
	issuesCmd.AddCommand(createReleaseIssueCmd)

	getReleaseCmd.Flags().IntP("release", "r", 0, "ID of the redmine release")
	err = getReleaseCmd.MarkFlagRequired("release")
	if err != nil {
		log.Fatalf(err.Error())
	}
	releasesCmd.AddCommand(getReleaseCmd)
}

var redmineCmd = &cobra.Command{
	Use:   "redmine",
	Short: "Manage Redmine",
	Long: "Manage Redmine.\n" +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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
		return nil
	},
}

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "Manage Redmine issues",
	Long: "Manage Redmine issues.\n" +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
}

var associateOrphans = &cobra.Command{
	Use:   "associate-orphans", // FIXME
	Short: "Find open issues without a release and version, assign them to the given release",
	Long: "Find open issues without a release and version, assign them to the given release.\n" +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
	Run: func(cmd *cobra.Command, args []string) {
		rID, err := cmd.Flags().GetInt("release")
		if err != nil {
			fmt.Printf("Error converting Redmine release ID to integer: %s", err)
			os.Exit(1)
		}
		pName, err := cmd.Flags().GetString("project")
		if err != nil {
			log.Fatalf("Error getting the requested project name: %s", err)
		}
		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err != nil {
			log.Fatalf("Error getting the dry-run parameter")
		}

		rm := redmine.NewClient(conf.Endpoint, conf.Apikey)
		p, err := rm.GetProjectByName(pName)
		if err != nil {
			log.Fatalf("Error retrieving project ID for '%s': %s", pName, err)
		}
		r, err := rm.GetRelease(rID)
		if err != nil {
			log.Fatalf("Error retrieving release '%d': %s", rID, err)
		}
		flt := redmine.IssueFilter{
			StatusID:  "open",
			ProjectID: fmt.Sprintf("%d", p.ID),
			// No values assigned on the following fields. It seems that using
			// an empty string is interpreted as 'any value'. The documentation
			// isn't clear, but after some trial & error, '!*' seems to do the trick.
			// https://www.redmine.org/projects/redmine/wiki/Rest_Issues
			ReleaseID: "!*",
			VersionID: "!*",
			ParentID:  "!*",
		}
		issues, err := rm.FilteredIssues(&flt)
		if err != nil {
			fmt.Printf("Error requesting unassigned open issues from project %d: %s", p.ID, err)
		}
		fmt.Printf("Found %d issues from project '%s' to assign to release '%s'...\n", len(issues), p.Name, r.Name)

		type job struct {
			issue  redmine.Issue
			rID    int
			dryRun bool
		}
		type result struct {
			msg     string
			success bool
		}
		var wg sync.WaitGroup
		jobs := make(chan job, len(issues))
		results := make(chan result, len(issues))

		worker := func(id int, jobs <-chan job, results chan<- result) {
			for j := range jobs {
				msg := fmt.Sprintf("#%d - %s ", j.issue.ID, j.issue.Subject)
				success := true
				if !j.dryRun {
					err = rm.SetRelease(j.issue, j.rID)
					if err != nil {
						success = false
						msg = fmt.Sprintf("%s [error] (%s)\n", msg, err)
					} else {
						msg = fmt.Sprintf("%s [changed]\n", msg)
					}
				} else {
					msg = fmt.Sprintf("%s [skipped]\n", msg)
				}
				results <- result{
					msg:     msg,
					success: success,
				}
			}
		}

		wn := 8
		if len(issues) < wn {
			wn = len(issues)
		}
		for w := 1; w <= wn; w++ {
			wg.Add(1)
			w := w
			go func() {
				defer wg.Done()
				worker(w, jobs, results)
			}()
		}

		for _, issue := range issues {
			jobs <- job{
				issue:  issue,
				rID:    rID,
				dryRun: dryRun,
			}
		}
		close(jobs)

		succeded := true
		errCount := 0
		var wg2 sync.WaitGroup
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for r := range results {
				fmt.Printf(r.msg)
				if !r.success {
					succeded = false
					errCount += 1
				}
			}
		}()

		wg.Wait()
		close(results)
		wg2.Wait()
		if !succeded {
			log.Fatalf("Warning: %d error(s) found.", errCount)
		}
	},
}

var associateIssueCmd = &cobra.Command{
	Use:   "associate",
	Short: "Associate an issue with a release",
	Long: "Associate an issue with a release.\n" +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
	Run: func(cmd *cobra.Command, args []string) {
		issueID, err := cmd.Flags().GetInt("issue")
		if err != nil {
			fmt.Printf("Error converting Redmine issue ID to integer: %s", err)
			os.Exit(1)
		}

		releaseID, err := cmd.Flags().GetInt("release")
		if err != nil {
			fmt.Printf("Error converting Redmine release ID to integer: %s", err)
			os.Exit(1)
		}

		redmine := redmine.NewClient(conf.Endpoint, conf.Apikey)

		i, err := redmine.GetIssue(issueID)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			os.Exit(1)
		}

		var setIt bool
		if i.Release == nil || i.Release["release"].ID == 0 {
			setIt = true
		} else if i.Release["release"].ID != releaseID {
			setIt = true
		}
		if setIt {
			err = redmine.SetRelease(*i, releaseID)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				os.Exit(1)
			} else {
				fmt.Printf("[changed] release for issue %d set to %d\n", i.ID, releaseID)
			}
		} else {
			fmt.Printf("[ok] release for issue %d was already set to %d, not updating\n", i.ID, i.Release["release"].ID)
		}
	},
}


var setIssueSprintCmd = &cobra.Command{
	Use:   "set-sprint",
	Short: "Set sprint for issue",
	Long: "Set the sprint for an issue.\n" +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
	Run: func(cmd *cobra.Command, args []string) {
		issueID, err := cmd.Flags().GetInt("issue")
		if err != nil {
			fmt.Printf("Error converting Redmine issue ID to integer: %s", err)
			os.Exit(1)
		}

		sprintID, err := cmd.Flags().GetInt("sprint")
		if err != nil {
			fmt.Printf("Error converting Redmine sprint ID to integer: %s", err)
			os.Exit(1)
		}

		redmine := redmine.NewClient(conf.Endpoint, conf.Apikey)

		i, err := redmine.GetIssue(issueID)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			os.Exit(1)
		}

		var setIt bool
		if i.FixedVersion == nil {
			setIt = true
		} else if i.FixedVersion.ID != sprintID {
			setIt = true
		}
		if setIt {
			err = redmine.SetSprint(*i, sprintID)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				os.Exit(1)
			} else {
				fmt.Printf("[changed] sprint for issue %d set to %d\n", i.ID, sprintID)
			}
		} else {
			fmt.Printf("[ok] sprint for issue %d was already set to %d, not updating\n", i.ID, i.FixedVersion.ID)
		}
	},
}

func checkError(err error) {
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
}

func checkError2(msg string, err error) {
	if err != nil {
		fmt.Printf("%s: %s\n", msg, err.Error())
		os.Exit(1)
	}
}

var findAndAssociateIssuesCmd = &cobra.Command{
	Use:   "find-and-associate",
	Short: "Find all issue numbers to associate with a release, and associate them",
	Long: "Find all issue numbers to associate with a release, and associate them.\n" +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
	Run: func(cmd *cobra.Command, args []string) {
		previousReleaseTag, err := cmd.Flags().GetString("previous-release-tag")
		if err != nil {
			log.Fatal(fmt.Errorf("Error retrieving previous release: %s", err))
			return
		}

		newReleaseCommitHash, err := cmd.Flags().GetString("new-release-commit")
		if err != nil {
			log.Fatal(fmt.Errorf("Error retrieving new release: %s", err))
			return
		}
		releaseID, err := cmd.Flags().GetInt("release")
		if err != nil {
			log.Fatal(fmt.Errorf("Error converting Redmine release ID to integer: %s", err))
			return
		}

		autoSet, err := cmd.Flags().GetBool("auto-set")
		if err != nil {
			log.Fatal(fmt.Errorf("Error getting auto-set value: %s", err))
			return
		}
		skipReleaseChange, err := cmd.Flags().GetBool("skip-release-change")
		if err != nil {
			log.Fatal(fmt.Errorf("Error getting skip-release-change value: %s", err))
			return
		}
		arvRepo, err := cmd.Flags().GetString("source-repo")
		if err != nil {
			log.Fatal(fmt.Errorf("Error getting source-repo value: %s", err))
			return
		}

		if len(previousReleaseTag) < 5 || len(previousReleaseTag) > 8 {
			log.Fatal(fmt.Errorf("The previous-release-tag argument is of an unexpected format. Expecting a semantic version (e.g. 2.3.0)"))
			return
		}
		if len(newReleaseCommitHash) != 7 && len(newReleaseCommitHash) != 40 {
			log.Fatal(fmt.Errorf("The new-release-commit argument is of an unexpected format. Expecting a git commit hash (7 or 40 digits long)"))
			return
		}

		// Clone the repo in memory

		// our own arvados repo won't clone,
		//arvRepo := "https://git.arvados.org/arvados.git"
		//arvRepo := "https://github.com/arvados/arvados.git"

		fmt.Println("Cloning " + arvRepo)
		repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL: arvRepo,
		})
		checkError(err)
		fmt.Println("... done")
		fmt.Println()
		start, err := repo.ResolveRevision(plumbing.Revision("refs/tags/" + previousReleaseTag))
		checkError2("repo.ResolveRevision", err)
		fmt.Printf("previous-release-tag: %s (%s)\n", previousReleaseTag, start)
		fmt.Printf("new-release-commit: %s\n", newReleaseCommitHash)
		fmt.Println()

		// Build the exclusion list
		seen := make(map[plumbing.Hash]bool)
		excludeIter, err := repo.Log(&git.LogOptions{From: *start, Order: git.LogOrderCommitterTime})
		checkError2("repo.Log", err)
		excludeIter.ForEach(func(c *object.Commit) error {
			seen[c.Hash] = true
			return nil
		})

		// isValid returns merge commits that are not in the exclusion list
		var isValid object.CommitFilter = func(commit *object.Commit) bool {
			_, ok := seen[commit.Hash]

			// use len(commit.ParentHashes) to only get merge commits
			return !ok && len(commit.ParentHashes) >= 2
		}

		headCommit, err := repo.CommitObject(plumbing.NewHash(newReleaseCommitHash))
		checkError2("repo.CommitObject", err)

		iter := object.NewFilterCommitIter(headCommit, &isValid, nil)

		issues := make(map[int]string)
		re := regexp.MustCompile(`Merge branch `)
		reNotMain := regexp.MustCompile(`Merge branch .(main|master)`)
		reIssueRef := regexp.MustCompile(`(Closes|closes|Refs|refs|Fixes|fixes) #(\d+)`)
		err = iter.ForEach(func(c *object.Commit) error {
			// We have a git commit hook that requires an issue reference on merge commits
			if re.MatchString(c.Message) && !reNotMain.MatchString(c.Message) {
				m := reIssueRef.FindStringSubmatch(c.Message)
				if len(m) == 3 {
					i, err := strconv.Atoi(m[2])
					if err != nil {
						checkError(err)
					}
					issues[i] = fmt.Sprintf("%s: %s", c.Hash, c.Message)
				}
			}

			if c.Hash == *start {
				return storer.ErrStop
			}
			return nil
		})
		checkError(err)

		// Sort the issue map keys
		keys := make([]int, 0, len(issues))
		for k := range issues {
			keys = append(keys, k)
		}
		sort.Ints(keys)

		r := redmine.NewClient(conf.Endpoint, conf.Apikey)

		for c, k := range keys {
			fmt.Printf("%d (%d/%d): ", k, c+1, len(keys))
			// Look up the issue, see if it is already associated with the desired release

			i, err := r.GetIssue(k)
			if err != nil {
				fmt.Println()
				fmt.Printf("[error] unable to retrieve issue: %s\n", err.Error())
				fmt.Println("============================================")
				continue
			}
			fmt.Println(i.Subject)
			fmt.Println(issues[k])

			if i.Release != nil && i.Release["release"].ID != 0 {
				if i.Release["release"].ID == releaseID {
					fmt.Printf("[ok] release is already set to %d, nothing to do\n", i.Release["release"].ID)
				} else if !skipReleaseChange {
					fmt.Printf("%s/issues/%d\n", conf.Endpoint, k)
					confirm := false
					prompt := &survey.Confirm{
						Message: fmt.Sprintf("release is set to %d, do you want to change it to %d ?", i.Release["release"].ID, releaseID),
					}
					err = survey.AskOne(prompt, &confirm)
					if err != nil {
						log.Fatal(err)
					}
					if confirm {
						err = r.SetRelease(*i, releaseID)
						if err != nil {
							log.Fatal(err)
						} else {
							fmt.Printf("[changed] release for issue %d set to %d\n", i.ID, releaseID)
						}
					}
				} else {
					fmt.Printf("[ok] release is set to %d, not changing it to %d\n", i.Release["release"].ID, releaseID)
				}
			} else {
				fmt.Printf("%s/issues/%d\n", conf.Endpoint, k)
				confirm := false
				if !autoSet {
					prompt := &survey.Confirm{
						Message: fmt.Sprintf("Release is not set, do you want to set it to %d ?", releaseID),
					}
					err = survey.AskOne(prompt, &confirm)
					if err != nil {
						return
					}
				}
				if confirm || autoSet {
					err = r.SetRelease(*i, releaseID)
					if err != nil {
						log.Fatal(err)
					} else {
						fmt.Printf("[changed] release for issue %d set to %d\n", i.ID, releaseID)
					}
				}
			}
			fmt.Println("============================================")
		}
	},
}

var createReleaseIssueCmd = &cobra.Command{
	Use:   "create-release-issue",
	Short: "Create a release ticket with numbered subtasks for all the steps on the release checklist",
	Long: "Create a release ticket with numbered subtasks for all the steps on the release checklist.\n" +
		"\nThe subtask subjects are read from a file named TASKS in the current directory.\n" +
		"\nFinally, a new Redmine release will also be created for the next release.\n" +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
	Run: func(cmd *cobra.Command, args []string) {
		newReleaseVersion, err := cmd.Flags().GetString("new-release-version")
		if err != nil {
			log.Fatal(fmt.Errorf("[error] can not get new release version: %s", err))
			return
		}

		versionID, err := cmd.Flags().GetInt("sprint")
		if err != nil {
			log.Fatal(fmt.Errorf("[error] can not convert Redmine sprint (version) ID to integer: %s", err))
			return
		}
		projectName, err := cmd.Flags().GetString("project")
		if err != nil {
			log.Fatal(fmt.Errorf("[error] can not get Redmine project name: %s", err))
			return
		}

		r := redmine.NewClient(conf.Endpoint, conf.Apikey)

		// Does this project exist?
		project, err := r.GetProjectByName(projectName)
		if err != nil {
			log.Fatalf("[error] can not find project with name %s: %s", projectName, err)
		}

		// Is the sprint (aka "version" in redmine) in the correct state?
		v, err := r.Version(versionID)
		if err != nil {
			log.Fatal(fmt.Errorf("[error] can not find sprint with id %d: %s", versionID, err))
		}
		if v.Status != "open" {
			log.Fatal(fmt.Errorf("[error] the sprint must be open; the status of the sprint with id %d is '%s'", v.ID, v.Status))
		}

		i, err := r.FindOrCreateIssue("Release Arvados "+newReleaseVersion, 0, v.ID, project.ID)
		if err != nil {
			log.Fatal(err)
		}
		if i.Status.Name != "New" {
			log.Fatal(fmt.Errorf("the release ticket status must be 'New'; the status of the release issue with id %d is '%s'", i.ID, v.Status))
		}

		fmt.Printf("[ok] the release ticket is '%s' with ID #%d (%s/issues/%d)\n", i.Subject, i.ID, conf.Endpoint, i.ID)

		// Get the list of subtasks from the "TASKS" file
		tasks, err := os.Open("TASKS")
		if err != nil {
			log.Fatal(fmt.Errorf("[error] unable to open the \"TASKS\" file: %s", err.Error()))
		}
		defer tasks.Close()

		scanner := bufio.NewScanner(tasks)
		count := 1
		for scanner.Scan() {
			task := scanner.Text()
			taskIssue, err := r.FindOrCreateIssue(fmt.Sprintf("%d. %s", count, task), i.ID, v.ID, project.ID)
			fmt.Printf("[ok] #%d: %d. %s\n", taskIssue.ID, count, task)
			count++
			if err != nil {
				log.Fatal(fmt.Errorf("Error reading from file: %s", err))
			}
		}

		// Create the next release in Redmine
		version, err := semver.NewVersion(newReleaseVersion)
		if err != nil {
			log.Fatalf("Error parsing version: %s", err)
		}
		nextVersion := version.IncPatch()

		var release *redmine.Release

		release, err = r.FindReleaseByName(project.Name, "Arvados "+nextVersion.String())
		if err != nil {
			log.Fatalf("Error finding release with name %s in project with name %s: %s", release.Name, project.Name, err)
		}
		if release == nil {
			// No release found, create it
			release = &redmine.Release{}
			release.Name = "Arvados " + nextVersion.String()
			release.Sharing = "hierarchy"
			release.ReleaseStartDate = time.Now().AddDate(0, 0, 7*1).Format("2006-01-02") // arbitrary choice, 1 week from today
			release.ReleaseEndDate = time.Now().AddDate(0, 0, 7*5).Format("2006-01-02")   // also arbitrary, 5 weeks from today
			release.ProjectID = project.ID
			release.Status = "open"
			// Populate Project
			tmp, err := r.GetProject(release.ProjectID)
			if err != nil {
				log.Fatalf("Unable to find project with ID %d: %s", release.ProjectID, err)
			}
			release.Project = &redmine.IDName{ID: release.ProjectID, Name: tmp.Name}

			release, err = r.CreateRelease(*release)
			if err != nil {
				log.Fatalf("Unable to create release: %s", err)
			}
		}
		fmt.Printf("[ok] the redmine release object for the next release is '%s' (%s/rb/release/%d)\n", release.Name, conf.Endpoint, release.ID)
	},
}

var releasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "Manage Redmine releases",
	Long: "Manage Redmine releases.\n" +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
}

var getReleaseCmd = &cobra.Command{
	Use:   "get",
	Short: "get a release",
	Long: "Get a release.\n" +
		"\nThe REDMINE_ENDPOINT environment variable must be set to the base URL of your redmine server." +
		"\nThe REDMINE_APIKEY environment variable must be set to your redmine API key.",
	Run: func(cmd *cobra.Command, args []string) {
		releaseID, err := cmd.Flags().GetInt("release")
		if err != nil {
			fmt.Printf("Error converting Redmine release ID to integer: %s", err)
			os.Exit(1)
		}

		r := redmine.NewClient(conf.Endpoint, conf.Apikey)

		release, err := r.GetRelease(releaseID)
		if err != nil {
			log.Fatalf("Error finding release with id %d: %s", releaseID, err)
		}
		releaseStr, err := json.MarshalIndent(release, "", "  ")
		if err != nil {
			log.Fatalf("Error decoding release with id %d: %s", releaseID, err)
		}
		fmt.Println(string(releaseStr))

	},
}
