package model

type Auth struct {
	Enabled bool          `json:"enabled"`
	Mode    string        `json:"mode"` // "extend" | "replace"
	Methods []*AuthMethod `json:"methods"`
}

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

func (m *AuthMethod) HasSingleType() bool {
	count := 0
	if m.Basic != nil {
		count++
	}
	if m.APIKey != nil {
		count++
	}
	if m.JWT != nil {
		count++
	}
	if m.IPValidation != nil {
		count++
	}
	return count == 1
}
