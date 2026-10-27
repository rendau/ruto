package constant

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

var SupportedJWTAlgorithms = []string{
	"RS256",
	"RS384",
	"RS512",
}

var supportedJWTAlgorithmMap = map[string]struct{}{
	"RS256": {},
	"RS384": {},
	"RS512": {},
}

func IsSupportedJWTAlgorithm(v string) bool {
	_, ok := supportedJWTAlgorithmMap[v]
	return ok
}
