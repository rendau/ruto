package model

import "github.com/rendau/ruto/internal/constant"

// Merge combines parent and child logging config following the child's mode.
//
//   - replace: the child fully overrides the parent.
//   - extend (default): the "what to log" flags are unioned (the child can add
//     to but not disable the parent's flags); level and body limits are taken
//     from the child when set, otherwise inherited from the parent.
func Merge(parent, child Logging) Logging {
	if child.Mode == constant.LoggingModeReplace {
		return child
	}

	result := Logging{
		Mode:        constant.LoggingModeExtend,
		Headers:     parent.Headers || child.Headers,
		QueryParams: parent.QueryParams || child.QueryParams,
		ReqBody:     parent.ReqBody || child.ReqBody,
		RespBody:    parent.RespBody || child.RespBody,
	}

	result.Level = child.Level
	if result.Level == "" {
		result.Level = parent.Level
	}

	result.ReqBodyLimit = child.ReqBodyLimit
	if result.ReqBodyLimit == 0 {
		result.ReqBodyLimit = parent.ReqBodyLimit
	}

	result.RespBodyLimit = child.RespBodyLimit
	if result.RespBodyLimit == 0 {
		result.RespBodyLimit = parent.RespBodyLimit
	}

	return result
}
