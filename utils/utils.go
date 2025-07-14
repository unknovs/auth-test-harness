package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// GenerateAuthCode generates a random authorization code
func GenerateAuthCode() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateAccessToken generates a random access token
func GenerateAccessToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}

// GenerateSubject generates a random subject identifier
func GenerateSubject() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// InMemoryStore represents a simple in-memory storage for codes and tokens
type InMemoryStore struct {
	authCodes    map[string]AuthCodeData
	accessTokens map[string]TokenData
}

// AuthCodeData holds information about an authorization code
type AuthCodeData struct {
	Code        string
	ClientID    string
	RedirectURI string
	Scope       string
	ACRValues   string
	ExpiresAt   time.Time
}

// TokenData holds information about an access token
type TokenData struct {
	Token     string
	ACRValues string
	ExpiresAt time.Time
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		authCodes:    make(map[string]AuthCodeData),
		accessTokens: make(map[string]TokenData),
	}
}

// StoreAuthCode stores an authorization code
func (s *InMemoryStore) StoreAuthCode(code, clientID, redirectURI, scope, acrValues string) {
	s.authCodes[code] = AuthCodeData{
		Code:        code,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		Scope:       scope,
		ACRValues:   acrValues,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
}

// GetAuthCode retrieves and removes an authorization code
func (s *InMemoryStore) GetAuthCode(code string) (AuthCodeData, bool) {
	data, exists := s.authCodes[code]
	if exists {
		delete(s.authCodes, code) // One-time use
	}
	return data, exists && time.Now().Before(data.ExpiresAt)
}

// StoreAccessToken stores an access token
func (s *InMemoryStore) StoreAccessToken(token, acrValues string) {
	s.accessTokens[token] = TokenData{
		Token:     token,
		ACRValues: acrValues,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
}

// GetAccessToken retrieves an access token
func (s *InMemoryStore) GetAccessToken(token string) (TokenData, bool) {
	data, exists := s.accessTokens[token]
	return data, exists && time.Now().Before(data.ExpiresAt)
}

// CleanupExpired removes expired codes and tokens
func (s *InMemoryStore) CleanupExpired() {
	now := time.Now()

	for code, data := range s.authCodes {
		if now.After(data.ExpiresAt) {
			delete(s.authCodes, code)
		}
	}

	for token, data := range s.accessTokens {
		if now.After(data.ExpiresAt) {
			delete(s.accessTokens, token)
		}
	}
}
