// Package rutokey holds the key the gateway signs its own tokens with, the
// gateway being an OpenID Connect broker for the fronts: the private key core
// derived and sent with the gateway settings, its key id and the public JWK
// set fronts check those tokens by. It knows nothing about providers or apps.
package rutokey

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/golang-jwt/jwt/v5"

	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
)

// Alg is the one algorithm ruto signs with.
const Alg = "ES256"

// ErrNoKey — core sent no signing key: the gateway issues no tokens.
var ErrNoKey = errors.New("rutokey: no signing key")

var instance = New()

func Ins() *Service {
	return instance
}

type key struct {
	private *ecdsa.PrivateKey
	kid     string
	jwks    []byte
}

type Service struct {
	current atomic.Pointer[key] // nil — no key
}

func New() *Service {
	return &Service{}
}

// Set installs the key core sent; nil or a key that does not parse leaves the
// gateway without one, and every pod with the same raw key gets the same kid.
func (s *Service) Set(raw []byte) {
	if len(raw) == 0 {
		s.current.Store(nil)
		return
	}
	k, err := newKey(raw)
	if err != nil {
		slog.Error("rutokey: signing key ignored", "error", err)
		s.current.Store(nil)
		return
	}
	s.current.Store(k)
}

// Kids returns the key ids of the gateway's own tokens; empty without a key.
func (s *Service) Kids() []string {
	k := s.current.Load()
	if k == nil {
		return nil
	}
	return []string{k.kid}
}

// JWKS returns the public JWK set (RFC 7517); ok is false without a key.
func (s *Service) JWKS() ([]byte, bool) {
	k := s.current.Load()
	if k == nil {
		return nil, false
	}
	return k.jwks, true
}

// Sign signs claims with the current key, the key id in the header.
func (s *Service) Sign(claims jwt.Claims) (string, error) {
	k := s.current.Load()
	if k == nil {
		return "", ErrNoKey
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = k.kid
	signed, err := token.SignedString(k.private)
	if err != nil {
		return "", fmt.Errorf("token.SignedString: %w", err)
	}
	return signed, nil
}

type jwkItem struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

func newKey(raw []byte) (*key, error) {
	if len(raw) != snapshotModel.SigningKeySize {
		return nil, fmt.Errorf("size %d, want %d", len(raw), snapshotModel.SigningKeySize)
	}
	private, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), raw)
	if err != nil {
		return nil, fmt.Errorf("ecdsa.ParseRawPrivateKey: %w", err)
	}
	point, err := private.PublicKey.Bytes() // 0x04 || x || y
	if err != nil {
		return nil, fmt.Errorf("PublicKey.Bytes: %w", err)
	}
	size := (len(point) - 1) / 2
	item := jwkItem{
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(point[1 : 1+size]),
		Y:   base64.RawURLEncoding.EncodeToString(point[1+size:]),
		Use: "sig",
		Alg: Alg,
	}
	item.Kid = thumbprint(&item)

	jwks, err := json.Marshal(map[string][]jwkItem{"keys": {item}})
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}
	return &key{private: private, kid: item.Kid, jwks: jwks}, nil
}

// thumbprint is the JWK thumbprint (RFC 7638): the hash of the required
// members in lexical order, so the kid follows from the key alone.
func thumbprint(item *jwkItem) string {
	canonical := `{"crv":"` + item.Crv + `","kty":"` + item.Kty + `","x":"` + item.X + `","y":"` + item.Y + `"}`
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
