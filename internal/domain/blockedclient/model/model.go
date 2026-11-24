package model

import (
	"time"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

const (
	// SourceAdmin — a person logged into the admin UI (JWT)
	SourceAdmin = "admin"
	// SourceAgent — a caller with an api key or OAuth grant: an MCP agent or a
	// service such as pulse calling the REST API
	SourceAgent = "agent"

	// RemovedSourceKeyDeleted — lifted because its api key was deleted; the
	// other removed sources are SourceAdmin and SourceAgent, "" — closed after
	// expiry
	RemovedSourceKeyDeleted = "key_deleted"
)

// BlockedClient is a client address or network the gateways answer 403 on
// every route. Ip is the masked prefix in CIDR notation ("203.0.113.7/32").
type BlockedClient struct {
	Id            string
	Ip            string
	Reason        string
	CreatedBy     string
	Source        string
	ApiKeyId      string // the key or OAuth grant of an agent block
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ExpiresAt     *time.Time // nil = without end
	RemovedAt     *time.Time
	RemovedBy     string
	RemoveReason  string
	RemovedSource string
}

func (m *BlockedClient) IsActive(now time.Time) bool {
	return m.RemovedAt == nil && (m.ExpiresAt == nil || m.ExpiresAt.After(now))
}

type ListReq struct {
	commonModel.ListParams

	Ip *string
	// ActiveAt keeps blocks active at that moment; InactiveAt — the others
	ActiveAt   *time.Time
	InactiveAt *time.Time
	// NotRemoved keeps blocks not lifted, expired ones included
	NotRemoved bool

	Source        *string
	ApiKeyId      *string
	CreatedAfter  *time.Time
	RemovedSource *string
	RemovedAfter  *time.Time
}

type Edit struct {
	Reason    *string
	UpdatedAt *time.Time
	Source    *string
	// SetExpiresAt writes ExpiresAt, nil included (= without end)
	SetExpiresAt bool
	ExpiresAt    *time.Time

	RemovedAt     *time.Time
	RemovedBy     *string
	RemoveReason  *string
	RemovedSource *string
}

// RemoveMany lifts every active agent block, or those of one api key.
type RemoveMany struct {
	ApiKeyId      *string
	Now           time.Time
	RemovedBy     string
	RemoveReason  string
	RemovedSource string
}

// Rule is an active block as the gateways get it.
type Rule struct {
	Ip        string
	ExpiresAt *time.Time
}

type Stats struct {
	Active int64
	// ByAgentsSince — blocks made by agents (source agent) since the given time,
	// lifted and expired ones included
	ByAgentsSince int64
}
