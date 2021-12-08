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
	"errors"
	"fmt"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	vgrpc "github.com/opencord/voltha-lib-go/v7/pkg/grpc"
	"github.com/opencord/voltha-protos/v5/go/common"
	ca "github.com/opencord/voltha-protos/v5/go/core_adapter"
	"github.com/opencord/voltha-protos/v5/go/health"
	"github.com/opencord/voltha-protos/v5/go/voltha"
	"google.golang.org/grpc"
)

// NewMockCoreClient creates a new mock core client for a given core service
func NewMockCoreClient(coreService *MockCoreService) *vgrpc.Client {
	cc, _ := vgrpc.NewClient("mock-local-endpoint", "mock-remote-endpoint", nil)
	cc.SetService(coreService)
	return cc
}

// MockCoreService represents a mock core service
type MockCoreService struct {
	// Values to be used in test can reside inside this structure
	// TODO store relevant info in this, use this info for negative and positive tests
	Devices     map[string]*voltha.Device
	DevicePorts map[string][]*voltha.Port
}

// GetHealthStatus implements mock GetHealthStatus
func (mcs MockCoreService) GetHealthStatus(ctx context.Context, in *common.Connection, opts ...grpc.CallOption) (*health.HealthStatus, error) {
	return &health.HealthStatus{State: health.HealthStatus_HEALTHY}, nil
}

// RegisterAdapter implements mock RegisterAdapter
func (mcs MockCoreService) RegisterAdapter(ctx context.Context, in *ca.AdapterRegistration, opts ...grpc.CallOption) (*empty.Empty, error) {
	if ctx == nil || in.Adapter == nil || in.DTypes == nil {
		return nil, errors.New("registerAdapter func parameters cannot be nil")
	}
	return &empty.Empty{}, nil
}

// DeviceUpdate implements mock DeviceUpdate
func (mcs MockCoreService) DeviceUpdate(ctx context.Context, in *voltha.Device, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.Id == "" {
		return nil, errors.New("no Device")
	}
	return &empty.Empty{}, nil
}

// PortCreated implements mock PortCreated
func (mcs MockCoreService) PortCreated(ctx context.Context, in *voltha.Port, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.DeviceId == "" {
		return nil, errors.New("no deviceID")
	}
	if in.Type > 7 {
		return nil, errors.New("invalid porttype")
	}
	return &empty.Empty{}, nil
}

// PortsStateUpdate implements mock PortsStateUpdate
func (mcs MockCoreService) PortsStateUpdate(ctx context.Context, in *ca.PortStateFilter, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.DeviceId == "" {
		return nil, errors.New("no Device")
	}
	return &empty.Empty{}, nil
}

// DeleteAllPorts implements mock DeleteAllPorts
func (mcs MockCoreService) DeleteAllPorts(ctx context.Context, in *common.ID, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.Id == "" {
		return nil, errors.New("no Device id")
	}
	return &empty.Empty{}, nil
}

// GetDevicePort implements mock GetDevicePort
func (mcs MockCoreService) GetDevicePort(ctx context.Context, in *ca.PortFilter, opts ...grpc.CallOption) (*voltha.Port, error) {
	for _, port := range mcs.DevicePorts[in.DeviceId] {
		if port.PortNo == in.Port {
			return port, nil
		}
	}
	return nil, errors.New("device/port not found")
}

// ListDevicePorts implements mock ListDevicePorts
func (mcs MockCoreService) ListDevicePorts(ctx context.Context, in *common.ID, opts ...grpc.CallOption) (*voltha.Ports, error) {
	ports, have := mcs.DevicePorts[in.Id]
	if !have {
		return nil, errors.New("device id not found")
	}
	return &voltha.Ports{Items: ports}, nil
}

// DeviceStateUpdate implements mock DeviceStateUpdate
func (mcs MockCoreService) DeviceStateUpdate(ctx context.Context, in *ca.DeviceStateFilter, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.DeviceId == "" {
		return nil, errors.New("no Device id")
	}
	return &empty.Empty{}, nil
}

// DevicePMConfigUpdate implements mock DevicePMConfigUpdate
func (mcs MockCoreService) DevicePMConfigUpdate(ctx context.Context, in *voltha.PmConfigs, opts ...grpc.CallOption) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

// ChildDeviceDetected implements mock ChildDeviceDetected
func (mcs MockCoreService) ChildDeviceDetected(ctx context.Context, in *ca.DeviceDiscovery, opts ...grpc.CallOption) (*voltha.Device, error) {
	if in.ParentId == "" {
		return nil, errors.New("no deviceID")
	}
	for k, v := range mcs.Devices {
		if strings.Contains(k, "onu") {
			return v, nil
		}
	}
	return nil, errors.New("no deviceID")
}

// ChildDevicesLost implements mock ChildDevicesLost
func (mcs MockCoreService) ChildDevicesLost(ctx context.Context, in *common.ID, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.Id == "" {
		return nil, errors.New("no device id")
	}
	return &empty.Empty{}, nil
}

// ChildDevicesDetected implements mock ChildDevicesDetected
func (mcs MockCoreService) ChildDevicesDetected(ctx context.Context, in *common.ID, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.Id == "" {
		return nil, errors.New("no device id")
	}
	return &empty.Empty{}, nil
}

// GetDevice implements mock GetDevice
func (mcs MockCoreService) GetDevice(ctx context.Context, in *common.ID, opts ...grpc.CallOption) (*voltha.Device, error) {
	if in.Id == "" {
		return &voltha.Device{}, errors.New("no deviceID")
	}
	for k, v := range mcs.Devices {
		if k == "olt" {
			return v, nil
		}
	}
	return nil, errors.New("device detection failed")
}

// GetChildDevice implements mock GetChildDevice
func (mcs MockCoreService) GetChildDevice(ctx context.Context, in *ca.ChildDeviceFilter, opts ...grpc.CallOption) (*voltha.Device, error) {
	if in.ParentId == "" {
		return nil, errors.New("device detection failed")
	}
	var onuDevice *voltha.Device
	for _, val := range mcs.Devices {
		if val.GetId() == fmt.Sprintf("%d", in.OnuId) {
			onuDevice = val
			break
		}
	}
	if onuDevice != nil {
		return onuDevice, nil
	}
	//return &voltha.Device{}, nil
	return nil, errors.New("device detection failed")
}

// GetChildDevices implements mock GetChildDevices
func (mcs MockCoreService) GetChildDevices(ctx context.Context, in *common.ID, opts ...grpc.CallOption) (*voltha.Devices, error) {
	if in.Id == "" {
		return nil, errors.New("no deviceID")
	}
	onuDevices := make([]*voltha.Device, 0)

	for _, val := range mcs.Devices {
		if val != nil && val.ParentId == in.Id {
			onuDevices = append(onuDevices, val)
		}
	}

	deviceList := &voltha.Devices{Items: onuDevices}
	if len(deviceList.Items) > 0 {
		return deviceList, nil
	}
	return nil, errors.New("device detection failed")
}

// SendPacketIn implements mock SendPacketIn
func (mcs MockCoreService) SendPacketIn(ctx context.Context, in *ca.PacketIn, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.DeviceId == "" {
		return nil, errors.New("no Device ID")
	}
	return &empty.Empty{}, nil
}

// DeviceReasonUpdate implements mock DeviceReasonUpdate
func (mcs MockCoreService) DeviceReasonUpdate(ctx context.Context, in *ca.DeviceReason, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.DeviceId == "" {
		return nil, errors.New("no Device ID")
	}
	return &empty.Empty{}, nil
}

// PortStateUpdate implements mock PortStateUpdate
func (mcs MockCoreService) PortStateUpdate(ctx context.Context, in *ca.PortState, opts ...grpc.CallOption) (*empty.Empty, error) {
	if in.DeviceId == "" {
		return nil, errors.New("no Device")
	}
	return &empty.Empty{}, nil
}

// Additional API found in the Core - unused?

// ReconcileChildDevices implements mock ReconcileChildDevices
func (mcs MockCoreService) ReconcileChildDevices(ctx context.Context, in *common.ID, opts ...grpc.CallOption) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

// GetChildDeviceWithProxyAddress implements mock GetChildDeviceWithProxyAddress
func (mcs MockCoreService) GetChildDeviceWithProxyAddress(ctx context.Context, in *voltha.Device_ProxyAddress, opts ...grpc.CallOption) (*voltha.Device, error) {
	return nil, nil
}

// GetPorts implements mock GetPorts
func (mcs MockCoreService) GetPorts(ctx context.Context, in *ca.PortFilter, opts ...grpc.CallOption) (*voltha.Ports, error) {
	return nil, nil
}

// ChildrenStateUpdate implements mock ChildrenStateUpdate
func (mcs MockCoreService) ChildrenStateUpdate(ctx context.Context, in *ca.DeviceStateFilter, opts ...grpc.CallOption) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

// UpdateImageDownload implements mock UpdateImageDownload
func (mcs MockCoreService) UpdateImageDownload(ctx context.Context, in *voltha.ImageDownload, opts ...grpc.CallOption) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}
