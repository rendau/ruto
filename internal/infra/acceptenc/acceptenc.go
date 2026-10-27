// Package acceptenc reads the Accept-Encoding request header: which content
// codings the client takes and which of the ones a server offers it prefers.
package acceptenc

import (
	"strconv"
	"strings"
)

// Accepts reports whether an Accept-Encoding header allows the coding: listed
// with a positive weight, or covered by "*". An absent coding is refused, as
// is one with q=0.
func Accepts(header, coding string) bool {
	weight, listed := weightOf(header, coding)
	if listed {
		return weight > 0
	}
	weight, listed = weightOf(header, "*")
	return listed && weight > 0
}

// Pick returns the first of the offered codings the header allows, or "" when
// the client takes none of them. Offered is the server's order of
// preference; the client's own q weights only include or exclude a coding.
func Pick(header string, offered []string) string {
	for _, coding := range offered {
		if Accepts(header, coding) {
			return coding
		}
	}
	return ""
}

func weightOf(header, coding string) (float64, bool) {
	for part := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(name), coding) {
			continue
		}

		q, found := strings.CutPrefix(strings.TrimSpace(params), "q=")
		if !found {
			return 1, true
		}
		weight, err := strconv.ParseFloat(strings.TrimSpace(q), 64)
		if err != nil {
			return 0, true
		}
		return weight, true
	}

	return 0, false
}
