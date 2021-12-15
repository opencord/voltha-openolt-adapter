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

//Package core provides the utility for olt devices, flows and statistics
package core

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	"github.com/opencord/voltha-protos/v5/go/common"
	"github.com/opencord/voltha-protos/v5/go/health"
	ia "github.com/opencord/voltha-protos/v5/go/inter_adapter"
	oltia "github.com/opencord/voltha-protos/v5/go/olt_inter_adapter_service"
)

//OpenOLT structure holds the OLT information
type OpenOLTInterAdapter struct {
	oltAdapter  *OpenOLT
	exitChannel chan struct{}
}

//NewOpenOLTInterAdapter returns a new instance of OpenOLTInterAdapter
func NewOpenOLTInterAdapter(oltAdapter *OpenOLT) *OpenOLTInterAdapter {
	return &OpenOLTInterAdapter{oltAdapter: oltAdapter, exitChannel: make(chan struct{})}
}

//Start starts (logs) the device manager
func (oo *OpenOLTInterAdapter) Start(ctx context.Context) error {
	return nil
}

//Stop terminates the session
func (oo *OpenOLTInterAdapter) Stop(ctx context.Context) error {
	close(oo.exitChannel)
	return nil
}

// GetHealthStatus is used as a service readiness validation as a grpc connection
func (oo *OpenOLTInterAdapter) GetHealthStatus(ctx context.Context, conn *common.Connection) (*health.HealthStatus, error) {
	return &health.HealthStatus{State: health.HealthStatus_HEALTHY}, nil
}

// ProxyOmciRequest proxies an OMCI request from the child adapter
func (oo *OpenOLTInterAdapter) ProxyOmciRequest(ctx context.Context, request *ia.OmciMessage) (*empty.Empty, error) {
	return oo.oltAdapter.ProxyOmciRequest(ctx, request)
}

// GetTechProfileInstance returns an instance of a tech profile
func (oo *OpenOLTInterAdapter) GetTechProfileInstance(ctx context.Context, request *ia.TechProfileInstanceRequestMessage) (*ia.TechProfileDownloadMessage, error) {
	return oo.oltAdapter.GetTechProfileInstance(ctx, request)
}

func (oo *OpenOLTInterAdapter) KeepAlive(remote oltia.OltInterAdapterService_KeepAliveServer) error {
	ctx := context.Background()
	logger.Debugw(ctx, "receive-stream-connection", log.Fields{"remote": remote})

	if remote == nil {
		return fmt.Errorf("conn-is-nil %v", remote)
	}
	initialRequestTime := time.Now()
	var err error
loop:
	for {
		select {
		case <-remote.Context().Done():
			logger.Infow(ctx, "stream-keep-alive-context-done", log.Fields{"remote": remote, "error": remote.Context().Err()})
			break loop
		case <-oo.exitChannel:
			logger.Warnw(ctx, "received-stop", log.Fields{"remote": remote, "initial-conn-time": initialRequestTime})
			break loop
		default:
		}

		remote, err := remote.Recv()
		if err != nil {
			logger.Warnw(ctx, "received-stream-error", log.Fields{"remote": remote, "error": err})
			break loop
		}
		logger.Debugw(ctx, "received-keep-alive", log.Fields{"remote": remote})
	}
	logger.Errorw(ctx, "connection-down", log.Fields{"remote": remote, "error": err, "initial-conn-time": initialRequestTime})
	return err
}
