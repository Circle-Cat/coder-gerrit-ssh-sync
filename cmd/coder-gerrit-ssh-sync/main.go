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

// coderClient connects with Coder API to sent requests
type coderClient struct{
    url string
    token string
    client *http.Client
}

type gerritClient struct{
    client *gerrit.Client
}

// Returns a pointer coderClient (reference)
func newCoderClient(url string, token string) *coderClient {
    return &coderClient {
        url: url,
        token: token,
        client: http.DefaultClient, // Assign http global client reference to client
    }
}

func newGerritClient(ctx context.Context, url string, gerritUsername string, gerritPassword string) (*gerritClient, error) {

    var err error
    client, err := gerrit.NewClient(ctx, url, nil)
    if err != nil {
        log.Fatalf("Create Gerrit client: %v", err)
    }
    if gerritUsername != "" && gerritPassword != "" {
        client.Authentication.SetBasicAuth(gerritUsername, gerritPassword)
    }

    return &gerritClient {
        client: client,
    }, nil
}

// Note: why not put pass coderClient as an input variable, instead build get as a method for coderClient
// Encapsulation: If method operate entirely on that data then build as method.
func (c *coderClient) get(ctx context.Context, path string, target any) error {
    req, err := http.NewRequestWithContext(ctx, "GET", c.url + path, nil)
    if err != nil {
        return err
    }

    req.Header.Set("Accept", "application/json")
    req.Header.Set("Coder-Session-Token", c.token)
    resp, err := c.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
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

func addSSHKey(ctx context.Context, account *gerrit.AccountInfo, key *coderUserGitSSHKeyResponse, gClient *gerritClient) error {
	pieces := strings.SplitN(strings.TrimSpace(key.PublicKey), " ", 3)
	if len(pieces) == 2 {
		pieces = append(pieces, "coder-sync")
	}
	keyStr := strings.Join(pieces, " ")

	log.Printf("Adding SSH key to Gerrit AccountID %d: %s", account.AccountID, keyStr)
	req, err := gClient.client.NewRawPutRequest(ctx, fmt.Sprintf("/accounts/%d/sshkeys", account.AccountID), keyStr)
	if err != nil {
		return err
	}

	req.Method = "POST"
	req.Header.Set("Content-Type", "text/plain")

	var resp gerrit.SSHKeyInfo
	if _, err := gClient.client.Do(req, &resp); err != nil {
		return err
	}

	log.Printf("Added SSH key: %v", resp)
	return nil
}

func syncUser(ctx context.Context, cClient *coderClient, gClient *gerritClient, user *coderUser) error {

	// Make API call to search gerrit account using email.
	log.Printf("Syncing user %q", user)
	gus, _, err := gClient.client.Accounts.QueryAccounts(ctx, &gerrit.QueryAccountOptions{
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
	if err := cClient.get(ctx, fmt.Sprintf("/api/v2/users/%s/gitsshkey", user.ID), &key); err != nil {
			return fmt.Errorf("get Coder Git SSH key: %w", err)
	}
	log.Printf("Got Git SSH key for user %q: %s", user, key.PublicKey)

	var errs []error
	for _, gu := range *gus {
		log.Printf("Got Gerrit user AccountID %d for Coder user %q", gu.AccountID, user)
		errs = append(errs, addSSHKey(ctx, &gu, &key, gClient))
	}
	return errors.Join(errs...)
}

func main() {

	ctx := context.Background()
	coderURL := flag.String("coder", "", "Base URL for Coder instance")
	filterOnly := flag.String("only", "", "Work on this specific user only for testing")
	gerritInstance := flag.String("gerrit", "", "Base URL for Gerrit instance") // same as gerritInstance

	log.Printf("version: %s\n", version.Version)

	flag.Parse()
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		log.Printf("FLAG: --%s=%q", f.Name, f.Value)
	})

		// Create gerrit client and authorization if username and password provided.
	gerritUsername := os.Getenv("GERRIT_USERNAME")
	gerritPassword := os.Getenv("GERRIT_PASSWORD")
	gClient, err := newGerritClient(ctx, *gerritInstance, gerritUsername, gerritPassword)

	if err != nil {
		log.Fatalf("Failed to initialize Gerrit client: %v", err)
	}

	gv, _, err := gClient.client.Config.GetVersion(ctx)
	if err != nil {
		log.Fatalf("Check Gerrit version: %v", err)
	}
	log.Printf("Gerrit version: %s", gv)

	token := os.Getenv("CODER_SESSION_TOKEN")
	if token == "" {
		fmt.Println("Error: CODER_SESSION_TOKEN is not set")
		return
	}

	cClient := newCoderClient(*coderURL, token)

	var bi coderBuildInfoResponse
	if err := cClient.get(ctx, "/api/v2/buildinfo", &bi); err != nil {
		log.Fatalf("Check Coder version: %v", err)
	}
	log.Printf("Coder version: %s", bi.Version)

	var cus coderUsersResponse
	if err := cClient.get(ctx, "/api/v2/users", &cus); err != nil {
		log.Fatalf("List Coder users: %v", err)
	}
	log.Printf("Coder version: %s", cus.Users)

	for _, cu := range cus.Users {
		if *filterOnly != "" && cu.Email != *filterOnly {
			continue
		}
		if err := syncUser(ctx, cClient, gClient, &cu); err != nil {
			log.Printf("Error syncing user %q: %v", cu, err)
		}
	}
}
