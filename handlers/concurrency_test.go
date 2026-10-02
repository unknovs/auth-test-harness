package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// quietLogin runs the whole handshake for a flow and returns the person's name,
// reporting a failure as an error rather than stopping the test, so it can run
// on any goroutine.
func quietLogin(h *OAuthHandler, acrValues string) (string, error) {
	rec := authorize(h, authorizeQuery(acrValues))
	if rec.Code != http.StatusFound {
		return "", fmt.Errorf("authorize: HTTP %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		return "", err
	}
	rec = tokenRequest(h, "dGVzdDp0ZXN0", url.Values{
		"grant_type": {"authorization_code"}, "code": {loc.Query().Get("code")}, "redirect_uri": {testRedirectURI},
	})
	if rec.Code != http.StatusOK {
		return "", fmt.Errorf("token: HTTP %d %s", rec.Code, rec.Body.String())
	}
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tr); err != nil {
		return "", err
	}
	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	req.Header.Set("Authorization", "Bearer "+tr.AccessToken)
	rec = httptest.NewRecorder()
	h.UserInfoHandler(rec, req)
	var info struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(strings.NewReader(rec.Body.String())).Decode(&info); err != nil {
		return "", err
	}

	return info.Name, nil
}

// The HTTP server answers requests concurrently, and the clean-up ticker sweeps
// the same store meanwhile: many logins at once, with sweeps running, must all
// succeed and must not race (run with -race).
func TestConcurrentLoginsWithCleanup(t *testing.T) {
	h := testHandler()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			name, err := quietLogin(h, flowMobileID)
			if err != nil || name != "Jane Mobile" {
				t.Errorf("login: %q, %v", name, err)
			}
		}()
		go func() {
			defer wg.Done()
			h.store.CleanupExpired()
		}()
	}
	wg.Wait()
}
