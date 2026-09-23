package responses

// UserInfoResponse is the profile the userinfo endpoint answers. A directory
// profile carries no identity code and no acr — a work account has neither —
// so both are omitted rather than sent empty.
type UserInfoResponse struct {
	Sub          string   `json:"sub"`
	Domain       string   `json:"domain"`
	ACR          string   `json:"acr,omitempty"`
	AMR          []string `json:"amr"`
	GivenName    string   `json:"given_name"`
	FamilyName   string   `json:"family_name"`
	Name         string   `json:"name"`
	SerialNumber string   `json:"serial_number,omitempty"`
	EIPS         string   `json:"eips"`
}
