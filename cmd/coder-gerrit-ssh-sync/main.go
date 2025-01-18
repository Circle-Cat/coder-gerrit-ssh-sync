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

	"github.com/andygrunwald/go-gerrit"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/coderclient"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/gerritclient"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/version"
	flag "github.com/spf13/pflag"
)

type config struct {
	coderURL       string
	token          string
	gerritInstance string
	gerritUsername string
	gerritPassword string
	filterOnly     string
}

func formatCoderUser(user *coderclient.CoderUser) string {
	return fmt.Sprintf("%s (%s, %s)", user.Username, user.ID, user.Email)
}

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

// syncUser synchronizes Coder user's SSH key with corresponding Gerrit accounts
// using client.
//
// If any step fails, it returns immediate errors or an aggregated error that
// combines all errors when adding SSH key to Gerrit accounts.
func syncUser(ctx context.Context, cClient *coderclient.CoderClient, gClient gerritclient.GerritAccountService, user *coderclient.CoderUser) error {
	// Make API call to search gerrit account using email
	log.Printf("Syncing user %q", formatCoderUser(user))
	gus, _, err := gClient.QueryAccounts(ctx, &gerrit.QueryAccountOptions{
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

	var key gerritclient.CoderUserGitSSHKeyResponse
	if err := cClient.Get(ctx, fmt.Sprintf("/api/v2/users/%s/gitsshkey", user.ID), &key); err != nil {
		return fmt.Errorf("get Coder Git SSH key: %w", err)
	}
	log.Printf("Got Git SSH key for user %q: %s", formatCoderUser(user), key.PublicKey)

	var errs []error
	for _, gu := range *gus {
		log.Printf("Got Gerrit user AccountID %d for Coder user %q", gu.AccountID, formatCoderUser(user))
		errs = append(errs, gerritclient.AddSSHKey(ctx, &gu, &key, gClient, ""))
	}
	return errors.Join(errs...)
}

func main() {
	ctx := context.Background()
	log.Printf("version: %s\n", version.Version)

	config := parseFlags()

	// Initialize gerrit client
	gClient, err := gerritclient.NewGerritClient(ctx, config.gerritInstance, config.gerritUsername, config.gerritPassword)
	if err != nil {
		log.Fatalf("Failed to initialize Gerrit client: %v", err)
	}

	gv, _, err := gClient.Client.Config.GetVersion(ctx)
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
		if err := syncUser(ctx, cClient, gClient, &cu); err != nil {
			log.Printf("Error syncing user %q: %v", formatCoderUser(&cu), err)
		}
	}
}
