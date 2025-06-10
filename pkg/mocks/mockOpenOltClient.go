/*
 * Copyright 2018-2024 Open Networking Foundation (ONF) and the ONF Contributors

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

// Package mocks provides the mocks for openolt-adapter.
package mocks

import (
	context "context"
	reflect "reflect"

	common "github.com/opencord/voltha-protos/v5/go/common"
	config "github.com/opencord/voltha-protos/v5/go/ext/config"
	extension "github.com/opencord/voltha-protos/v5/go/extension"
	openolt "github.com/opencord/voltha-protos/v5/go/openolt"
	tech_profile "github.com/opencord/voltha-protos/v5/go/tech_profile"
	gomock "go.uber.org/mock/gomock"
	grpc "google.golang.org/grpc"
)

// MockOpenoltClient is a mock of OpenoltClient interface.
type MockOpenoltClient struct {
	ctrl     *gomock.Controller
	recorder *MockOpenoltClientMockRecorder
	isgomock struct{}
}

// MockOpenoltClientMockRecorder is the mock recorder for MockOpenoltClient.
type MockOpenoltClientMockRecorder struct {
	mock *MockOpenoltClient
}

// NewMockOpenoltClient creates a new mock instance.
func NewMockOpenoltClient(ctrl *gomock.Controller) *MockOpenoltClient {
	mock := &MockOpenoltClient{ctrl: ctrl}
	mock.recorder = &MockOpenoltClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOpenoltClient) EXPECT() *MockOpenoltClientMockRecorder {
	return m.recorder
}

// ActivateOnu mocks base method.
func (m *MockOpenoltClient) ActivateOnu(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ActivateOnu", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ActivateOnu indicates an expected call of ActivateOnu.
func (mr *MockOpenoltClientMockRecorder) ActivateOnu(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActivateOnu", reflect.TypeOf((*MockOpenoltClient)(nil).ActivateOnu), varargs...)
}

// CollectStatistics mocks base method.
func (m *MockOpenoltClient) CollectStatistics(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CollectStatistics", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CollectStatistics indicates an expected call of CollectStatistics.
func (mr *MockOpenoltClientMockRecorder) CollectStatistics(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CollectStatistics", reflect.TypeOf((*MockOpenoltClient)(nil).CollectStatistics), varargs...)
}

// CreateTrafficQueues mocks base method.
func (m *MockOpenoltClient) CreateTrafficQueues(ctx context.Context, in *tech_profile.TrafficQueues, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CreateTrafficQueues", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateTrafficQueues indicates an expected call of CreateTrafficQueues.
func (mr *MockOpenoltClientMockRecorder) CreateTrafficQueues(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTrafficQueues", reflect.TypeOf((*MockOpenoltClient)(nil).CreateTrafficQueues), varargs...)
}

// CreateTrafficSchedulers mocks base method.
func (m *MockOpenoltClient) CreateTrafficSchedulers(ctx context.Context, in *tech_profile.TrafficSchedulers, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CreateTrafficSchedulers", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateTrafficSchedulers indicates an expected call of CreateTrafficSchedulers.
func (mr *MockOpenoltClientMockRecorder) CreateTrafficSchedulers(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTrafficSchedulers", reflect.TypeOf((*MockOpenoltClient)(nil).CreateTrafficSchedulers), varargs...)
}

// DeactivateOnu mocks base method.
func (m *MockOpenoltClient) DeactivateOnu(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeactivateOnu", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeactivateOnu indicates an expected call of DeactivateOnu.
func (mr *MockOpenoltClientMockRecorder) DeactivateOnu(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeactivateOnu", reflect.TypeOf((*MockOpenoltClient)(nil).DeactivateOnu), varargs...)
}

// DeleteGroup mocks base method.
func (m *MockOpenoltClient) DeleteGroup(ctx context.Context, in *openolt.Group, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteGroup", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteGroup indicates an expected call of DeleteGroup.
func (mr *MockOpenoltClientMockRecorder) DeleteGroup(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteGroup", reflect.TypeOf((*MockOpenoltClient)(nil).DeleteGroup), varargs...)
}

// DeleteOnu mocks base method.
func (m *MockOpenoltClient) DeleteOnu(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteOnu", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteOnu indicates an expected call of DeleteOnu.
func (mr *MockOpenoltClientMockRecorder) DeleteOnu(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteOnu", reflect.TypeOf((*MockOpenoltClient)(nil).DeleteOnu), varargs...)
}

// DisableOlt mocks base method.
func (m *MockOpenoltClient) DisableOlt(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DisableOlt", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DisableOlt indicates an expected call of DisableOlt.
func (mr *MockOpenoltClientMockRecorder) DisableOlt(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DisableOlt", reflect.TypeOf((*MockOpenoltClient)(nil).DisableOlt), varargs...)
}

// DisablePonIf mocks base method.
func (m *MockOpenoltClient) DisablePonIf(ctx context.Context, in *openolt.Interface, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DisablePonIf", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DisablePonIf indicates an expected call of DisablePonIf.
func (mr *MockOpenoltClientMockRecorder) DisablePonIf(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DisablePonIf", reflect.TypeOf((*MockOpenoltClient)(nil).DisablePonIf), varargs...)
}

// EnableIndication mocks base method.
func (m *MockOpenoltClient) EnableIndication(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (openolt.Openolt_EnableIndicationClient, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "EnableIndication", varargs...)
	ret0, _ := ret[0].(openolt.Openolt_EnableIndicationClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EnableIndication indicates an expected call of EnableIndication.
func (mr *MockOpenoltClientMockRecorder) EnableIndication(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnableIndication", reflect.TypeOf((*MockOpenoltClient)(nil).EnableIndication), varargs...)
}

// EnablePonIf mocks base method.
func (m *MockOpenoltClient) EnablePonIf(ctx context.Context, in *openolt.Interface, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "EnablePonIf", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EnablePonIf indicates an expected call of EnablePonIf.
func (mr *MockOpenoltClientMockRecorder) EnablePonIf(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnablePonIf", reflect.TypeOf((*MockOpenoltClient)(nil).EnablePonIf), varargs...)
}

// FlowAdd mocks base method.
func (m *MockOpenoltClient) FlowAdd(ctx context.Context, in *openolt.Flow, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "FlowAdd", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FlowAdd indicates an expected call of FlowAdd.
func (mr *MockOpenoltClientMockRecorder) FlowAdd(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FlowAdd", reflect.TypeOf((*MockOpenoltClient)(nil).FlowAdd), varargs...)
}

// FlowRemove mocks base method.
func (m *MockOpenoltClient) FlowRemove(ctx context.Context, in *openolt.Flow, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "FlowRemove", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FlowRemove indicates an expected call of FlowRemove.
func (mr *MockOpenoltClientMockRecorder) FlowRemove(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FlowRemove", reflect.TypeOf((*MockOpenoltClient)(nil).FlowRemove), varargs...)
}

// GetAllocIdStatistics mocks base method.
func (m *MockOpenoltClient) GetAllocIdStatistics(ctx context.Context, in *openolt.OnuPacket, opts ...grpc.CallOption) (*openolt.OnuAllocIdStatistics, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetAllocIdStatistics", varargs...)
	ret0, _ := ret[0].(*openolt.OnuAllocIdStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllocIdStatistics indicates an expected call of GetAllocIdStatistics.
func (mr *MockOpenoltClientMockRecorder) GetAllocIdStatistics(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllocIdStatistics", reflect.TypeOf((*MockOpenoltClient)(nil).GetAllocIdStatistics), varargs...)
}

// GetDeviceInfo mocks base method.
func (m *MockOpenoltClient) GetDeviceInfo(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.DeviceInfo, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetDeviceInfo", varargs...)
	ret0, _ := ret[0].(*openolt.DeviceInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeviceInfo indicates an expected call of GetDeviceInfo.
func (mr *MockOpenoltClientMockRecorder) GetDeviceInfo(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeviceInfo", reflect.TypeOf((*MockOpenoltClient)(nil).GetDeviceInfo), varargs...)
}

// GetExtValue mocks base method.
func (m *MockOpenoltClient) GetExtValue(ctx context.Context, in *openolt.ValueParam, opts ...grpc.CallOption) (*extension.ReturnValues, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetExtValue", varargs...)
	ret0, _ := ret[0].(*extension.ReturnValues)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExtValue indicates an expected call of GetExtValue.
func (mr *MockOpenoltClientMockRecorder) GetExtValue(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExtValue", reflect.TypeOf((*MockOpenoltClient)(nil).GetExtValue), varargs...)
}

// GetGemPortStatistics mocks base method.
func (m *MockOpenoltClient) GetGemPortStatistics(ctx context.Context, in *openolt.OnuPacket, opts ...grpc.CallOption) (*openolt.GemPortStatistics, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetGemPortStatistics", varargs...)
	ret0, _ := ret[0].(*openolt.GemPortStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetGemPortStatistics indicates an expected call of GetGemPortStatistics.
func (mr *MockOpenoltClientMockRecorder) GetGemPortStatistics(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetGemPortStatistics", reflect.TypeOf((*MockOpenoltClient)(nil).GetGemPortStatistics), varargs...)
}

// GetLogicalOnuDistance mocks base method.
func (m *MockOpenoltClient) GetLogicalOnuDistance(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.OnuLogicalDistance, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetLogicalOnuDistance", varargs...)
	ret0, _ := ret[0].(*openolt.OnuLogicalDistance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLogicalOnuDistance indicates an expected call of GetLogicalOnuDistance.
func (mr *MockOpenoltClientMockRecorder) GetLogicalOnuDistance(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogicalOnuDistance", reflect.TypeOf((*MockOpenoltClient)(nil).GetLogicalOnuDistance), varargs...)
}

// GetLogicalOnuDistanceZero mocks base method.
func (m *MockOpenoltClient) GetLogicalOnuDistanceZero(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.OnuLogicalDistance, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetLogicalOnuDistanceZero", varargs...)
	ret0, _ := ret[0].(*openolt.OnuLogicalDistance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLogicalOnuDistanceZero indicates an expected call of GetLogicalOnuDistanceZero.
func (mr *MockOpenoltClientMockRecorder) GetLogicalOnuDistanceZero(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogicalOnuDistanceZero", reflect.TypeOf((*MockOpenoltClient)(nil).GetLogicalOnuDistanceZero), varargs...)
}

// GetNniPortStatistics mocks base method.
func (m *MockOpenoltClient) GetNniPortStatistics(ctx context.Context, in *openolt.Interface, opts ...grpc.CallOption) (*common.PortStatistics, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetNniPortStatistics", varargs...)
	ret0, _ := ret[0].(*common.PortStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNniPortStatistics indicates an expected call of GetNniPortStatistics.
func (mr *MockOpenoltClientMockRecorder) GetNniPortStatistics(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNniPortStatistics", reflect.TypeOf((*MockOpenoltClient)(nil).GetNniPortStatistics), varargs...)
}

// GetOnuInfo mocks base method.
func (m *MockOpenoltClient) GetOnuInfo(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.OnuInfo, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetOnuInfo", varargs...)
	ret0, _ := ret[0].(*openolt.OnuInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOnuInfo indicates an expected call of GetOnuInfo.
func (mr *MockOpenoltClientMockRecorder) GetOnuInfo(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOnuInfo", reflect.TypeOf((*MockOpenoltClient)(nil).GetOnuInfo), varargs...)
}

// GetOnuStatistics mocks base method.
func (m *MockOpenoltClient) GetOnuStatistics(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.OnuStatistics, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetOnuStatistics", varargs...)
	ret0, _ := ret[0].(*openolt.OnuStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOnuStatistics indicates an expected call of GetOnuStatistics.
func (mr *MockOpenoltClientMockRecorder) GetOnuStatistics(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOnuStatistics", reflect.TypeOf((*MockOpenoltClient)(nil).GetOnuStatistics), varargs...)
}

// GetPonInterfaceInfo mocks base method.
func (m *MockOpenoltClient) GetPonInterfaceInfo(ctx context.Context, in *openolt.Interface, opts ...grpc.CallOption) (*openolt.PonIntfInfo, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetPonInterfaceInfo", varargs...)
	ret0, _ := ret[0].(*openolt.PonIntfInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPonInterfaceInfo indicates an expected call of GetPonInterfaceInfo.
func (mr *MockOpenoltClientMockRecorder) GetPonInterfaceInfo(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPonInterfaceInfo", reflect.TypeOf((*MockOpenoltClient)(nil).GetPonInterfaceInfo), varargs...)
}

// GetPonPortStatistics mocks base method.
func (m *MockOpenoltClient) GetPonPortStatistics(ctx context.Context, in *openolt.Interface, opts ...grpc.CallOption) (*common.PortStatistics, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetPonPortStatistics", varargs...)
	ret0, _ := ret[0].(*common.PortStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPonPortStatistics indicates an expected call of GetPonPortStatistics.
func (mr *MockOpenoltClientMockRecorder) GetPonPortStatistics(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPonPortStatistics", reflect.TypeOf((*MockOpenoltClient)(nil).GetPonPortStatistics), varargs...)
}

// GetPonRxPower mocks base method.
func (m *MockOpenoltClient) GetPonRxPower(ctx context.Context, in *openolt.Onu, opts ...grpc.CallOption) (*openolt.PonRxPowerData, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetPonRxPower", varargs...)
	ret0, _ := ret[0].(*openolt.PonRxPowerData)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPonRxPower indicates an expected call of GetPonRxPower.
func (mr *MockOpenoltClientMockRecorder) GetPonRxPower(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPonRxPower", reflect.TypeOf((*MockOpenoltClient)(nil).GetPonRxPower), varargs...)
}

// HeartbeatCheck mocks base method.
func (m *MockOpenoltClient) HeartbeatCheck(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Heartbeat, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "HeartbeatCheck", varargs...)
	ret0, _ := ret[0].(*openolt.Heartbeat)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HeartbeatCheck indicates an expected call of HeartbeatCheck.
func (mr *MockOpenoltClientMockRecorder) HeartbeatCheck(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HeartbeatCheck", reflect.TypeOf((*MockOpenoltClient)(nil).HeartbeatCheck), varargs...)
}

// OmciMsgOut mocks base method.
func (m *MockOpenoltClient) OmciMsgOut(ctx context.Context, in *openolt.OmciMsg, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "OmciMsgOut", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OmciMsgOut indicates an expected call of OmciMsgOut.
func (mr *MockOpenoltClientMockRecorder) OmciMsgOut(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OmciMsgOut", reflect.TypeOf((*MockOpenoltClient)(nil).OmciMsgOut), varargs...)
}

// OnuItuPonAlarmSet mocks base method.
func (m *MockOpenoltClient) OnuItuPonAlarmSet(ctx context.Context, in *config.OnuItuPonAlarm, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "OnuItuPonAlarmSet", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OnuItuPonAlarmSet indicates an expected call of OnuItuPonAlarmSet.
func (mr *MockOpenoltClientMockRecorder) OnuItuPonAlarmSet(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnuItuPonAlarmSet", reflect.TypeOf((*MockOpenoltClient)(nil).OnuItuPonAlarmSet), varargs...)
}

// OnuPacketOut mocks base method.
func (m *MockOpenoltClient) OnuPacketOut(ctx context.Context, in *openolt.OnuPacket, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "OnuPacketOut", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OnuPacketOut indicates an expected call of OnuPacketOut.
func (mr *MockOpenoltClientMockRecorder) OnuPacketOut(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnuPacketOut", reflect.TypeOf((*MockOpenoltClient)(nil).OnuPacketOut), varargs...)
}

// PerformGroupOperation mocks base method.
func (m *MockOpenoltClient) PerformGroupOperation(ctx context.Context, in *openolt.Group, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "PerformGroupOperation", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PerformGroupOperation indicates an expected call of PerformGroupOperation.
func (mr *MockOpenoltClientMockRecorder) PerformGroupOperation(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PerformGroupOperation", reflect.TypeOf((*MockOpenoltClient)(nil).PerformGroupOperation), varargs...)
}

// Reboot mocks base method.
func (m *MockOpenoltClient) Reboot(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Reboot", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Reboot indicates an expected call of Reboot.
func (mr *MockOpenoltClientMockRecorder) Reboot(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Reboot", reflect.TypeOf((*MockOpenoltClient)(nil).Reboot), varargs...)
}

// ReenableOlt mocks base method.
func (m *MockOpenoltClient) ReenableOlt(ctx context.Context, in *openolt.Empty, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ReenableOlt", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReenableOlt indicates an expected call of ReenableOlt.
func (mr *MockOpenoltClientMockRecorder) ReenableOlt(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReenableOlt", reflect.TypeOf((*MockOpenoltClient)(nil).ReenableOlt), varargs...)
}

// RemoveTrafficQueues mocks base method.
func (m *MockOpenoltClient) RemoveTrafficQueues(ctx context.Context, in *tech_profile.TrafficQueues, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "RemoveTrafficQueues", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RemoveTrafficQueues indicates an expected call of RemoveTrafficQueues.
func (mr *MockOpenoltClientMockRecorder) RemoveTrafficQueues(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveTrafficQueues", reflect.TypeOf((*MockOpenoltClient)(nil).RemoveTrafficQueues), varargs...)
}

// RemoveTrafficSchedulers mocks base method.
func (m *MockOpenoltClient) RemoveTrafficSchedulers(ctx context.Context, in *tech_profile.TrafficSchedulers, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "RemoveTrafficSchedulers", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RemoveTrafficSchedulers indicates an expected call of RemoveTrafficSchedulers.
func (mr *MockOpenoltClientMockRecorder) RemoveTrafficSchedulers(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveTrafficSchedulers", reflect.TypeOf((*MockOpenoltClient)(nil).RemoveTrafficSchedulers), varargs...)
}

// UplinkPacketOut mocks base method.
func (m *MockOpenoltClient) UplinkPacketOut(ctx context.Context, in *openolt.UplinkPacket, opts ...grpc.CallOption) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UplinkPacketOut", varargs...)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UplinkPacketOut indicates an expected call of UplinkPacketOut.
func (mr *MockOpenoltClientMockRecorder) UplinkPacketOut(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UplinkPacketOut", reflect.TypeOf((*MockOpenoltClient)(nil).UplinkPacketOut), varargs...)
}
