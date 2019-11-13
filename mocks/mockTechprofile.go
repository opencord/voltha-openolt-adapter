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
	"github.com/opencord/voltha-lib-go/v2/pkg/db"
	"github.com/opencord/voltha-lib-go/v2/pkg/log"
	tp "github.com/opencord/voltha-lib-go/v2/pkg/techprofile"
	tp_pb "github.com/opencord/voltha-protos/v2/go/tech_profile"
)

// MockTechProfile mock struct for OpenoltClient.
type MockTechProfile struct {
	TpID uint32
}

// SetKVClient to mock techprofile SetKVClient method
func (m MockTechProfile) SetKVClient() *db.Backend {
	return &db.Backend{Client: &MockKVClient{}}
}

// GetTechProfileInstanceKVPath to mock techprofile GetTechProfileInstanceKVPath method
func (m MockTechProfile) GetTechProfileInstanceKVPath(techProfiletblID uint32, uniPortName string) string {
	return ""

}

// GetTPInstanceFromKVStore to mock techprofile GetTPInstanceFromKVStore method
func (m MockTechProfile) GetTPInstanceFromKVStore(techProfiletblID uint32, path string) (*tp.TechProfile, error) {
	log.Debug("Warning Warning Warning: GetTPInstanceFromKVStore")
	return nil, nil

}

// CreateTechProfInstance to mock techprofile CreateTechProfInstance method
func (m MockTechProfile) CreateTechProfInstance(techProfiletblID uint32, uniPortName string, intfID uint32) *tp.TechProfile {

	return &tp.TechProfile{
		Name:                           "mock-tech-profile",
		SubscriberIdentifier:           "257",
		ProfileType:                    "mock",
		Version:                        0,
		NumGemPorts:                    2,
		UpstreamGemPortAttributeList:   nil,
		DownstreamGemPortAttributeList: nil,
	}

}

// DeleteTechProfileInstance to mock techprofile DeleteTechProfileInstance method
func (m MockTechProfile) DeleteTechProfileInstance(techProfiletblID uint32, uniPortName string) error {
	return nil
}

// GetprotoBufParamValue to mock techprofile GetprotoBufParamValue method
func (m MockTechProfile) GetprotoBufParamValue(paramType string, paramKey string) int32 {
	return 0

}

// GetUsScheduler to mock techprofile GetUsScheduler method
func (m MockTechProfile) GetUsScheduler(tpInstance *tp.TechProfile) *tp_pb.SchedulerConfig {
	return &tp_pb.SchedulerConfig{}

}

// GetDsScheduler to mock techprofile GetDsScheduler method
func (m MockTechProfile) GetDsScheduler(tpInstance *tp.TechProfile) *tp_pb.SchedulerConfig {
	return &tp_pb.SchedulerConfig{}
}

// GetTrafficScheduler to mock techprofile GetTrafficScheduler method
func (m MockTechProfile) GetTrafficScheduler(tpInstance *tp.TechProfile, SchedCfg *tp_pb.SchedulerConfig,
	ShapingCfg *tp_pb.TrafficShapingInfo) *tp_pb.TrafficScheduler {
	return &tp_pb.TrafficScheduler{}

}

// GetTrafficQueues to mock techprofile GetTrafficQueues method
func (m MockTechProfile) GetTrafficQueues(tp *tp.TechProfile, Dir tp_pb.Direction) []*tp_pb.TrafficQueue {
	return []*tp_pb.TrafficQueue{{}}
}

// GetGemportIDForPbit to mock techprofile GetGemportIDForPbit method
func (m MockTechProfile) GetGemportIDForPbit(tp *tp.TechProfile, Dir tp_pb.Direction, pbit uint32) uint32 {
	return 0
}

// FindAllTpInstances to mock techprofile FindAllTpInstances method
func (m MockTechProfile) FindAllTpInstances(techProfiletblID uint32, ponIntf uint32, onuID uint32) []tp.TechProfile {
	return []tp.TechProfile{}
}
