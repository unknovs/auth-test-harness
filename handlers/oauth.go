package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/unknovs/auth-test-harness.git/env"
	"github.com/unknovs/auth-test-harness.git/routes/requests"
	"github.com/unknovs/auth-test-harness.git/routes/responses"
	"github.com/unknovs/auth-test-harness.git/utils"
)

// OAuthHandler handles OAuth operations
type OAuthHandler struct {
	config *env.Config
	store  *utils.InMemoryStore
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(config *env.Config, store *utils.InMemoryStore) *OAuthHandler {
	return &OAuthHandler{
		config: config,
		store:  store,
	}
}

// AuthorizeHandler handles the authorization endpoint
func (h *OAuthHandler) AuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	req := &requests.AuthorizeRequest{
		ResponseType: r.URL.Query().Get("response_type"),
		ClientID:     r.URL.Query().Get("client_id"),
		State:        r.URL.Query().Get("state"),
		RedirectURI:  r.URL.Query().Get("redirect_uri"),
		Scope:        r.URL.Query().Get("scope"),
		Prompt:       r.URL.Query().Get("prompt"),
		ACRValues:    r.URL.Query().Get("acr_values"),
		UILocales:    r.URL.Query().Get("ui_locales"),
	}

	// Validate required parameters
	if req.ResponseType != "code" {
		h.sendError(w, "invalid_request", "response_type must be 'code'")
		return
	}

	if req.ClientID == "" {
		h.sendError(w, "invalid_request", "client_id is required")
		return
	}

	if req.RedirectURI == "" {
		h.sendError(w, "invalid_request", "redirect_uri is required")
		return
	}

	// Validate scope
	isValidScope := false
	for _, validScope := range h.config.ScopesSupported {
		if req.Scope == validScope {
			isValidScope = true
			break
		}
	}
	if !isValidScope {
		h.sendError(w, "invalid_scope", "Invalid scope")
		return
	}

	// Validate acr_values
	isValidACR := false
	for _, valid := range h.config.ACRValuesSupported {
		if req.ACRValues == valid {
			isValidACR = true
			break
		}
	}

	if !isValidACR {
		h.sendError(w, "invalid_request", "Invalid acr_values")
		return
	}

	// Generate authorization code
	code := utils.GenerateAuthCode()

	// Store the authorization code
	h.store.StoreAuthCode(code, req.ClientID, req.RedirectURI, req.Scope, req.ACRValues)

	log.Printf("Generated auth code: %s for client: %s", code, req.ClientID)

	// Build redirect URL
	redirectURL, err := url.Parse(req.RedirectURI)
	if err != nil {
		h.sendError(w, "invalid_request", "Invalid redirect_uri")
		return
	}

	query := redirectURL.Query()
	query.Set("code", code)
	if req.State != "" {
		query.Set("state", req.State)
	}
	redirectURL.RawQuery = query.Encode()

	// Redirect to the callback URL
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// TokenHandler handles the token endpoint
func (h *OAuthHandler) TokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		h.sendError(w, "invalid_client", "Basic authentication required")
		return
	}

	// Validate basic auth
	encodedCreds := strings.TrimPrefix(auth, "Basic ")
	if encodedCreds != h.config.BasicAuthValue {
		h.sendError(w, "invalid_client", "Invalid client credentials")
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		h.sendError(w, "invalid_request", "Invalid form data")
		return
	}

	req := &requests.TokenRequest{
		GrantType:   r.Form.Get("grant_type"),
		RedirectURI: r.Form.Get("redirect_uri"),
		Code:        r.Form.Get("code"),
	}

	// Validate grant type
	if req.GrantType != "authorization_code" {
		h.sendError(w, "unsupported_grant_type", "Only authorization_code grant type is supported")
		return
	}

	// Validate authorization code
	authCodeData, valid := h.store.GetAuthCode(req.Code)
	if !valid {
		log.Printf("Invalid or expired auth code: %s", req.Code)
		h.sendError(w, "invalid_grant", "Invalid or expired authorization code")
		return
	}

	log.Printf("Valid auth code: %s for client: %s", req.Code, authCodeData.ClientID)

	// Validate redirect URI
	if req.RedirectURI != authCodeData.RedirectURI {
		h.sendError(w, "invalid_grant", "Redirect URI mismatch")
		return
	}

	// Generate access token
	accessToken := utils.GenerateAccessToken()

	// Store access token
	h.store.StoreAccessToken(accessToken, authCodeData.ACRValues)

	// Prepare response
	response := &responses.TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   h.config.TokenExpirationMin * 60, // Convert to seconds
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	json.NewEncoder(w).Encode(response)
}

// UserInfoHandler handles the user info endpoint
func (h *OAuthHandler) UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract Bearer token
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		h.sendError(w, "invalid_token", "Bearer token required")
		return
	}

	token := strings.TrimPrefix(auth, "Bearer ")

	// Validate token
	tokenData, valid := h.store.GetAccessToken(token)
	if !valid {
		h.sendError(w, "invalid_token", "Invalid or expired access token")
		return
	}

	// Generate user info based on ACR values
	response := h.generateUserInfo(tokenData.ACRValues)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// generateUserInfo generates user information based on ACR values
func (h *OAuthHandler) generateUserInfo(acrValues string) *responses.UserInfoResponse {
	response := &responses.UserInfoResponse{
		Sub:          utils.GenerateSubject(),
		Domain:       "citizen",                                              // hardcoded as per requirements
		ACR:          "urn:safelayer:tws:policies:authentication:level:high", // hardcoded
		SerialNumber: h.config.SerialNumber,
		EIPS:         "",
	}

	switch acrValues {
	case "urn:eparaksts:authentication:flow:mobileid":
		response.AMR = []string{"urn:eparaksts:tws:policies:authentication:adaptive:methods:mobileid"}
		response.GivenName = h.config.MobileGivenName
		response.FamilyName = h.config.MobileFamilyName
		response.Name = h.config.MobileGivenName + " " + h.config.MobileFamilyName
	case "urn:eparaksts:authentication:flow:sc_plugin":
		response.AMR = []string{"urn:eparaksts:tws:policies:authentication:adaptive:methods:sc_plugin"}
		response.GivenName = h.config.SCGivenName
		response.FamilyName = h.config.SCFamilyName
		response.Name = h.config.SCGivenName + " " + h.config.SCFamilyName
	default:
		// Default fallback
		response.AMR = []string{"urn:authentication:adaptive:methods:plugin"}
		response.GivenName = h.config.SCGivenName
		response.FamilyName = h.config.SCFamilyName
		response.Name = h.config.SCGivenName + " " + h.config.SCFamilyName
	}

	return response
}

// sendError sends an error response
func (h *OAuthHandler) sendError(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	errorResp := &responses.ErrorResponse{
		Error:            errorCode,
		ErrorDescription: description,
	}

	json.NewEncoder(w).Encode(errorResp)
}
