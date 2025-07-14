package responses

import (
	"fmt"
	"strings"
)

func ServiceInfoResponse(host, authEndpoint, tokenEndpoint, userinfoEndpoint string, scopes, acrValues []string) string {
	scopesJSON := `["` + strings.Join(scopes, `","`) + `"]`
	acrValuesJSON := `["` + strings.Join(acrValues, `","`) + `"]`

	return fmt.Sprintf(`{
	"service": "OAuth OIDC Mock Service",
	"version": "1.0.0",
	"openid_configuration": "http://%s/.well-known/openid_configuration",
	"endpoints": {
		"authorize": "%s",
		"token": "%s",
		"userinfo": "%s",
		"health": "/health"
	},
	"supported_scopes": %s,
	"supported_acr_values": %s
}`, host, authEndpoint, tokenEndpoint, userinfoEndpoint, scopesJSON, acrValuesJSON)
}
