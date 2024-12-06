package main

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
)


type TestCase struct {
    name        string
    inputPath   string
    mockResponse func(w http.ResponseWriter, r *http.Request)
    expected    coderBuildInfoResponse
    expectedErr string
}

type TestCasesNewRequestWithContext struct {
    name string
    inputPath string
    coderInstance string
    expected    coderBuildInfoResponse
    expectedErr string
}

type TestCasesDefaultClient struct {

    name string
    mockRoundTrip func(req *http.Request) (*http.Response, error)
    expectedErr string
    inputPath string

}

type MockRoundTripper struct {
    RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    return m.RoundTripFunc(req)
}

// Test CoderGe. Split with TestCoderGetNewRequestWithContext cuz this will assumes valid coderInstance+path but server side/ Decode problem.
func TestCoderGet(t *testing.T) {
    ctx := context.Background()

    testCases := []TestCase{
        {
            // Scenario 1: if coderGet doesn't fail, verify return struct vs expected struct. NOT verify the inside data!
            name: "Successful response",
            mockResponse: func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK) // set HTTP 200 means success.
                fmt.Fprintln(w, `{"Version": "1.0.0"}`) // mimic the actual return file from the server.
            },
            expected: coderBuildInfoResponse{Version: "1.0.0"},
            expectedErr: "",
            inputPath: "/api/v2/buildinfo",
        },

        {
            // Scenario 2: if server was reached and understood the request, but can't find resource to return
            name: "Not found 404 error",
            mockResponse: func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusNotFound) // 404 means requested resources not found on server
            },
            expected: coderBuildInfoResponse{},
            expectedErr: "Coder HTTP status: 404 Not Found",
            inputPath: "/api/v2/buildinfo",
        },

        {
            // Scenario 3: if server was reach and understood the request, but fail to return the Version.
            // Decode will return "invalid character '}' looking for beginning of value" automatically
            name: "Failed to return Version in JSON",
            mockResponse: func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
                fmt.Fprintln(w, `{"Version":}`) // Version empty will trigger invalid character'}'
            },
            expected: coderBuildInfoResponse{},
            expectedErr: "invalid character '}' looking for beginning of value",
            inputPath: "/api/v2/buildinfo",
        },

    }


    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {

            // Step 1: Create a mock HTTP server
            // httptest.NewServer: set up HTTP serverfor testing specific. Runs locally doesn't need internet access.
            mockServer := httptest.NewServer(http.HandlerFunc(tc.mockResponse))
            defer mockServer.Close() // cose the server after test is done.

            // Step 2: Let coderInstance point to mock server
            *coderInstance = mockServer.URL

            // Step 3: Prepare the target structure to decode the JSON response
            var target coderBuildInfoResponse

            // Step 4: Call the function under test
            // coderGet: store decoded version info in target, and return nil if decode success or error if fail.
            err := coderGet(ctx, tc.inputPath, &target)


            // Step 5: Assert the result
            if tc.expectedErr != "" {
                if err == nil || err.Error() != tc.expectedErr {
                    t.Fatalf("For %s: expected error: %v, got: %v", tc.name, tc.expectedErr, err)
                }
            } else {
                if err != nil {
                    t.Fatalf("For %s: unexpected error: %v", tc.name, err)
                }

                if target != tc.expected {
                    t.Fatalf("For %s: expected: %+v, got: %+v", tc.name, tc.expected, target)
                }
            }
        })
    }


}

// Test NewRequestWithContext in CoderGet. Split with TestCoderGet cuz this will provide invalid coderInstance+path.
func TestCoderGetNewRequestWithContext(t *testing.T){

    ctx := context.Background()

    testCasesNewRequestWithContext := []TestCasesNewRequestWithContext{

        {
            // Scenario 1: http:// is missing
            name: "http:// is missing",
            inputPath: "/api/v2/buildinfo",
            coderInstance: "127.0.0.1:port", // Should be http://127.0.0.1:port
            expected: coderBuildInfoResponse{},
            expectedErr: "parse \"127.0.0.1:port/api/v2/buildinfo\": first path segment in URL cannot contain colon",
        },
    }

    for _, tc := range testCasesNewRequestWithContext {
        t.Run(tc.name, func(t *testing.T){


            originalCoderInstance := *coderInstance
            defer func() {
                *coderInstance = originalCoderInstance
            }()

            // Assign invalid path to coderInstance
            *coderInstance = tc.coderInstance

            var target coderBuildInfoResponse

            err := coderGet(ctx, tc.inputPath, &target)


            if err != nil {
                t.Logf("Actual error: %v", err)
            }

            if err == nil || err.Error() != tc.expectedErr {
                t.Fatalf("Expected error: %s, got: %v", tc.expectedErr, err)
            }

            if(target != tc.expected){
                t.Fatalf("Expected target return {}, got: %+v", target)
            }

        })
    }
}



// Test http.DefaultClient.Do(req) in coderGet
func TestHttpClientError(t *testing.T) {

    ctx := context.Background()

    testCasesDefaultClient := []TestCasesDefaultClient{
        {
            name: "Simulate transport error",
            mockRoundTrip: func(req *http.Request) (*http.Response, error) {
                return nil, fmt.Errorf("simulated transport error")
            },
            expectedErr : fmt.Sprintf("Get \"%s%s\": simulated transport error", *coderInstance, "/api/v2/buildinfo"),
            inputPath: "/api/v2/buildinfo",
        },
    }

    for _, tc := range testCasesDefaultClient {

        originalTransport := http.DefaultClient.Transport
        defer func() {
            http.DefaultClient.Transport = originalTransport
        }()

        // Create a mock RoundTripper that always returns an error
        // Basically can control the behavior of HTTP requests made by client without relying on real network communication
        mockTransport := &MockRoundTripper{
            RoundTripFunc: tc.mockRoundTrip,
        }
        http.DefaultClient.Transport = mockTransport


        var target coderBuildInfoResponse
        err := coderGet(ctx, tc.inputPath, &target)

        if err == nil || err.Error() != tc.expectedErr {
            t.Fatalf("expected error: %s, got: %v", tc.expectedErr, err)
        }


    }


}

