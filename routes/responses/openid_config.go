package responses

import (
	"fmt"
	"strings"
)

func OpenIDConfigurationResponse(protocol, host, authEndpoint, tokenEndpoint, userinfoEndpoint string, scopes, acrValues []string) string {
	scopesJSON := `["` + strings.Join(scopes, `","`) + `"]`
	acrValuesJSON := `["` + strings.Join(acrValues, `","`) + `"]`

	buildURL := func(protocol, host, path string) string {
		return fmt.Sprintf("%s://%s%s", protocol, host, path)
	}

	issuer := buildURL(protocol, host, "")
	authURL := buildURL(protocol, host, authEndpoint)
	tokenURL := buildURL(protocol, host, tokenEndpoint)
	userinfoURL := buildURL(protocol, host, userinfoEndpoint)
	jwksURL := buildURL(protocol, host, "/.well-known/jwks.json")

	return fmt.Sprintf(`{
	"issuer": "%s",
	"authorization_endpoint": "%s",
	"token_endpoint": "%s",
	"userinfo_endpoint": "%s",
	"jwks_uri": "%s",
	"scopes_supported": %s,
	"response_types_supported": ["code"],
	"grant_types_supported": ["authorization_code"],
	"subject_types_supported": ["public"],
	"id_token_signing_alg_values_supported": ["RS256"],
	"token_endpoint_auth_methods_supported": ["client_secret_basic"],
	"acr_values_supported": %s
}`, issuer, authURL, tokenURL, userinfoURL, jwksURL, scopesJSON, acrValuesJSON)
}
