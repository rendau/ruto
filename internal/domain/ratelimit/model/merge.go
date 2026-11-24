package model

import (
	"github.com/rendau/ruto/internal/constant"
)

// Merge gives the rules that apply on the child level: the parent's plus the
// child's own, or only the child's with mode "replace". The rules are copied,
// so the result never aliases either side.
func Merge(parent, child RateLimit) RateLimit {
	result := RateLimit{Mode: child.Mode}

	if child.Mode == constant.RateLimitModeReplace {
		result.Rules = child.CloneRules()
		return result
	}

	result.Rules = append(parent.CloneRules(), child.CloneRules()...)

	return result
}
