package model

type Auth struct {
	Enabled bool          `json:"enabled"`
	Mode    string        `json:"mode"` // "extend" | "replace"
	Methods []*AuthMethod `json:"methods"`
}

type AuthMethodType string

const (
	AuthMethodTypeUnknown      AuthMethodType = "unknown"
	AuthMethodTypeBasic        AuthMethodType = "basic"
	AuthMethodTypeAPIKey       AuthMethodType = "api_key"
	AuthMethodTypeJWT          AuthMethodType = "jwt"
	AuthMethodTypeIPValidation AuthMethodType = "ip_validation"
)

type AuthMethod struct {
	Basic        *AuthMethodBasic        `json:"basic,omitempty"`
	APIKey       *AuthMethodAPIKey       `json:"api_key,omitempty"`
	JWT          *AuthMethodJWT          `json:"jwt,omitempty"`
	IPValidation *AuthMethodIPValidation `json:"ip_validation,omitempty"`
}

type AuthMethodBasic struct {
	Users []AuthMethodBasicUser `json:"users"`
}

type AuthMethodBasicUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthMethodAPIKey struct {
	Header string   `json:"header"`
	Keys   []string `json:"keys"`
}

type AuthMethodJWT struct {
	Kid   string   `json:"kid"`
	Roles []string `json:"roles"`
}

type AuthMethodIPValidation struct {
	AllowedIps []string `json:"allowed_ips"`
}

func (m *Auth) CloneMethods() []*AuthMethod {
	result := make([]*AuthMethod, len(m.Methods))
	for i, method := range m.Methods {
		result[i] = method.Clone()
	}
	return result
}

func (m *AuthMethod) Type() AuthMethodType {
	var (
		typ   AuthMethodType
		count int
	)

	if m.Basic != nil {
		typ = AuthMethodTypeBasic
		count++
	}

	if m.APIKey != nil {
		typ = AuthMethodTypeAPIKey
		count++
	}

	if m.JWT != nil {
		typ = AuthMethodTypeJWT
		count++
	}

	if m.IPValidation != nil {
		typ = AuthMethodTypeIPValidation
		count++
	}

	if count != 1 {
		return AuthMethodTypeUnknown
	}

	return typ
}

func (m *AuthMethod) Clone() *AuthMethod {
	return &AuthMethod{
		Basic:        m.Basic.Clone(),
		APIKey:       m.APIKey.Clone(),
		JWT:          m.JWT.Clone(),
		IPValidation: m.IPValidation.Clone(),
	}
}

func (m *AuthMethodBasic) Clone() *AuthMethodBasic {
	if m == nil {
		return nil
	}

	return &AuthMethodBasic{
		Users: append([]AuthMethodBasicUser(nil), m.Users...),
	}
}

func (m *AuthMethodAPIKey) Clone() *AuthMethodAPIKey {
	if m == nil {
		return nil
	}

	return &AuthMethodAPIKey{
		Header: m.Header,
		Keys:   append([]string(nil), m.Keys...),
	}
}

func (m *AuthMethodJWT) Clone() *AuthMethodJWT {
	if m == nil {
		return nil
	}

	return &AuthMethodJWT{
		Kid:   m.Kid,
		Roles: append([]string(nil), m.Roles...),
	}
}

func (m *AuthMethodIPValidation) Clone() *AuthMethodIPValidation {
	if m == nil {
		return nil
	}

	return &AuthMethodIPValidation{
		AllowedIps: append([]string(nil), m.AllowedIps...),
	}
}
