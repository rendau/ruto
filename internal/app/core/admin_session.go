package core

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"google.golang.org/grpc/metadata"

	sessionService "github.com/rendau/ruto/internal/domain/session/service"
	"github.com/rendau/ruto/internal/errs"
	"github.com/rendau/ruto/internal/infra/clientip"
)

// The admin UI keeps its token in an HttpOnly cookie: scripts of the page
// (a browser extension, a bad npm dependency) cannot read it and carry it off.
// The cookie is sent to /api only; api keys and scripts keep using the
// Authorization header, which wins over the cookie.
const (
	adminSessionCookie = "ruto_session"
	adminSessionPath   = "/api"
)

// AdminLoginI checks a username and password and returns an admin token.
type AdminLoginI interface {
	Login(ctx context.Context, username, password string, clientIP netip.Addr) (string, error)
}

type adminLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NewAdminSessionHandler serves /api/session: POST signs in and sets the
// cookie, DELETE signs out and removes it.
func NewAdminSessionHandler(login AdminLoginI, resolver ClientIPResolverI) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sameOriginRequest(r) {
			writeSessionError(w, http.StatusForbidden, errs.NoPermission, "cross-site request")
			return
		}

		switch r.Method {
		case http.MethodPost:
			var req adminLoginReq
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
				writeSessionError(w, http.StatusBadRequest, errs.InvalidRequest, "bad request body")
				return
			}
			token, err := login.Login(r.Context(), req.Username, req.Password, resolver.ClientIP(clientip.HTTPRequest(r)))
			if err != nil {
				if errors.Is(err, errs.NotAuthorized) {
					writeSessionError(w, http.StatusUnauthorized, errs.NotAuthorized, "wrong username or password")
					return
				}
				if errors.Is(err, errs.TooManyAttempts) {
					writeSessionError(w, http.StatusTooManyRequests, errs.TooManyAttempts,
						"too many failed sign-in attempts, try again in 15 minutes")
					return
				}
				slog.Error("admin session: login", "error", err)
				writeSessionError(w, http.StatusInternalServerError, errs.ServiceNA, "login failed")
				return
			}
			setSessionCookie(w, token, int(sessionService.TokenTTL.Seconds()))
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			setSessionCookie(w, "", -1)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "POST, DELETE")
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookie,
		Value:    value,
		Path:     adminSessionPath,
		MaxAge:   maxAge,
		HttpOnly: true,
		// browsers accept Secure cookies on http://localhost too
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func writeSessionError(w http.ResponseWriter, httpStatus int, code errs.Err, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code.Error(), "message": message})
}

// sessionCookieToken is the admin token of a request authenticated by the
// cookie alone, "" when it carries an Authorization header or no cookie.
func sessionCookieToken(r *http.Request) string {
	if r.Header.Get("Authorization") != "" {
		return ""
	}
	cookie, err := r.Cookie(adminSessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// grpcGatewaySessionMetadata turns the cookie into the Authorization metadata
// the session interceptor reads.
func grpcGatewaySessionMetadata(_ context.Context, r *http.Request) metadata.MD {
	token := sessionCookieToken(r)
	if token == "" {
		return nil
	}
	return metadata.Pairs("authorization", "Bearer "+token)
}

// sameOriginRequest refuses a state-changing request from another origin.
// SameSite=Strict keeps other sites from sending the cookie, but every
// subdomain of the same registrable domain (any *.mdev.kz service) counts as
// the same site; browsers send Origin with every POST, PUT and DELETE.
func sameOriginRequest(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// NewSessionCSRFGuard refuses a state-changing request that is authenticated
// by the session cookie and comes from another origin.
func NewSessionCSRFGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sessionCookieToken(r) != "" && !sameOriginRequest(r) {
			writeSessionError(w, http.StatusForbidden, errs.NoPermission, "cross-site request")
			return
		}
		next.ServeHTTP(w, r)
	})
}
