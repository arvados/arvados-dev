// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package main

import (
  "flag"
  "fmt"
  "os"
)

func usage(fs *flag.FlagSet) {
  fmt.Fprintf(os.Stderr, `
Cost Analyzer analyzes the cost of an Arvados container request and all its children.

Options:
`)
  fs.PrintDefaults()
}
