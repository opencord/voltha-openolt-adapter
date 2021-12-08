/*
 * Copyright 2021-present Open Networking Foundation

 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at

 * http://www.apache.org/licenses/LICENSE-2.0

 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//Package mocks provides the mocks for openolt-adapter.
package mocks

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	vgrpc "github.com/opencord/voltha-lib-go/v7/pkg/grpc"
	"github.com/opencord/voltha-protos/v5/go/common"
	"github.com/opencord/voltha-protos/v5/go/health"
	ia "github.com/opencord/voltha-protos/v5/go/inter_adapter"
	"google.golang.org/grpc"
)

// NewMockChildAdapterClient create a mock child adapter client
func NewMockChildAdapterClient(srv *MockOnuInterAdapterService) *vgrpc.Client {
	cc, _ := vgrpc.NewClient("mock-local-endpoint", "mock-remote-endpoint", nil)
	cc.SetService(srv)
	return cc
}

// MockOnuInterAdapterService represents a child adapter mock service
type MockOnuInterAdapterService struct {
}

// GetHealthStatus implements mock GetHealthStatus
func (mos MockOnuInterAdapterService) GetHealthStatus(ctx context.Context, in *common.Connection, opts ...grpc.CallOption) (*health.HealthStatus, error) {
	return &health.HealthStatus{State: health.HealthStatus_HEALTHY}, nil
}

// OnuIndication implements mock OnuIndication
func (mos *MockOnuInterAdapterService) OnuIndication(ctx context.Context, in *ia.OnuIndicationMessage, opts ...grpc.CallOption) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

// OmciResponse implements mock OmciResponse
func (mos *MockOnuInterAdapterService) OmciResponse(ctx context.Context, in *ia.OmciMessage, opts ...grpc.CallOption) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

// DownloadTechProfile implements mock DownloadTechProfile
func (mos *MockOnuInterAdapterService) DownloadTechProfile(ctx context.Context, in *ia.TechProfileDownloadMessage, opts ...grpc.CallOption) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

// DeleteGemPort implements mock DeleteGemPort
func (mos *MockOnuInterAdapterService) DeleteGemPort(ctx context.Context, in *ia.DeleteGemPortMessage, opts ...grpc.CallOption) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

// DeleteTCont implements mock DeleteTCont
func (mos *MockOnuInterAdapterService) DeleteTCont(ctx context.Context, in *ia.DeleteTcontMessage, opts ...grpc.CallOption) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}
