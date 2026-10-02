package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"

	"github.com/unknovs/auth-test-harness/identities"
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

// InMemoryStore represents a simple in-memory storage for codes and tokens. The
// HTTP server answers requests concurrently and the clean-up ticker sweeps the
// same maps meanwhile, so every access holds the lock.
type InMemoryStore struct {
	mu           sync.Mutex
	authCodes    map[string]AuthCodeData
	accessTokens map[string]TokenData
	pending      map[string]PendingAuthorization
}

// AuthCodeData holds information about an authorization code. Nonce is the value
// the client sent with its authorization request, carried into the id_token the
// code is exchanged for so the client can bind the token to that request.
type AuthCodeData struct {
	Code        string
	ClientID    string
	RedirectURI string
	Scope       string
	ACRValues   string
	Nonce       string
	// Person is the person whose code was entered at the code step; nil when the
	// flow answers as its configured profile.
	Person    *identities.Person
	ExpiresAt time.Time
}

// TokenData holds information about an access token
type TokenData struct {
	Token     string
	ACRValues string
	Person    *identities.Person
	ExpiresAt time.Time
}

// PendingAuthorization is an authorization request waiting at the code step for
// the person's code: everything needed to answer the client once it is entered.
type PendingAuthorization struct {
	ClientID    string
	RedirectURI string
	Scope       string
	ACRValues   string
	State       string
	Nonce       string
	ExpiresAt   time.Time
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		authCodes:    make(map[string]AuthCodeData),
		accessTokens: make(map[string]TokenData),
		pending:      make(map[string]PendingAuthorization),
	}
}

// StoreAuthCode stores an authorization code
func (s *InMemoryStore) StoreAuthCode(code, clientID, redirectURI, scope, acrValues, nonce string, person *identities.Person) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authCodes[code] = AuthCodeData{
		Code:        code,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		Scope:       scope,
		ACRValues:   acrValues,
		Nonce:       nonce,
		Person:      person,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
}

// GetAuthCode retrieves and removes an authorization code
func (s *InMemoryStore) GetAuthCode(code string) (AuthCodeData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, exists := s.authCodes[code]
	if exists {
		delete(s.authCodes, code) // One-time use
	}
	return data, exists && time.Now().Before(data.ExpiresAt)
}

// StoreAccessToken stores an access token
func (s *InMemoryStore) StoreAccessToken(token, acrValues string, person *identities.Person) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessTokens[token] = TokenData{
		Token:     token,
		ACRValues: acrValues,
		Person:    person,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
}

// GetAccessToken retrieves an access token
func (s *InMemoryStore) GetAccessToken(token string) (TokenData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, exists := s.accessTokens[token]
	return data, exists && time.Now().Before(data.ExpiresAt)
}

// StorePendingAuthorization keeps an authorization request at the code step under
// id for as long as an authorization code lives.
func (s *InMemoryStore) StorePendingAuthorization(id string, p PendingAuthorization) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.ExpiresAt = time.Now().Add(10 * time.Minute)
	s.pending[id] = p
}

// GetPendingAuthorization reads a request waiting at the code step, leaving it in
// place: a wrong code may be corrected and entered again.
func (s *InMemoryStore) GetPendingAuthorization(id string) (PendingAuthorization, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, exists := s.pending[id]
	return data, exists && time.Now().Before(data.ExpiresAt)
}

// TakePendingAuthorization reads and removes a request waiting at the code step,
// so one request is answered once.
func (s *InMemoryStore) TakePendingAuthorization(id string) (PendingAuthorization, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, exists := s.pending[id]
	if exists {
		delete(s.pending, id)
	}
	return data, exists && time.Now().Before(data.ExpiresAt)
}

// CleanupExpired removes expired codes and tokens
func (s *InMemoryStore) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
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

	for id, data := range s.pending {
		if now.After(data.ExpiresAt) {
			delete(s.pending, id)
		}
	}
}
