package config_client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/goccy/go-json"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	currentVersion string
}

func New(
	globalCtx context.Context,
	address string,
	onConfig configCallback,
) (*Service, error) {
	var err error

	service := &Service{
		globalCtx: globalCtx,
		onConfig:  onConfig,
	}

	if address != "" {
		service.conn, err = grpc.NewClient(
			address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, fmt.Errorf("grpc.NewClient: %w", err)
		}
		service.client = ruto_v1.NewSnapshotClient(service.conn)
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
	serverVersion, err := s.fetchVersion()
	if err != nil {
		slog.Error("config-client: fetchVersion failed", "error", err)
		return
	}
	if serverVersion == "" || serverVersion == s.currentVersion {
		return
	}

	err = s.fetchAndApplyConfig()
	if err != nil {
		slog.Error("config-client: fetchAndApplyConfig failed", "error", err, "version", serverVersion)
		return
	}

	slog.Info("config-client: config applied", "version", serverVersion)

	s.currentVersion = serverVersion
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
