package responses

import (
	"fmt"
	"strings"
)

func buildURL(protocol, host, path string) string {
	return fmt.Sprintf("%s://%s%s", protocol, host, path)
}

// ServiceInfoResponse is the service information document. logoutEndpoint is
// listed only when it is served.
func ServiceInfoResponse(protocol, host, authEndpoint, tokenEndpoint, userinfoEndpoint, logoutEndpoint string, scopes, acrValues []string) string {
	scopesJSON := `["` + strings.Join(scopes, `","`) + `"]`
	acrValuesJSON := `["` + strings.Join(acrValues, `","`) + `"]`

	openidConfig := buildURL(protocol, host, "/.well-known/openid-configuration")
	authorizeURL := buildURL(protocol, host, authEndpoint)
	tokenURL := buildURL(protocol, host, tokenEndpoint)
	userinfoURL := buildURL(protocol, host, userinfoEndpoint)
	healthURL := buildURL(protocol, host, "/health")
	logout := ""
	if logoutEndpoint != "" {
		logout = fmt.Sprintf("\n\t\t\"logout\": \"%s\",", buildURL(protocol, host, logoutEndpoint))
	}

	return fmt.Sprintf(`{
	"service": "OAuth OIDC Mock Service",
	"version": "1.0.0",
	"openid_configuration": "%s",
	"endpoints": {
		"authorize": "%s",
		"token": "%s",
		"userinfo": "%s",%s
		"health": "%s"
	},
	"supported_scopes": %s,
	"supported_acr_values": %s
}`, openidConfig, authorizeURL, tokenURL, userinfoURL, logout, healthURL, scopesJSON, acrValuesJSON)
}
