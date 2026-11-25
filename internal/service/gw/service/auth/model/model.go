package model

import (
	"encoding/base64"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/infra/clientip"
)

type AuthRequest struct {
	Headers     http.Header
	QueryParams url.Values
	RemoteAddr  string

	token          authToken
	basic          authBasic
	apiKeyByHeader map[string]string
	remoteIP       netip.Addr
	forwardedFor   authIPs
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

func NewAuthRequest() *AuthRequest {
	return &AuthRequest{}
}

func (r *AuthRequest) SetHttpHeader(headers http.Header) {
	r.Headers = headers
}

func (r *AuthRequest) SetHttpQueryParams(qPars url.Values) {
	r.QueryParams = qPars
}

func (r *AuthRequest) SetRemoteAddr(remoteAddr string) {
	r.RemoteAddr = strings.TrimSpace(remoteAddr)
	r.remoteIP = clientip.ParseAddr(r.RemoteAddr)
}

func (r *AuthRequest) ExtractToken() (finalResult string) {
	if r.token.isSet {
		return r.token.value
	}

	defer func() {
		r.token.value = finalResult
		r.token.isSet = true
	}()

	return ParseBearer(r.findValue("Authorization", "auth_token"))
}

// ParseBearer gives the token of an Authorization value: "Bearer <token>" in
// any case and spacing, or a bare token; "" for anything else.
func ParseBearer(value string) string {
	parts := strings.Fields(value)
	switch {
	case len(parts) == 1:
		return parts[0]
	case len(parts) == 2 && strings.EqualFold(parts[0], "bearer"):
		return parts[1]
	default:
		return ""
	}
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

	user, pass, _ := ParseBasic(r.getHeaderValue("Authorization"))
	return user, pass
}

// ParseBasic decodes a "Basic <base64 user:password>" Authorization value.
func ParseBasic(value string) (string, string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "basic") {
		return "", "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", false
	}

	user, pass, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return "", "", false
	}
	return user, pass, true
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

// ExtractForwardedFor gives the X-Forwarded-For addresses in order, across all
// the header lines; an entry that is not an address is kept as an invalid
// Addr, so a reader walking from the right can tell the chain is broken.
func (r *AuthRequest) ExtractForwardedFor() []netip.Addr {
	if !r.forwardedFor.isSet {
		r.forwardedFor.value = clientip.ParseForwardedFor(r.getHeaderValues("X-Forwarded-For"))
		r.forwardedFor.isSet = true
	}
	return r.forwardedFor.value
}

// ExtractRealIP gives the X-Real-Ip address, invalid when there is none.
func (r *AuthRequest) ExtractRealIP() netip.Addr {
	return clientip.ParseAddr(r.getHeaderValue("X-Real-Ip"))
}

// ExtractCfConnectingIP gives the Cf-Connecting-Ip address, invalid when there
// is none.
func (r *AuthRequest) ExtractCfConnectingIP() netip.Addr {
	return clientip.ParseAddr(r.getHeaderValue("Cf-Connecting-Ip"))
}

func (r *AuthRequest) ExtractRemoteAddrIP() netip.Addr {
	return r.remoteIP
}

func (r *AuthRequest) findValue(headerKey, queryParamKey string) string {
	val := r.getHeaderValue(headerKey)
	if val == "" {
		val = r.getQueryParamValue(queryParamKey)
	}
	return val
}

// getHeaderValues gives every line of a header, also under the lowercase key
// gRPC metadata uses.
func (r *AuthRequest) getHeaderValues(key string) []string {
	if len(r.Headers) == 0 {
		return nil
	}
	return append(slices.Clip(r.Headers.Values(key)), r.Headers[strings.ToLower(key)]...)
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
