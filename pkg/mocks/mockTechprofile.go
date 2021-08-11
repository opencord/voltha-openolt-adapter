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

	"github.com/opencord/voltha-lib-go/v7/pkg/db"
	tp_pb "github.com/opencord/voltha-protos/v5/go/tech_profile"
)

// MockTechProfile mock struct for OpenoltClient.
type MockTechProfile struct {
	TpID uint32
}

// SetKVClient to mock techprofile SetKVClient method
func (m MockTechProfile) SetKVClient(ctx context.Context, pathPrefix string) *db.Backend {
	return &db.Backend{Client: &MockKVClient{}}
}

// GetTPInstance to mock techprofile GetTPInstance method
func (m MockTechProfile) GetTPInstance(ctx context.Context, path string) (interface{}, error) {
	logger.Debug(ctx, "GetTPInstanceFromKVStore")
	var usGemPortAttributeList []*tp_pb.GemPortAttributes
	var dsGemPortAttributeList []*tp_pb.GemPortAttributes
	usGemPortAttributeList = append(usGemPortAttributeList, &tp_pb.GemPortAttributes{
		GemportId: 1,
		PbitMap:   "0b11111111",
	})
	dsGemPortAttributeList = append(dsGemPortAttributeList, &tp_pb.GemPortAttributes{
		GemportId: 1,
		PbitMap:   "0b11111111",
	})
	return &tp_pb.TechProfileInstance{
		Name:                 "mock-tech-profile",
		SubscriberIdentifier: "257",
		ProfileType:          "mock",
		Version:              0,
		NumGemPorts:          1,
		InstanceControl: &tp_pb.InstanceControl{
			Onu:               "multi-instance",
			Uni:               "single-instance",
			MaxGemPayloadSize: "",
		},
		UsScheduler: &tp_pb.SchedulerAttributes{
			AllocId:      1,
			Direction:    tp_pb.Direction_UPSTREAM,
			AdditionalBw: tp_pb.AdditionalBW_AdditionalBW_None,
			Priority:     0,
			Weight:       0,
			QSchedPolicy: tp_pb.SchedulingPolicy_WRR,
		},
		DsScheduler: &tp_pb.SchedulerAttributes{
			AllocId:      1,
			Direction:    tp_pb.Direction_DOWNSTREAM,
			AdditionalBw: tp_pb.AdditionalBW_AdditionalBW_None,
			Priority:     0,
			Weight:       0,
			QSchedPolicy: tp_pb.SchedulingPolicy_WRR,
		},
		UpstreamGemPortAttributeList:   usGemPortAttributeList,
		DownstreamGemPortAttributeList: dsGemPortAttributeList,
	}, nil

}

// CreateTechProfileInstance to mock techprofile CreateTechProfileInstance method
func (m MockTechProfile) CreateTechProfileInstance(ctx context.Context, techProfiletblID uint32, uniPortName string, intfID uint32) (interface{}, error) {

	var usGemPortAttributeList []*tp_pb.GemPortAttributes
	var dsGemPortAttributeList []*tp_pb.GemPortAttributes
	if techProfiletblID == 64 {
		usGemPortAttributeList = append(usGemPortAttributeList, &tp_pb.GemPortAttributes{
			GemportId: 1,
			PbitMap:   "0b11111111",
		})
		dsGemPortAttributeList = append(dsGemPortAttributeList, &tp_pb.GemPortAttributes{
			GemportId: 1,
			PbitMap:   "0b11111111",
		})
		return &tp_pb.TechProfileInstance{
			Name:                 "mock-tech-profile",
			SubscriberIdentifier: "257",
			ProfileType:          "mock",
			Version:              0,
			NumGemPorts:          1,
			InstanceControl: &tp_pb.InstanceControl{
				Onu:               "multi-instance",
				Uni:               "single-instance",
				MaxGemPayloadSize: "",
			},
			UsScheduler: &tp_pb.SchedulerAttributes{
				AllocId:      1,
				Direction:    tp_pb.Direction_UPSTREAM,
				AdditionalBw: tp_pb.AdditionalBW_AdditionalBW_None,
				Priority:     0,
				Weight:       0,
				QSchedPolicy: tp_pb.SchedulingPolicy_WRR,
			},
			DsScheduler: &tp_pb.SchedulerAttributes{
				AllocId:      1,
				Direction:    tp_pb.Direction_DOWNSTREAM,
				AdditionalBw: tp_pb.AdditionalBW_AdditionalBW_None,
				Priority:     0,
				Weight:       0,
				QSchedPolicy: tp_pb.SchedulingPolicy_WRR,
			},
			UpstreamGemPortAttributeList:   usGemPortAttributeList,
			DownstreamGemPortAttributeList: dsGemPortAttributeList,
		}, nil
	} else if techProfiletblID == 65 {
		return &tp_pb.EponTechProfileInstance{
			Name:                         "mock-epon-profile",
			SubscriberIdentifier:         "257",
			ProfileType:                  "mock",
			Version:                      0,
			NumGemPorts:                  2,
			UpstreamQueueAttributeList:   nil,
			DownstreamQueueAttributeList: nil,
		}, nil
	} else {
		return nil, nil
	}
}

// DeleteTechProfileInstance to mock techprofile DeleteTechProfileInstance method
func (m MockTechProfile) DeleteTechProfileInstance(ctx context.Context, techProfiletblID uint32, uniPortName string) error {
	return nil
}

// GetprotoBufParamValue to mock techprofile GetprotoBufParamValue method
func (m MockTechProfile) GetprotoBufParamValue(ctx context.Context, paramType string, paramKey string) int32 {
	return 0

}

// GetUsScheduler to mock techprofile GetUsScheduler method
func (m MockTechProfile) GetUsScheduler(tpInstance *tp_pb.TechProfileInstance) *tp_pb.SchedulerConfig {
	return &tp_pb.SchedulerConfig{}

}

// GetDsScheduler to mock techprofile GetDsScheduler method
func (m MockTechProfile) GetDsScheduler(tpInstance *tp_pb.TechProfileInstance) *tp_pb.SchedulerConfig {
	return &tp_pb.SchedulerConfig{}
}

// GetTrafficScheduler to mock techprofile GetTrafficScheduler method
func (m MockTechProfile) GetTrafficScheduler(tpInstance *tp_pb.TechProfileInstance, SchedCfg *tp_pb.SchedulerConfig,
	ShapingCfg *tp_pb.TrafficShapingInfo) *tp_pb.TrafficScheduler {
	return &tp_pb.TrafficScheduler{}

}

// GetTrafficQueues to mock techprofile GetTrafficQueues method
func (m MockTechProfile) GetTrafficQueues(ctx context.Context, tp *tp_pb.TechProfileInstance, Dir tp_pb.Direction) ([]*tp_pb.TrafficQueue, error) {
	return []*tp_pb.TrafficQueue{{}}, nil
}

// GetMulticastTrafficQueues to mock techprofile GetMulticastTrafficQueues method
func (m MockTechProfile) GetMulticastTrafficQueues(ctx context.Context, tp *tp_pb.TechProfileInstance) []*tp_pb.TrafficQueue {
	return []*tp_pb.TrafficQueue{{}}
}

// GetGemportForPbit to mock techprofile GetGemportForPbit method
func (m MockTechProfile) GetGemportForPbit(ctx context.Context, tpInst interface{}, Dir tp_pb.Direction, pbit uint32) interface{} {
	return &tp_pb.GemPortAttributes{
		GemportId:     1,
		PbitMap:       "0b11111111",
		AesEncryption: "false",
	}
}

// FindAllTpInstances to mock techprofile FindAllTpInstances method
func (m MockTechProfile) FindAllTpInstances(ctx context.Context, oltDeviceID string, tpID uint32, ponIntf uint32, onuID uint32) interface{} {
	return []tp_pb.TechProfileInstance{}
}

// GetResourceID to mock techprofile GetResourceID method
func (m MockTechProfile) GetResourceID(ctx context.Context, IntfID uint32, ResourceType string, NumIDs uint32) ([]uint32, error) {
	return []uint32{}, nil
}

// FreeResourceID to mock techprofile FreeResourceID method
func (m MockTechProfile) FreeResourceID(ctx context.Context, IntfID uint32, ResourceType string, ReleaseContent []uint32) error {
	return nil
}

// GetTechProfileInstanceKey to mock techprofile GetTechProfileInstanceKey method
func (m MockTechProfile) GetTechProfileInstanceKey(ctx context.Context, tpID uint32, uniPortName string) string {
	return ""
}
