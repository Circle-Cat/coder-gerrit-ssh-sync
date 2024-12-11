package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jingyuanliang/coder-gerrit-ssh-sync/cmd/coder-gerrit-ssh-sync/coderClient"
)

func TestGet(t *testing.T) {
	ctx := context.Background()

	testCases := []struct{
		name string
		mockResponse func(w http.ResponseWriter, r *http.Request)
		expected coderBuildInfoResponse
		expectErr bool
		inputPath string
		baseURL string
	}{
		{
			// Scenario 1: Return version successfully
			name: "Success_response",
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK) // set HTTP 200 means success.
				fmt.Fprintln(w, `{"Version": "1.0.0"}`) // writes data to w, mimic return file from the server.
			},
			expected: coderBuildInfoResponse{Version: "1.0.0"},
			expectErr: false,
			inputPath: "/api/v2/buildinfo",
		},
		{
			// Scenario 2: Server was reached and understood the request, but can't find
			// resource to return (invalid path)
			name: "Not_found_404",
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound) // 404 means requested resources not found on server
			},
			expected: coderBuildInfoResponse{},
			expectErr: true,
			inputPath: "/api/v2/invalidPath",
		},
		{
            // Scenario 3: Server was reach and understood the request, but fail to
			// return the Version. Decode will return "invalid character '}' looking for
			// beginning of value" automatically
			name: "Invalid_json",
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, `{"Version":}`) // Version empty will trigger invalid character'}'
			},
			expected: coderBuildInfoResponse{},
			expectErr: true,
			inputPath: "/api/v2/buildinfo",
        },
		{
			// Scenario 4: Invalid URL error
			name: "Invalid_url",
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
			},
			expected: coderBuildInfoResponse{},
			expectErr: true,
			inputPath: "/api/v2/buildinfo",
			baseURL: "http://:%22",
		},
		{
			// Scenario 5: Unreachable address error
			name: "Unreachable_address",
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
			},
			expected: coderBuildInfoResponse{},
			expectErr: true,
			inputPath: "/api/v2/buildinfo",
			baseURL: "http://192.0.2.1",
		},

	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Create a mock HTTP server using httptest.NewServer. It runs locally and
			// doesn't require internet access.
			//
			// http.HandlerFunc(tc.mockResponse) converts tc.mockResponse into an
			// http.Handler interface, which httptest expects. httptest automatically
			// provides http.ResponseWriter and *http.Request as inputs.
			server := httptest.NewServer(http.HandlerFunc(tc.mockResponse))
			defer server.Close()

			var client *coderclient.CoderClient
			url := server.URL
			if tc.baseURL != "" {
				url = tc.baseURL
			} 
			client = coderclient.NewCoderClient(url, "test-token")
			

			// Test request
			var bi coderBuildInfoResponse
			err := client.Get(ctx, tc.inputPath, &bi)

			gotErr := err != nil
			if gotErr != tc.expectErr {
				t.Errorf("Test %q failed: got error = %v, want error presence = %v", tc.name, err, tc.expectErr)
			}

			if !gotErr {
				if diff := cmp.Diff(tc.expected, bi); diff != "" {
					t.Errorf("Test %q failed: response mismatch (-want +got): \n%s", tc.name, diff)
				}
			}

		})
	}
}
