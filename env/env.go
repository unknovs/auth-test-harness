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

	// User profile. SerialNumber is the identity code every profile reports
	// unless that profile overrides it below.
	SerialNumber string

	// Mobile ID user profile
	MobileGivenName    string
	MobileFamilyName   string
	MobileSerialNumber string

	// Smart Card user profile
	SCGivenName    string
	SCFamilyName   string
	SCSerialNumber string

	// eID Scan user profile (the phone reads the physical eID card). Falls back
	// to the Smart Card names when unset, since both are card-based.
	EIDScanGivenName    string
	EIDScanFamilyName   string
	EIDScanSerialNumber string
}

// Load loads environment variables with default values
func Load() *Config {
	config := &Config{
		Port:               getEnv("PORT", "8080"), // port binded inside the container. Shall be same as in Dockerfile
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
		MobileGivenName:    os.Getenv("MOBILE_GIVEN_NAME"),
		MobileFamilyName:   os.Getenv("MOBILE_FAMILY_NAME"),
		MobileSerialNumber: getEnv("MOBILE_SERIAL_NUMBER", os.Getenv("SERIAL_NUMBER")),

		// Smart Card user profile
		SCGivenName:    os.Getenv("SC_GIVEN_NAME"),
		SCFamilyName:   os.Getenv("SC_FAMILY_NAME"),
		SCSerialNumber: getEnv("SC_SERIAL_NUMBER", os.Getenv("SERIAL_NUMBER")),

		// eID Scan user profile
		EIDScanGivenName:    getEnv("EIDSCAN_GIVEN_NAME", os.Getenv("SC_GIVEN_NAME")),
		EIDScanFamilyName:   getEnv("EIDSCAN_FAMILY_NAME", os.Getenv("SC_FAMILY_NAME")),
		EIDScanSerialNumber: getEnv("EIDSCAN_SERIAL_NUMBER", os.Getenv("SERIAL_NUMBER")),
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
