package handlers

import (
	"html/template"
	"log"
	"net/http"
	"net/url"

	"github.com/unknovs/auth-test-harness/utils"
)

// CodeStepPath is where the code step's form is sent: the sign-in page of the
// flows that take a personal code, when an identity list is in use.
const CodeStepPath = "/identify"

// codeStepPage is the sign-in page of the flows that take a personal code. A
// plain form: one field for the code, the request it answers kept hidden.
var codeStepPage = template.Must(template.New("code-step").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in with {{.Method}}</title>
<style>
body{font-family:system-ui,sans-serif;max-width:28rem;margin:3rem auto;padding:0 1rem;line-height:1.4}
label,input,button{display:block;width:100%;box-sizing:border-box}
input,button{font-size:1rem;padding:.5rem;margin:.5rem 0}
.error{color:#b00020}
</style>
</head>
<body>
<h1>Sign in with {{.Method}}</h1>
<p>Test sign-in: enter the personal code of a person in the identity list.</p>
{{if .Error}}<p class="error" role="alert">{{.Error}}</p>
{{end}}{{if .Request}}<form method="post" action="{{.Action}}">
<input type="hidden" name="request" value="{{.Request}}">
<label for="code">Personal code</label>
<input id="code" name="code" value="{{.Code}}" autocomplete="off" autofocus required>
<button type="submit">Continue</button>
</form>
{{end}}</body>
</html>
`))

type codeStepView struct {
	Method  string
	Action  string
	Request string // the pending request's id; empty when there is none to answer
	Code    string // the code entered, shown again with an error
	Error   string
}

// methodLabel names a flow that takes a personal code, for the page's heading.
func methodLabel(acrValues string) string {
	if acrValues == flowEIDScan {
		return "eID Scan"
	}

	return "Mobile ID"
}

func renderCodeStep(w http.ResponseWriter, status int, v codeStepView) {
	v.Action = CodeStepPath
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// The status and headers are already on the wire; a write failure here can
	// only be logged, and this is a test double on a loopback address.
	_ = codeStepPage.Execute(w, v)
}

// startCodeStep keeps a valid authorization request waiting and answers the
// sign-in page that asks for the person's code.
func (h *OAuthHandler) startCodeStep(w http.ResponseWriter, req utils.PendingAuthorization) {
	id := utils.GenerateAuthCode()
	h.store.StorePendingAuthorization(id, req)
	renderCodeStep(w, http.StatusOK, codeStepView{Method: methodLabel(req.ACRValues), Request: id})
}

// CodeStepHandler takes the code entered on the sign-in page. A code the list
// names answers the waiting request: back to the client with an authorization
// code for that person. A code it does not name shows the page again, so the
// code can be corrected.
func (h *OAuthHandler) CodeStepHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.sendError(w, "invalid_request", "Invalid form data")
		return
	}
	id, code := r.PostForm.Get("request"), r.PostForm.Get("code")

	pending, ok := h.store.GetPendingAuthorization(id)
	if !ok {
		renderCodeStep(w, http.StatusBadRequest, codeStepView{
			Method: "Mobile ID or eID Scan",
			Error:  "This sign-in has expired or is unknown. Start it again from the application.",
		})
		return
	}
	view := codeStepView{Method: methodLabel(pending.ACRValues), Request: id, Code: code}

	person, found := h.identities.Lookup(code)
	if !found {
		view.Error = "No one with this code is in the identity list."
		if code == "" {
			view.Error = "Enter a personal code."
		}
		log.Printf("Code step: the code entered is not in the identity list")
		renderCodeStep(w, http.StatusOK, view)
		return
	}

	// The request is answered once, even if the form is sent twice.
	if _, ok := h.store.TakePendingAuthorization(id); !ok {
		renderCodeStep(w, http.StatusBadRequest, codeStepView{
			Method: view.Method,
			Error:  "This sign-in has expired or is unknown. Start it again from the application.",
		})
		return
	}
	log.Printf("Code step: signed in as %s %s", person.GivenName, person.FamilyName)
	h.redirectWithCode(w, r, pending, &person)
}

// LogoutHandler is the identity provider's session-termination endpoint: the
// browser is sent here with the address to return to, and is sent back to it.
// This service keeps no sign-in session — every authorization request is
// answered afresh — so there is no session to end; access tokens already issued
// stay valid until they expire.
func (h *OAuthHandler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target := r.URL.Query().Get("redirect_uri")
	if target == "" {
		h.sendError(w, "invalid_request", "redirect_uri is required")
		return
	}
	back, err := url.Parse(target)
	if err != nil || !back.IsAbs() {
		h.sendError(w, "invalid_request", "Invalid redirect_uri")
		return
	}

	// As at the authorization endpoint, the mock sends the browser wherever the
	// test asks, on purpose.
	//nolint:gosec // G710: open redirect is the point of a mock logout endpoint
	http.Redirect(w, r, back.String(), http.StatusFound)
}
