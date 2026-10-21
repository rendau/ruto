package oauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/rendau/ruto/internal/domain/oauth/model"
	oauthService "github.com/rendau/ruto/internal/domain/oauth/service"
	"github.com/rendau/ruto/internal/errs"
)

// OAuth error codes (RFC 6749, 5.2; RFC 7591, 3.2.2).
const (
	ErrInvalidRequest        = "invalid_request"
	ErrInvalidClient         = "invalid_client"
	ErrInvalidGrant          = "invalid_grant"
	ErrUnsupportedGrantType  = "unsupported_grant_type"
	ErrInvalidTarget         = "invalid_target"
	ErrInvalidRedirectUri    = "invalid_redirect_uri"
	ErrInvalidClientMetadata = "invalid_client_metadata"
	ErrTemporarilyUnavail    = "temporarily_unavailable"
)

// Error is an OAuth protocol error, rendered as {"error", "error_description"}.
type Error struct {
	Code        string
	Description string
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Description
}

// HTTPStatus is the status of the error response.
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case ErrInvalidClient:
		return http.StatusUnauthorized
	case ErrTemporarilyUnavail:
		return http.StatusServiceUnavailable
	}
	return http.StatusBadRequest
}

func newError(code, format string, args ...any) *Error {
	return &Error{Code: code, Description: fmt.Sprintf(format, args...)}
}

var errNotEnabled = &Error{Code: ErrTemporarilyUnavail, Description: "OAuth is not enabled on this server"}

type TokenReq struct {
	GrantType    string
	Code         string
	RedirectUri  string
	CodeVerifier string
	RefreshToken string
	Resource     string
	ClientId     string
	ClientSecret string
}

type TokenRep struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	Scope        string
}

// Token is the token endpoint: exchanges a code (with its PKCE verifier) or a
// refresh token for a new access + refresh token pair. Codes and refresh
// tokens are single-use.
func (u *Usecase) Token(ctx context.Context, req *TokenReq) (*TokenRep, error) {
	if !u.conf.Enabled {
		return nil, errNotEnabled
	}

	if err := u.authenticateClient(req.ClientId, req.ClientSecret); err != nil {
		return nil, err
	}
	if !u.resourceMatches(req.Resource) {
		return nil, newError(ErrInvalidTarget, "resource is not this server (%s)", u.conf.Issuer)
	}

	var (
		result *TokenRep
		grant  *model.Grant
	)
	err := u.txm.TxFn(ctx, func(ctx context.Context) error {
		var err error

		switch req.GrantType {
		case grantTypeAuthorizationCode:
			grant, err = u.takeCode(ctx, req)
		case grantTypeRefreshToken:
			grant, err = u.takeRefresh(ctx, req)
		default:
			return newError(ErrUnsupportedGrantType, "grant_type must be authorization_code or refresh_token")
		}
		if err != nil {
			return err
		}

		if _, err = u.grantSession(ctx, grant); err != nil {
			if errors.Is(err, errs.NotAuthorized) {
				return newError(ErrInvalidGrant, "the user of this connection is not an active administrator")
			}
			return err
		}

		if err = u.svc.MarkGrantConnected(ctx, grant.Id); err != nil {
			return fmt.Errorf("svc.MarkGrantConnected: %w", err)
		}

		tokens, err := u.svc.IssueTokens(ctx, grant.Id, map[string]time.Duration{
			model.TokenKindAccess:  u.conf.AccessTTL,
			model.TokenKindRefresh: u.conf.RefreshTTL,
		})
		if err != nil {
			return fmt.Errorf("svc.IssueTokens: %w", err)
		}

		result = &TokenRep{
			AccessToken:  tokens[model.TokenKindAccess],
			RefreshToken: tokens[model.TokenKindRefresh],
			ExpiresIn:    int64(u.conf.AccessTTL.Seconds()),
			Scope:        Scope,
		}

		return nil
	})
	if err != nil {
		if oauthErr, ok := errors.AsType[*Error](err); ok {
			return nil, oauthErr
		}
		return nil, err
	}

	slog.Info("oauth: tokens issued",
		"grant_type", req.GrantType,
		"grant_id", grant.Id,
		"client_name", grant.ClientName,
		"usr_id", grant.UsrId,
		"access_ttl", u.conf.AccessTTL.String(),
	)

	return result, nil
}

func (u *Usecase) takeCode(ctx context.Context, req *TokenReq) (*model.Grant, error) {
	if req.Code == "" || req.CodeVerifier == "" {
		return nil, newError(ErrInvalidRequest, "code and code_verifier are required")
	}

	item, found, err := u.svc.TakeToken(ctx, req.Code, model.TokenKindCode)
	if err != nil {
		return nil, fmt.Errorf("svc.TakeToken: %w", err)
	}
	if !found {
		return nil, newError(ErrInvalidGrant, "the code is invalid, expired or already used")
	}

	grant, err := u.clientGrant(ctx, item.GrantId, req.ClientId)
	if err != nil {
		return nil, err
	}
	if req.RedirectUri != grant.RedirectUri {
		return nil, newError(ErrInvalidGrant, "redirect_uri differs from the authorization request")
	}
	if !oauthService.VerifyPKCE(grant.CodeChallenge, req.CodeVerifier) {
		return nil, newError(ErrInvalidGrant, "code_verifier does not match the code_challenge")
	}

	return grant, nil
}

func (u *Usecase) takeRefresh(ctx context.Context, req *TokenReq) (*model.Grant, error) {
	if req.RefreshToken == "" {
		return nil, newError(ErrInvalidRequest, "refresh_token is required")
	}

	item, found, err := u.svc.TakeToken(ctx, req.RefreshToken, model.TokenKindRefresh)
	if err != nil {
		return nil, fmt.Errorf("svc.TakeToken: %w", err)
	}
	if !found {
		return nil, newError(ErrInvalidGrant, "the refresh token is invalid, expired or already used")
	}

	return u.clientGrant(ctx, item.GrantId, req.ClientId)
}

// clientGrant loads the grant of a token and checks it was issued to the
// client presenting the token.
func (u *Usecase) clientGrant(ctx context.Context, grantId, clientId string) (*model.Grant, error) {
	grant, found, err := u.svc.GetGrant(ctx, grantId, false)
	if err != nil {
		return nil, fmt.Errorf("svc.GetGrant: %w", err)
	}
	if !found {
		return nil, newError(ErrInvalidGrant, "the connection was removed")
	}
	if grant.ClientId != clientId {
		return nil, newError(ErrInvalidGrant, "the token was issued to another client")
	}
	return grant, nil
}

// Revoke is token revocation (RFC 7009): revoking any token of a connection
// removes the whole connection. Unknown tokens are not an error.
func (u *Usecase) Revoke(ctx context.Context, token, clientId, clientSecret string) error {
	if !u.conf.Enabled {
		return errNotEnabled
	}

	if err := u.authenticateClient(clientId, clientSecret); err != nil {
		return err
	}
	if token == "" {
		return newError(ErrInvalidRequest, "token is required")
	}

	item, found, err := u.svc.FindAnyToken(ctx, token)
	if err != nil {
		return fmt.Errorf("svc.FindAnyToken: %w", err)
	}
	if !found {
		return nil
	}

	grant, found, err := u.svc.GetGrant(ctx, item.GrantId, false)
	if err != nil {
		return fmt.Errorf("svc.GetGrant: %w", err)
	}
	if !found || grant.ClientId != clientId {
		return nil
	}

	if err = u.svc.DeleteGrant(ctx, grant.Id); err != nil {
		return fmt.Errorf("svc.DeleteGrant: %w", err)
	}

	slog.Info("oauth: connection revoked by the client", "grant_id", grant.Id, "client_name", grant.ClientName, "kind", item.Kind)

	return nil
}

// authenticateClient checks the client_id and, for a confidential client, its
// secret. A secret presented by a public client must still be right.
func (u *Usecase) authenticateClient(clientId, clientSecret string) error {
	client, ok := u.clientCodec.Decode(clientId)
	if !ok {
		return newError(ErrInvalidClient, "unknown client_id")
	}
	if clientSecret == "" && client.AuthMethod == model.ClientAuthNone {
		return nil
	}
	if !u.clientCodec.VerifySecret(clientId, clientSecret) {
		return newError(ErrInvalidClient, "client authentication failed")
	}
	return nil
}
