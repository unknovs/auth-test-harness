package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/unknovs/auth-test-harness/identities"
)

// Two listed people: codes assembled at run time, as in the other tests.
var (
	listedA = testIDCodeLV("7")
	listedB = testIDCodeLV("8")
)

func writeList(t *testing.T, path string, tick int, people ...[3]string) {
	t.Helper()
	entries := make([]string, len(people))
	for i, p := range people {
		entries[i] = fmt.Sprintf(`{"serial_number": %q, "given_name": %q, "family_name": %q}`, p[0], p[1], p[2])
	}
	if err := os.WriteFile(path, []byte(`{"identities": [`+strings.Join(entries, ",")+`]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(time.Duration(tick) * time.Minute)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// listHandler is the test handler with an identity list of two people in use; it
// returns the list's path so a test can change the file.
func listHandler(t *testing.T) (*OAuthHandler, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "identities.json")
	writeList(t, path, 0, [3]string{listedA, "Anna", "Bērziņa"}, [3]string{listedB, "Jānis", "Ozols"})
	h := testHandler()
	h.SetIdentityList(identities.Open(path, t.Logf))

	return h, path
}

var hiddenRequest = regexp.MustCompile(`name="request" value="([^"]+)"`)

// codeStep checks that an authorization request was answered with the sign-in
// page, and returns the id of the request waiting there.
func codeStep(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("want the sign-in page, got HTTP %d %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q", ct)
	}
	m := hiddenRequest.FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatalf("the page carries no request: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `action="`+CodeStepPath+`"`) || !strings.Contains(rec.Body.String(), `name="code"`) {
		t.Fatalf("the page has no code field posting to the code step: %s", rec.Body.String())
	}

	return m[1]
}

func enterCode(h *OAuthHandler, request, code string) *httptest.ResponseRecorder {
	form := url.Values{"request": {request}, "code": {code}}
	req := httptest.NewRequest(http.MethodPost, CodeStepPath, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.CodeStepHandler(rec, req)

	return rec
}

// listLogin signs in through the code step as the person a typed code names,
// and returns the userinfo document.
func listLogin(t *testing.T, h *OAuthHandler, acrValues, typed string) map[string]any {
	t.Helper()
	request := codeStep(t, authorize(h, authorizeQuery(acrValues)))

	return exchangeAndRead(t, h, codeFrom(t, enterCode(h, request, typed)))
}

// With a list in use, a flow that takes a personal code asks for it, and the
// login answers as the person it names — by Mobile ID and by eID Scan, each with
// its flow as the acr and the one mobile method as the amr.
func TestListLoginAnswersAsThePersonEntered(t *testing.T) {
	h, _ := listHandler(t)

	for acr, wantAMR := range map[string]string{
		flowMobileID: amrMobile,
		flowEIDScan:  amrMobile,
	} {
		info := listLogin(t, h, acr, listedB)
		if info["given_name"] != "Jānis" || info["family_name"] != "Ozols" || info["name"] != "Jānis Ozols" || info["serial_number"] != listedB {
			t.Errorf("%s: answered as %v %v (%v), want Jānis Ozols (%s)", acr, info["given_name"], info["family_name"], info["serial_number"], listedB)
		}
		amr, _ := info["amr"].([]any)
		if len(amr) != 1 || amr[0] != wantAMR {
			t.Errorf("%s: amr = %v, want [%s]", acr, info["amr"], wantAMR)
		}
		if info["acr"] != acr || info["domain"] != "citizen" {
			t.Errorf("%s: acr %v domain %v", acr, info["acr"], info["domain"])
		}
	}
}

// The page names the method it signs in with.
func TestCodeStepPageNamesTheMethod(t *testing.T) {
	h, _ := listHandler(t)
	for acr, label := range map[string]string{flowMobileID: "Mobile ID", flowEIDScan: "eID Scan"} {
		rec := authorize(h, authorizeQuery(acr))
		codeStep(t, rec)
		if !strings.Contains(rec.Body.String(), "Sign in with "+label) {
			t.Errorf("%s: the page does not say %q", acr, label)
		}
	}
}

// A person types their code without its prefix; that names them too.
func TestListLoginByTheCodeAsAPersonTypesIt(t *testing.T) {
	h, _ := listHandler(t)
	info := listLogin(t, h, flowMobileID, " "+strings.ToLower(strings.TrimPrefix(listedA, "PNOLV-"))+" ")
	if info["serial_number"] != listedA || info["name"] != "Anna Bērziņa" {
		t.Fatalf("answered as %v (%v)", info["name"], info["serial_number"])
	}
}

// The subject derives from the method and the person's code: the same on every
// login by one method, unchanged by a renamed entry, and different by the other
// method and between people — as at the platform, where one person signs in
// under a different subject by each method.
func TestListSubjectIsStablePerCode(t *testing.T) {
	h, path := listHandler(t)

	first := listLogin(t, h, flowMobileID, listedA)["sub"]
	if again := listLogin(t, h, flowMobileID, listedA)["sub"]; again != first {
		t.Fatalf("the subject changed between two logins: %v vs %v", first, again)
	}
	scan := listLogin(t, h, flowEIDScan, listedA)["sub"]
	if scan == first {
		t.Fatal("one person has the same subject by both methods")
	}
	if again := listLogin(t, h, flowEIDScan, listedA)["sub"]; again != scan {
		t.Fatalf("the eID Scan subject changed between two logins: %v vs %v", scan, again)
	}
	if other := listLogin(t, h, flowMobileID, listedB)["sub"]; other == first {
		t.Fatal("two people share one subject")
	}

	writeList(t, path, 1, [3]string{listedA, "Anna", "Kalniņa"})
	renamed := listLogin(t, h, flowMobileID, listedA)
	if renamed["sub"] != first || renamed["family_name"] != "Kalniņa" {
		t.Fatalf("after a rename: sub %v (want %v), family_name %v", renamed["sub"], first, renamed["family_name"])
	}

	// A restart does not turn a known person into a stranger.
	h2, _ := listHandler(t)
	if again := listLogin(t, h2, flowMobileID, listedA)["sub"]; again != first {
		t.Fatalf("the subject is not stable across restarts: %v vs %v", again, first)
	}
}

// The id_token names the person entered, with the same subject as userinfo, and
// carries the request's nonce.
func TestListLoginIDToken(t *testing.T) {
	h, _ := listHandler(t)
	q := authorizeQuery(flowEIDScan)
	q.Set("nonce", "n-list")
	request := codeStep(t, authorize(h, q))
	code := codeFrom(t, enterCode(h, request, listedA))

	rec := tokenRequest(h, "dGVzdDp0ZXN0", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {testRedirectURI},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("token: HTTP %d %s", rec.Code, rec.Body.String())
	}
	tok := regexp.MustCompile(`"id_token":"([^"]+)"`).FindStringSubmatch(rec.Body.String())
	if tok == nil {
		t.Fatalf("no id_token: %s", rec.Body.String())
	}
	_, claims, _, _ := decodeJWT(t, tok[1])
	if claims["name"] != "Anna Bērziņa" || claims["nonce"] != "n-list" || claims["aud"] != "cid" {
		t.Fatalf("claims = %v", claims)
	}
	if claims["sub"] != listLogin(t, h, flowEIDScan, listedA)["sub"] {
		t.Fatal("the id_token and userinfo name different subjects")
	}
}

// A code the list does not name shows the page again with the reason, the code
// as entered and the same request, which is still answered by a correct code.
func TestCodeStepUnknownCodeShowsThePageAgain(t *testing.T) {
	h, _ := listHandler(t)
	request := codeStep(t, authorize(h, authorizeQuery(flowMobileID)))
	unknown := testIDCodeLV("9")

	rec := enterCode(h, request, unknown)
	if again := codeStep(t, rec); again != request {
		t.Fatalf("the page came back for another request: %s vs %s", again, request)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "No one with this code is in the identity list.") || !strings.Contains(body, `value="`+unknown+`"`) {
		t.Fatalf("the page does not say why, or lost the code: %s", body)
	}

	rec = enterCode(h, request, "")
	codeStep(t, rec)
	if !strings.Contains(rec.Body.String(), "Enter a personal code.") {
		t.Fatalf("an empty code: %s", rec.Body.String())
	}

	exchangeAndRead(t, h, codeFrom(t, enterCode(h, request, listedA)))
}

// A request the code step does not hold — never issued, or already answered —
// is refused, and no code is issued for it.
func TestCodeStepRefusesARequestItDoesNotHold(t *testing.T) {
	h, _ := listHandler(t)

	for name, request := range map[string]string{"unknown": "no-such-request", "empty": ""} {
		rec := enterCode(h, request, listedA)
		if rec.Code != http.StatusBadRequest || rec.Header().Get("Location") != "" {
			t.Errorf("%s request: HTTP %d, Location %q", name, rec.Code, rec.Header().Get("Location"))
		}
		if !strings.Contains(rec.Body.String(), "This sign-in has expired or is unknown.") || hiddenRequest.MatchString(rec.Body.String()) {
			t.Errorf("%s request: %s", name, rec.Body.String())
		}
	}

	request := codeStep(t, authorize(h, authorizeQuery(flowMobileID)))
	codeFrom(t, enterCode(h, request, listedA))
	if rec := enterCode(h, request, listedA); rec.Code != http.StatusBadRequest {
		t.Fatalf("a request answered twice: HTTP %d", rec.Code)
	}

	rec := httptest.NewRecorder()
	h.CodeStepHandler(rec, httptest.NewRequest(http.MethodGet, CodeStepPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET code step: HTTP %d, want 405", rec.Code)
	}
}

// The person is fixed when the code is entered: a list changed before the token
// is used does not change who the login names.
func TestListChangeAfterTheCodeStepKeepsThePerson(t *testing.T) {
	h, path := listHandler(t)
	request := codeStep(t, authorize(h, authorizeQuery(flowMobileID)))
	code := codeFrom(t, enterCode(h, request, listedA))

	writeList(t, path, 1, [3]string{listedB, "Jānis", "Ozols"})
	if info := exchangeAndRead(t, h, code); info["name"] != "Anna Bērziņa" {
		t.Fatalf("answered as %v", info["name"])
	}
}

// The flows that take no code — the smart card, the directory — keep their
// configured profiles with a list in use, and answer at once.
func TestListLeavesTheOtherFlowsConfigured(t *testing.T) {
	h, _ := listHandler(t)

	if info := login(t, h, flowSCPlugin); info["name"] != "John Cardreader" || info["serial_number"] != personA {
		t.Errorf("smart card: %v (%v)", info["name"], info["serial_number"])
	}
	if info := login(t, h, flowDirectory); info["name"] != "Ilze Ozola" {
		t.Errorf("directory: %v", info["name"])
	}
}

// Without a list, the flows that take a code answer at once as their configured
// profiles, as before.
func TestWithoutAListNoCodeIsAsked(t *testing.T) {
	h := testHandler()
	if h.UsesIdentityList() {
		t.Fatal("a handler without a list reports one")
	}
	if info := login(t, h, flowEIDScan); info["name"] != "Erik Scanner" {
		t.Fatalf("eID Scan: %v", info["name"])
	}
}

func logout(h *OAuthHandler, method, query string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.LogoutHandler(rec, httptest.NewRequest(method, "/trustedx-authserver/lvrtc-eipsign-idp/logout"+query, nil))

	return rec
}

// Logout sends the browser back to the redirect_uri it was given, exactly.
func TestLogoutSendsTheBrowserBack(t *testing.T) {
	h := testHandler()
	back := "https://app.example/signed-out?x=1&y=two"

	rec := logout(h, http.MethodGet, "?redirect_uri="+url.QueryEscape(back))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != back {
		t.Fatalf("HTTP %d, Location %q, want 302 to %s", rec.Code, rec.Header().Get("Location"), back)
	}
}

func TestLogoutRefusals(t *testing.T) {
	h := testHandler()

	wantError(t, logout(h, http.MethodGet, ""), http.StatusBadRequest, "invalid_request")
	wantError(t, logout(h, http.MethodGet, "?redirect_uri=%2Fonly-a-path"), http.StatusBadRequest, "invalid_request")
	if rec := logout(h, http.MethodPost, "?redirect_uri="+url.QueryEscape(testRedirectURI)); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST logout: HTTP %d, want 405", rec.Code)
	}
}
