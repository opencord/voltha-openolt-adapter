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
	metadata "google.golang.org/grpc/metadata"
)

// MockisIndication_Data is a mock of isIndication_Data interface.
type MockisIndication_Data struct {
	ctrl     *gomock.Controller
	recorder *MockisIndication_DataMockRecorder
	isgomock struct{}
}

// MockisIndication_DataMockRecorder is the mock recorder for MockisIndication_Data.
type MockisIndication_DataMockRecorder struct {
	mock *MockisIndication_Data
}

// NewMockisIndication_Data creates a new mock instance.
func NewMockisIndication_Data(ctrl *gomock.Controller) *MockisIndication_Data {
	mock := &MockisIndication_Data{ctrl: ctrl}
	mock.recorder = &MockisIndication_DataMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockisIndication_Data) EXPECT() *MockisIndication_DataMockRecorder {
	return m.recorder
}

// isIndication_Data mocks base method.
func (m *MockisIndication_Data) isIndication_Data() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "isIndication_Data")
}

// isIndication_Data indicates an expected call of isIndication_Data.
func (mr *MockisIndication_DataMockRecorder) isIndication_Data() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "isIndication_Data", reflect.TypeOf((*MockisIndication_Data)(nil).isIndication_Data))
}

// MockisAlarmIndication_Data is a mock of isAlarmIndication_Data interface.
type MockisAlarmIndication_Data struct {
	ctrl     *gomock.Controller
	recorder *MockisAlarmIndication_DataMockRecorder
	isgomock struct{}
}

// MockisAlarmIndication_DataMockRecorder is the mock recorder for MockisAlarmIndication_Data.
type MockisAlarmIndication_DataMockRecorder struct {
	mock *MockisAlarmIndication_Data
}

// NewMockisAlarmIndication_Data creates a new mock instance.
func NewMockisAlarmIndication_Data(ctrl *gomock.Controller) *MockisAlarmIndication_Data {
	mock := &MockisAlarmIndication_Data{ctrl: ctrl}
	mock.recorder = &MockisAlarmIndication_DataMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockisAlarmIndication_Data) EXPECT() *MockisAlarmIndication_DataMockRecorder {
	return m.recorder
}

// isAlarmIndication_Data mocks base method.
func (m *MockisAlarmIndication_Data) isAlarmIndication_Data() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "isAlarmIndication_Data")
}

// isAlarmIndication_Data indicates an expected call of isAlarmIndication_Data.
func (mr *MockisAlarmIndication_DataMockRecorder) isAlarmIndication_Data() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "isAlarmIndication_Data", reflect.TypeOf((*MockisAlarmIndication_Data)(nil).isAlarmIndication_Data))
}

// MockisOnuItuPonStatsIndication_Stats is a mock of isOnuItuPonStatsIndication_Stats interface.
type MockisOnuItuPonStatsIndication_Stats struct {
	ctrl     *gomock.Controller
	recorder *MockisOnuItuPonStatsIndication_StatsMockRecorder
	isgomock struct{}
}

// MockisOnuItuPonStatsIndication_StatsMockRecorder is the mock recorder for MockisOnuItuPonStatsIndication_Stats.
type MockisOnuItuPonStatsIndication_StatsMockRecorder struct {
	mock *MockisOnuItuPonStatsIndication_Stats
}

// NewMockisOnuItuPonStatsIndication_Stats creates a new mock instance.
func NewMockisOnuItuPonStatsIndication_Stats(ctrl *gomock.Controller) *MockisOnuItuPonStatsIndication_Stats {
	mock := &MockisOnuItuPonStatsIndication_Stats{ctrl: ctrl}
	mock.recorder = &MockisOnuItuPonStatsIndication_StatsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockisOnuItuPonStatsIndication_Stats) EXPECT() *MockisOnuItuPonStatsIndication_StatsMockRecorder {
	return m.recorder
}

// isOnuItuPonStatsIndication_Stats mocks base method.
func (m *MockisOnuItuPonStatsIndication_Stats) isOnuItuPonStatsIndication_Stats() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "isOnuItuPonStatsIndication_Stats")
}

// isOnuItuPonStatsIndication_Stats indicates an expected call of isOnuItuPonStatsIndication_Stats.
func (mr *MockisOnuItuPonStatsIndication_StatsMockRecorder) isOnuItuPonStatsIndication_Stats() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "isOnuItuPonStatsIndication_Stats", reflect.TypeOf((*MockisOnuItuPonStatsIndication_Stats)(nil).isOnuItuPonStatsIndication_Stats))
}

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

// MockOpenolt_EnableIndicationClient is a mock of Openolt_EnableIndicationClient interface.
type MockOpenolt_EnableIndicationClient struct {
	ctrl     *gomock.Controller
	recorder *MockOpenolt_EnableIndicationClientMockRecorder
	isgomock struct{}
}

// MockOpenolt_EnableIndicationClientMockRecorder is the mock recorder for MockOpenolt_EnableIndicationClient.
type MockOpenolt_EnableIndicationClientMockRecorder struct {
	mock *MockOpenolt_EnableIndicationClient
}

// NewMockOpenolt_EnableIndicationClient creates a new mock instance.
func NewMockOpenolt_EnableIndicationClient(ctrl *gomock.Controller) *MockOpenolt_EnableIndicationClient {
	mock := &MockOpenolt_EnableIndicationClient{ctrl: ctrl}
	mock.recorder = &MockOpenolt_EnableIndicationClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOpenolt_EnableIndicationClient) EXPECT() *MockOpenolt_EnableIndicationClientMockRecorder {
	return m.recorder
}

// CloseSend mocks base method.
func (m *MockOpenolt_EnableIndicationClient) CloseSend() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseSend")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseSend indicates an expected call of CloseSend.
func (mr *MockOpenolt_EnableIndicationClientMockRecorder) CloseSend() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseSend", reflect.TypeOf((*MockOpenolt_EnableIndicationClient)(nil).CloseSend))
}

// Context mocks base method.
func (m *MockOpenolt_EnableIndicationClient) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context.
func (mr *MockOpenolt_EnableIndicationClientMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockOpenolt_EnableIndicationClient)(nil).Context))
}

// Header mocks base method.
func (m *MockOpenolt_EnableIndicationClient) Header() (metadata.MD, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Header")
	ret0, _ := ret[0].(metadata.MD)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Header indicates an expected call of Header.
func (mr *MockOpenolt_EnableIndicationClientMockRecorder) Header() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Header", reflect.TypeOf((*MockOpenolt_EnableIndicationClient)(nil).Header))
}

// Recv mocks base method.
func (m *MockOpenolt_EnableIndicationClient) Recv() (*openolt.Indication, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Recv")
	ret0, _ := ret[0].(*openolt.Indication)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Recv indicates an expected call of Recv.
func (mr *MockOpenolt_EnableIndicationClientMockRecorder) Recv() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Recv", reflect.TypeOf((*MockOpenolt_EnableIndicationClient)(nil).Recv))
}

// RecvMsg mocks base method.
func (m_2 *MockOpenolt_EnableIndicationClient) RecvMsg(m any) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "RecvMsg", m)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg.
func (mr *MockOpenolt_EnableIndicationClientMockRecorder) RecvMsg(m any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockOpenolt_EnableIndicationClient)(nil).RecvMsg), m)
}

// SendMsg mocks base method.
func (m_2 *MockOpenolt_EnableIndicationClient) SendMsg(m any) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "SendMsg", m)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg.
func (mr *MockOpenolt_EnableIndicationClientMockRecorder) SendMsg(m any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockOpenolt_EnableIndicationClient)(nil).SendMsg), m)
}

// Trailer mocks base method.
func (m *MockOpenolt_EnableIndicationClient) Trailer() metadata.MD {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Trailer")
	ret0, _ := ret[0].(metadata.MD)
	return ret0
}

// Trailer indicates an expected call of Trailer.
func (mr *MockOpenolt_EnableIndicationClientMockRecorder) Trailer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Trailer", reflect.TypeOf((*MockOpenolt_EnableIndicationClient)(nil).Trailer))
}

// MockOpenoltServer is a mock of OpenoltServer interface.
type MockOpenoltServer struct {
	ctrl     *gomock.Controller
	recorder *MockOpenoltServerMockRecorder
	isgomock struct{}
}

// MockOpenoltServerMockRecorder is the mock recorder for MockOpenoltServer.
type MockOpenoltServerMockRecorder struct {
	mock *MockOpenoltServer
}

// NewMockOpenoltServer creates a new mock instance.
func NewMockOpenoltServer(ctrl *gomock.Controller) *MockOpenoltServer {
	mock := &MockOpenoltServer{ctrl: ctrl}
	mock.recorder = &MockOpenoltServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOpenoltServer) EXPECT() *MockOpenoltServerMockRecorder {
	return m.recorder
}

// ActivateOnu mocks base method.
func (m *MockOpenoltServer) ActivateOnu(arg0 context.Context, arg1 *openolt.Onu) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ActivateOnu", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ActivateOnu indicates an expected call of ActivateOnu.
func (mr *MockOpenoltServerMockRecorder) ActivateOnu(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActivateOnu", reflect.TypeOf((*MockOpenoltServer)(nil).ActivateOnu), arg0, arg1)
}

// CollectStatistics mocks base method.
func (m *MockOpenoltServer) CollectStatistics(arg0 context.Context, arg1 *openolt.Empty) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CollectStatistics", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CollectStatistics indicates an expected call of CollectStatistics.
func (mr *MockOpenoltServerMockRecorder) CollectStatistics(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CollectStatistics", reflect.TypeOf((*MockOpenoltServer)(nil).CollectStatistics), arg0, arg1)
}

// CreateTrafficQueues mocks base method.
func (m *MockOpenoltServer) CreateTrafficQueues(arg0 context.Context, arg1 *tech_profile.TrafficQueues) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTrafficQueues", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateTrafficQueues indicates an expected call of CreateTrafficQueues.
func (mr *MockOpenoltServerMockRecorder) CreateTrafficQueues(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTrafficQueues", reflect.TypeOf((*MockOpenoltServer)(nil).CreateTrafficQueues), arg0, arg1)
}

// CreateTrafficSchedulers mocks base method.
func (m *MockOpenoltServer) CreateTrafficSchedulers(arg0 context.Context, arg1 *tech_profile.TrafficSchedulers) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTrafficSchedulers", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateTrafficSchedulers indicates an expected call of CreateTrafficSchedulers.
func (mr *MockOpenoltServerMockRecorder) CreateTrafficSchedulers(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTrafficSchedulers", reflect.TypeOf((*MockOpenoltServer)(nil).CreateTrafficSchedulers), arg0, arg1)
}

// DeactivateOnu mocks base method.
func (m *MockOpenoltServer) DeactivateOnu(arg0 context.Context, arg1 *openolt.Onu) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeactivateOnu", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeactivateOnu indicates an expected call of DeactivateOnu.
func (mr *MockOpenoltServerMockRecorder) DeactivateOnu(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeactivateOnu", reflect.TypeOf((*MockOpenoltServer)(nil).DeactivateOnu), arg0, arg1)
}

// DeleteGroup mocks base method.
func (m *MockOpenoltServer) DeleteGroup(arg0 context.Context, arg1 *openolt.Group) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteGroup", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteGroup indicates an expected call of DeleteGroup.
func (mr *MockOpenoltServerMockRecorder) DeleteGroup(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteGroup", reflect.TypeOf((*MockOpenoltServer)(nil).DeleteGroup), arg0, arg1)
}

// DeleteOnu mocks base method.
func (m *MockOpenoltServer) DeleteOnu(arg0 context.Context, arg1 *openolt.Onu) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteOnu", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteOnu indicates an expected call of DeleteOnu.
func (mr *MockOpenoltServerMockRecorder) DeleteOnu(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteOnu", reflect.TypeOf((*MockOpenoltServer)(nil).DeleteOnu), arg0, arg1)
}

// DisableOlt mocks base method.
func (m *MockOpenoltServer) DisableOlt(arg0 context.Context, arg1 *openolt.Empty) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DisableOlt", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DisableOlt indicates an expected call of DisableOlt.
func (mr *MockOpenoltServerMockRecorder) DisableOlt(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DisableOlt", reflect.TypeOf((*MockOpenoltServer)(nil).DisableOlt), arg0, arg1)
}

// DisablePonIf mocks base method.
func (m *MockOpenoltServer) DisablePonIf(arg0 context.Context, arg1 *openolt.Interface) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DisablePonIf", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DisablePonIf indicates an expected call of DisablePonIf.
func (mr *MockOpenoltServerMockRecorder) DisablePonIf(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DisablePonIf", reflect.TypeOf((*MockOpenoltServer)(nil).DisablePonIf), arg0, arg1)
}

// EnableIndication mocks base method.
func (m *MockOpenoltServer) EnableIndication(arg0 *openolt.Empty, arg1 openolt.Openolt_EnableIndicationServer) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnableIndication", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// EnableIndication indicates an expected call of EnableIndication.
func (mr *MockOpenoltServerMockRecorder) EnableIndication(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnableIndication", reflect.TypeOf((*MockOpenoltServer)(nil).EnableIndication), arg0, arg1)
}

// EnablePonIf mocks base method.
func (m *MockOpenoltServer) EnablePonIf(arg0 context.Context, arg1 *openolt.Interface) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnablePonIf", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EnablePonIf indicates an expected call of EnablePonIf.
func (mr *MockOpenoltServerMockRecorder) EnablePonIf(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnablePonIf", reflect.TypeOf((*MockOpenoltServer)(nil).EnablePonIf), arg0, arg1)
}

// FlowAdd mocks base method.
func (m *MockOpenoltServer) FlowAdd(arg0 context.Context, arg1 *openolt.Flow) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FlowAdd", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FlowAdd indicates an expected call of FlowAdd.
func (mr *MockOpenoltServerMockRecorder) FlowAdd(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FlowAdd", reflect.TypeOf((*MockOpenoltServer)(nil).FlowAdd), arg0, arg1)
}

// FlowRemove mocks base method.
func (m *MockOpenoltServer) FlowRemove(arg0 context.Context, arg1 *openolt.Flow) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FlowRemove", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FlowRemove indicates an expected call of FlowRemove.
func (mr *MockOpenoltServerMockRecorder) FlowRemove(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FlowRemove", reflect.TypeOf((*MockOpenoltServer)(nil).FlowRemove), arg0, arg1)
}

// GetAllocIdStatistics mocks base method.
func (m *MockOpenoltServer) GetAllocIdStatistics(arg0 context.Context, arg1 *openolt.OnuPacket) (*openolt.OnuAllocIdStatistics, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllocIdStatistics", arg0, arg1)
	ret0, _ := ret[0].(*openolt.OnuAllocIdStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllocIdStatistics indicates an expected call of GetAllocIdStatistics.
func (mr *MockOpenoltServerMockRecorder) GetAllocIdStatistics(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllocIdStatistics", reflect.TypeOf((*MockOpenoltServer)(nil).GetAllocIdStatistics), arg0, arg1)
}

// GetDeviceInfo mocks base method.
func (m *MockOpenoltServer) GetDeviceInfo(arg0 context.Context, arg1 *openolt.Empty) (*openolt.DeviceInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeviceInfo", arg0, arg1)
	ret0, _ := ret[0].(*openolt.DeviceInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeviceInfo indicates an expected call of GetDeviceInfo.
func (mr *MockOpenoltServerMockRecorder) GetDeviceInfo(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeviceInfo", reflect.TypeOf((*MockOpenoltServer)(nil).GetDeviceInfo), arg0, arg1)
}

// GetExtValue mocks base method.
func (m *MockOpenoltServer) GetExtValue(arg0 context.Context, arg1 *openolt.ValueParam) (*extension.ReturnValues, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetExtValue", arg0, arg1)
	ret0, _ := ret[0].(*extension.ReturnValues)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExtValue indicates an expected call of GetExtValue.
func (mr *MockOpenoltServerMockRecorder) GetExtValue(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExtValue", reflect.TypeOf((*MockOpenoltServer)(nil).GetExtValue), arg0, arg1)
}

// GetGemPortStatistics mocks base method.
func (m *MockOpenoltServer) GetGemPortStatistics(arg0 context.Context, arg1 *openolt.OnuPacket) (*openolt.GemPortStatistics, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetGemPortStatistics", arg0, arg1)
	ret0, _ := ret[0].(*openolt.GemPortStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetGemPortStatistics indicates an expected call of GetGemPortStatistics.
func (mr *MockOpenoltServerMockRecorder) GetGemPortStatistics(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetGemPortStatistics", reflect.TypeOf((*MockOpenoltServer)(nil).GetGemPortStatistics), arg0, arg1)
}

// GetLogicalOnuDistance mocks base method.
func (m *MockOpenoltServer) GetLogicalOnuDistance(arg0 context.Context, arg1 *openolt.Onu) (*openolt.OnuLogicalDistance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLogicalOnuDistance", arg0, arg1)
	ret0, _ := ret[0].(*openolt.OnuLogicalDistance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLogicalOnuDistance indicates an expected call of GetLogicalOnuDistance.
func (mr *MockOpenoltServerMockRecorder) GetLogicalOnuDistance(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogicalOnuDistance", reflect.TypeOf((*MockOpenoltServer)(nil).GetLogicalOnuDistance), arg0, arg1)
}

// GetLogicalOnuDistanceZero mocks base method.
func (m *MockOpenoltServer) GetLogicalOnuDistanceZero(arg0 context.Context, arg1 *openolt.Onu) (*openolt.OnuLogicalDistance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLogicalOnuDistanceZero", arg0, arg1)
	ret0, _ := ret[0].(*openolt.OnuLogicalDistance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLogicalOnuDistanceZero indicates an expected call of GetLogicalOnuDistanceZero.
func (mr *MockOpenoltServerMockRecorder) GetLogicalOnuDistanceZero(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogicalOnuDistanceZero", reflect.TypeOf((*MockOpenoltServer)(nil).GetLogicalOnuDistanceZero), arg0, arg1)
}

// GetNniPortStatistics mocks base method.
func (m *MockOpenoltServer) GetNniPortStatistics(arg0 context.Context, arg1 *openolt.Interface) (*common.PortStatistics, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNniPortStatistics", arg0, arg1)
	ret0, _ := ret[0].(*common.PortStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNniPortStatistics indicates an expected call of GetNniPortStatistics.
func (mr *MockOpenoltServerMockRecorder) GetNniPortStatistics(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNniPortStatistics", reflect.TypeOf((*MockOpenoltServer)(nil).GetNniPortStatistics), arg0, arg1)
}

// GetOnuInfo mocks base method.
func (m *MockOpenoltServer) GetOnuInfo(arg0 context.Context, arg1 *openolt.Onu) (*openolt.OnuInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOnuInfo", arg0, arg1)
	ret0, _ := ret[0].(*openolt.OnuInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOnuInfo indicates an expected call of GetOnuInfo.
func (mr *MockOpenoltServerMockRecorder) GetOnuInfo(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOnuInfo", reflect.TypeOf((*MockOpenoltServer)(nil).GetOnuInfo), arg0, arg1)
}

// GetOnuStatistics mocks base method.
func (m *MockOpenoltServer) GetOnuStatistics(arg0 context.Context, arg1 *openolt.Onu) (*openolt.OnuStatistics, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOnuStatistics", arg0, arg1)
	ret0, _ := ret[0].(*openolt.OnuStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOnuStatistics indicates an expected call of GetOnuStatistics.
func (mr *MockOpenoltServerMockRecorder) GetOnuStatistics(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOnuStatistics", reflect.TypeOf((*MockOpenoltServer)(nil).GetOnuStatistics), arg0, arg1)
}

// GetPonInterfaceInfo mocks base method.
func (m *MockOpenoltServer) GetPonInterfaceInfo(arg0 context.Context, arg1 *openolt.Interface) (*openolt.PonIntfInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPonInterfaceInfo", arg0, arg1)
	ret0, _ := ret[0].(*openolt.PonIntfInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPonInterfaceInfo indicates an expected call of GetPonInterfaceInfo.
func (mr *MockOpenoltServerMockRecorder) GetPonInterfaceInfo(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPonInterfaceInfo", reflect.TypeOf((*MockOpenoltServer)(nil).GetPonInterfaceInfo), arg0, arg1)
}

// GetPonPortStatistics mocks base method.
func (m *MockOpenoltServer) GetPonPortStatistics(arg0 context.Context, arg1 *openolt.Interface) (*common.PortStatistics, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPonPortStatistics", arg0, arg1)
	ret0, _ := ret[0].(*common.PortStatistics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPonPortStatistics indicates an expected call of GetPonPortStatistics.
func (mr *MockOpenoltServerMockRecorder) GetPonPortStatistics(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPonPortStatistics", reflect.TypeOf((*MockOpenoltServer)(nil).GetPonPortStatistics), arg0, arg1)
}

// GetPonRxPower mocks base method.
func (m *MockOpenoltServer) GetPonRxPower(arg0 context.Context, arg1 *openolt.Onu) (*openolt.PonRxPowerData, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPonRxPower", arg0, arg1)
	ret0, _ := ret[0].(*openolt.PonRxPowerData)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPonRxPower indicates an expected call of GetPonRxPower.
func (mr *MockOpenoltServerMockRecorder) GetPonRxPower(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPonRxPower", reflect.TypeOf((*MockOpenoltServer)(nil).GetPonRxPower), arg0, arg1)
}

// HeartbeatCheck mocks base method.
func (m *MockOpenoltServer) HeartbeatCheck(arg0 context.Context, arg1 *openolt.Empty) (*openolt.Heartbeat, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HeartbeatCheck", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Heartbeat)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HeartbeatCheck indicates an expected call of HeartbeatCheck.
func (mr *MockOpenoltServerMockRecorder) HeartbeatCheck(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HeartbeatCheck", reflect.TypeOf((*MockOpenoltServer)(nil).HeartbeatCheck), arg0, arg1)
}

// OmciMsgOut mocks base method.
func (m *MockOpenoltServer) OmciMsgOut(arg0 context.Context, arg1 *openolt.OmciMsg) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OmciMsgOut", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OmciMsgOut indicates an expected call of OmciMsgOut.
func (mr *MockOpenoltServerMockRecorder) OmciMsgOut(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OmciMsgOut", reflect.TypeOf((*MockOpenoltServer)(nil).OmciMsgOut), arg0, arg1)
}

// OnuItuPonAlarmSet mocks base method.
func (m *MockOpenoltServer) OnuItuPonAlarmSet(arg0 context.Context, arg1 *config.OnuItuPonAlarm) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OnuItuPonAlarmSet", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OnuItuPonAlarmSet indicates an expected call of OnuItuPonAlarmSet.
func (mr *MockOpenoltServerMockRecorder) OnuItuPonAlarmSet(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnuItuPonAlarmSet", reflect.TypeOf((*MockOpenoltServer)(nil).OnuItuPonAlarmSet), arg0, arg1)
}

// OnuPacketOut mocks base method.
func (m *MockOpenoltServer) OnuPacketOut(arg0 context.Context, arg1 *openolt.OnuPacket) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OnuPacketOut", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OnuPacketOut indicates an expected call of OnuPacketOut.
func (mr *MockOpenoltServerMockRecorder) OnuPacketOut(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnuPacketOut", reflect.TypeOf((*MockOpenoltServer)(nil).OnuPacketOut), arg0, arg1)
}

// PerformGroupOperation mocks base method.
func (m *MockOpenoltServer) PerformGroupOperation(arg0 context.Context, arg1 *openolt.Group) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PerformGroupOperation", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PerformGroupOperation indicates an expected call of PerformGroupOperation.
func (mr *MockOpenoltServerMockRecorder) PerformGroupOperation(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PerformGroupOperation", reflect.TypeOf((*MockOpenoltServer)(nil).PerformGroupOperation), arg0, arg1)
}

// Reboot mocks base method.
func (m *MockOpenoltServer) Reboot(arg0 context.Context, arg1 *openolt.Empty) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Reboot", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Reboot indicates an expected call of Reboot.
func (mr *MockOpenoltServerMockRecorder) Reboot(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Reboot", reflect.TypeOf((*MockOpenoltServer)(nil).Reboot), arg0, arg1)
}

// ReenableOlt mocks base method.
func (m *MockOpenoltServer) ReenableOlt(arg0 context.Context, arg1 *openolt.Empty) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReenableOlt", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReenableOlt indicates an expected call of ReenableOlt.
func (mr *MockOpenoltServerMockRecorder) ReenableOlt(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReenableOlt", reflect.TypeOf((*MockOpenoltServer)(nil).ReenableOlt), arg0, arg1)
}

// RemoveTrafficQueues mocks base method.
func (m *MockOpenoltServer) RemoveTrafficQueues(arg0 context.Context, arg1 *tech_profile.TrafficQueues) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveTrafficQueues", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RemoveTrafficQueues indicates an expected call of RemoveTrafficQueues.
func (mr *MockOpenoltServerMockRecorder) RemoveTrafficQueues(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveTrafficQueues", reflect.TypeOf((*MockOpenoltServer)(nil).RemoveTrafficQueues), arg0, arg1)
}

// RemoveTrafficSchedulers mocks base method.
func (m *MockOpenoltServer) RemoveTrafficSchedulers(arg0 context.Context, arg1 *tech_profile.TrafficSchedulers) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveTrafficSchedulers", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RemoveTrafficSchedulers indicates an expected call of RemoveTrafficSchedulers.
func (mr *MockOpenoltServerMockRecorder) RemoveTrafficSchedulers(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveTrafficSchedulers", reflect.TypeOf((*MockOpenoltServer)(nil).RemoveTrafficSchedulers), arg0, arg1)
}

// UplinkPacketOut mocks base method.
func (m *MockOpenoltServer) UplinkPacketOut(arg0 context.Context, arg1 *openolt.UplinkPacket) (*openolt.Empty, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UplinkPacketOut", arg0, arg1)
	ret0, _ := ret[0].(*openolt.Empty)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UplinkPacketOut indicates an expected call of UplinkPacketOut.
func (mr *MockOpenoltServerMockRecorder) UplinkPacketOut(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UplinkPacketOut", reflect.TypeOf((*MockOpenoltServer)(nil).UplinkPacketOut), arg0, arg1)
}

// MockOpenolt_EnableIndicationServer is a mock of Openolt_EnableIndicationServer interface.
type MockOpenolt_EnableIndicationServer struct {
	ctrl     *gomock.Controller
	recorder *MockOpenolt_EnableIndicationServerMockRecorder
	isgomock struct{}
}

// MockOpenolt_EnableIndicationServerMockRecorder is the mock recorder for MockOpenolt_EnableIndicationServer.
type MockOpenolt_EnableIndicationServerMockRecorder struct {
	mock *MockOpenolt_EnableIndicationServer
}

// NewMockOpenolt_EnableIndicationServer creates a new mock instance.
func NewMockOpenolt_EnableIndicationServer(ctrl *gomock.Controller) *MockOpenolt_EnableIndicationServer {
	mock := &MockOpenolt_EnableIndicationServer{ctrl: ctrl}
	mock.recorder = &MockOpenolt_EnableIndicationServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOpenolt_EnableIndicationServer) EXPECT() *MockOpenolt_EnableIndicationServerMockRecorder {
	return m.recorder
}

// Context mocks base method.
func (m *MockOpenolt_EnableIndicationServer) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context.
func (mr *MockOpenolt_EnableIndicationServerMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockOpenolt_EnableIndicationServer)(nil).Context))
}

// RecvMsg mocks base method.
func (m_2 *MockOpenolt_EnableIndicationServer) RecvMsg(m any) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "RecvMsg", m)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg.
func (mr *MockOpenolt_EnableIndicationServerMockRecorder) RecvMsg(m any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockOpenolt_EnableIndicationServer)(nil).RecvMsg), m)
}

// Send mocks base method.
func (m *MockOpenolt_EnableIndicationServer) Send(arg0 *openolt.Indication) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send.
func (mr *MockOpenolt_EnableIndicationServerMockRecorder) Send(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockOpenolt_EnableIndicationServer)(nil).Send), arg0)
}

// SendHeader mocks base method.
func (m *MockOpenolt_EnableIndicationServer) SendHeader(arg0 metadata.MD) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendHeader indicates an expected call of SendHeader.
func (mr *MockOpenolt_EnableIndicationServerMockRecorder) SendHeader(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendHeader", reflect.TypeOf((*MockOpenolt_EnableIndicationServer)(nil).SendHeader), arg0)
}

// SendMsg mocks base method.
func (m_2 *MockOpenolt_EnableIndicationServer) SendMsg(m any) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "SendMsg", m)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg.
func (mr *MockOpenolt_EnableIndicationServerMockRecorder) SendMsg(m any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockOpenolt_EnableIndicationServer)(nil).SendMsg), m)
}

// SetHeader mocks base method.
func (m *MockOpenolt_EnableIndicationServer) SetHeader(arg0 metadata.MD) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetHeader indicates an expected call of SetHeader.
func (mr *MockOpenolt_EnableIndicationServerMockRecorder) SetHeader(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHeader", reflect.TypeOf((*MockOpenolt_EnableIndicationServer)(nil).SetHeader), arg0)
}

// SetTrailer mocks base method.
func (m *MockOpenolt_EnableIndicationServer) SetTrailer(arg0 metadata.MD) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetTrailer", arg0)
}

// SetTrailer indicates an expected call of SetTrailer.
func (mr *MockOpenolt_EnableIndicationServerMockRecorder) SetTrailer(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTrailer", reflect.TypeOf((*MockOpenolt_EnableIndicationServer)(nil).SetTrailer), arg0)
}
