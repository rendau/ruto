package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	blockedClientModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
	"github.com/rendau/ruto/internal/errs"
	usecaseBlockedClient "github.com/rendau/ruto/internal/usecase/blockedclient"
)

// maxListedBlocks keeps list_blocked_clients within an agent's context.
const maxListedBlocks = 500

const blocklistGuards = `Guards for agents: no private or special ranges (10/8, 172.16/12, 192.168/16, 127/8, 169.254/16, 100.64/10, 0/8, multicast and reserved, ::1, fc00::/7, fe80::/10, ff00::/8 and IPv4-mapped forms) — internal systems live there; no network wider than /24 (IPv4) or /48 (IPv6); nothing overlapping a trusted client (root trusted_clients, the whitelist — it changes only through propose_changes update_root and waits for a person); no network overlapping one a person unblocked in the last 7 days (the error says when and why). All agents together (every key) may make at most 60 new blocks per rolling hour and hold at most 500 active blocks.`

func (s *sessionServer) registerBlocklistTools(srv *mcpsdk.Server) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_blocked_clients",
		Description: "The gateway-wide block list of client addresses: active blocks (blocked clients get 403 on every route), newest first; with_history=true adds lifted and expired ones. Each row: id, ip (normalized network), reason, created_by, source (admin = a person, agent = an api key: MCP or a service like pulse), expires_at (empty = without end), removed_*. Also the current block list version (gateways report the one they apply in get_publish_status).",
	}, s.listBlockedClients)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "block_client",
		Description: `Block a client address or network on every gateway route. Applies IMMEDIATELY — no change request, no publishing: within seconds the client gets 403 on every route of every app. Use it against clear abuse (a scan, brute force, a flood seen in logs or metrics), never on a guess.

ip: an address or a CIDR network (203.0.113.7, 203.0.113.0/24, 2001:db8:1::/48). reason (required): what you saw, kept in the history. duration: a Go duration from "1m" to "168h", default "24h"; the block ends by itself.
Blocking a network that already has an active block prolongs it: it ends at the later of the two times (but at most 7 days after the block was first made) and the reason is replaced; a block is never shortened this way. If a person made or confirmed that block, nothing changes and the block is returned as it is.

` + blocklistGuards,
	}, s.blockClient)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "unblock_client",
		Description: "Lift an active block at once (the gateways let the client in within seconds); the row stays in the history. Address it by id (from list_blocked_clients) or by ip (the exact network of an active block). reason is required. You can lift only blocks made by agents (source agent): a block a person made or confirmed is lifted by a person in the ruto admin UI.",
	}, s.unblockClient)
}

type BlockedClientOut struct {
	Id           string `json:"id"`
	Ip           string `json:"ip"`
	Reason       string `json:"reason"`
	CreatedBy    string `json:"created_by"`
	Source       string `json:"source"`
	ApiKeyId     string `json:"api_key_id,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	RemovedAt    string `json:"removed_at,omitempty"`
	RemovedBy    string `json:"removed_by,omitempty"`
	RemoveReason string `json:"remove_reason,omitempty"`
	Active       bool   `json:"active"`
}

func blockedClientOut(v *blockedClientModel.BlockedClient, _ int) BlockedClientOut {
	return BlockedClientOut{
		Id:           v.Id,
		Ip:           v.Ip,
		Reason:       v.Reason,
		CreatedBy:    v.CreatedBy,
		Source:       v.Source,
		ApiKeyId:     v.ApiKeyId,
		CreatedAt:    v.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    v.UpdatedAt.Format(time.RFC3339),
		ExpiresAt:    timeOut(v.ExpiresAt),
		RemovedAt:    timeOut(v.RemovedAt),
		RemovedBy:    v.RemovedBy,
		RemoveReason: v.RemoveReason,
		Active:       v.IsActive(time.Now()),
	}
}

type BlockedClientListIn struct {
	WithHistory bool `json:"with_history,omitempty" jsonschema:"also return lifted and expired blocks"`
}

type BlockedClientListOut struct {
	Version     string             `json:"version"`
	ActiveCount int                `json:"active_count"`
	Truncated   bool               `json:"truncated,omitempty"`
	Blocks      []BlockedClientOut `json:"blocks"`
}

func (s *sessionServer) listBlockedClients(ctx context.Context, req *mcpsdk.CallToolRequest, in BlockedClientListIn) (*mcpsdk.CallToolResult, BlockedClientListOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, BlockedClientListOut{}, err
	}

	items, err := s.h.blockedClientUc.List(ctx, in.WithHistory)
	if err != nil {
		return nil, BlockedClientListOut{}, toolErr(err)
	}
	version, err := s.h.blockedClientUc.GetVersion(ctx)
	if err != nil {
		return nil, BlockedClientListOut{}, toolErr(err)
	}

	now := time.Now()
	result := BlockedClientListOut{
		Version:     version,
		ActiveCount: lo.CountBy(items, func(v *blockedClientModel.BlockedClient) bool { return v.IsActive(now) }),
	}
	if len(items) > maxListedBlocks {
		items = items[:maxListedBlocks]
		result.Truncated = true
	}
	result.Blocks = lo.Map(items, blockedClientOut)

	return nil, result, nil
}

type BlockClientIn struct {
	Ip       string `json:"ip" jsonschema:"an address or CIDR network, e.g. 203.0.113.7 or 203.0.113.0/24"`
	Reason   string `json:"reason" jsonschema:"what you saw: kept in the history and shown to people"`
	Duration string `json:"duration,omitempty" jsonschema:"Go duration from 1m to 168h, default 24h"`
}

type BlockClientOut struct {
	Block   BlockedClientOut `json:"block"`
	Message string           `json:"message"`
}

func (s *sessionServer) blockClient(ctx context.Context, req *mcpsdk.CallToolRequest, in BlockClientIn) (*mcpsdk.CallToolResult, BlockClientOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, BlockClientOut{}, err
	}

	duration, err := parseBlockDuration(in.Duration)
	if err != nil {
		return nil, BlockClientOut{}, toolErr(err)
	}

	item, err := s.h.blockedClientUc.Create(ctx, &usecaseBlockedClient.CreateReq{
		Ip:       in.Ip,
		Reason:   in.Reason,
		Duration: duration,
	})
	if err != nil {
		return nil, BlockClientOut{}, toolErr(err)
	}

	return nil, BlockClientOut{
		Block: blockedClientOut(item, 0),
		Message: fmt.Sprintf("%s is blocked on every gateway route until %s; the gateways apply it within seconds. Nothing else to approve.",
			item.Ip, lo.CoalesceOrEmpty(timeOut(item.ExpiresAt), "it is lifted (no end)")),
	}, nil
}

func parseBlockDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return usecaseBlockedClient.AgentDefaultDuration, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, errs.ErrFull{Err: errs.InvalidRequest, Desc: `duration: a Go duration such as "30m", "24h", "168h"`}
	}
	return d, nil
}

type UnblockClientIn struct {
	Id     string `json:"id,omitempty" jsonschema:"id of the block (list_blocked_clients)"`
	Ip     string `json:"ip,omitempty" jsonschema:"or the exact network of an active block, e.g. 203.0.113.7"`
	Reason string `json:"reason" jsonschema:"why the block is lifted"`
}

type UnblockClientOut struct {
	Block   BlockedClientOut `json:"block"`
	Message string           `json:"message"`
}

func (s *sessionServer) unblockClient(ctx context.Context, req *mcpsdk.CallToolRequest, in UnblockClientIn) (*mcpsdk.CallToolResult, UnblockClientOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, UnblockClientOut{}, err
	}

	item, err := s.h.blockedClientUc.Remove(ctx, &usecaseBlockedClient.RemoveReq{
		Id:     in.Id,
		Ip:     in.Ip,
		Reason: in.Reason,
	})
	if err != nil {
		return nil, UnblockClientOut{}, toolErr(err)
	}

	return nil, UnblockClientOut{
		Block:   blockedClientOut(item, 0),
		Message: item.Ip + " is unblocked; the gateways let it in within seconds.",
	}, nil
}
