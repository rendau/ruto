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
	result := append(make([]*AuthMethod, 0, len(parent)+len(child)), parent...)
	jwtMethodByKid := make(map[string]*AuthMethod, len(parent))

	for _, method := range result {
		if method.JWT == nil || method.JWT.Kid == "" {
			continue
		}

		// Skip JWT merging if method has other auth types
		if *method != (AuthMethod{JWT: method.JWT}) {
			continue
		}

		jwtMethodByKid[method.JWT.Kid] = method
	}

	for _, method := range child {
		if method.JWT == nil || method.JWT.Kid == "" {
			result = append(result, method)
			continue
		}

		// Skip JWT merging if method has other auth types
		if *method != (AuthMethod{JWT: method.JWT}) {
			result = append(result, method)
			continue
		}

		if parentMethod, ok := jwtMethodByKid[method.JWT.Kid]; ok {
			parentMethod.JWT.Roles = lo.Uniq(append(parentMethod.JWT.Roles, method.JWT.Roles...))
			continue
		}

		jwtMethodByKid[method.JWT.Kid] = method

		result = append(result, method)
	}

	return result
}
