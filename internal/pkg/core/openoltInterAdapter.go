/*
 * Copyright 2018-present Open Networking Foundation

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
	"github.com/opencord/voltha-protos/v5/go/adapter_services"
	"github.com/opencord/voltha-protos/v5/go/common"
	ic "github.com/opencord/voltha-protos/v5/go/inter_container"
	"github.com/opencord/voltha-protos/v5/go/voltha"
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
func (oo *OpenOLTInterAdapter) GetHealthStatus(ctx context.Context, empty *empty.Empty) (*voltha.HealthStatus, error) {
	return &voltha.HealthStatus{State: voltha.HealthStatus_HEALTHY}, nil
}

// ProxyOmciRequest proxies an OMCI request from the child adapter
func (oo *OpenOLTInterAdapter) ProxyOmciRequest(ctx context.Context, request *ic.OmciMessage) (*empty.Empty, error) {
	return oo.oltAdapter.ProxyOmciRequest(ctx, request)
}

// GetTechProfileInstance returns an instance of a tech profile
func (oo *OpenOLTInterAdapter) GetTechProfileInstance(ctx context.Context, request *ic.TechProfileInstanceRequestMessage) (*ic.TechProfileDownloadMessage, error) {
	return oo.oltAdapter.GetTechProfileInstance(ctx, request)
}

func (oo *OpenOLTInterAdapter) KeepAliveConnection(conn *common.Connection, remote adapter_services.OltInterAdapterService_KeepAliveConnectionServer) error {
	logger.Debugw(context.Background(), "receive-stream-connection", log.Fields{"connection": conn})

	if conn == nil {
		return fmt.Errorf("conn-is-nil %v", conn)
	}
	var err error
loop:
	for {
		keepAliveTimer := time.NewTimer(time.Duration(conn.KeepAliveInterval))
		select {
		case <-keepAliveTimer.C:
			if err = remote.Send(&common.Connection{Endpoint: oo.oltAdapter.config.AdapterEndpoint}); err != nil {
				break loop
			} else {
				logger.Debugw(context.Background(), "send-on-connection", log.Fields{"endpoint": oo.oltAdapter.config.AdapterEndpoint})
			}
		case <-oo.exitChannel:
			logger.Warnw(context.Background(), "received-stop", log.Fields{"remote": conn.Endpoint})
			break loop
		}
	}
	logger.Errorw(context.Background(), "connection-down", log.Fields{"remote": conn.Endpoint, "error": err})
	return err
}
