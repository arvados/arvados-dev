// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"

	"git.arvados.org/arvados-dev.git/lib/redmine"
	survey "github.com/AlecAivazis/survey/v2"
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
	issuesCmd.AddCommand(findAndAssociateIssuesCmd)
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

func checkError(err error) {
	if err != nil {
		fmt.Printf("%s\n", err.Error())
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

		if len(previousReleaseTag) < 5 || len(previousReleaseTag) > 8 {
			log.Fatal(fmt.Errorf("The previous-release-tag argument is of an unexpected format. Expecting a semantic version (e.g. 2.3.0)"))
			return
		}
		if len(newReleaseCommitHash) != 7 && len(newReleaseCommitHash) != 40 {
			log.Fatal(fmt.Errorf("The new-release-commit argument is of an unexpected format. Expecting a git commit hash (7 or 40 digits long)"))
			return
		}

		// Clone the repo in memory
		fmt.Println("Cloning https://github.com/arvados/arvados.git")
		repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL: "https://github.com/arvados/arvados.git",
		})
		checkError(err)
		fmt.Println("... done")
		fmt.Println()
		start, err := repo.ResolveRevision(plumbing.Revision("refs/tags/" + previousReleaseTag))
		checkError(err)
		fmt.Printf("previous-release-tag: %s (%s)\n", previousReleaseTag, start)
		fmt.Printf("new-release-commit: %s\n", newReleaseCommitHash)
		fmt.Println()

		// Build the exclusion list
		seen := make(map[plumbing.Hash]bool)
		excludeIter, err := repo.Log(&git.LogOptions{From: *start, Order: git.LogOrderCommitterTime})
		checkError(err)
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
		checkError(err)

		iter := object.NewFilterCommitIter(headCommit, &isValid, nil)

		issues := make(map[int]bool)
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
					issues[i] = true
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

		redmine := redmine.NewClient(conf.Endpoint, conf.Apikey)

		for c, k := range keys {
			fmt.Printf("%d (%d/%d): ", k, c+1, len(keys))
			// Look up the issue, see if it is already associated with the desired release

			i, err := redmine.GetIssue(k)
			if err != nil {
				fmt.Println()
				fmt.Printf("[error] unable to retrieve issue: %s\n", err.Error())
				fmt.Println("============================================")
				continue
			}
			fmt.Println(i.Subject)

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
						err = redmine.SetRelease(*i, releaseID)
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
					err = redmine.SetRelease(*i, releaseID)
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
