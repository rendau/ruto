package model

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/samber/lo"
)

type AuthRequest struct {
	Headers     http.Header
	QueryParams url.Values
	RemoteAddr  string

	token          authToken
	basic          authBasic
	apiKeyByHeader map[string]string
	cookies        authCookies
	cookieToken    string
	rutoBearer     bool
	remoteIP       netip.Addr
	ipAddrs        authIPs

	ctx          context.Context
	respCookies  []*http.Cookie
	stripCookies []string
	challenge    string
}

type authToken struct {
	value string
	isSet bool
}

type authBasic struct {
	username string
	password string
	isSet    bool
}

type authIPs struct {
	value []netip.Addr
	isSet bool
}

type authCookies struct {
	value map[string]string
	isSet bool
}

func NewAuthRequest() *AuthRequest {
	return &AuthRequest{}
}

func (r *AuthRequest) SetHttpHeader(headers http.Header) {
	r.Headers = headers
}

func (r *AuthRequest) SetHttpQueryParams(qPars url.Values) {
	r.QueryParams = qPars
}

// SetContext is the context of the request: an authorizer that calls a
// provider (the app token exchange) stops when the client goes away.
func (r *AuthRequest) SetContext(ctx context.Context) {
	r.ctx = ctx
}

func (r *AuthRequest) Context() context.Context {
	if r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

func (r *AuthRequest) SetRemoteAddr(remoteAddr string) {
	r.RemoteAddr = strings.TrimSpace(remoteAddr)
	r.remoteIP, _ = parseRemoteAddrIP(r.RemoteAddr)
}

func (r *AuthRequest) ExtractToken() (finalResult string) {
	if r.token.isSet {
		return r.token.value
	}

	defer func() {
		r.token.value = finalResult
		r.token.isSet = true
	}()

	value := r.findValue("Authorization", "auth_token")
	if value == "" {
		return ""
	}

	parts := strings.Fields(value)
	var token string
	switch {
	case len(parts) == 1:
		token = parts[0]
	case len(parts) == 2 && strings.EqualFold(parts[0], "bearer"):
		token = parts[1]
	default:
		return ""
	}

	// A token of the gateway itself is what an OpenID Connect SDK of a front
	// holds; it proves nothing to the gateway, the session is in the cookies.
	if IsRutoToken(token) {
		r.rutoBearer = true
		return ""
	}

	return token
}

// RutoBearer reports that the request presented a token of the gateway itself
// (see ExtractToken): the transport replaces it with the token the request
// was let in by.
func (r *AuthRequest) RutoBearer() bool {
	r.ExtractToken()
	return r.rutoBearer
}

// ExtractCookie returns the value of the named cookie from the Cookie header,
// the first one when the name repeats. The header is parsed once per request
// and only when some method asks for a cookie, so requests that carry a bearer
// token never pay for it.
func (r *AuthRequest) ExtractCookie(name string) string {
	if !r.cookies.isSet {
		r.cookies.value = parseCookies(r.getHeaderValues("Cookie"))
		r.cookies.isSet = true
	}
	return r.cookies.value[name]
}

// MarkCookieToken records the token a jwt method accepted from a cookie, so the
// transport can hand it to the backend as Authorization as well.
func (r *AuthRequest) MarkCookieToken(token string) {
	r.cookieToken = token
}

func (r *AuthRequest) CookieToken() string {
	return r.cookieToken
}

// SetResponseCookie records a cookie for the response, whatever the outcome
// of the check: an app token the gateway got is worth keeping. A cookie with
// the same name and path replaces the recorded one.
func (r *AuthRequest) SetResponseCookie(cookie *http.Cookie) {
	for i, c := range r.respCookies {
		if c.Name == cookie.Name && c.Path == cookie.Path {
			r.respCookies[i] = cookie
			return
		}
	}
	r.respCookies = append(r.respCookies, cookie)
}

func (r *AuthRequest) ResponseCookies() []*http.Cookie {
	return r.respCookies
}

// StripCookie records a cookie the backend must not get when the request is
// let in: the shared token, once the backend gets an app token instead.
func (r *AuthRequest) StripCookie(name string) {
	if !slices.Contains(r.stripCookies, name) {
		r.stripCookies = append(r.stripCookies, name)
	}
}

func (r *AuthRequest) StrippedCookies() []string {
	return r.stripCookies
}

// SetChallenge records the WWW-Authenticate value of a 401 answer.
func (r *AuthRequest) SetChallenge(value string) {
	r.challenge = value
}

func (r *AuthRequest) Challenge() string {
	return r.challenge
}

func (r *AuthRequest) ExtractBasic() (finalUsername string, finalPassword string) {
	if r.basic.isSet {
		return r.basic.username, r.basic.password
	}

	defer func() {
		r.basic.username = finalUsername
		r.basic.password = finalPassword
		r.basic.isSet = true
	}()

	headerValue := r.getHeaderValue("Authorization")
	if headerValue == "" {
		return "", ""
	}

	parts := strings.Fields(headerValue)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "basic") {
		return "", ""
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ""
	}

	user, pass, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return "", ""
	}

	return user, pass
}

func (r *AuthRequest) ExtractAPIKey(header string) (finalResult string) {
	if r.apiKeyByHeader == nil {
		r.apiKeyByHeader = make(map[string]string, 1)
	} else if result, ok := r.apiKeyByHeader[header]; ok {
		return result
	}

	defer func() {
		r.apiKeyByHeader[header] = finalResult
	}()

	value := r.findValue(header, header)

	return value
}

func (r *AuthRequest) ExtractIPAddrs() (finalResult []netip.Addr) {
	if r.ipAddrs.isSet {
		return r.ipAddrs.value
	}

	defer func() {
		r.ipAddrs.value = finalResult
		r.ipAddrs.isSet = true
	}()

	result := make([]netip.Addr, 0, 10)
	seen := make(map[netip.Addr]struct{}, 10)

	appendParsedIP := func(ip netip.Addr) {
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		result = append(result, ip)
	}

	appendIP := func(raw string) {
		if raw = strings.TrimSpace(raw); raw == "" {
			return
		}
		ip, err := netip.ParseAddr(raw)
		if err != nil {
			return
		}
		appendParsedIP(ip)
	}

	appendIPList := func(raw string) {
		if raw == "" {
			return
		}
		for _, x := range strings.Split(raw, ",") {
			appendIP(x)
		}
	}
	appendIPList(r.getHeaderValue("X-Forwarded-For"))
	appendIP(r.getHeaderValue("X-Real-Ip"))
	appendIP(r.getHeaderValue("Cf-Connecting-Ip"))
	appendIP(r.getHeaderValue("True-Client-Ip"))
	appendIP(r.getHeaderValue("X-Client-Ip"))
	appendIP(r.getHeaderValue("X-Cluster-Client-Ip"))

	return result
}

func (r *AuthRequest) ExtractRemoteAddrIP() netip.Addr {
	return r.remoteIP
}

func parseRemoteAddrIP(raw string) (netip.Addr, bool) {
	if raw == "" {
		return netip.Addr{}, false
	}

	if ip, err := netip.ParseAddr(raw); err == nil {
		return ip, true
	}

	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		return netip.Addr{}, false
	}

	ip, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}, false
	}

	return ip, true
}

func (r *AuthRequest) findValue(headerKey, queryParamKey string) string {
	val := r.getHeaderValue(headerKey)
	if val == "" {
		val = r.getQueryParamValue(queryParamKey)
	}
	return val
}

func (r *AuthRequest) getHeaderValue(key string) string {
	if key == "" || r.Headers == nil || len(r.Headers) == 0 {
		return ""
	}

	val := strings.TrimSpace(r.Headers.Get(key))
	if val == "" {
		val = strings.TrimSpace(lo.FirstOrEmpty(r.Headers[strings.ToLower(key)]))
	}

	return val
}

func (r *AuthRequest) getHeaderValues(key string) []string {
	if key == "" || len(r.Headers) == 0 {
		return nil
	}
	vals := r.Headers.Values(key)
	if len(vals) == 0 {
		vals = r.Headers[strings.ToLower(key)]
	}
	return vals
}

// parseCookies is lenient, like (*http.Request).Cookie: a malformed pair is
// skipped instead of discarding the whole header, so a stray third-party
// cookie cannot hide the gateway's own one.
func parseCookies(lines []string) map[string]string {
	result := make(map[string]string, 4)
	for _, line := range lines {
		for pair := range strings.SplitSeq(line, ";") {
			name, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
			if !ok || name == "" {
				continue
			}
			if _, seen := result[name]; seen {
				continue
			}
			if len(value) > 1 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			result[name] = value
		}
	}
	return result
}

func (r *AuthRequest) getQueryParamValue(key string) string {
	if key == "" || r.QueryParams == nil || len(r.QueryParams) == 0 {
		return ""
	}

	val := strings.TrimSpace(r.QueryParams.Get(key))
	if val == "" {
		val = strings.TrimSpace(r.QueryParams.Get(strings.ToLower(key)))
	}

	return val
}
