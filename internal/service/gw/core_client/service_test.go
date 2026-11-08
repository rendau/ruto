package core_client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type fakeSnapshotClient struct {
	ruto_v1.SnapshotClient
	version, settingsVersion string
	settings                 *ruto_v1.GatewaySettings
}

func (c *fakeSnapshotClient) GetVersion(context.Context, *emptypb.Empty, ...grpc.CallOption) (*ruto_v1.SnapshotVersion, error) {
	return &ruto_v1.SnapshotVersion{Version: c.version, SettingsVersion: c.settingsVersion}, nil
}

func (c *fakeSnapshotClient) Get(context.Context, *emptypb.Empty, ...grpc.CallOption) (*ruto_v1.SnapshotResponse, error) {
	data, _ := structpb.NewStruct(map[string]any{"base_url": "https://api.example.test"})
	return &ruto_v1.SnapshotResponse{Data: data, Gateway: c.settings}, nil
}

func TestRefreshAppliesSettingsAndConfigSeparately(t *testing.T) {
	core := &fakeSnapshotClient{version: "v1"}
	var configs int
	var settings []snapshotModel.GatewaySettings
	svc, err := New(context.Background(), "",
		func(*rootModel.Root) error { configs++; return nil },
		func(s *snapshotModel.GatewaySettings) { settings = append(settings, *s) },
	)
	require.NoError(t, err)
	svc.client = core

	// a core without settings: the config only
	svc.refresh()
	assert.Equal(t, 1, configs)
	assert.Empty(t, settings)

	// the settings change alone: no rebuild of the handlers
	core.settingsVersion = "s1"
	core.settings = &ruto_v1.GatewaySettings{RedisAddr: "redis:6379", RedisPassword: "p", RedisDb: 2}
	svc.refresh()
	assert.Equal(t, 1, configs)
	require.Len(t, settings, 1)
	assert.Equal(t, snapshotModel.GatewaySettings{RedisAddr: "redis:6379", RedisPassword: "p", RedisDB: 2}, settings[0])

	// nothing changed
	svc.refresh()
	assert.Equal(t, 1, configs)
	assert.Len(t, settings, 1)

	// the config changes alone
	core.version = "v2"
	svc.refresh()
	assert.Equal(t, 2, configs)
	assert.Len(t, settings, 1)

	// the settings are dropped in core
	core.settingsVersion = ""
	core.settings = nil
	svc.refresh()
	require.Len(t, settings, 2)
	assert.Equal(t, snapshotModel.GatewaySettings{}, settings[1])
}
