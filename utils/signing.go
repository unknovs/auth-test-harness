package utils

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
)

// SigningKey is the RSA key this service signs its id_tokens with. It is
// generated when the service starts and published at /.well-known/jwks.json —
// a restart rotates it, which is what a real provider's key rotation looks like
// to a client, and a client that refetches the key set on an unknown kid keeps
// working across it. Nothing here is a credential of any deployment: the key
// lives exactly as long as the process.
type SigningKey struct {
	key *rsa.PrivateKey
	kid string
}

// NewSigningKey generates a 2048-bit RSA key and names it by a hash of its
// modulus, so two starts never publish two keys under one kid.
func NewSigningKey() (*SigningKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	sum := sha256.Sum256(key.N.Bytes())

	return &SigningKey{key: key, kid: hex.EncodeToString(sum[:8])}, nil
}

// KeyID is the kid the published key and every signed token carry.
func (k *SigningKey) KeyID() string { return k.kid }

// Public is the verifying half, for a test that checks a token the way a client would.
func (k *SigningKey) Public() *rsa.PublicKey { return &k.key.PublicKey }

// JWKS is the key set document a client fetches from the jwks_uri: the one
// public key, RS256, for signatures.
func (k *SigningKey) JWKS() []byte {
	pub := k.key.PublicKey
	doc := map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": k.kid,
			"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	}
	out, _ := json.Marshal(doc) // a map of strings cannot fail to marshal

	return out
}

// SignRS256 signs a claim set as a compact JWS (a JWT): the protected header
// names the algorithm and the kid, the payload is the claims, the signature is
// RSASSA-PKCS1-v1_5 over SHA-256 of the signing input — RS256 as every JWT
// library reads it.
func (k *SigningKey) SignRS256(claims map[string]any) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": k.kid})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// StableSubject derives a subject identifier that is the same on every login of
// the same profile and different between profiles: a hash of the parts that
// make the profile, 32 hex characters. A provider's `sub` is stable for a
// person, and a client that stores a credential under it — or checks that the
// id_token and userinfo name the same person — needs it to be.
func StableSubject(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil))[:32]
}

// StableObjectID derives a directory object identifier in the 8-4-4-4-12 shape
// directories issue them in, stable for the same parts.
func StableObjectID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	s := hex.EncodeToString(h.Sum(nil))

	return s[0:8] + "-" + s[8:12] + "-4" + s[13:16] + "-8" + s[17:20] + "-" + s[20:32]
}
