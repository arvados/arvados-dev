// Copyright (C) The Azure-Samples Authors. All rights reserved.
//
// SPDX-License-Identifier: MIT

// Largely borrowed from
// https://github.com/Azure-Samples/azure-sdk-for-go-samples/blob/master/internal/iam/authorizers.go

package main

import (
	"fmt"
	"log"

	"github.com/arvados/arvados-dev/compute-image-cleaner/config"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

// OAuthGrantType specifies which grant type to use.
type OAuthGrantType int

const (
	// OAuthGrantTypeServicePrincipal for client credentials flow
	OAuthGrantTypeServicePrincipal OAuthGrantType = iota
	// OAuthGrantTypeDeviceFlow for device flow
	OAuthGrantTypeDeviceFlow
)

var (
	armAuthorizer autorest.Authorizer
)

// GrantType returns what grant type has been configured.
func grantType() OAuthGrantType {
	if config.UseDeviceFlow() {
		return OAuthGrantTypeDeviceFlow
	}
	return OAuthGrantTypeServicePrincipal
}

func getAuthorizerForResource(grantType OAuthGrantType, resource string) (autorest.Authorizer, error) {
	var a autorest.Authorizer
	var err error

	switch grantType {

	case OAuthGrantTypeServicePrincipal:
		oauthConfig, err := adal.NewOAuthConfig(
			config.Environment().ActiveDirectoryEndpoint, config.TenantID())
		if err != nil {
			return nil, err
		}

		token, err := adal.NewServicePrincipalToken(
			*oauthConfig, config.ClientID(), config.ClientSecret(), resource)
		if err != nil {
			return nil, err
		}
		a = autorest.NewBearerAuthorizer(token)

	case OAuthGrantTypeDeviceFlow:
		deviceconfig := auth.NewDeviceFlowConfig(config.ClientID(), config.TenantID())
		deviceconfig.Resource = resource
		a, err = deviceconfig.Authorizer()
		if err != nil {
			return nil, err
		}

	default:
		return a, fmt.Errorf("invalid grant type specified")
	}

	return a, err
}

// GetResourceManagementAuthorizer gets an OAuthTokenAuthorizer for Azure Resource Manager
func GetResourceManagementAuthorizer() (autorest.Authorizer, error) {
	if armAuthorizer != nil {
		return armAuthorizer, nil
	}

	var a autorest.Authorizer
	var err error

	a, err = getAuthorizerForResource(
		grantType(), config.Environment().ResourceManagerEndpoint)

	if err == nil {
		// cache
		armAuthorizer = a
	} else {
		// clear cache
		armAuthorizer = nil
	}
	return armAuthorizer, err
}

func getStorageAccountsClient() storage.AccountsClient {
	storageAccountsClient := storage.NewAccountsClient(config.SubscriptionID())
	auth, err := GetResourceManagementAuthorizer()
	if err != nil {
		log.Fatal(err)
	}
	storageAccountsClient.Authorizer = auth
	err = storageAccountsClient.AddToUserAgent("compute-image-cleaner")
	if err != nil {
		log.Fatal(err)
	}
	return storageAccountsClient
}
