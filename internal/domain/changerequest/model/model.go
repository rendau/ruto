package model

import (
	"encoding/json"
	"time"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

const (
	StatusPending   = "pending"
	StatusApplied   = "applied"
	StatusRejected  = "rejected"
	StatusCancelled = "cancelled" // withdrawn by the agent
	StatusExpired   = "expired"   // nobody decided within the TTL
	StatusOutdated  = "outdated"  // a touched object changed after the proposal
)

const (
	OpCreateApp      = "create_app"
	OpUpdateApp      = "update_app"
	OpDeleteApp      = "delete_app"
	OpCreateEndpoint = "create_endpoint"
	OpUpdateEndpoint = "update_endpoint"
	OpDeleteEndpoint = "delete_endpoint"
	OpUpdateRoot     = "update_root"
	// OpDeleteSeenPaths removes rows of an app's seen paths report, e.g. the
	// paths the same request registers as exact endpoints.
	OpDeleteSeenPaths = "delete_seen_paths"
)

// ChangeRequest is a batch of configuration changes proposed by an agent.
// Nothing is written to the configuration until an admin applies it; then
// all operations run in one transaction.
type ChangeRequest struct {
	Id          int64
	Status      string
	Title       string
	Description string
	Publish     bool
	Plan        Plan

	CreatedAt   time.Time
	AuthorUsrId int64
	ApiKeyId    string
	ApiKeyName  string

	DecidedAt    *time.Time
	DecidedUsrId int64
	Comment      string
	Error        string
	Result       Result

	PublishedAt  *time.Time
	PublishError string
}

type Plan struct {
	Ops []*Op `json:"ops"`
	// Versions are the fingerprints, at proposal time, of every object the
	// request reads or writes ("app:<id>" → version). Apply refuses to run if
	// any of them changed since.
	Versions map[string]string `json:"versions"`
}

// Op is one operation. Data is the full desired object; secret positions
// hold value sources (see package secretref), never secret values.
type Op struct {
	Op string `json:"op"`
	// Ref names an app created by this request ("$payments") so later
	// operations can use it as App.
	Ref string `json:"ref,omitempty"`
	// Id is the target of update/delete operations.
	Id string `json:"id,omitempty"`
	// App is the owner of a new endpoint: an app id or a Ref.
	App  string          `json:"app,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// SeenPathsData is the data of delete_seen_paths. App is pinned to an app id
// when the request is proposed.
type SeenPathsData struct {
	App  string        `json:"app"`
	Rows []SeenPathRow `json:"rows"`
}

// SeenPathRow is a report row as list_seen_paths shows it; the wildcard
// endpoint that counted it is not needed.
type SeenPathRow struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Result struct {
	// Created maps refs and op indexes ("#3") to ids of created objects.
	Created map[string]string `json:"created,omitempty"`
	// Review is the rendered diff as it was when the request was decided;
	// after that the live configuration no longer shows what changed.
	Review json.RawMessage `json:"review,omitempty"`
}

type ListReq struct {
	commonModel.ListParams

	Status   *string
	ApiKeyId *string
}

type Edit struct {
	Status       *string
	DecidedAt    *time.Time
	DecidedUsrId *int64
	Comment      *string
	Error        *string
	Result       *Result
	PublishedAt  *time.Time
	PublishError *string
}

// PendingStats counts requests waiting for a decision: Stuck — of them created
// before the stuck threshold, OldestAt — when the oldest one was created.
type PendingStats struct {
	Count    int
	Stuck    int
	OldestAt *time.Time
}

func (m *ChangeRequest) IsClosed() bool {
	return m.Status != StatusPending
}
