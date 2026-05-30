package model

import (
	"encoding/base64"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/samber/lo"
)

type AuthRequest struct {
	Headers     map[string]string
	QueryParams map[string]string
	RemoteAddr  string

	token          *string
	basic          *authBasic
	apiKeyByHeader map[string]string
	ips            []string
}

type authBasic struct {
	username string
	password string
}

func NewAuthRequest() *AuthRequest {
	return &AuthRequest{}
}

func (r *AuthRequest) SetHttpHeader(headers http.Header) {
	r.Headers = make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			r.Headers[strings.ToLower(key)] = values[0]
		}
	}
}

func (r *AuthRequest) SetHttpQueryParams(qPars url.Values) {
	r.QueryParams = make(map[string]string, len(qPars))
	for key, values := range qPars {
		if len(values) > 0 {
			r.QueryParams[strings.ToLower(key)] = values[0]
		}
	}
}

func (r *AuthRequest) SetRemoteAddr(remoteAddr string) {
	r.RemoteAddr = strings.TrimSpace(remoteAddr)
}

func (r *AuthRequest) ExtractToken() (finalResult string) {
	if r.token != nil {
		return *r.token
	}

	defer func() {
		r.token = new(finalResult)
	}()

	var value string

	if r.Headers != nil {
		value = strings.TrimSpace(r.Headers["authorization"])
	}
	if value == "" && r.QueryParams != nil {
		value = strings.TrimSpace(r.QueryParams["auth_token"])
	}
	if value == "" {
		return ""
	}

	parts := strings.Fields(value)
	if len(parts) == 0 {
		return ""
	}

	if len(parts) == 1 {
		return parts[0]
	}

	if len(parts) != 2 {
		return ""
	}

	if !strings.EqualFold(parts[0], "bearer") {
		return ""
	}

	return parts[1]
}

func (r *AuthRequest) ExtractBasic() (username string, password string) {
	if r.basic != nil {
		return r.basic.username, r.basic.password
	}

	defer func() {
		r.basic = &authBasic{
			username: username,
			password: password,
		}
	}()

	headerValue := ""
	if r.Headers != nil {
		headerValue = strings.TrimSpace(r.Headers["authorization"])
	}
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
	header = strings.ToLower(header)
	if header == "" {
		return ""
	}

	if r.apiKeyByHeader == nil {
		r.apiKeyByHeader = make(map[string]string, 1)
	} else if result, ok := r.apiKeyByHeader[header]; ok {
		return result
	}

	defer func() {
		r.apiKeyByHeader[header] = finalResult
	}()

	var value string
	if r.Headers != nil {
		value = strings.TrimSpace(r.Headers[header])
	}
	if value == "" && r.QueryParams != nil {
		value = strings.TrimSpace(r.QueryParams[header])
	}

	return value
}

func (r *AuthRequest) ExtractIPs() (finalResult []string) {
	if r.ips != nil {
		return r.ips
	}

	defer func() {
		r.ips = lo.Uniq(finalResult)
	}()

	result := make([]string, 0, 10)

	appendIP := func(raw string) {
		if raw = strings.TrimSpace(raw); raw == "" {
			return
		}

		ip, err := netip.ParseAddr(raw)
		if err != nil {
			return
		}

		result = append(result, ip.String())
	}

	appendIPList := func(raw string) {
		if raw = strings.TrimSpace(raw); raw == "" {
			return
		}
		for _, x := range strings.Split(raw, ",") {
			appendIP(x)
		}
	}
	appendRemoteIP := func(raw string) {
		if raw = strings.TrimSpace(raw); raw == "" {
			return
		}

		appendIP(raw)

		host, _, err := net.SplitHostPort(raw)
		if err != nil {
			return
		}

		appendIP(host)
	}

	if r.Headers != nil {
		appendIPList(r.Headers["x-forwarded-for"])
		appendIP(r.Headers["x-real-ip"])
		appendIP(r.Headers["cf-connecting-ip"])
		appendIP(r.Headers["true-client-ip"])
		appendIP(r.Headers["x-client-ip"])
		appendIP(r.Headers["x-cluster-client-ip"])
	}
	appendRemoteIP(r.RemoteAddr)

	return result
}

func (r *AuthRequest) resetExtractedData() {
	r.token = nil
	r.basic = nil
	r.apiKeyByHeader = nil
	r.ips = nil
}
