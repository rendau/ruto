package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type httpRequest struct {
	r *http.Request
}

// HTTPRequest reads the addresses of a plain http request.
func HTTPRequest(r *http.Request) RequestI {
	return &httpRequest{r: r}
}

func (h *httpRequest) ExtractRemoteAddrIP() netip.Addr {
	return ParseAddr(h.r.RemoteAddr)
}

func (h *httpRequest) ExtractForwardedFor() []netip.Addr {
	return ParseForwardedFor(h.r.Header.Values("X-Forwarded-For"))
}

func (h *httpRequest) ExtractRealIP() netip.Addr {
	return ParseAddr(h.r.Header.Get("X-Real-Ip"))
}

func (h *httpRequest) ExtractCfConnectingIP() netip.Addr {
	return ParseAddr(h.r.Header.Get("Cf-Connecting-Ip"))
}

// ParseAddr reads an address with or without a port; invalid when it is
// neither.
func ParseAddr(raw string) netip.Addr {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return netip.Addr{}
	}
	if ip, err := netip.ParseAddr(raw); err == nil {
		return ip
	}
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		return netip.Addr{}
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}
	}
	return ip
}

// ParseForwardedFor gives the X-Forwarded-For addresses in order, across all
// the header lines.
func ParseForwardedFor(lines []string) []netip.Addr {
	var result []netip.Addr
	for _, line := range lines {
		for raw := range strings.SplitSeq(line, ",") {
			if raw = strings.TrimSpace(raw); raw == "" {
				continue
			}
			result = append(result, ParseAddr(raw))
		}
	}
	return result
}
