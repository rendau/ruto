package model

type Session struct {
	Id    int64
	Admin bool
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
