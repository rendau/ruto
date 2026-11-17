package apptoken

import (
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// SameSession tells whether a token got by exchange (an app or a front token
// kept in a cookie) still belongs to the login session of the shared token:
// the same person, and the same provider session when both tokens name one
// (sid, OpenID Connect Front-Channel Logout §3). The sid survives a refresh
// and changes with every login, so a token of a closed session or of an
// earlier login is not taken for the current one. A provider without sid
// binds by the person only.
func SameSession(shared, token jwtv5.MapClaims) bool {
	sub, _ := shared["sub"].(string)
	if got, _ := token["sub"].(string); sub == "" || got != sub {
		return false
	}
	sid, _ := shared["sid"].(string)
	got, _ := token["sid"].(string)
	return sid == "" || got == "" || got == sid
}
