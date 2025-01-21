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
	"net/http"
	"os"
	"strings"

	"github.com/andygrunwald/go-gerrit"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/coderclient"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/version"
	flag "github.com/spf13/pflag"
)

// GerritAccountService defines the methods for interacting with Gerrit accounts.
type gerritAccountService interface {

	// nueryAccounts queries Gerrit accounts based on the provided  account options.
	queryAccounts(ctx context.Context, opts *gerrit.QueryAccountOptions) (*[]gerrit.AccountInfo, *gerrit.Response, error)

	// newRawPutRequest creates a HTTP PUT request to update body to specified Gerrit API path.
	newRawPutRequest(ctx context.Context, path string, body string) (*http.Request, error)

	// do executes the provided HTTP request and decode the response.
	do(req *http.Request, v interface{}) (*gerrit.Response, error)
}

// gerritClient is a client for interacting with the Gerrit.
type gerritClient struct {
	client *gerrit.Client
}

// queryAccounts retrieves a list of Gerrit accounts based on specified account options.
func (g *gerritClient) queryAccounts(ctx context.Context, opts *gerrit.QueryAccountOptions) (*[]gerrit.AccountInfo, *gerrit.Response, error) {
	return g.client.Accounts.QueryAccounts(ctx, opts)
}

// newRawPutRequest creates a HTTP PUT request to update body to specified Gerrit API path.
func (g *gerritClient) newRawPutRequest(ctx context.Context, path string, body string) (*http.Request, error) {
	return g.client.NewRawPutRequest(ctx, path, body)
}

// do executes the HTTP request and decode the response.
func (g *gerritClient) do(req *http.Request, v interface{}) (*gerrit.Response, error) {
	return g.client.Do(req, v)
}

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

type coderUserGitSSHKeyResponse struct {
	PublicKey string `json:"public_key"`
}

// newGerritClient initializes and returns a new Gerrit client with authentication.
// It sets up the client using the provided username and password and API endpoint.
func newGerritClient(ctx context.Context, path string, gerritUsername string, gerritPassword string) (*gerritClient, error) {

	// Creates a Gerrit client using the provided base URL path.
	client, err := gerrit.NewClient(ctx, path, nil)
	if err != nil {
		log.Fatalf("Create Gerrit client: %v", err)
	}
	// Set authentication if username and password are provided
	if gerritUsername != "" && gerritPassword != "" {
		client.Authentication.SetBasicAuth(gerritUsername, gerritPassword)
	}

	return &gerritClient{
		client: client,
	}, nil
}

// addSSHKey add a Coder user's SSH key to the Gerrit account specified by account.
// The key parameter contains the SSH key details.
//
// It return an error if the request fails.
func addSSHKey(ctx context.Context, account *gerrit.AccountInfo, key *coderUserGitSSHKeyResponse, gClient gerritAccountService) error {
	pieces := strings.SplitN(strings.TrimSpace(key.PublicKey), " ", 3)
	if len(pieces) == 2 {
		pieces = append(pieces, "coder-sync")
	}
	keyStr := strings.Join(pieces, " ")

	log.Printf("Adding SSH key to Gerrit AccountID %d: %s", account.AccountID, keyStr)
	req, err := gClient.newRawPutRequest(ctx, fmt.Sprintf("/accounts/%d/sshkeys", account.AccountID), keyStr)
	if err != nil {
		return err
	}

	req.Method = http.MethodPost
	req.Header.Set("Content-Type", "text/plain")

	var resp gerrit.SSHKeyInfo
	if _, err := gClient.do(req, &resp); err != nil {
		return err
	}

	log.Printf("Added SSH key: %v", resp)
	return nil
}

// syncUser synchronizes Coder user's SSH key with corresponding Gerrit accounts
// using client.
//
// If any step fails, it returns immediate errors or an aggregated error that
// combines all errors when adding SSH key to Gerrit accounts.
func syncUser(ctx context.Context, client *coderclient.CoderClient, gClient gerritAccountService, user *coderclient.CoderUser) error {
	// Make API call to search gerrit account using email
	log.Printf("Syncing user %q", formatCoderUser(user))
	gus, _, err := gClient.queryAccounts(ctx, &gerrit.QueryAccountOptions{
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

	var key coderUserGitSSHKeyResponse
	if err := client.Get(ctx, fmt.Sprintf("/api/v2/users/%s/gitsshkey", user.ID), &key); err != nil {
		return fmt.Errorf("get Coder Git SSH key: %w", err)
	}
	log.Printf("Got Git SSH key for user %q: %s", formatCoderUser(user), key.PublicKey)

	var errs []error
	for _, gu := range *gus {
		log.Printf("Got Gerrit user AccountID %d for Coder user %q", gu.AccountID, formatCoderUser(user))
		errs = append(errs, addSSHKey(ctx, &gu, &key, gClient))
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

	gv, _, err := gClient.client.Config.GetVersion(ctx)
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
			log.Printf("Error syncing user %q: %v", cu, err)
		}
	}
}
