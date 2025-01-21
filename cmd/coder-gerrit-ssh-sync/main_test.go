package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andygrunwald/go-gerrit"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/coderclient"
)

type MockGerritClient struct {
	QueryResult []gerrit.AccountInfo
	QueryErr    error
}

// queryAccounts simulates the QueryAccounts in Gerrit and returns preconfigured mock data and errors.
func (m *MockGerritClient) queryAccounts(ctx context.Context, opts *gerrit.QueryAccountOptions) (*[]gerrit.AccountInfo, *gerrit.Response, error) {

	if m.QueryErr != nil {
		return nil, nil, m.QueryErr
	}

	if m.QueryResult == nil {
		return &[]gerrit.AccountInfo{}, nil, nil
	}

	mockResponse := &gerrit.Response{
		Response: &http.Response{
			StatusCode: http.StatusOK,
		},
	}

	return &m.QueryResult, mockResponse, nil
}

// newRawPutRequest mocks a HTTP PUT request to specified Gerrit API path.
func (m *MockGerritClient) newRawPutRequest(ctx context.Context, path string, body string) (*http.Request, error) {

	if strings.Contains(path, "/accounts/-1/") {
		return nil, fmt.Errorf("failed to create request for path: %s", path)
	}

	return http.NewRequestWithContext(ctx, http.MethodPut, path, strings.NewReader(body))
}

// do mocks the execution of an HTTP request and define mocked response from Gerrit API.
func (m *MockGerritClient) do(req *http.Request, v interface{}) (*gerrit.Response, error) {

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

func TestSyncUser(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		mockGerrit   *MockGerritClient
		mockResponse func(w http.ResponseWriter, r *http.Request)
		user         *coderclient.CoderUser
		expectErr    bool
	}{
		{
			// Successfully sync user.
			name: "Success_sync",
			mockGerrit: &MockGerritClient{
				QueryResult: []gerrit.AccountInfo{{AccountID: 123}},
				QueryErr:    nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"public_key": "ssh-rsa AAAAB3NzaC1yc2E"}`)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr: false,
		},
		{
			// QueryAccount failed to retrieve gerrit account.
			name: "Queryaccount_fail",
			mockGerrit: &MockGerritClient{
				QueryResult: nil,
				QueryErr:    errors.New("QuerryAccount failed"),
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"public_key": "ssh-rsa AAAAB3NzaC1yc2E"}`)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr: true,
		},
		{
			// Gerrit account not found
			name: "User_not_exist",
			mockGerrit: &MockGerritClient{
				QueryResult: nil,
				QueryErr:    nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"public_key": "ssh-rsa AAAAB3NzaC1yc2E"}`)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr: false,
		},
		{
			// Failed to retrieve Coder SSH key
			name: "CoderGet_fail",
			mockGerrit: &MockGerritClient{
				QueryResult: []gerrit.AccountInfo{{AccountID: 123}},
				QueryErr:    nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr: true,
		},
		{
			// Failed to add SSH key to Gerrit.
			name: "AddSSHKey_fail",
			mockGerrit: &MockGerritClient{
				QueryResult: []gerrit.AccountInfo{{AccountID: -1}},
				QueryErr:    nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"public_key": "ssh-rsa AAAAB3NzaC1yc2E"}`)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tc.mockResponse))
			defer server.Close()

			mockCoderClient := coderclient.NewCoderClient(server.URL, "test-token")
			err := syncUser(ctx, mockCoderClient, tc.mockGerrit, tc.user)

			t.Logf("This is a log message: %v", err)

			if err == nil && tc.expectErr {
				t.Errorf("Expected an error but got none")
			}

			if err != nil && !tc.expectErr {
				t.Errorf("Did not expect an error but got one: %v", err)
			}
		})
	}
}
