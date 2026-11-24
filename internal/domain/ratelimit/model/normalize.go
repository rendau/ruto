package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
)

func (m *RateLimit) Normalize() error {
	m.Mode = strings.ToLower(strings.TrimSpace(m.Mode))
	if m.Mode == "" {
		m.Mode = constant.RateLimitModeExtend
	}
	if !constant.RateLimitModeIsValid(m.Mode) {
		return fmt.Errorf("mode: is invalid")
	}

	m.Rules = lo.Filter(m.Rules, func(v *Rule, _ int) bool { return v != nil })
	for i, rule := range m.Rules {
		if err := rule.Normalize(); err != nil {
			return fmt.Errorf("rules[%d]: %w", i, err)
		}
	}

	return nil
}

func (m *Rule) Normalize() error {
	m.Name = strings.TrimSpace(m.Name)
	m.Scope = ""

	m.Key = strings.ToLower(strings.TrimSpace(m.Key))
	if m.Key == "" {
		m.Key = constant.RateLimitKeyIP
	}
	if !constant.RateLimitKeyIsValid(m.Key) {
		return fmt.Errorf("key: is invalid")
	}

	if m.Requests <= 0 {
		return fmt.Errorf("requests: must be > 0")
	}
	if m.Requests > constant.RateLimitMaxRequests {
		return fmt.Errorf("requests: must be <= %d", constant.RateLimitMaxRequests)
	}

	m.Period = strings.TrimSpace(m.Period)
	period, err := time.ParseDuration(m.Period)
	if err != nil {
		return fmt.Errorf("period: %w", err)
	}
	if period < constant.RateLimitMinPeriod || period > constant.RateLimitMaxPeriod {
		return fmt.Errorf("period: must be from %s to %s", constant.RateLimitMinPeriod, constant.RateLimitMaxPeriod)
	}
	if period%time.Second != 0 {
		return fmt.Errorf("period: must be whole seconds")
	}
	m.ParsedPeriod = period

	return nil
}
