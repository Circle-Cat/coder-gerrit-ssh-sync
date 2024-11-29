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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/andygrunwald/go-gerrit"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/version"
	flag "github.com/spf13/pflag"
)

var (
	coderInstance  = flag.String("coder", "", "Base URL for Coder instance")
	gerritInstance = flag.String("gerrit", "", "Base URL for Gerrit instance")
	filterOnly     = flag.String("only", "", "Work on this specific user only for testing")

	coderToken     = os.Getenv("CODER_SESSION_TOKEN")
	gerritUsername = os.Getenv("GERRIT_USERNAME")
	gerritPassword = os.Getenv("GERRIT_PASSWORD")

	gerritClient *gerrit.Client
)

func coderGet(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *coderInstance+path, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Coder-Session-Token", coderToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Coder HTTP status: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

type coderBuildInfoResponse struct {
	Version string `json:"version"`
}

type coderUsersResponse struct {
	Users []coderUser `json:"users"`
}

type coderUser struct {
	Email    string `json:"email"`
	ID       string `json:"id"`
	Username string `json:"username"`
}

func (u *coderUser) String() string {
	return fmt.Sprintf("%s (%s, %s)", u.Username, u.ID, u.Email)
}

type coderUserGitSSHKeyResponse struct {
	PublicKey string `json:"public_key"`
}

func addSSHKey(ctx context.Context, account *gerrit.AccountInfo, key *coderUserGitSSHKeyResponse) error {
	pieces := strings.SplitN(strings.TrimSpace(key.PublicKey), " ", 3)
	if len(pieces) == 2 {
		pieces = append(pieces, "coder-sync")
	}
	keyStr := strings.Join(pieces, " ")

	log.Printf("Adding SSH key to Gerrit AccountID %d: %s", account.AccountID, keyStr)
	req, err := gerritClient.NewRawPutRequest(ctx, fmt.Sprintf("/accounts/%d/sshkeys", account.AccountID), keyStr)
	if err != nil {
		return err
	}

	req.Method = http.MethodPost
	req.Header.Set("Content-Type", "text/plain")

	var resp gerrit.SSHKeyInfo
	if _, err := gerritClient.Do(req, &resp); err != nil {
		return err
	}

	log.Printf("Added SSH key: %v", resp)
	return nil
}

func syncUser(ctx context.Context, user *coderUser) error {
	log.Printf("Syncing user %q", user)
	gus, _, err := gerritClient.Accounts.QueryAccounts(ctx, &gerrit.QueryAccountOptions{
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
	if err := coderGet(ctx, fmt.Sprintf("/api/v2/users/%s/gitsshkey", user.ID), &key); err != nil {
		return fmt.Errorf("get Coder Git SSH key: %w", err)
	}
	log.Printf("Got Git SSH key for user %q: %s", user, key.PublicKey)

	var errs []error
	for _, gu := range *gus {
		log.Printf("Got Gerrit user AccountID %d for Coder user %q", gu.AccountID, user)
		errs = append(errs, addSSHKey(ctx, &gu, &key))
	}
	return errors.Join(errs...)
}

func main() {
	ctx := context.Background()
	log.Printf("version: %s\n", version.Version)

	flag.Parse()
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		log.Printf("FLAG: --%s=%q", f.Name, f.Value)
	})

	var err error
	gerritClient, err = gerrit.NewClient(ctx, *gerritInstance, nil)
	if err != nil {
		log.Fatalf("Create Gerrit client: %v", err)
	}
	if gerritUsername != "" && gerritPassword != "" {
		gerritClient.Authentication.SetBasicAuth(gerritUsername, gerritPassword)
	}

	gv, _, err := gerritClient.Config.GetVersion(ctx)
	if err != nil {
		log.Fatalf("Check Gerrit version: %v", err)
	}
	log.Printf("Gerrit version: %s", gv)

	var bi coderBuildInfoResponse
	if err := coderGet(ctx, "/api/v2/buildinfo", &bi); err != nil {
		log.Fatalf("Check Coder version: %v", err)
	}
	log.Printf("Coder version: %s", bi.Version)

	var cus coderUsersResponse
	if err := coderGet(ctx, "/api/v2/users", &cus); err != nil {
		log.Fatalf("List Coder users: %v", err)
	}

	for _, cu := range cus.Users {
		if *filterOnly != "" && cu.Email != *filterOnly {
			continue
		}
		if err := syncUser(ctx, &cu); err != nil {
			log.Printf("Error syncing user %q: %v", cu, err)
		}
	}
}
