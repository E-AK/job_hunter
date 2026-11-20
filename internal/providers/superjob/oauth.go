package superjob

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"job-hunter-mcp/internal/domain"
)

const (
	oauthEndpoint     = "https://api.superjob.ru/2.0/oauth2/"
	tokenRefreshSkew  = time.Minute
	maxOAuthErrorBody = 4096
)

type TokenStore interface {
	GetSuperJobOAuthToken(context.Context) (domain.SuperJobOAuthToken, bool, error)
	SaveSuperJobOAuthToken(context.Context, domain.SuperJobOAuthToken) error
}

type TokenManager struct {
	ClientID          string
	ClientSecret      string
	AuthorizationCode string
	RedirectURI       string
	StaticAccessToken string
	Store             TokenStore
	HTTP              *http.Client

	mu sync.Mutex
}

func (m *TokenManager) AccessToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var stored domain.SuperJobOAuthToken
	found := false
	if m.Store != nil {
		var err error
		stored, found, err = m.Store.GetSuperJobOAuthToken(ctx)
		if err != nil {
			return "", fmt.Errorf("load SuperJob OAuth token: %w", err)
		}
		if found && stored.ExpiresAt.After(time.Now().Add(tokenRefreshSkew)) {
			return stored.AccessToken, nil
		}
	}

	switch {
	case found && stored.RefreshToken != "":
		token, err := m.refresh(ctx, stored.RefreshToken)
		if err != nil {
			return "", err
		}
		if err = m.Store.SaveSuperJobOAuthToken(ctx, token); err != nil {
			return "", fmt.Errorf("save refreshed SuperJob OAuth token: %w", err)
		}
		return token.AccessToken, nil
	case found:
		return "", fmt.Errorf("stored SuperJob access token expired without a refresh token")
	case m.AuthorizationCode != "":
		if m.Store == nil {
			return "", fmt.Errorf("SuperJob OAuth token storage is required")
		}
		token, err := m.exchange(ctx, m.AuthorizationCode)
		if err != nil {
			return "", err
		}
		if err = m.Store.SaveSuperJobOAuthToken(ctx, token); err != nil {
			return "", fmt.Errorf("save SuperJob OAuth token: %w", err)
		}
		return token.AccessToken, nil
	case m.StaticAccessToken != "":
		return m.StaticAccessToken, nil
	default:
		return "", nil
	}
}

func (m *TokenManager) exchange(ctx context.Context, code string) (domain.SuperJobOAuthToken, error) {
	if err := m.validateOAuthConfig(); err != nil {
		return domain.SuperJobOAuthToken{}, err
	}
	values := url.Values{
		"code":          {code},
		"client_id":     {m.ClientID},
		"client_secret": {m.ClientSecret},
		"redirect_uri":  {m.RedirectURI},
	}
	return m.requestToken(ctx, http.MethodPost, "access_token/", values)
}

func (m *TokenManager) refresh(ctx context.Context, refreshToken string) (domain.SuperJobOAuthToken, error) {
	if err := m.validateOAuthConfig(); err != nil {
		return domain.SuperJobOAuthToken{}, err
	}
	values := url.Values{
		"refresh_token": {refreshToken},
		"client_id":     {m.ClientID},
		"client_secret": {m.ClientSecret},
	}
	return m.requestToken(ctx, http.MethodGet, "refresh_token/", values)
}

func (m *TokenManager) validateOAuthConfig() error {
	if m.ClientID == "" || m.ClientSecret == "" {
		return fmt.Errorf("SUPERJOB_CLIENT_ID and SUPERJOB_APP_SECRET are required for OAuth")
	}
	return nil
}

func (m *TokenManager) requestToken(ctx context.Context, method, endpoint string, values url.Values) (domain.SuperJobOAuthToken, error) {
	requestURL := oauthEndpoint + endpoint
	var body io.Reader
	if method == http.MethodGet {
		requestURL += "?" + values.Encode()
	} else {
		body = strings.NewReader(values.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return domain.SuperJobOAuthToken{}, fmt.Errorf("create SuperJob OAuth request: %w", err)
	}
	req.Header.Set("X-Api-App-Id", m.ClientSecret)
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	client := m.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return domain.SuperJobOAuthToken{}, fmt.Errorf("request SuperJob OAuth token: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(res.Body, maxOAuthErrorBody))
		if readErr != nil {
			return domain.SuperJobOAuthToken{}, fmt.Errorf("SuperJob OAuth %s: HTTP %d (read error: %w)", endpoint, res.StatusCode, readErr)
		}
		return domain.SuperJobOAuthToken{}, fmt.Errorf("SuperJob OAuth %s: HTTP %d: %s", endpoint, res.StatusCode, strings.TrimSpace(string(body)))
	}

	var response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err = json.NewDecoder(res.Body).Decode(&response); err != nil {
		return domain.SuperJobOAuthToken{}, fmt.Errorf("decode SuperJob OAuth response: %w", err)
	}
	if response.AccessToken == "" || response.RefreshToken == "" || response.ExpiresIn <= 0 {
		return domain.SuperJobOAuthToken{}, fmt.Errorf("SuperJob OAuth response is missing a token or valid expires_in")
	}
	return domain.SuperJobOAuthToken{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
	}, nil
}
