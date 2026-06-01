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
	Headers     http.Header
	QueryParams url.Values
	RemoteAddr  string

	token          authToken
	basic          authBasic
	apiKeyByHeader map[string]string
	ips            []string
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
		if raw == "" {
			return
		}
		for _, x := range strings.Split(raw, ",") {
			appendIP(x)
		}
	}
	appendRemoteIP := func(raw string) {
		if raw == "" {
			return
		}
		appendIP(raw)
		host, _, err := net.SplitHostPort(raw)
		if err != nil {
			return
		}
		appendIP(host)
	}

	appendIPList(r.getHeaderValue("X-Forwarded-For"))
	appendIP(r.getHeaderValue("X-Real-Ip"))
	appendIP(r.getHeaderValue("Cf-Connecting-Ip"))
	appendIP(r.getHeaderValue("True-Client-Ip"))
	appendIP(r.getHeaderValue("X-Client-Ip"))
	appendIP(r.getHeaderValue("X-Cluster-Client-Ip"))
	appendRemoteIP(r.RemoteAddr)

	return result
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

	return strings.TrimSpace(r.Headers.Get(key))
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
