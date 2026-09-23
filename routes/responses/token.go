package responses

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	// IDToken is the signed statement of who logged in, as every OpenID
	// provider issues one: verifiable against /.well-known/jwks.json.
	IDToken string `json:"id_token"`
}
