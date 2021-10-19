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
	"io"

	config "github.com/opencord/voltha-protos/v5/go/ext/config"
	"github.com/opencord/voltha-protos/v5/go/extension"
	openolt "github.com/opencord/voltha-protos/v5/go/openolt"
	tech_profile "github.com/opencord/voltha-protos/v5/go/tech_profile"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// MockOpenoltClient mock struct for OpenoltClient.
type MockOpenoltClient struct {
	counter     int
	IsRestarted bool
}

// DisableOlt mocks the DisableOlt function of Openoltclient.
func (ooc *MockOpenoltClient) DisableOlt(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Empty, error) {
	//return &openolt.Empty{}, nil
	if ooc.counter == 0 {
		ooc.counter++
		return &openolt.Empty{}, nil
	}
	return nil, errors.New("disableOlt failed")
}

// ReenableOlt mocks the ReenableOlt function of Openoltclient.
func (ooc *MockOpenoltClient) ReenableOlt(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Empty, error) {
	if ooc.counter == 0 {
		ooc.counter++
		return &openolt.Empty{}, nil
	}
	return nil, errors.New("reenable olt failed")
}

// ActivateOnu mocks the ActivateOnu function of Openoltclient.
func (ooc *MockOpenoltClient) ActivateOnu(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.Empty, error) {
	if in == nil {
		return nil, errors.New("invalid onuId")
	}
	return &openolt.Empty{}, nil
}

// DeactivateOnu mocks the DeactivateOnu function of Openoltclient.
func (ooc *MockOpenoltClient) DeactivateOnu(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// DeleteOnu mocks the DeleteOnu function of Openoltclient.
func (ooc *MockOpenoltClient) DeleteOnu(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// OmciMsgOut mocks the OmciMsgOut function of Openoltclient.
func (ooc *MockOpenoltClient) OmciMsgOut(ctx context.Context, in *openolt.OmciMsg, opts ...grpc.CallOption) (*openolt.Empty, error) {
	if in == nil {
		return nil, errors.New("invalid Omci Msg")
	}
	return &openolt.Empty{}, nil
}

// OnuPacketOut mocks the OnuPacketOut function of Openoltclient.
func (ooc *MockOpenoltClient) OnuPacketOut(ctx context.Context, in *openolt.OnuPacket, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// UplinkPacketOut mocks the UplinkPacketOut function of Openoltclient.
func (ooc *MockOpenoltClient) UplinkPacketOut(ctx context.Context, in *openolt.UplinkPacket, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// FlowAdd mocks the FlowAdd function of Openoltclient.
func (ooc *MockOpenoltClient) FlowAdd(ctx context.Context, in *openolt.Flow, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// FlowRemove mocks the FlowRemove function of Openoltclient.
func (ooc *MockOpenoltClient) FlowRemove(ctx context.Context, in *openolt.Flow, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// HeartbeatCheck mocks the HeartbeatCheck function of Openoltclient.
func (ooc *MockOpenoltClient) HeartbeatCheck(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Heartbeat, error) {
	return nil, nil
}

// EnablePonIf mocks the EnablePonIf function of Openoltclient.
func (ooc *MockOpenoltClient) EnablePonIf(ctx context.Context, in *openolt.Interface, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// DisablePonIf mocks the DisablePonIf function of Openoltclient.
func (ooc *MockOpenoltClient) DisablePonIf(ctx context.Context, in *openolt.Interface, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// GetDeviceInfo mocks the GetDeviceInfo function of Openoltclient.
func (ooc *MockOpenoltClient) GetDeviceInfo(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.DeviceInfo, error) {
	if ooc.counter == 0 {
		ooc.counter++
		deviceInfo := &openolt.DeviceInfo{Vendor: "Openolt", Model: "1.0", HardwareVersion: "1.0", FirmwareVersion: "1.0", DeviceId: "olt", DeviceSerialNumber: "olt"}
		return deviceInfo, nil
	}
	if ooc.counter == 1 {
		ooc.counter++
		deviceInfo := &openolt.DeviceInfo{Vendor: "Openolt", Model: "1.0", HardwareVersion: "1.0", FirmwareVersion: "1.0", DeviceId: "", DeviceSerialNumber: "olt"}
		return deviceInfo, nil
	}
	if ooc.counter == 2 {
		ooc.counter++
		return nil, nil
	}

	return nil, errors.New("device info not found")
}

// Reboot mocks the Reboot function of Openoltclient.
func (ooc *MockOpenoltClient) Reboot(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Empty, error) {
	if ooc.counter == 0 {
		ooc.counter++
		ooc.IsRestarted = true
		return &openolt.Empty{}, nil
	}
	return nil, errors.New("reboot failed")
}

// CollectStatistics mocks the CollectStatistics function of Openoltclient.
func (ooc *MockOpenoltClient) CollectStatistics(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// CreateTrafficSchedulers mocks the CreateTrafficSchedulers function of Openoltclient.
func (ooc *MockOpenoltClient) CreateTrafficSchedulers(ctx context.Context, in *tech_profile.TrafficSchedulers, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// RemoveTrafficSchedulers mocks the RemoveTrafficSchedulers function of Openoltclient.
func (ooc *MockOpenoltClient) RemoveTrafficSchedulers(ctx context.Context, in *tech_profile.TrafficSchedulers, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// CreateTrafficQueues mocks the CreateTrafficQueues function of Openoltclient.
func (ooc *MockOpenoltClient) CreateTrafficQueues(ctx context.Context, in *tech_profile.TrafficQueues, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// RemoveTrafficQueues mocks the RemoveTrafficQueues function of Openoltclient.
func (ooc *MockOpenoltClient) RemoveTrafficQueues(ctx context.Context, in *tech_profile.TrafficQueues, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// DeleteGroup mocks the DeleteGroup RPC of Openoltclient.
func (ooc *MockOpenoltClient) DeleteGroup(ctx context.Context, in *openolt.Group, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// GetLogicalOnuDistance mocks the GetLogicalOnuDistance RPC of Openoltclient.
func (ooc *MockOpenoltClient) GetLogicalOnuDistance(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.OnuLogicalDistance, error) {
	return &openolt.OnuLogicalDistance{}, nil
}

// GetLogicalOnuDistanceZero mocks the GetLogicalOnuDistanceZero RPC of Openoltclient.
func (ooc *MockOpenoltClient) GetLogicalOnuDistanceZero(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.OnuLogicalDistance, error) {
	return &openolt.OnuLogicalDistance{}, nil
}

// EnableIndication mocks the EnableIndication function of Openoltclient.
func (ooc *MockOpenoltClient) EnableIndication(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (openolt.Openolt_EnableIndicationClient, error) {
	if ooc.counter < 2 {
		ooc.counter++
		mockInd := &mockOpenoltEnableIndicationClient{0}
		return mockInd, nil
	}
	if ooc.counter == 2 {
		ooc.counter++
		return nil, nil
	}
	return nil, errors.New("invalid method invocation")
}

type mockOpenoltEnableIndicationClient struct {
	count int
}

func (mock *mockOpenoltEnableIndicationClient) Recv() (*openolt.Indication, error) {
	if mock.count == 0 {
		mock.count = mock.count + 1
		indi := &openolt.Indication{Data: &openolt.Indication_OltInd{OltInd: &openolt.OltIndication{OperState: "Down"}}}
		return indi, nil
	}
	if mock.count == 1 {
		mock.count = mock.count + 1
		return nil, errors.New("error, while processing indication")
	}

	return nil, io.EOF
}

func (mock *mockOpenoltEnableIndicationClient) Header() (metadata.MD, error) {
	return nil, nil
}

func (mock *mockOpenoltEnableIndicationClient) Trailer() metadata.MD {
	return nil
}

func (mock *mockOpenoltEnableIndicationClient) CloseSend() error {
	return nil
}

func (mock *mockOpenoltEnableIndicationClient) Context() context.Context {
	return context.Background()
}

func (mock *mockOpenoltEnableIndicationClient) SendMsg(m interface{}) error {
	return nil
}

func (mock *mockOpenoltEnableIndicationClient) RecvMsg(m interface{}) error {
	return nil
}

// GetExtValue mocks the GetExtValue function of Openoltclient.
func (ooc *MockOpenoltClient) GetExtValue(ctx context.Context, in *openolt.ValueParam, opts ...grpc.CallOption) (*extension.ReturnValues, error) {
	return &extension.ReturnValues{}, nil
}

// OnuItuPonAlarmSet mocks the OnuItuPonAlarmSet function of Openoltclient.
func (ooc *MockOpenoltClient) OnuItuPonAlarmSet(ctx context.Context, in *config.OnuItuPonAlarm, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

// PerformGroupOperation mocks the PerformGroupOperation function of Openoltclient.
func (ooc *MockOpenoltClient) PerformGroupOperation(ctx context.Context, in *openolt.Group, opts ...grpc.CallOption) (*openolt.Empty, error) {
	return &openolt.Empty{}, nil
}

//GetOnuStatistics mocks the GetOnuStatistics function of Openoltclient.
func (ooc *MockOpenoltClient) GetOnuStatistics(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.OnuStatistics, error) {
	return &openolt.OnuStatistics{}, nil
}

//GetGemPortStatistics mocks the GetGemPortStatistics function of Openoltclient.
func (ooc *MockOpenoltClient) GetGemPortStatistics(ctx context.Context, in *openolt.OnuPacket, opts ...grpc.CallOption) (*openolt.GemPortStatistics, error) {
	return &openolt.GemPortStatistics{}, nil
}

//GetPonRxPower mocks the GetPonRxPower function of Openoltclient.
func (ooc *MockOpenoltClient) GetPonRxPower(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.PonRxPowerData, error) {
	return &openolt.PonRxPowerData{}, nil
}
