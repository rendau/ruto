package model

type Session struct {
	Id      int64
	Admin   bool
	AllApps bool
	AppIds  []string

	// ApiKeyId/ApiKeyName are set when the session comes from an MCP api key
	// rather than an admin login.
	ApiKeyId   string
	ApiKeyName string
}

func New(id int64) *Session {
	return &Session{Id: id}
}

func (s *Session) IsAuthorized() bool {
	return s != nil && s.Id > 0
}

func (s *Session) IsAdmin() bool {
	return s != nil && s.IsAuthorized() && s.Admin
}

// IsHuman reports a session of a person logged into the admin UI, as opposed
// to an agent authenticated by an api key.
func (s *Session) IsHuman() bool {
	return s.IsAuthorized() && s.ApiKeyId == ""
}
