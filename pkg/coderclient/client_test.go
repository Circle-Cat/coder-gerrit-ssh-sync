package coderclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGet(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		mockResponse func(w http.ResponseWriter, r *http.Request)
		expected     CoderBuildInfoResponse
		expectErr    bool
		inputPath    string
		baseURL      string
	}{
		{
			// Scenario 1: Return version successfully
			name: "Success_response",
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)            // set HTTP 200 means success.
				fmt.Fprintln(w, `{"Version": "1.0.0"}`) // writes data to w, mimic return file from the server.
			},
			expected:  CoderBuildInfoResponse{Version: "1.0.0"},
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
			expected:  CoderBuildInfoResponse{},
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
			expected:  CoderBuildInfoResponse{},
			expectErr: true,
			inputPath: "/api/v2/buildinfo",
		},
		{
			// Scenario 4: Invalid URL error
			name: "Invalid_url",
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
			},
			expected:  CoderBuildInfoResponse{},
			expectErr: true,
			inputPath: "/api/v2/buildinfo",
			baseURL:   "http://:%22",
		},
		{
			// Scenario 5: Unreachable address error
			name: "Unreachable_address",
			mockResponse: func(w http.ResponseWriter, r *http.Request) {
			},
			expected:  CoderBuildInfoResponse{},
			expectErr: true,
			inputPath: "/api/v2/buildinfo",
			baseURL:   "http://coder.invalid",
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

			url := server.URL
			if tc.baseURL != "" {
				url = tc.baseURL
			}
			client := NewCoderClient(url, "test-token")

			// Test request
			var bi CoderBuildInfoResponse
			err := client.Get(ctx, tc.inputPath, &bi)

			gotErr := err != nil
			if gotErr != tc.expectErr {
				t.Errorf("Test %q failed: got error = %v, want error presence = %v", tc.name, err, tc.expectErr)
				return
			}

			diff := cmp.Diff(tc.expected, bi)
			if diff != "" {
				t.Errorf("Test %q failed: response mismatch (-want +got): \n%s", tc.name, diff)
				return
			}

		})
	}
}
