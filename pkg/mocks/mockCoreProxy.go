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
	"fmt"

	"github.com/opencord/voltha-lib-go/v3/pkg/kafka"
	"github.com/opencord/voltha-protos/v3/go/voltha"
)

// MockCoreProxy mocks the CoreProxy interface
type MockCoreProxy struct {
	// Values to be used in test can reside inside this structure
	// TODO store relevant info in this, use this info for negative and positive tests
	Devices     map[string]*voltha.Device
	DevicePorts map[string][]*voltha.Port
}

// UpdateCoreReference mock updatesCoreReference
func (mcp *MockCoreProxy) UpdateCoreReference(deviceID string, coreReference string) {
	panic("implement me")
}

// DeleteCoreReference mock DeleteCoreReference function
func (mcp *MockCoreProxy) DeleteCoreReference(deviceID string) {
	panic("implement me")
}

// GetCoreTopic implements mock GetCoreTopic
func (mcp *MockCoreProxy) GetCoreTopic(deviceID string) kafka.Topic {
	panic("implement me")
}

// GetAdapterTopic implements mock GetAdapterTopic
func (mcp *MockCoreProxy) GetAdapterTopic(args ...string) kafka.Topic {
	panic("implement me")
}

// RegisterAdapter implements mock RegisterAdapter
func (mcp *MockCoreProxy) RegisterAdapter(ctx context.Context, adapter *voltha.Adapter,
	deviceTypes *voltha.DeviceTypes) error {
	if ctx == nil || adapter == nil || deviceTypes == nil {
		return errors.New("registerAdapter func parameters cannot be nil")
	}
	return nil
}

// DeviceUpdate implements mock DeviceUpdate
func (mcp *MockCoreProxy) DeviceUpdate(ctx context.Context, device *voltha.Device) error {
	if device.Id == "" {
		return errors.New("no Device")
	}
	return nil
}

// PortCreated implements mock PortCreated
func (mcp *MockCoreProxy) PortCreated(ctx context.Context, deviceID string, port *voltha.Port) error {
	if deviceID == "" {
		return errors.New("no deviceID")
	}
	if port.Type > 7 {
		return errors.New("invalid porttype")
	}
	return nil
}

// PortsStateUpdate implements mock PortsStateUpdate
func (mcp *MockCoreProxy) PortsStateUpdate(ctx context.Context, deviceID string, portTypeFilter uint32, operStatus voltha.OperStatus_Types) error {
	if deviceID == "" {
		return errors.New("no Device")
	}
	return nil
}

// DeleteAllPorts implements mock DeleteAllPorts
func (mcp *MockCoreProxy) DeleteAllPorts(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return errors.New("no Device id")
	}
	return nil
}

// GetDevicePort implements mock GetDevicePort
func (mcp *MockCoreProxy) GetDevicePort(ctx context.Context, deviceID string, portID uint32) (*voltha.Port, error) {
	for _, port := range mcp.DevicePorts[deviceID] {
		if port.PortNo == portID {
			return port, nil
		}
	}
	return nil, errors.New("device/port not found")
}

// ListDevicePorts implements mock ListDevicePorts
func (mcp *MockCoreProxy) ListDevicePorts(ctx context.Context, deviceID string) ([]*voltha.Port, error) {
	ports, have := mcp.DevicePorts[deviceID]
	if !have {
		return nil, errors.New("device id not found")
	}
	return ports, nil
}

// DeviceStateUpdate implements mock DeviceStateUpdate
func (mcp *MockCoreProxy) DeviceStateUpdate(ctx context.Context, deviceID string,
	connStatus voltha.ConnectStatus_Types, operStatus voltha.OperStatus_Types) error {
	if deviceID == "" {
		return errors.New("no Device id")
	}
	return nil
}

// ChildDeviceDetected implements mock ChildDeviceDetected
func (mcp *MockCoreProxy) ChildDeviceDetected(ctx context.Context, parentdeviceID string, parentPortNo int,
	childDeviceType string, channelID int, vendorID string, serialNumber string, onuID int64) (*voltha.Device, error) {
	if parentdeviceID == "" {
		return nil, errors.New("no deviceID")
	}
	return nil, nil
}

// ChildDevicesLost implements mock ChildDevicesLost.
func (mcp *MockCoreProxy) ChildDevicesLost(ctx context.Context, parentdeviceID string) error {
	//panic("implement me")
	if parentdeviceID == "" {
		return errors.New("no device id")
	}
	return nil
}

// ChildDevicesDetected implements mock ChildDevicesDetecte
func (mcp *MockCoreProxy) ChildDevicesDetected(ctx context.Context, parentdeviceID string) error {
	if parentdeviceID == "" {
		return errors.New("no device id")
	}
	return nil
}

// GetDevice implements mock GetDevice
func (mcp *MockCoreProxy) GetDevice(ctx context.Context, parentdeviceID string, deviceID string) (*voltha.Device, error) {
	if parentdeviceID == "" || deviceID == "" {
		return &voltha.Device{}, errors.New("no deviceID")
	}
	for k, v := range mcp.Devices {
		if k == "olt" {
			return v, nil
		}
	}
	return nil, errors.New("device detection failed")
}

// GetChildDevice implements mock GetChildDevice
func (mcp *MockCoreProxy) GetChildDevice(ctx context.Context, parentdeviceID string, kwargs map[string]interface{}) (*voltha.Device, error) {

	if parentdeviceID == "" {
		return nil, errors.New("device detection failed")
	}
	onuID := kwargs["onu_id"]
	var onuDevice *voltha.Device
	for _, val := range mcp.Devices {
		if val.GetId() == fmt.Sprintf("%v", onuID) {
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
func (mcp *MockCoreProxy) GetChildDevices(ctx context.Context, parentdeviceID string) (*voltha.Devices, error) {
	if parentdeviceID == "" {
		return nil, errors.New("no deviceID")
	}
	onuDevices := make([]*voltha.Device, 0)

	for _, val := range mcp.Devices {
		if val != nil {
			onuDevices = append(onuDevices, val)
		}
	}

	deviceList := &voltha.Devices{Items: onuDevices}
	if len(deviceList.Items) > 0 {
		return deviceList, nil
	}
	return nil, errors.New("device detection failed")
}

// SendPacketIn  implements mock SendPacketIn
func (mcp *MockCoreProxy) SendPacketIn(ctx context.Context, deviceID string, port uint32, pktPayload []byte) error {
	if deviceID == "" {
		return errors.New("no Device ID")
	}
	return nil
}

// DeviceReasonUpdate  implements mock SendPacketIn
func (mcp *MockCoreProxy) DeviceReasonUpdate(ctx context.Context, deviceID string, reason string) error {
	if deviceID == "" {
		return errors.New("no Device ID")
	}
	return nil
}

// DevicePMConfigUpdate implements mock DevicePMConfigUpdate
func (mcp *MockCoreProxy) DevicePMConfigUpdate(ctx context.Context, pmConfigs *voltha.PmConfigs) error {
	return nil
}

// PortStateUpdate implements mock PortStateUpdate
func (mcp *MockCoreProxy) PortStateUpdate(ctx context.Context, deviceID string, pType voltha.Port_PortType, portNo uint32,
	operStatus voltha.OperStatus_Types) error {
	if deviceID == "" {
		return errors.New("no Device")
	}
	return nil
}
