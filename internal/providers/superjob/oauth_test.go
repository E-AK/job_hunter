package superjob

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"job-hunter-mcp/internal/domain"
)

type memoryTokenStore struct {
	token domain.SuperJobOAuthToken
	found bool
	saves int
}

func (s *memoryTokenStore) GetSuperJobOAuthToken(context.Context) (domain.SuperJobOAuthToken, bool, error) {
	return s.token, s.found, nil
}

func (s *memoryTokenStore) SaveSuperJobOAuthToken(_ context.Context, token domain.SuperJobOAuthToken) error {
	s.token = token
	s.found = true
	s.saves++
	return nil
}

func TestTokenManagerExchangesAuthorizationCodeAndPersistsTokens(t *testing.T) {
	store := &memoryTokenStore{}
	requests := 0
	manager := &TokenManager{
		ClientID:          "12345",
		ClientSecret:      "app-secret",
		AuthorizationCode: "auth-code",
		RedirectURI:       "https://example.test/callback",
		Store:             store,
		HTTP: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			if req.Method != http.MethodPost || req.URL.Path != "/2.0/oauth2/access_token/" {
				t.Errorf("request = %s %s, want POST access_token endpoint", req.Method, req.URL.Path)
			}
			if got := req.Header.Get("X-Api-App-Id"); got != "app-secret" {
				t.Errorf("X-Api-App-Id = %q, want app secret", got)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			form, err := url.ParseQuery(string(body))
			if err != nil {
				return nil, err
			}
			for key, want := range map[string]string{
				"code":          "auth-code",
				"client_id":     "12345",
				"client_secret": "app-secret",
				"redirect_uri":  "https://example.test/callback",
			} {
				if got := form.Get(key); got != want {
					t.Errorf("%s = %q, want configured value", key, got)
				}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"access-1","refresh_token":"refresh-1","expires_in":3600}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	token, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token != "access-1" || !store.found || store.saves != 1 || requests != 1 {
		t.Fatalf("token=%q stored=%t saves=%d requests=%d", token, store.found, store.saves, requests)
	}
	if store.token.RefreshToken != "refresh-1" || !store.token.ExpiresAt.After(time.Now()) {
		t.Fatalf("stored token metadata = %#v", store.token)
	}

	if token, err = manager.AccessToken(context.Background()); err != nil || token != "access-1" || requests != 1 {
		t.Fatalf("cached AccessToken = %q, err=%v, requests=%d", token, err, requests)
	}
}

func TestTokenManagerRefreshesExpiredAccessToken(t *testing.T) {
	store := &memoryTokenStore{
		token: domain.SuperJobOAuthToken{
			AccessToken:  "expired-access",
			RefreshToken: "refresh-old",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
		found: true,
	}
	manager := &TokenManager{
		ClientID:     "12345",
		ClientSecret: "app-secret",
		Store:        store,
		HTTP: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet || req.URL.Path != "/2.0/oauth2/refresh_token/" {
				t.Errorf("request = %s %s, want GET refresh_token endpoint", req.Method, req.URL.Path)
			}
			query := req.URL.Query()
			for key, want := range map[string]string{
				"refresh_token": "refresh-old",
				"client_id":     "12345",
				"client_secret": "app-secret",
			} {
				if got := query.Get(key); got != want {
					t.Errorf("%s = %q, want configured value", key, got)
				}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"access-new","refresh_token":"refresh-new","expires_in":3600}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	token, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token != "access-new" || store.token.RefreshToken != "refresh-new" || store.saves != 1 {
		t.Fatalf("token=%q stored=%#v saves=%d", token, store.token, store.saves)
	}
}
