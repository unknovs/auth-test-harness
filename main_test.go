package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unknovs/auth-test-harness/env"
	"github.com/unknovs/auth-test-harness/handlers"
	"github.com/unknovs/auth-test-harness/identities"
	"github.com/unknovs/auth-test-harness/utils"
)

// The endpoint paths the example compose file configures: the eParaksts paths.
const (
	testAuthorizePath = "/trustedx-authserver/oauth/lvrtc-eipsign-as"
	testTokenPath     = "/trustedx-authserver/oauth/lvrtc-eipsign-as/token"
	testUserInfoPath  = "/trustedx-resources/openid/v1/users/me"
)

func testConfig() *env.Config {
	return &env.Config{
		Protocol:              "http",
		Host:                  "idp.test:8080",
		BasicAuthValue:        "dGVzdDp0ZXN0",
		TokenExpirationMin:    10,
		AuthorizationEndpoint: testAuthorizePath,
		TokenEndpoint:         testTokenPath,
		UserInfoEndpoint:      testUserInfoPath,
		ScopesSupported:       []string{"urn:lvrtc:fpeil:aa"},
		ACRValuesSupported:    []string{"urn:eparaksts:authentication:flow:mobileid"},
	}
}

func testMux(t *testing.T, config *env.Config) *http.ServeMux {
	t.Helper()
	key, err := utils.NewSigningKey()
	if err != nil {
		t.Fatal(err)
	}

	return newMux(config, handlers.NewOAuthHandler(config, utils.NewInMemoryStore(), key))
}

func get(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec
}

// Discovery answers at the spec path and at the underscore path the service first
// shipped, with the same document, and that document advertises the configured
// endpoints under the service's own address.
func TestDiscoveryAnswersAtBothPaths(t *testing.T) {
	mux := testMux(t, testConfig())

	var docs [2]map[string]any
	for i, path := range []string{"/.well-known/openid-configuration", "/.well-known/openid_configuration"} {
		rec := get(t, mux, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: HTTP %d", path, rec.Code)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &docs[i]); err != nil {
			t.Fatalf("%s: not JSON: %v", path, err)
		}
	}

	doc := docs[0]
	want := map[string]string{
		"issuer":                 "http://idp.test:8080",
		"authorization_endpoint": "http://idp.test:8080" + testAuthorizePath,
		"token_endpoint":         "http://idp.test:8080" + testTokenPath,
		"userinfo_endpoint":      "http://idp.test:8080" + testUserInfoPath,
		"jwks_uri":               "http://idp.test:8080/.well-known/jwks.json",
	}
	for k, v := range want {
		if doc[k] != v {
			t.Errorf("%s = %v, want %s", k, doc[k], v)
		}
	}
	a, _ := json.Marshal(docs[0])
	b, _ := json.Marshal(docs[1])
	if string(a) != string(b) {
		t.Errorf("the two discovery paths answer different documents:\n%s\n%s", a, b)
	}
}

func TestHealth(t *testing.T) {
	rec := get(t, testMux(t, testConfig()), "/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("body = %s (%v)", rec.Body.String(), err)
	}
}

// The root answers the service information document; any other unknown path is
// a plain 404, not the information document.
func TestRootAnswersServiceInfoAndNothingElse(t *testing.T) {
	mux := testMux(t, testConfig())

	rec := get(t, mux, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("/: HTTP %d", rec.Code)
	}
	var info map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("/: not JSON: %v", err)
	}
	if info["openid_configuration"] != "http://idp.test:8080/.well-known/openid-configuration" {
		t.Errorf("openid_configuration = %v", info["openid_configuration"])
	}

	if rec := get(t, mux, "/no-such-path"); rec.Code != http.StatusNotFound {
		t.Errorf("/no-such-path: HTTP %d, want 404", rec.Code)
	}
}

// The configured endpoints are mounted where the configuration says.
func TestConfiguredEndpointsAreMounted(t *testing.T) {
	mux := testMux(t, testConfig())

	// Each answers its own error for a bare request — proof it is mounted, since
	// an unmounted path would fall through to the root's 404.
	for path, want := range map[string]int{
		testAuthorizePath:        http.StatusBadRequest,
		testTokenPath:            http.StatusMethodNotAllowed,
		testUserInfoPath:         http.StatusBadRequest,
		"/.well-known/jwks.json": http.StatusOK,
	} {
		if rec := get(t, mux, path); rec.Code != want {
			t.Errorf("GET %s: HTTP %d, want %d", path, rec.Code, want)
		}
	}
}

// The logout endpoint is served only where LOGOUT_ENDPOINT puts it, and the
// service information document lists it only then.
func TestLogoutIsMountedOnlyWhenConfigured(t *testing.T) {
	const logoutPath = "/trustedx-authserver/lvrtc-eipsign-idp/logout"

	mux := testMux(t, testConfig())
	if rec := get(t, mux, logoutPath+"?redirect_uri=https%3A%2F%2Fapp.example%2Fout"); rec.Code != http.StatusNotFound {
		t.Fatalf("unconfigured logout: HTTP %d, want 404", rec.Code)
	}
	if strings.Contains(get(t, mux, "/").Body.String(), `"logout"`) {
		t.Fatal("the service information lists a logout that is not served")
	}

	config := testConfig()
	config.LogoutEndpoint = logoutPath
	mux = testMux(t, config)
	rec := get(t, mux, logoutPath+"?redirect_uri=https%3A%2F%2Fapp.example%2Fout")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://app.example/out" {
		t.Fatalf("logout: HTTP %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}
	var info struct {
		Endpoints map[string]string `json:"endpoints"`
	}
	if err := json.Unmarshal(get(t, mux, "/").Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Endpoints["logout"] != "http://idp.test:8080"+logoutPath {
		t.Fatalf("service information logout = %q", info.Endpoints["logout"])
	}
}

// USED_IDENTITIES: config keeps the profiles and serves no code step; list needs
// a file and serves the code step; anything else is refused at start.
func TestUseIdentities(t *testing.T) {
	logf := func(string, ...any) {}
	handler := func(config *env.Config) *handlers.OAuthHandler {
		key, err := utils.NewSigningKey()
		if err != nil {
			t.Fatal(err)
		}
		return handlers.NewOAuthHandler(config, utils.NewInMemoryStore(), key)
	}

	config := testConfig()
	config.UsedIdentities = env.IdentitiesFromConfig
	h := handler(config)
	if err := useIdentities(config, h, logf); err != nil || h.UsesIdentityList() {
		t.Fatalf("config: err %v, list %v", err, h.UsesIdentityList())
	}
	rec := httptest.NewRecorder()
	newMux(config, h).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, handlers.CodeStepPath, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("config: the code step answers HTTP %d, want 404", rec.Code)
	}

	// list with no file: the built-in list, nothing to manage.
	config.UsedIdentities = env.IdentitiesFromList
	h = handler(config)
	if err := useIdentities(config, h, logf); err != nil || !h.UsesIdentityList() {
		t.Fatalf("list without a file: err %v, list %v", err, h.UsesIdentityList())
	}

	config.IdentitiesFile = filepath.Join(t.TempDir(), "identities.json")
	h = handler(config)
	if err := useIdentities(config, h, logf); err != nil || !h.UsesIdentityList() {
		t.Fatalf("list: err %v, list %v", err, h.UsesIdentityList())
	}
	rec = httptest.NewRecorder()
	newMux(config, h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, handlers.CodeStepPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("list: GET code step answers HTTP %d, want 405 (mounted, POST only)", rec.Code)
	}

	config.UsedIdentities = "file"
	if err := useIdentities(config, handler(config), logf); err == nil || !strings.Contains(err.Error(), `"file"`) {
		t.Fatalf("an unknown mode: err %v", err)
	}
}

// The built-in list is usable: at least twenty people, found by their codes,
// and it is the repository's example list.
func TestTheBuiltInListIsTheExampleList(t *testing.T) {
	l, err := identities.Builtin(builtinIdentities)
	if err != nil {
		t.Fatalf("the built-in list is refused: %v", err)
	}
	if l.Len() < 20 {
		t.Fatalf("the built-in list holds %d people, want at least 20", l.Len())
	}
	for _, code := range []string{"PNOLV-000123-00001", "000123-00020"} {
		if _, ok := l.Lookup(code); !ok {
			t.Errorf("%s is not in the built-in list", code)
		}
	}
	file, err := os.ReadFile(filepath.Join("examples", "identities", "identities.json"))
	if err != nil || string(file) != string(builtinIdentities) {
		t.Fatalf("the built-in list is not the example file (%v)", err)
	}
}
