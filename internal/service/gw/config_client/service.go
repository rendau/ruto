package config_client

import (
	"context"
	"fmt"
	"time"

	"github.com/goccy/go-json"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type configCallback func(*rootModel.Root) error

type Service struct {
	globalCtx context.Context
	onConfig  configCallback

	conn    *grpc.ClientConn
	client  ruto_v1.SnapshotClient
	version string
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
}

func (s *Service) sleepOrDone(d time.Duration) {
	select {
	case <-time.After(d):
	case <-s.globalCtx.Done():
	}
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
