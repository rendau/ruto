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
	Header string                 `json:"header"`
	Keys   []AuthMethodAPIKeyItem `json:"keys"`
}

type AuthMethodAPIKeyItem struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// AuthMethodJWT accepts tokens signed by a key of Provider (a Root jwt
// provider name; empty — any provider). Kid optionally pins one key. A role
// written as "client:role" is looked up only among that client's roles
// (resource_access.<client>.roles); a bare role anywhere roles live.
type AuthMethodJWT struct {
	Provider string   `json:"provider"`
	Kid      string   `json:"kid"`
	Roles    []string `json:"roles"`
	// CookieName is the cookie the token may be read from: the provider's
	// cookie, or the cookie of the app's own login client. Set when the
	// config is inherited down, so it is never stored.
	CookieName string `json:"-"`
}

// JwtClient is a registered OAuth client of a provider: the gateway logs the
// user in on its behalf. A provider carries the default one, an app may bring
// its own.
type JwtClient struct {
	Id     string   `json:"id"`
	Secret string   `json:"secret"`
	Scopes []string `json:"scopes"`
}

type AuthMethodIPValidation struct {
	AllowedIps []AuthMethodIPValidationItem `json:"allowed_ips"`
}

type AuthMethodIPValidationItem struct {
	Name string `json:"name"`
	Ip   string `json:"ip"`
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
		Keys:   append([]AuthMethodAPIKeyItem(nil), m.Keys...),
	}
}

func (m *AuthMethodJWT) Clone() *AuthMethodJWT {
	if m == nil {
		return nil
	}

	return &AuthMethodJWT{
		Provider:   m.Provider,
		Kid:        m.Kid,
		Roles:      append([]string(nil), m.Roles...),
		CookieName: m.CookieName,
	}
}

func (m *JwtClient) Clone() *JwtClient {
	if m == nil {
		return nil
	}
	return &JwtClient{
		Id:     m.Id,
		Secret: m.Secret,
		Scopes: append([]string(nil), m.Scopes...),
	}
}

func (m *AuthMethodIPValidation) Clone() *AuthMethodIPValidation {
	if m == nil {
		return nil
	}

	return &AuthMethodIPValidation{
		AllowedIps: append([]AuthMethodIPValidationItem(nil), m.AllowedIps...),
	}
}

// MergeKey identifies the key set a jwt method accepts: methods with the same
// provider and kid merge their roles on extend.
func (m *AuthMethodJWT) MergeKey() string {
	return m.Provider + "|" + m.Kid
}
