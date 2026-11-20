package superjob

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"job-hunter-mcp/internal/domain"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSearchSendsConfiguredCredentials(t *testing.T) {
	client := &Client{
		AppID: "test-app-id",
		Token: "test-access-token",
		HTTP: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("X-Api-App-Id"); got != "test-app-id" {
				t.Errorf("X-Api-App-Id = %q, want configured app id", got)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer test-access-token" {
				t.Errorf("Authorization = %q, want configured bearer token", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"objects":[]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	if _, err := client.Search(domain.SearchParams{Text: "developer", AreaID: "113", Limit: 10}); err != nil {
		t.Fatalf("Search: %v", err)
	}
}

func TestSearchReportsForbiddenStatus(t *testing.T) {
	var logs bytes.Buffer
	previousLogOutput := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousLogOutput) })

	responseBody := `{"error":{"code":"forbidden","message":"App ID rejected"},"padding":"` +
		strings.Repeat("x", maxLoggedErrorBodyBytes) + `"}`
	client := &Client{
		AppID: "do-not-log-app-id",
		Token: "do-not-log-access-token",
		HTTP: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader(responseBody)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, err := client.Search(domain.SearchParams{Text: "developer", AreaID: "113", Limit: 10})
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("Search error = %v, want HTTP 403", err)
	}
	requestIndex := strings.Index(logs.String(), "superjob API request:")
	responseIndex := strings.Index(logs.String(), "superjob API response:")
	if requestIndex < 0 || responseIndex <= requestIndex ||
		!strings.Contains(logs.String(), "method=GET") ||
		!strings.Contains(logs.String(), "keyword=developer") ||
		!strings.Contains(logs.String(), "X-Api-App-Id=[REDACTED]") ||
		!strings.Contains(logs.String(), "Authorization=[REDACTED]") ||
		!strings.Contains(logs.String(), "body=<empty>") ||
		!strings.Contains(logs.String(), "status=403") ||
		!strings.Contains(logs.String(), `\"message\":\"App ID rejected\"`) ||
		!strings.Contains(logs.String(), "response_body_truncated=true") {
		t.Fatalf("403 diagnostic log missing request details, response, or truncation marker: %s", logs.String())
	}
	if len(logs.String()) > maxLoggedErrorBodyBytes+512 {
		t.Fatalf("403 diagnostic log too large: %d bytes", len(logs.String()))
	}
	if strings.Contains(logs.String(), client.AppID) || strings.Contains(logs.String(), client.Token) {
		t.Fatalf("403 diagnostic log contains credentials: %s", logs.String())
	}
}
