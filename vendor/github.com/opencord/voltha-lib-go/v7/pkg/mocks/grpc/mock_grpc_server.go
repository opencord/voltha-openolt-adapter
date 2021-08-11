package grpc

import (
	"context"
	"strconv"

	vgrpc "github.com/opencord/voltha-lib-go/v7/pkg/grpc"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	"github.com/opencord/voltha-lib-go/v7/pkg/probe"
	"github.com/opencord/voltha-protos/v5/go/adapter_services"
	"github.com/opencord/voltha-protos/v5/go/core"
	"github.com/phayes/freeport"
	"google.golang.org/grpc"
)

const (
	mockGrpcServer = "mock-grpc-server"
)

type MockGRPCServer struct {
	ApiEndpoint string
	server      *vgrpc.GrpcServer
	probe       *probe.Probe
}

func NewMockGRPCServer(ctx context.Context) (*MockGRPCServer, error) {
	grpcPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}
	s := &MockGRPCServer{
		ApiEndpoint: "127.0.0.1:" + strconv.Itoa(grpcPort),
		probe:       &probe.Probe{},
	}
	probePort, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}
	probeEndpoint := "127.0.0.1:" + strconv.Itoa(probePort)
	go s.probe.ListenAndServe(ctx, probeEndpoint)
	s.probe.RegisterService(ctx, mockGrpcServer)
	s.server = vgrpc.NewGrpcServer(s.ApiEndpoint, nil, false, s.probe)

	logger.Infow(ctx, "mock-grpc-server-created", log.Fields{"endpoint": s.ApiEndpoint})
	return s, nil
}

func (s *MockGRPCServer) AddCoreService(ctx context.Context, srv core.CoreServiceServer) {
	s.server.AddService(func(server *grpc.Server) {
		core.RegisterCoreServiceServer(server, srv)
	})
}

func (s *MockGRPCServer) AddAdapterService(ctx context.Context, srv adapter_services.AdapterServiceServer) {
	s.server.AddService(func(server *grpc.Server) {
		adapter_services.RegisterAdapterServiceServer(server, srv)
	})
}

func (s *MockGRPCServer) Start(ctx context.Context) {
	s.probe.UpdateStatus(ctx, mockGrpcServer, probe.ServiceStatusRunning)
	s.server.Start(ctx)
	s.probe.UpdateStatus(ctx, mockGrpcServer, probe.ServiceStatusStopped)
}

func (s *MockGRPCServer) Stop() {
	if s.server != nil {
		s.server.Stop()
	}
}
