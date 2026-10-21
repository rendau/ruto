package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rendau/ruto/internal/domain/oauth/model"
)

const (
	// clientIdPrefix marks client ids issued by ruto.
	clientIdPrefix = "rtcl_"

	clientNonceBytes = 9
)

// ClientCodec issues stateless client ids for dynamic client registration:
// the client metadata is packed into the id and signed, so registering (an
// open, unauthenticated endpoint) stores nothing.
type ClientCodec struct {
	key []byte
}

type clientPayload struct {
	// Nonce makes every registration a distinct client, even with the same
	// metadata: a token is bound to the registration that obtained it.
	Nonce        string   `json:"i"`
	Name         string   `json:"n"`
	RedirectUris []string `json:"r"`
	AuthMethod   string   `json:"m"`
}

// NewClientCodec derives its signing key from secret (the admin JWT secret),
// so the ids are tied to the installation and the secret itself is not reused
// as is.
func NewClientCodec(secret string) *ClientCodec {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("ruto oauth client"))
	return &ClientCodec{key: mac.Sum(nil)}
}

func (c *ClientCodec) Encode(client *model.Client) (string, error) {
	nonce := make([]byte, clientNonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}

	raw, err := json.Marshal(&clientPayload{
		Nonce:        base64.RawURLEncoding.EncodeToString(nonce),
		Name:         client.Name,
		RedirectUris: client.RedirectUris,
		AuthMethod:   client.AuthMethod,
	})
	if err != nil {
		return "", err
	}

	payload := base64.RawURLEncoding.EncodeToString(raw)

	return clientIdPrefix + payload + "." + c.sign("id:"+payload), nil
}

func (c *ClientCodec) Decode(clientId string) (*model.Client, bool) {
	rest, ok := strings.CutPrefix(clientId, clientIdPrefix)
	if !ok {
		return nil, false
	}
	payload, sig, ok := strings.Cut(rest, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(c.sign("id:"+payload))) {
		return nil, false
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, false
	}
	p := &clientPayload{}
	if err = json.Unmarshal(raw, p); err != nil {
		return nil, false
	}

	return &model.Client{
		Name:         p.Name,
		RedirectUris: p.RedirectUris,
		AuthMethod:   p.AuthMethod,
	}, true
}

// Secret is the client secret of a confidential client, derived from its id.
func (c *ClientCodec) Secret(clientId string) string {
	return c.sign("secret:" + clientId)
}

func (c *ClientCodec) VerifySecret(clientId, secret string) bool {
	return secret != "" && hmac.Equal([]byte(secret), []byte(c.Secret(clientId)))
}

func (c *ClientCodec) sign(value string) string {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
