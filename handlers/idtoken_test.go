package handlers

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/unknovs/auth-test-harness/utils"
)

// A provider's subject identifies a person: the same on every login of one
// profile, different between profiles — and the guest variant of the directory
// profile is the same person.
func TestProfileSubjectIsStableAndPerProfile(t *testing.T) {
	h := testHandler()

	if a, b := h.profileFor(flowMobileID).Sub, h.profileFor(flowMobileID).Sub; a != b || a == "" {
		t.Fatalf("the mobile profile's subject changed between two logins: %q vs %q", a, b)
	}
	if h.profileFor(flowMobileID).Sub == h.profileFor(flowEIDScan).Sub {
		t.Fatal("two different people share one subject")
	}
	if h.profileFor(flowDirectory).Sub != h.profileFor(flowDirectoryGuest).Sub {
		t.Fatal("the guest variant of the directory profile is the same person, and must keep the subject")
	}
	// A fresh handler with the same configuration answers the same subjects: a
	// restart does not turn a known person into a stranger.
	if testHandler().profileFor(flowDirectory).Sub != h.profileFor(flowDirectory).Sub {
		t.Fatal("the directory profile's subject is not stable across restarts")
	}
}

// A directory profile is a work account: a name, a durable object id — and no
// identity code, no acr. The userinfo answer omits what it does not have rather
// than sending empty values that read like a code of "".
func TestDirectoryProfileHasNoIdentityCode(t *testing.T) {
	h := testHandler()

	info := h.generateUserInfo(flowDirectory)
	if info.SerialNumber != "" || info.ACR != "" {
		t.Fatalf("a directory profile must carry no identity code and no acr, got serial=%q acr=%q", info.SerialNumber, info.ACR)
	}
	if len(info.AMR) != 1 || info.AMR[0] != amrDirectory {
		t.Fatalf("a directory login reports the directory method, got %v", info.AMR)
	}
	if info.Name != "Ilze Ozola" {
		t.Fatalf("name = %q", info.Name)
	}

	raw, _ := json.Marshal(info)
	for _, absent := range []string{`"serial_number"`, `"acr"`} {
		if strings.Contains(string(raw), absent) {
			t.Fatalf("the userinfo document carries %s for a directory profile: %s", absent, raw)
		}
	}
	// The directory identifiers live in the id_token, where a directory puts
	// them — the userinfo answer carries neither.
	for _, absent := range []string{`"oid"`, `"acct"`} {
		if strings.Contains(string(raw), absent) {
			t.Fatalf("the userinfo document carries %s: %s", absent, raw)
		}
	}
}

// decodeJWT splits a compact JWS and decodes its header and payload; it also
// returns the signing input and the signature for verification.
func decodeJWT(t *testing.T, tok string) (header, payload map[string]any, signingInput string, sig []byte) {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("not a compact JWS: %q", tok)
	}
	for i, target := range []*map[string]any{&header, &payload} {
		b, err := base64.RawURLEncoding.DecodeString(parts[i])
		if err != nil {
			t.Fatalf("part %d is not base64url: %v", i, err)
		}
		if err := json.Unmarshal(b, target); err != nil {
			t.Fatalf("part %d is not JSON: %v", i, err)
		}
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("signature is not base64url: %v", err)
	}

	return header, payload, parts[0] + "." + parts[1], sig
}

// The id_token says who logged in — issued by the advertised issuer, to the client
// that asked, for the profile's stable subject, bound to the request's nonce, with
// the directory identifiers a directory puts there — and it verifies with the key
// the service publishes.
func TestIDTokenCarriesTheLoginAndVerifiesWithThePublishedKey(t *testing.T) {
	h := testHandler()
	p := h.profileFor(flowDirectory)

	tok, err := h.idToken(utils.AuthCodeData{ClientID: "cid", ACRValues: flowDirectory, Nonce: "n-1"})
	if err != nil {
		t.Fatal(err)
	}
	header, claims, input, sig := decodeJWT(t, tok)

	if header["alg"] != "RS256" || header["kid"] != h.key.KeyID() {
		t.Fatalf("header = %v, want RS256 with kid %s", header, h.key.KeyID())
	}
	digest := sha256.Sum256([]byte(input))
	if err := rsa.VerifyPKCS1v15(h.key.Public(), crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("the id_token does not verify with the published key: %v", err)
	}

	want := map[string]any{
		"iss": "http://idp.test:8080", "aud": "cid", "sub": p.Sub, "nonce": "n-1",
		"name": "Ilze Ozola", "given_name": "Ilze", "family_name": "Ozola",
		"oid": p.ObjectID, "acct": float64(0),
	}
	for k, v := range want {
		if claims[k] != v {
			t.Errorf("claim %s = %v, want %v", k, claims[k], v)
		}
	}
	if _, has := claims["acr"]; has {
		t.Error("a directory login carries no acr")
	}
	if exp, iat := claims["exp"].(float64), claims["iat"].(float64); exp <= iat {
		t.Errorf("exp %v is not after iat %v", exp, iat)
	}
	if p.ObjectID == "" || !strings.Contains(p.ObjectID, "-") {
		t.Errorf("the directory object id is not derived: %q", p.ObjectID)
	}
}

// The guest variant: the same person, flagged as a guest of the directory.
func TestIDTokenGuestVariant(t *testing.T) {
	h := testHandler()

	tok, err := h.idToken(utils.AuthCodeData{ClientID: "cid", ACRValues: flowDirectoryGuest})
	if err != nil {
		t.Fatal(err)
	}
	_, claims, _, _ := decodeJWT(t, tok)
	if claims["acct"] != float64(accountStatusGuest) {
		t.Fatalf("acct = %v, want %d", claims["acct"], accountStatusGuest)
	}
	if claims["sub"] != h.profileFor(flowDirectory).Sub || claims["oid"] != h.profileFor(flowDirectory).ObjectID {
		t.Fatal("the guest variant must be the same person as the member")
	}
	if _, has := claims["nonce"]; has {
		t.Fatal("a request that sent no nonce gets no nonce claim")
	}
}

// A coded profile's id_token carries the method and the level, and none of the
// directory identifiers — and its subject equals the userinfo subject.
func TestIDTokenForACodedProfile(t *testing.T) {
	h := testHandler()

	tok, err := h.idToken(utils.AuthCodeData{ClientID: "cid", ACRValues: flowMobileID, Nonce: "n-2"})
	if err != nil {
		t.Fatal(err)
	}
	_, claims, _, _ := decodeJWT(t, tok)
	for _, absent := range []string{"oid", "acct"} {
		if _, has := claims[absent]; has {
			t.Errorf("a coded profile's id_token carries %s", absent)
		}
	}
	if claims["acr"] != "urn:safelayer:tws:policies:authentication:level:high" {
		t.Errorf("acr = %v", claims["acr"])
	}
	if claims["sub"] != h.generateUserInfo(flowMobileID).Sub {
		t.Error("the id_token and userinfo must name the same subject")
	}
}

// The published key set describes the signing key: RSA, for signatures, RS256,
// the kid every token carries, the modulus and the public exponent.
func TestJWKSPublishesTheSigningKey(t *testing.T) {
	h := testHandler()

	var set struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(h.key.JWKS(), &set); err != nil {
		t.Fatal(err)
	}
	if len(set.Keys) != 1 {
		t.Fatalf("keys = %d, want 1", len(set.Keys))
	}
	k := set.Keys[0]
	if k["kty"] != "RSA" || k["use"] != "sig" || k["alg"] != "RS256" || k["kid"] != h.key.KeyID() {
		t.Fatalf("key = %v", k)
	}
	n, err := base64.RawURLEncoding.DecodeString(k["n"])
	if err != nil || string(n) != string(h.key.Public().N.Bytes()) {
		t.Fatalf("the modulus does not match the key: %v", err)
	}
	if k["e"] != "AQAB" {
		t.Fatalf("e = %q, want AQAB (65537)", k["e"])
	}
}

// The whole handshake: an authorization request carrying a nonce answers a code;
// the token endpoint exchanges it for an access token AND an id_token whose nonce
// is the one sent; userinfo names the same subject the id_token does.
func TestTokenEndpointAnswersAnIDToken(t *testing.T) {
	h := testHandler()

	q := url.Values{
		"response_type": {"code"}, "client_id": {"cid"}, "redirect_uri": {"https://cb.example/x"},
		"scope": {"openid"}, "acr_values": {flowDirectory}, "state": {"s"}, "nonce": {"n-3"},
	}
	rec := httptest.NewRecorder()
	h.AuthorizeHandler(rec, httptest.NewRequest(http.MethodGet, "/authz?"+q.Encode(), nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("authorize: HTTP %d %s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	code := loc.Query().Get("code")

	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"https://cb.example/x"}}
	req := httptest.NewRequest(http.MethodPost, "/tok", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic dGVzdDp0ZXN0")
	rec = httptest.NewRecorder()
	h.TokenHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("token: HTTP %d %s", rec.Code, rec.Body.String())
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tr); err != nil {
		t.Fatal(err)
	}
	if tr.IDToken == "" {
		t.Fatal("the token response carries no id_token")
	}
	_, claims, _, _ := decodeJWT(t, tr.IDToken)
	if claims["nonce"] != "n-3" || claims["aud"] != "cid" {
		t.Fatalf("id_token nonce/aud = %v/%v", claims["nonce"], claims["aud"])
	}

	req = httptest.NewRequest(http.MethodGet, "/ui", nil)
	req.Header.Set("Authorization", "Bearer "+tr.AccessToken)
	rec = httptest.NewRecorder()
	h.UserInfoHandler(rec, req)
	var info struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Sub != claims["sub"] {
		t.Fatalf("userinfo sub %q differs from the id_token's %v", info.Sub, claims["sub"])
	}
}
