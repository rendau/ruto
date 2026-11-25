package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiKeyModel "github.com/rendau/ruto/internal/domain/apikey/model"
	apiKeyService "github.com/rendau/ruto/internal/domain/apikey/service"
	blockedClientModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
	blockedClientService "github.com/rendau/ruto/internal/domain/blockedclient/service"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	sessionService "github.com/rendau/ruto/internal/domain/session/service"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/internal/errs"
	"github.com/rendau/ruto/internal/infra/clientip"
	usecaseApiKey "github.com/rendau/ruto/internal/usecase/apikey"
	usecaseBlockedClient "github.com/rendau/ruto/internal/usecase/blockedclient"
	usecaseOAuth "github.com/rendau/ruto/internal/usecase/oauth"
)

const (
	testKey          = "rtk_test-key-for-mcp"
	testBlocklistKey = "rtk_test-key-blocklist-only"
)

type testKeySvc struct{}

func (s *testKeySvc) List(context.Context, *apiKeyModel.ListReq) ([]*apiKeyModel.ApiKey, int64, error) {
	return nil, 0, nil
}
func (s *testKeySvc) Get(context.Context, string, bool) (*apiKeyModel.ApiKey, bool, error) {
	return nil, false, nil
}
func (s *testKeySvc) GetByKey(_ context.Context, key string) (*apiKeyModel.ApiKey, bool, error) {
	switch key {
	case testKey:
		return &apiKeyModel.ApiKey{Id: "key-1", UsrId: 1, Name: "night-watch", Scope: apiKeyModel.ScopeFull, KeyHash: apiKeyService.HashKey(key)}, true, nil
	case testBlocklistKey:
		return &apiKeyModel.ApiKey{Id: "key-2", UsrId: 1, Name: "pulse", Scope: apiKeyModel.ScopeBlocklist, KeyHash: apiKeyService.HashKey(key)}, true, nil
	}
	return nil, false, nil
}
func (s *testKeySvc) Create(context.Context, *apiKeyModel.ApiKey) (string, error) { return "", nil }
func (s *testKeySvc) Delete(context.Context, string) error                        { return nil }
func (s *testKeySvc) TouchLastUsed(context.Context, string) error                 { return nil }

type testUsrSvc struct{}

func (s *testUsrSvc) Get(_ context.Context, id int64, _ bool) (*usrModel.Usr, bool, error) {
	return &usrModel.Usr{Id: id, Name: "Ivan", Active: true, IsAdmin: true}, true, nil
}

// testBlockSvc is a minimal in-memory block list: the guards themselves are
// tested in usecase/blockedclient.
type testBlockSvc struct {
	items []*blockedClientModel.BlockedClient
}

func (s *testBlockSvc) List(_ context.Context, pars *blockedClientModel.ListReq) ([]*blockedClientModel.BlockedClient, int64, error) {
	items := lo.Filter(s.items, func(v *blockedClientModel.BlockedClient, _ int) bool {
		switch {
		case pars.Ip != nil && v.Ip != *pars.Ip:
			return false
		case pars.ActiveAt != nil && !v.IsActive(*pars.ActiveAt):
			return false
		case pars.InactiveAt != nil && v.IsActive(*pars.InactiveAt):
			return false
		case pars.NotRemoved && v.RemovedAt != nil:
			return false
		}
		return true
	})
	return items, int64(len(items)), nil
}

func (s *testBlockSvc) Get(_ context.Context, id string, _ bool) (*blockedClientModel.BlockedClient, bool, error) {
	for _, v := range s.items {
		if v.Id == id {
			return v, true, nil
		}
	}
	return nil, false, errs.ObjectNotFound
}

func (s *testBlockSvc) GetNotRemoved(ctx context.Context, ip string) (*blockedClientModel.BlockedClient, bool, error) {
	items, _, _ := s.List(ctx, &blockedClientModel.ListReq{Ip: &ip, NotRemoved: true})
	if len(items) == 0 {
		return nil, false, nil
	}
	return items[0], true, nil
}

func (s *testBlockSvc) CountActive(context.Context, time.Time, *string) (int64, error) {
	return int64(len(s.items)), nil
}

func (s *testBlockSvc) CountCreatedSince(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}

func (s *testBlockSvc) ListLiftedByAdmin(context.Context, time.Time) ([]*blockedClientModel.BlockedClient, error) {
	return nil, nil
}

func (s *testBlockSvc) RemoveMany(context.Context, *blockedClientModel.RemoveMany) (int64, error) {
	return 0, nil
}

func (s *testBlockSvc) Create(_ context.Context, obj *blockedClientModel.BlockedClient) (string, error) {
	item := new(*obj)
	item.Id = strconv.Itoa(len(s.items) + 1)
	item.UpdatedAt = item.CreatedAt
	s.items = append(s.items, item)
	return item.Id, nil
}

func (s *testBlockSvc) Update(_ context.Context, id string, e *blockedClientModel.Edit) error {
	for _, v := range s.items {
		if v.Id == id && e.RemovedAt != nil {
			v.RemovedAt, v.RemovedBy, v.RemoveReason = e.RemovedAt, *e.RemovedBy, *e.RemoveReason
		}
	}
	return nil
}

func (s *testBlockSvc) Lock(context.Context) error                            { return nil }
func (s *testBlockSvc) CloseExpired(context.Context, string, time.Time) error { return nil }
func (s *testBlockSvc) Prune(context.Context, time.Time) (int64, error)       { return 0, nil }
func (s *testBlockSvc) Active(ctx context.Context, now time.Time) ([]*blockedClientModel.Rule, string, error) {
	items, _, _ := s.List(ctx, &blockedClientModel.ListReq{ActiveAt: &now})
	rules := lo.Map(items, func(v *blockedClientModel.BlockedClient, _ int) *blockedClientModel.Rule {
		return &blockedClientModel.Rule{Ip: v.Ip, ExpiresAt: v.ExpiresAt}
	})
	return rules, blockedClientService.Version(rules), nil
}

type testTxm struct{}

func (t *testTxm) TxFn(ctx context.Context, f func(context.Context) error) error { return f(ctx) }

type testRootSvc struct{}

func (r *testRootSvc) Get(context.Context) (*rootModel.Root, error) {
	return &rootModel.Root{TrustedClients: []rootModel.RootTrustedClient{{Name: "office", Ip: "198.51.100.0/24"}}}, nil
}

type testNotifier struct{ n int }

func (n *testNotifier) NotifyBlocklist() { n.n++ }

type headerTransport struct{ key string }

func (t *headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+t.key)
	return http.DefaultTransport.RoundTrip(r)
}

func newTestSession(t *testing.T, key string) (*mcpsdk.ClientSession, *testBlockSvc, *testNotifier) {
	sessionSvc := sessionService.New("test-secret")
	blocks := &testBlockSvc{}
	notifier := &testNotifier{}

	h := &Handler{
		sessionSvc:      sessionSvc,
		apiKeyUc:        usecaseApiKey.New(&testKeySvc{}, &testUsrSvc{}, sessionSvc, nil),
		oauthUc:         usecaseOAuth.New(usecaseOAuth.Config{}, nil, nil, nil, sessionSvc, nil),
		blockedClientUc: usecaseBlockedClient.New(blocks, &testTxm{}, sessionSvc, &testRootSvc{}, &testUsrSvc{}, notifier),
		clientIP:        clientip.New(nil, nil),
	}
	srv := httptest.NewServer(h.HTTPHandler())
	t.Cleanup(srv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:             srv.URL,
		HTTPClient:           &http.Client{Transport: &headerTransport{key: key}},
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	return cs, blocks, notifier
}

func callTool[T any](t *testing.T, cs *mcpsdk.ClientSession, name string, args map[string]any) (T, string) {
	var out T
	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	if res.IsError {
		text := ""
		for _, c := range res.Content {
			if tc, ok := c.(*mcpsdk.TextContent); ok {
				text += tc.Text
			}
		}
		return out, text
	}
	raw, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &out))
	return out, ""
}

func TestBlocklistTools(t *testing.T) {
	cs, blocks, notifier := newTestSession(t, testKey)

	tools, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := lo.Map(tools.Tools, func(v *mcpsdk.Tool, _ int) string { return v.Name })
	assert.Subset(t, names, []string{"list_blocked_clients", "block_client", "unblock_client"})
	for _, tool := range tools.Tools {
		if tool.Name == "block_client" {
			assert.Contains(t, tool.Description, "IMMEDIATELY")
			assert.Contains(t, tool.Description, "403")
			assert.Contains(t, tool.Description, "trusted_clients")
		}
	}

	t.Run("default duration is 24h", func(t *testing.T) {
		out, errText := callTool[BlockClientOut](t, cs, "block_client", map[string]any{"ip": "203.0.113.7", "reason": "ssh brute force"})
		require.Empty(t, errText)
		assert.Equal(t, "203.0.113.7/32", out.Block.Ip)
		assert.Equal(t, blockedClientModel.SourceAgent, out.Block.Source)
		assert.Equal(t, "Ivan (key night-watch)", out.Block.CreatedBy)
		assert.True(t, out.Block.Active)
		require.NotNil(t, blocks.items[0].ExpiresAt)
		assert.WithinDuration(t, time.Now().Add(24*time.Hour), *blocks.items[0].ExpiresAt, time.Minute)
		assert.Equal(t, 1, notifier.n)
	})

	t.Run("guards reach the agent", func(t *testing.T) {
		_, errText := callTool[BlockClientOut](t, cs, "block_client", map[string]any{"ip": "10.0.0.1", "reason": "x"})
		assert.Contains(t, errText, "private or special range")
		_, errText = callTool[BlockClientOut](t, cs, "block_client", map[string]any{"ip": "198.51.100.9", "reason": "x"})
		assert.Contains(t, errText, `trusted client "office"`)
		_, errText = callTool[BlockClientOut](t, cs, "block_client", map[string]any{"ip": "203.0.113.8", "reason": "x", "duration": "169h"})
		assert.Contains(t, errText, "duration")
		_, errText = callTool[BlockClientOut](t, cs, "block_client", map[string]any{"ip": "203.0.113.8", "reason": "x", "duration": "a day"})
		assert.Contains(t, errText, "Go duration")
		_, errText = callTool[BlockClientOut](t, cs, "block_client", map[string]any{"ip": "203.0.113.8"})
		assert.Contains(t, errText, "reason", "the schema requires it")
		_, errText = callTool[BlockClientOut](t, cs, "block_client", map[string]any{"ip": "203.0.113.8", "reason": " "})
		assert.Contains(t, errText, "reason is required")
		assert.Len(t, blocks.items, 1)
	})

	t.Run("list", func(t *testing.T) {
		out, errText := callTool[BlockedClientListOut](t, cs, "list_blocked_clients", map[string]any{})
		require.Empty(t, errText)
		assert.Equal(t, 1, out.ActiveCount)
		require.Len(t, out.Blocks, 1)
		assert.NotEmpty(t, out.Version)
	})

	t.Run("unblock by ip", func(t *testing.T) {
		_, errText := callTool[UnblockClientOut](t, cs, "unblock_client", map[string]any{"ip": "203.0.113.7", "reason": ""})
		assert.Contains(t, errText, "reason is required")

		out, errText := callTool[UnblockClientOut](t, cs, "unblock_client", map[string]any{"ip": "203.0.113.7", "reason": "false positive"})
		require.Empty(t, errText)
		assert.False(t, out.Block.Active)
		assert.Equal(t, "Ivan (key night-watch)", out.Block.RemovedBy)
		assert.Equal(t, 2, notifier.n)
	})

	t.Run("a person's block stays", func(t *testing.T) {
		blocks.items = append(blocks.items, &blockedClientModel.BlockedClient{Id: "99", Ip: "203.0.113.50/32", Source: blockedClientModel.SourceAdmin})
		_, errText := callTool[UnblockClientOut](t, cs, "unblock_client", map[string]any{"id": "99", "reason": "x"})
		assert.Contains(t, errText, "only a person can lift it")
		assert.Nil(t, blocks.items[1].RemovedAt)
	})
}

// A "blocklist" key sees and calls only the block list tools; a full key —
// everything.
func TestKeyScope(t *testing.T) {
	toolNames := func(cs *mcpsdk.ClientSession) []string {
		tools, err := cs.ListTools(context.Background(), nil)
		require.NoError(t, err)
		return lo.Map(tools.Tools, func(v *mcpsdk.Tool, _ int) string { return v.Name })
	}

	full, _, _ := newTestSession(t, testKey)
	assert.Subset(t, toolNames(full), []string{"get_root", "propose_changes", "block_client"})

	limited, _, _ := newTestSession(t, testBlocklistKey)
	assert.ElementsMatch(t, []string{"list_blocked_clients", "block_client", "unblock_client"}, toolNames(limited))
	assert.Contains(t, limited.InitializeResult().Instructions, "only manage the block list")

	for _, name := range []string{"get_root", "propose_changes", "list_change_requests"} {
		res, err := limited.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: map[string]any{}})
		if err == nil {
			assert.True(t, res.IsError, name)
		}
	}

	out, errText := callTool[BlockClientOut](t, limited, "block_client", map[string]any{"ip": "203.0.113.7", "reason": "scan"})
	require.Empty(t, errText)
	assert.Equal(t, "Ivan (key pulse)", out.Block.CreatedBy)
	assert.Equal(t, "key-2", out.Block.ApiKeyId)
}

// toolCtx refuses a tool outside the scope even if it were registered.
func TestToolCtxScope(t *testing.T) {
	sessionSvc := sessionService.New("test-secret")
	h := &Handler{
		sessionSvc: sessionSvc,
		apiKeyUc:   usecaseApiKey.New(&testKeySvc{}, &testUsrSvc{}, sessionSvc, nil),
	}
	s := &sessionServer{h: h, principal: "key-2", scope: apiKeyModel.ScopeBlocklist}
	req := func(name string) *mcpsdk.CallToolRequest {
		return &mcpsdk.CallToolRequest{
			Params: &mcpsdk.CallToolParamsRaw{Name: name},
			Extra:  &mcpsdk.RequestExtra{Header: http.Header{"Authorization": {"Bearer " + testBlocklistKey}}},
		}
	}

	_, err := s.toolCtx(context.Background(), req("get_root"))
	assert.ErrorIs(t, err, errScope)
	_, err = s.toolCtx(context.Background(), req("block_client"))
	assert.NoError(t, err)
}

func TestParseBlockDuration(t *testing.T) {
	d, err := parseBlockDuration(" ")
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, d)

	d, err = parseBlockDuration("90m")
	require.NoError(t, err)
	assert.Equal(t, 90*time.Minute, d)

	_, err = parseBlockDuration("7 days")
	assert.Error(t, err)
}
