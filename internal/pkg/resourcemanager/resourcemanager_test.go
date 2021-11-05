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
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/opencord/voltha-lib-go/v7/pkg/techprofile"
	"github.com/opencord/voltha-openolt-adapter/pkg/mocks"

	"github.com/opencord/voltha-lib-go/v7/pkg/db"
	"github.com/opencord/voltha-lib-go/v7/pkg/db/kvstore"
	fu "github.com/opencord/voltha-lib-go/v7/pkg/flows"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	ponrmgr "github.com/opencord/voltha-lib-go/v7/pkg/ponresourcemanager"
	ofp "github.com/opencord/voltha-protos/v5/go/openflow_13"
	"github.com/opencord/voltha-protos/v5/go/openolt"
)

func init() {
	_, _ = log.SetDefaultLogger(log.JSON, log.DebugLevel, nil)
}

const (
	// MeterConfig meter to extract meter
	MeterConfig = "meter_id"
	// TpIDSuffixPath to extract Techprofile
	// TpIDSuffixPath = "tp_id"
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
	DeviceID       string
	Address        string
	Args           string
	KVStore        *db.Backend
	DeviceType     string
	DevInfo        *openolt.DeviceInfo
	PonRsrMgr      *ponrmgr.PONResourceManager
	NumOfPonPorts  uint32
	TechProfileRef techprofile.TechProfileIf
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
	resMgr.PonRsrMgr = &ponrmgr.PONResourceManager{}
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
	resMgr.NumOfPonPorts = 16
	resMgr.PonRsrMgr.DeviceID = "onu-1"
	resMgr.PonRsrMgr.IntfIDs = []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	resMgr.PonRsrMgr.KVStore = &db.Backend{
		Client: &MockResKVClient{},
	}
	resMgr.PonRsrMgr.Technology = "XGS-PON"
	resMgr.PonRsrMgr.PonResourceRanges = ranges
	resMgr.PonRsrMgr.SharedIdxByType = sharedIdxByType
	resMgr.TechProfileRef = mocks.MockTechProfile{}

	/*
		tpMgr, err := tp.NewTechProfile(ctx, resMgr.PonRsrMgr, "etcd", "127.0.0.1", "/")
		if err != nil {
			logger.Fatal(ctx, err.Error())
		}
	*/

	return &resMgr
}

// List function implemented for KVClient.
func (kvclient *MockResKVClient) List(ctx context.Context, key string) (map[string]*kvstore.KVPair, error) {
	return nil, errors.New("key didn't find")
}

// Get mock function implementation for KVClient
func (kvclient *MockResKVClient) Get(ctx context.Context, key string) (*kvstore.KVPair, error) {
	logger.Debugw(ctx, "Warning Warning Warning: Get of MockKVClient called", log.Fields{"key": key})
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
			logger.Debug(ctx, "Error Error Error Key:", FlowIDpool, GemportIDPool, AllocIDPool)
			data := make(map[string]interface{})
			data["pool"] = "1024"
			data["start_idx"] = 1
			data["end_idx"] = 1024
			str, _ := json.Marshal(data)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, FlowIDInfo) || strings.Contains(key, FlowIDs) {
			logger.Debug(ctx, "Error Error Error Key:", FlowIDs, FlowIDInfo)
			str, _ := json.Marshal([]uint32{1, 2})
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, AllocIDs) || strings.Contains(key, GemportIDs) {
			logger.Debug(ctx, "Error Error Error Key:", AllocIDs, GemportIDs)
			str, _ := json.Marshal(1)
			return kvstore.NewKVPair(key, str, "mock", 3000, 1), nil
		}
		if strings.Contains(key, McastQueuesForIntf) {
			logger.Debug(ctx, "Error Error Error Key:", McastQueuesForIntf)
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

// DeleteWithPrefix mock function implementation for KVClient
func (kvclient *MockResKVClient) DeleteWithPrefix(ctx context.Context, prefix string) error {
	return nil
}

// Reserve mock function implementation for KVClient
func (kvclient *MockResKVClient) Reserve(ctx context.Context, key string, value interface{}, ttl time.Duration) (interface{}, error) {
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
func (kvclient *MockResKVClient) Watch(ctx context.Context, key string, withPrefix bool) chan *kvstore.Event {
	return nil
}

// AcquireLock mock function implementation for KVClient
func (kvclient *MockResKVClient) AcquireLock(ctx context.Context, lockName string, timeout time.Duration) error {
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
func (kvclient *MockResKVClient) CloseWatch(ctx context.Context, key string, ch chan *kvstore.Event) {
}

// Close mock function implementation for KVClient
func (kvclient *MockResKVClient) Close(ctx context.Context) {
}

// testResMgrObject maps fields type to OpenOltResourceMgr type.
func testResMgrObject(testResMgr *fields) *OpenOltResourceMgr {
	var rsrMgr = OpenOltResourceMgr{
		DeviceID:       testResMgr.DeviceID,
		Args:           testResMgr.Args,
		KVStore:        testResMgr.KVStore,
		DeviceType:     testResMgr.DeviceType,
		Address:        testResMgr.Address,
		DevInfo:        testResMgr.DevInfo,
		PonRsrMgr:      testResMgr.PonRsrMgr,
		TechprofileRef: testResMgr.TechProfileRef,
	}
	rsrMgr.InitLocalCache()

	return &rsrMgr
}

func TestNewResourceMgr(t *testing.T) {
	type args struct {
		deviceID       string
		intfID         uint32
		KVStoreAddress string
		kvStoreType    string
		deviceType     string
		devInfo        *openolt.DeviceInfo
		kvStorePrefix  string
	}
	tests := []struct {
		name string
		args args
		want *OpenOltResourceMgr
	}{
		{"NewResourceMgr-2", args{"olt1", 0, "1:2", "etcd",
			"onu", &openolt.DeviceInfo{OnuIdStart: 1, OnuIdEnd: 1}, "service/voltha"}, &OpenOltResourceMgr{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got := NewResourceMgr(ctx, tt.args.intfID, tt.args.deviceID, tt.args.KVStoreAddress, tt.args.kvStoreType, tt.args.deviceType, tt.args.devInfo, tt.args.kvStorePrefix); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("NewResourceMgr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_Delete(t *testing.T) {
	type args struct {
		intfID uint32
	}
	tests := []struct {
		name    string
		fields  *fields
		wantErr error
		args    args
	}{
		{"Delete-1", getResMgr(), errors.New("failed to clear device resource pool"), args{intfID: 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.Delete(ctx, tt.args.intfID); (err != nil) && reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
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
		{"FreePONResourcesForONU-1", getResMgr(), args{1, 0, 2}},
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
		{"FreeOnuID-1", getResMgr(), args{1, []uint32{1, 2}}},
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
		{"GetCurrentAllocIDForOnu-1", getResMgr(), args{1, 2, 2}, []uint32{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got := RsrcMgr.GetCurrentAllocIDsForOnu(ctx, tt.args.intfID, tt.args.onuID, tt.args.uniID)
			if len(got) != len(tt.want) {
				t.Errorf("GetCurrentAllocIDsForOnu() = %v, want %v", got, tt.want)
			} else {
				for i := range tt.want {
					if got[i] != tt.want[i] {
						t.Errorf("GetCurrentAllocIDsForOnu() = %v, want %v", got, tt.want)
						break
					}
				}
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
		want   []uint64
	}{
		{"GetCurrentFlowIDsForOnu-1", getResMgr(), args{1, 2, 2}, []uint64{1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := RsrcMgr.GetCurrentFlowIDsForOnu(ctx, tt.args.PONIntfID, tt.args.ONUID, tt.args.UNIID)
			if err != nil {
				t.Errorf("GetCurrentFlowIDsForOnu() returned error")
			}
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("GetCurrentFlowIDsForOnu() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltResourceMgr_DeleteAllFlowIDsForGemForIntf(t *testing.T) {

	type args struct {
		PONIntfID uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
		want   error
	}{
		{"DeleteAllFlowIDsForGemForIntf-1", getResMgr(), args{0}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := RsrcMgr.DeleteAllFlowIDsForGemForIntf(ctx, tt.args.PONIntfID)
			if err != nil {
				t.Errorf("DeleteAllFlowIDsForGemForIntf() returned error")
			}
		})
	}
}

func TestOpenOltResourceMgr_DeleteAllOnuGemInfoForIntf(t *testing.T) {

	type args struct {
		PONIntfID uint32
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
		want   error
	}{
		{"DeleteAllOnuGemInfoForIntf-1", getResMgr(), args{0}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := RsrcMgr.DeleteAllOnuGemInfoForIntf(ctx, tt.args.PONIntfID)
			if err != nil {
				t.Errorf("DeleteAllOnuGemInfoForIntf() returned error")
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
		{"GetCurrentGEMPortIDsForOnu-1", getResMgr(), args{1, 2, 2}, []uint32{}},
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

func TestOpenOltResourceMgr_GetMeterInfoForOnu(t *testing.T) {
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
		want    *MeterInfo
		wantErr error
	}{
		{"GetMeterInfoForOnu", getResMgr(), args{"DOWNSTREAM", 0, 1, 1, 64},
			&MeterInfo{}, errors.New("failed to get Meter config from kvstore for path")},
		{"GetMeterInfoForOnu", getResMgr(), args{"DOWNSTREAM", 1, 2, 2, 65},
			&MeterInfo{}, errors.New("failed to get Meter config from kvstore for path")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := RsrcMgr.GetMeterInfoForOnu(ctx, tt.args.Direction, tt.args.IntfID, tt.args.OnuID, tt.args.UniID, tt.args.tpID)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) && err != nil {
				t.Errorf("GetMeterInfoForOnu() got = %v, want %v", got, tt.want)
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
		{"GetONUID-1", getResMgr(), args{1}, uint32(0), errors.New("json errors")},
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
		{"GetTechProfileIDForOnu-1", getResMgr(), args{1, 2, 2},
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
			if err := RsrcMgr.RemoveMeterInfoForOnu(ctx, tt.args.Direction, tt.args.IntfID, tt.args.OnuID, tt.args.UniID,
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
		{"RemoveTechProfileIDForOnu-1", getResMgr(), args{1, 2, 2, 64},
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
		{"UpdateAllocIdsForOnu-1", getResMgr(), args{1, 2, 2, []uint32{1, 2}},
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
		{"UpdateGEMPortIDsForOnu-1", getResMgr(), args{1, 2, 2,
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

func TestOpenOltResourceMgr_UpdateMeterIDForOnu(t *testing.T) {
	type args struct {
		Direction string
		IntfID    uint32
		OnuID     uint32
		UniID     uint32
		tpID      uint32
		MeterInfo *MeterInfo
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"UpdateMeterIDForOnu-1", getResMgr(), args{"DOWNSTREAM", 1, 2,
			2, 64, &MeterInfo{}}, errors.New("failed to get Meter config from kvstore for path")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RsrcMgr := testResMgrObject(tt.fields)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := RsrcMgr.StoreMeterInfoForOnu(ctx, tt.args.Direction, tt.args.IntfID, tt.args.OnuID, tt.args.UniID,
				tt.args.tpID, tt.args.MeterInfo); reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr) && err != nil {
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
		{"UpdateTechProfileIDForOnu-1", getResMgr(), args{1, 2, 2,
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
		backend       string
		address       string
		DeviceID      string
		kvStorePrefix string
	}
	tests := []struct {
		name string
		args args
		want *db.Backend
	}{
		{"setKVClient-1", args{"etcd", "1.1.1.1:1", "olt1", "service/voltha"}, &db.Backend{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SetKVClient(context.Background(), tt.args.backend, tt.args.address, tt.args.DeviceID, tt.args.kvStorePrefix); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("SetKVClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newKVClient(t *testing.T) {
	type args struct {
		storeType string
		address   string
		timeout   time.Duration
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
			got, err := newKVClient(context.Background(), tt.args.storeType, tt.args.address, tt.args.timeout)
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
			err := RsrcMgr.RemoveFlowGroupFromKVStore(ctx, tt.args.groupID, tt.args.cached)
			if err != nil {
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
