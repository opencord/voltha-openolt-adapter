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
	"fmt"
	"testing"

	"github.com/opencord/voltha-go/common/log"
	tp "github.com/opencord/voltha-go/common/techprofile"
	"github.com/opencord/voltha-go/db/model"
	fu "github.com/opencord/voltha-go/rw_core/utils"
	"github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	"github.com/opencord/voltha-openolt-adapter/mocks"
	ofp "github.com/opencord/voltha-protos/go/openflow_13"
	"github.com/opencord/voltha-protos/go/openolt"
	openoltpb2 "github.com/opencord/voltha-protos/go/openolt"
	tp_pb "github.com/opencord/voltha-protos/go/tech_profile"
	"github.com/opencord/voltha-protos/go/voltha"
)

func init() {
	log.SetDefaultLogger(log.JSON, log.DebugLevel, nil)
}
func newMockResourceMgr() *resourcemanager.OpenOltResourceMgr {
	ranges := []*openolt.DeviceInfo_DeviceResourceRanges{
		{IntfIds: []uint32{0, 1, 2}}}

	deviceinfo := &openolt.DeviceInfo{Vendor: "openolt", Model: "openolt", HardwareVersion: "1.0", FirmwareVersion: "1.0",
		DeviceId: "olt", DeviceSerialNumber: "openolt", PonPorts: 3, Technology: "Default",
		OnuIdStart: 1, OnuIdEnd: 1, AllocIdStart: 1, AllocIdEnd: 1,
		GemportIdStart: 1, GemportIdEnd: 1, FlowIdStart: 1, FlowIdEnd: 1,
		Ranges: ranges,
	}
	rsrMgr := resourcemanager.NewResourceMgr("olt", "127.0.0.1:2379", "etcd", "olt", deviceinfo)
	return rsrMgr
}

func newMockFlowmgr() *OpenOltFlowMgr {
	rsrMgr := newMockResourceMgr()
	dh := newMockDeviceHandler()

	rsrMgr.KVStore = &model.Backend{}
	rsrMgr.KVStore.Client = &mocks.MockKVClient{}

	dh.resourceMgr = rsrMgr
	flwMgr := NewFlowManager(dh, rsrMgr)
	onuIds := make(map[onuIDKey]onuInfo)
	onuIds[onuIDKey{intfID: 1, onuID: 1}] = onuInfo{intfID: 1, onuID: 1, serialNumber: "1"}
	onuIds[onuIDKey{intfID: 2, onuID: 2}] = onuInfo{intfID: 2, onuID: 2, serialNumber: "2"}
	flwMgr.onuIds = onuIds

	onuSerialNumbers := make(map[string]onuInfo)
	onuSerialNumbers["1"] = onuInfo{intfID: 1, onuID: 1, serialNumber: "1"}
	onuSerialNumbers["2"] = onuInfo{intfID: 2, onuID: 1, serialNumber: "2"}
	flwMgr.onuSerialNumbers = onuSerialNumbers

	onuGemPortIds := make(map[gemPortKey]onuInfo)
	onuGemPortIds[gemPortKey{intfID: 1, gemPort: 1}] = onuInfo{intfID: 1, onuID: 1, serialNumber: "1"}
	onuGemPortIds[gemPortKey{intfID: 2, gemPort: 2}] = onuInfo{intfID: 2, onuID: 2, serialNumber: "2"}
	flwMgr.onuGemPortIds = onuGemPortIds

	packetInGemPort := make(map[packetInInfoKey]uint32)
	packetInGemPort[packetInInfoKey{intfID: 1, onuID: 1, logicalPort: 1}] = 1
	packetInGemPort[packetInInfoKey{intfID: 2, onuID: 2, logicalPort: 2}] = 2

	flwMgr.packetInGemPort = packetInGemPort
	tps := make([]*tp.TechProfileMgr, len(rsrMgr.ResourceMgrs))
	for key, val := range rsrMgr.ResourceMgrs {
		tps[key] = val.TechProfileMgr
	}
	flwMgr.techprofile = tps
	return flwMgr
}
func TestOpenOltFlowMgr_CreateSchedulerQueues(t *testing.T) {
	flowMgr := newMockFlowmgr()

	tprofile := &tp.TechProfile{Name: "tp1", SubscriberIdentifier: "subscriber1",
		ProfileType: "pt1", NumGemPorts: 1, NumTconts: 1, Version: 1,
		InstanceCtrl: tp.InstanceControl{Onu: "1", Uni: "1", MaxGemPayloadSize: "1"},
	}
	tprofile.UsScheduler.Direction = "UPSTREAM"
	tprofile.UsScheduler.AdditionalBw = "AdditionalBW_None"
	tprofile.UsScheduler.QSchedPolicy = "WRR"

	tprofile2 := tprofile
	tprofile2.DsScheduler.Direction = "DOWNSTREAM"
	tprofile2.DsScheduler.AdditionalBw = "AdditionalBW_None"
	tprofile2.DsScheduler.QSchedPolicy = "WRR"
	bands := make([]*ofp.OfpMeterBandHeader, 2)
	bands[0] = &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DROP, Rate: 1000, BurstSize: 5000, Data: &ofp.OfpMeterBandHeader_Drop{}}
	bands[1] = &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DROP, Rate: 2000, BurstSize: 5000, Data: &ofp.OfpMeterBandHeader_Drop{}}
	ofpMeterConfig := &ofp.OfpMeterConfig{Flags: 1, MeterId: 1, Bands: bands}
	flowmetadata := &voltha.FlowMetadata{
		Meters: []*ofp.OfpMeterConfig{ofpMeterConfig},
	}
	type args struct {
		Dir          tp_pb.Direction
		IntfID       uint32
		OnuID        uint32
		UniID        uint32
		UniPort      uint32
		TpInst       *tp.TechProfile
		MeterID      uint32
		flowMetadata *voltha.FlowMetadata
	}
	tests := []struct {
		name          string
		schedQueue    schedQueue
		wantErr       bool
	}{
		// TODO: Add test cases.
		{"CreateSchedulerQueues-1", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 1, flowmetadata}, false},
		{"CreateSchedulerQueues-2", schedQueue{tp_pb.Direction_DOWNSTREAM, 1,1,1, 65, 1, tprofile2, 1, flowmetadata}, false},
		{"CreateSchedulerQueues-3", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-4", schedQueue{tp_pb.Direction_DOWNSTREAM, 1,1,1,65, 1, tprofile2, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-5", schedQueue{tp_pb.Direction_UPSTREAM, 2, 2, 2, 64, 2, tprofile, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-6", schedQueue{tp_pb.Direction_DOWNSTREAM, 2,2,2, 65, 2, tprofile2, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-13",schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 1, flowmetadata}, false},
		//Negative testcases
		{"CreateSchedulerQueues-7", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 1, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-8", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 0 , &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-9", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 1, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-10",schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 2, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-11",schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 2,  &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-12",schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 2, nil}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := flowMgr.CreateSchedulerQueues(tt.schedQueue); (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.CreateSchedulerQueues() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltFlowMgr_RemoveSchedulerQueues(t *testing.T) {

	flowMgr := newMockFlowmgr()
	tprofile := &tp.TechProfile{Name: "tp1", SubscriberIdentifier: "subscriber1",
		ProfileType: "pt1", NumGemPorts: 1, NumTconts: 1, Version: 1,
		InstanceCtrl: tp.InstanceControl{Onu: "1", Uni: "1", MaxGemPayloadSize: "1"},
	}
	tprofile.UsScheduler.Direction = "UPSTREAM"
	tprofile.UsScheduler.AdditionalBw = "AdditionalBW_None"
	tprofile.UsScheduler.QSchedPolicy = "WRR"

	tprofile2 := tprofile
	tprofile2.DsScheduler.Direction = "DOWNSTREAM"
	tprofile2.DsScheduler.AdditionalBw = "AdditionalBW_None"
	tprofile2.DsScheduler.QSchedPolicy = "WRR"
	//defTprofile := &tp.DefaultTechProfile{}
	type args struct {
		Dir     tp_pb.Direction
		IntfID  uint32
		OnuID   uint32
		UniID   uint32
		UniPort uint32
		TpInst  *tp.TechProfile
	}
	tests := []struct {
		name          string
		schedQueue    schedQueue
		wantErr       bool
	}{
		// TODO: Add test cases.
		{"RemoveSchedulerQueues", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 0, nil}, false},
		{"RemoveSchedulerQueues", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 0, nil}, false},
		// negative test cases
		{"RemoveSchedulerQueues", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65,1, tprofile2, 0, nil}, false},
		{"RemoveSchedulerQueues", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 0, nil}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if err := flowMgr.RemoveSchedulerQueues(tt.schedQueue); (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.RemoveSchedulerQueues() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltFlowMgr_RemoveFlow(t *testing.T) {
	flowMgr := newMockFlowmgr()

	fa := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(2),
			fu.Metadata_ofp(2),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
		},
		Actions: []*ofp.OfpAction{
			fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 101)),
			fu.Output(1),
		},
	}
	ofpstats := fu.MkFlowStat(fa)
	type args struct {
		flow *ofp.OfpFlowStats
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"RemoveFlow", args{flow: ofpstats}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flowMgr.RemoveFlow(tt.args.flow)
		})
	}
}

func TestOpenOltFlowMgr_AddFlow(t *testing.T) {

	flowMgr := newMockFlowmgr()
	kw := make(map[string]uint64)
	kw["table_id"] = 1
	kw["meter_id"] = 1
	kw["write_metadata"] = 2
	fa := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(1000),
			fu.Metadata_ofp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 101)),
			fu.Output(65533),
		},
		KV: kw,
	}

	ofpstats := fu.MkFlowStat(fa)
	fmt.Println("ofpstats ", ofpstats)
	//ofpstats.Dat
	ofpMeterConfig := &ofp.OfpMeterConfig{Flags: 1, MeterId: 1}
	//ofpWritemetaData := &ofp.ofp
	flowMetadata := &voltha.FlowMetadata{
		Meters: []*ofp.OfpMeterConfig{ofpMeterConfig},
	}
	type args struct {
		flow         *ofp.OfpFlowStats
		flowMetadata *voltha.FlowMetadata
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"AddFlow", args{flow: ofpstats, flowMetadata: flowMetadata}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flowMgr.AddFlow(tt.args.flow, tt.args.flowMetadata)
		})
	}
}

func TestOpenOltFlowMgr_UpdateOnuInfo(t *testing.T) {
	flowMgr := newMockFlowmgr()
	type args struct {
		intfID    uint32
		onuID     uint32
		serialNum string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"UpdateOnuInfo", args{1, 1, "onu1"}},
		{"UpdateOnuInfo", args{2, 3, "onu1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			flowMgr.UpdateOnuInfo(tt.args.intfID, tt.args.onuID, tt.args.serialNum)
		})
	}
}

func TestOpenOltFlowMgr_GetLogicalPortFromPacketIn(t *testing.T) {
	flowMgr := newMockFlowmgr()
	type args struct {
		packetIn *openoltpb2.PacketIndication
	}
	tests := []struct {
		name    string
		args    args
		want    uint32
		wantErr bool
	}{
		// TODO: Add test cases.
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "pon", IntfId: 1, GemportId: 1, FlowId: 100, PortNo: 1, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 1, false},
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "nni", IntfId: 1, GemportId: 1, FlowId: 100, PortNo: 1, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 65537, false},
		// Negative Test cases.
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "pon", IntfId: 2, GemportId: 1, FlowId: 100, PortNo: 1, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 0, true},
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "pon", IntfId: 1, GemportId: 1, FlowId: 100, PortNo: 0, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 2064, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := flowMgr.GetLogicalPortFromPacketIn(tt.args.packetIn)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.GetLogicalPortFromPacketIn() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("OpenOltFlowMgr.GetLogicalPortFromPacketIn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltFlowMgr_GetPacketOutGemPortID(t *testing.T) {
	flwMgr := newMockFlowmgr()

	type args struct {
		intfID  uint32
		onuID   uint32
		portNum uint32
	}
	tests := []struct {
		name    string
		args    args
		want    uint32
		wantErr bool
	}{
		// TODO: Add test cases.
		{"GetPacketOutGemPortID", args{intfID: 1, onuID: 1, portNum: 1}, 1, false},
		{"GetPacketOutGemPortID", args{intfID: 2, onuID: 2, portNum: 2}, 2, false},
		{"GetPacketOutGemPortID", args{intfID: 1, onuID: 2, portNum: 2}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := flwMgr.GetPacketOutGemPortID(tt.args.intfID, tt.args.onuID, tt.args.portNum)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.GetPacketOutGemPortID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("OpenOltFlowMgr.GetPacketOutGemPortID() = %v, want %v", got, tt.want)
			}

		})
	}
}

func TestOpenOltFlowMgr_DeleteTechProfileInstance(t *testing.T) {
	flwMgr := newMockFlowmgr()
	type args struct {
		intfID uint32
		onuID  uint32
		uniID  uint32
		sn     string
		tpID   uint32
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{"DeleteTechProfileInstance", args{intfID: 0, onuID: 1, uniID: 1, sn: "", tpID: 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if err := flwMgr.DeleteTechProfileInstance(tt.args.intfID, tt.args.onuID, tt.args.uniID, tt.args.sn, tt.args.tpID); (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.DeleteTechProfileInstance() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
