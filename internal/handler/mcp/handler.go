// Package mcp is the built-in MCP (Model Context Protocol) server of core for
// AI agents: streamable HTTP on its own port (MCP_ENABLED / MCP_PORT),
// authenticated by api keys or, with MCP_OAUTH_ENABLED, OAuth access tokens
// (clients that connect by URL, such as Claude connectors).
//
// Security model: an agent reads the configuration with every secret masked
// (see package secretref) and never writes it directly. Every change is
// proposed as a change request that a person applies in the admin UI. The one
// exception is the block list of client addresses (block_client,
// unblock_client): it applies at once, within the guards of
// usecase/blockedclient.
package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	apiKeyModel "github.com/rendau/ruto/internal/domain/apikey/model"
	oauthService "github.com/rendau/ruto/internal/domain/oauth/service"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	sessionService "github.com/rendau/ruto/internal/domain/session/service"
	"github.com/rendau/ruto/internal/infra/clientip"
	usecaseApiKey "github.com/rendau/ruto/internal/usecase/apikey"
	usecaseApp "github.com/rendau/ruto/internal/usecase/app"
	usecaseBlockedClient "github.com/rendau/ruto/internal/usecase/blockedclient"
	usecaseChangeRequest "github.com/rendau/ruto/internal/usecase/changerequest"
	usecaseEndpoint "github.com/rendau/ruto/internal/usecase/endpoint"
	usecaseGateway "github.com/rendau/ruto/internal/usecase/gateway"
	usecaseMonitoring "github.com/rendau/ruto/internal/usecase/monitoring"
	usecaseOAuth "github.com/rendau/ruto/internal/usecase/oauth"
	usecaseRoot "github.com/rendau/ruto/internal/usecase/root"
	usecaseSeenOrigin "github.com/rendau/ruto/internal/usecase/seenorigin"
	usecaseSeenPath "github.com/rendau/ruto/internal/usecase/seenpath"
	usecaseSnapshot "github.com/rendau/ruto/internal/usecase/snapshot"
)

const (
	serverVersion = "1.0.0"

	sessionIdleTimeout = time.Hour

	serverInstructions = `ruto is an API gateway. Configuration: root (gateway-wide defaults) → apps (a backend with a path_prefix) → endpoints (method + path routed to the app's backend). auth and variables are inherited root → app → endpoint.

Rules:
1. You never change the configuration directly. propose_changes creates a change request; a person reviews it in the ruto admin UI and applies or rejects it. Tell the user the request id and that it waits for approval. Check the outcome with get_change_request (status pending / applied / rejected with a comment / outdated / expired / cancelled).
2. Secrets are never shown to you. Basic-auth passwords, api keys, variable values, backend header and query-param values appear as {"secret_ref": "...", "chars": N}. Do not ask the user for secret values and do not invent them.
3. Writing secret positions (the same fields) in propose_changes:
   - leave the {"secret_ref": ...} object as you read it → the value is kept;
   - put a secret_ref object read elsewhere → that value is copied there (e.g. reuse an app's TOKEN variable in another app);
   - {"generate": {"format": "alnum|digits|hex|base64url|uuid", "length": 32}, "name": "x"} → a new random value; {"reuse": "x"} puts the same generated value elsewhere in the request;
   - a plain string → that literal value (only for non-secret data such as hosts or ids).
   A variable can be used in other fields as {{NAME}}; the gateway substitutes it.
4. update_* operations take the full object (as returned by get_*, edited) and the version you read it with. If the object changed meanwhile the proposal fails: read it again.
5. One request is applied as a whole or not at all. Group related changes (an app and its endpoints) into one request; a new app can be referenced by later operations through its ref ("$name").
6. Publishing makes changes reach the gateways. Set publish=true to ask for publishing right after the request is applied; the person decides. get_publish_status shows the published version, whether there are unpublished changes and which gateways already run the latest version.
7. The block list of client addresses is the one exception to rule 1: block_client and unblock_client apply IMMEDIATELY, without a change request or publishing — a blocked client gets 403 on every route within seconds. Block only on clear evidence (e.g. a scan or brute force seen in logs or metrics), always with a reason and the shortest sensible duration (default 24h, at most 168h). You cannot block private or special ranges (10/8, 192.168/16, 127/8, fc00::/7 ...), networks wider than /24 (IPv4) or /48 (IPv6), anything overlapping a trusted client (root trusted_clients) or a network a person unblocked in the last 7 days; you can lift only blocks made by agents; all agents together may make at most 60 new blocks per hour and hold at most 500. Trusted clients (the whitelist) change only through propose_changes update_root and wait for a person.`
)

type Handler struct {
	sessionSvc *sessionService.Service

	apiKeyUc        *usecaseApiKey.Usecase
	oauthUc         *usecaseOAuth.Usecase
	changeRequestUc *usecaseChangeRequest.Usecase
	rootUc          *usecaseRoot.Usecase
	appUc           *usecaseApp.Usecase
	endpointUc      *usecaseEndpoint.Usecase
	seenPathUc      *usecaseSeenPath.Usecase
	seenOriginUc    *usecaseSeenOrigin.Usecase
	snapshotUc      *usecaseSnapshot.Usecase
	gatewayUc       *usecaseGateway.Usecase
	monitoringUc    *usecaseMonitoring.Usecase
	blockedClientUc *usecaseBlockedClient.Usecase

	clientIP *clientip.Resolver
}

func New(
	sessionSvc *sessionService.Service,
	apiKeyUc *usecaseApiKey.Usecase,
	oauthUc *usecaseOAuth.Usecase,
	changeRequestUc *usecaseChangeRequest.Usecase,
	rootUc *usecaseRoot.Usecase,
	appUc *usecaseApp.Usecase,
	endpointUc *usecaseEndpoint.Usecase,
	seenPathUc *usecaseSeenPath.Usecase,
	seenOriginUc *usecaseSeenOrigin.Usecase,
	snapshotUc *usecaseSnapshot.Usecase,
	gatewayUc *usecaseGateway.Usecase,
	monitoringUc *usecaseMonitoring.Usecase,
	blockedClientUc *usecaseBlockedClient.Usecase,
	clientIP *clientip.Resolver,
) *Handler {
	return &Handler{
		sessionSvc:      sessionSvc,
		apiKeyUc:        apiKeyUc,
		oauthUc:         oauthUc,
		changeRequestUc: changeRequestUc,
		rootUc:          rootUc,
		appUc:           appUc,
		endpointUc:      endpointUc,
		seenPathUc:      seenPathUc,
		seenOriginUc:    seenOriginUc,
		snapshotUc:      snapshotUc,
		gatewayUc:       gatewayUc,
		monitoringUc:    monitoringUc,
		blockedClientUc: blockedClientUc,
		clientIP:        clientIP,
	}
}

type ctxKeyT int

const (
	// ctxKeyPrincipal holds the id of the api key or OAuth grant of a request.
	ctxKeyPrincipal ctxKeyT = iota
	// ctxKeyScope holds the scope of that key (apiKeyModel.Scope*).
	ctxKeyScope
	// ctxKeyClientIP holds the caller's address the access log found.
	ctxKeyClientIP
)

// blocklistTools are all a key with scope "blocklist" may call.
var blocklistTools = map[string]bool{
	"list_blocked_clients": true,
	"block_client":         true,
	"unblock_client":       true,
}

const blocklistInstructions = `ruto is an API gateway. This key may only manage the block list of client addresses: list_blocked_clients, block_client, unblock_client. They apply IMMEDIATELY, without a change request or publishing — a blocked client gets 403 on every route within seconds. Block only on clear evidence (e.g. a scan or brute force seen in logs or metrics), always with a reason and the shortest sensible duration (default 24h, at most 168h). You cannot block private or special ranges, networks wider than /24 (IPv4) or /48 (IPv6), anything overlapping a trusted client, or a network a person unblocked in the last 7 days; you can lift only blocks made by agents. All agents together may make at most 60 new blocks per hour and hold at most 500.`

// HTTPHandler serves streamable HTTP MCP, authenticating every request, and,
// with OAuth enabled, the OAuth endpoints next to it.
func (h *Handler) HTTPHandler() http.Handler {
	streamable := mcpsdk.NewStreamableHTTPHandler(func(r *http.Request) *mcpsdk.Server {
		principal, _ := r.Context().Value(ctxKeyPrincipal).(string)
		if principal == "" {
			return nil
		}
		scope, _ := r.Context().Value(ctxKeyScope).(string)
		return h.newSessionServer(principal, scope)
	}, &mcpsdk.StreamableHTTPOptions{
		SessionTimeout: sessionIdleTimeout,
	})

	var oauth *oauthEndpoints
	if h.oauthUc.Config().Enabled {
		oauth = newOAuthEndpoints(h.oauthUc)
	}

	mcpHandler := h.authMiddleware(streamable, oauth)
	if oauth == nil {
		return h.withAccessLog(mcpHandler)
	}

	mux := http.NewServeMux()
	oauth.register(mux)
	mux.Handle("/", mcpHandler)

	return h.withAccessLog(mux)
}

func (h *Handler) authMiddleware(next http.Handler, oauth *oauthEndpoints) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credential := bearerToken(r.Header.Get("Authorization"))
		if credential == "" {
			unauthorized(w, oauth, "", "ruto mcp: an api key or OAuth token is required (Authorization: Bearer ...)")
			return
		}

		session, scope, err := h.authenticate(r.Context(), credential)
		if err != nil {
			slog.Warn("mcp: credential rejected", "credential", credentialKind(r), "client_ip", clientIP(r), "error", err)
			unauthorized(w, oauth, "invalid_token", "ruto mcp: invalid api key or token")
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyPrincipal, session.ApiKeyId)
		ctx = context.WithValue(ctx, ctxKeyScope, scope)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticate resolves an api key (rtk_) or an OAuth access token (rto_)
// into a session and its scope; OAuth connections have the full scope.
func (h *Handler) authenticate(ctx context.Context, credential string) (*sessionModel.Session, string, error) {
	if strings.HasPrefix(credential, oauthService.AccessTokenPrefix) {
		session, err := h.oauthUc.SessionFromToken(ctx, credential)
		return session, apiKeyModel.ScopeFull, err
	}
	return h.apiKeyUc.Authenticate(ctx, credential)
}

// unauthorized answers 401; with OAuth enabled it points the client to the
// protected resource metadata (RFC 9728), which starts the OAuth flow.
func unauthorized(w http.ResponseWriter, oauth *oauthEndpoints, oauthErr, msg string) {
	if oauth != nil {
		value := fmt.Sprintf(`Bearer resource_metadata=%q, scope=%q`, oauth.resourceMetadataURL(), usecaseOAuth.Scope)
		if oauthErr != "" {
			value += fmt.Sprintf(`, error=%q`, oauthErr)
		}
		w.Header().Set("WWW-Authenticate", value)
	}
	http.Error(w, msg, http.StatusUnauthorized)
}

func bearerToken(headerValue string) string {
	parts := strings.Fields(headerValue)
	switch {
	case len(parts) == 1:
		return parts[0]
	case len(parts) == 2 && strings.EqualFold(parts[0], "bearer"):
		return parts[1]
	}
	return ""
}

// sessionServer serves one MCP session, bound to the api key or OAuth grant
// it was opened with. A grant, not a token: access tokens rotate on refresh
// while the session goes on.
type sessionServer struct {
	h         *Handler
	principal string
	scope     string
}

// newSessionServer: a "blocklist" key sees only the block list tools (and
// toolCtx refuses any other call); every other key — all of them.
func (h *Handler) newSessionServer(principal, scope string) *mcpsdk.Server {
	s := &sessionServer{h: h, principal: principal, scope: scope}
	blocklistOnly := scope == apiKeyModel.ScopeBlocklist

	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "ruto",
		Title:   "ruto API gateway",
		Version: serverVersion,
	}, &mcpsdk.ServerOptions{
		Instructions: lo.Ternary(blocklistOnly, blocklistInstructions, serverInstructions),
	})

	if !blocklistOnly {
		s.registerReadTools(srv)
		s.registerChangeTools(srv)
	}
	s.registerBlocklistTools(srv)

	return srv
}

// toolCtx re-authenticates the credential of every tool call (a session id
// alone must not grant access) and puts the owner's fresh session into ctx.
func (s *sessionServer) toolCtx(ctx context.Context, req *mcpsdk.CallToolRequest) (context.Context, error) {
	credential := ""
	if req != nil && req.Extra != nil && req.Extra.Header != nil {
		credential = bearerToken(req.Extra.Header.Get("Authorization"))
	}
	if credential == "" {
		return nil, errKeyMismatch
	}

	session, scope, err := s.h.authenticate(ctx, credential)
	if err != nil {
		return nil, errKeyInvalid
	}
	if session.ApiKeyId != s.principal || scope != s.scope {
		return nil, errKeyMismatch
	}
	if scope == apiKeyModel.ScopeBlocklist && (req.Params == nil || !blocklistTools[req.Params.Name]) {
		return nil, errScope
	}

	return s.h.sessionSvc.WithContext(ctx, session), nil
}
