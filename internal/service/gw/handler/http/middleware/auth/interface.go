package auth

import (
	"net/http"
)

type authorizerI interface {
	Authorize(r *http.Request) bool
}
