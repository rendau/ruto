package authflow

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"sync/atomic"

	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
)

// cookieKey seals the refresh cookie. It comes from core with the gateway
// settings, the same on every pod, and may change without the handlers being
// rebuilt, so the units read it on every use.
var cookieKey atomic.Pointer[[]byte]

// SetCookieKey installs the key core sent; nil or a key of the wrong size
// means no refresh cookie is issued and none is read.
func SetCookieKey(key []byte) {
	if len(key) == 0 {
		cookieKey.Store(nil)
		slog.Warn("authflow: core sends no cookie key, login sessions will not be renewable")
		return
	}
	if len(key) != snapshotModel.CookieKeySize {
		cookieKey.Store(nil)
		slog.Error("authflow: cookie key has a wrong size, login sessions will not be renewable",
			"size", len(key), "want", snapshotModel.CookieKeySize)
		return
	}
	owned := append([]byte(nil), key...)
	cookieKey.Store(&owned)
}

func sealer() (cipher.AEAD, bool) {
	key := cookieKey.Load()
	if key == nil {
		return nil, false
	}
	block, err := aes.NewCipher(*key)
	if err != nil {
		return nil, false
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, false
	}
	return aead, true
}

// seal encrypts plain for the cookie called name (AES-256-GCM, the name as
// associated data, so a sealed value is only good in the cookie it was made
// for) and returns it base64url-encoded: nonce, then ciphertext and tag.
func seal(name string, plain []byte) (string, bool) {
	aead, ok := sealer()
	if !ok {
		return "", false
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err) // the system random source failing is not something to handle
	}
	sealed := aead.Seal(nonce, nonce, plain, []byte(name))
	return base64.RawURLEncoding.EncodeToString(sealed), true
}

// open reverses seal; anything that was not sealed for this cookie with the
// current key is rejected.
func open(name, value string) ([]byte, bool) {
	aead, ok := sealer()
	if !ok {
		return nil, false
	}
	sealed, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(sealed) < aead.NonceSize() {
		return nil, false
	}
	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ciphertext, []byte(name))
	if err != nil {
		return nil, false
	}
	return plain, true
}
