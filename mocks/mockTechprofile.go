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
	tp "github.com/opencord/voltha-go/common/techprofile"
	"github.com/opencord/voltha-go/db/model"
	tp_pb "github.com/opencord/voltha-protos/go/tech_profile"
)

// MockTechprofile mock struct for OpenoltClient.
type MockTechprofile struct {
	TpID uint32
}

// SetKVClient to mock tefhprofile SetKVClient method
func (m *MockTechprofile) SetKVClient() *model.Backend {
	return &model.Backend{Client: &MockKVClient{}}
}

// GetTechProfileInstanceKVPath to mock tefhprofile GetTechProfileInstanceKVPath method
func (m *MockTechprofile) GetTechProfileInstanceKVPath(techProfiletblID uint32, uniPortName string) string {
	return ""

}

// GetTPInstanceFromKVStore to mock tefhprofile GetTPInstanceFromKVStore method
func (m *MockTechprofile) GetTPInstanceFromKVStore(techProfiletblID uint32, path string) (*tp.TechProfile, error) {
	return nil, nil

}

// CreateTechProfInstance to mock tefhprofile CreateTechProfInstance method
func (m *MockTechprofile) CreateTechProfInstance(techProfiletblID uint32, uniPortName string, intfId uint32) *tp.TechProfile {
	return nil
}

// DeleteTechProfileInstance to mock tefhprofile DeleteTechProfileInstance method
func (m *MockTechprofile) DeleteTechProfileInstance(techProfiletblID uint32, uniPortName string) error {
	return nil

}

// GetprotoBufParamValue to mock tefhprofile GetprotoBufParamValue method
func (m *MockTechprofile) GetprotoBufParamValue(paramType string, paramKey string) int32 {
	return 0

}

// GetUsScheduler to mock tefhprofile GetUsScheduler method
func (m *MockTechprofile) GetUsScheduler(tpInstance *tp.TechProfile) *tp_pb.SchedulerConfig {
	return &tp_pb.SchedulerConfig{}

}

// GetDsScheduler to mock tefhprofile GetDsScheduler method
func (m *MockTechprofile) GetDsScheduler(tpInstance *tp.TechProfile) *tp_pb.SchedulerConfig {
	return &tp_pb.SchedulerConfig{}
}

// GetTrafficScheduler to mock tefhprofile GetTrafficScheduler method
func (m *MockTechprofile) GetTrafficScheduler(tpInstance *tp.TechProfile, SchedCfg *tp_pb.SchedulerConfig,
	ShapingCfg *tp_pb.TrafficShapingInfo) *tp_pb.TrafficScheduler {
	return &tp_pb.TrafficScheduler{}

}

// GetTrafficQueues to mock tefhprofile GetTrafficQueues method
func (m *MockTechprofile) GetTrafficQueues(tp *tp.TechProfile, Dir tp_pb.Direction) []*tp_pb.TrafficQueue {
	return []*tp_pb.TrafficQueue{{}}
}

// GetGemportIDForPbit to mock tefhprofile GetGemportIDForPbit method
func (m *MockTechprofile) GetGemportIDForPbit(tp *tp.TechProfile, Dir tp_pb.Direction, pbit uint32) uint32 {
	return 0
}
