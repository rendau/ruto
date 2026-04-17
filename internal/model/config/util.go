package config

import (
	"errors"
	"strings"
)

var (
	errEmptyValue    = errors.New("must not be empty")
	errNegativeValue = errors.New("must be >= 0")
)

func normalizeStringList(values []string) []string {
	finalValues := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		finalValues = append(finalValues, value)
	}
	return finalValues
}

func validateNotEmpty(value string) error {
	if value == "" {
		return errEmptyValue
	}
	return nil
}

func validateNonNegative[T ~int64](value T) error {
	if value < 0 {
		return errNegativeValue
	}
	return nil
}
