package env

import (
	"os"
	"strings"
)

// Config holds all environment variables
type Config struct {
	Port               string
	Host               string
	Protocol           string
	BindAddress        string
	BasicAuthValue     string
	TokenExpirationMin int

	// Endpoint configurations
	AuthorizationEndpoint string
	TokenEndpoint         string
	UserInfoEndpoint      string

	// Supported values
	ScopesSupported    []string
	ACRValuesSupported []string

	// User profile
	SerialNumber string

	// Mobile ID user profile
	MobileGivenName  string
	MobileFamilyName string

	// Smart Card user profile
	SCGivenName  string
	SCFamilyName string
}

// Load loads environment variables with default values
func Load() *Config {
	config := &Config{
		Port:               getEnv("PORT", "8080"),
		Host:               getEnv("HOST", "localhost:8080"),
		Protocol:           getEnv("PROTOCOL", "http"),
		BindAddress:        getEnv("BIND_ADDRESS", "0.0.0.0"),
		BasicAuthValue:     getEnv("BASIC_AUTH_VALUE", "dGVzdDp0ZXN0"), // base64 encoded "test:test"
		TokenExpirationMin: 10,                                         // hardcoded as per requirements

		// Endpoint configurations
		AuthorizationEndpoint: os.Getenv("AUTHORIZATION_ENDPOINT"),
		TokenEndpoint:         os.Getenv("TOKEN_ENDPOINT"),
		UserInfoEndpoint:      os.Getenv("USERINFO_ENDPOINT"),

		// Supported values
		ScopesSupported:    getEnvArray("SCOPES_SUPPORTED", ""),
		ACRValuesSupported: getEnvArray("ACR_VALUES_SUPPORTED", ""),

		// User profile
		SerialNumber: os.Getenv("SERIAL_NUMBER"),

		// Mobile ID user profile
		MobileGivenName:  os.Getenv("MOBILE_GIVEN_NAME"),
		MobileFamilyName: os.Getenv("MOBILE_FAMILY_NAME"),

		// Smart Card user profile
		SCGivenName:  os.Getenv("SC_GIVEN_NAME"),
		SCFamilyName: os.Getenv("SC_FAMILY_NAME"),
	}
	return config
}

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvArray gets an environment variable as comma-separated array with fallback
func getEnvArray(key, fallback string) []string {
	value := getEnv(key, fallback)
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}
