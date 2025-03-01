package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andygrunwald/go-gerrit"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/coderclient"
	"github.com/stretchr/testify/mock"
)

type MockGerritClient struct {
	mock.Mock
	QueryResult  []gerrit.AccountInfo
	QueryErr     error
	AddSSHKeyErr error
}

// QueryAccounts simulates the QueryAccounts in Gerrit and returns preconfigured mock data and errors.
func (m *MockGerritClient) QueryAccounts(ctx context.Context, opts *gerrit.QueryAccountOptions) (*[]gerrit.AccountInfo, *gerrit.Response, error) {

	if m.QueryErr != nil {
		return nil, nil, m.QueryErr
	}

	mockResponse := &gerrit.Response{
		Response: &http.Response{
			StatusCode: http.StatusOK,
		},
	}

	return &m.QueryResult, mockResponse, nil
}

// AddSSHKey simulate AddSSHKey in Gerrit and return preconfigured mock data and errors.
func (m *MockGerritClient) AddSSHKey(ctx context.Context, accountID string, sshKey string) (*gerrit.SSHKeyInfo, *gerrit.Response, error) {
	args := m.Called(ctx, accountID, sshKey)

	return args.Get(0).(*gerrit.SSHKeyInfo), args.Get(1).(*gerrit.Response), args.Error(2)
}

func TestSyncUser(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		mockGerrit   *MockGerritClient
		mockResponse func(w http.ResponseWriter, r *http.Request)
		user         *coderclient.CoderUser
		expectErr    bool
		expectedIDs  []string
		expectedKey  string
	}{
		{
			// Successfully sync user.
			name: "Success_sync",
			mockGerrit: &MockGerritClient{
				Mock:         mock.Mock{},
				QueryResult:  []gerrit.AccountInfo{{AccountID: 123}},
				QueryErr:     nil,
				AddSSHKeyErr: nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"public_key": "ssh-rsa AAAAB3NzaC1yc2E"}`)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr:   false,
			expectedIDs: []string{"123"},
			expectedKey: "ssh-rsa AAAAB3NzaC1yc2E",
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
				Mock:         mock.Mock{},
				QueryResult:  []gerrit.AccountInfo{{AccountID: 123}},
				QueryErr:     nil,
				AddSSHKeyErr: fmt.Errorf("failed to add SSH key"),
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"public_key": "ssh-rsa AAAAB3NzaC1yc2E"}`)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr:   true,
			expectedIDs: []string{"123"},
			expectedKey: "ssh-rsa AAAAB3NzaC1yc2E",
		},
		{
			// Multiple AddSSHKey calls.
			name: "AddSSHKey_Extra_Calls",
			mockGerrit: &MockGerritClient{
				Mock: mock.Mock{},
				QueryResult: []gerrit.AccountInfo{
					{AccountID: 123},
					{AccountID: 456},
				},
				QueryErr:     nil,
				AddSSHKeyErr: nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"public_key": "ssh-rsa AAAAB3NzaC1yc2E"}`)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr:   false,
			expectedIDs: []string{"123", "456"},
			expectedKey: "ssh-rsa AAAAB3NzaC1yc2E",
		},
		{
			//  Gerrit accountId is invalid.
			name: "Invalid_AccountID",
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
			expectErr: false,
		},
		{
			// Coder SSH key is missing.
			name: "No_SSHKey",
			mockGerrit: &MockGerritClient{
				QueryResult: []gerrit.AccountInfo{{AccountID: 123}},
				QueryErr:    nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"public_key": ""}`)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr: true,
		},
		{
			//  Inactive Gerrit accountId
			name: "Inactive_AccountID",
			mockGerrit: &MockGerritClient{
				QueryResult: []gerrit.AccountInfo{{AccountID: 123, Inactive: true}},
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tc.mockResponse))
			defer server.Close()

			mockCoderClient := coderclient.NewCoderClient(server.URL, "test-token")

			for _, gid := range tc.expectedIDs {
				tc.mockGerrit.On("AddSSHKey", ctx, gid, tc.expectedKey).
					Return(&gerrit.SSHKeyInfo{}, &gerrit.Response{}, tc.mockGerrit.AddSSHKeyErr).
					Once()
			}

			err := syncUser(ctx, mockCoderClient, tc.mockGerrit, tc.user)

			if err == nil && tc.expectErr {
				t.Errorf("Expected an error but got none")
			}

			if err != nil && !tc.expectErr {
				t.Errorf("Did not expect an error but got one: %v", err)
			}

			tc.mockGerrit.AssertNumberOfCalls(t, "AddSSHKey", len(tc.expectedIDs))

			for _, gid := range tc.expectedIDs {
				tc.mockGerrit.AssertCalled(t, "AddSSHKey", ctx, gid, tc.expectedKey)
			}
		})
	}
}
