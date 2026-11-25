package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	apikeyService "github.com/rendau/ruto/internal/domain/apikey/service"
	oauthService "github.com/rendau/ruto/internal/domain/oauth/service"
	"github.com/rendau/ruto/internal/infra/clientip"
)

// rpcPeekMaxBytes bounds how much of a JSON-RPC body is read ahead to log its
// method; a larger body is passed on untouched and logged without it.
const rpcPeekMaxBytes = 64 * 1024

// statusRecorder keeps the response status for the access log. Unwrap lets
// http.ResponseController reach Flush of the streamed (SSE) responses.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

// withAccessLog logs every request of the MCP port. Clients reach it through
// Cloudflare and traefik, so the caller's address comes from their headers,
// believed only from TRUSTED_PROXY_ADDRESSES and Cloudflare (see
// clientip.Resolver); cf_ray matches the request in Cloudflare security
// events. Credentials are never logged, only their kind.
func (h *Handler) withAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()

		if ip := h.clientIP.ClientIP(clientip.HTTPRequest(r)); ip.IsValid() {
			r = r.WithContext(context.WithValue(r.Context(), ctxKeyClientIP, ip.String()))
		}
		rpcMethod := peekRPCMethod(r)
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		slog.Info("mcp: request",
			"method", r.Method,
			"path", r.URL.Path,
			"rpc_method", rpcMethod,
			"status", status,
			"duration_ms", time.Since(started).Milliseconds(),
			"credential", credentialKind(r),
			"mcp_session", r.Header.Get("Mcp-Session-Id") != "",
			"client_ip", clientIP(r),
			"cf_ray", r.Header.Get("Cf-Ray"),
			"user_agent", r.UserAgent(),
		)
	})
}

// peekRPCMethod reads the JSON-RPC method (or methods of a batch) of a POST
// and puts the read bytes back in front of the body.
func peekRPCMethod(r *http.Request) string {
	if r.Method != http.MethodPost || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return ""
	}

	head, err := io.ReadAll(io.LimitReader(r.Body, rpcPeekMaxBytes+1))
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head), r.Body), r.Body}
	if err != nil || len(head) > rpcPeekMaxBytes {
		return ""
	}

	type rpcMessage struct {
		Method string `json:"method"`
	}

	head = bytes.TrimSpace(head)
	if bytes.HasPrefix(head, []byte("[")) {
		var batch []rpcMessage
		if json.Unmarshal(head, &batch) != nil {
			return ""
		}
		methods := make([]string, 0, len(batch))
		for _, m := range batch {
			methods = append(methods, m.Method)
		}
		return strings.Join(methods, ",")
	}

	var msg rpcMessage
	if json.Unmarshal(head, &msg) != nil {
		return ""
	}
	return msg.Method
}

func credentialKind(r *http.Request) string {
	credential := bearerToken(r.Header.Get("Authorization"))
	switch {
	case credential == "":
		return "none"
	case strings.HasPrefix(credential, oauthService.AccessTokenPrefix):
		return "oauth"
	case strings.HasPrefix(credential, apikeyService.KeyPrefix):
		return "api_key"
	}
	return "unknown"
}

// clientIP is the caller's address the access log found, for the logs further
// down the request; empty when it is unknown.
func clientIP(r *http.Request) string {
	v, _ := r.Context().Value(ctxKeyClientIP).(string)
	return v
}
