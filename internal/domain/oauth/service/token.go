package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/rendau/ruto/internal/domain/oauth/model"
)

const (
	// AccessTokenPrefix tells OAuth access tokens from api keys (rtk_) on the
	// MCP port.
	AccessTokenPrefix  = "rto_"
	RefreshTokenPrefix = "rtr_"
	CodePrefix         = "rtc_"

	tokenRandomBytes = 32

	pkceVerifierMinLen = 43
	pkceVerifierMaxLen = 128
)

var tokenPrefixes = map[string]string{
	model.TokenKindCode:    CodePrefix,
	model.TokenKindAccess:  AccessTokenPrefix,
	model.TokenKindRefresh: RefreshTokenPrefix,
}

// GenerateToken returns a new token of the kind and its hash for storage.
func GenerateToken(kind string) (token, hash string, err error) {
	raw := make([]byte, tokenRandomBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("rand.Read: %w", err)
	}

	token = tokenPrefixes[kind] + hex.EncodeToString(raw)

	return token, HashToken(token), nil
}

// HashToken: tokens are high-entropy random strings, so a plain sha256 without
// salt is enough.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// VerifyPKCE checks a code_verifier against the S256 code_challenge (RFC 7636).
func VerifyPKCE(challenge, verifier string) bool {
	if len(verifier) < pkceVerifierMinLen || len(verifier) > pkceVerifierMaxLen {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(challenge)) == 1
}

// RedirectAllowed reports whether a redirect URI matches one of the allowed
// ones. An allowed URI without a path admits any path on its origin; a
// loopback one (localhost, 127.0.0.1, [::1]) admits any port, as native apps
// such as Claude Code listen on a random port (RFC 8252, 7.3).
func RedirectAllowed(allowed []string, uri string) bool {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Fragment != "" || u.User != nil {
		return false
	}

	for _, item := range allowed {
		a, err := url.Parse(item)
		if err != nil {
			continue
		}
		if !strings.EqualFold(a.Scheme, u.Scheme) || !strings.EqualFold(a.Hostname(), u.Hostname()) {
			continue
		}
		if !isLoopback(a.Hostname()) && a.Port() != u.Port() {
			continue
		}
		if path := strings.TrimSuffix(a.Path, "/"); path != "" && path != u.Path {
			continue
		}
		return true
	}

	return false
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
