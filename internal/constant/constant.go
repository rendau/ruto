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
