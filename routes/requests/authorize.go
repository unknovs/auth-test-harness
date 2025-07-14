package requests

type AuthorizeRequest struct {
	ResponseType string `json:"response_type" form:"response_type"`
	ClientID     string `json:"client_id" form:"client_id"`
	State        string `json:"state" form:"state"`
	RedirectURI  string `json:"redirect_uri" form:"redirect_uri"`
	Scope        string `json:"scope" form:"scope"`
	Prompt       string `json:"prompt" form:"prompt"`
	ACRValues    string `json:"acr_values" form:"acr_values"`
	UILocales    string `json:"ui_locales" form:"ui_locales"`
}
