package responses

type UserInfoResponse struct {
	Sub          string   `json:"sub"`
	Domain       string   `json:"domain"`
	ACR          string   `json:"acr"`
	AMR          []string `json:"amr"`
	GivenName    string   `json:"given_name"`
	FamilyName   string   `json:"family_name"`
	Name         string   `json:"name"`
	SerialNumber string   `json:"serial_number"`
	EIPS         string   `json:"eips"`
}
