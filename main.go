package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/unknovs/auth-test-harness/env"
	"github.com/unknovs/auth-test-harness/handlers"
	"github.com/unknovs/auth-test-harness/identities"
	"github.com/unknovs/auth-test-harness/routes/responses"
	"github.com/unknovs/auth-test-harness/utils"
)

func main() {
	config := env.Load()

	store := utils.NewInMemoryStore()

	// The id_token signing key: generated per start, published at the jwks_uri.
	key, err := utils.NewSigningKey()
	if err != nil {
		log.Fatal("Could not generate the signing key:", err)
	}

	oauthHandler := handlers.NewOAuthHandler(config, store, key)
	if err := useIdentities(config, oauthHandler, log.Printf); err != nil {
		log.Fatal(err)
	}

	mux := newMux(config, oauthHandler)

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			store.CleanupExpired()
			log.Println("Cleaned up expired tokens and codes")
		}
	}()

	// Start server
	addr := config.BindAddress + ":" + config.Port
	log.Printf("Starting OAuth OIDC Mock Service on %s", addr)
	log.Printf("Host: %s", config.Host)
	log.Printf("Protocol: %s", config.Protocol)
	log.Printf("Endpoints:")
	log.Printf("  Authorization: %s://%s%s", config.Protocol, config.Host, config.AuthorizationEndpoint)
	log.Printf("  Token: %s://%s%s", config.Protocol, config.Host, config.TokenEndpoint)
	log.Printf("  UserInfo: %s://%s%s", config.Protocol, config.Host, config.UserInfoEndpoint)
	if config.LogoutEndpoint != "" {
		log.Printf("  Logout: %s://%s%s", config.Protocol, config.Host, config.LogoutEndpoint)
	}
	log.Printf("  Health: %s://%s/health", config.Protocol, config.Host)
	if oauthHandler.UsesIdentityList() && config.IdentitiesFile == "" {
		log.Printf("Identities: the built-in list, for the flows that take a personal code")
	} else if oauthHandler.UsesIdentityList() {
		log.Printf("Identities: the list %s, for the flows that take a personal code", config.IdentitiesFile)
	} else {
		log.Printf("Identities: the configured profiles")
	}

	// No read/write timeouts: this is a mock identity provider that exists for the length
	// of a test run, on a loopback address, driven by the test harness itself. There is no
	// untrusted client for a timeout to protect against.
	//nolint:gosec // test double, not an exposed server
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

// builtinIdentities is the identity list the service carries: twenty made-up
// people, used with USED_IDENTITIES=list when no IDENTITIES_FILE is given, so a
// test environment needs nothing to manage.
//
//go:embed examples/identities/identities.json
var builtinIdentities []byte

// useIdentities applies USED_IDENTITIES: the configured profiles, or the
// identity list for the flows that take a personal code — the built-in one, or
// the file IDENTITIES_FILE names.
func useIdentities(config *env.Config, h *handlers.OAuthHandler, logf func(string, ...any)) error {
	switch config.UsedIdentities {
	case env.IdentitiesFromConfig:
		return nil
	case env.IdentitiesFromList:
		if config.IdentitiesFile == "" {
			l, err := identities.Builtin(builtinIdentities)
			if err != nil {
				return fmt.Errorf("the built-in identity list: %w", err)
			}
			logf("identity list: the built-in one, %d people", l.Len())
			h.SetIdentityList(l)
			return nil
		}
		h.SetIdentityList(identities.Open(config.IdentitiesFile, logf))
		return nil
	default:
		return fmt.Errorf("USED_IDENTITIES must be %q or %q, not %q", env.IdentitiesFromConfig, env.IdentitiesFromList, config.UsedIdentities)
	}
}

// newMux mounts every route the service answers: the three configured OAuth
// endpoints, the logout endpoint and the code step when they are in use, the
// key set, the discovery document, the health check and the service
// information document.
func newMux(config *env.Config, oauthHandler *handlers.OAuthHandler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc(config.AuthorizationEndpoint, oauthHandler.AuthorizeHandler)

	mux.HandleFunc(config.TokenEndpoint, oauthHandler.TokenHandler)

	mux.HandleFunc(config.UserInfoEndpoint, oauthHandler.UserInfoHandler)

	if config.LogoutEndpoint != "" {
		mux.HandleFunc(config.LogoutEndpoint, oauthHandler.LogoutHandler)
	}

	if oauthHandler.UsesIdentityList() {
		mux.HandleFunc(handlers.CodeStepPath, oauthHandler.CodeStepHandler)
	}

	// The key set the id_tokens verify against — the address the discovery
	// document has always advertised.
	mux.HandleFunc("/.well-known/jwks.json", oauthHandler.JWKSHandler)

	discovery := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := responses.OpenIDConfigurationResponse(
			config.Protocol,
			config.Host,
			config.AuthorizationEndpoint,
			config.TokenEndpoint,
			config.UserInfoEndpoint,
			config.ScopesSupported,
			config.ACRValuesSupported,
		)
		// Response already committed; a short write has nowhere to go from here.
		_, _ = w.Write([]byte(response))
	}

	// OpenID Connect Discovery defines this document at
	// /.well-known/openid-configuration (hyphen). That is the path every
	// spec-compliant client fetches, so it is the one to serve.
	mux.HandleFunc("/.well-known/openid-configuration", discovery)
	// The underscore spelling this service originally shipped, kept so anything
	// already pointing at it keeps working. Prefer the hyphen path above.
	mux.HandleFunc("/.well-known/openid_configuration", discovery)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Response already committed; a short write has nowhere to go from here.
		_, _ = w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Root endpoint - Service Information
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := responses.ServiceInfoResponse(
			config.Protocol,
			config.Host,
			config.AuthorizationEndpoint,
			config.TokenEndpoint,
			config.UserInfoEndpoint,
			config.LogoutEndpoint,
			config.ScopesSupported,
			config.ACRValuesSupported,
		)
		// Response already committed; a short write has nowhere to go from here.
		_, _ = w.Write([]byte(response))
	})

	return mux
}
