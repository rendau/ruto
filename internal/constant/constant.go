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
