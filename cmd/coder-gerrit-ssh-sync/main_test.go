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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockGerritClient struct {
	mock.Mock
	QueryResult  []gerrit.AccountInfo
	QueryErr     error
	CallCount    int
	AddSSHKeyErr error
}

// QueryAccounts simulates the QueryAccounts in Gerrit and returns preconfigured mock data and errors.
func (m *MockGerritClient) QueryAccounts(ctx context.Context, opts *gerrit.QueryAccountOptions) (*[]gerrit.AccountInfo, *gerrit.Response, error) {

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

// AddSSHKey simulate AddSSHKey in Gerrit and return preconfigured mock data and errors.
func (m *MockGerritClient) AddSSHKey(ctx context.Context, accountID string, sshKey string) (*gerrit.SSHKeyInfo, *gerrit.Response, error) {
	m.CallCount++
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
		expectCalls  int
		expectedID   string
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
			expectCalls: 1,
			expectedID:  "123",
			expectedIDs: nil,
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
			expectErr:   true,
			expectCalls: 0,
			expectedID:  "123",
			expectedIDs: nil,
			expectedKey: "ssh-rsa AAAAB3NzaC1yc2E",
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
			expectErr:   false,
			expectCalls: 0,
			expectedID:  "123",
			expectedIDs: nil,
			expectedKey: "ssh-rsa AAAAB3NzaC1yc2E",
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
			expectErr:   true,
			expectCalls: 0,
			expectedID:  "123",
			expectedIDs: nil,
			expectedKey: "ssh-rsa AAAAB3NzaC1yc2E",
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
			expectCalls: 1,
			expectedID:  "123",
			expectedIDs: nil,
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
			expectCalls: 2,
			expectedID:  "",
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
			expectErr:   false,
			expectCalls: 0,
			expectedID:  "",
			expectedIDs: nil,
			expectedKey: "ssh-rsa AAAAB3NzaC1yc2E",
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
			expectErr:   true,
			expectCalls: 0,
			expectedID:  "",
			expectedIDs: nil,
			expectedKey: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tc.mockResponse))
			defer server.Close()

			mockCoderClient := coderclient.NewCoderClient(server.URL, "test-token")

			if tc.expectCalls > 0 {

				loopExpectedIDs := []string{tc.expectedID}
				if tc.expectCalls > 1 {
					loopExpectedIDs = tc.expectedIDs
				}
				for _, tmpExpectedID := range loopExpectedIDs {
					tc.mockGerrit.On("AddSSHKey", ctx, tmpExpectedID, tc.expectedKey).
						Return(&gerrit.SSHKeyInfo{}, &gerrit.Response{}, tc.mockGerrit.AddSSHKeyErr).
						Times(tc.expectCalls)
				}
			}

			err := syncUser(ctx, mockCoderClient, tc.mockGerrit, tc.user)

			t.Logf("This is a log message: %v", err)

			if err == nil && tc.expectErr {
				t.Errorf("Expected an error but got none")
			}

			if err != nil && !tc.expectErr {
				t.Errorf("Did not expect an error but got one: %v", err)
			}

			assert.Equal(t, tc.expectCalls, tc.mockGerrit.CallCount)

			if tc.expectCalls == 0 {
				tc.mockGerrit.AssertNotCalled(t, "AddSSHKey")
				return
			}

			// Test the return variables when expectCalls >= 1.
			loopExpectedIDs := []string{tc.expectedID}
			if tc.expectCalls > 1 {
				loopExpectedIDs = tc.expectedIDs
			}
			for _, tmpExpectedID := range loopExpectedIDs {
				tc.mockGerrit.AssertCalled(t, "AddSSHKey", ctx, tmpExpectedID, tc.expectedKey)
			}
		})
	}
}
