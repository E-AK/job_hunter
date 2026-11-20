package config

import "testing"

func TestLoadReadsSuperJobCredentials(t *testing.T) {
	t.Setenv("SUPERJOB_APP_ID", "test-app-id")
	t.Setenv("SUPERJOB_ACCESS_TOKEN", "test-access-token")
	t.Setenv("SUPERJOB_IZHEVSK_TOWN_ID", "test-town-id")

	got := Load()
	if got.SuperJobAppID != "test-app-id" {
		t.Errorf("SuperJobAppID = %q, want configured value", got.SuperJobAppID)
	}
	if got.SuperJobToken != "test-access-token" {
		t.Errorf("SuperJobToken = %q, want configured value", got.SuperJobToken)
	}
	if got.SuperJobIzhevskTownID != "test-town-id" {
		t.Errorf("SuperJobIzhevskTownID = %q, want configured value", got.SuperJobIzhevskTownID)
	}
}
