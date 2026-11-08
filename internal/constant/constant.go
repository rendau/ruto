package constant

import "strings"

const (
	ServiceName = "ruto"
)

const (
	AuthModeExtend  = "extend"
	AuthModeReplace = "replace"
)

func AuthModeIsValid(v string) bool {
	return v == AuthModeExtend || v == AuthModeReplace
}

// AuthAPIKeyDefaultHeader carries the api key when an api_key method sets no header.
const AuthAPIKeyDefaultHeader = "x-api-key"

// JwtCookiePrefix starts the name of the cookie that holds a provider's JWT:
// one cookie per provider, so logins of different providers live side by side
// on the same host. An app with its own login client gets its own cookie,
// named after the provider and the app's path prefix.
const JwtCookiePrefix = "ruto_"

// JwtRefreshCookiePrefix and JwtLoginCookiePrefix start the names of the
// refresh cookie and of the short-lived login (state + PKCE) cookie of a login
// unit. Their "-" keeps them apart from any access cookie name.
const (
	JwtRefreshCookiePrefix = "ruto-rt_"
	JwtLoginCookiePrefix   = "ruto-login_"
)

func JwtCookieName(provider string) string {
	return JwtCookiePrefix + provider
}

// JwtAppCookieName names the access cookie of an app's own login client:
// "ruto_<provider>_<prefix>", the prefix slashes replaced, since a cookie name
// may not contain "/".
func JwtAppCookieName(provider, appPathPrefix string) string {
	return JwtCookiePrefix + provider + "_" + LoginUnitSlug(appPathPrefix)
}

// LoginUnitSlug turns an app path prefix ("/shop/api") into a cookie-safe
// suffix ("shop.api").
func LoginUnitSlug(appPathPrefix string) string {
	return strings.ReplaceAll(strings.Trim(appPathPrefix, "/"), "/", ".")
}

// AuthRoutePrefix is the first path segment of the gateway's own login routes
// (/_auth/<provider>[/<app prefix>]/login|callback|refresh|logout); no app may take it.
const AuthRoutePrefix = "_auth"

// OidcScopeOpenid is the scope every OpenID Connect login must ask for.
const OidcScopeOpenid = "openid"

const (
	LoggingModeExtend  = "extend"
	LoggingModeReplace = "replace"

	LoggingLevelAll   = "all"
	LoggingLevelError = "error"
	LoggingLevelNone  = "none"
)

func LoggingModeIsValid(v string) bool {
	return v == LoggingModeExtend || v == LoggingModeReplace
}

func LoggingLevelIsValid(v string) bool {
	return v == LoggingLevelAll || v == LoggingLevelError || v == LoggingLevelNone
}

// DefaultBodyLogLimit is the fallback byte limit applied when body logging
// is enabled but no explicit limit is configured.
const DefaultBodyLogLimit = 4096

const (
	CacheScopePrivate = "private"
	CacheScopePublic  = "public"
	CacheScopeNoStore = "no-store"
)

func CacheScopeIsValid(v string) bool {
	return v == CacheScopePrivate || v == CacheScopePublic || v == CacheScopeNoStore
}

// DefaultCacheMaxBodySize is the fallback cap on a response body the gateway
// keeps in its cache when an endpoint sets no explicit limit.
const DefaultCacheMaxBodySize = 1 << 20

const (
	CompressionBackendIdentity = "identity" // ask the backend for an uncompressed body
	CompressionBackendClient   = "client"   // pass the client's Accept-Encoding on

	CompressionResponseOn  = "on"
	CompressionResponseOff = "off"

	CompressionAlgorithmBr   = "br"
	CompressionAlgorithmZstd = "zstd"
	CompressionAlgorithmGzip = "gzip"
)

func CompressionBackendIsValid(v string) bool {
	return v == CompressionBackendIdentity || v == CompressionBackendClient
}

func CompressionResponseIsValid(v string) bool {
	return v == CompressionResponseOn || v == CompressionResponseOff
}

func CompressionAlgorithmIsValid(v string) bool {
	return v == CompressionAlgorithmBr || v == CompressionAlgorithmZstd || v == CompressionAlgorithmGzip
}

// DefaultCompressionAlgorithms is the order the gateway offers encodings in
// when the root sets none: the client gets the first one it accepts.
var DefaultCompressionAlgorithms = []string{
	CompressionAlgorithmBr,
	CompressionAlgorithmZstd,
	CompressionAlgorithmGzip,
}

// DefaultCompressionMinSize is the body size in bytes under which a response
// is sent as is: the encoding overhead would eat the gain.
const DefaultCompressionMinSize = 1024

// StreamingContentTypes name responses sent as a stream of small pieces,
// each of which must reach the client at once. For every other media type a
// response under the compression threshold is held back until it ends.
var StreamingContentTypes = []string{
	"text/event-stream",
	"application/x-ndjson",
	"application/stream+json",
	"multipart/x-mixed-replace",
}

// DefaultCompressionContentTypes are the media types compressed when the root
// lists none: text and structured data. Images, archives and binary bodies
// do not shrink. "type/*" matches a whole type; a "+json"/"+xml" structured
// suffix matches the plain application type.
var DefaultCompressionContentTypes = []string{
	"text/*",
	"application/json",
	"application/xml",
	"application/javascript",
	"application/x-ndjson",
	"application/x-www-form-urlencoded",
	"image/svg+xml",
}

// SupportedJWTAlgorithms are the asymmetric JWS algorithms (RFC 7518, RFC
// 8037) the gateway verifies tokens with: RSA PKCS#1 v1.5 and PSS, ECDSA on
// the NIST curves, Ed25519. Symmetric ones (HS*) need the provider's secret
// and are not accepted.
var SupportedJWTAlgorithms = []string{
	"RS256", "RS384", "RS512",
	"PS256", "PS384", "PS512",
	"ES256", "ES384", "ES512",
	"EdDSA",
}

var supportedJWTAlgorithmMap = func() map[string]struct{} {
	m := make(map[string]struct{}, len(SupportedJWTAlgorithms))
	for _, alg := range SupportedJWTAlgorithms {
		m[alg] = struct{}{}
	}
	return m
}()

func IsSupportedJWTAlgorithm(v string) bool {
	_, ok := supportedJWTAlgorithmMap[v]
	return ok
}

// JWTKeyAlgorithm is the algorithm a JWK signs with: its alg when named;
// otherwise, for a curve key, the one algorithm its curve allows (RFC 7518
// §3.4, RFC 8037). An RSA key without alg could use any RS*/PS*, so it gets
// none and is not accepted.
func JWTKeyAlgorithm(kty, crv, alg string) string {
	if alg != "" {
		return alg
	}
	switch kty {
	case "EC":
		switch crv {
		case "P-256":
			return "ES256"
		case "P-384":
			return "ES384"
		case "P-521":
			return "ES512"
		}
	case "OKP":
		if crv == "Ed25519" {
			return "EdDSA"
		}
	}
	return ""
}
