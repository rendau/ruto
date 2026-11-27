package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/errs"
)

// TokenTTL is the lifetime of an admin token and of the admin session cookie.
const TokenTTL = 12 * time.Hour

type contextKey string

const sessionContextKey = contextKey("session")

type Service struct {
	secret string
}

func New(secret string) *Service {
	return &Service{
		secret: strings.TrimSpace(secret),
	}
}

func (s *Service) WithContext(ctx context.Context, session *sessionModel.Session) context.Context {
	if session == nil {
		session = sessionModel.New(0)
	}
	return context.WithValue(ctx, sessionContextKey, session)
}

func (s *Service) FromContext(ctx context.Context) *sessionModel.Session {
	if ctx != nil {
		if session, ok := ctx.Value(sessionContextKey).(*sessionModel.Session); ok && session != nil {
			return session
		}
	}
	return sessionModel.New(0)
}

func (s *Service) CtxIsAuthorized(ctx context.Context) bool {
	return s.FromContext(ctx).IsAuthorized()
}

func (s *Service) CtxIsAdmin(ctx context.Context) bool {
	return s.FromContext(ctx).IsAdmin()
}

func (s *Service) CtxHasFullAppAccess(ctx context.Context) bool {
	sess := s.FromContext(ctx)
	return sess.IsAdmin() || (sess.IsAuthorized() && sess.AllApps)
}

func (s *Service) CtxGetAppIds(ctx context.Context) []string {
	return s.FromContext(ctx).AppIds
}

// ParseToken checks an admin token and returns the user it was issued to and
// the fingerprint of the password it was issued under. Rights are not read
// from the token: the caller builds the session from the live user record.
func (s *Service) ParseToken(tokenStr string) (int64, string, error) {
	if s.secret == "" {
		return 0, "", errs.InvalidConfig
	}

	claims := jwtv5.MapClaims{}
	parsedToken, err := jwtv5.ParseWithClaims(
		tokenStr,
		claims,
		func(_ *jwtv5.Token) (any, error) {
			return []byte(s.secret), nil
		},
		jwtv5.WithValidMethods([]string{jwtv5.SigningMethodHS256.Alg()}),
	)
	if err != nil || parsedToken == nil || !parsedToken.Valid {
		if err == nil {
			return 0, "", fmt.Errorf("fail to parse token")
		}
		return 0, "", err
	}

	usrRaw, ok := claims["id"]
	if !ok {
		return 0, "", fmt.Errorf("missing user id claim in token")
	}
	usrId, err := usrIDFromClaim(usrRaw)
	if err != nil {
		return 0, "", err
	}

	passwordVersion, _ := claims["pv"].(string)
	if passwordVersion == "" {
		return 0, "", fmt.Errorf("missing pv claim in token")
	}

	return usrId, passwordVersion, nil
}

func (s *Service) CreateToken(usrId int64, passwordVersion string) (string, error) {
	if s.secret == "" {
		return "", errs.InvalidConfig
	}

	now := time.Now().UTC()
	claims := jwtv5.MapClaims{
		"id":  usrId,
		"pv":  passwordVersion,
		"iat": now.Unix(),
		"exp": now.Add(TokenTTL).Unix(),
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", fmt.Errorf("token.SignedString: %w", err)
	}

	return tokenStr, nil
}

func usrIDFromClaim(v any) (int64, error) {
	switch x := v.(type) {
	case float64:
		return int64(x), nil
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case string:
		parsedId, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("strconv.ParseInt: %w", err)
		}
		return parsedId, nil
	default:
		return 0, fmt.Errorf("invalid user id claim")
	}
}
