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
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"

	"github.com/opencord/voltha-lib-go/v3/pkg/db/kvstore"
	ofp "github.com/opencord/voltha-protos/v3/go/openflow_13"
	openolt "github.com/opencord/voltha-protos/v3/go/openolt"
)

const (
	// MeterConfig meter to extarct meter
	MeterConfig = "meter_id"
	// TpIDPathSuffix to extract Techprofile
	TpIDPathSuffix = "tp_id"
	// FlowIDpool to extract Flow ids
	FlowIDpool = "flow_id_pool"
	// FlowIDs to extract flow_ids
	FlowIDs = "flow_ids"
	// FlowIDInfo  to extract flowId info
	FlowIDInfo = "flow_id_info"
	// GemportIDs to gemport_ids
	GemportIDs = "gemport_ids"
	// AllocIDs to extract alloc_ids
	AllocIDs = "alloc_ids"
	//FlowGroup flow_groups/<flow_group_id>
	FlowGroup = "flow_groups"
	//FlowGroupCached flow_groups_cached/<flow_group_id>
	FlowGroupCached = "flow_groups_cached"
)

// MockKVClient mocks the AdapterProxy interface.
type MockKVClient struct {
}

// List mock function implementation for KVClient
func (kvclient *MockKVClient) List(ctx context.Context, key string) (map[string]*kvstore.KVPair, error) {
	if key != "" {
		maps := make(map[string]*kvstore.KVPair)
		maps[key] = &kvstore.KVPair{Key: key}
		return maps, nil
	}
	return nil, errors.New("key didn't find")
}

// Get mock function implementation for KVClient
func (kvclient *MockKVClient) Get(ctx context.Context, key string) (*kvstore.KVPair, error) {
	log.Debugw("Warning Warning Warning: Get of MockKVClient called", log.Fields{"key": key})
	if key != "" {
		log.Debug("Warning Key Not Blank")
		if strings.Contains(key, "meter_id/{0,62,8}/{upstream}") {
			meterConfig := ofp.OfpMeterConfig{
				Flags:   0,
				MeterId: 1,
			}
			str, _ := json.Marshal(meterConfig)
			return kvstore.NewKVPair(key, string(str), "mock", 3000, 1), nil
		}
		if strings.Contains(key, MeterConfig) {
			var bands []*ofp.OfpMeterBandHeader
			bands = append(bands, &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DSCP_REMARK,
				Rate: 1024, Data: &ofp.OfpMeterBandHeader_DscpRemark{DscpRemark: &ofp.OfpMeterBandDscpRemark{PrecLevel: 2}}})

			bands = append(bands, &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DSCP_REMARK,
				Rate: 1024, Data: &ofp.OfpMeterBandHeader_DscpRemark{DscpRemark: &ofp.OfpMeterBandDscpRemark{PrecLevel: 3}}})

			sep := strings.Split(key, "/")[1]
			val, _ := strconv.ParseInt(strings.Split(sep, ",")[1], 10, 32)
			if uint32(val) > 1 {
				meterConfig := &ofp.OfpMeterConfig{MeterId: uint32(val), Bands: bands}
				str, _ := json.Marshal(meterConfig)

				return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
			}

			if strings.Contains(key, "meter_id/{1,1,1}/{downstream}") {

				band1 := &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DROP, Rate: 1000, BurstSize: 5000}
				band2 := &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DROP, Rate: 2000, BurstSize: 5000}
				bands := []*ofp.OfpMeterBandHeader{band1, band2}
				ofpMeterConfig := &ofp.OfpMeterConfig{Flags: 1, MeterId: 1, Bands: bands}
				str, _ := json.Marshal(ofpMeterConfig)
				return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
			}
			if uint32(val) == 1 {
				return nil, nil
			}
			return nil, errors.New("invalid meter")
		}
		if strings.Contains(key, TpIDPathSuffix) {
			str, _ := json.Marshal(64)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, FlowIDpool) {
			log.Debug("Error Error Error Key:", FlowIDpool)
			data := make(map[string]interface{})
			data["pool"] = "1024"
			data["start_idx"] = 1
			data["end_idx"] = 1024
			str, _ := json.Marshal(data)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, FlowIDs) {
			data := []uint32{1, 2}
			log.Debug("Error Error Error Key:", FlowIDs)
			str, _ := json.Marshal(data)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, FlowIDInfo) {

			data := []resourcemanager.FlowInfo{
				{
					Flow:            &openolt.Flow{FlowId: 1, OnuId: 1, UniId: 1, GemportId: 1},
					FlowStoreCookie: uint64(48132224281636694),
					LogicalFlowID:   1,
				},
			}
			log.Debug("Error Error Error Key:", FlowIDs)
			str, _ := json.Marshal(data)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, GemportIDs) {
			log.Debug("Error Error Error Key:", GemportIDs)
			str, _ := json.Marshal(1)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, AllocIDs) {
			log.Debug("Error Error Error Key:", AllocIDs)
			str, _ := json.Marshal(1)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, FlowGroup) || strings.Contains(key, FlowGroupCached) {
			log.Debug("Error Error Error Key:", FlowGroup)
			groupInfo := resourcemanager.GroupInfo{
				GroupID:  2,
				OutPorts: []uint32{1},
			}
			str, _ := json.Marshal(&groupInfo)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}

		maps := make(map[string]*kvstore.KVPair)
		maps[key] = &kvstore.KVPair{Key: key}
		return maps[key], nil
	}
	return nil, errors.New("key didn't find")
}

// Put mock function implementation for KVClient
func (kvclient *MockKVClient) Put(ctx context.Context, key string, value interface{}) error {
	if key != "" {

		return nil
	}
	return errors.New("key didn't find")
}

// Delete mock function implementation for KVClient
func (kvclient *MockKVClient) Delete(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("key didn't find")
	}
	return nil
}

// Reserve mock function implementation for KVClient
func (kvclient *MockKVClient) Reserve(ctx context.Context, key string, value interface{}, ttl int64) (interface{}, error) {
	if key != "" {
		maps := make(map[string]*kvstore.KVPair)
		maps[key] = &kvstore.KVPair{Key: key}
		return maps[key], nil
	}
	return nil, errors.New("key didn't find")
}

// ReleaseReservation mock function implementation for KVClient
func (kvclient *MockKVClient) ReleaseReservation(ctx context.Context, key string) error {
	// return nil
	if key == "" {
		return errors.New("key didn't find")
	}
	return nil
}

// ReleaseAllReservations mock function implementation for KVClient
func (kvclient *MockKVClient) ReleaseAllReservations(ctx context.Context) error {
	return nil
}

// RenewReservation mock function implementation for KVClient
func (kvclient *MockKVClient) RenewReservation(ctx context.Context, key string) error {
	// return nil
	if key == "" {
		return errors.New("key didn't find")
	}
	return nil
}

// Watch mock function implementation for KVClient
func (kvclient *MockKVClient) Watch(ctx context.Context, key string) chan *kvstore.Event {
	return nil
	// if key == "" {
	// 	return nil
	// }
	// return &kvstore.Event{EventType: 1, Key: key}
}

// AcquireLock mock function implementation for KVClient
func (kvclient *MockKVClient) AcquireLock(ctx context.Context, lockName string, timeout int) error {
	return nil
}

// ReleaseLock mock function implementation for KVClient
func (kvclient *MockKVClient) ReleaseLock(lockName string) error {
	return nil
}

// IsConnectionUp mock function implementation for KVClient
func (kvclient *MockKVClient) IsConnectionUp(ctx context.Context) bool {
	// timeout in second
	t, _ := ctx.Deadline()
	if t.Second()-time.Now().Second() < 1 {
		return false
	}
	return true
}

// CloseWatch mock function implementation for KVClient
func (kvclient *MockKVClient) CloseWatch(key string, ch chan *kvstore.Event) {
}

// Close mock function implementation for KVClient
func (kvclient *MockKVClient) Close() {
}
