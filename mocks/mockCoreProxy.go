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

//Package mocks provides the mocks for openolt-adapter.
package mocks

import (
	"context"
	"errors"

	"github.com/opencord/voltha-go/kafka"
	"github.com/opencord/voltha-protos/go/voltha"
)

// MockCoreProxy mocks the CoreProxy interface
type MockCoreProxy struct {
	// Values to be used in test can reside inside this structure
	// TODO store relevant info in this, use this info for negative and positive tests
}

// UpdateCoreReference mock updatesCoreReference
func (mp *MockCoreProxy) UpdateCoreReference(deviceID string, coreReference string) {
	panic("implement me")
}

// DeleteCoreReference mock DeleteCoreReference function
func (mp *MockCoreProxy) DeleteCoreReference(deviceID string) {
	panic("implement me")
}

// GetCoreTopic implements mock GetCoreTopic
func (mp *MockCoreProxy) GetCoreTopic(deviceID string) kafka.Topic {
	panic("implement me")
}

// GetAdapterTopic implements mock GetAdapterTopic
func (mp *MockCoreProxy) GetAdapterTopic(args ...string) kafka.Topic {
	panic("implement me")
}

// RegisterAdapter implements mock RegisterAdapter
func (mp *MockCoreProxy) RegisterAdapter(ctx context.Context, adapter *voltha.Adapter,
	deviceTypes *voltha.DeviceTypes) error {
	if ctx == nil || adapter == nil || deviceTypes == nil {

		return errors.New("registerAdapter func parameters cannot be nil")
	}
	return nil

}

// DeviceUpdate implements mock DeviceUpdate
func (mp *MockCoreProxy) DeviceUpdate(ctx context.Context, device *voltha.Device) error {
	panic("implement me")
}

// PortCreated implements mock PortCreated
func (mp *MockCoreProxy) PortCreated(ctx context.Context, deviceID string, port *voltha.Port) error {
	panic("implement me")
}

// PortsStateUpdate implements mock PortsStateUpdate
func (mp *MockCoreProxy) PortsStateUpdate(ctx context.Context, deviceID string, operStatus voltha.OperStatus_OperStatus) error {
	panic("implement me")
}

// DeleteAllPorts implements mock DeleteAllPorts
func (mp *MockCoreProxy) DeleteAllPorts(ctx context.Context, deviceID string) error {
	panic("implement me")
}

// DeviceStateUpdate implements mock DeviceStateUpdate
func (mp *MockCoreProxy) DeviceStateUpdate(ctx context.Context, deviceID string,
	connStatus voltha.ConnectStatus_ConnectStatus, operStatus voltha.OperStatus_OperStatus) error {
	panic("implement me")
}

// ChildDeviceDetected implements mock ChildDeviceDetected
func (mp *MockCoreProxy) ChildDeviceDetected(ctx context.Context, parentDeviceID string, parentPortNo int,
	childDeviceType string, channelID int, vendorID string, serialNumber string, onuID int64) (*voltha.Device, error) {
	panic("implement me")
}

// ChildDevicesLost implements mock ChildDevicesLost
func (mp *MockCoreProxy) ChildDevicesLost(ctx context.Context, parentDeviceID string) error {
	panic("implement me")
}

// ChildDevicesDetected implements mock ChildDevicesDetecte
func (mp *MockCoreProxy) ChildDevicesDetected(ctx context.Context, parentDeviceID string) error {
	panic("implement me")
}

// GetDevice implements mock GetDevice
func (mp *MockCoreProxy) GetDevice(ctx context.Context, parentDeviceID string, deviceID string) (*voltha.Device, error) {
	if parentDeviceID != "" {
		return &voltha.Device{}, nil
	}
	return nil, errors.New("device detection failed")
}

// GetChildDevice implements mock GetChildDevice
func (mp *MockCoreProxy) GetChildDevice(ctx context.Context, parentDeviceID string, kwargs map[string]interface{}) (*voltha.Device, error) {
	if parentDeviceID != "" {
		return &voltha.Device{}, nil
	}
	return nil, errors.New("device detection failed")
}

// GetChildDevices implements mock GetChildDevices
func (mp *MockCoreProxy) GetChildDevices(ctx context.Context, parentDeviceID string) (*voltha.Devices, error) {
	if parentDeviceID != "" {
		return &voltha.Devices{}, nil
	}
	return nil, errors.New("device detection failed")
}

// SendPacketIn  implements mock SendPacketIn
func (mp *MockCoreProxy) SendPacketIn(ctx context.Context, deviceID string, port uint32, pktPayload []byte) error {
	panic("implement me")
}
