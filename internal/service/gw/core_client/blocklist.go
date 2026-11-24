package core_client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

// blocklistWorker is the single goroutine that refreshes the block list: at
// start, every blocklistPoll and on blocklistTrigger. Being the only caller,
// it never runs two fetches at once.
func (s *Service) blocklistWorker() {
	ticker := time.NewTicker(s.blocklistPoll)
	defer ticker.Stop()

	for {
		s.refreshBlocklist()

		select {
		case <-s.globalCtx.Done():
			return
		case <-ticker.C:
		case <-s.blocklistTrigger:
		}
	}
}

// refreshBlocklist fetches the active list when its version differs from the
// applied one. On an error the applied list stays as it is.
func (s *Service) refreshBlocklist() {
	version, err := s.fetchBlocklistVersion()
	if err == nil && s.blocks.Applied() && version == s.blocks.Version() {
		s.blocklistReachable()
		return
	}

	var rep *ruto_v1.BlockedClientActiveRep
	if err == nil {
		rep, err = s.fetchBlocklist()
	}
	if err != nil {
		if s.globalCtx.Err() != nil {
			return
		}
		if !s.blocklistFailing {
			s.blocklistFailing = true
			slog.Warn("core-client: block list not refreshed, the last one stays", "error", err, "version", s.blocks.Version())
		}
		return
	}
	s.blocklistReachable()

	applied := lo.CoalesceOrEmpty(rep.GetVersion(), version)
	s.blocks.Set(applied, lo.Map(rep.GetRules(), decodeBlockedClientRule))
	slog.Info("core-client: block list applied", "version", applied, "rules", s.blocks.Len())

	s.triggerHeartbeat()
}

func (s *Service) blocklistReachable() {
	if s.blocklistFailing {
		s.blocklistFailing = false
		slog.Info("core-client: block list refreshed again")
	}
}

func (s *Service) fetchBlocklistVersion() (string, error) {
	ctx, cancel := context.WithTimeout(s.globalCtx, rpcTimeout)
	defer cancel()

	rep, err := s.blockedClient.GetVersion(ctx, &emptypb.Empty{})
	if err != nil {
		return "", fmt.Errorf("blockedClient.GetVersion: %w", err)
	}

	return rep.GetVersion(), nil
}

func (s *Service) fetchBlocklist() (*ruto_v1.BlockedClientActiveRep, error) {
	ctx, cancel := context.WithTimeout(s.globalCtx, rpcTimeout)
	defer cancel()

	rep, err := s.blockedClient.GetActive(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("blockedClient.GetActive: %w", err)
	}

	return rep, nil
}

func decodeBlockedClientRule(v *ruto_v1.BlockedClientRule, _ int) blocklist.Rule {
	return blocklist.Rule{
		Ip:            v.GetIp(),
		ExpiresAtUnix: v.GetExpiresAtUnix(),
	}
}
