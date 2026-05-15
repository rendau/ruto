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
