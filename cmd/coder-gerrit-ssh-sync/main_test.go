package main

import (
	"context"
	"strings"
	"net/http"
	"net/http/httptest"
	"testing"
)

type coderGetTestCase struct {
	name        string
	path        string
	wantErr     bool
	errContains string
}

var (
	CODER_GET_TESTCASES = []coderGetTestCase{
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
			errContains: "invalid character",
		},
	}
)

func verifyRequestHeaders(t *testing.T, r *http.Request) {
	if r.Header.Get("Accept") != "application/json" {
		t.Error("Missing Accept header")
	}
	if r.Header.Get("Coder-Session-Token") != "test-token" {
		t.Error("Missing or invalid auth token")
	}
}

func writeWithErrorCheck(t *testing.T, w http.ResponseWriter, data []byte) {
    n, err := w.Write(data)
    if err != nil {
        t.Errorf("failed to write %d bytes: %v", n, err)
    }
}

func handleMockServerResponses(t *testing.T, w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "/api/success":
        w.WriteHeader(http.StatusOK)
        writeWithErrorCheck(t, w, []byte(`{"key": "value"}`));
    case "/api/error":
        w.WriteHeader(http.StatusInternalServerError)
    case "/api/bad-json":
        w.WriteHeader(http.StatusOK)
        writeWithErrorCheck(t, w, []byte(`{invalid json`));
    }
}

func setupTestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verifyRequestHeaders(t, r)
		handleMockServerResponses(t, w, r)
	}))
}

func restoreCoderInstance() (originalCoderInstance *string, originalCoderToken string) {
	originalCoderInstance = coderInstance
	originalCoderToken = coderToken
	return originalCoderInstance, originalCoderToken
}

func updateTestValues(ts *httptest.Server) {
	testInstance := ts.URL
	coderInstance = &testInstance
	coderToken = "test-token"
}

func runCoderGetTest(t *testing.T, path string, wantErr bool, errContains string) {
	var target map[string]string
	err := coderGet(context.Background(), path, &target)

	if wantErr {
		if err == nil {
			t.Error("expected error, got nil")
		} else if errContains != "" && !strings.Contains(err.Error(), errContains) {
			t.Errorf("error = %v, want error containing %v", err, errContains)
		}
	} else {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if target["key"] != "value" {
			t.Errorf("got target = %v, want {key: value}", target)
		}
	}
}

func TestCoderGet(t *testing.T) {
	originalCoderInstance, originalCoderToken := restoreCoderInstance()
	defer func() {
		coderInstance = originalCoderInstance
		coderToken = originalCoderToken
	}()

	ts := setupTestServer(t)
	defer ts.Close()

	updateTestValues(ts)

	for _, tt := range CODER_GET_TESTCASES {
		t.Run(tt.name, func(t *testing.T) {
			runCoderGetTest(t, tt.path, tt.wantErr, tt.errContains)
		})
	}
}