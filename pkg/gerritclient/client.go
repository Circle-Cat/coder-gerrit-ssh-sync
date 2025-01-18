package gerritclient

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/andygrunwald/go-gerrit"
)

// GerritAccountService defines the methods for interacting with Gerrit accounts.
type GerritAccountService interface {

	// QueryAccounts queries Gerrit accounts based on the provided  account options.
	QueryAccounts(ctx context.Context, opts *gerrit.QueryAccountOptions) (*[]gerrit.AccountInfo, *gerrit.Response, error)

	// NewRawPutRequest creates a HTTP PUT request to update body to specified Gerrit API path.
	NewRawPutRequest(ctx context.Context, path string, body string) (*http.Request, error)

	// Do executes the provided HTTP request and decode the response.
	Do(req *http.Request, v interface{}) (*gerrit.Response, error)
}

// GerritClient is a client for interacting with the Gerrit.
type GerritClient struct {
	Client *gerrit.Client
}

// CoderUserGitSSHKeyResponse stores the public SSH key
type CoderUserGitSSHKeyResponse struct {
	PublicKey string `json:"public_key"`
}

// QueryAccounts retrieves a list of Gerrit accounts based on specified account options.
func (g *GerritClient) QueryAccounts(ctx context.Context, opts *gerrit.QueryAccountOptions) (*[]gerrit.AccountInfo, *gerrit.Response, error) {
	return g.Client.Accounts.QueryAccounts(ctx, opts)
}

// NewRawPutRequest creates a HTTP PUT request to update body to specified Gerrit API path.
func (g *GerritClient) NewRawPutRequest(ctx context.Context, path string, body string) (*http.Request, error) {
	return g.Client.NewRawPutRequest(ctx, path, body)
}

// Do executes the HTTP request and decode the response.
func (g *GerritClient) Do(req *http.Request, v interface{}) (*gerrit.Response, error) {
	return g.Client.Do(req, v)
}

// NewGerritClient initializes and returns a new Gerrit client with authentication.
// It sets up the client using the provided username and password and API endpoint.
func NewGerritClient(ctx context.Context, path string, gerritUsername string, gerritPassword string) (*GerritClient, error) {

	// Creates a Gerrit client using the provided base URL path.
	client, err := gerrit.NewClient(ctx, path, nil)
	if err != nil {
		log.Fatalf("Create Gerrit client: %v", err)
	}
	// Set authentication if username and password are provided
	if gerritUsername != "" && gerritPassword != "" {
		client.Authentication.SetBasicAuth(gerritUsername, gerritPassword)
	}

	return &GerritClient{
		Client: client,
	}, nil
}

// AddSSHKey add a Coder user's SSH key to the Gerrit account specified by account.
// The key parameter contains the SSH key details.
//
// It return an error if the request fails.
func AddSSHKey(ctx context.Context, account *gerrit.AccountInfo, key *CoderUserGitSSHKeyResponse, gClient GerritAccountService, path string) error {

	if path == "" {
		path = fmt.Sprintf("/accounts/%d/sshkeys", account.AccountID)
	}

	pieces := strings.SplitN(strings.TrimSpace(key.PublicKey), " ", 3)
	if len(pieces) == 2 {
		pieces = append(pieces, "coder-sync")
	}
	keyStr := strings.Join(pieces, " ")

	log.Printf("Adding SSH key to Gerrit AccountID %d: %s", account.AccountID, keyStr)
	req, err := gClient.NewRawPutRequest(ctx, path, keyStr)
	if err != nil {
		return err
	}

	req.Method = http.MethodPost
	req.Header.Set("Content-Type", "text/plain")

	var resp gerrit.SSHKeyInfo
	if _, err := gClient.Do(req, &resp); err != nil {
		return err
	}

	log.Printf("Added SSH key: %v", resp)
	return nil
}
