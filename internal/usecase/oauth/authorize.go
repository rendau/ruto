package oauth

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/rendau/ruto/internal/domain/oauth/model"
	oauthService "github.com/rendau/ruto/internal/domain/oauth/service"
	"github.com/rendau/ruto/internal/errs"
)

const (
	grantTypeAuthorizationCode = "authorization_code"
	grantTypeRefreshToken      = "refresh_token"

	responseTypeCode = "code"

	codeChallengeMethodS256 = "S256"

	clientNameMaxLen  = 100
	redirectUrisMax   = 10
	redirectUriMaxLen = 2000
	defaultClientName = "MCP client"
)

var (
	supportedGrantTypes    = []string{grantTypeAuthorizationCode, grantTypeRefreshToken}
	supportedResponseTypes = []string{responseTypeCode}
	supportedAuthMethods   = []string{model.ClientAuthNone, model.ClientAuthSecretPost, model.ClientAuthSecretBasic}
)

type RegisterReq struct {
	RedirectUris            []string
	ClientName              string
	TokenEndpointAuthMethod string
	GrantTypes              []string
	ResponseTypes           []string
}

type RegisterRep struct {
	ClientId      string
	ClientSecret  string // empty for a public client
	IssuedAt      time.Time
	Client        *model.Client
	GrantTypes    []string
	ResponseTypes []string
}

// Register is dynamic client registration (RFC 7591). It is open by design (a
// client registers before anyone signs in), so it only admits redirect URIs
// from the configured list: a code can then reach only a known client.
func (u *Usecase) Register(_ context.Context, req *RegisterReq) (*RegisterRep, error) {
	if !u.conf.Enabled {
		return nil, errNotEnabled
	}

	if len(req.RedirectUris) == 0 || len(req.RedirectUris) > redirectUrisMax {
		return nil, newError(ErrInvalidRedirectUri, "redirect_uris must list 1 to %d URIs", redirectUrisMax)
	}
	for _, uri := range req.RedirectUris {
		if len(uri) > redirectUriMaxLen || !oauthService.RedirectAllowed(u.conf.RedirectUris, uri) {
			return nil, newError(ErrInvalidRedirectUri, "redirect URI %q is not allowed by this server", uri)
		}
	}

	grantTypes := req.GrantTypes
	if len(grantTypes) == 0 {
		grantTypes = []string{grantTypeAuthorizationCode}
	}
	for _, v := range grantTypes {
		if !slices.Contains(supportedGrantTypes, v) {
			return nil, newError(ErrInvalidClientMetadata, "grant type %q is not supported", v)
		}
	}

	responseTypes := req.ResponseTypes
	if len(responseTypes) == 0 {
		responseTypes = supportedResponseTypes
	}
	for _, v := range responseTypes {
		if !slices.Contains(supportedResponseTypes, v) {
			return nil, newError(ErrInvalidClientMetadata, "response type %q is not supported", v)
		}
	}

	// RFC 7591: client_secret_basic when omitted.
	authMethod := req.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = model.ClientAuthSecretBasic
	}
	if !slices.Contains(supportedAuthMethods, authMethod) {
		return nil, newError(ErrInvalidClientMetadata, "token endpoint auth method %q is not supported", authMethod)
	}

	name := strings.TrimSpace(req.ClientName)
	if name == "" {
		name = defaultClientName
	}
	if len([]rune(name)) > clientNameMaxLen {
		name = string([]rune(name)[:clientNameMaxLen])
	}

	client := &model.Client{
		Name:         name,
		RedirectUris: req.RedirectUris,
		AuthMethod:   authMethod,
	}

	clientId, err := u.clientCodec.Encode(client)
	if err != nil {
		return nil, fmt.Errorf("clientCodec.Encode: %w", err)
	}

	result := &RegisterRep{
		ClientId:      clientId,
		IssuedAt:      time.Now(),
		Client:        client,
		GrantTypes:    grantTypes,
		ResponseTypes: responseTypes,
	}
	if authMethod != model.ClientAuthNone {
		result.ClientSecret = u.clientCodec.Secret(clientId)
	}

	return result, nil
}

type AuthorizeReq struct {
	ResponseType        string
	ClientId            string
	RedirectUri         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Scope               string
	Resource            string
}

type AuthorizeInfo struct {
	ClientName   string
	RedirectUri  string
	RedirectHost string
}

// DescribeAuthorization checks an authorization request for the consent page
// and describes the client to the admin.
func (u *Usecase) DescribeAuthorization(ctx context.Context, req *AuthorizeReq) (*AuthorizeInfo, error) {
	if _, err := u.requireHumanAdmin(ctx); err != nil {
		return nil, err
	}

	client, err := u.checkAuthorizeReq(req)
	if err != nil {
		return nil, err
	}

	redirect, _ := url.Parse(req.RedirectUri)

	return &AuthorizeInfo{
		ClientName:   client.Name,
		RedirectUri:  req.RedirectUri,
		RedirectHost: redirect.Host,
	}, nil
}

// Authorize records the admin's decision on the consent page and returns the
// URL to send the browser back to the client: with a code when approved,
// with error=access_denied otherwise.
func (u *Usecase) Authorize(ctx context.Context, req *AuthorizeReq, approve bool) (string, error) {
	session, err := u.requireHumanAdmin(ctx)
	if err != nil {
		return "", err
	}

	client, err := u.checkAuthorizeReq(req)
	if err != nil {
		return "", err
	}

	if !approve {
		return redirectWith(req.RedirectUri, map[string]string{
			"error": "access_denied",
			"state": req.State,
		}), nil
	}

	var code string
	err = u.txm.TxFn(ctx, func(ctx context.Context) error {
		grantId, err := u.svc.CreateGrant(ctx, &model.Grant{
			UsrId:         session.Id,
			ClientId:      req.ClientId,
			ClientName:    client.Name,
			RedirectUri:   req.RedirectUri,
			CodeChallenge: req.CodeChallenge,
		})
		if err != nil {
			return fmt.Errorf("svc.CreateGrant: %w", err)
		}

		code, err = u.svc.IssueToken(ctx, grantId, model.TokenKindCode, codeTTL)
		if err != nil {
			return fmt.Errorf("svc.IssueToken: %w", err)
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return redirectWith(req.RedirectUri, map[string]string{
		"code":  code,
		"state": req.State,
	}), nil
}

func (u *Usecase) checkAuthorizeReq(req *AuthorizeReq) (*model.Client, error) {
	if !u.conf.Enabled {
		return nil, errs.OAuthNotEnabled
	}

	client, ok := u.clientCodec.Decode(req.ClientId)
	if !ok {
		return nil, invalidAuthorize("unknown client_id: register the client again")
	}
	// The registered list is checked against the current config too, so a
	// URI removed from the config stops working for clients registered before.
	if !slices.Contains(client.RedirectUris, req.RedirectUri) ||
		!oauthService.RedirectAllowed(u.conf.RedirectUris, req.RedirectUri) {
		return nil, invalidAuthorize("redirect_uri is not registered for this client")
	}

	if req.ResponseType != responseTypeCode {
		return nil, invalidAuthorize("response_type must be code")
	}
	if req.CodeChallenge == "" || req.CodeChallengeMethod != codeChallengeMethodS256 {
		return nil, invalidAuthorize("PKCE is required: code_challenge with code_challenge_method S256")
	}
	if !u.resourceMatches(req.Resource) {
		return nil, invalidAuthorize(fmt.Sprintf("resource %q is not this server (%s)", req.Resource, u.conf.Issuer))
	}

	return client, nil
}

// resourceMatches checks the RFC 8707 resource indicator, when given, against
// the MCP server URL.
func (u *Usecase) resourceMatches(resource string) bool {
	if resource == "" {
		return true
	}
	return normalizeURL(resource) == normalizeURL(u.conf.Issuer)
}

func normalizeURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return value
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	return parsed.String()
}

func redirectWith(redirectUri string, params map[string]string) string {
	parsed, err := url.Parse(redirectUri)
	if err != nil {
		return redirectUri
	}
	query := parsed.Query()
	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func invalidAuthorize(desc string) error {
	return errs.ErrFull{Err: errs.OAuthInvalidRequest, Desc: desc}
}
