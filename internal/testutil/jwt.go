// Package testutil contains helpers shared across test packages.
package testutil

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// JWT returns an unsigned JWT-shaped token with the provided claims.
func JWT(t *testing.T, claims map[string]any) string {
	t.Helper()

	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("json.Marshal(header) error = %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(header) +
		"." +
		base64.RawURLEncoding.EncodeToString(payload) +
		".signature"
}

// JWTWithExpiration returns an unsigned JWT-shaped token with an exp claim.
func JWTWithExpiration(t *testing.T, exp time.Time) string {
	t.Helper()

	return JWT(t, map[string]any{"exp": exp.UTC().Unix()})
}

// ValidJWT returns a token with an expiration in the future.
func ValidJWT(t *testing.T) string {
	t.Helper()

	return JWTWithExpiration(t, time.Now().UTC().Add(24*time.Hour))
}

// ExpiredJWT returns a token with an expiration in the past.
func ExpiredJWT(t *testing.T) string {
	t.Helper()

	return JWTWithExpiration(t, time.Now().UTC().Add(-24*time.Hour))
}
