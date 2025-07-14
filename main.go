package main

import (
	"log"
	"net/http"
	"time"

	"github.com/unknovs/auth-test-harness.git/env"
	"github.com/unknovs/auth-test-harness.git/handlers"
	"github.com/unknovs/auth-test-harness.git/routes/responses"
	"github.com/unknovs/auth-test-harness.git/utils"
)

func main() {
	config := env.Load()

	store := utils.NewInMemoryStore()

	oauthHandler := handlers.NewOAuthHandler(config, store)

	mux := http.NewServeMux()

	mux.HandleFunc(config.AuthorizationEndpoint, oauthHandler.AuthorizeHandler)

	mux.HandleFunc(config.TokenEndpoint, oauthHandler.TokenHandler)

	mux.HandleFunc(config.UserInfoEndpoint, oauthHandler.UserInfoHandler)

	mux.HandleFunc("/.well-known/openid_configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := responses.OpenIDConfigurationResponse(
			config.Host,
			config.AuthorizationEndpoint,
			config.TokenEndpoint,
			config.UserInfoEndpoint,
			config.ScopesSupported,
			config.ACRValuesSupported,
		)
		w.Write([]byte(response))
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
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
			config.Host,
			config.AuthorizationEndpoint,
			config.TokenEndpoint,
			config.UserInfoEndpoint,
			config.ScopesSupported,
			config.ACRValuesSupported,
		)
		w.Write([]byte(response))
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
	addr := ":" + config.Port
	log.Printf("Starting OAuth OIDC Mock Service on %s", addr)
	log.Printf("Host: %s", config.Host)
	log.Printf("Endpoints:")
	log.Printf("  Authorization: http://%s%s", config.Host, config.AuthorizationEndpoint)
	log.Printf("  Token: http://%s%s", config.Host, config.TokenEndpoint)
	log.Printf("  UserInfo: http://%s%s", config.Host, config.UserInfoEndpoint)
	log.Printf("  Health: http://%s/health", config.Host)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
