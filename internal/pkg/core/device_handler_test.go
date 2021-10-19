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

//Package core provides the utility for olt devices, flows and statistics
package core

import (
	"context"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	conf "github.com/opencord/voltha-lib-go/v7/pkg/config"
	vgrpc "github.com/opencord/voltha-lib-go/v7/pkg/grpc"

	"github.com/opencord/voltha-lib-go/v7/pkg/db"
	fu "github.com/opencord/voltha-lib-go/v7/pkg/flows"
	plt "github.com/opencord/voltha-lib-go/v7/pkg/platform"
	"github.com/opencord/voltha-lib-go/v7/pkg/pmmetrics"
	ponrmgr "github.com/opencord/voltha-lib-go/v7/pkg/ponresourcemanager"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"
	"github.com/opencord/voltha-openolt-adapter/pkg/mocks"
	ia "github.com/opencord/voltha-protos/v5/go/inter_adapter"
	of "github.com/opencord/voltha-protos/v5/go/openflow_13"
	ofp "github.com/opencord/voltha-protos/v5/go/openflow_13"
	oop "github.com/opencord/voltha-protos/v5/go/openolt"
	"github.com/opencord/voltha-protos/v5/go/voltha"
)

const (
	NumPonPorts  = 16
	OnuIDStart   = 1
	OnuIDEnd     = 32
	AllocIDStart = 1
	AllocIDEnd   = 10
	GemIDStart   = 1
	GemIDEnd     = 10
	FlowIDStart  = 1
	FlowIDEnd    = 10
)

func newMockOnuInterAdapterService() *mocks.MockOnuInterAdapterService {
	return &mocks.MockOnuInterAdapterService{}
}

func newMockCoreService() *mocks.MockCoreService {
	mcp := mocks.MockCoreService{
		Devices:     make(map[string]*voltha.Device),
		DevicePorts: make(map[string][]*voltha.Port),
	}
	var pm []*voltha.PmConfig
	mcp.Devices["olt"] = &voltha.Device{
		Id:              "olt",
		Root:            true,
		ParentId:        "logical_device",
		ParentPortNo:    1,
		AdapterEndpoint: "mock-olt-endpoint",
		ProxyAddress: &voltha.Device_ProxyAddress{
			DeviceId:       "olt",
			DeviceType:     "onu",
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
		PmConfigs: &voltha.PmConfigs{
			DefaultFreq:  10,
			Id:           "olt",
			FreqOverride: false,
			Grouped:      false,
			Metrics:      pm,
		},
	}
	mcp.DevicePorts["olt"] = []*voltha.Port{
		{PortNo: 1, Label: "pon"},
		{PortNo: 2, Label: "nni"},
	}

	mcp.Devices["onu1"] = &voltha.Device{
		Id:              "1",
		Root:            false,
		ParentId:        "olt",
		ParentPortNo:    1,
		AdapterEndpoint: "mock-onu-endpoint",
		OperStatus:      4,
		ProxyAddress: &voltha.Device_ProxyAddress{
			OnuId:          1,
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
		PmConfigs: &voltha.PmConfigs{
			DefaultFreq:  10,
			Id:           "olt",
			FreqOverride: false,
			Grouped:      false,
			Metrics:      pm,
		},
	}
	mcp.DevicePorts["onu1"] = []*voltha.Port{
		{PortNo: 1, Label: "pon"},
		{PortNo: 2, Label: "uni"},
	}

	mcp.Devices["onu2"] = &voltha.Device{
		Id:              "2",
		Root:            false,
		ParentId:        "olt",
		OperStatus:      2,
		AdapterEndpoint: "mock-onu-endpoint",
		ParentPortNo:    1,

		ProxyAddress: &voltha.Device_ProxyAddress{
			OnuId:          2,
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
		PmConfigs: &voltha.PmConfigs{
			DefaultFreq:  10,
			Id:           "olt",
			FreqOverride: false,
			Grouped:      false,
			Metrics:      pm,
		},
	}
	mcp.DevicePorts["onu2"] = []*voltha.Port{
		{PortNo: 1, Label: "pon"},
		{PortNo: 2, Label: "uni"},
	}
	return &mcp
}
func newMockDeviceHandler() *DeviceHandler {
	device := &voltha.Device{
		Id:              "olt",
		Root:            true,
		ParentId:        "logical_device",
		AdapterEndpoint: "mock-olt-endpoint",
		ProxyAddress: &voltha.Device_ProxyAddress{
			DeviceId:       "olt",
			DeviceType:     "onu",
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
	}
	mcs := newMockCoreService()
	cc := mocks.NewMockCoreClient(mcs)
	ep := &mocks.MockEventProxy{}
	cm := &conf.ConfigManager{}
	cm.Backend = &db.Backend{StoreType: "etcd", Client: &mocks.MockKVClient{}}
	cfg := &config.AdapterFlags{OmccEncryption: true}
	openOLT := &OpenOLT{eventProxy: ep, config: cfg}
	dh := NewDeviceHandler(cc, ep, device, openOLT, cm, cfg)
	oopRanges := []*oop.DeviceInfo_DeviceResourceRanges{{
		IntfIds:    []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		Technology: "xgs-pon",
		Pools:      []*oop.DeviceInfo_DeviceResourceRanges_Pool{{}},
	}}

	deviceInf := &oop.DeviceInfo{Vendor: "openolt", Ranges: oopRanges, Model: "openolt", DeviceId: dh.device.Id, PonPorts: NumPonPorts}
	dh.deviceInfo = deviceInf
	dh.device = device
	dh.resourceMgr = make([]*resourcemanager.OpenOltResourceMgr, deviceInf.PonPorts)
	var i uint32
	for i = 0; i < deviceInf.PonPorts; i++ {
		dh.resourceMgr[i] = &resourcemanager.OpenOltResourceMgr{DeviceID: dh.device.Id, DeviceType: dh.device.Type, DevInfo: deviceInf,
			KVStore: &db.Backend{
				StoreType: "etcd",
				Client:    &mocks.MockKVClient{},
			}}
		dh.resourceMgr[i].InitLocalCache()
	}

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

	ponmgr := &ponrmgr.PONResourceManager{}
	ponmgr.DeviceID = "onu-1"
	ponmgr.IntfIDs = []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	ponmgr.KVStore = &db.Backend{
		Client: &mocks.MockKVClient{},
	}
	ponmgr.KVStoreForConfig = &db.Backend{
		Client: &mocks.MockKVClient{},
	}
	ponmgr.Backend = "etcd"
	ponmgr.PonResourceRanges = ranges
	ponmgr.SharedIdxByType = sharedIdxByType
	ponmgr.Technology = "XGS-PON"
	for i = 0; i < deviceInf.PonPorts; i++ {
		dh.resourceMgr[i].PonRsrMgr = ponmgr
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dh.groupMgr = NewGroupManager(ctx, dh, dh.resourceMgr[0])
	dh.totalPonPorts = NumPonPorts
	dh.flowMgr = make([]*OpenOltFlowMgr, dh.totalPonPorts)
	for i = 0; i < dh.totalPonPorts; i++ {
		dh.flowMgr[i] = &OpenOltFlowMgr{}
		dh.flowMgr[i].deviceHandler = dh
		dh.flowMgr[i].ponPortIdx = i
		dh.flowMgr[i].grpMgr = dh.groupMgr
		dh.flowMgr[i].resourceMgr = dh.resourceMgr[i]
		dh.flowMgr[i].techprofile = mocks.MockTechProfile{}
		dh.flowMgr[i].gemToFlowIDs = make(map[uint32][]uint64)
		dh.flowMgr[i].packetInGemPort = make(map[resourcemanager.PacketInInfoKey]uint32)
		dh.flowMgr[i].flowIDToGems = make(map[uint64][]uint32)

		dh.resourceMgr[i].TechprofileRef = dh.flowMgr[i].techprofile

		// Create a slice of buffered channels for handling concurrent flows per ONU.
		// The additional entry (+1) is to handle the NNI trap flows on a separate channel from individual ONUs channel
		dh.flowMgr[i].incomingFlows = make([]chan flowControlBlock, plt.MaxOnusPerPon+1)
		dh.flowMgr[i].stopFlowHandlerRoutine = make([]chan bool, plt.MaxOnusPerPon+1)
		dh.flowMgr[i].flowHandlerRoutineActive = make([]bool, plt.MaxOnusPerPon+1)
		for j := range dh.flowMgr[i].incomingFlows {
			dh.flowMgr[i].incomingFlows[j] = make(chan flowControlBlock, maxConcurrentFlowsPerOnu)
			dh.flowMgr[i].stopFlowHandlerRoutine[j] = make(chan bool, 1)
			// Spin up a go routine to handling incoming flows (add/remove).
			// There will be on go routine per ONU.
			// This routine will be blocked on the flowMgr.incomingFlows[onu-id] channel for incoming flows.
			dh.flowMgr[i].flowHandlerRoutineActive[j] = true
			go dh.flowMgr[i].perOnuFlowHandlerRoutine(j, dh.flowMgr[i].incomingFlows[j], dh.flowMgr[i].stopFlowHandlerRoutine[j])
		}
		dh.flowMgr[i].onuGemInfoMap = make(map[uint32]*resourcemanager.OnuGemInfo)
	}
	dh.Client = &mocks.MockOpenoltClient{}
	dh.eventMgr = &OpenOltEventMgr{eventProxy: &mocks.MockEventProxy{}, handler: dh}
	dh.transitionMap = &TransitionMap{}
	dh.portStats = &OpenOltStatisticsMgr{}

	var pmNames = []string{
		"rx_bytes",
		"rx_packets",
		"rx_mcast_packets",
		"rx_bcast_packets",
		"tx_bytes",
		"tx_packets",
		"tx_mcast_packets",
		"tx_bcast_packets",
	}

	dh.metrics = pmmetrics.NewPmMetrics(device.Id, pmmetrics.Frequency(2), pmmetrics.FrequencyOverride(false), pmmetrics.Grouped(false), pmmetrics.Metrics(pmNames))

	// Set the children endpoints
	dh.childAdapterClients = map[string]*vgrpc.Client{
		"mock-onu-endpoint": mocks.NewMockChildAdapterClient(newMockOnuInterAdapterService()),
	}
	return dh
}

func negativeDeviceHandler() *DeviceHandler {
	dh := newMockDeviceHandler()
	device := dh.device
	device.Id = ""
	return dh
}

func negativeDeviceHandlerNilFlowMgr() *DeviceHandler {
	dh := newMockDeviceHandler()
	dh.flowMgr = nil
	return dh
}

func Test_generateMacFromHost(t *testing.T) {
	ctx := context.Background()
	type args struct {
		host string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"generateMacFromHost-1", args{host: "localhost"}, "00:00:7f:00:00:01", false},
		{"generateMacFromHost-2", args{host: "10.10.10.10"}, "00:00:0a:0a:0a:0a", false},
		//{"generateMacFromHost-3", args{host: "google.com"}, "00:00:d8:3a:c8:8e", false},
		{"generateMacFromHost-4", args{host: "testing3"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateMacFromHost(ctx, tt.args.host)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateMacFromHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("generateMacFromHost() = %v, want %v", got, tt.want)
			}
		})
	}
}
func Test_macifyIP(t *testing.T) {
	type args struct {
		ip net.IP
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		"macifyIP-1",
		args{ip: net.ParseIP("10.10.10.10")},
		"00:00:0a:0a:0a:0a",
	},
		{
			"macifyIP-2",
			args{ip: net.ParseIP("127.0.0.1")},
			"00:00:7f:00:00:01",
		},
		{
			"macifyIP-3",
			args{ip: net.ParseIP("127.0.0.1/24")},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := macifyIP(tt.args.ip); got != tt.want {
				t.Errorf("macifyIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func sparseCompare(keys []string, spec, target interface{}) bool {
	if spec == target {
		return true
	}
	if spec == nil || target == nil {
		return false
	}
	typeSpec := reflect.TypeOf(spec)
	typeTarget := reflect.TypeOf(target)
	if typeSpec != typeTarget {
		return false
	}

	vSpec := reflect.ValueOf(spec)
	vTarget := reflect.ValueOf(target)
	if vSpec.Kind() == reflect.Ptr {
		vSpec = vSpec.Elem()
		vTarget = vTarget.Elem()
	}

	for _, key := range keys {
		fSpec := vSpec.FieldByName(key)
		fTarget := vTarget.FieldByName(key)
		if !reflect.DeepEqual(fSpec.Interface(), fTarget.Interface()) {
			return false
		}
	}
	return true
}

func TestDeviceHandler_GetChildDevice(t *testing.T) {
	ctx := context.Background()
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	type args struct {
		parentPort uint32
		onuID      uint32
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
		want          *voltha.Device
		errType       reflect.Type
	}{
		{"GetChildDevice-1", dh1,
			args{parentPort: 1,
				onuID: 1},
			&voltha.Device{
				Id:           "1",
				ParentId:     "olt",
				ParentPortNo: 1,
			},
			nil,
		},
		{"GetChildDevice-2", dh2,
			args{parentPort: 1,
				onuID: 1},
			nil,
			reflect.TypeOf(&olterrors.ErrNotFound{}),
		},
	}

	/*
	   --- FAIL: TestDeviceHandler_GetChildDevice/GetChildDevice-1 (0.00s)
	       device_handler_test.go:309: GetportLabel() => want=(, <nil>) got=(id:"1" parent_id:"olt" parent_port_no:1 proxy_address:<channel_id:1 channel_group_id:1 onu_id:1 > oper_status:ACTIVE connect_status:UNREACHABLE ports:<port_no:1 label:"pon" > ports:<port_no:2 label:"uni" > pm_configs:<id:"olt" default_freq:10 > , <nil>)
	   --- FAIL: TestDeviceHandler_GetChildDevice/GetChildDevice-2 (0.00s)
	*/
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.devicehandler.GetChildDevice(ctx, tt.args.parentPort, tt.args.onuID)
			if reflect.TypeOf(err) != tt.errType || !sparseCompare([]string{"Id", "ParentId", "ParentPortNo"}, tt.want, got) {
				t.Errorf("GetportLabel() => want=(%v, %v) got=(%v, %v)",
					tt.want, tt.errType, got, reflect.TypeOf(err))
				return
			}
			t.Log("onu device id", got)
		})
	}
}

func TestGetportLabel(t *testing.T) {
	invalid := reflect.TypeOf(&olterrors.ErrInvalidValue{})
	type args struct {
		portNum  uint32
		portType voltha.Port_PortType
	}
	tests := []struct {
		name    string
		args    args
		want    string
		errType reflect.Type
	}{
		{"GetportLabel-1", args{portNum: 0, portType: 0}, "", invalid},
		{"GetportLabel-2", args{portNum: 1, portType: 1}, "nni-1", nil},
		{"GetportLabel-3", args{portNum: 2, portType: 2}, "", invalid},
		{"GetportLabel-4", args{portNum: 3, portType: 3}, "pon-3", nil},
		{"GetportLabel-5", args{portNum: 4, portType: 4}, "", invalid},
		{"GetportLabel-6", args{portNum: 5, portType: 5}, "", invalid},
		{"GetportLabel-7", args{portNum: 6, portType: 6}, "", invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetportLabel(tt.args.portNum, tt.args.portType)
			if reflect.TypeOf(err) != tt.errType || got != tt.want {
				t.Errorf("GetportLabel() => want=(%v, %v) got=(%v, %v)",
					tt.want, tt.errType, got, reflect.TypeOf(err))
			}

		})
	}
}

// func TestDeviceHandler_ProcessInterAdapterMessage(t *testing.T) {
// 	ctx := context.Background()
// 	dh := newMockDeviceHandler()
// 	proxyAddr := dh.device.ProxyAddress
// 	body := &ca.InterAdapterOmciMessage{
// 		Message:      []byte("asdfasdfasdfasdfas"),
// 		ProxyAddress: proxyAddr,
// 	}
// 	body2 := &ca.InterAdapterOmciMessage{
// 		Message: []byte("asdfasdfasdfasdfas"),
// 		//ProxyAddress: &voltha.Device_ProxyAddress{},
// 	}
// 	body3 := &ca.InterAdapterTechProfileDownloadMessage{}
// 	var marshalledData *any.Any
// 	var err error

// 	if marshalledData, err = ptypes.MarshalAny(body); err != nil {
// 		logger.Errorw(ctx, "cannot-marshal-request", log.Fields{"err": err})
// 	}

// 	var marshalledData1 *any.Any

// 	if marshalledData1, err = ptypes.MarshalAny(body2); err != nil {
// 		logger.Errorw(ctx, "cannot-marshal-request", log.Fields{"err": err})
// 	}
// 	var marshalledData2 *any.Any

// 	if marshalledData2, err = ptypes.MarshalAny(body3); err != nil {
// 		logger.Errorw(ctx, "cannot-marshal-request", log.Fields{"err": err})
// 	}
// 	type args struct {
// 		msg *ca.InterAdapterMessage
// 	}
// 	invalid := reflect.TypeOf(&olterrors.ErrInvalidValue{})
// 	tests := []struct {
// 		name    string
// 		args    args
// 		wantErr reflect.Type
// 	}{
// 		{"ProcessInterAdapterMessage-1", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_FLOW_REQUEST,
// 			},
// 			Body: marshalledData,
// 		}}, invalid},
// 		{"ProcessInterAdapterMessage-2", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_FLOW_RESPONSE,
// 			},
// 			Body: marshalledData1,
// 		}}, invalid},
// 		{"ProcessInterAdapterMessage-3", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_OMCI_REQUEST,
// 			},
// 			Body: marshalledData,
// 		}}, reflect.TypeOf(&olterrors.ErrCommunication{})},
// 		{"ProcessInterAdapterMessage-4", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_OMCI_RESPONSE,
// 			}, Body: marshalledData,
// 		}}, invalid},
// 		{"ProcessInterAdapterMessage-5", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_METRICS_REQUEST,
// 			}, Body: marshalledData1,
// 		}}, invalid},
// 		{"ProcessInterAdapterMessage-6", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_METRICS_RESPONSE,
// 			}, Body: marshalledData,
// 		}}, invalid},
// 		{"ProcessInterAdapterMessage-7", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_ONU_IND_REQUEST,
// 			}, Body: marshalledData,
// 		}}, invalid},
// 		{"ProcessInterAdapterMessage-8", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_ONU_IND_RESPONSE,
// 			}, Body: marshalledData,
// 		}}, invalid},
// 		{"ProcessInterAdapterMessage-9", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_TECH_PROFILE_DOWNLOAD_REQUEST,
// 			}, Body: marshalledData,
// 		}}, invalid},
// 		{"ProcessInterAdapterMessage-10", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_DELETE_GEM_PORT_REQUEST,
// 			}, Body: marshalledData2,
// 		}}, invalid},
// 		{"ProcessInterAdapterMessage-11", args{msg: &ca.InterAdapterMessage{
// 			Header: &ca.InterAdapterHeader{
// 				Id:   "012345",
// 				Type: ca.InterAdapterMessageType_DELETE_TCONT_REQUEST,
// 			}, Body: marshalledData2,
// 		}}, invalid},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {

// 			if err := dh.ProcessInterAdapterMessage(ctx, tt.args.msg); reflect.TypeOf(err) != tt.wantErr {
// 				t.Errorf("DeviceHandler.ProcessInterAdapterMessage() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

func TestDeviceHandler_ProxyOmciMessage(t *testing.T) {
	ctx := context.Background()
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	device1 := &voltha.Device{
		Id:       "onu1",
		Root:     false,
		ParentId: "logical_device",
		ProxyAddress: &voltha.Device_ProxyAddress{
			DeviceId:       "onu1",
			DeviceType:     "onu",
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
	}
	device2 := device1
	device2.ConnectStatus = 2
	iaomciMsg1 := &ia.OmciMessage{
		ProxyAddress: &voltha.Device_ProxyAddress{
			DeviceId:       "onu2",
			DeviceType:     "onu",
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
	}
	iaomciMsg2 := &ia.OmciMessage{
		ProxyAddress: &voltha.Device_ProxyAddress{
			DeviceId:       "onu3",
			DeviceType:     "onu",
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
	}
	type args struct {
		onuDevice *voltha.Device
		omciMsg   *ia.OmciMessage
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
	}{
		{"sendProxiedMessage-1", dh1, args{onuDevice: device1, omciMsg: &ia.OmciMessage{}}},
		{"sendProxiedMessage-2", dh1, args{onuDevice: device2, omciMsg: &ia.OmciMessage{}}},
		{"sendProxiedMessage-3", dh1, args{onuDevice: nil, omciMsg: iaomciMsg1}},
		{"sendProxiedMessage-4", dh1, args{onuDevice: nil, omciMsg: iaomciMsg2}},
		{"sendProxiedMessage-5", dh2, args{onuDevice: nil, omciMsg: iaomciMsg2}},
		{"sendProxiedMessage-6", dh2, args{onuDevice: device1, omciMsg: &ia.OmciMessage{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = tt.devicehandler.ProxyOmciMessage(ctx, tt.args.omciMsg)
			//TODO: actually verify test cases
		})
	}
}

func TestDeviceHandler_SendPacketInToCore(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()

	type args struct {
		logicalPort   uint32
		packetPayload []byte
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
	}{
		{"SendPacketInToCore-1", dh1, args{logicalPort: 1, packetPayload: []byte("test1")}},
		{"SendPacketInToCore-2", dh1, args{logicalPort: 1, packetPayload: []byte("")}},
		{"SendPacketInToCore-3", dh2, args{logicalPort: 1, packetPayload: []byte("test1")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = tt.devicehandler.SendPacketInToCore(context.Background(), tt.args.logicalPort, tt.args.packetPayload)
			//TODO: actually verify test cases
		})
	}
}

func TestDeviceHandler_DisableDevice(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
		wantErr       bool
	}{
		{"DisableDevice-1", dh1, args{device: dh1.device}, false},
		{"DisableDevice-2", dh1, args{device: dh2.device}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.devicehandler.DisableDevice(context.Background(), tt.args.device); (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.DisableDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeviceHandler_ReenableDevice(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
		wantErr       bool
	}{
		{"ReenableDevice-1", dh1, args{device: dh1.device}, false},
		{"ReenableDevice-2", dh1, args{device: &voltha.Device{}}, true},
		{"ReenableDevice-3", dh2, args{device: dh1.device}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dh := tt.devicehandler
			if err := dh.ReenableDevice(context.Background(), tt.args.device); (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.ReenableDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeviceHandler_RebootDevice(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := newMockDeviceHandler()
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
		wantErr       bool
	}{
		// TODO: Add test cases.
		{"RebootDevice-1", dh1, args{device: dh1.device}, false},
		{"RebootDevice-2", dh1, args{device: dh2.device}, true},
		{"RebootDevice-3", dh2, args{device: dh2.device}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if err := tt.devicehandler.RebootDevice(context.Background(), tt.args.device); (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.RebootDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeviceHandler_handleIndication(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	dh3 := newMockDeviceHandler()
	dh3.onus = sync.Map{}
	dh3.onus.Store("onu1", NewOnuDevice("onu1", "onu1", "onu1", 1, 1, "onu1", false, "mock_endpoint"))
	dh3.onus.Store("onu2", NewOnuDevice("onu2", "onu2", "onu2", 2, 2, "onu2", false, "mock_endpoint"))

	type args struct {
		indication *oop.Indication
	}
	tests := []struct {
		name          string
		deviceHandler *DeviceHandler
		args          args
	}{
		// TODO: Add test cases.
		{"handleIndication-1", dh1, args{indication: &oop.Indication{Data: &oop.Indication_OltInd{OltInd: &oop.OltIndication{OperState: "up"}}}}},
		{"handleIndication-2", dh1, args{indication: &oop.Indication{Data: &oop.Indication_OltInd{OltInd: &oop.OltIndication{OperState: "down"}}}}},
		{"handleIndication-3", dh1, args{indication: &oop.Indication{Data: &oop.Indication_IntfInd{IntfInd: &oop.IntfIndication{IntfId: 1, OperState: "up"}}}}},
		{"handleIndication-4", dh1, args{indication: &oop.Indication{Data: &oop.Indication_IntfInd{IntfInd: &oop.IntfIndication{IntfId: 1, OperState: "down"}}}}},
		{"handleIndication-5", dh1, args{indication: &oop.Indication{Data: &oop.Indication_IntfOperInd{IntfOperInd: &oop.IntfOperIndication{Type: "nni", IntfId: 1, OperState: "up"}}}}},
		{"handleIndication-6", dh1, args{indication: &oop.Indication{Data: &oop.Indication_IntfOperInd{IntfOperInd: &oop.IntfOperIndication{Type: "pon", IntfId: 1, OperState: "up"}}}}},
		{"handleIndication-7", dh1, args{indication: &oop.Indication{Data: &oop.Indication_OnuDiscInd{OnuDiscInd: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}}}}},
		{"handleIndication-8", dh1, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "up", AdminState: "up"}}}}},
		{"handleIndication-9", dh1, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "up", AdminState: "down"}}}}},
		{"handleIndication-10", dh1, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "down", AdminState: "up"}}}}},
		{"handleIndication-11", dh1, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "down", AdminState: "down"}}}}},
		{"handleIndication-12", dh1, args{indication: &oop.Indication{Data: &oop.Indication_OmciInd{OmciInd: &oop.OmciIndication{IntfId: 1, OnuId: 1, Pkt: []byte("onu123-random value")}}}}},
		{"handleIndication-13", dh1, args{indication: &oop.Indication{Data: &oop.Indication_PktInd{PktInd: &oop.PacketIndication{IntfType: "nni", IntfId: 1, GemportId: 1, FlowId: 1234, PortNo: 1}}}}},
		{"handleIndication-14", dh1, args{indication: &oop.Indication{Data: &oop.Indication_PortStats{PortStats: &oop.PortStatistics{IntfId: 1, RxBytes: 100, RxPackets: 100, RxUcastPackets: 100, RxMcastPackets: 100, RxBcastPackets: 100, RxErrorPackets: 100, TxBytes: 100, TxPackets: 100, TxUcastPackets: 100, TxMcastPackets: 100, TxBcastPackets: 100, TxErrorPackets: 100, RxCrcErrors: 100, BipErrors: 100, Timestamp: 1000}}}}},
		{"handleIndication-15", dh1, args{indication: &oop.Indication{Data: &oop.Indication_FlowStats{FlowStats: &oop.FlowStatistics{RxBytes: 100, RxPackets: 100, TxBytes: 100, TxPackets: 100, Timestamp: 1000}}}}},
		{"handleIndication-16", dh1, args{indication: &oop.Indication{Data: &oop.Indication_AlarmInd{AlarmInd: &oop.AlarmIndication{}}}}},
		{"handleIndication-17", dh1, args{indication: &oop.Indication{Data: &oop.Indication_PktInd{PktInd: &oop.PacketIndication{IntfType: "nni", FlowId: 1234, PortNo: 1}}}}},
		{"handleIndication-18", dh1, args{indication: &oop.Indication{Data: &oop.Indication_PktInd{PktInd: &oop.PacketIndication{}}}}},

		// Negative testcases
		{"handleIndication-19", dh2, args{indication: &oop.Indication{Data: &oop.Indication_OltInd{OltInd: &oop.OltIndication{OperState: "up"}}}}},
		{"handleIndication-20", dh2, args{indication: &oop.Indication{Data: &oop.Indication_OltInd{OltInd: &oop.OltIndication{OperState: "down"}}}}},
		{"handleIndication-21", dh2, args{indication: &oop.Indication{Data: &oop.Indication_IntfInd{IntfInd: &oop.IntfIndication{IntfId: 1, OperState: "up"}}}}},
		{"handleIndication-22", dh2, args{indication: &oop.Indication{Data: &oop.Indication_IntfInd{IntfInd: &oop.IntfIndication{IntfId: 1, OperState: "down"}}}}},
		{"handleIndication-23", dh2, args{indication: &oop.Indication{Data: &oop.Indication_IntfOperInd{IntfOperInd: &oop.IntfOperIndication{Type: "nni", IntfId: 1, OperState: "up"}}}}},
		{"handleIndication-24", dh2, args{indication: &oop.Indication{Data: &oop.Indication_IntfOperInd{IntfOperInd: &oop.IntfOperIndication{Type: "pon", IntfId: 1, OperState: "up"}}}}},
		{"handleIndication-25", dh2, args{indication: &oop.Indication{Data: &oop.Indication_OnuDiscInd{OnuDiscInd: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}}}}},
		{"handleIndication-26", dh2, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "up", AdminState: "up"}}}}},
		{"handleIndication-27", dh2, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "up", AdminState: "down"}}}}},
		{"handleIndication-28", dh2, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "down", AdminState: "up"}}}}},
		{"handleIndication-29", dh2, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "down", AdminState: "down"}}}}},
		{"handleIndication-30", dh2, args{indication: &oop.Indication{Data: &oop.Indication_OmciInd{OmciInd: &oop.OmciIndication{IntfId: 1, OnuId: 1, Pkt: []byte("onu123-random value")}}}}},
		{"handleIndication-31", dh2, args{indication: &oop.Indication{Data: &oop.Indication_PktInd{PktInd: &oop.PacketIndication{IntfType: "nni", IntfId: 1, GemportId: 1, FlowId: 1234, PortNo: 1}}}}},
		{"handleIndication-32", dh2, args{indication: &oop.Indication{Data: &oop.Indication_PortStats{PortStats: &oop.PortStatistics{IntfId: 1, RxBytes: 100, RxPackets: 100, RxUcastPackets: 100, RxMcastPackets: 100, RxBcastPackets: 100, RxErrorPackets: 100, TxBytes: 100, TxPackets: 100, TxUcastPackets: 100, TxMcastPackets: 100, TxBcastPackets: 100, TxErrorPackets: 100, RxCrcErrors: 100, BipErrors: 100, Timestamp: 1000}}}}},
		{"handleIndication-33", dh2, args{indication: &oop.Indication{Data: &oop.Indication_FlowStats{FlowStats: &oop.FlowStatistics{RxBytes: 100, RxPackets: 100, TxBytes: 100, TxPackets: 100, Timestamp: 1000}}}}},
		{"handleIndication-34", dh2, args{indication: &oop.Indication{Data: &oop.Indication_AlarmInd{AlarmInd: &oop.AlarmIndication{}}}}},
		//
		{"handleIndication-35", dh3, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "up", AdminState: "up"}}}}},
		{"handleIndication-36", dh3, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "down", AdminState: "up"}}}}},
		{"handleIndication-37", dh3, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "up", AdminState: "down"}}}}},
		{"handleIndication-38", dh3, args{indication: &oop.Indication{Data: &oop.Indication_OnuInd{OnuInd: &oop.OnuIndication{IntfId: 1, OnuId: 1, OperState: "down", AdminState: "down"}}}}},
		{"handleIndication-30", dh1, args{indication: &oop.Indication{Data: &oop.Indication_OmciInd{OmciInd: &oop.OmciIndication{IntfId: 1, OnuId: 4, Pkt: []byte("onu123-random value")}}}}},
		{"handleIndication-30", dh2, args{indication: &oop.Indication{Data: &oop.Indication_OmciInd{OmciInd: &oop.OmciIndication{IntfId: 1, OnuId: 4, Pkt: []byte("onu123-random value")}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dh := tt.deviceHandler
			time.Sleep(5 * time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			dh.handleIndication(ctx, tt.args.indication)
		})
	}
}

func TestDeviceHandler_addPort(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	type args struct {
		intfID   uint32
		portType voltha.Port_PortType
		state    string
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
	}{
		// State up
		{"addPort.1", dh1, args{intfID: 1, portType: voltha.Port_UNKNOWN, state: "up"}},
		{"addPort.2", dh1, args{intfID: 1, portType: voltha.Port_VENET_OLT, state: "up"}},
		{"addPort.3", dh1, args{intfID: 1, portType: voltha.Port_VENET_ONU, state: "up"}},
		{"addPort.4", dh1, args{intfID: 1, portType: voltha.Port_ETHERNET_NNI, state: "up"}},
		{"addPort.5", dh1, args{intfID: 1, portType: voltha.Port_ETHERNET_UNI, state: "up"}},
		{"addPort.6", dh1, args{intfID: 1, portType: voltha.Port_PON_OLT, state: "up"}},
		{"addPort.7", dh1, args{intfID: 1, portType: voltha.Port_PON_ONU, state: "up"}},
		{"addPort.8", dh1, args{intfID: 1, portType: 8, state: "up"}},
		// state discovery
		{"addPort.9", dh1, args{intfID: 1, portType: voltha.Port_UNKNOWN, state: "down"}},
		{"addPort.10", dh1, args{intfID: 1, portType: voltha.Port_VENET_OLT, state: "down"}},
		{"addPort.11", dh1, args{intfID: 1, portType: voltha.Port_VENET_ONU, state: "down"}},
		{"addPort.12", dh1, args{intfID: 1, portType: voltha.Port_ETHERNET_NNI, state: "down"}},
		{"addPort.13", dh1, args{intfID: 1, portType: voltha.Port_ETHERNET_UNI, state: "down"}},
		{"addPort.14", dh1, args{intfID: 1, portType: voltha.Port_PON_OLT, state: "down"}},
		{"addPort.15", dh1, args{intfID: 1, portType: voltha.Port_PON_ONU, state: "down"}},
		{"addPort.16", dh1, args{intfID: 1, portType: 8, state: "down"}},

		{"addPort.17", dh2, args{intfID: 1, portType: voltha.Port_ETHERNET_NNI, state: "up"}},
		{"addPort.18", dh2, args{intfID: 1, portType: voltha.Port_ETHERNET_UNI, state: "up"}},
		{"addPort.19", dh2, args{intfID: 1, portType: voltha.Port_ETHERNET_NNI, state: "down"}},
		{"addPort.20", dh2, args{intfID: 1, portType: voltha.Port_ETHERNET_UNI, state: "down"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = tt.devicehandler.addPort(context.Background(), tt.args.intfID, tt.args.portType, tt.args.state)
			//TODO: actually verify test cases
		})
	}
}

func Test_macAddressToUint32Array(t *testing.T) {
	type args struct {
		mac string
	}
	tests := []struct {
		name string
		args args
		want []uint32
	}{
		// TODO: Add test cases.
		{"macAddressToUint32Array-1", args{mac: "00:00:00:00:00:01"}, []uint32{0, 0, 0, 0, 0, 1}},
		{"macAddressToUint32Array-2", args{mac: "0abcdef"}, []uint32{11259375}},
		{"macAddressToUint32Array-3", args{mac: "testing"}, []uint32{1, 2, 3, 4, 5, 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := macAddressToUint32Array(tt.args.mac); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("macAddressToUint32Array() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceHandler_handleOltIndication(t *testing.T) {

	type args struct {
		oltIndication *oop.OltIndication
	}
	tests := []struct {
		name string
		args args
	}{
		{"handleOltIndication-1", args{oltIndication: &oop.OltIndication{OperState: "up"}}},
		{"handleOltIndication-2", args{oltIndication: &oop.OltIndication{OperState: "down"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dh := newMockDeviceHandler()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := dh.handleOltIndication(ctx, tt.args.oltIndication); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestDeviceHandler_AdoptDevice(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
	}{
		// TODO: Add test cases.
		{"AdoptDevice-1", dh1, args{device: dh1.device}},
		{"AdoptDevice-2", dh2, args{device: dh2.device}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//dh.doStateInit()
			//	context.
			//dh.AdoptDevice(tt.args.device)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tt.devicehandler.postInit(ctx); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestDeviceHandler_activateONU(t *testing.T) {
	dh := newMockDeviceHandler()
	dh1 := negativeDeviceHandler()
	type args struct {
		intfID       uint32
		onuID        int64
		serialNum    *oop.SerialNumber
		serialNumber string
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
	}{
		{"activateONU-1", dh, args{intfID: 0, onuID: 1, serialNum: &oop.SerialNumber{VendorId: []byte("onu1")}}},
		{"activateONU-2", dh, args{intfID: 1, onuID: 2, serialNum: &oop.SerialNumber{VendorId: []byte("onu2")}}},
		{"activateONU-3", dh1, args{intfID: 0, onuID: 1, serialNum: &oop.SerialNumber{VendorId: []byte("onu1")}}},
		{"activateONU-4", dh1, args{intfID: 1, onuID: 2, serialNum: &oop.SerialNumber{VendorId: []byte("onu2")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tt.devicehandler.activateONU(ctx, tt.args.intfID, tt.args.onuID, tt.args.serialNum, tt.args.serialNumber)
			//TODO: actually verify test cases
		})
	}
}

func TestDeviceHandler_start(t *testing.T) {
	dh := newMockDeviceHandler()
	dh1 := negativeDeviceHandler()
	dh.start(context.Background())
	dh.stop(context.Background())

	dh1.start(context.Background())
	dh1.stop(context.Background())

}

func TestDeviceHandler_PacketOut(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	acts := []*ofp.OfpAction{
		fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA))),
		fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 101)),
		fu.Output(1),
	}
	pktout := &ofp.OfpPacketOut{BufferId: 0, InPort: 1, Actions: acts, Data: []byte("AYDCAAAOAODsSE5TiMwCBwQA4OxITlIEBQUwLzUxBgIAFAgEMC81MQoJbG9jYWxob3N0EBwFAawbqqACAAAAoRAxLjMuNi4xLjQuMS40NDEz/gYAgMILAgD+GQCAwgkDAAAAAGQAAAAAAAAAAgICAgICAgL+GQCAwgoDAAAAAGQAAAAAAAAAAgICAgICAgIAAA==")}
	type args struct {
		egressPortNo int
		packet       *of.OfpPacketOut
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
		wantErr       bool
	}{
		// TODO: Add test cases.
		//{"test1", args{egressPortNo: 0, packet: &ofp.OfpPacketOut{}}, true},
		{"PacketOut-1", dh1, args{egressPortNo: 0, packet: pktout}, false},
		{"PacketOut-2", dh2, args{egressPortNo: 1, packet: pktout}, false},
		{"PacketOut-3", dh2, args{egressPortNo: 4112, packet: pktout}, false},
		{"PacketOut-4", dh1, args{egressPortNo: 16777217, packet: pktout}, false},
		{"PacketOut-5", dh2, args{egressPortNo: 16777216, packet: pktout}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dh := tt.devicehandler
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := dh.PacketOut(ctx, uint32(tt.args.egressPortNo), tt.args.packet); (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.PacketOut() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

//
func TestDeviceHandler_doStateUp(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := newMockDeviceHandler()

	dh2.device.Id = ""
	dh3 := negativeDeviceHandler()

	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		wantErr       bool
	}{
		{"dostateup-1", dh1, false},
		{"dostateup-2", dh2, false},
		{"dostateup-3", dh3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tt.devicehandler.doStateUp(ctx); (err != nil) != tt.wantErr {
				t.Logf("DeviceHandler.doStateUp() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.devicehandler.stopCollector <- true //stop the stat collector invoked from doStateUp
		})
	}
}
func TestDeviceHandler_doStateDown(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	dh3 := newMockDeviceHandler()
	dh3.device.OperStatus = voltha.OperStatus_UNKNOWN
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		wantErr       bool
	}{
		{"dostatedown-1", dh1, false},
		{"dostatedown-2", dh2, true},
		{"dostatedown-2", dh3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tt.devicehandler.doStateDown(ctx); (err != nil) != tt.wantErr {
				t.Logf("DeviceHandler.doStateDown() error = %v", err)
				//TODO: should fail this test case (Errorf) if result is not as expected
			}
		})
	}
}

func TestDeviceHandler_GetOfpDeviceInfo(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
		wantErr       bool
	}{
		// TODO: Add test cases.
		{"GetOfpDeviceInfo-1", dh1, args{dh1.device}, false},
		{"GetOfpDeviceInfo-2", dh1, args{&voltha.Device{}}, false},
		{"GetOfpDeviceInfo-3", dh2, args{dh1.device}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dh := tt.devicehandler
			_, err := dh.GetOfpDeviceInfo(tt.args.device)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.GetOfpDeviceInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDeviceHandler_onuDiscIndication(t *testing.T) {

	dh1 := newMockDeviceHandler()
	dh1.discOnus = sync.Map{}
	dh1.discOnus.Store("onu1", true)
	dh1.discOnus.Store("onu2", false)
	dh1.discOnus.Store("onu3", true)
	dh1.discOnus.Store("onu4", true)
	dh1.onus = sync.Map{}
	dh1.onus.Store("onu3", NewOnuDevice("onu3", "onu3", "onu3", 3, 3, "onu3", true, "mock_endpoint"))
	dh1.onus.Store("onu4", NewOnuDevice("onu4", "onu4", "onu4", 4, 4, "onu4", true, "mock_endpoint"))
	dh2 := negativeDeviceHandler()
	type args struct {
		onuDiscInd *oop.OnuDiscIndication
		sn         string
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
	}{
		// TODO: Add test cases.
		{"onuDiscIndication-1", dh1, args{onuDiscInd: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}}},
		{"onuDiscIndication-2", dh1, args{onuDiscInd: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{}}}},
		{"onuDiscIndication-3", dh1, args{onuDiscInd: &oop.OnuDiscIndication{SerialNumber: &oop.SerialNumber{}}}},
		{"onuDiscIndication-4", dh1, args{onuDiscInd: &oop.OnuDiscIndication{}}},
		{"onuDiscIndication-5", dh1, args{onuDiscInd: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}, sn: "onu1"}},
		{"onuDiscIndication-6", dh1, args{onuDiscInd: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}, sn: "onu2"}},
		{"onuDiscIndication-7", dh1, args{onuDiscInd: &oop.OnuDiscIndication{IntfId: 3, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}, sn: "onu3"}},
		{"onuDiscIndication-8", dh1, args{onuDiscInd: &oop.OnuDiscIndication{IntfId: 3, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}, sn: "onu4"}},
		{"onuDiscIndication-9", dh2, args{onuDiscInd: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tt.devicehandler.onuDiscIndication(ctx, tt.args.onuDiscInd)
			//TODO: actually verify test cases
		})
	}
}

func TestDeviceHandler_populateDeviceInfo(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	tests := []struct {
		name          string
		devicehandler *DeviceHandler

		wantErr bool
	}{
		// TODO: Add test cases.
		{"populateDeviceInfo-1", dh1, false},
		{"populateDeviceInfo-2", dh1, true},
		{"populateDeviceInfo-3", dh1, true},
		{"populateDeviceInfo-4", dh1, true},
		{"populateDeviceInfo-5", dh2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, err := tt.devicehandler.populateDeviceInfo(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.populateDeviceInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

		})
	}
}

func TestDeviceHandler_readIndications(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := newMockDeviceHandler()
	dh3 := newMockDeviceHandler()
	dh3.device.AdminState = voltha.AdminState_DISABLED
	dh4 := negativeDeviceHandler()
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
	}{
		// TODO: Add test cases.
		{"readIndications-1", dh1},
		{"readIndications-2", dh2},
		{"readIndications-3", dh2},
		{"readIndications-4", dh2},
		{"readIndications-5", dh2},
		{"readIndications-6", dh3},
		{"readIndications-7", dh3},
		{"readIndications-8", dh4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tt.devicehandler.readIndications(ctx)
			// TODO: actually verify test cases
		})
	}
}

func Test_startCollector(t *testing.T) {
	type args struct {
		dh *DeviceHandler
	}
	dh := newMockDeviceHandler()
	mcs := newMockCoreService()
	mcs.DevicePorts[dh.device.Id] = []*voltha.Port{
		{PortNo: 1, Label: "pon", Type: voltha.Port_PON_OLT},
		{PortNo: 16777216, Label: "nni", Type: voltha.Port_ETHERNET_NNI},
		{PortNo: 16777218, Label: "nni", Type: voltha.Port_ETHERNET_NNI},
	}
	dh.coreClient.SetService(mcs)
	dh.portStats.NorthBoundPort = make(map[uint32]*NniPort)
	dh.portStats.NorthBoundPort[1] = &NniPort{Name: "OLT-1"}
	dh.portStats.NorthBoundPort[2] = &NniPort{Name: "OLT-1"}
	dh.portStats.SouthBoundPort = make(map[uint32]*PonPort)
	dh.portStats.Device = dh
	for i := 0; i < 16; i++ {
		dh.portStats.SouthBoundPort[uint32(i)] = &PonPort{DeviceID: "OLT-1"}
	}
	dh1 := newMockDeviceHandler()
	mcs = newMockCoreService()
	mcs.DevicePorts[dh.device.Id] = []*voltha.Port{}
	dh.coreClient.SetService(mcs)
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"StartCollector-1", args{dh}},
		{"StartCollector-2", args{dh1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			go func() {
				time.Sleep(1 * time.Second) // simulated wait time to stop startCollector
				tt.args.dh.stopCollector <- true
			}()
			startCollector(context.Background(), tt.args.dh)
		})
	}
}

func TestDeviceHandler_TestReconcileStatus(t *testing.T) {

	// olt disconnect (not reboot)
	dh1 := newMockDeviceHandler()
	dh1.adapterPreviouslyConnected = false
	dh1.agentPreviouslyConnected = true

	// adapter restart
	dh2 := newMockDeviceHandler()
	dh2.Client = &mocks.MockOpenoltClient{}
	dh2.adapterPreviouslyConnected = true
	dh2.agentPreviouslyConnected = true

	// first connection or olt restart
	dh3 := newMockDeviceHandler()
	dh3.Client = &mocks.MockOpenoltClient{}
	dh3.adapterPreviouslyConnected = false
	dh3.agentPreviouslyConnected = false

	// olt and adapter restart at the same time (first case)
	dh4 := newMockDeviceHandler()
	dh4.Client = &mocks.MockOpenoltClient{}
	dh4.adapterPreviouslyConnected = true
	dh4.agentPreviouslyConnected = false

	// adapter restart and olt disconnect at the same time
	dh5 := newMockDeviceHandler()
	dh5.Client = &mocks.MockOpenoltClient{}
	dh5.adapterPreviouslyConnected = true
	dh5.agentPreviouslyConnected = true

	tests := []struct {
		name            string
		devicehandler   *DeviceHandler
		expectedRestart bool
		wantErr         bool
	}{
		{"dostateup-1", dh1, true, false},
		{"dostateup-2", dh2, false, false},
		{"dostateup-3", dh3, false, false},
		{"dostateup-4", dh4, true, false},
		{"dostateup-5", dh5, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tt.devicehandler.doStateUp(ctx); (err != nil) != tt.wantErr {
				t.Logf("DeviceHandler.doStateUp() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.devicehandler.stopCollector <- true //stop the stat collector invoked from doStateUp
			isRestarted := tt.devicehandler.Client.(*mocks.MockOpenoltClient).IsRestarted
			if tt.expectedRestart != isRestarted {
				t.Errorf("olt-reboot-failed expected= %v, got= %v", tt.expectedRestart, isRestarted)
			}
		})
	}
}

func Test_UpdateFlowsIncrementallyNegativeTestCases(t *testing.T) {
	dh1 := negativeDeviceHandlerNilFlowMgr()
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		wantErr       bool
	}{
		{"update-flow-when-device-handler-is-nil", dh1, true},
	}

	flowMetadata0 := of.FlowMetadata{Meters: []*of.OfpMeterConfig{
		{
			Flags:   5,
			MeterId: 1,
			Bands: []*of.OfpMeterBandHeader{
				{
					Type:      of.OfpMeterBandType_OFPMBT_DROP,
					Rate:      16000,
					BurstSize: 0,
				},
				{
					Type:      of.OfpMeterBandType_OFPMBT_DROP,
					Rate:      32000,
					BurstSize: 30,
				},
				{
					Type:      of.OfpMeterBandType_OFPMBT_DROP,
					Rate:      64000,
					BurstSize: 30,
				},
			},
		},
	}}

	kwTable0Meter1 := make(map[string]uint64)
	kwTable0Meter1["table_id"] = 0
	kwTable0Meter1["meter_id"] = 1
	kwTable0Meter1["write_metadata"] = 0x4000000000 // Tech-Profile-ID 64

	// Upstream flow DHCP flow - ONU1 UNI0 PON0
	fa0 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(536870912),
			fu.Metadata_ofp(1),
			fu.IpProto(17), // dhcp
			fu.VlanPcp(0),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT)),
			fu.TunnelId(256),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 257)),
			fu.Output(2147483645),
			fu.PushVlan(0x8100),
		},
		KV: kwTable0Meter1,
	}

	flow0, _ := fu.MkFlowStat(fa0)
	flowAdd := of.Flows{Items: make([]*of.OfpFlowStats, 0)}
	flowAdd.Items = append(flowAdd.Items, flow0)
	flowRemove := of.Flows{Items: make([]*of.OfpFlowStats, 0)}
	flowChanges := &ofp.FlowChanges{ToAdd: &flowAdd, ToRemove: &flowRemove}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.devicehandler.UpdateFlowsIncrementally(context.Background(), tt.devicehandler.device, flowChanges, nil, &flowMetadata0)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.populateDeviceInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
