package broker

import (
	"net/http"

	"github.com/rendau/ruto/internal/constant"
)

// endSession is the end session endpoint (OpenID Connect RP-Initiated
// Logout): the gateway's session cookies, the app tokens and this front's
// token are dropped, and the browser goes through the provider's logout to
// the front's post_logout_redirect_uri, checked like a login return address.
// The id_token_hint of the front (a ruto token) is not needed: the provider
// gets its own id token from the refresh cookie. The front tokens of other
// fronts stay, but without a session they serve nothing and expire.
func (u *unit) endSession(w http.ResponseWriter, r *http.Request, client string) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		methodNotAllowed(w, "GET, POST")
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "the request is not readable", http.StatusBadRequest)
		return
	}
	if id := r.Form.Get("client_id"); id != "" && id != client {
		http.Error(w, "client_id must be the client of this address", http.StatusBadRequest)
		return
	}
	redirect := r.Form.Get("post_logout_redirect_uri")
	if redirect == "" {
		redirect = u.publicUrl + "/"
	} else if !u.redirectAllowed(redirect) {
		http.Error(w, "post_logout_redirect_uri is not allowed: an absolute address on a CORS origin is required", http.StatusBadRequest)
		return
	}

	u.sessions.ClearCookie(w, constant.JwtFrontCookieName(u.name, client), constant.BrokerPath(u.name, client))
	u.sessions.Logout(w, r, redirect, r.Form.Get("state"))
}
