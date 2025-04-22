package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/andygrunwald/go-gerrit"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/pkg/coderclient"
	"github.com/stretchr/testify/mock"
)

type MockGerritClient struct {
	mock.Mock
	QueryResult       []gerrit.AccountInfo
	QueryErr          error
	AddSSHKeyErr      error
	ListSSHKeysResult []gerrit.SSHKeyInfo
	ListSSHKeysErr    error
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

// ListSSHKeys simulates ListSSHKeys in Gerrit and returns preconfigured mock data and errors.
func (m *MockGerritClient) ListSSHKeys(ctx context.Context, accountID string) (*[]gerrit.SSHKeyInfo, *gerrit.Response, error) {
	if m.ListSSHKeysErr != nil {
		return nil, nil, m.ListSSHKeysErr
	}

	mockResponse := &gerrit.Response{
		Response: &http.Response{
			StatusCode: http.StatusOK,
		},
	}

	return &m.ListSSHKeysResult, mockResponse, nil
}

func generateTestSSHKey(t *testing.T) string {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to generate public key: %v", err)
	}

	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
}

func TestSyncUser(t *testing.T) {
	ctx := context.Background()
	testNormalizedSSHKey := generateTestSSHKey(t)

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
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr:   false,
			expectedIDs: []string{"123"},
			expectedKey: testNormalizedSSHKey,
		},
		{
			// QueryAccount failed to retrieve gerrit account.
			name: "Queryaccount_fail",
			mockGerrit: &MockGerritClient{
				QueryResult: nil,
				QueryErr:    errors.New("QuerryAccount failed"),
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
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
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
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
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr:   true,
			expectedIDs: []string{"123"},
			expectedKey: testNormalizedSSHKey,
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
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr:   false,
			expectedIDs: []string{"123", "456"},
			expectedKey: testNormalizedSSHKey,
		},
		{
			//  Gerrit accountId is invalid.
			name: "Invalid_AccountID",
			mockGerrit: &MockGerritClient{
				QueryResult: []gerrit.AccountInfo{{AccountID: -1}},
				QueryErr:    nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
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
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
			},
			user: &coderclient.CoderUser{
				Email:    "test@example.com",
				ID:       "user123",
				Username: "testUser1",
			},
			expectErr: false,
		},
		{
			//  Key Already Exists in Gerrit
			name: "Key_Already_Exists",
			mockGerrit: &MockGerritClient{
				QueryResult:       []gerrit.AccountInfo{{AccountID: 123}},
				ListSSHKeysResult: []gerrit.SSHKeyInfo{{SSHPublicKey: testNormalizedSSHKey}},
				QueryErr:          nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
			},
			user: &coderclient.CoderUser{
				Email: "test@example.com",
				ID:    "user123",
			},
			expectErr: false,
		},
		{
			// Non-active Coder user: Suspended
			name: "Suspended_Coder_User",
			mockGerrit: &MockGerritClient{
				QueryResult: []gerrit.AccountInfo{{AccountID: 123}},
				QueryErr:    nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
			},
			user: &coderclient.CoderUser{
				Email:    "suspendedUser@example.com",
				ID:       "user123",
				Username: "suspendedUser",
				Status:   coderclient.UserStatusSuspended,
			},
			expectErr:   false,
			expectedIDs: []string{},
			expectedKey: testNormalizedSSHKey,
		},
		{
			// Non-active Coder user: Dormant
			name: "Dormant_Coder_User",
			mockGerrit: &MockGerritClient{
				QueryResult: []gerrit.AccountInfo{{AccountID: 123}},
				QueryErr:    nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
			},
			user: &coderclient.CoderUser{
				Email:    "dormantUser@example.com",
				ID:       "user123",
				Username: "dormantUser",
				Status:   coderclient.UserStatusDormant,
			},
			expectErr:   false,
			expectedIDs: []string{},
			expectedKey: testNormalizedSSHKey,
		},
		{
			// Key Already Exists including Comments
			name: "Key_Equality_Ignores_Comment",
			mockGerrit: &MockGerritClient{
				Mock:        mock.Mock{},
				QueryResult: []gerrit.AccountInfo{{AccountID: 123}},
				ListSSHKeysResult: []gerrit.SSHKeyInfo{
					{SSHPublicKey: testNormalizedSSHKey + " some-comment"},
				},
				QueryErr: nil,
			},
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"public_key": "%s"}`, testNormalizedSSHKey)
			},
			user: &coderclient.CoderUser{
				Email:    "comment-test@example.com",
				ID:       "user-comment-test",
				Username: "comment-tester",
			},
			expectErr:   false,
			expectedIDs: []string{},
			expectedKey: testNormalizedSSHKey,
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
				t.Errorf("Did not expect an error but got : %v", err)
			}

			tc.mockGerrit.AssertNumberOfCalls(t, "AddSSHKey", len(tc.expectedIDs))

			for _, gid := range tc.expectedIDs {
				tc.mockGerrit.AssertCalled(t, "AddSSHKey", ctx, gid, tc.expectedKey)
			}
		})
	}
}
