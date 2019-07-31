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

package mocks

import (
	"context"
	"errors"

	"github.com/opencord/voltha-go/kafka"
	"github.com/opencord/voltha-protos/go/voltha"
)

type MockCoreProxy struct {
	// Values to be used in test can reside inside this structure
	// TODO store relevant info in this, use this info for negative and positive tests
}

func (MockCoreProxy) UpdateCoreReference(deviceId string, coreReference string) {
	panic("implement me")
}

func (MockCoreProxy) DeleteCoreReference(deviceId string) {
	panic("implement me")
}

func (MockCoreProxy) GetCoreTopic(deviceId string) kafka.Topic {
	panic("implement me")
}

func (MockCoreProxy) GetAdapterTopic(args ...string) kafka.Topic {
	panic("implement me")
}

func (MockCoreProxy) RegisterAdapter(ctx context.Context, adapter *voltha.Adapter,
	deviceTypes *voltha.DeviceTypes) error {
	if ctx == nil || adapter == nil || deviceTypes == nil {

		return errors.New("RegisterAdapter func parameters cannot be nil")
	}
	return nil
	// TODO match adapter value recieved with the value stored.
	// If does'nt match, then failure case
}

func (MockCoreProxy) DeviceUpdate(ctx context.Context, device *voltha.Device) error {
	panic("implement me")
}

func (MockCoreProxy) PortCreated(ctx context.Context, deviceId string, port *voltha.Port) error {
	panic("implement me")
}

func (MockCoreProxy) PortsStateUpdate(ctx context.Context, deviceId string, operStatus voltha.OperStatus_OperStatus) error {
	panic("implement me")
}

func (MockCoreProxy) DeleteAllPorts(ctx context.Context, deviceId string) error {
	panic("implement me")
}

func (MockCoreProxy) DeviceStateUpdate(ctx context.Context, deviceId string,
	connStatus voltha.ConnectStatus_ConnectStatus, operStatus voltha.OperStatus_OperStatus) error {
	panic("implement me")
}

func (MockCoreProxy) ChildDeviceDetected(ctx context.Context, parentDeviceID string, parentPortNo int,
	childDeviceType string, channelID int, vendorID string, serialNumber string, onuID int64) (*voltha.Device, error) {
	panic("implement me")
}

func (MockCoreProxy) ChildDevicesLost(ctx context.Context, parentDeviceId string) error {
	panic("implement me")
}

func (MockCoreProxy) ChildDevicesDetected(ctx context.Context, parentDeviceId string) error {
	panic("implement me")
}

func (MockCoreProxy) GetDevice(ctx context.Context, parentDeviceId string, deviceId string) (*voltha.Device, error) {
	//panic("implement me")
	if parentDeviceId != "" {
		return &voltha.Device{}, nil
	}
	return nil, errors.New("Device detection failed")
}

func (MockCoreProxy) GetChildDevice(ctx context.Context, parentDeviceId string, kwargs map[string]interface{}) (*voltha.Device, error) {
	//panic("implement me")
	if parentDeviceId != "" {
		return &voltha.Device{}, nil
	}
	return nil, errors.New("Device detection failed")
}

func (MockCoreProxy) GetChildDevices(ctx context.Context, parentDeviceId string) (*voltha.Devices, error) {
	//panic("implement me")
	if parentDeviceId != "" {
		return &voltha.Devices{}, nil
	}
	return nil, errors.New("Device detection failed")
}

func (MockCoreProxy) SendPacketIn(ctx context.Context, deviceId string, port uint32, pktPayload []byte) error {
	panic("implement me")
}
