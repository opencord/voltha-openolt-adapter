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

/*
This file contains unit test cases for functions in the file resourcemanager.go.
This file also implements the Client interface to mock the kv-client, fields struct to mock OpenOltResourceMgr
and few utility functions.
*/

//Package adaptercore provides the utility for olt devices, flows and statistics
package resourcemanager

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/opencord/voltha-lib-go/v3/pkg/db"
	"github.com/opencord/voltha-lib-go/v3/pkg/db/kvstore"
	fu "github.com/opencord/voltha-lib-go/v3/pkg/flows"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	ponrmgr "github.com/opencord/voltha-lib-go/v3/pkg/ponresourcemanager"
	ofp "github.com/opencord/voltha-protos/v3/go/openflow_13"
	"github.com/opencord/voltha-protos/v3/go/openolt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func init() {
	log.SetDefaultLogger(log.JSON, log.DebugLevel, nil)
}

const (
	// MeterConfig meter to extract meter
	MeterConfig = "meter_id"
	// TpIDSuffixPath to extract Techprofile
	TpIDSuffixPath = "tp_id"
	// FlowIDInfo to extract flows
	FlowIDInfo = "flow_id_info"
	// FlowIds to extract flows
	FlowIDs = "flow_ids"
	// GemportIDs to gemport_ids
	GemportIDs = "gemport_ids"
	// AllocIDs to extract alloc_ids
	AllocIDs = "alloc_ids"
	// GemportIDPool to extract gemport
	GemportIDPool = "gemport_id_pool"
	// AllocIDPool to extract allocid
	AllocIDPool = "alloc_id_pool"
	// FlowIDpool to extract Flow ids
	FlowIDpool = "flow_id_pool"
)

// fields mocks  OpenOltResourceMgr struct.
type fields struct {
	DeviceID     string
	HostAndPort  string
	Args         string
	KVStore      *db.Backend
	DeviceType   string
	Host         string
	Port         int
	DevInfo      *openolt.DeviceInfo
	ResourceMgrs map[uint32]*ponrmgr.PONResourceManager
}

// MockKVClient mocks the AdapterProxy interface.
type MockResKVClient struct {
}

// getResMgr mocks OpenOltResourceMgr struct.
func getResMgr() *fields {
	var resMgr fields
	resMgr.KVStore = &db.Backend{
		Client: &MockResKVClient{},
	}
	resMgr.ResourceMgrs = make(map[uint32]*ponrmgr.PONResourceManager)
	ranges := make(map[string]interface{})
	sharedIdxByType := make(map[string]string)
	sharedIdxByType["ALLOC_ID"] = "ALLOC_ID"
	sharedIdxByType["ONU_ID"] = "ONU_ID"
	sharedIdxByType["GEMPORT_ID"] = "GEMPORT_ID"
	sharedIdxByType["FLOW_ID"] = "FLOW_ID"
	ranges["ONU_ID"] = uint32(0)
	ranges["GEMPORT_ID"] = uint32(0)
	ranges["ALLOC_ID"] = uint32(0)
	ranges["FLOW_ID"] = uint32(0)
	ranges["onu_id_shared"] = uint32(0)
	ranges["alloc_id_shared"] = uint32(0)
	ranges["gemport_id_shared"] = uint32(0)
	ranges["flow_id_shared"] = uint32(0)
	ponMgr := &ponrmgr.PONResourceManager{
		DeviceID: "onu-1",
		IntfIDs:  []uint32{1, 2},
		KVStore: &db.Backend{
			Client: &MockResKVClient{},
		},
		PonResourceRanges: ranges,
		SharedIdxByType:   sharedIdxByType,
	}
	resMgr.ResourceMgrs[1] = ponMgr
	resMgr.ResourceMgrs[2] = ponMgr
	return &resMgr
}

// List function implemented for KVClient.
func (kvclient *MockResKVClient) List(ctx context.Context, key string) (map[string]*kvstore.KVPair, error) {
	return nil, errors.New("key didn't find")
}

// Get mock function implementation for KVClient
func (kvclient *MockResKVClient) Get(ctx context.Context, key string) (*kvstore.KVPair, error) {
	log.Debugw("Warning Warning Warning: Get of MockKVClient called", log.Fields{"key": key})
	if key != "" {
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
			return nil, errors.New("invalid meter")
		}
		if strings.Contains(key, FlowIDpool) || strings.Contains(key, GemportIDPool) || strings.Contains(key, AllocIDPool) {
			log.Debug("Error Error Error Key:", FlowIDpool, GemportIDPool, AllocIDPool)
			data := make(map[string]interface{})
			data["pool"] = "1024"
			data["start_idx"] = 1
			data["end_idx"] = 1024
			str, _ := json.Marshal(data)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, FlowIDInfo) || strings.Contains(key, FlowIDs) {
			log.Debug("Error Error Error Key:", FlowIDs, FlowIDInfo)
			str, _ := json.Marshal([]uint32{1, 2})
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, AllocIDs) || strings.Contains(key, GemportIDs) {
			log.Debug("Error Error Error Key:", AllocIDs, GemportIDs)
			str, _ := json.Marshal(1)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, McastQueuesForIntf) {
			log.Debug("Error Error Error Key:", McastQueuesForIntf)
			mcastQueues := make(map[uint32][]uint32)
			mcastQueues[10] = []uint32{4000, 0}
			str, _ := json.Marshal(mcastQueues)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, "flow_groups") && !strings.Contains(key, "1000") {
			groupInfo := GroupInfo{GroupID: 2, OutPorts: []uint32{2}}
			str, _ := json.Marshal(groupInfo)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}

		maps := make(map[string]*kvstore.KVPair)
		maps[key] = &kvstore.KVPair{Key: key}
		return maps[key], nil
	}
	return nil, errors.New("key didn't find")
}

// Put mock function implementation for KVClient
func (kvclient *MockResKVClient) Put(ctx context.Context, key string, value interface{}) error {
	if key != "" {
		return nil
	}
	return errors.New("key didn't find")
}

// Delete mock function implementation for KVClient
func (kvclient *MockResKVClient) Delete(ctx context.Context, key string) error {
	return nil
}

// Reserve mock function implementation for KVClient
func (kvclient *MockResKVClient) Reserve(ctx context.Context, key string, value interface{}, ttl int64) (interface{}, error) {
	return nil, errors.New("key didn't find")
}

// ReleaseReservation mock function implementation for KVClient
func (kvclient *MockResKVClient) ReleaseReservation(ctx context.Context, key string) error {
	return nil
}

// ReleaseAllReservations mock function implementation for KVClient
func (kvclient *MockResKVClient) ReleaseAllReservations(ctx context.Context) error {
	return nil
}

// RenewReservation mock function implementation for KVClient
func (kvclient *MockResKVClient) RenewReservation(ctx context.Context, key string) error {
	return nil
}

// Watch mock function implementation for KVClient
func (kvclient *MockResKVClient) Watch(ctx context.Context, key string) chan *kvstore.Event {
	return nil
}

// AcquireLock mock function implementation for KVClient
func (kvclient *MockResKVClient) AcquireLock(ctx context.Context, lockName string, timeout int) error {
	return nil
}

// ReleaseLock mock function implementation for KVClient
func (kvclient *MockResKVClient) ReleaseLock(lockName string) error {
	return nil
}

// IsConnectionUp mock function implementation for KVClient
func (kvclient *MockResKVClient) IsConnectionUp(ctx context.Context) bool { // timeout in second
	return true
}

// CloseWatch mock function implementation for KVClient
func (kvclient *MockResKVClient) CloseWatch(key string, ch chan *kvstore.Event) {
}

// Close mock function implementation for KVClient
func (kvclient *MockResKVClient) Close() {
}

// testResMgrObject maps fields type to OpenOltResourceMgr type.
func testResMgrObject(testResMgr *fields) *OpenOltResourceMgr {
	return &OpenOltResourceMgr{
		DeviceID:     testResMgr.DeviceID,
		HostAndPort:  testResMgr.HostAndPort,
		Args:         testResMgr.Args,
		KVStore:      testResMgr.KVStore,
		DeviceType:   testResMgr.DeviceType,
		Host:         testResMgr.Host,
		Port:         testResMgr.Port,
		DevInfo:      testResMgr.DevInfo,
		ResourceMgrs: testResMgr.ResourceMgrs,
	}
}

func TestNewResourceMgr(t *testing.T) {
	type args struct {
		deviceID        string
		KVStoreHostPort string
		kvStoreType     string
		deviceType      string
		devInfo         *openolt.DeviceInfo
	}
	tests := []struct {
		name string
		args args
		want *OpenOltResourceMgr
	}{
		{"NewResourceMgr-1", args{"olt1", "1:2", "consul",
			"onu", &openolt.DeviceInfo{OnuIdStart: 1, OnuIdEnd: 1}}, &OpenOltResourceMgr{}},
		{"NewResourceMgr-2", args{"olt2", "3:4", "etcd",
			"onu", &openolt.DeviceInfo{OnuIdStart: 1, OnuIdEnd: 1}}, &OpenOltResourceMgr{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got := NewResourceMgr(ctx, tt.args.deviceID, tt.args.KVStoreHostPort, tt.args.kvStoreType, tt.args.deviceType, tt.args.devInfo); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("NewResourceMgr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_Delete(t *testing.T) {
	tests := []struct {
		name    string
		fields  *fields
		wantErr error
	}{
		{"Delete-1", getResMgr(), errors.New("failed to clear device resource pool")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.Delete(ctx); (err != nil) && reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltResourceMgr_FreeFlowID(t *testing.T) {
	type args struct {
		IntfID uint32
		onuID  int32
		uniID  int32
		FlowID uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
	}{
		{"FreeFlowID-1", getResMgr(), args{2, 2, 2, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			RsrcMgr.FreeFlowID(ctx, tt.args.IntfID, tt.args.onuID, tt.args.uniID, tt.args.FlowID)
		})
	}
}

func TestOpenOltResourceMgr_FreeFlowIDs(t *testing.T) {

	type args struct {
		IntfID uint32
		onuID  uint32
		uniID  uint32
		FlowID []uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
	}{
		{"FreeFlowIDs-1", getResMgr(), args{2, 2, 2, []uint32{1, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			RsrcMgr.FreeFlowIDs(ctx, tt.args.IntfID, tt.args.onuID, tt.args.uniID, tt.args.FlowID)
		})
	}
}

func TestOpenOltResourceMgr_FreePONResourcesForONU(t *testing.T) {
	type args struct {
		intfID uint32
		onuID  uint32
		uniID  uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
	}{
		{"FreePONResourcesForONU-1", getResMgr(), args{2, 0, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			RsrcMgr.FreePONResourcesForONU(ctx, tt.args.intfID, tt.args.onuID, tt.args.uniID)
		})
	}
}

func TestOpenOltResourceMgr_FreeonuID(t *testing.T) {
	type args struct {
		intfID uint32
		onuID  []uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
	}{
		{"FreeOnuID-1", getResMgr(), args{2, []uint32{1, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			RsrcMgr.FreeonuID(ctx, tt.args.intfID, tt.args.onuID)
		})
	}
}

func TestOpenOltResourceMgr_GetAllocID(t *testing.T) {

	type args struct {
		intfID uint32
		onuID  uint32
		uniID  uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
		want   uint32
	}{
		{"GetAllocID-1", getResMgr(), args{2, 2, 2}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got := RsrcMgr.GetAllocID(ctx, tt.args.intfID, tt.args.onuID, tt.args.uniID); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("GetAllocID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_GetCurrentAllocIDForOnu(t *testing.T) {
	type args struct {
		intfID uint32
		onuID  uint32
		uniID  uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
		want   []uint32
	}{
		{"GetCurrentAllocIDForOnu-1", getResMgr(), args{2, 2, 2}, []uint32{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got := RsrcMgr.GetCurrentAllocIDsForOnu(ctx, tt.args.intfID, tt.args.onuID, tt.args.uniID); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCurrentAllocIDsForOnu() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_GetCurrentFlowIDsForOnu(t *testing.T) {

	type args struct {
		PONIntfID uint32
		ONUID     int32
		UNIID     int32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
		want   []uint32
	}{
		{"GetCurrentFlowIDsForOnu-1", getResMgr(), args{2, 2, 2}, []uint32{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got := RsrcMgr.GetCurrentFlowIDsForOnu(ctx, tt.args.PONIntfID, tt.args.ONUID, tt.args.UNIID); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("GetCurrentFlowIDsForOnu() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_GetCurrentGEMPortIDsForOnu(t *testing.T) {
	type args struct {
		intfID uint32
		onuID  uint32
		uniID  uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
		want   []uint32
	}{
		{"GetCurrentGEMPortIDsForOnu-1", getResMgr(), args{2, 2, 2}, []uint32{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got := RsrcMgr.GetCurrentGEMPortIDsForOnu(ctx, tt.args.intfID, tt.args.onuID, tt.args.uniID); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("GetCurrentGEMPortIDsForOnu() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_GetFlowID(t *testing.T) {

	type args struct {
		ponIntfID       uint32
		ONUID           int32
		uniID           int32
		gemportID       uint32
		flowStoreCookie uint64
		flowCategory    string
		vlanPcp         []uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    uint32
		wantErr error
	}{
		{"GetFlowID-1", getResMgr(), args{2, 2, 2, 2, 2,
			"HSIA", nil}, 0, errors.New("failed to get flows")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := RsrcMgr.GetFlowID(ctx, tt.args.ponIntfID, tt.args.ONUID, tt.args.uniID, tt.args.gemportID, tt.args.flowStoreCookie, tt.args.flowCategory, tt.args.vlanPcp...)
			if err != nil && reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) {
				t.Errorf("GetFlowID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("GetFlowID() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_GetGEMPortID(t *testing.T) {
	type args struct {
		ponPort    uint32
		onuID      uint32
		uniID      uint32
		NumOfPorts uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    []uint32
		wantErr error
	}{
		{"GetGEMPortID-1", getResMgr(), args{2, 2, 2, 2}, []uint32{},
			errors.New("failed to get gem port")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := RsrcMgr.GetGEMPortID(ctx, tt.args.ponPort, tt.args.onuID, tt.args.uniID, tt.args.NumOfPorts)
			if reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) && err != nil {
				t.Errorf("GetGEMPortID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("GetGEMPortID() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_GetMeterIDForOnu(t *testing.T) {
	type args struct {
		Direction string
		IntfID    uint32
		OnuID     uint32
		UniID     uint32
		tpID      uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    *ofp.OfpMeterConfig
		wantErr error
	}{
		{"GetMeterIDOnu", getResMgr(), args{"DOWNSTREAM", 1, 1, 1, 64},
			&ofp.OfpMeterConfig{}, errors.New("failed to get Meter config from kvstore for path")},
		{"GetMeterIDOnu", getResMgr(), args{"DOWNSTREAM", 2, 2, 2, 65},
			&ofp.OfpMeterConfig{}, errors.New("failed to get Meter config from kvstore for path")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := RsrcMgr.GetMeterIDForOnu(ctx, tt.args.Direction, tt.args.IntfID, tt.args.OnuID, tt.args.UniID, tt.args.tpID)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) && err != nil {
				t.Errorf("GetMeterIDForOnu() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_GetONUID(t *testing.T) {
	type args struct {
		ponIntfID uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    uint32
		wantErr error
	}{
		{"GetONUID-1", getResMgr(), args{2}, uint32(0), errors.New("json errors")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := RsrcMgr.GetONUID(ctx, tt.args.ponIntfID)
			if got != tt.want && err != nil {
				t.Errorf("GetONUID() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_GetTechProfileIDForOnu(t *testing.T) {

	type args struct {
		IntfID uint32
		OnuID  uint32
		UniID  uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
		want   []uint32
	}{
		{"GetTechProfileIDForOnu-1", getResMgr(), args{2, 2, 2},
			[]uint32{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got := RsrcMgr.GetTechProfileIDForOnu(ctx, tt.args.IntfID, tt.args.OnuID, tt.args.UniID); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("GetTechProfileIDForOnu() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_IsFlowCookieOnKVStore(t *testing.T) {
	type args struct {
		ponIntfID       uint32
		onuID           int32
		uniID           int32
		flowStoreCookie uint64
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
		want   bool
	}{
		{"IsFlowCookieOnKVStore-1", getResMgr(), args{2, 2, 2, 2}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got := RsrcMgr.IsFlowCookieOnKVStore(ctx, tt.args.ponIntfID, tt.args.onuID, tt.args.uniID, tt.args.flowStoreCookie); got != tt.want {
				t.Errorf("IsFlowCookieOnKVStore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_RemoveMeterIDForOnu(t *testing.T) {

	type args struct {
		Direction string
		IntfID    uint32
		OnuID     uint32
		UniID     uint32
		tpID      uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"RemoveMeterIdForOnu-1", getResMgr(), args{"DOWNSTREAM", 1, 1, 1, 64},
			errors.New("failed to delete meter id %s from kvstore")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.RemoveMeterIDForOnu(ctx, tt.args.Direction, tt.args.IntfID, tt.args.OnuID, tt.args.UniID,
				tt.args.tpID); reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) && err != nil {
				t.Errorf("RemoveMeterIDForOnu() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltResourceMgr_RemoveTechProfileIDForOnu(t *testing.T) {
	type args struct {
		IntfID uint32
		OnuID  uint32
		UniID  uint32
		tpID   uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"RemoveTechProfileIDForOnu-1", getResMgr(), args{2, 2, 2, 64},
			errors.New("failed to delete techprofile id resource %s in KV store")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.RemoveTechProfileIDForOnu(ctx, tt.args.IntfID, tt.args.OnuID, tt.args.UniID,
				tt.args.tpID); reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) && err != nil {
				t.Errorf("RemoveTechProfileIDForOnu() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltResourceMgr_UpdateAllocIdsForOnu(t *testing.T) {
	type args struct {
		ponPort uint32
		onuID   uint32
		uniID   uint32
		allocID []uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"UpdateAllocIdsForOnu-1", getResMgr(), args{2, 2, 2, []uint32{1, 2}},
			errors.New("")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.UpdateAllocIdsForOnu(ctx, tt.args.ponPort, tt.args.onuID, tt.args.uniID, tt.args.allocID); err != nil && reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) {
				t.Errorf("UpdateAllocIdsForOnu() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltResourceMgr_UpdateFlowIDInfo(t *testing.T) {
	type args struct {
		ponIntfID int32
		onuID     int32
		uniID     int32
		flowID    uint32
		flowData  *[]FlowInfo
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"UpdateFlowIDInfo-1", getResMgr(), args{2, 2, 2, 2, &[]FlowInfo{}}, errors.New("")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.UpdateFlowIDInfo(ctx, tt.args.ponIntfID, tt.args.onuID, tt.args.uniID, tt.args.flowID, tt.args.flowData); err != nil && reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) {
				t.Errorf("UpdateFlowIDInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltResourceMgr_UpdateGEMPortIDsForOnu(t *testing.T) {

	type args struct {
		ponPort     uint32
		onuID       uint32
		uniID       uint32
		GEMPortList []uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"UpdateGEMPortIDsForOnu-1", getResMgr(), args{2, 2, 2,
			[]uint32{1, 2}}, errors.New("failed to update resource")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.UpdateGEMPortIDsForOnu(ctx, tt.args.ponPort, tt.args.onuID, tt.args.uniID, tt.args.GEMPortList); err != nil && reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) {
				t.Errorf("UpdateGEMPortIDsForOnu() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltResourceMgr_UpdateGEMportsPonportToOnuMapOnKVStore(t *testing.T) {
	type args struct {
		gemPorts []uint32
		PonPort  uint32
		onuID    uint32
		uniID    uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"UpdateGEMportsPonportToOnuMapOnKVStore-1", getResMgr(), args{[]uint32{1, 2},
			2, 2, 2}, errors.New("failed to update resource")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.UpdateGEMportsPonportToOnuMapOnKVStore(ctx, tt.args.gemPorts, tt.args.PonPort,
				tt.args.onuID, tt.args.uniID); err != nil && reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) {
				t.Errorf("UpdateGEMportsPonportToOnuMapOnKVStore() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltResourceMgr_UpdateMeterIDForOnu(t *testing.T) {
	type args struct {
		Direction   string
		IntfID      uint32
		OnuID       uint32
		UniID       uint32
		tpID        uint32
		MeterConfig *ofp.OfpMeterConfig
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"UpdateMeterIDForOnu-1", getResMgr(), args{"DOWNSTREAM", 2, 2,
			2, 64, &ofp.OfpMeterConfig{}}, errors.New("failed to get Meter config from kvstore for path")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.UpdateMeterIDForOnu(ctx, tt.args.Direction, tt.args.IntfID, tt.args.OnuID, tt.args.UniID,
				tt.args.tpID, tt.args.MeterConfig); reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) && err != nil {
				t.Errorf("UpdateMeterIDForOnu() got = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltResourceMgr_UpdateTechProfileIDForOnu(t *testing.T) {
	type args struct {
		IntfID uint32
		OnuID  uint32
		UniID  uint32
		TpID   uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"UpdateTechProfileIDForOnu-1", getResMgr(), args{2, 2, 2,
			2}, errors.New("failed to update resource")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.UpdateTechProfileIDForOnu(ctx, tt.args.IntfID, tt.args.OnuID, tt.args.UniID, tt.args.TpID); reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) && err != nil {
				t.Errorf("UpdateTechProfileIDForOnu() got = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetKVClient(t *testing.T) {
	type args struct {
		backend  string
		Host     string
		Port     int
		DeviceID string
	}
	tests := []struct {
		name string
		args args
		want *db.Backend
	}{
		{"setKVClient-1", args{"consul", "1.1.1.1", 1, "olt1"}, &db.Backend{}},
		{"setKVClient-1", args{"etcd", "2.2.2.2", 2, "olt2"}, &db.Backend{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SetKVClient(tt.args.backend, tt.args.Host, tt.args.Port, tt.args.DeviceID); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("SetKVClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getFlowIDFromFlowInfo(t *testing.T) {
	type args struct {
		FlowInfo        *[]FlowInfo
		flowID          uint32
		gemportID       uint32
		flowStoreCookie uint64
		flowCategory    string
		vlanPcp         []uint32
	}
	flowInfo := &[]FlowInfo{
		{
			&openolt.Flow{
				FlowId:    1,
				GemportId: 1,
				Classifier: &openolt.Classifier{
					OPbits: 1,
				}},
			1,
			"HSIA_FLOW",
			2000,
		},
		{
			&openolt.Flow{
				GemportId: 1,
			},
			1,
			"EAPOL",
			3000,
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{"getFlowIdFromFlowInfo-1", args{}, errors.New("invalid flow-info")},
		{"getFlowIdFromFlowInfo-2", args{flowInfo, 1, 1, 1,
			"HSIA_FLOW", []uint32{1, 2}}, errors.New("invalid flow-info")},
		{"getFlowIdFromFlowInfo-2", args{flowInfo, 1, 1, 1,
			"EAPOL", []uint32{1, 2}}, errors.New("invalid flow-info")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := getFlowIDFromFlowInfo(tt.args.FlowInfo, tt.args.flowID, tt.args.gemportID, tt.args.flowStoreCookie, tt.args.flowCategory, tt.args.vlanPcp...)
			if reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) && err != nil {
				t.Errorf("getFlowIDFromFlowInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				t.Log("return'd nil")
			}
		})
	}
}

func Test_newKVClient(t *testing.T) {
	type args struct {
		storeType string
		address   string
		timeout   uint32
	}
	var kvClient kvstore.Client
	tests := []struct {
		name    string
		args    args
		want    kvstore.Client
		wantErr error
	}{
		{"newKVClient-1", args{"", "3.3.3.3", 1}, kvClient, errors.New("unsupported-kv-store")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newKVClient(tt.args.storeType, tt.args.address, tt.args.timeout)
			if got != nil && reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("newKVClient() got = %v, want %v", got, tt.want)
			}
			if (err != nil) && reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) {
				t.Errorf("newKVClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

		})
	}
}

func TestOpenOltResourceMgr_AddMcastQueueForIntf(t *testing.T) {
	type args struct {
		intf            uint32
		gem             uint32
		servicePriority uint32
	}
	tests := []struct {
		name   string
		args   args
		fields *fields
	}{
		{"AddMcastQueueForIntf-1", args{0, 4000, 0}, getResMgr()},
		{"AddMcastQueueForIntf-2", args{1, 4000, 1}, getResMgr()},
		{"AddMcastQueueForIntf-3", args{2, 4000, 2}, getResMgr()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := RsrcMgr.AddMcastQueueForIntf(ctx, tt.args.intf, tt.args.gem, tt.args.servicePriority)
			if err != nil {
				t.Errorf("%s got err= %s wants nil", tt.name, err)
				return
			}
		})
	}
}

func newGroup(groupID uint32, outPorts []uint32) *ofp.OfpGroupEntry {
	groupDesc := ofp.OfpGroupDesc{
		Type:    ofp.OfpGroupType_OFPGT_ALL,
		GroupId: groupID,
	}
	groupEntry := ofp.OfpGroupEntry{
		Desc: &groupDesc,
	}
	for i := 0; i < len(outPorts); i++ {
		var acts []*ofp.OfpAction
		acts = append(acts, fu.Output(outPorts[i]))
		bucket := ofp.OfpBucket{
			Actions: acts,
		}
		groupDesc.Buckets = append(groupDesc.Buckets, &bucket)
	}
	return &groupEntry
}

func TestOpenOltResourceMgr_AddFlowGroupToKVStore(t *testing.T) {
	type args struct {
		group  *ofp.OfpGroupEntry
		cached bool
	}
	//create group 1
	group1 := newGroup(1, []uint32{1})
	//create group 2
	group2 := newGroup(2, []uint32{2})
	//define test set
	tests := []struct {
		name   string
		args   args
		fields *fields
	}{
		{"AddFlowGroupToKVStore-1", args{group1, true}, getResMgr()},
		{"AddFlowGroupToKVStore-2", args{group2, false}, getResMgr()},
	}
	//execute tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := RsrcMgr.AddFlowGroupToKVStore(ctx, tt.args.group, tt.args.cached)
			if err != nil {
				t.Errorf("%s got err= %s wants nil", tt.name, err)
				return
			}
		})
	}
}

func TestOpenOltResourceMgr_RemoveFlowGroupFromKVStore(t *testing.T) {
	type args struct {
		groupID uint32
		cached  bool
	}
	//define test set
	tests := []struct {
		name   string
		args   args
		fields *fields
	}{
		{"RemoveFlowGroupFromKVStore-1", args{1, true}, getResMgr()},
		{"RemoveFlowGroupFromKVStore-2", args{2, false}, getResMgr()},
	}
	//execute tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			success := RsrcMgr.RemoveFlowGroupFromKVStore(ctx, tt.args.groupID, tt.args.cached)
			if !success {
				t.Errorf("%s got false but wants true", tt.name)
				return
			}
		})
	}
}

func TestOpenOltResourceMgr_GetFlowGroupFromKVStore(t *testing.T) {
	type args struct {
		groupID uint32
		cached  bool
	}
	//define test set
	tests := []struct {
		name   string
		args   args
		fields *fields
	}{
		{"GetFlowGroupFromKVStore-1", args{1, true}, getResMgr()},
		{"GetFlowGroupFromKVStore-2", args{2, false}, getResMgr()},
		{"GetFlowGroupFromKVStore-3", args{1000, false}, getResMgr()},
	}
	//execute tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			exists, groupInfo, err := RsrcMgr.GetFlowGroupFromKVStore(ctx, tt.args.groupID, tt.args.cached)
			if err != nil {
				t.Errorf("%s got error but wants nil error", tt.name)
				return
			} else if exists && (groupInfo.GroupID == 0) {
				t.Errorf("%s got true and nil group info but expected not nil group info", tt.name)
				return
			} else if tt.args.groupID == 3 && exists {
				t.Errorf("%s got true but wants false", tt.name)
				return
			}
		})
	}
}
