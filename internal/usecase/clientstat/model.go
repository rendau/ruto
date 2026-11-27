package clientstat

import (
	"time"

	blockedClientModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
	"github.com/rendau/ruto/internal/domain/clientstat/model"
)

type TopReq struct {
	// 0 = DefaultRange
	Range time.Duration
	// "" = KindRequests
	Kind string
	// 0 = defaultLimit
	Limit int64
}

type TopRep struct {
	Items []*TopItem
	// Root rate-limit rules with key "ip", strictest first
	IpLimits []*IpLimit
	// the bucket of TopItem.RecentRequests
	RecentFrom time.Time
}

// IpLimit is a Root rate-limit rule with key "ip": it counts every request of
// a client through the whole gateway, as the client stats do, so a client's
// rate compares to it. App and endpoint rules count only a part of a client's
// requests and are left out.
type IpLimit struct {
	Name     string
	Requests int
	Period   time.Duration
	// requests per minute
	Rpm float64
}

// TopItem is a client of Top with the active block covering it, if any.
type TopItem struct {
	*model.Item
	Block *blockedClientModel.BlockedClient
}
