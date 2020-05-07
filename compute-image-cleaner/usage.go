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
compute-image-cleaner removes old compute images from the specified storage account/container.

The following environment variables must be set:

  AZURE_TENANT_ID
  AZURE_SUBSCRIPTION_ID
  AZURE_CLIENT_ID
  AZURE_CLIENT_SECRET

For more information about those values and for instructions to create a service principal, see
https://docs.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal

Usage:
`)
	fs.PrintDefaults()
}
