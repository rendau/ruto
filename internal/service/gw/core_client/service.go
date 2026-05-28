package core_client

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

const (
	CheckInterval = 10 * time.Second
)

type configCallback func(*rootModel.Root) error

type Service struct {
	globalCtx context.Context
	onConfig  configCallback

	conn           *grpc.ClientConn
	client         ruto_v1.SnapshotClient
	gatewayClient  ruto_v1.GatewayClient
	currentVersion string
	identity       *identity
	startedAtUnix  int64
	lastApplyAt    int64
	lastError      string
}

func New(
	globalCtx context.Context,
	address string,
	onConfig configCallback,
) (*Service, error) {
	var err error

	service := &Service{
		globalCtx:     globalCtx,
		onConfig:      onConfig,
		identity:      newIdentity(),
		startedAtUnix: time.Now().Unix(),
	}

	if address != "" {
		service.conn, err = grpc.NewClient(
			"dns:///"+address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(
				keepalive.ClientParameters{
					Time:                10 * time.Second,
					Timeout:             1 * time.Second,
					PermitWithoutStream: true,
				},
			),
		)
		if err != nil {
			return nil, fmt.Errorf("grpc.NewClient: %w", err)
		}
		service.client = ruto_v1.NewSnapshotClient(service.conn)
		service.gatewayClient = ruto_v1.NewGatewayClient(service.conn)
	}

	return service, nil
}

func (s *Service) Start() {
	if s.client != nil {
		go s.worker()
	}
}

func (s *Service) worker() {
	select {
	case <-s.globalCtx.Done():
		return
	case <-time.After(time.Second):
	}

	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for s.globalCtx.Err() == nil {
		s.refresh()

		select {
		case <-s.globalCtx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) refresh() {
	if err := s.sendHeartbeat(); err != nil {
		slog.Warn("core-client: heartbeat failed", "error", err)
	}

	serverVersion, err := s.fetchVersion()
	if err != nil {
		s.lastError = err.Error()
		slog.Error("config-client: fetchVersion failed", "error", err)
		return
	}
	if serverVersion == "" || serverVersion == s.currentVersion {
		return
	}

	err = s.fetchAndApplyConfig()
	if err != nil {
		s.lastError = err.Error()
		slog.Error("config-client: fetchAndApplyConfig failed", "error", err, "version", serverVersion)
		return
	}

	slog.Info("config-client: config applied", "version", serverVersion)

	s.currentVersion = serverVersion
	s.lastApplyAt = time.Now().Unix()
	s.lastError = ""
}

func (s *Service) fetchVersion() (string, error) {
	ctx, cancel := context.WithTimeout(s.globalCtx, 5*time.Second)
	defer cancel()

	versionRep, err := s.client.GetVersion(ctx, &emptypb.Empty{})
	if err != nil {
		if s.globalCtx.Err() == nil {
			return "", fmt.Errorf("client.GetVersion: %w", err)
		}
		return "", nil
	}

	return versionRep.GetVersion(), nil
}

func (s *Service) fetchAndApplyConfig() error {
	ctx, cancel := context.WithTimeout(s.globalCtx, 5*time.Second)
	defer cancel()

	configRep, err := s.client.Get(ctx, &emptypb.Empty{})
	if err != nil {
		if s.globalCtx.Err() == nil {
			return fmt.Errorf("client.Get: %w", err)
		}
		return nil
	}

	conf, err := decodeConfig(configRep)
	if err != nil {
		return fmt.Errorf("decodeConfig: %w", err)
	}

	err = s.onConfig(conf)
	if err != nil {
		return fmt.Errorf("onConfig: %w", err)
	}

	return nil
}

func (s *Service) sendHeartbeat() error {
	if s.gatewayClient == nil || s.identity == nil {
		return nil
	}

	lastError := strings.TrimSpace(s.lastError)
	if len(lastError) > 512 {
		lastError = lastError[:512]
	}

	ctx, cancel := context.WithTimeout(s.globalCtx, 3*time.Second)
	defer cancel()

	_, err := s.gatewayClient.Heartbeat(ctx, &ruto_v1.GatewayHeartbeatRequest{
		GatewayId:       s.identity.GatewayID,
		PodName:         s.identity.PodName,
		HostName:        s.identity.HostName,
		SnapshotVersion: s.currentVersion,
		LastApplyAtUnix: s.lastApplyAt,
		StartedAtUnix:   s.startedAtUnix,
		LastError:       lastError,
	})
	if err != nil && s.globalCtx.Err() == nil {
		return fmt.Errorf("gatewayClient.Heartbeat: %w", err)
	}

	return nil
}

func decodeConfig(rep *ruto_v1.SnapshotResponse) (*rootModel.Root, error) {
	if rep == nil || rep.Data == nil {
		return rootModel.NewEmpty(), nil
	}

	body, err := json.Marshal(rep.Data.AsMap())
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	result := new(rootModel.Root{})
	if err = json.Unmarshal(body, result); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return result, nil
}
