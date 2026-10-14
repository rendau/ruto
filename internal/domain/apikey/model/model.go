package model

import (
	"time"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

// ApiKey is a long-lived credential of the built-in MCP server. KeyHash is
// the sha256 of the key; the key itself is never stored.
type ApiKey struct {
	Id         string
	UsrId      int64
	Name       string
	KeyHash    string
	KeyPrefix  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

type ListReq struct {
	commonModel.ListParams

	UsrId   *int64
	KeyHash *string
}
