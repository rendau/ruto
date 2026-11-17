package authflow

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var (
	// ErrNoSession — the request has no login session that can be used or
	// renewed: the person has to log in.
	ErrNoSession = errors.New("authflow: no login session")
	// ErrUnavailable — the provider cannot be reached or answered with
	// something the gateway does not accept.
	ErrUnavailable = errors.New("authflow: login provider is unavailable")
)

const (
	// sessionMargin: an access token with less time left is renewed before it
	// is handed on, so that what is built on it does not expire on the way.
	sessionMargin = 10 * time.Second
	// freshMinimum stands for half the life of an access token without iat.
	freshMinimum = time.Minute
)

// Session is the live login session of a request.
type Session struct {
	AccessToken string // the shared access token (A), verified
	Claims      jwtv5.MapClaims
	Expires     time.Time // of the access token
	IssuedAt    time.Time // of the access token; zero when it has no iat
	// SessionExpires is when the session can no longer be renewed; zero when
	// unknown.
	SessionExpires time.Time
	// AuthTime is when the person entered their credentials (unix seconds),
	// zero when the provider does not say.
	AuthTime int64
}

// Session returns the login session of the request: the access cookie when it
// is valid with sessionMargin to spare, otherwise the session renewed by the
// refresh cookie, the fresh cookies set on w. The error is ErrNoSession or
// ErrUnavailable, wrapped.
func (u *Unit) Session(w http.ResponseWriter, r *http.Request) (*Session, error) {
	return u.session(w, r, func(s *Session) bool {
		return time.Until(s.Expires) > sessionMargin
	})
}

// FreshSession is Session for a front renewing its own tokens, which live as
// long as the access token: the access token is renewed once less than half
// of its life is left, so that the front gets tokens it does not have to
// renew again at once.
func (u *Unit) FreshSession(w http.ResponseWriter, r *http.Request) (*Session, error) {
	return u.session(w, r, func(s *Session) bool {
		left := time.Until(s.Expires)
		if s.IssuedAt.IsZero() {
			return left > freshMinimum
		}
		return left > sessionMargin && left > s.Expires.Sub(s.IssuedAt)/2
	})
}

// session returns the access cookie's session when keep accepts it,
// otherwise the session renewed by the refresh cookie.
func (u *Unit) session(w http.ResponseWriter, r *http.Request, keep func(*Session) bool) (*Session, error) {
	refresh := u.readSession(r)

	var current *Session
	if cookie, err := r.Cookie(u.cookieName); err == nil && cookie.Value != "" {
		current, _ = u.newSession(cookie.Value, refresh)
	}
	if current != nil && keep(current) {
		return current, nil
	}

	tokens, err := u.renew(w, r, refresh)
	if err != nil {
		// A token about to expire still serves when it cannot be renewed.
		if current != nil && errors.Is(err, ErrNoSession) && time.Now().Before(current.Expires) {
			return current, nil
		}
		return nil, err
	}
	renewed := refresh
	renewed.IdToken = tokens.IdToken
	session, err := u.newSession(tokens.AccessToken, renewed)
	if err != nil {
		return nil, fmt.Errorf("%w: renewed access token: %w", ErrUnavailable, err)
	}
	return session, nil
}

func (u *Unit) newSession(accessToken string, refresh sessionCookie) (*Session, error) {
	claims, err := u.parseAccessToken(accessToken)
	if err != nil {
		return nil, err
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return nil, errors.New("access token has no exp")
	}
	session := &Session{AccessToken: accessToken, Claims: claims, Expires: exp.Time}
	if iat, iatErr := claims.GetIssuedAt(); iatErr == nil && iat != nil {
		session.IssuedAt = iat.Time
	}
	if refresh.Expires > 0 {
		session.SessionExpires = time.Unix(refresh.Expires, 0)
	}
	session.AuthTime = authTime(claims)
	if session.AuthTime == 0 && refresh.IdToken != "" {
		// verified at login; read as is, a cookie its owner broke only hurts its owner
		if idt, _, parseErr := jwtv5.NewParser().ParseUnverified(refresh.IdToken, jwtv5.MapClaims{}); parseErr == nil {
			session.AuthTime = authTime(idt.Claims.(jwtv5.MapClaims))
		}
	}
	return session, nil
}

func authTime(claims jwtv5.MapClaims) int64 {
	if v, ok := claims["auth_time"].(float64); ok {
		return int64(v)
	}
	return 0
}

// LoginPath is the gateway path that logs the person in at the provider and
// sends the browser to returnPath (a path on the gateway) afterwards; a
// failed login comes back to it with auth_error. prompt is passed on as it is.
func (u *Unit) LoginPath(returnPath, prompt string) string {
	query := url.Values{"redirect": {returnPath}}
	if prompt != "" {
		query.Set("prompt", prompt)
	}
	return u.basePath + "/login?" + query.Encode()
}

// ParseProviderToken checks a token of this provider: the signature by a key
// of the provider, the issuer and the expiry; the claims are returned.
func (u *Unit) ParseProviderToken(token string) (jwtv5.MapClaims, error) {
	return u.parseAccessToken(token)
}

// Issuer is the provider's public address.
func (u *Unit) Issuer() string {
	return u.provider.Issuer
}

// PublicUrl is the gateway address the browser sees for this provider.
func (u *Unit) PublicUrl() string {
	return u.publicUrl
}

// SetCookie sets an httpOnly cookie with the attributes of the unit's own
// cookies (SameSite=Lax, Secure on an https gateway).
func (u *Unit) SetCookie(w http.ResponseWriter, name, value, path string, maxAge int) {
	u.setCookie(w, name, value, path, maxAge)
}

// ClearCookie drops a cookie set by SetCookie.
func (u *Unit) ClearCookie(w http.ResponseWriter, name, path string) {
	u.clearCookie(w, name, path)
}

// Seal encrypts plain with the cookie key for the purpose named by name (see
// seal); false without a key.
func Seal(name string, plain []byte) (string, bool) {
	return seal(name, plain)
}

// Open reverses Seal.
func Open(name, value string) ([]byte, bool) {
	return open(name, value)
}
