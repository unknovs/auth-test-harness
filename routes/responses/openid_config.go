package responses

import (
	"fmt"
	"strings"
)

func OpenIDConfigurationResponse(host, authEndpoint, tokenEndpoint, userinfoEndpoint string, scopes, acrValues []string) string {
	scopesJSON := `["` + strings.Join(scopes, `","`) + `"]`
	acrValuesJSON := `["` + strings.Join(acrValues, `","`) + `"]`

	return fmt.Sprintf(`{
	"issuer": "http://%s",
	"authorization_endpoint": "http://%s%s",
	"token_endpoint": "http://%s%s",
	"userinfo_endpoint": "http://%s%s",
	"jwks_uri": "http://%s/.well-known/jwks.json",
	"scopes_supported": %s,
	"response_types_supported": ["code"],
	"grant_types_supported": ["authorization_code"],
	"subject_types_supported": ["public"],
	"id_token_signing_alg_values_supported": ["RS256"],
	"token_endpoint_auth_methods_supported": ["client_secret_basic"],
	"acr_values_supported": %s
}`, host, host, authEndpoint, host, tokenEndpoint, host, userinfoEndpoint, host, scopesJSON, acrValuesJSON)
}
