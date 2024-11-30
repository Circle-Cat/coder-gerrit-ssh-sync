package main

import (
	"context"
	"strings"
	"net/http"
	"net/http/httptest"
	"testing"
	"fmt"
	"encoding/json"

)

type CoderClient struct {
	Instance *string
	Token    string
}

func NewClient(instanceURL string, token string) *CoderClient {
	return &CoderClient{
		Instance: &instanceURL,
		Token:    token,
	}
}

func (c *CoderClient) Get(ctx context.Context, path string, target *map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *c.Instance+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

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

	client := NewClient(server.URL, "test-token")

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
			errContains: "HTTP status: 500",
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
			err := client.Get(context.Background(), tt.path, &target)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if target["key"] != "value" {
					t.Errorf("got target = %v, want {key: value}", target)
				}
			}
		})
	}
}
