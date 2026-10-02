package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testRedirectURI = "https://cb.example/back"

// authorizeQuery is a valid authorization request for one flow; a case changes
// or removes a parameter from it.
func authorizeQuery(acrValues string) url.Values {
	return url.Values{
		"response_type": {"code"}, "client_id": {"cid"}, "redirect_uri": {testRedirectURI},
		"scope": {"openid"}, "acr_values": {acrValues}, "state": {"st-1"},
	}
}

func authorize(h *OAuthHandler, q url.Values) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.AuthorizeHandler(rec, httptest.NewRequest(http.MethodGet, "/authz?"+q.Encode(), nil))

	return rec
}

func tokenRequest(h *OAuthHandler, basic string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/tok", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basic != "" {
		req.Header.Set("Authorization", "Basic "+basic)
	}
	rec := httptest.NewRecorder()
	h.TokenHandler(rec, req)

	return rec
}

func userInfo(h *OAuthHandler, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.UserInfoHandler(rec, req)

	return rec
}

// codeFrom reads the code out of a redirect back to the client, and checks the
// redirect goes where the client asked, carrying its state.
func codeFrom(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusFound {
		t.Fatalf("want a redirect back, got HTTP %d %s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if got := loc.Scheme + "://" + loc.Host + loc.Path; got != testRedirectURI {
		t.Fatalf("redirected to %s, want %s", got, testRedirectURI)
	}
	if loc.Query().Get("state") != "st-1" {
		t.Fatalf("state = %q, want st-1", loc.Query().Get("state"))
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatal("the redirect carries no code")
	}

	return code
}

func wantError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("HTTP %d, want %d (%s)", rec.Code, status, rec.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not a JSON error: %s", rec.Body.String())
	}
	if body.Error != code {
		t.Fatalf("error = %q, want %q", body.Error, code)
	}
}

// A valid request is answered at once with a code, back at the redirect_uri
// with the state; no state sent, none sent back.
func TestAuthorizeRedirectsBackWithACode(t *testing.T) {
	h := testHandler()
	codeFrom(t, authorize(h, authorizeQuery(flowMobileID)))

	q := authorizeQuery(flowMobileID)
	q.Del("state")
	rec := authorize(h, q)
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if _, has := loc.Query()["state"]; has {
		t.Fatal("a request without state got a state back")
	}
}

// Each malformed authorization request is refused with its own error, and no
// code is issued.
func TestAuthorizeRefusesMalformedRequests(t *testing.T) {
	h := testHandler()

	cases := []struct {
		name  string
		edit  func(url.Values)
		error string
	}{
		{"response_type not code", func(q url.Values) { q.Set("response_type", "token") }, "invalid_request"},
		{"no client_id", func(q url.Values) { q.Del("client_id") }, "invalid_request"},
		{"no redirect_uri", func(q url.Values) { q.Del("redirect_uri") }, "invalid_request"},
		{"scope not supported", func(q url.Values) { q.Set("scope", "other") }, "invalid_scope"},
		{"acr_values not supported", func(q url.Values) { q.Set("acr_values", "urn:other:flow") }, "invalid_request"},
		{"no acr_values", func(q url.Values) { q.Del("acr_values") }, "invalid_request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := authorizeQuery(flowMobileID)
			tc.edit(q)
			wantError(t, authorize(h, q), http.StatusBadRequest, tc.error)
		})
	}

	rec := httptest.NewRecorder()
	h.AuthorizeHandler(rec, httptest.NewRequest(http.MethodPost, "/authz", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST authorize: HTTP %d, want 405", rec.Code)
	}
}

// The token endpoint takes only the configured client credentials, only the
// authorization_code grant, only a code it issued, only once, and only with the
// redirect_uri the code was issued for.
func TestTokenEndpointRefusals(t *testing.T) {
	h := testHandler()
	const basic = "dGVzdDp0ZXN0"

	exchange := func(code string) url.Values {
		return url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {testRedirectURI}}
	}

	code := codeFrom(t, authorize(h, authorizeQuery(flowMobileID)))
	wantError(t, tokenRequest(h, "", exchange(code)), http.StatusBadRequest, "invalid_client")
	wantError(t, tokenRequest(h, "d3Jvbmc6d3Jvbmc=", exchange(code)), http.StatusBadRequest, "invalid_client")

	form := exchange(code)
	form.Set("grant_type", "client_credentials")
	wantError(t, tokenRequest(h, basic, form), http.StatusBadRequest, "unsupported_grant_type")

	wantError(t, tokenRequest(h, basic, exchange("not-a-code")), http.StatusBadRequest, "invalid_grant")

	// The code is spent by its first exchange — even one that is then refused for
	// its redirect_uri.
	form = exchange(code)
	form.Set("redirect_uri", "https://cb.example/elsewhere")
	wantError(t, tokenRequest(h, basic, form), http.StatusBadRequest, "invalid_grant")
	wantError(t, tokenRequest(h, basic, exchange(code)), http.StatusBadRequest, "invalid_grant")

	code = codeFrom(t, authorize(h, authorizeQuery(flowMobileID)))
	if rec := tokenRequest(h, basic, exchange(code)); rec.Code != http.StatusOK {
		t.Fatalf("a valid exchange: HTTP %d %s", rec.Code, rec.Body.String())
	}
	wantError(t, tokenRequest(h, basic, exchange(code)), http.StatusBadRequest, "invalid_grant")

	rec := httptest.NewRecorder()
	h.TokenHandler(rec, httptest.NewRequest(http.MethodGet, "/tok", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET token: HTTP %d, want 405", rec.Code)
	}
}

// Userinfo answers only a bearer token the token endpoint issued.
func TestUserInfoRefusals(t *testing.T) {
	h := testHandler()

	wantError(t, userInfo(h, ""), http.StatusBadRequest, "invalid_token")
	wantError(t, userInfo(h, "not-a-token"), http.StatusBadRequest, "invalid_token")

	rec := httptest.NewRecorder()
	h.UserInfoHandler(rec, httptest.NewRequest(http.MethodPost, "/ui", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST userinfo: HTTP %d, want 405", rec.Code)
	}
}

// login runs the whole handshake for a flow in configuration mode and returns the
// userinfo document.
func login(t *testing.T, h *OAuthHandler, acrValues string) map[string]any {
	t.Helper()
	code := codeFrom(t, authorize(h, authorizeQuery(acrValues)))

	return exchangeAndRead(t, h, code)
}

// exchangeAndRead exchanges a code and reads userinfo with the access token.
func exchangeAndRead(t *testing.T, h *OAuthHandler, code string) map[string]any {
	t.Helper()
	rec := tokenRequest(h, "dGVzdDp0ZXN0", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {testRedirectURI},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("token: HTTP %d %s", rec.Code, rec.Body.String())
	}
	var tr struct {
		AccessToken string  `json:"access_token"`
		TokenType   string  `json:"token_type"`
		ExpiresIn   int     `json:"expires_in"`
		Scope       *string `json:"scope"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tr); err != nil {
		t.Fatal(err)
	}
	if tr.TokenType != "Bearer" || tr.ExpiresIn != 600 {
		t.Fatalf("token_type %q expires_in %d, want Bearer 600", tr.TokenType, tr.ExpiresIn)
	}
	// The granted scope comes back: the one the authorization request asked for.
	if tr.Scope == nil {
		t.Fatal("the token answer carries no scope")
	}
	if *tr.Scope != "openid" {
		t.Fatalf("scope = %q, want the requested openid", *tr.Scope)
	}

	rec = userInfo(h, tr.AccessToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("userinfo: HTTP %d %s", rec.Code, rec.Body.String())
	}
	var info map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}

	return info
}

// Over the wire, each flow answers its configured person, the fixed domain, the
// requested flow echoed as the acr, and an empty eips.
func TestUserInfoOverTheWirePerFlow(t *testing.T) {
	h := testHandler()

	for acr, want := range map[string][2]string{
		flowMobileID: {"Jane Mobile", personA},
		flowSCPlugin: {"John Cardreader", personA},
		flowEIDScan:  {"Erik Scanner", personB},
	} {
		info := login(t, h, acr)
		if info["name"] != want[0] || info["serial_number"] != want[1] {
			t.Errorf("%s: name %v serial %v, want %s %s", acr, info["name"], info["serial_number"], want[0], want[1])
		}
		if info["domain"] != "citizen" || info["acr"] != acr || info["eips"] != "" {
			t.Errorf("%s: domain %v acr %v eips %v", acr, info["domain"], info["acr"], info["eips"])
		}
	}
}
