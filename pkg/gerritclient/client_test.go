package gerritclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/andygrunwald/go-gerrit"
)

// MockGerritClient is a mocked gerrit client.
type MockGerritClient struct{}

// QueryAccounts simulates the QueryAccounts in Gerrit and returns preconfigured mock data and errors.
func (m *MockGerritClient) QueryAccounts(ctx context.Context, opts *gerrit.QueryAccountOptions) (*[]gerrit.AccountInfo, *gerrit.Response, error) {
	mockResponse := &gerrit.Response{
		Response: &http.Response{
			StatusCode: http.StatusOK,
		},
	}
	return &[]gerrit.AccountInfo{}, mockResponse, nil
}

// NewRawPutRequest mocks a HTTP PUT request to specified Gerrit API path.
func (m *MockGerritClient) NewRawPutRequest(ctx context.Context, path string, body string) (*http.Request, error) {

	if path == "/put_request_nil" {
		return nil, fmt.Errorf("failed to create request for path %s", path)
	}

	fullURL, err := url.JoinPath("http://mock.url", path)
	if err != nil {
		return nil, fmt.Errorf("failed to join URL path: %w", err)
	}

	return http.NewRequestWithContext(ctx, "PUT", fullURL, strings.NewReader(body))
}

// Do mocks the execution of an HTTP request and define mocked response from Gerrit API.
func (m *MockGerritClient) Do(req *http.Request, v interface{}) (*gerrit.Response, error) {

	if req.URL.Path == "/do_return_nil" {
		return nil, errors.New("add SSH key error")
	}

	resp := v.(*gerrit.SSHKeyInfo)
	*resp = gerrit.SSHKeyInfo{
		Seq:        1,
		EncodedKey: "ssh-rsa AAAAB3NzaC1yc2E",
		Comment:    "coder-sync",
		Valid:      true,
	}
	return &gerrit.Response{
		Response: &http.Response{
			StatusCode: http.StatusOK,
		},
	}, nil
}

func TestAddSSHKey(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name       string
		account    *gerrit.AccountInfo
		key        *CoderUserGitSSHKeyResponse
		mockGerrit GerritAccountService
		path       string
		expectErr  bool
	}{
		{
			// Scenario 1: Return version successfully
			name:    "Success_addSSHKey",
			account: &gerrit.AccountInfo{AccountID: 123},
			key: &CoderUserGitSSHKeyResponse{
				PublicKey: "ssh-rsa AAAAB3NzaC1yc2E",
			},
			mockGerrit: &MockGerritClient{},
			path:       "",
			expectErr:  false,
		},
		{
			// Scenario 2: Do function failed
			name:    "Fail_do_function",
			account: &gerrit.AccountInfo{AccountID: 123},
			key: &CoderUserGitSSHKeyResponse{
				PublicKey: "ssh-rsa AAAAB3NzaC1yc2E",
			},
			mockGerrit: &MockGerritClient{},
			path:       "/do_return_nil",
			expectErr:  true,
		},
		{
			// Scenario 3: NewRawPutRequest failed
			name:    "Fail_put_request_function",
			account: &gerrit.AccountInfo{AccountID: 123},
			key: &CoderUserGitSSHKeyResponse{
				PublicKey: "ssh-rsa AAAAB3NzaC1yc2E",
			},
			mockGerrit: &MockGerritClient{},
			path:       "/put_request_nil",
			expectErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			err := AddSSHKey(ctx, tc.account, tc.key, tc.mockGerrit, tc.path)

			if err != nil && !tc.expectErr {
				t.Errorf("Did not expect an error but got one: %v", err)
			}

			if err == nil && tc.expectErr {
				t.Errorf("Expected an error but got none")
			}
		})
	}
}
