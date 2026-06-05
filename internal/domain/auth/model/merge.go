package model

import (
	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
)

func Merge(parent, child Auth) Auth {
	if !child.Enabled {
		return Auth{
			Enabled: false,
			Mode:    child.Mode,
		}
	}

	result := Auth{
		Enabled: true,
		Mode:    child.Mode,
	}

	switch child.Mode {
	case constant.AuthModeReplace:
		result.Methods = child.CloneMethods()
	case constant.AuthModeExtend:
		result.Methods = mergeMethods(parent.Methods, child.Methods)
	}

	return result
}

func mergeMethods(parent, child []*AuthMethod) []*AuthMethod {
	result := make([]*AuthMethod, 0, len(parent)+len(child))

	var (
		basicMethod        *AuthMethod
		ipValidationMethod *AuthMethod
	)

	apiKeyMethods := make(map[string]*AuthMethod)
	jwtMethods := make(map[string]*AuthMethod)

	appendMethod := func(method *AuthMethod) {
		if method == nil {
			result = append(result, nil)
			return
		}

		switch method.Type() {
		case AuthMethodTypeBasic:
			if basicMethod == nil {
				basicMethod = method.Clone()
				result = append(result, basicMethod)
				return
			}

			basicMethod.Basic.Users = append(
				basicMethod.Basic.Users,
				method.Basic.Users...,
			)

		case AuthMethodTypeAPIKey:
			if existing, ok := apiKeyMethods[method.APIKey.Header]; ok {
				existing.APIKey.Keys = append(
					existing.APIKey.Keys,
					method.APIKey.Keys...,
				)
				return
			}

			cloned := method.Clone()
			apiKeyMethods[cloned.APIKey.Header] = cloned
			result = append(result, cloned)

		case AuthMethodTypeIPValidation:
			if ipValidationMethod == nil {
				ipValidationMethod = method.Clone()
				result = append(result, ipValidationMethod)
				return
			}

			ipValidationMethod.IPValidation.AllowedIps = append(
				ipValidationMethod.IPValidation.AllowedIps,
				method.IPValidation.AllowedIps...,
			)

		case AuthMethodTypeJWT:
			if existing, ok := jwtMethods[method.JWT.Kid]; ok {
				existing.JWT.Roles = lo.Uniq(
					append(existing.JWT.Roles, method.JWT.Roles...),
				)
				return
			}

			cloned := method.Clone()
			jwtMethods[cloned.JWT.Kid] = cloned
			result = append(result, cloned)

		default:
			result = append(result, method.Clone())
		}
	}

	for _, method := range parent {
		appendMethod(method)
	}

	for _, method := range child {
		appendMethod(method)
	}

	return result
}
