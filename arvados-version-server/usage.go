// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"flag"
	"fmt"
	"os"
)

var exampleConfigFile = []byte(`
dirPath: "/tmp/arvados-version-server-checkout"
cacheDirPath: "/tmp/arvados-version-server-cache"
gitExecutablePath: "/usr/bin/git"
listenPort: 8080
shortHashLength: 7
`)

func usage(fs *flag.FlagSet) {
	fmt.Fprintf(os.Stderr, `
Arvados Version Server is a JSON REST service that generates package version
numbers for a given git commit hash.

Options:
`)
	fs.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Example config file:
%s
`, exampleConfigFile)
}
