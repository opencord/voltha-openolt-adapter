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
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/opencord/voltha-go/db/kvstore"
	ofp "github.com/opencord/voltha-protos/go/openflow_13"
)

const (
	// MeterConfig meter to extarct meter
	MeterConfig = "meter_id"
	// TpIDPathSuffix to extract Techprofile
	TpIDPathSuffix = "tp_id"
)

// MockKVClient mocks the AdapterProxy interface.
type MockKVClient struct {
}

// List mock function implementation for KVClient
func (kvclient *MockKVClient) List(key string, timeout int, lock ...bool) (map[string]*kvstore.KVPair, error) {
	if key != "" {
		maps := make(map[string]*kvstore.KVPair)
		maps[key] = &kvstore.KVPair{Key: key}
		return maps, nil
	}
	return nil, errors.New("key didn't find")
}

// Get mock function implementation for KVClient
func (kvclient *MockKVClient) Get(key string, timeout int, lock ...bool) (*kvstore.KVPair, error) {
	if key != "" {

		if strings.Contains(key, MeterConfig) {
			var bands []*ofp.OfpMeterBandHeader
			bands = append(bands, &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DSCP_REMARK,
				Rate: 1024, Data: &ofp.OfpMeterBandHeader_DscpRemark{DscpRemark: &ofp.OfpMeterBandDscpRemark{PrecLevel: 2}}})

			bands = append(bands, &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DSCP_REMARK,
				Rate: 1024, Data: &ofp.OfpMeterBandHeader_DscpRemark{DscpRemark: &ofp.OfpMeterBandDscpRemark{PrecLevel: 3}}})

			//	bands = append(bands, &ofp.OfpMeterBandHeader{})
			// Data: &ofp.OfpMeterBandHeader_Drop{Drop: &ofp.OfpMeterBandDrop{}}
			sep := strings.Split(key, "/")[2]
			val, _ := strconv.ParseInt(strings.Split(sep, ",")[1], 10, 32)
			if uint32(val) > 1 {
				meterConfig := &ofp.OfpMeterConfig{MeterId: uint32(val), Bands: bands}
				str, _ := json.Marshal(meterConfig)
				//json.marshall()
				return kvstore.NewKVPair(key, string(str), "mock", 3000, 1), nil
			}
			if uint32(val) == 1 {
				return nil, nil
			}
			return nil, errors.New("invalid meter")
		}
		if strings.Contains(key, TpIDPathSuffix) {
			str, _ := json.Marshal(1)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		maps := make(map[string]*kvstore.KVPair)
		maps[key] = &kvstore.KVPair{Key: key}
		return maps[key], nil
	}
	return nil, errors.New("key didn't find")
}

// Put mock function implementation for KVClient
func (kvclient *MockKVClient) Put(key string, value interface{}, timeout int, lock ...bool) error {
	if key != "" {

		return nil
	}
	return errors.New("key didn't find")
}

// Delete mock function implementation for KVClient
func (kvclient *MockKVClient) Delete(key string, timeout int, lock ...bool) error {
	if key == "" {
		return errors.New("key didn't find")
	}
	return nil
}

// Reserve mock function implementation for KVClient
func (kvclient *MockKVClient) Reserve(key string, value interface{}, ttl int64) (interface{}, error) {
	if key != "" {
		maps := make(map[string]*kvstore.KVPair)
		maps[key] = &kvstore.KVPair{Key: key}
		return maps[key], nil
	}
	return nil, errors.New("key didn't find")
}

// ReleaseReservation mock function implementation for KVClient
func (kvclient *MockKVClient) ReleaseReservation(key string) error {
	// return nil
	if key == "" {
		return errors.New("key didn't find")
	}
	return nil
}

// ReleaseAllReservations mock function implementation for KVClient
func (kvclient *MockKVClient) ReleaseAllReservations() error {
	return nil
}

// RenewReservation mock function implementation for KVClient
func (kvclient *MockKVClient) RenewReservation(key string) error {
	// return nil
	if key == "" {
		return errors.New("key didn't find")
	}
	return nil
}

// Watch mock function implementation for KVClient
func (kvclient *MockKVClient) Watch(key string) chan *kvstore.Event {
	return nil
	// if key == "" {
	// 	return nil
	// }
	// return &kvstore.Event{EventType: 1, Key: key}
}

// AcquireLock mock function implementation for KVClient
func (kvclient *MockKVClient) AcquireLock(lockName string, timeout int) error {
	return nil
}

// ReleaseLock mock function implementation for KVClient
func (kvclient *MockKVClient) ReleaseLock(lockName string) error {
	return nil
}

// IsConnectionUp mock function implementation for KVClient
func (kvclient *MockKVClient) IsConnectionUp(timeout int) bool { // timeout in second
	if timeout < 1 {
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
