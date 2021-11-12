// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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
}

var rootCmd = &cobra.Command{
	Use:   "art",
	Short: "art - Arvados Release Tool",
	Long: `
art (Arvados Release Tool) supports the Arvados development process

https://git.arvados.org/arvados-dev.git/cmd/art`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func Execute() {
	conf = loadConfig()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
