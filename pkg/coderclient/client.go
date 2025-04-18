package coderclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusDormant   UserStatus = "dormant"
	UserStatusSuspended UserStatus = "suspended"
)

// CoderClient is a client for interacting with the Coder API.
type CoderClient struct {
	// url is the base URL of Coder API.
	url string

	// token is the authentication token for API requests.
	token string

	// client is the HTTP client used to make requests to Coder API.
	client *http.Client
}

// CoderBuildInfoResponse includes the version of the Coder system.
type CoderBuildInfoResponse struct {
	Version string `json:"version"`
}

// CoderUsersResponse represents list of users retrieved from API.
type CoderUsersResponse struct {
	Users []CoderUser `json:"users"`
}

// CoderUserGitSSHKeyResponse includes user SSH key.
type CoderUserGitSSHKeyResponse struct {
	PublicKey string `json:"public_key"`
}

// CoderUser represents the user details retrieved from Coder API.
type CoderUser struct {
	Email    string     `json:"email"`
	ID       string     `json:"id"`
	Username string     `json:"username"`
	Status   UserStatus `json:"status"`
}

// NewCoderClient returns a pointer coderClient (reference).
func NewCoderClient(url string, token string) *CoderClient {
	return &CoderClient{
		url:    url,
		token:  token,
		client: http.DefaultClient, // Assign http global client reference to client
	}
}

func (u *CoderUser) String() string {
	return fmt.Sprintf("%s (%s, %s, %s)", u.Username, u.ID, u.Email, u.Status)
}

// Get sends an HTTP GET request to the specified path using the coderClient.
// It decodes the JSON response into the target variable.
func (c *CoderClient) Get(ctx context.Context, path string, target any) error {

	fullURL, err := url.JoinPath(c.url, path)
	if err != nil {
		return fmt.Errorf("failed to join URL path: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Coder HTTP status: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}
