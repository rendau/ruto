package model

import (
	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
)

func Merge(parent, child Auth) Auth {
	result := Auth{}

	result.applyChild(parent)
	result.applyChild(child)

	return result
}

func (m *Auth) applyChild(child Auth) {
	m.Enabled = child.Enabled
	m.Mode = child.Mode

	if m.Enabled {
		switch m.Mode {
		case constant.AuthModeReplace:
			m.Methods = append(make([]*AuthMethod, 0, len(child.Methods)), child.Methods...)
		case constant.AuthModeExtend:
			m.Methods = mergeMethods(m.Methods, child.Methods)
		}
	} else {
		m.Methods = []*AuthMethod{}
	}
}

func mergeMethods(parent, child []*AuthMethod) []*AuthMethod {
	result := make([]*AuthMethod, 0, len(parent)+len(child))
	jwtMethodByKid := make(map[string]*AuthMethod, len(parent))
	var (
		basicMethod        *AuthMethod
		apiKeyMethod       *AuthMethod
		ipValidationMethod *AuthMethod
	)

	appendMethod := func(method *AuthMethod) {
		if method == nil {
			result = append(result, method)
			return
		}

		if !method.HasSingleType() {
			result = append(result, method)
			return
		}

		switch {
		case method.Basic != nil:
			if basicMethod == nil {
				basicMethod = method
				result = append(result, method)
				return
			}

			basicMethod.Basic.Users = append(basicMethod.Basic.Users, method.Basic.Users...)
		case method.APIKey != nil:
			if apiKeyMethod == nil {
				apiKeyMethod = method
				result = append(result, method)
				return
			}

			if apiKeyMethod.APIKey.Header == "" {
				apiKeyMethod.APIKey.Header = method.APIKey.Header
			}
			apiKeyMethod.APIKey.Keys = append(apiKeyMethod.APIKey.Keys, method.APIKey.Keys...)
		case method.IPValidation != nil:
			if ipValidationMethod == nil {
				ipValidationMethod = method
				result = append(result, method)
				return
			}

			ipValidationMethod.IPValidation.AllowedIps = append(ipValidationMethod.IPValidation.AllowedIps, method.IPValidation.AllowedIps...)
		case method.JWT != nil:
			if method.JWT.Kid == "" {
				result = append(result, method)
				return
			}

			if parentMethod, ok := jwtMethodByKid[method.JWT.Kid]; ok {
				parentMethod.JWT.Roles = lo.Uniq(append(parentMethod.JWT.Roles, method.JWT.Roles...))
				return
			}

			jwtMethodByKid[method.JWT.Kid] = method
			result = append(result, method)
		default:
			result = append(result, method)
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
