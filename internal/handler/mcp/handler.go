// Package mcp is the built-in MCP (Model Context Protocol) server of core for
// AI agents: streamable HTTP on its own port (MCP_ENABLED / MCP_PORT),
// authenticated by api keys or, with MCP_OAUTH_ENABLED, OAuth access tokens
// (clients that connect by URL, such as Claude connectors).
//
// Security model: an agent reads the configuration with every secret masked
// (see package secretref) and never writes it directly. Every change is
// proposed as a change request that a person applies in the admin UI.
package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	oauthService "github.com/rendau/ruto/internal/domain/oauth/service"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	sessionService "github.com/rendau/ruto/internal/domain/session/service"
	usecaseApiKey "github.com/rendau/ruto/internal/usecase/apikey"
	usecaseApp "github.com/rendau/ruto/internal/usecase/app"
	usecaseChangeRequest "github.com/rendau/ruto/internal/usecase/changerequest"
	usecaseEndpoint "github.com/rendau/ruto/internal/usecase/endpoint"
	usecaseGateway "github.com/rendau/ruto/internal/usecase/gateway"
	usecaseMonitoring "github.com/rendau/ruto/internal/usecase/monitoring"
	usecaseOAuth "github.com/rendau/ruto/internal/usecase/oauth"
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
6. Publishing makes changes reach the gateways. Set publish=true to ask for publishing right after the request is applied; the person decides. get_publish_status shows the published version, whether there are unpublished changes and which gateways already run the latest version.`
)

type Handler struct {
	sessionSvc *sessionService.Service

	apiKeyUc        *usecaseApiKey.Usecase
	oauthUc         *usecaseOAuth.Usecase
	changeRequestUc *usecaseChangeRequest.Usecase
	appUc           *usecaseApp.Usecase
	endpointUc      *usecaseEndpoint.Usecase
	seenPathUc      *usecaseSeenPath.Usecase
	snapshotUc      *usecaseSnapshot.Usecase
	gatewayUc       *usecaseGateway.Usecase
	monitoringUc    *usecaseMonitoring.Usecase
}

func New(
	sessionSvc *sessionService.Service,
	apiKeyUc *usecaseApiKey.Usecase,
	oauthUc *usecaseOAuth.Usecase,
	changeRequestUc *usecaseChangeRequest.Usecase,
	appUc *usecaseApp.Usecase,
	endpointUc *usecaseEndpoint.Usecase,
	seenPathUc *usecaseSeenPath.Usecase,
	snapshotUc *usecaseSnapshot.Usecase,
	gatewayUc *usecaseGateway.Usecase,
	monitoringUc *usecaseMonitoring.Usecase,
) *Handler {
	return &Handler{
		sessionSvc:      sessionSvc,
		apiKeyUc:        apiKeyUc,
		oauthUc:         oauthUc,
		changeRequestUc: changeRequestUc,
		appUc:           appUc,
		endpointUc:      endpointUc,
		seenPathUc:      seenPathUc,
		snapshotUc:      snapshotUc,
		gatewayUc:       gatewayUc,
		monitoringUc:    monitoringUc,
	}
}

type ctxKeyT int

// ctxKeyPrincipal holds the id of the api key or OAuth grant of a request.
const ctxKeyPrincipal ctxKeyT = iota

// HTTPHandler serves streamable HTTP MCP, authenticating every request, and,
// with OAuth enabled, the OAuth endpoints next to it.
func (h *Handler) HTTPHandler() http.Handler {
	streamable := mcpsdk.NewStreamableHTTPHandler(func(r *http.Request) *mcpsdk.Server {
		principal, _ := r.Context().Value(ctxKeyPrincipal).(string)
		if principal == "" {
			return nil
		}
		return h.newSessionServer(principal)
	}, &mcpsdk.StreamableHTTPOptions{
		SessionTimeout: sessionIdleTimeout,
	})

	var oauth *oauthEndpoints
	if h.oauthUc.Config().Enabled {
		oauth = newOAuthEndpoints(h.oauthUc)
	}

	mcpHandler := h.authMiddleware(streamable, oauth)
	if oauth == nil {
		return mcpHandler
	}

	mux := http.NewServeMux()
	oauth.register(mux)
	mux.Handle("/", mcpHandler)

	return mux
}

func (h *Handler) authMiddleware(next http.Handler, oauth *oauthEndpoints) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credential := bearerToken(r.Header.Get("Authorization"))
		if credential == "" {
			unauthorized(w, oauth, "", "ruto mcp: an api key or OAuth token is required (Authorization: Bearer ...)")
			return
		}

		session, err := h.authenticate(r.Context(), credential)
		if err != nil {
			unauthorized(w, oauth, "invalid_token", "ruto mcp: invalid api key or token")
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyPrincipal, session.ApiKeyId)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticate resolves an api key (rtk_) or an OAuth access token (rto_).
func (h *Handler) authenticate(ctx context.Context, credential string) (*sessionModel.Session, error) {
	if strings.HasPrefix(credential, oauthService.AccessTokenPrefix) {
		return h.oauthUc.SessionFromToken(ctx, credential)
	}
	return h.apiKeyUc.SessionFromKey(ctx, credential)
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
}

func (h *Handler) newSessionServer(principal string) *mcpsdk.Server {
	s := &sessionServer{h: h, principal: principal}

	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "ruto",
		Title:   "ruto API gateway",
		Version: serverVersion,
	}, &mcpsdk.ServerOptions{
		Instructions: serverInstructions,
	})

	s.registerReadTools(srv)
	s.registerChangeTools(srv)

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

	session, err := s.h.authenticate(ctx, credential)
	if err != nil {
		return nil, errKeyInvalid
	}
	if session.ApiKeyId != s.principal {
		return nil, errKeyMismatch
	}

	return s.h.sessionSvc.WithContext(ctx, session), nil
}
