package model

import (
	"fmt"
	"regexp"
	"strings"
)

var interpolationPattern = regexp.MustCompile(`\{\{([A-Za-z0-9_.-]+)\}\}`)
var variableNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type Vars map[string]string

func New(cap int) Vars {
	return make(Vars, cap)
}

func NewFromMerge(vars ...Vars) Vars {
	resultCap := 0
	for _, v := range vars {
		resultCap += len(v)
	}
	result := New(resultCap)
	result.Assign(vars...)
	return result
}

func (m *Vars) Assign(vars ...Vars) {
	if *m == nil {
		*m = New(0)
	}

	for _, v := range vars {
		for k, val := range v {
			(*m)[k] = val
		}
	}
}

func (m *Vars) Normalize() error {
	if *m == nil {
		*m = New(0)
		return nil
	}

	normalized := make(Vars, len(*m))

	for k, v := range *m {
		k = strings.TrimSpace(k)
		if k == "" {
			return fmt.Errorf("empty key after normalization")
		}
		if !variableNamePattern.MatchString(k) {
			return fmt.Errorf("invalid key %q after normalization", k)
		}

		v = strings.TrimSpace(v)
		if v == "" {
			return fmt.Errorf("empty value for key %q after normalization", k)
		}

		normalized[k] = v
	}

	*m = normalized

	return nil
}

func (m *Vars) InterpolateString(s string) string {
	if s == "" {
		return s
	}

	return interpolationPattern.ReplaceAllStringFunc(s, func(match string) string {
		key := match[2 : len(match)-2] // "{{" and "}}"

		if value, ok := (*m)[key]; ok {
			return value
		}

		return match
	})
}

func (m *Vars) InterpolateVars(vars Vars) Vars {
	if len(vars) == 0 {
		return vars
	}

	result := New(len(vars))

	for k, v := range vars {
		result[m.InterpolateString(k)] = m.InterpolateString(v)
	}

	return result
}

func (m *Vars) InterpolateStrings(values []string) []string {
	result := make([]string, len(values))

	for i, value := range values {
		result[i] = m.InterpolateString(value)
	}

	return result
}
