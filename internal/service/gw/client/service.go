package client

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

type OnConfigFn func(*rootModel.Root) error

type Service struct {
	globalCtx context.Context
	address   string
	onConfig  OnConfigFn
}

func New(globalCtx context.Context, address string, onConfig OnConfigFn) *Service {
	return &Service{
		globalCtx: globalCtx,
		address:   address,
		onConfig:  onConfig,
	}
}

func (s *Service) Start() {
	if s.address == "" {
		return
	}
	go s.worker()
}

func (s *Service) worker() {
	for s.globalCtx.Err() == nil {
		conn, client, err := s.connect()
		if err != nil {
			slog.Error("gw snapshot-client: connect", "error", err, "address", s.address)
			s.sleepOrDone(time.Second)
			continue
		}

		err = s.runSession(client)
		_ = conn.Close()
		if err != nil && s.globalCtx.Err() == nil {
			slog.Error("gw snapshot-client: run session", "error", err)
			s.sleepOrDone(time.Second)
		}
	}
}

func (s *Service) runSession(client ruto_v1.SnapshotClient) error {
	_, err := s.fetchAndApply(client)
	if err != nil {
		return fmt.Errorf("fetchAndApply(initial): %w", err)
	}

	stream, err := client.SubscribeVersions(s.globalCtx, &emptypb.Empty{})
	if err != nil {
		return fmt.Errorf("SubscribeVersions: %w", err)
	}

	lastVersion := ""
	for s.globalCtx.Err() == nil {
		msg, recvErr := stream.Recv()
		if recvErr != nil {
			return fmt.Errorf("stream.Recv: %w", recvErr)
		}

		version := msg.GetVersion()
		if version == "" || version == lastVersion {
			continue
		}
		lastVersion = version

		_, err = s.fetchAndApply(client)
		if err != nil {
			slog.Error("gw snapshot-client: fetchAndApply(on update)", "error", err, "version", version)
			continue
		}
	}

	return s.globalCtx.Err()
}

func (s *Service) fetchAndApply(client ruto_v1.SnapshotClient) (*rootModel.Root, error) {
	rep, err := client.Get(s.globalCtx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("Snapshot.Get: %w", err)
	}

	conf, err := decodeConfig(rep)
	if err != nil {
		return nil, fmt.Errorf("decodeConfig: %w", err)
	}

	if s.onConfig != nil {
		if err = s.onConfig(conf); err != nil {
			return nil, fmt.Errorf("onConfig: %w", err)
		}
	}

	return conf, nil
}

func (s *Service) connect() (*grpc.ClientConn, ruto_v1.SnapshotClient, error) {
	conn, err := grpc.NewClient(
		s.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("grpc.NewClient: %w", err)
	}

	return conn, ruto_v1.NewSnapshotClient(conn), nil
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
