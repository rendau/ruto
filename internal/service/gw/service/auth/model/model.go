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
	Headers     url.Values
	QueryParams url.Values
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
	r.Headers = url.Values(headers)
}

func (r *AuthRequest) SetHttpQueryParams(qPars url.Values) {
	r.QueryParams = qPars
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
	if r.basic != nil {
		return r.basic.username, r.basic.password
	}

	defer func() {
		r.basic = &authBasic{
			username: finalUsername,
			password: finalPassword,
		}
	}()

	headerValue := r.getHeadersValue("Authorization")
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

	appendRemoteIP(r.RemoteAddr)
	appendIPList(r.getHeadersValue("X-Forwarded-For"))
	appendIP(r.getHeadersValue("X-Real-Ip"))
	appendIP(r.getHeadersValue("Cf-Connecting-Ip"))
	appendIP(r.getHeadersValue("True-Client-Ip"))
	appendIP(r.getHeadersValue("X-Client-Ip"))
	appendIP(r.getHeadersValue("X-Cluster-Client-Ip"))

	return result
}

func (r *AuthRequest) findValue(headerKey, queryParamKey string) string {
	val := r.getHeadersValue(headerKey)
	if val == "" {
		val = r.getQueryParamsValue(queryParamKey)
	}
	return val
}

func (r *AuthRequest) getHeadersValue(key string) string {
	return getValueFromUrlValues(r.Headers, key)
}

func (r *AuthRequest) getQueryParamsValue(key string) string {
	return getValueFromUrlValues(r.QueryParams, key)
}

func getValueFromUrlValues(source url.Values, key string) string {
	if key == "" || source == nil || len(source) == 0 {
		return ""
	}

	val := strings.TrimSpace(source.Get(key))
	if val == "" {
		val = strings.TrimSpace(source.Get(strings.ToLower(key)))
	}

	return val
}
