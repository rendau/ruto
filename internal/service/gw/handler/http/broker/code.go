package broker

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
)

// codeTTL bounds the time between the code and its exchange.
const codeTTL = 60 * time.Second

// loginCode is what the broker's authorization code carries to the token
// endpoint. It is not stored anywhere: the code is the record itself, sealed
// with the gateway's cookie key, so any pod opens it. PKCE and the short life
// guard it; with Redis a used code is also marked and refused again.
type loginCode struct {
	RedirectUri string `json:"r"`
	Challenge   string `json:"cc"`
	Nonce       string `json:"n,omitempty"`
	Scope       string `json:"sc,omitempty"`
	Sub         string `json:"s"`
	AuthTime    int64  `json:"at,omitempty"`
	Expires     int64  `json:"e"`
	Id          string `json:"j"`
}

// codeSealName binds a code to its provider and front: sealed for one, it
// does not open for another.
func (u *unit) codeSealName(client string) string {
	return "ruto-code_" + u.name + "_" + client
}

// issueCode fills in the expiry and the id and seals the code; false without
// a cookie key.
func (u *unit) issueCode(client string, code *loginCode) (string, bool) {
	code.Expires = time.Now().Add(codeTTL).Unix()
	code.Id = randomToken(16)
	raw, _ := json.Marshal(code)
	return authflow.Seal(u.codeSealName(client), raw)
}

// openCode opens a code of the front; false for anything not sealed for it
// or expired.
func (u *unit) openCode(client, value string) (*loginCode, bool) {
	raw, ok := authflow.Open(u.codeSealName(client), value)
	if !ok {
		return nil, false
	}
	code := new(loginCode)
	if json.Unmarshal(raw, code) != nil || time.Now().Unix() >= code.Expires {
		return nil, false
	}
	return code, true
}

// pkceMatches checks the verifier against the S256 challenge (RFC 7636 §4.6).
func pkceMatches(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func randomToken(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // the system random source failing is not something to handle
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
