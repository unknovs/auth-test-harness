package responses

import (
	"fmt"
	"strings"
)

func OpenIDConfigurationResponse(protocol, host, authEndpoint, tokenEndpoint, userinfoEndpoint string, scopes, acrValues []string) string {
	scopesJSON := `["` + strings.Join(scopes, `","`) + `"]`
	acrValuesJSON := `["` + strings.Join(acrValues, `","`) + `"]`

	return fmt.Sprintf(`{
	"issuer": "%s://%s",
	"authorization_endpoint": "%s://%s%s",
	"token_endpoint": "%s://%s%s",
	"userinfo_endpoint": "%s://%s%s",
	"jwks_uri": "%s://%s/.well-known/jwks.json",
	"scopes_supported": %s,
	"response_types_supported": ["code"],
	"grant_types_supported": ["authorization_code"],
	"subject_types_supported": ["public"],
	"id_token_signing_alg_values_supported": ["RS256"],
	"token_endpoint_auth_methods_supported": ["client_secret_basic"],
	"acr_values_supported": %s
}`, protocol, host, protocol, host, authEndpoint, protocol, host, tokenEndpoint, protocol, host, userinfoEndpoint, protocol, host, scopesJSON, acrValuesJSON)
}
