package requests

type TokenRequest struct {
	GrantType   string `json:"grant_type" form:"grant_type"`
	RedirectURI string `json:"redirect_uri" form:"redirect_uri"`
	Code        string `json:"code" form:"code"`
}
