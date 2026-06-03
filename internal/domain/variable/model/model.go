package model

import (
	"fmt"
	"regexp"
	"strings"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

var interpolationPattern = regexp.MustCompile(`\$\{([A-Za-z0-9_.-]+)\}|\{\{([A-Za-z0-9_.-]+)\}\}`)

type Variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NormalizeList(items []Variable) ([]Variable, error) {
	if len(items) == 0 {
		return nil, nil
	}

	result := make([]Variable, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		item.Key = strings.TrimSpace(item.Key)
		item.Value = strings.TrimSpace(item.Value)
		if item.Key == "" {
			return nil, fmt.Errorf("[%d].key: empty", i)
		}
		if _, ok := seen[item.Key]; ok {
			return nil, fmt.Errorf("[%d].key: duplicate %q", i, item.Key)
		}
		seen[item.Key] = struct{}{}
		result = append(result, item)
	}

	return result, nil
}

func Merge(parent, child []Variable) []Variable {
	if len(parent) == 0 {
		return child
	}
	if len(child) == 0 {
		return parent
	}

	values := make(map[string]Variable, len(parent)+len(child))
	order := make([]string, 0, len(parent)+len(child))
	for _, item := range parent {
		if _, ok := values[item.Key]; !ok {
			order = append(order, item.Key)
		}
		values[item.Key] = item
	}
	for _, item := range child {
		if _, ok := values[item.Key]; !ok {
			order = append(order, item.Key)
		}
		values[item.Key] = item
	}

	result := make([]Variable, 0, len(values))
	for _, key := range order {
		result = append(result, values[key])
	}
	return result
}

func Resolve(items []Variable) (map[string]string, error) {
	raw := make(map[string]string, len(items))
	for _, item := range items {
		raw[item.Key] = item.Value
	}

	resolved := make(map[string]string, len(raw))
	resolving := make(map[string]struct{}, len(raw))
	for key := range raw {
		if _, err := resolveKey(key, raw, resolved, resolving); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}

func ResolveList(items []Variable) ([]Variable, error) {
	scope, err := Resolve(items)
	if err != nil {
		return nil, err
	}

	result := make([]Variable, 0, len(items))
	for _, item := range items {
		result = append(result, Variable{
			Key:   item.Key,
			Value: scope[item.Key],
		})
	}
	return result, nil
}

func InterpolateString(value string, scope map[string]string) (string, error) {
	var resultErr error
	result := interpolationPattern.ReplaceAllStringFunc(value, func(match string) string {
		if resultErr != nil {
			return match
		}
		varName := variableNameFromMatch(match)
		if varName == "" {
			return match
		}
		replacement, ok := scope[varName]
		if !ok {
			resultErr = fmt.Errorf("unknown variable %q", varName)
			return match
		}
		return replacement
	})
	if resultErr != nil {
		return "", resultErr
	}
	return result, nil
}

func InterpolateMap(values map[string]string, scope map[string]string) (map[string]string, error) {
	if len(values) == 0 {
		return values, nil
	}

	result := make(map[string]string, len(values))
	for key, value := range values {
		nextKey, err := InterpolateString(key, scope)
		if err != nil {
			return nil, fmt.Errorf("%s key: %w", key, err)
		}
		nextValue, err := InterpolateString(value, scope)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		if _, ok := result[nextKey]; ok {
			return nil, fmt.Errorf("%s: duplicate key after interpolation", key)
		}
		result[nextKey] = nextValue
	}
	return result, nil
}

func InterpolateAuth(src authModel.Auth, scope map[string]string) (authModel.Auth, error) {
	result := authModel.Auth{
		Enabled: src.Enabled,
		Mode:    src.Mode,
		Methods: make([]*authModel.AuthMethod, 0, len(src.Methods)),
	}

	for methodIndex, method := range src.Methods {
		nextMethod, err := interpolateAuthMethod(method, scope)
		if err != nil {
			return authModel.Auth{}, fmt.Errorf("methods[%d]: %w", methodIndex, err)
		}
		result.Methods = append(result.Methods, nextMethod)
	}

	return result, nil
}

func resolveKey(key string, raw, resolved map[string]string, resolving map[string]struct{}) (string, error) {
	if value, ok := resolved[key]; ok {
		return value, nil
	}
	value, ok := raw[key]
	if !ok {
		return "", fmt.Errorf("unknown variable %q", key)
	}
	if _, ok = resolving[key]; ok {
		return "", fmt.Errorf("variable cycle at %q", key)
	}

	resolving[key] = struct{}{}
	var resultErr error
	result := interpolationPattern.ReplaceAllStringFunc(value, func(match string) string {
		if resultErr != nil {
			return match
		}
		varName := variableNameFromMatch(match)
		if varName == "" {
			return match
		}
		replacement, err := resolveKey(varName, raw, resolved, resolving)
		if err != nil {
			resultErr = err
			return match
		}
		return replacement
	})
	delete(resolving, key)
	if resultErr != nil {
		return "", resultErr
	}

	resolved[key] = result
	return result, nil
}

func variableNameFromMatch(match string) string {
	groups := interpolationPattern.FindStringSubmatch(match)
	for _, group := range groups[1:] {
		if group != "" {
			return group
		}
	}
	return ""
}

func interpolateAuthMethod(src *authModel.AuthMethod, scope map[string]string) (*authModel.AuthMethod, error) {
	if src == nil {
		return nil, nil
	}

	result := &authModel.AuthMethod{}
	var err error
	if src.Basic != nil {
		result.Basic, err = interpolateBasic(src.Basic, scope)
		if err != nil {
			return nil, fmt.Errorf("basic: %w", err)
		}
	}
	if src.APIKey != nil {
		result.APIKey, err = interpolateAPIKey(src.APIKey, scope)
		if err != nil {
			return nil, fmt.Errorf("api_key: %w", err)
		}
	}
	if src.JWT != nil {
		result.JWT = &authModel.AuthMethodJWT{
			Kid:   src.JWT.Kid,
			Roles: append([]string(nil), src.JWT.Roles...),
		}
	}
	if src.IPValidation != nil {
		result.IPValidation, err = interpolateIPValidation(src.IPValidation, scope)
		if err != nil {
			return nil, fmt.Errorf("ip_validation: %w", err)
		}
	}

	return result, nil
}

func interpolateBasic(src *authModel.AuthMethodBasic, scope map[string]string) (*authModel.AuthMethodBasic, error) {
	result := &authModel.AuthMethodBasic{
		Users: make([]authModel.AuthMethodBasicUser, 0, len(src.Users)),
	}
	for i, user := range src.Users {
		username, err := InterpolateString(user.Username, scope)
		if err != nil {
			return nil, fmt.Errorf("users[%d].username: %w", i, err)
		}
		password, err := InterpolateString(user.Password, scope)
		if err != nil {
			return nil, fmt.Errorf("users[%d].password: %w", i, err)
		}
		result.Users = append(result.Users, authModel.AuthMethodBasicUser{
			Username: username,
			Password: password,
		})
	}
	return result, nil
}

func interpolateAPIKey(src *authModel.AuthMethodAPIKey, scope map[string]string) (*authModel.AuthMethodAPIKey, error) {
	header, err := InterpolateString(src.Header, scope)
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	keys := make([]string, 0, len(src.Keys))
	for i, key := range src.Keys {
		nextKey, err := InterpolateString(key, scope)
		if err != nil {
			return nil, fmt.Errorf("keys[%d]: %w", i, err)
		}
		keys = append(keys, nextKey)
	}
	return &authModel.AuthMethodAPIKey{
		Header: header,
		Keys:   keys,
	}, nil
}

func interpolateIPValidation(src *authModel.AuthMethodIPValidation, scope map[string]string) (*authModel.AuthMethodIPValidation, error) {
	allowedIps := make([]string, 0, len(src.AllowedIps))
	for i, ip := range src.AllowedIps {
		nextIP, err := InterpolateString(ip, scope)
		if err != nil {
			return nil, fmt.Errorf("allowed_ips[%d]: %w", i, err)
		}
		allowedIps = append(allowedIps, nextIP)
	}
	return &authModel.AuthMethodIPValidation{AllowedIps: allowedIps}, nil
}
