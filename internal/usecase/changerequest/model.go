package changerequest

import (
	"time"

	"github.com/rendau/ruto/internal/domain/changerequest/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type Config struct {
	// PendingTTL: a request nobody decided within it expires.
	PendingTTL time.Duration
	// Retention: closed requests are deleted this long after the decision.
	Retention time.Duration
	// MaxPendingPerKey caps undecided requests of one api key.
	MaxPendingPerKey int64
}

type ProposeReq struct {
	Title       string
	Description string
	Publish     bool
	Ops         []*ProposeOp
}

// ProposeOp is model.Op plus the version of the target the agent based the
// change on.
type ProposeOp struct {
	model.Op
	Version string
}

// Detail is a request with its operations rendered for review.
type Detail struct {
	*model.ChangeRequest

	AuthorName  string
	DecidedName string
	// Outdated: an object the request touches changed since the proposal,
	// so it can no longer be applied.
	Outdated       bool
	OutdatedReason string
	Ops            []*OpView
}

// OpView and Change are stored with a decided request (model.Result.Review),
// hence the json tags.
type OpView struct {
	Op         string `json:"op"`
	TargetKind string `json:"target_kind"`
	TargetId   string `json:"target_id"`
	// Title names the target for people: an app name, "GET /v1/x" or "root".
	Title   string    `json:"title"`
	AppName string    `json:"app_name"`
	Changes []*Change `json:"changes"`
	Notes   []string  `json:"notes"`
	// SeenPaths are the report rows a delete_seen_paths removes.
	SeenPaths []*SeenPathView `json:"seen_paths,omitempty"`
}

type SeenPathView struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	Hits         int64  `json:"hits"`
	HitsNotFound int64  `json:"hits_not_found"`
	Sample       string `json:"sample"`
}

type Change struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
	Secret bool   `json:"secret"`
}

func listOnlyCount() commonModel.ListParams {
	return commonModel.ListParams{OnlyCount: true, WithTotalCount: true}
}
