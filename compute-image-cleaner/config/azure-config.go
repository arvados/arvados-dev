// Copyright (C) The Azure-Samples Authors. All rights reserved.
//
// SPDX-License-Identifier: MIT

// Largely borrowed from
// https://github.com/Azure-Samples/azure-sdk-for-go-samples/tree/master/internal/config

package config

import (
  "fmt"
  "os"

  "github.com/Azure/go-autorest/autorest/azure"
)

var (
  clientID               string
  clientSecret           string
  tenantID               string
  subscriptionID         string
  cloudName              string = "AzurePublicCloud"
  useDeviceFlow          bool
  environment            *azure.Environment
)

// ClientID is the OAuth client ID.
func ClientID() string {
  return clientID
}

// ClientSecret is the OAuth client secret.
func ClientSecret() string {
  return clientSecret
}

// TenantID is the AAD tenant to which this client belongs.
func TenantID() string {
  return tenantID
}

// SubscriptionID is a target subscription for Azure resources.
func SubscriptionID() string {
  return subscriptionID
}

// UseDeviceFlow specifies if interactive auth should be used. Interactive
// auth uses the OAuth Device Flow grant type.
func UseDeviceFlow() bool {
  return useDeviceFlow
}

// Environment returns an `azure.Environment{...}` for the current cloud.
func Environment() *azure.Environment {
  if environment != nil {
    return environment
  }
  env, err := azure.EnvironmentFromName(cloudName)
  if err != nil {
    // TODO: move to initialization of var
    panic(fmt.Sprintf(
      "invalid cloud name '%s' specified, cannot continue\n", cloudName))
  }
  environment = &env
  return environment
}

// ParseEnvironment loads the Azure environment variables for authentication
func ParseEnvironment() error {
  // these must be provided by environment
  // clientID
  clientID = os.Getenv("AZURE_CLIENT_ID")

  // clientSecret
  clientSecret = os.Getenv("AZURE_CLIENT_SECRET")

  // tenantID (AAD)
  tenantID = os.Getenv("AZURE_TENANT_ID")

  // subscriptionID (ARM)
  subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")

  return nil
}
