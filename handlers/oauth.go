package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/unknovs/auth-test-harness/env"
	"github.com/unknovs/auth-test-harness/routes/requests"
	"github.com/unknovs/auth-test-harness/routes/responses"
	"github.com/unknovs/auth-test-harness/utils"
)

// OAuthHandler handles OAuth operations
type OAuthHandler struct {
	config *env.Config
	store  *utils.InMemoryStore
	key    *utils.SigningKey
}

// NewOAuthHandler creates a new OAuth handler. key signs the id_tokens; it is
// the key published at the jwks_uri.
func NewOAuthHandler(config *env.Config, store *utils.InMemoryStore, key *utils.SigningKey) *OAuthHandler {
	return &OAuthHandler{
		config: config,
		store:  store,
		key:    key,
	}
}

// JWKSHandler publishes the signing key set at /.well-known/jwks.json — the
// address the discovery document has advertised all along.
func (h *OAuthHandler) JWKSHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	// Response already committed; a short write has nowhere to go from here.
	_, _ = w.Write(h.key.JWKS())
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
		Nonce:        r.URL.Query().Get("nonce"),
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

	// Store the authorization code (with the nonce the id_token must carry back)
	h.store.StoreAuthCode(code, req.ClientID, req.RedirectURI, req.Scope, req.ACRValues, req.Nonce)

	// The code is this service's own generated value and the client id came from the
	// request; both are echoed into a test-run log on a loopback address, never into a
	// shared log sink.
	//nolint:gosec // G706: deliberate — a test double logging its own handshake
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

	// Redirect to the callback URL. A real authorization server validates redirect_uri
	// against the client's registered allowlist; this mock accepts whatever the test asks
	// for ON PURPOSE — exercising a wrong or hostile redirect_uri is one of the things a
	// harness exists to do. Never run this service anywhere a real user could reach it.
	//nolint:gosec // G710: open redirect is the point of a mock authorization endpoint
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
		//nolint:gosec // G706: as above — a test double logging its own handshake
		log.Printf("Invalid or expired auth code: %s", req.Code)
		h.sendError(w, "invalid_grant", "Invalid or expired authorization code")
		return
	}

	//nolint:gosec // G706: as above — a test double logging its own handshake
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

	// The id_token: who logged in, signed with the published key, bound to the
	// client that asked and to the nonce it sent.
	idToken, err := h.idToken(authCodeData)
	if err != nil {
		h.sendError(w, "server_error", "could not sign the id_token")
		return
	}

	// Prepare response
	response := &responses.TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   h.config.TokenExpirationMin * 60, // Convert to seconds
		IDToken:     idToken,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	// The status and headers are already on the wire; an encode failure here can
	// only be logged, and this is a test double on a loopback address.
	_ = json.NewEncoder(w).Encode(response)
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
	// The status and headers are already on the wire; an encode failure here can
	// only be logged, and this is a test double on a loopback address.
	_ = json.NewEncoder(w).Encode(response)
}

// profile is the person a requested flow answers as: everything both the
// userinfo answer and the id_token are built from, so the two can never name
// different people.
type profile struct {
	Sub          string
	GivenName    string
	FamilyName   string
	Name         string
	SerialNumber string // "" for a directory profile — a work account carries no identity code
	ACR          string // "" for a directory profile
	AMR          []string
	// Directory profiles only: the durable object id the directory knows the
	// person by, and whether they are a member of it (0) or a guest in it (1).
	Directory     bool
	ObjectID      string
	AccountStatus int
}

// profileFor selects the profile a requested flow answers as.
//
// Each eParaksts-shaped profile may carry its own identity code. Systems that key
// a person on their identity code then see a DIFFERENT person per flow, which is
// what lets one instance stand in for two parties — an owner and a counterparty —
// in a flow where a document is shared between them. Unset per profile, they all
// report the one SERIAL_NUMBER, i.e. the same person by any method. The
// authentication method is reported in the amr, derived from the flow that was
// requested, so a client that forces a specific method gets that method back and
// can verify the binding.
//
// The directory flows answer as a person with a work account: a name, a durable
// object id, an account status — and no identity code, because a directory holds
// none. Their amr names the directory method, so a client's vocabulary can map
// it. The guest flow is the same person flagged as a guest of the directory.
//
// The subject is STABLE per profile — the same on every login, different between
// profiles — because a provider's `sub` identifies a person: a client stores a
// credential under it and checks that the id_token and userinfo name the same one.
func (h *OAuthHandler) profileFor(acrValues string) profile {
	var p profile

	switch acrValues {
	case flowDirectory, flowDirectoryGuest:
		p.Directory = true
		p.GivenName = h.config.DirectoryGivenName
		p.FamilyName = h.config.DirectoryFamilyName
		p.ObjectID = h.config.DirectoryObjectID
		if p.ObjectID == "" {
			p.ObjectID = utils.StableObjectID("directory", p.GivenName, p.FamilyName)
		}
		if acrValues == flowDirectoryGuest {
			p.AccountStatus = accountStatusGuest
		}
		p.AMR = []string{amrDirectory}
		p.Sub = utils.StableSubject("directory", p.ObjectID)
	case flowMobileID:
		p.GivenName = h.config.MobileGivenName
		p.FamilyName = h.config.MobileFamilyName
		p.SerialNumber = h.config.MobileSerialNumber
	case flowEIDScan:
		p.GivenName = h.config.EIDScanGivenName
		p.FamilyName = h.config.EIDScanFamilyName
		p.SerialNumber = h.config.EIDScanSerialNumber
	default:
		// Smart card, and any other advertised flow.
		p.GivenName = h.config.SCGivenName
		p.FamilyName = h.config.SCFamilyName
		p.SerialNumber = h.config.SCSerialNumber
	}

	if !p.Directory {
		p.ACR = "urn:safelayer:tws:policies:authentication:level:high" // hardcoded, as the provider reports it
		p.AMR = []string{amrForFlow(acrValues)}
		// One credential per method for a person: the subject is the profile's.
		p.Sub = utils.StableSubject(acrValues, p.SerialNumber, p.GivenName, p.FamilyName)
	}
	p.Name = strings.TrimSpace(p.GivenName + " " + p.FamilyName)

	return p
}

// generateUserInfo generates user information based on ACR values
func (h *OAuthHandler) generateUserInfo(acrValues string) *responses.UserInfoResponse {
	p := h.profileFor(acrValues)

	return &responses.UserInfoResponse{
		Sub:          p.Sub,
		Domain:       "citizen", // hardcoded as per requirements
		ACR:          p.ACR,
		AMR:          p.AMR,
		GivenName:    p.GivenName,
		FamilyName:   p.FamilyName,
		Name:         p.Name,
		SerialNumber: p.SerialNumber,
		EIPS:         "",
	}
}

// idToken signs the statement of who logged in for one exchanged code: issued by
// this service's advertised issuer, to the client that asked, for the profile's
// stable subject, carrying the nonce the client sent, the names, the method, and
// — for a directory profile — the durable object id and the account status,
// which is where a directory puts them (its userinfo carries neither).
func (h *OAuthHandler) idToken(code utils.AuthCodeData) (string, error) {
	p := h.profileFor(code.ACRValues)
	now := time.Now()
	claims := map[string]any{
		"iss":         h.config.Protocol + "://" + h.config.Host,
		"aud":         code.ClientID,
		"sub":         p.Sub,
		"iat":         now.Unix(),
		"exp":         now.Add(time.Duration(h.config.TokenExpirationMin) * time.Minute).Unix(),
		"name":        p.Name,
		"given_name":  p.GivenName,
		"family_name": p.FamilyName,
		"amr":         p.AMR,
	}
	if code.Nonce != "" {
		claims["nonce"] = code.Nonce
	}
	if p.ACR != "" {
		claims["acr"] = p.ACR
	}
	if p.Directory {
		claims["oid"] = p.ObjectID
		claims["acct"] = p.AccountStatus
	}

	return h.key.SignRS256(claims)
}

// Requested authentication flows (the acr_values a client sends).
const (
	flowMobileID = "urn:eparaksts:authentication:flow:mobileid"
	flowSCPlugin = "urn:eparaksts:authentication:flow:sc_plugin"
	flowEIDScan  = "urn:eparaksts:authentication:flow:mobile-eid"

	// The directory flows: a work account at the person's organisation, as a
	// member of that directory or as a guest in it.
	flowDirectory      = "urn:auth-test-harness:flow:directory"
	flowDirectoryGuest = "urn:auth-test-harness:flow:directory-guest"

	// The reported-method prefix: this URN plus the method segment forms the amr.
	amrMethodPrefix = "urn:eparaksts:tws:policies:authentication:adaptive:methods:"
	// The method a directory login reports.
	amrDirectory = "urn:auth-test-harness:methods:directory"

	// The account status a directory reports for a guest (a member is 0).
	accountStatusGuest = 1
)

// amrForFlow maps a requested flow URN to the reported authentication-method
// URN by carrying its trailing method segment across — so mobileid, sc_plugin,
// mobile-eid and any future flow segment are all reported truthfully without a
// per-method branch. An unrecognisable value reports the smart-card method, the
// same fallback the name profile uses.
//
// Note for anyone comparing against the live platform: it currently reports the
// same amr (…methods:mobileid) for both the mobile and the eID Scan flow and
// distinguishes them only in the acr. Reporting the requested method here is the
// more useful behaviour for a test double, because it lets a caller assert that
// the method it forced is the method it got.
func amrForFlow(acrValues string) string {
	segment := ""
	if i := strings.LastIndex(acrValues, ":"); i >= 0 && i+1 < len(acrValues) {
		segment = acrValues[i+1:]
	}
	if segment == "" {
		segment = strings.TrimPrefix(flowSCPlugin, "urn:eparaksts:authentication:flow:")
	}

	return amrMethodPrefix + segment
}

// sendError sends an error response
func (h *OAuthHandler) sendError(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	errorResp := &responses.ErrorResponse{
		Error:            errorCode,
		ErrorDescription: description,
	}

	// The status and headers are already on the wire; an encode failure here can
	// only be logged, and this is a test double on a loopback address.
	_ = json.NewEncoder(w).Encode(errorResp)
}
