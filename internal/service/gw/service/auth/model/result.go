package model

// Result is the outcome of an auth check. Failures are ordered by precedence:
// when no method lets a request in, the highest one is answered.
//   - InvalidCredentials is on top: the client sent stale credentials and must
//     get 401 to renew them, even if another method denies it by ip.
//   - DeniedAuthenticated beats NoCredentials: the client has proven who it is,
//     and 401 would only make it renew a valid token in a loop.
//   - NoCredentials beats Denied: some method still lets the client in once it
//     presents credentials.
type Result int

const (
	Allowed Result = iota
	// Denied — refused whatever credentials the request carries (ip not allowed).
	Denied
	// NoCredentials — the credentials a method needs were not presented.
	NoCredentials
	// DeniedAuthenticated — the credentials are valid but grant no access.
	DeniedAuthenticated
	// InvalidCredentials — the credentials were presented but rejected.
	InvalidCredentials
)

func (r Result) Forbidden() bool {
	return r == Denied || r == DeniedAuthenticated
}
