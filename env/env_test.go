package env

import "testing"

// Unset, USED_IDENTITIES keeps today's behaviour — the configured profiles — and
// no logout endpoint is served.
func TestLoadDefaults(t *testing.T) {
	t.Setenv("USED_IDENTITIES", "")
	t.Setenv("IDENTITIES_FILE", "")
	t.Setenv("LOGOUT_ENDPOINT", "")

	c := Load()
	if c.UsedIdentities != IdentitiesFromConfig || c.IdentitiesFile != "" || c.LogoutEndpoint != "" {
		t.Fatalf("UsedIdentities %q IdentitiesFile %q LogoutEndpoint %q", c.UsedIdentities, c.IdentitiesFile, c.LogoutEndpoint)
	}
}

func TestLoadReadsTheNewSettings(t *testing.T) {
	t.Setenv("USED_IDENTITIES", "list")
	t.Setenv("IDENTITIES_FILE", "/identities/identities.json")
	t.Setenv("LOGOUT_ENDPOINT", "/trustedx-authserver/lvrtc-eipsign-idp/logout")

	c := Load()
	if c.UsedIdentities != IdentitiesFromList || c.IdentitiesFile != "/identities/identities.json" ||
		c.LogoutEndpoint != "/trustedx-authserver/lvrtc-eipsign-idp/logout" {
		t.Fatalf("UsedIdentities %q IdentitiesFile %q LogoutEndpoint %q", c.UsedIdentities, c.IdentitiesFile, c.LogoutEndpoint)
	}
}
