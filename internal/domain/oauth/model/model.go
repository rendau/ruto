package model

import (
	"time"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

const (
	TokenKindCode    = "code"
	TokenKindAccess  = "access"
	TokenKindRefresh = "refresh"
)

const (
	ClientAuthNone        = "none"
	ClientAuthSecretPost  = "client_secret_post"
	ClientAuthSecretBasic = "client_secret_basic"
)

// Client is a dynamically registered OAuth client. It is not stored: the
// client_id carries it, signed by core (see service.ClientCodec).
type Client struct {
	Name         string
	RedirectUris []string
	AuthMethod   string
}

// Grant is a connection of an OAuth client approved by an admin. Its tokens
// act like an api key of that admin.
type Grant struct {
	Id            string
	UsrId         int64
	ClientId      string
	ClientName    string
	RedirectUri   string
	CodeChallenge string
	CreatedAt     time.Time
	ConnectedAt   *time.Time
	LastUsedAt    *time.Time
}

type GrantListReq struct {
	commonModel.ListParams

	UsrId     *int64
	Connected *bool
}

// Token is an authorization code, access or refresh token of a grant. Hash is
// the sha256 of the token; the token itself is never stored.
type Token struct {
	Hash      string
	GrantId   string
	Kind      string
	ExpiresAt time.Time
}
