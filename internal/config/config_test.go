package config

import "testing"

func TestLoadReadsSuperJobCredentials(t *testing.T) {
	t.Setenv("SUPERJOB_APP_ID", "test-app-id")
	t.Setenv("SUPERJOB_APP_SECRET", "test-app-secret")
	t.Setenv("SUPERJOB_CLIENT_ID", "123")
	t.Setenv("SUPERJOB_AUTHORIZATION_CODE", "test-auth-code")
	t.Setenv("SUPERJOB_REDIRECT_URI", "https://example.test/callback")
	t.Setenv("SUPERJOB_ACCESS_TOKEN", "test-access-token")
	t.Setenv("SUPERJOB_IZHEVSK_TOWN_ID", "test-town-id")

	got := Load()
	if got.SuperJobAppSecret != "test-app-secret" {
		t.Errorf("SuperJobAppSecret = %q, want configured value", got.SuperJobAppSecret)
	}
	if got.SuperJobClientID != "123" || got.SuperJobAuthorizationCode != "test-auth-code" ||
		got.SuperJobRedirectURI != "https://example.test/callback" {
		t.Errorf("OAuth configuration = %#v, want configured values", got)
	}
	if got.SuperJobToken != "test-access-token" {
		t.Errorf("SuperJobToken = %q, want configured value", got.SuperJobToken)
	}
	if got.SuperJobIzhevskTownID != "test-town-id" {
		t.Errorf("SuperJobIzhevskTownID = %q, want configured value", got.SuperJobIzhevskTownID)
	}
}

func TestLoadSupportsLegacySuperJobAppIDSecret(t *testing.T) {
	t.Setenv("SUPERJOB_APP_SECRET", "")
	t.Setenv("SUPERJOB_APP_ID", "legacy-secret")

	if got := Load().SuperJobAppSecret; got != "legacy-secret" {
		t.Errorf("SuperJobAppSecret = %q, want legacy SUPERJOB_APP_ID value", got)
	}
}
