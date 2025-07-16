package responses

import (
	"fmt"
	"strings"
)

func ServiceInfoResponse(protocol, host, authEndpoint, tokenEndpoint, userinfoEndpoint string, scopes, acrValues []string) string {
	scopesJSON := `["` + strings.Join(scopes, `","`) + `"]`
	acrValuesJSON := `["` + strings.Join(acrValues, `","`) + `"]`

	return fmt.Sprintf(`{
	"service": "OAuth OIDC Mock Service",
	"version": "1.0.0",
	"openid_configuration": "%s://%s/.well-known/openid_configuration",
	"endpoints": {
		"authorize": "%s://%s%s",
		"token": "%s://%s%s",
		"userinfo": "%s://%s%s",
		"health": "%s://%s/health"
	},
	"supported_scopes": %s,
	"supported_acr_values": %s
}`, protocol, host, protocol, host, authEndpoint, protocol, host, tokenEndpoint, protocol, host, userinfoEndpoint, protocol, host, scopesJSON, acrValuesJSON)
}
