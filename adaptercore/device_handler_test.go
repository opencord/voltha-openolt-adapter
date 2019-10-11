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

//Package adaptercore provides the utility for olt devices, flows and statistics
package adaptercore

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/opencord/voltha-lib-go/pkg/pmmetrics"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	fu "github.com/opencord/voltha-lib-go/v2/pkg/flows"
	"github.com/opencord/voltha-lib-go/v2/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	"github.com/opencord/voltha-openolt-adapter/mocks"
	ic "github.com/opencord/voltha-protos/go/inter_container"
	of "github.com/opencord/voltha-protos/go/openflow_13"
	ofp "github.com/opencord/voltha-protos/go/openflow_13"
	oop "github.com/opencord/voltha-protos/go/openolt"
	"github.com/opencord/voltha-protos/go/voltha"
)

func init() {
	_, _ = log.AddPackage(log.JSON, log.DebugLevel, nil)
}

func newMockCoreProxy() *mocks.MockCoreProxy {
	mcp := mocks.MockCoreProxy{}
	mcp.Devices = make(map[string]*voltha.Device)
	var pm []*voltha.PmConfig
	mcp.Devices["olt"] = &voltha.Device{

		Id:           "olt",
		Root:         true,
		ParentId:     "logical_device",
		ParentPortNo: 1,

		Ports: []*voltha.Port{
			{PortNo: 1, Label: "pon"},
			{PortNo: 2, Label: "nni"},
		},
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
	mcp.Devices["onu1"] = &voltha.Device{

		Id:           "1",
		Root:         false,
		ParentId:     "olt",
		ParentPortNo: 1,

		Ports: []*voltha.Port{
			{PortNo: 1, Label: "pon"},
			{PortNo: 2, Label: "uni"},
		},
		OperStatus: 4,
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
	mcp.Devices["onu2"] = &voltha.Device{
		Id:         "2",
		Root:       false,
		ParentId:   "olt",
		OperStatus: 2,
		Ports: []*voltha.Port{
			{PortNo: 1, Label: "pon"},
			{PortNo: 2, Label: "uni"},
		},

		ParentPortNo: 1,

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
	return &mcp
}
func newMockDeviceHandler() *DeviceHandler {
	device := &voltha.Device{
		Id:       "olt",
		Root:     true,
		ParentId: "logical_device",
		Ports: []*voltha.Port{
			{PortNo: 1, Label: "pon"},
			{PortNo: 2, Label: "nni"},
		},
		ProxyAddress: &voltha.Device_ProxyAddress{
			DeviceId:       "olt",
			DeviceType:     "onu",
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
	}
	cp := newMockCoreProxy()
	ap := &mocks.MockAdapterProxy{}
	ep := &mocks.MockEventProxy{}
	openOLT := &OpenOLT{coreProxy: cp, adapterProxy: ap, eventProxy: ep}
	dh := NewDeviceHandler(cp, ap, ep, device, openOLT)
	dh.nniIntfID = 1
	deviceInf := &oop.DeviceInfo{Vendor: "openolt", Ranges: nil, Model: "openolt", DeviceId: dh.deviceID}
	dh.resourceMgr = &resourcemanager.OpenOltResourceMgr{DeviceID: dh.deviceID, DeviceType: dh.deviceType, DevInfo: deviceInf}
	dh.flowMgr = NewFlowManager(dh, dh.resourceMgr)
	dh.Client = &mocks.MockOpenoltClient{}
	dh.eventMgr = &OpenOltEventMgr{eventProxy: &mocks.MockEventProxy{}}
	dh.transitionMap = &TransitionMap{}
	dh.portStats = &OpenOltStatisticsMgr{}
	dh.metrics = &pmmetrics.PmMetrics{}
	return dh
}

func negativeDeviceHandler() *DeviceHandler {
	dh := newMockDeviceHandler()
	device := dh.device
	device.Id = ""
	dh.adminState = "down"
	return dh
}
func Test_generateMacFromHost(t *testing.T) {
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
			got, err := generateMacFromHost(tt.args.host)
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

func TestDeviceHandler_GetChildDevice(t *testing.T) {
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
	}{
		{"GetChildDevice-1", dh1,
			args{parentPort: 1,
				onuID: 1},
			&voltha.Device{},
		},
		{"GetChildDevice-2", dh2,
			args{parentPort: 1,
				onuID: 1},
			&voltha.Device{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.devicehandler.GetChildDevice(tt.args.parentPort, tt.args.onuID)
			t.Log("onu device id", got)
		})
	}
}

func TestGetportLabel(t *testing.T) {
	type args struct {
		portNum  uint32
		portType voltha.Port_PortType
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"GetportLabel-1", args{portNum: 0, portType: 0}, ""},
		{"GetportLabel-2", args{portNum: 1, portType: 1}, "nni-1"},
		{"GetportLabel-3", args{portNum: 2, portType: 2}, ""},
		{"GetportLabel-4", args{portNum: 3, portType: 3}, "pon-3"},
		{"GetportLabel-5", args{portNum: 4, portType: 4}, ""},
		{"GetportLabel-6", args{portNum: 5, portType: 5}, ""},
		{"GetportLabel-7", args{portNum: 6, portType: 6}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetportLabel(tt.args.portNum, tt.args.portType); got != tt.want {
				t.Errorf("GetportLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceHandler_ProcessInterAdapterMessage(t *testing.T) {
	dh := newMockDeviceHandler()
	proxyAddr := dh.device.ProxyAddress
	body := &ic.InterAdapterOmciMessage{
		Message:      []byte("asdfasdfasdfasdfas"),
		ProxyAddress: proxyAddr,
	}
	body2 := &ic.InterAdapterOmciMessage{
		Message: []byte("asdfasdfasdfasdfas"),
		//ProxyAddress: &voltha.Device_ProxyAddress{},
	}
	body3 := &ic.InterAdapterTechProfileDownloadMessage{}
	var marshalledData *any.Any
	var err error

	if marshalledData, err = ptypes.MarshalAny(body); err != nil {
		log.Errorw("cannot-marshal-request", log.Fields{"error": err})
	}

	var marshalledData1 *any.Any

	if marshalledData1, err = ptypes.MarshalAny(body2); err != nil {
		log.Errorw("cannot-marshal-request", log.Fields{"error": err})
	}
	var marshalledData2 *any.Any

	if marshalledData2, err = ptypes.MarshalAny(body3); err != nil {
		log.Errorw("cannot-marshal-request", log.Fields{"error": err})
	}
	type args struct {
		msg *ic.InterAdapterMessage
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"ProcessInterAdapterMessage-1", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 0,
			},
			Body: marshalledData,
		}}, false},
		{"ProcessInterAdapterMessage-2", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 1,
			},
			Body: marshalledData1,
		}}, false},
		{"ProcessInterAdapterMessage-3", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 2,
			},
			Body: marshalledData,
		}}, false},
		{"ProcessInterAdapterMessage-4", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 3,
			}, Body: marshalledData,
		}}, false},
		{"ProcessInterAdapterMessage-5", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 4,
			}, Body: marshalledData1,
		}}, false},
		{"ProcessInterAdapterMessage-6", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 4,
			}, Body: marshalledData,
		}}, false},
		{"ProcessInterAdapterMessage-7", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 5,
			}, Body: marshalledData,
		}}, false},
		{"ProcessInterAdapterMessage-8", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 6,
			}, Body: marshalledData,
		}}, false},
		{"ProcessInterAdapterMessage-9", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 7,
			}, Body: marshalledData,
		}}, false},
		{"ProcessInterAdapterMessage-10", args{msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:   "012345",
				Type: 7,
			}, Body: marshalledData2,
		}}, false},
		//marshalledData2
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if err := dh.ProcessInterAdapterMessage(tt.args.msg); (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.ProcessInterAdapterMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeviceHandler_sendProxiedMessage(t *testing.T) {
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
	iaomciMsg1 := &ic.InterAdapterOmciMessage{
		ProxyAddress: &voltha.Device_ProxyAddress{
			DeviceId:       "onu2",
			DeviceType:     "onu",
			ChannelId:      1,
			ChannelGroupId: 1,
			//OnuId:          2,
		},
		ConnectStatus: 1,
	}
	iaomciMsg2 := &ic.InterAdapterOmciMessage{
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
		omciMsg   *ic.InterAdapterOmciMessage
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
	}{
		{"sendProxiedMessage-1", dh1, args{onuDevice: device1, omciMsg: &ic.InterAdapterOmciMessage{}}},
		{"sendProxiedMessage-2", dh1, args{onuDevice: device2, omciMsg: &ic.InterAdapterOmciMessage{}}},
		{"sendProxiedMessage-3", dh1, args{onuDevice: nil, omciMsg: iaomciMsg1}},
		{"sendProxiedMessage-4", dh1, args{onuDevice: nil, omciMsg: iaomciMsg2}},
		{"sendProxiedMessage-5", dh2, args{onuDevice: nil, omciMsg: iaomciMsg2}},
		{"sendProxiedMessage-6", dh2, args{onuDevice: device1, omciMsg: &ic.InterAdapterOmciMessage{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.devicehandler.sendProxiedMessage(tt.args.onuDevice, tt.args.omciMsg)
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
			tt.devicehandler.SendPacketInToCore(tt.args.logicalPort, tt.args.packetPayload)
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
		{"DisableDevice-2", dh1, args{device: dh1.device}, true},
		{"DisableDevice-3", dh2, args{device: dh2.device}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if err := tt.devicehandler.DisableDevice(tt.args.device); (err != nil) != tt.wantErr {
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
			if err := dh.ReenableDevice(tt.args.device); (err != nil) != tt.wantErr {
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

			if err := tt.devicehandler.RebootDevice(tt.args.device); (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.RebootDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeviceHandler_handleIndication(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	dh3 := newMockDeviceHandler()
	dh3.onus = map[string]*OnuDevice{"onu1": NewOnuDevice("onu1", "onu1", "onu1", 1, 1, "onu1"),
		"onu2": NewOnuDevice("onu2", "onu2", "onu2", 2, 2, "onu2")}
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
			dh.handleIndication(tt.args.indication)
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
			tt.devicehandler.addPort(tt.args.intfID, tt.args.portType, tt.args.state)
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
			dh.handleOltIndication(tt.args.oltIndication)
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
			tt.devicehandler.postInit()
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
		{"activateONU-1", dh, args{intfID: 1, onuID: 1, serialNum: &oop.SerialNumber{VendorId: []byte("onu1")}}},
		{"activateONU-2", dh, args{intfID: 2, onuID: 2, serialNum: &oop.SerialNumber{VendorId: []byte("onu2")}}},
		{"activateONU-3", dh1, args{intfID: 1, onuID: 1, serialNum: &oop.SerialNumber{VendorId: []byte("onu1")}}},
		{"activateONU-4", dh1, args{intfID: 2, onuID: 2, serialNum: &oop.SerialNumber{VendorId: []byte("onu2")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tt.devicehandler.activateONU(tt.args.intfID, tt.args.onuID,
				tt.args.serialNum, tt.args.serialNumber)
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
		{"PacketOut-2", dh2, args{egressPortNo: 115000, packet: pktout}, false},
		{"PacketOut-3", dh1, args{egressPortNo: 65536, packet: pktout}, false},
		{"PacketOut-4", dh2, args{egressPortNo: 65535, packet: pktout}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dh := tt.devicehandler
			if err := dh.PacketOut(tt.args.egressPortNo, tt.args.packet); (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.PacketOut() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

//
func TestDeviceHandler_doStateUp(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := newMockDeviceHandler()

	dh2.deviceID = ""
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
			if err := tt.devicehandler.doStateUp(); (err != nil) != tt.wantErr {
				t.Logf("DeviceHandler.doStateUp() error = %v, wantErr %v", err, tt.wantErr)
			}
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
			if err := tt.devicehandler.doStateDown(); (err != nil) != tt.wantErr {
				t.Logf("DeviceHandler.doStateDown() error = %v", err)
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

func TestDeviceHandler_GetOfpPortInfo(t *testing.T) {
	dh1 := newMockDeviceHandler()
	dh2 := negativeDeviceHandler()
	type args struct {
		device *voltha.Device
		portNo int64
	}
	tests := []struct {
		name          string
		devicehandler *DeviceHandler
		args          args
		wantErr       bool
	}{
		{"GetOfpPortInfo-1", dh1, args{device: dh1.device, portNo: 1}, false},
		{"GetOfpPortInfo-2", dh2, args{device: dh2.device, portNo: 1}, false},
		{"GetOfpPortInfo-3", dh1, args{device: dh1.device, portNo: 0}, false},
		{"GetOfpPortInfo-4", dh2, args{device: dh2.device, portNo: 0}, false},
		{"GetOfpPortInfo-5", dh1, args{device: &voltha.Device{}, portNo: 1}, false},
		{"GetOfpPortInfo-6", dh2, args{device: &voltha.Device{}, portNo: 0}, false},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dh := tt.devicehandler
			_, err := dh.GetOfpPortInfo(tt.args.device, tt.args.portNo)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeviceHandler.GetOfpPortInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDeviceHandler_onuDiscIndication(t *testing.T) {

	dh1 := newMockDeviceHandler()
	dh1.discOnus = map[string]bool{"onu1": true, "onu2": false}
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
		{"onuDiscIndication-7", dh2, args{onuDiscInd: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.devicehandler.onuDiscIndication(tt.args.onuDiscInd, tt.args.sn)
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

			_, err := tt.devicehandler.populateDeviceInfo()
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
	dh2.adminState = "down"
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
			tt.devicehandler.readIndications()
		})
	}
}

func Test_startCollector(t *testing.T) {
	type args struct {
		dh *DeviceHandler
	}
	dh := newMockDeviceHandler()
	dh.portStats.NorthBoundPort = make(map[uint32]*NniPort)
	dh.portStats.NorthBoundPort[0] = &NniPort{Name: "OLT-1"}
	dh.portStats.SouthBoundPort = make(map[uint32]*PonPort)
	dh.portStats.Device = dh
	for i := 0; i < 16; i++ {
		dh.portStats.SouthBoundPort[uint32(i)] = &PonPort{DeviceID: "OLT-1"}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"StartCollector-1", args{dh}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			go func() {
				time.Sleep(2 * time.Minute)
				tt.args.dh.stopCollector <- true
			}()
			startCollector(tt.args.dh)
		})
	}
}
