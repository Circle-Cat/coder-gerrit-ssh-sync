package main

import (
	"context"
	"strings"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoderGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/success":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"key": "value"}`))
		case "/api/error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/api/bad-json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"key":`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	testCoderClientConfig := CoderClientConfig{
		Instance: &server.URL,
		Token: "test-token",
	}

	testCases := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name:        "successful request",
			path:        "/api/success",
			wantErr:     false,
			errContains: "",
		},
		{
			name:        "server error",
			path:        "/api/error",
			wantErr:     true,
			errContains: "Coder HTTP status: 500",
		},
		{
			name:        "invalid json response",
			path:        "/api/bad-json",
			wantErr:     true,
			errContains: "EOF",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var target map[string]string
			
			err := coderGet(context.Background(), tt.path, &target, testCoderClientConfig)
		
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if target["key"] != "value" {
					t.Errorf("Got target = %v, want {key: value}", target)
				}
			}
		})
	}
}
