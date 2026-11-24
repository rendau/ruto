package blockedclient

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/rendau/ruto/internal/domain/blockedclient/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/errs"
)

// The guards protect production from a confused or manipulated agent: an api
// key (an MCP agent, a service like pulse) blocks at once, without a person.
const (
	// MaxActive caps the active blocks made by people, MaxAgentActive — by
	// agents. They are separate so agents can never use up the room people
	// need: at most MaxActive + MaxAgentActive blocks in total (the gateways
	// hold the list in memory and check every request against it).
	MaxActive      = 10000
	MaxAgentActive = 500

	// AgentNewBlocksPerHour is the budget of new agent blocks per rolling
	// hour, shared by all keys (prolongs do not count).
	AgentNewBlocksPerHour = 60
	// AgentCooldownAfterLift — after a person lifts a block, agents may not
	// block an overlapping network for this long.
	AgentCooldownAfterLift = 7 * 24 * time.Hour
	// AgentMaxBlockAge — an agent cannot prolong its block past created_at
	// plus this; a new block after it expired is a new row.
	AgentMaxBlockAge = 7 * 24 * time.Hour

	AgentMinDuration     = time.Minute
	AgentMaxDuration     = 7 * 24 * time.Hour
	AgentDefaultDuration = 24 * time.Hour

	AdminMinDuration = time.Minute
	AdminMaxDuration = 10 * 365 * 24 * time.Hour

	// the widest network a caller may block, in prefix bits
	agentMinBits4 = 24
	agentMinBits6 = 48
	adminMinBits4 = 16
	adminMinBits6 = 32

	maxReasonLen = 500
)

// agentForbidden are private and special ranges: internal systems (1C and
// the like) live there, and blocking them by mistake stops the business. Only
// a person may block them. IPv4-mapped (::ffff:0:0/96), IPv4-compatible
// (::/96) and NAT64 (64:ff9b::/96) forms carry IPv4 addresses inside IPv6.
var agentForbidden = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.168.0.0/16", "224.0.0.0/4", "240.0.0.0/4",
	"::/96", "::ffff:0:0/96", "64:ff9b::/96", "fc00::/7", "fe80::/10", "ff00::/8",
)

func mustPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		result = append(result, netip.MustParsePrefix(v))
	}
	return result
}

func invalid(desc string) error {
	return errs.ErrFull{Err: errs.InvalidRequest, Desc: desc}
}

func noPermission(desc string) error {
	return errs.ErrFull{Err: errs.NoPermission, Desc: desc}
}

func checkReason(reason string, required bool) error {
	if required && reason == "" {
		return invalid("reason is required: say why, it is kept in the history")
	}
	if len([]rune(reason)) > maxReasonLen {
		return invalid(fmt.Sprintf("reason is longer than %d characters", maxReasonLen))
	}
	return nil
}

func checkDuration(source string, d time.Duration) error {
	if source == model.SourceAgent {
		if d < AgentMinDuration || d > AgentMaxDuration {
			return invalid(fmt.Sprintf("duration must be from %s to %s for an api key (a block without end is for admins only)", AgentMinDuration, AgentMaxDuration))
		}
		return nil
	}
	if d == 0 {
		return nil
	}
	if d < AdminMinDuration || d > AdminMaxDuration {
		return invalid(fmt.Sprintf("duration must be 0 (without end) or from %s to %s", AdminMinDuration, AdminMaxDuration))
	}
	return nil
}

func checkWidth(source string, prefix netip.Prefix) error {
	min4, min6 := adminMinBits4, adminMinBits6
	if source == model.SourceAgent {
		min4, min6 = agentMinBits4, agentMinBits6
	}
	minBits := min4
	if prefix.Addr().Is6() {
		minBits = min6
	}
	if prefix.Bits() < minBits {
		return invalid(fmt.Sprintf("network %s is too wide: at most /%d for IPv4 and /%d for IPv6", prefix, min4, min6))
	}
	return nil
}

func checkSpecial(source string, prefix netip.Prefix) error {
	if source != model.SourceAgent {
		return nil
	}
	for _, special := range agentForbidden {
		if special.Overlaps(prefix) {
			return noPermission(fmt.Sprintf("%s overlaps the private or special range %s: internal systems live there, only an admin can block it", prefix, special))
		}
	}
	return nil
}

// checkTrusted refuses a network that contains or lies inside a trusted
// client of the root configuration: trusted clients are never blocked. The
// stored root has no parsed Prefix, so each entry is parsed here.
func checkTrusted(root *rootModel.Root, prefix netip.Prefix) error {
	for _, tc := range root.TrustedClients {
		trusted, err := rootModel.ParseClientPrefix(tc.Ip)
		if err != nil {
			return fmt.Errorf("root trusted_clients %q: %w", tc.Ip, err)
		}
		if trusted.Overlaps(prefix) {
			return noPermission(fmt.Sprintf("%s overlaps the trusted client %q (%s): trusted clients are never blocked; "+
				"trusted_clients change only through a change request (update_root) that a person applies", prefix, tc.Name, trusted))
		}
	}
	return nil
}

// laterExpiry: nil (without end) is later than any time.
func laterExpiry(a, b *time.Time) *time.Time {
	if a == nil || b == nil {
		return nil
	}
	if b.After(*a) {
		return b
	}
	return a
}

func sameExpiry(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
