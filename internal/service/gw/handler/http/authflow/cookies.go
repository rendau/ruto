package authflow

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rendau/ruto/internal/constant"
)

const (
	// loginCookieMaxAge bounds a login attempt: the time between leaving for
	// the provider and coming back with the code.
	loginCookieMaxAge = 10 * 60
	// defaultSessionMaxAge is the cookie lifetime when the provider does not
	// say how long its refresh token lives.
	defaultSessionMaxAge = 24 * 60 * 60
	// cookieSizeWarn is the common browser limit on one cookie.
	cookieSizeWarn = 4096
	// hostPrefix makes a browser accept the cookie only from this very host
	// over https: a sibling subdomain cannot plant one.
	hostPrefix = "__Host-"
)

func (u *unit) unitSlug() string {
	slug := u.provider.Name
	if u.cookiePath != "/" {
		slug += "_" + constant.LoginUnitSlug(u.cookiePath)
	}
	return slug
}

func (u *unit) refreshCookieName() string {
	return constant.JwtRefreshCookiePrefix + u.unitSlug()
}

// loginCookieName is the cookie of a login in progress (state, PKCE verifier,
// return address). With https it gets the __Host- prefix, which requires
// Path=/ and Secure.
func (u *unit) loginCookieName() string {
	name := constant.JwtLoginCookiePrefix + u.unitSlug()
	if u.secure {
		return hostPrefix + name
	}
	return name
}

func (u *unit) setCookie(w http.ResponseWriter, name, value, path string, maxAge int) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   u.secure,
		SameSite: http.SameSiteLaxMode,
	}
	if size := len(cookie.String()); size > cookieSizeWarn {
		slog.Warn("authflow: cookie is over the usual browser limit, the browser may drop it",
			"unit", u.name, "cookie", name, "size", size)
	}
	http.SetCookie(w, cookie)
}

func (u *unit) clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   u.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// loginState is what the login cookie carries from /login to /callback.
type loginState struct {
	state    string
	verifier string
	nonce    string // must come back inside the id token
	redirect string
}

// encode joins the four values with "." — none of them contains one: the
// first three are base64url, the address is base64url-encoded too.
func (s *loginState) encode() string {
	return s.state + "." + s.verifier + "." + s.nonce + "." + base64.RawURLEncoding.EncodeToString([]byte(s.redirect))
}

func decodeLoginState(raw string) (*loginState, bool) {
	parts := strings.Split(raw, ".")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, false
	}
	redirect, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, false
	}
	return &loginState{state: parts[0], verifier: parts[1], nonce: parts[2], redirect: string(redirect)}, true
}

func randomToken(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // the system random source failing is not something to handle
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// pkceChallenge is the S256 transform of RFC 7636.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
