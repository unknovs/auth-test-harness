package main

import (
	"log"
	"net/http"
	"time"

	"github.com/unknovs/auth-test-harness/env"
	"github.com/unknovs/auth-test-harness/handlers"
	"github.com/unknovs/auth-test-harness/routes/responses"
	"github.com/unknovs/auth-test-harness/utils"
)

func main() {
	config := env.Load()

	store := utils.NewInMemoryStore()

	oauthHandler := handlers.NewOAuthHandler(config, store)

	mux := http.NewServeMux()

	mux.HandleFunc(config.AuthorizationEndpoint, oauthHandler.AuthorizeHandler)

	mux.HandleFunc(config.TokenEndpoint, oauthHandler.TokenHandler)

	mux.HandleFunc(config.UserInfoEndpoint, oauthHandler.UserInfoHandler)

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
			config.ScopesSupported,
			config.ACRValuesSupported,
		)
		// Response already committed; a short write has nowhere to go from here.
		_, _ = w.Write([]byte(response))
	})

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
	log.Printf("  Health: %s://%s/health", config.Protocol, config.Host)

	// No read/write timeouts: this is a mock identity provider that exists for the length
	// of a test run, on a loopback address, driven by the test harness itself. There is no
	// untrusted client for a timeout to protect against.
	//nolint:gosec // test double, not an exposed server
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
