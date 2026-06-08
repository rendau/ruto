package model

import (
	"fmt"
	"strings"

	"github.com/rendau/ruto/internal/constant"
)

func (m *Logging) Normalize() error {
	m.Mode = strings.ToLower(strings.TrimSpace(m.Mode))
	if m.Mode == "" {
		m.Mode = constant.LoggingModeExtend
	}
	if !constant.LoggingModeIsValid(m.Mode) {
		return fmt.Errorf("mode: is invalid")
	}

	m.Level = strings.ToLower(strings.TrimSpace(m.Level))
	if m.Level != "" && !constant.LoggingLevelIsValid(m.Level) {
		return fmt.Errorf("level: is invalid")
	}

	if m.ReqBodyLimit < 0 {
		return fmt.Errorf("req_body_limit: must be >= 0")
	}
	if m.RespBodyLimit < 0 {
		return fmt.Errorf("resp_body_limit: must be >= 0")
	}

	return nil
}
