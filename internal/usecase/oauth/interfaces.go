package oauth

import (
	"context"
	"time"

	"github.com/rendau/ruto/internal/domain/oauth/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
)

type ServiceI interface {
	ListGrants(ctx context.Context, pars *model.GrantListReq) ([]*model.Grant, int64, error)
	GetGrant(ctx context.Context, id string, errNE bool) (*model.Grant, bool, error)
	CreateGrant(ctx context.Context, obj *model.Grant) (string, error)
	DeleteGrant(ctx context.Context, id string) error
	MarkGrantConnected(ctx context.Context, id string) error
	TouchGrantLastUsed(ctx context.Context, id string) error

	IssueToken(ctx context.Context, grantId, kind string, ttl time.Duration) (string, error)
	IssueTokens(ctx context.Context, grantId string, ttls map[string]time.Duration) (map[string]string, error)
	FindToken(ctx context.Context, token, kind string) (*model.Token, bool, error)
	FindAnyToken(ctx context.Context, token string) (*model.Token, bool, error)
	TakeToken(ctx context.Context, token, kind string) (*model.Token, bool, error)

	Cleanup(ctx context.Context, grantsCreatedBefore time.Time) (tokens, grants int64, err error)
}

type ClientCodecI interface {
	Encode(client *model.Client) (string, error)
	Decode(clientId string) (*model.Client, bool)
	Secret(clientId string) string
	VerifySecret(clientId, secret string) bool
}

type UsrServiceI interface {
	Get(ctx context.Context, id int64, errNE bool) (*usrModel.Usr, bool, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
}

type TransactionManagerI interface {
	TxFn(ctx context.Context, f func(context.Context) error) error
}
