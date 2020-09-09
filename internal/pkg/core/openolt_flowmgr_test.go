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
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/opencord/voltha-protos/v3/go/voltha"

	"github.com/opencord/voltha-lib-go/v3/pkg/db"
	fu "github.com/opencord/voltha-lib-go/v3/pkg/flows"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	tp "github.com/opencord/voltha-lib-go/v3/pkg/techprofile"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"
	"github.com/opencord/voltha-openolt-adapter/pkg/mocks"
	ofp "github.com/opencord/voltha-protos/v3/go/openflow_13"
	"github.com/opencord/voltha-protos/v3/go/openolt"
	openoltpb2 "github.com/opencord/voltha-protos/v3/go/openolt"
	tp_pb "github.com/opencord/voltha-protos/v3/go/tech_profile"
)

var flowMgr []*OpenOltFlowMgr

func init() {
	_, _ = log.SetDefaultLogger(log.JSON, log.DebugLevel, nil)
	flowMgr = newMockFlowmgr()
}
func newMockResourceMgr() *resourcemanager.OpenOltResourceMgr {
	ranges := []*openolt.DeviceInfo_DeviceResourceRanges{
		{
			IntfIds:    []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			Technology: "Default",
		},
	}

	deviceinfo := &openolt.DeviceInfo{Vendor: "openolt", Model: "openolt", HardwareVersion: "1.0", FirmwareVersion: "1.0",
		DeviceId: "olt", DeviceSerialNumber: "openolt", PonPorts: 16, Technology: "Default",
		OnuIdStart: 1, OnuIdEnd: 1, AllocIdStart: 1, AllocIdEnd: 1,
		GemportIdStart: 1, GemportIdEnd: 1, FlowIdStart: 1, FlowIdEnd: 1,
		Ranges: ranges,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rsrMgr := resourcemanager.NewResourceMgr(ctx, "olt", "127.0.0.1:2379", "etcd", "olt", deviceinfo)
	for key := range rsrMgr.ResourceMgrs {
		rsrMgr.ResourceMgrs[key].KVStore = &db.Backend{}
		rsrMgr.ResourceMgrs[key].KVStore.Client = &mocks.MockKVClient{}
		rsrMgr.ResourceMgrs[key].TechProfileMgr = mocks.MockTechProfile{TpID: key}
	}
	return rsrMgr
}

func newMockFlowmgr() []*OpenOltFlowMgr {
	rMgr := newMockResourceMgr()
	dh := newMockDeviceHandler()

	rMgr.KVStore = &db.Backend{}
	rMgr.KVStore.Client = &mocks.MockKVClient{}

	dh.resourceMgr = rMgr

	// onuGemInfo := make([]rsrcMgr.OnuGemInfo, NumPonPorts)
	var i uint32

	for i = 0; i < NumPonPorts; i++ {
		packetInGemPort := make(map[rsrcMgr.PacketInInfoKey]uint32)
		packetInGemPort[rsrcMgr.PacketInInfoKey{IntfID: i, OnuID: i+1, LogicalPort: i+1, VlanID:uint16(i), Priority:uint8(i)}] = i+1

		dh.flowMgr[i].packetInGemPort = packetInGemPort
		tps := make(map[uint32]tp.TechProfileIf)
		for key := range rMgr.ResourceMgrs {
			tps[key] = mocks.MockTechProfile{TpID: key}
		}
		dh.flowMgr[i].techprofile = tps
		interface2mcastQeueuMap := make(map[uint32]*queueInfoBrief)
		interface2mcastQeueuMap[0] = &queueInfoBrief{
			gemPortID:       4000,
			servicePriority: 3,
		}
		dh.flowMgr[i].grpMgr.interfaceToMcastQueueMap = interface2mcastQeueuMap
	}

	return dh.flowMgr
}

func TestOpenOltFlowMgr_CreateSchedulerQueues(t *testing.T) {
	tprofile := &tp.TechProfile{Name: "tp1", SubscriberIdentifier: "subscriber1",
		ProfileType: "pt1", NumGemPorts: 1, Version: 1,
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
	tests := []struct {
		name       string
		schedQueue schedQueue
		wantErr    bool
	}{
		// TODO: Add test cases.
		{"CreateSchedulerQueues-1", schedQueue{tp_pb.Direction_UPSTREAM, 0, 1, 1, 64, 1, tprofile, 1, flowmetadata}, false},
		{"CreateSchedulerQueues-2", schedQueue{tp_pb.Direction_DOWNSTREAM, 0, 1, 1, 65, 1, tprofile2, 1, flowmetadata}, false},
		{"CreateSchedulerQueues-3", schedQueue{tp_pb.Direction_UPSTREAM, 0, 1, 1, 64, 1, tprofile, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-4", schedQueue{tp_pb.Direction_DOWNSTREAM, 0, 1, 1, 65, 1, tprofile2, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-5", schedQueue{tp_pb.Direction_UPSTREAM, 1, 2, 2, 64, 2, tprofile, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-6", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 2, 2, 65, 2, tprofile2, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-13", schedQueue{tp_pb.Direction_DOWNSTREAM, 0, 1, 1, 65, 1, tprofile2, 1, flowmetadata}, false},
		//Negative testcases
		{"CreateSchedulerQueues-7", schedQueue{tp_pb.Direction_UPSTREAM, 0, 1, 1, 64, 1, tprofile, 1, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-8", schedQueue{tp_pb.Direction_UPSTREAM, 0, 1, 1, 64, 1, tprofile, 0, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-9", schedQueue{tp_pb.Direction_DOWNSTREAM, 0, 1, 1, 65, 1, tprofile2, 1, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-10", schedQueue{tp_pb.Direction_UPSTREAM, 0, 1, 1, 64, 1, tprofile, 2, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-11", schedQueue{tp_pb.Direction_DOWNSTREAM, 0, 1, 1, 65, 1, tprofile2, 2, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-12", schedQueue{tp_pb.Direction_DOWNSTREAM, 0, 1, 1, 65, 1, tprofile2, 2, nil}, true},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := flowMgr[tt.schedQueue.intfID].CreateSchedulerQueues(ctx, tt.schedQueue); (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.CreateSchedulerQueues() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltFlowMgr_RemoveSchedulerQueues(t *testing.T) {
	tprofile := &tp.TechProfile{Name: "tp1", SubscriberIdentifier: "subscriber1",
		ProfileType: "pt1", NumGemPorts: 1, Version: 1,
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
	tests := []struct {
		name       string
		schedQueue schedQueue
		wantErr    bool
	}{
		// TODO: Add test cases.
		{"RemoveSchedulerQueues", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 0, nil}, false},
		{"RemoveSchedulerQueues", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 0, nil}, false},
		// negative test cases
		{"RemoveSchedulerQueues", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 0, nil}, false},
		{"RemoveSchedulerQueues", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 0, nil}, false},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := flowMgr[tt.schedQueue.intfID].RemoveSchedulerQueues(ctx, tt.schedQueue); (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.RemoveSchedulerQueues() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

}

func TestOpenOltFlowMgr_createTcontGemports(t *testing.T) {
	bands := make([]*ofp.OfpMeterBandHeader, 2)
	bands[0] = &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DROP, Rate: 1000, BurstSize: 5000, Data: &ofp.OfpMeterBandHeader_Drop{}}
	bands[1] = &ofp.OfpMeterBandHeader{Type: ofp.OfpMeterBandType_OFPMBT_DROP, Rate: 2000, BurstSize: 5000, Data: &ofp.OfpMeterBandHeader_Drop{}}
	ofpMeterConfig := &ofp.OfpMeterConfig{Flags: 1, MeterId: 1, Bands: bands}
	flowmetadata := &voltha.FlowMetadata{
		Meters: []*ofp.OfpMeterConfig{ofpMeterConfig},
	}
	type args struct {
		intfID       uint32
		onuID        uint32
		uniID        uint32
		uni          string
		uniPort      uint32
		TpID         uint32
		UsMeterID    uint32
		DsMeterID    uint32
		flowMetadata *voltha.FlowMetadata
	}
	tests := []struct {
		name string
		args args
	}{
		{"createTcontGemports-1", args{intfID: 0, onuID: 1, uniID: 1, uni: "16", uniPort: 1, TpID: 64, UsMeterID: 1, DsMeterID: 1, flowMetadata: flowmetadata}},
		{"createTcontGemports-1", args{intfID: 0, onuID: 1, uniID: 1, uni: "16", uniPort: 1, TpID: 65, UsMeterID: 1, DsMeterID: 1, flowMetadata: flowmetadata}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, tpInst := flowMgr[tt.args.intfID].createTcontGemports(ctx, tt.args.intfID, tt.args.onuID, tt.args.uniID, tt.args.uni, tt.args.uniPort, tt.args.TpID, tt.args.UsMeterID, tt.args.DsMeterID, tt.args.flowMetadata)
			switch tpInst := tpInst.(type) {
			case *tp.TechProfile:
				if tt.args.TpID != 64 {
					t.Errorf("OpenOltFlowMgr.createTcontGemports() error = different tech, tech %v", tpInst)
				}
			case *tp.EponProfile:
				if tt.args.TpID != 65 {
					t.Errorf("OpenOltFlowMgr.createTcontGemports() error = different tech, tech %v", tpInst)
				}
			default:
				t.Errorf("OpenOltFlowMgr.createTcontGemports() error = different tech, tech %v", tpInst)
			}
		})
	}
}

func TestOpenOltFlowMgr_RemoveFlow(t *testing.T) {
	ctx := context.Background()
	logger.Debug(ctx, "Info Warning Error: Starting RemoveFlow() test")
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
	ofpstats, _ := fu.MkFlowStat(fa)
	ofpstats.Cookie = ofpstats.Id
	lldpFa := &fu.FlowArgs{
		KV: fu.OfpFlowModArgs{"priority": 1000, "cookie": 48132224281636694},
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(1),
			fu.EthType(0x88CC),
			fu.TunnelId(536870912),
		},
		Actions: []*ofp.OfpAction{
			fu.Output(uint32(ofp.OfpPortNo_OFPP_CONTROLLER)),
		},
	}
	lldpofpstats, _ := fu.MkFlowStat(lldpFa)
	//lldpofpstats.Cookie = lldpofpstats.Id

	dhcpFa := &fu.FlowArgs{
		KV: fu.OfpFlowModArgs{"priority": 1000, "cookie": 48132224281636694},
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(1),
			fu.UdpSrc(67),
			//fu.TunnelId(536870912),
			fu.IpProto(17),
		},
		Actions: []*ofp.OfpAction{
			fu.Output(uint32(ofp.OfpPortNo_OFPP_CONTROLLER)),
		},
	}
	dhcpofpstats, _ := fu.MkFlowStat(dhcpFa)
	//dhcpofpstats.Cookie = dhcpofpstats.Id

	//multicast flow
	multicastFa := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(65536),
			fu.VlanVid(660),             //vlan
			fu.Metadata_ofp(uint64(66)), //inner vlan
			fu.EthType(0x800),           //ipv4
			fu.Ipv4Dst(3809869825),      //227.22.0.1
		},
		Actions: []*ofp.OfpAction{
			fu.Group(1),
		},
	}
	multicastOfpStats, _ := fu.MkFlowStat(multicastFa)
	multicastOfpStats.Id = 1

	type args struct {
		flow *ofp.OfpFlowStats
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"RemoveFlow", args{flow: ofpstats}},
		{"RemoveFlow", args{flow: lldpofpstats}},
		{"RemoveFlow", args{flow: dhcpofpstats}},
		{"RemoveFlow", args{flow: multicastOfpStats}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := flowMgr[0].RemoveFlow(ctx, tt.args.flow); err != nil {
				logger.Warn(ctx, err)
			}
		})
	}
	// t.Error("=====")
}

func TestOpenOltFlowMgr_AddFlow(t *testing.T) {
	kw := make(map[string]uint64)
	kw["table_id"] = 1
	kw["meter_id"] = 1
	kw["write_metadata"] = 0x4000000000 // Tech-Profile-ID 64

	// Upstream flow
	fa := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(536870912),
			fu.Metadata_ofp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 257)),
			fu.Output(65536),
			fu.PushVlan(0x8100),
		},
		KV: kw,
	}

	// Downstream flow
	fa3 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(65536),
			fu.Metadata_ofp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 257),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			//fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 101)),
			fu.PopVlan(),
			fu.Output(536870912),
		},
		KV: kw,
	}

	fa2 := &fu.FlowArgs{
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

	// TODO Add LLDP flow
	// TODO Add DHCP flow

	// Flows for negative scenarios
	// Failure in formulateActionInfoFromFlow()
	fa4 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(1000),
			fu.Metadata_ofp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
		},
		Actions: []*ofp.OfpAction{
			fu.Experimenter(257, []byte{1, 2, 3, 4}),
		},
		KV: kw,
	}

	// Invalid Output
	fa5 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(1000),
			fu.Metadata_ofp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
		},
		Actions: []*ofp.OfpAction{
			fu.Output(0),
		},
		KV: kw,
	}

	// Tech-Profile-ID update (not supported)
	kw6 := make(map[string]uint64)
	kw6["table_id"] = 1
	kw6["meter_id"] = 1
	kw6["write_metadata"] = 0x4100000000 // TpID Other than the stored one
	fa6 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(536870912),
			fu.TunnelId(16),
			fu.Metadata_ofp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 257)),
			fu.Output(65535),
		},
		KV: kw6,
	}

	lldpFa := &fu.FlowArgs{
		KV: fu.OfpFlowModArgs{"priority": 1000, "cookie": 48132224281636694},
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(1),
			fu.EthType(0x88CC),
			fu.TunnelId(536870912),
		},
		Actions: []*ofp.OfpAction{
			fu.Output(uint32(ofp.OfpPortNo_OFPP_CONTROLLER)),
		},
	}

	dhcpFa := &fu.FlowArgs{
		KV: fu.OfpFlowModArgs{"priority": 1000, "cookie": 48132224281636694},
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(1),
			fu.UdpSrc(67),
			//fu.TunnelId(536870912),
			fu.IpProto(17),
		},
		Actions: []*ofp.OfpAction{
			fu.Output(uint32(ofp.OfpPortNo_OFPP_CONTROLLER)),
		},
	}
	igmpFa := &fu.FlowArgs{
		KV: fu.OfpFlowModArgs{"priority": 1000, "cookie": 48132224281636694},
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(1),
			fu.UdpSrc(67),
			//fu.TunnelId(536870912),
			fu.IpProto(2),
		},
		Actions: []*ofp.OfpAction{
			fu.Output(uint32(ofp.OfpPortNo_OFPP_CONTROLLER)),
		},
	}

	fa9 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(536870912),
			fu.TunnelId(16),
			fu.Metadata_ofp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
			fu.VlanPcp(1000),
			fu.UdpDst(65535),
			fu.UdpSrc(536870912),
			fu.Ipv4Dst(65535),
			fu.Ipv4Src(536870912),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 257)),
			fu.Output(65535),
		},
		KV: kw6,
	}

	fa10 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(65533),
			//	fu.TunnelId(16),
			fu.Metadata_ofp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
			fu.VlanPcp(1000),
			fu.UdpDst(65535),
			fu.UdpSrc(536870912),
			fu.Ipv4Dst(65535),
			fu.Ipv4Src(536870912),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 257)),
			fu.Output(65535),
		},
		KV: kw6,
	}
	//multicast flow
	fa11 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(65536),
			fu.VlanVid(660),             //vlan
			fu.Metadata_ofp(uint64(66)), //inner vlan
			fu.EthType(0x800),           //ipv4
			fu.Ipv4Dst(3809869825),      //227.22.0.1
		},
		Actions: []*ofp.OfpAction{
			fu.Group(1),
		},
		KV: kw6,
	}
	ofpstats, _ := fu.MkFlowStat(fa)
	ofpstats2, _ := fu.MkFlowStat(fa2)
	ofpstats3, _ := fu.MkFlowStat(fa3)
	ofpstats4, _ := fu.MkFlowStat(fa4)
	ofpstats5, _ := fu.MkFlowStat(fa5)
	ofpstats6, _ := fu.MkFlowStat(fa6)
	ofpstats7, _ := fu.MkFlowStat(lldpFa)
	ofpstats8, _ := fu.MkFlowStat(dhcpFa)
	ofpstats9, _ := fu.MkFlowStat(fa9)
	ofpstats10, _ := fu.MkFlowStat(fa10)
	igmpstats, _ := fu.MkFlowStat(igmpFa)
	ofpstats11, _ := fu.MkFlowStat(fa11)

	fmt.Println(ofpstats6, ofpstats9, ofpstats10)

	ofpMeterConfig := &ofp.OfpMeterConfig{Flags: 1, MeterId: 1}
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
		{"AddFlow", args{flow: ofpstats2, flowMetadata: flowMetadata}},
		{"AddFlow", args{flow: ofpstats3, flowMetadata: flowMetadata}},
		{"AddFlow", args{flow: ofpstats4, flowMetadata: flowMetadata}},
		{"AddFlow", args{flow: ofpstats5, flowMetadata: flowMetadata}},
		//{"AddFlow", args{flow: ofpstats6, flowMetadata: flowMetadata}},
		{"AddFlow", args{flow: ofpstats7, flowMetadata: flowMetadata}},
		{"AddFlow", args{flow: ofpstats8, flowMetadata: flowMetadata}},
		//{"AddFlow", args{flow: ofpstats9, flowMetadata: flowMetadata}},
		{"AddFlow", args{flow: igmpstats, flowMetadata: flowMetadata}},
		//{"AddFlow", args{flow: ofpstats10, flowMetadata: flowMetadata}},
		//ofpstats10
		{"AddFlow", args{flow: ofpstats11, flowMetadata: flowMetadata}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = flowMgr[0].AddFlow(ctx, tt.args.flow, tt.args.flowMetadata)
			// TODO: actually verify test cases
		})
	}
}

func TestOpenOltFlowMgr_UpdateOnuInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wg := sync.WaitGroup{}

	intfCount := NumPonPorts
	onuCount := OnuIdEnd - OnuIdStart + 1

	for i := 0; i < intfCount; i++ {
		for j := 1; j <= onuCount; j++ {
			wg.Add(1)
			go func(i uint32, j uint32) {
				// TODO: actually verify success
				_ = flowMgr[i].UpdateOnuInfo(ctx, i, i, fmt.Sprintf("onu-%d", i))
				wg.Done()
			}(uint32(i), uint32(j))
		}

	}

	wg.Wait()
}

func TestOpenOltFlowMgr_addGemPortToOnuInfoMap(t *testing.T) {
	intfNum := NumPonPorts
		onuNum := OnuIdEnd - OnuIdStart + 1

	// clean the flowMgr
	for i := 0; i < intfNum; i++ {
		flowMgr[i].onuGemInfo = make(map[uint32][]rsrcMgr.OnuGemInfo, intfNum)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create OnuInfo
	for i := 0; i < intfNum; i++ {
		for o := 1; o <= onuNum; o++ {
			// TODO: actually verify success
			_ = flowMgr[i].UpdateOnuInfo(ctx, uint32(i), uint32(o), fmt.Sprintf("i%do%d", i, o-1))
		}
	}

	// Add gemPorts to OnuInfo in parallel threads
	wg := sync.WaitGroup{}

	for o := 1; o <= onuNum; o++ {
		for i := 0; i < intfNum; i++ {
			wg.Add(1)
			go func(intfId uint32, onuId uint32) {
				gemID, _ := strconv.Atoi(fmt.Sprintf("90%d%d", intfId, onuId-1))

				flowMgr[intfId].addGemPortToOnuInfoMap(ctx, intfId, onuId, uint32(gemID))
				wg.Done()
			}(uint32(i), uint32(o))
		}
	}

	wg.Wait()

	// check that each entry of onuGemInfo has the correct number of ONUs
	for i := 0; i < intfNum; i++ {
		lenofOnu := len(flowMgr[i].onuGemInfo[uint32(i)])
		if onuNum != lenofOnu {
			t.Errorf("OnuGemInfo length is not as expected len = %d, want %d", lenofOnu, onuNum)
		}

		for o := 1; o <= onuNum; o++ {
			lenOfGemPorts := len(flowMgr[i].onuGemInfo[uint32(i)][o-1].GemPorts)
			// check that each onuEntry has 1 gemPort
			if lenOfGemPorts != 1 {
				t.Errorf("Expected 1 GemPort per ONU, found %d", lenOfGemPorts)
			}

			// check that the value of the gemport is correct
			gemID, _ := strconv.Atoi(fmt.Sprintf("90%d%d", i, o-1))
			currentValue := flowMgr[i].onuGemInfo[uint32(i)][o-1].GemPorts[0]
			if uint32(gemID) != currentValue {
				t.Errorf("Expected GemPort value to be %d, found %d", gemID, currentValue)
			}
		}
	}
}

func TestOpenOltFlowMgr_deleteGemPortFromLocalCache(t *testing.T) {
	// Create fresh flowMgr instance
	flowMgr = newMockFlowmgr()
	type args struct {
		intfID                uint32
		onuID                 uint32
		gemPortIDs            []uint32
		gemPortIDsToBeDeleted []uint32
		gemPortIDsRemaining   []uint32
		serialNum             string
		finalLength           int
	}
	tests := []struct {
		name string
		args args
	}{
		// Add/Delete single gem port
		{"DeleteGemPortFromLocalCache1", args{0, 1, []uint32{1}, []uint32{1}, []uint32{}, "onu1", 0}},
		// Delete all gemports
		{"DeleteGemPortFromLocalCache2", args{0, 1, []uint32{1, 2, 3, 4}, []uint32{1, 2, 3, 4}, []uint32{}, "onu1", 0}},
		// Try to delete when there is no gem port
		{"DeleteGemPortFromLocalCache3", args{0, 1, []uint32{}, []uint32{1, 2}, []uint32{}, "onu1", 0}},
		// Try to delete non-existent gem port
		{"DeleteGemPortFromLocalCache4", args{0, 1, []uint32{1}, []uint32{2}, []uint32{1}, "onu1", 1}},
		// Try to delete two of the gem ports
		{"DeleteGemPortFromLocalCache5", args{0, 1, []uint32{1, 2, 3, 4}, []uint32{2, 4}, []uint32{1, 3}, "onu1", 2}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: should check returned errors are as expected?
			_ = flowMgr[tt.args.intfID].UpdateOnuInfo(ctx, tt.args.intfID, tt.args.onuID, tt.args.serialNum)
			for _, gemPort := range tt.args.gemPortIDs {
				flowMgr[tt.args.intfID].addGemPortToOnuInfoMap(ctx, tt.args.intfID, tt.args.onuID, gemPort)
			}
			for _, gemPortDeleted := range tt.args.gemPortIDsToBeDeleted {
				flowMgr[tt.args.intfID].deleteGemPortFromLocalCache(ctx, tt.args.intfID, tt.args.onuID, gemPortDeleted)
			}
			lenofGemPorts := len(flowMgr[tt.args.intfID].onuGemInfo[tt.args.intfID][0].GemPorts)
			if lenofGemPorts != tt.args.finalLength {
				t.Errorf("GemPorts length is not as expected len = %d, want %d", lenofGemPorts, tt.args.finalLength)
			}
			gemPorts := flowMgr[tt.args.intfID].onuGemInfo[tt.args.intfID][0].GemPorts
			if !reflect.DeepEqual(tt.args.gemPortIDsRemaining, gemPorts) {
				t.Errorf("GemPorts are not as expected = %v, want %v", gemPorts, tt.args.gemPortIDsRemaining)
			}

		})
	}
}

func TestOpenOltFlowMgr_GetLogicalPortFromPacketIn(t *testing.T) {
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
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "pon", IntfId: 0, GemportId: 1, FlowId: 100, PortNo: 1, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 1, false},
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "nni", IntfId: 0, GemportId: 1, FlowId: 100, PortNo: 1, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 1048576, false},
		// Negative Test cases.
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "pon", IntfId: 1, GemportId: 1, FlowId: 100, PortNo: 1, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 0, true},
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "pon", IntfId: 0, GemportId: 1, FlowId: 100, PortNo: 0, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 16, false},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := flowMgr[tt.args.packetIn.IntfId].GetLogicalPortFromPacketIn(ctx, tt.args.packetIn)
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
	// Create fresh flowMgr instance
	flowMgr = newMockFlowmgr()

	//untagged packet in hex string
	untaggedStr := "01005e000002000000000001080046c00020000040000102fa140a000001e00000029404000017000705e10000fa"
	untagged, err := hex.DecodeString(untaggedStr)
	if err != nil {
		t.Error("Unable to parse hex string", err)
		panic(err)
	}
	//single-tagged packet in hex string. vlanID.pbit: 1.1
	singleTaggedStr := "01005e0000010025ba48172481002001080046c0002000004000010257deab140023e0000001940400001164ee9b0000000000000000000000000000"
	singleTagged, err := hex.DecodeString(singleTaggedStr)
	if err != nil {
		t.Error("Unable to parse hex string", err)
		panic(err)
	}
	//double-tagged packet in hex string. vlanID.pbit: 210.0-0.0
	doubleTaggedStr := "01005e000016deadbeefba118100021081000000080046000028000000000102c5b87f000001e0000016940400002200f8030000000104000000e10000fa"
	doubleTagged, err := hex.DecodeString(doubleTaggedStr)
	if err != nil {
		t.Error("Unable to parse hex string", err)
		panic(err)
	}

	type args struct {
		intfID  uint32
		onuID   uint32
		portNum uint32
		packet  []byte
	}
	tests := []struct {
		name    string
		args    args
		want    uint32
		wantErr bool
	}{
		// TODO: Add test cases.
		{"GetPacketOutGemPortID", args{intfID: 0, onuID: 1, portNum: 1, packet: untagged}, 1, false},
		{"GetPacketOutGemPortID", args{intfID: 1, onuID: 2, portNum: 2, packet: singleTagged}, 2, false},
		{"GetPacketOutGemPortID", args{intfID: 0, onuID: 1, portNum: 1, packet: doubleTagged}, 1, false},
		{"GetPacketOutGemPortID", args{intfID: 0, onuID: 10, portNum: 10, packet: untagged}, 2, true},
		{"GetPacketOutGemPortID", args{intfID: 0, onuID: 1, portNum: 3, packet: []byte{}}, 3, true},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := flowMgr[tt.args.intfID].GetPacketOutGemPortID(ctx, tt.args.intfID, tt.args.onuID, tt.args.portNum, tt.args.packet)
			if tt.wantErr {
				if err == nil {
					//error expected but got value
					t.Errorf("OpenOltFlowMgr.GetPacketOutGemPortID() = %v, wantErr %v", got, tt.wantErr)
				}
			} else {
				if err != nil {
					//error is not expected but got error
					t.Errorf("OpenOltFlowMgr.GetPacketOutGemPortID() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if got != tt.want {
					t.Errorf("OpenOltFlowMgr.GetPacketOutGemPortID() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestOpenOltFlowMgr_DeleteTechProfileInstance(t *testing.T) {
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
		{"DeleteTechProfileInstance", args{intfID: 0, onuID: 1, uniID: 1, sn: "", tpID: 64}, false},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := flowMgr[tt.args.intfID].DeleteTechProfileInstance(ctx, tt.args.intfID, tt.args.onuID, tt.args.uniID, tt.args.sn, tt.args.tpID); (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.DeleteTechProfileInstance() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltFlowMgr_checkAndAddFlow(t *testing.T) {
	ctx := context.Background()
	kw := make(map[string]uint64)
	kw["table_id"] = 1
	kw["meter_id"] = 1
	kw["write_metadata"] = 0x4000000000 // Tech-Profile-ID 64

	// Upstream flow
	fa := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(536870912),
			fu.Metadata_ofp(1),
			fu.IpProto(17), // dhcp
			fu.VlanPcp(0),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 257)),
			fu.Output(65536),
			fu.PushVlan(0x8100),
		},
		KV: kw,
	}

	// EAPOL
	fa2 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(536870912),
			fu.Metadata_ofp(1),
			fu.EthType(0x888E),
			fu.VlanPcp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 257),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 257)),
			fu.Output(65536),
			fu.PushVlan(0x8100),
		},
		KV: kw,
	}

	// HSIA
	fa3 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(536870912),
			fu.Metadata_ofp(1),
			//fu.EthType(0x8100),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0)),
			fu.Output(65536),
			fu.PushVlan(0x8100),
		},
		KV: kw,
	}

	fa4 := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(65535),
			fu.Metadata_ofp(1),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0),
			fu.VlanPcp(1),
		},
		Actions: []*ofp.OfpAction{
			//fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 0)),
			fu.Output(536870912),
			fu.PopVlan(),
		},
		KV: kw,
	}

	classifierInfo := make(map[string]interface{})
	actionInfo := make(map[string]interface{})
	classifierInfo2 := make(map[string]interface{})
	actionInfo2 := make(map[string]interface{})
	classifierInfo3 := make(map[string]interface{})
	actionInfo3 := make(map[string]interface{})
	classifierInfo4 := make(map[string]interface{})
	actionInfo4 := make(map[string]interface{})
	flowState, _ := fu.MkFlowStat(fa)
	flowState2, _ := fu.MkFlowStat(fa2)
	flowState3, _ := fu.MkFlowStat(fa3)
	flowState4, _ := fu.MkFlowStat(fa4)
	formulateClassifierInfoFromFlow(ctx, classifierInfo, flowState)
	formulateClassifierInfoFromFlow(ctx, classifierInfo2, flowState2)
	formulateClassifierInfoFromFlow(ctx, classifierInfo3, flowState3)
	formulateClassifierInfoFromFlow(ctx, classifierInfo4, flowState4)

	err := formulateActionInfoFromFlow(ctx, actionInfo, classifierInfo, flowState)
	if err != nil {
		// Error logging is already done in the called function
		// So just return in case of error
		return
	}

	err = formulateActionInfoFromFlow(ctx, actionInfo2, classifierInfo2, flowState2)
	if err != nil {
		// Error logging is already done in the called function
		// So just return in case of error
		return
	}

	err = formulateActionInfoFromFlow(ctx, actionInfo3, classifierInfo3, flowState3)
	if err != nil {
		// Error logging is already done in the called function
		// So just return in case of error
		return
	}

	err = formulateActionInfoFromFlow(ctx, actionInfo4, classifierInfo4, flowState4)
	if err != nil {
		// Error logging is already done in the called function
		// So just return in case of error
		return
	}

	//ofpMeterConfig := &ofp.OfpMeterConfig{Flags: 1, MeterId: 1}
	//flowMetadata := &voltha.FlowMetadata{
	//	Meters: []*ofp.OfpMeterConfig{ofpMeterConfig},
	//}

	TpInst := &tp.TechProfile{
		Name:                 "Test-Tech-Profile",
		SubscriberIdentifier: "257",
		ProfileType:          "Mock",
		Version:              1,
		NumGemPorts:          4,
		InstanceCtrl: tp.InstanceControl{
			Onu: "1",
			Uni: "16",
		},
	}

	type args struct {
		args           map[string]uint32
		classifierInfo map[string]interface{}
		actionInfo     map[string]interface{}
		flow           *ofp.OfpFlowStats
		gemPort        uint32
		intfID         uint32
		onuID          uint32
		uniID          uint32
		portNo         uint32
		TpInst         *tp.TechProfile
		allocID        []uint32
		gemPorts       []uint32
		TpID           uint32
		uni            string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "checkAndAddFlow-1",
			args: args{
				args:           nil,
				classifierInfo: classifierInfo,
				actionInfo:     actionInfo,
				flow:           flowState,
				gemPort:        1,
				intfID:         1,
				onuID:          1,
				uniID:          16,
				portNo:         1,
				TpInst:         TpInst,
				allocID:        []uint32{0x8001, 0x8002, 0x8003, 0x8004},
				gemPorts:       []uint32{1, 2, 3, 4},
				TpID:           64,
				uni:            "16",
			},
		},
		{
			name: "checkAndAddFlow-2",
			args: args{
				args:           nil,
				classifierInfo: classifierInfo2,
				actionInfo:     actionInfo2,
				flow:           flowState2,
				gemPort:        1,
				intfID:         1,
				onuID:          1,
				uniID:          16,
				portNo:         1,
				TpInst:         TpInst,
				allocID:        []uint32{0x8001, 0x8002, 0x8003, 0x8004},
				gemPorts:       []uint32{1, 2, 3, 4},
				TpID:           64,
				uni:            "16",
			},
		},
		{
			name: "checkAndAddFlow-3",
			args: args{
				args:           nil,
				classifierInfo: classifierInfo3,
				actionInfo:     actionInfo3,
				flow:           flowState3,
				gemPort:        1,
				intfID:         1,
				onuID:          1,
				uniID:          16,
				portNo:         1,
				TpInst:         TpInst,
				allocID:        []uint32{0x8001, 0x8002, 0x8003, 0x8004},
				gemPorts:       []uint32{1, 2, 3, 4},
				TpID:           64,
				uni:            "16",
			},
		},
		{
			name: "checkAndAddFlow-4",
			args: args{
				args:           nil,
				classifierInfo: classifierInfo4,
				actionInfo:     actionInfo4,
				flow:           flowState4,
				gemPort:        1,
				intfID:         1,
				onuID:          1,
				uniID:          16,
				portNo:         1,
				TpInst:         TpInst,
				allocID:        []uint32{0x8001, 0x8002, 0x8003, 0x8004},
				gemPorts:       []uint32{1, 2, 3, 4},
				TpID:           64,
				uni:            "16",
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flowMgr[tt.args.intfID].checkAndAddFlow(ctx, tt.args.args, tt.args.classifierInfo, tt.args.actionInfo, tt.args.flow,
				tt.args.TpInst, tt.args.gemPorts, tt.args.TpID, tt.args.uni)
		})
	}
}

func TestOpenOltFlowMgr_TestMulticastFlowAndGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//create group
	group := newGroup(2, []uint32{1})
	err := flowMgr[0].grpMgr.AddGroup(ctx, group)
	if err != nil {
		t.Error("group-add failed", err)
		return
	}
	//create multicast flow
	multicastFlowArgs := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(1048576),
			fu.VlanVid(660),             //vlan
			fu.Metadata_ofp(uint64(66)), //inner vlan
			fu.EthType(0x800),           //ipv4
			fu.Ipv4Dst(3809869825),      //227.22.0.1
		},
		Actions: []*ofp.OfpAction{
			fu.Group(1),
		},
	}
	ofpStats, _ := fu.MkFlowStat(multicastFlowArgs)
	fmt.Println(ofpStats.Id)
	err = flowMgr[0].AddFlow(ctx, ofpStats, &voltha.FlowMetadata{})
	if err != nil {
		t.Error("Multicast flow-add failed", err)
		return
	}

	//add bucket to the group
	group = newGroup(2, []uint32{1, 2})
	err = flowMgr[0].grpMgr.ModifyGroup(ctx, group)
	if err != nil {
		t.Error("modify-group failed", err)
		return
	}
	//remove the multicast flow
	err = flowMgr[0].RemoveFlow(ctx, ofpStats)
	if err != nil {
		t.Error("Multicast flow-remove failed", err)
		return
	}

	//remove the group
	err = flowMgr[0].grpMgr.DeleteGroup(ctx, group)
	if err != nil {
		t.Error("delete-group failed", err)
		return
	}
}
