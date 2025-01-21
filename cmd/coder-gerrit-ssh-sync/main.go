/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package main is the entrypoint for coder-gerrit-ssh-sync.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/andygrunwald/go-gerrit"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/coderclient"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/version"
	flag "github.com/spf13/pflag"
)

// gerritAccountsService defines the methods for interacting with Gerrit accounts.
type gerritAccountsService interface {

	// QueryAccounts queries Gerrit accounts based on the provided account options.
	QueryAccounts(ctx context.Context, opts *gerrit.QueryAccountOptions) (*[]gerrit.AccountInfo, *gerrit.Response, error)

	// AddSSHKey add Coder SSH key to corresponding Gerrit accounts.
	AddSSHKey(ctx context.Context, accountID string, sshKey string) (*gerrit.SSHKeyInfo, *gerrit.Response, error)
}

type config struct {
	coderURL       string
	token          string
	gerritInstance string
	gerritUsername string
	gerritPassword string
	filterOnly     string
}

// parseFlags parses command line flags and environment variables to configure the application.
func parseFlags() *config {
	coderURL := flag.String("coder", "", "Base URL for Coder instance")
	token := os.Getenv("CODER_SESSION_TOKEN")
	gerritInstance := flag.String("gerrit", "", "Base URL for Gerrit instance")
	gerritUsername := os.Getenv("GERRIT_USERNAME")
	gerritPassword := os.Getenv("GERRIT_PASSWORD")
	filterOnly := flag.String("only", "", "Work on this specific user only for testing")

	flag.Parse()

	if token == "" {
		log.Fatal("Error: CODER_SESSION_TOKEN is not set")
	}

	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		log.Printf("FLAG: --%s=%q", f.Name, f.Value)
	})

	return &config{
		coderURL:       *coderURL,
		token:          token,
		gerritInstance: *gerritInstance,
		gerritUsername: gerritUsername,
		gerritPassword: gerritPassword,
		filterOnly:     *filterOnly,
	}
}

// newGerritClient initializes and returns a new Gerrit client with authentication.
// It sets up the client using the provided username and password and API endpoint.
func newGerritClient(ctx context.Context, path string, gerritUsername string, gerritPassword string) (*gerrit.Client, error) {

	// Creates a Gerrit client using the provided base URL path.
	client, err := gerrit.NewClient(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create Gerrit client: %w", err)
	}
	// Set authentication if username and password are provided
	if gerritUsername != "" && gerritPassword != "" {
		client.Authentication.SetBasicAuth(gerritUsername, gerritPassword)
	}

	return client, nil
}

// syncUser synchronizes Coder user's SSH key with corresponding Gerrit accounts
// using client.
//
// If any step fails, it returns immediate errors or an aggregated error that
// combines all errors when adding SSH key to Gerrit accounts.
func syncUser(ctx context.Context, client *coderclient.CoderClient, gAccountService gerritAccountsService, user *coderclient.CoderUser) error {
	// Make API call to search gerrit account using email
	log.Printf("Syncing user %q", user)
	gus, _, err := gAccountService.QueryAccounts(ctx, &gerrit.QueryAccountOptions{
		QueryOptions: gerrit.QueryOptions{
			Query: []string{
				fmt.Sprintf("email:%q", user.Email),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("query Gerrit user: %w", err)
	}

	if len(*gus) == 0 {
		log.Printf("No matching Gerrit user for email %q", user.Email)
		return nil
	}

	var key coderclient.CoderUserGitSSHKeyResponse
	if err := client.Get(ctx, fmt.Sprintf("/api/v2/users/%s/gitsshkey", user.ID), &key); err != nil {
		return fmt.Errorf("get Coder Git SSH key: %w", err)
	}
	if key.PublicKey == "" {
		return fmt.Errorf("no SSH key found for user %q", user)
	}
	log.Printf("Got Git SSH key for user %q: %s", user, key.PublicKey)

	var errs []error
	for _, gu := range *gus {

		if gu.AccountID <= 0 {
			log.Printf("Skipping invalid Gerrit user AccountID %d", gu.AccountID)
			continue
		}

		log.Printf("Got Gerrit user AccountID %d for Coder user %q", gu.AccountID, user)
		_, _, err = gAccountService.AddSSHKey(ctx, strconv.Itoa(gu.AccountID), key.PublicKey)

		if err != nil {
			errs = append(errs, fmt.Errorf("Failed to add SSH key for Gerrit user %d: %w", gu.AccountID, err))
			continue
		}
		log.Printf("Added SSH key %q: %v", user, key.PublicKey)

	}
	return errors.Join(errs...)
}

func main() {
	ctx := context.Background()
	log.Printf("version: %s\n", version.Version)

	config := parseFlags()

	// Initialize gerrit client
	gClient, err := newGerritClient(ctx, config.gerritInstance, config.gerritUsername, config.gerritPassword)
	if err != nil {
		log.Fatalf("Failed to initialize Gerrit client: %v", err)
	}

	gv, _, err := gClient.Config.GetVersion(ctx)
	if err != nil {
		log.Fatalf("Check Gerrit version: %v", err)
	}
	log.Printf("Gerrit version: %s", gv)

	cClient := coderclient.NewCoderClient(config.coderURL, config.token)

	var bi coderclient.CoderBuildInfoResponse
	if err := cClient.Get(ctx, "/api/v2/buildinfo", &bi); err != nil {
		log.Fatalf("Check Coder version: %v", err)
	}
	log.Printf("Coder version: %s", bi.Version)

	var cus coderclient.CoderUsersResponse
	if err := cClient.Get(ctx, "/api/v2/users", &cus); err != nil {
		log.Fatalf("List Coder users: %v", err)
	}

	for _, cu := range cus.Users {
		if config.filterOnly != "" && cu.Email != config.filterOnly {
			continue
		}
		if err := syncUser(ctx, cClient, gClient.Accounts, &cu); err != nil {
			log.Printf("Error syncing user %q: %v", cu, err)
		}
	}
}
