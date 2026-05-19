package model

type Session struct {
	Id      int64
	IsAdmin bool
}

func New(id int64) *Session {
	return &Session{Id: id}
}
