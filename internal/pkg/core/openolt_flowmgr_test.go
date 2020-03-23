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
	"fmt"
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

var flowMgr *OpenOltFlowMgr

func init() {
	log.SetDefaultLogger(log.JSON, log.DebugLevel, nil)
	flowMgr = newMockFlowmgr()
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

func newMockFlowmgr() *OpenOltFlowMgr {
	rMgr := newMockResourceMgr()
	dh := newMockDeviceHandler()

	rMgr.KVStore = &db.Backend{}
	rMgr.KVStore.Client = &mocks.MockKVClient{}

	dh.resourceMgr = rMgr
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	flwMgr := NewFlowManager(ctx, dh, rMgr)

	onuGemInfo1 := make([]rsrcMgr.OnuGemInfo, 2)
	onuGemInfo2 := make([]rsrcMgr.OnuGemInfo, 2)
	onuGemInfo1[0] = rsrcMgr.OnuGemInfo{OnuID: 1, SerialNumber: "1", IntfID: 1, GemPorts: []uint32{1}}
	onuGemInfo2[1] = rsrcMgr.OnuGemInfo{OnuID: 2, SerialNumber: "2", IntfID: 2, GemPorts: []uint32{2}}
	flwMgr.onuGemInfo[1] = onuGemInfo1
	flwMgr.onuGemInfo[2] = onuGemInfo2

	packetInGemPort := make(map[rsrcMgr.PacketInInfoKey]uint32)
	packetInGemPort[rsrcMgr.PacketInInfoKey{IntfID: 1, OnuID: 1, LogicalPort: 1}] = 1
	packetInGemPort[rsrcMgr.PacketInInfoKey{IntfID: 2, OnuID: 2, LogicalPort: 2}] = 2

	flwMgr.packetInGemPort = packetInGemPort
	tps := make(map[uint32]tp.TechProfileIf)
	for key := range rMgr.ResourceMgrs {
		tps[key] = mocks.MockTechProfile{TpID: key}
	}
	flwMgr.techprofile = tps

	interface2mcastQeueuMap := make(map[uint32]*queueInfoBrief)
	interface2mcastQeueuMap[0] = &queueInfoBrief{
		gemPortID:       4000,
		servicePriority: 3,
	}
	flwMgr.interfaceToMcastQueueMap = interface2mcastQeueuMap
	return flwMgr
}

func TestOpenOltFlowMgr_CreateSchedulerQueues(t *testing.T) {
	// flowMgr := newMockFlowmgr()

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
		name       string
		schedQueue schedQueue
		wantErr    bool
	}{
		// TODO: Add test cases.
		{"CreateSchedulerQueues-1", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 1, flowmetadata}, false},
		{"CreateSchedulerQueues-2", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 1, flowmetadata}, false},
		{"CreateSchedulerQueues-3", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-4", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-5", schedQueue{tp_pb.Direction_UPSTREAM, 2, 2, 2, 64, 2, tprofile, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-6", schedQueue{tp_pb.Direction_DOWNSTREAM, 2, 2, 2, 65, 2, tprofile2, 2, flowmetadata}, true},
		{"CreateSchedulerQueues-13", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 1, flowmetadata}, false},
		//Negative testcases
		{"CreateSchedulerQueues-7", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 1, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-8", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 0, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-9", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 1, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-10", schedQueue{tp_pb.Direction_UPSTREAM, 1, 1, 1, 64, 1, tprofile, 2, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-11", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 2, &voltha.FlowMetadata{}}, true},
		{"CreateSchedulerQueues-12", schedQueue{tp_pb.Direction_DOWNSTREAM, 1, 1, 1, 65, 1, tprofile2, 2, nil}, true},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := flowMgr.CreateSchedulerQueues(ctx, tt.schedQueue); (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.CreateSchedulerQueues() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltFlowMgr_RemoveSchedulerQueues(t *testing.T) {

	// flowMgr := newMockFlowmgr()
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
	type args struct {
		Dir     tp_pb.Direction
		IntfID  uint32
		OnuID   uint32
		UniID   uint32
		UniPort uint32
		TpInst  *tp.TechProfile
	}
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
			if err := flowMgr.RemoveSchedulerQueues(ctx, tt.schedQueue); (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.RemoveSchedulerQueues() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

}

func TestOpenOltFlowMgr_RemoveFlow(t *testing.T) {
	// flowMgr := newMockFlowmgr()
	log.Debug("Info Warning Error: Starting RemoveFlow() test")
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
			flowMgr.RemoveFlow(ctx, tt.args.flow)
		})
	}
	// t.Error("=====")
}

func TestOpenOltFlowMgr_AddFlow(t *testing.T) {
	// flowMgr := newMockFlowmgr()
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
			flowMgr.AddFlow(ctx, tt.args.flow, tt.args.flowMetadata)
		})
	}
}

func TestOpenOltFlowMgr_UpdateOnuInfo(t *testing.T) {
	// flowMgr := newMockFlowmgr()
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			flowMgr.UpdateOnuInfo(ctx, tt.args.intfID, tt.args.onuID, tt.args.serialNum)
		})
	}
}

func TestOpenOltFlowMgr_deleteGemPortFromLocalCache(t *testing.T) {
	// flowMgr := newMockFlowmgr()
	type args struct {
		intfID                uint32
		onuID                 uint32
		gemPortIDs            []uint32
		gemPortIDsToBeDeleted []uint32
		serialNum             string
		finalLength           int
	}
	tests := []struct {
		name string
		args args
	}{
		// Add/Delete single gem port
		{"DeleteGemPortFromLocalCache1", args{0, 1, []uint32{1}, []uint32{1}, "onu1", 0}},
		// Delete all gemports
		{"DeleteGemPortFromLocalCache2", args{0, 1, []uint32{1, 2, 3, 4}, []uint32{1, 2, 3, 4}, "onu1", 0}},
		// Try to delete when there is no gem port
		{"DeleteGemPortFromLocalCache3", args{0, 1, []uint32{}, []uint32{1, 2}, "onu1", 0}},
		// Try to delete non-existent gem port
		{"DeleteGemPortFromLocalCache4", args{0, 1, []uint32{1}, []uint32{2}, "onu1", 1}},
		// Try to delete two of the gem ports
		{"DeleteGemPortFromLocalCache5", args{0, 1, []uint32{1, 2, 3, 4}, []uint32{2, 4}, "onu1", 2}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flowMgr.UpdateOnuInfo(ctx, tt.args.intfID, tt.args.onuID, tt.args.serialNum)
			for _, gemPort := range tt.args.gemPortIDs {
				flowMgr.addGemPortToOnuInfoMap(ctx, tt.args.intfID, tt.args.onuID, gemPort)
			}
			for _, gemPortDeleted := range tt.args.gemPortIDsToBeDeleted {
				flowMgr.deleteGemPortFromLocalCache(tt.args.intfID, tt.args.onuID, gemPortDeleted)
			}
			lenofGemPorts := len(flowMgr.onuGemInfo[tt.args.intfID][0].GemPorts)
			if lenofGemPorts != tt.args.finalLength {
				t.Errorf("GemPorts length is not as expected len = %d, want %d", lenofGemPorts, tt.args.finalLength)
			}

		})
	}
}

func TestOpenOltFlowMgr_GetLogicalPortFromPacketIn(t *testing.T) {
	// flowMgr := newMockFlowmgr()
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
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "nni", IntfId: 1, GemportId: 1, FlowId: 100, PortNo: 1, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 1048577, false},
		// Negative Test cases.
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "pon", IntfId: 2, GemportId: 1, FlowId: 100, PortNo: 1, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 0, true},
		{"GetLogicalPortFromPacketIn", args{packetIn: &openoltpb2.PacketIndication{IntfType: "pon", IntfId: 1, GemportId: 1, FlowId: 100, PortNo: 0, Cookie: 100, Pkt: []byte("GetLogicalPortFromPacketIn")}}, 4112, false},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := flowMgr.GetLogicalPortFromPacketIn(ctx, tt.args.packetIn)
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
	// flwMgr := newMockFlowmgr()

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := flowMgr.GetPacketOutGemPortID(ctx, tt.args.intfID, tt.args.onuID, tt.args.portNum)
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
	// flwMgr := newMockFlowmgr()
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
			if err := flowMgr.DeleteTechProfileInstance(ctx, tt.args.intfID, tt.args.onuID, tt.args.uniID, tt.args.sn, tt.args.tpID); (err != nil) != tt.wantErr {
				t.Errorf("OpenOltFlowMgr.DeleteTechProfileInstance() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOltFlowMgr_checkAndAddFlow(t *testing.T) {
	// flowMgr := newMockFlowmgr()
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
	formulateClassifierInfoFromFlow(classifierInfo, flowState)
	formulateClassifierInfoFromFlow(classifierInfo2, flowState2)
	formulateClassifierInfoFromFlow(classifierInfo3, flowState3)
	formulateClassifierInfoFromFlow(classifierInfo4, flowState4)

	err := formulateActionInfoFromFlow(actionInfo, classifierInfo, flowState)
	if err != nil {
		// Error logging is already done in the called function
		// So just return in case of error
		return
	}

	err = formulateActionInfoFromFlow(actionInfo2, classifierInfo2, flowState2)
	if err != nil {
		// Error logging is already done in the called function
		// So just return in case of error
		return
	}

	err = formulateActionInfoFromFlow(actionInfo3, classifierInfo3, flowState3)
	if err != nil {
		// Error logging is already done in the called function
		// So just return in case of error
		return
	}

	err = formulateActionInfoFromFlow(actionInfo4, classifierInfo4, flowState4)
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

	type fields struct {
		techprofile   []tp.TechProfileIf
		deviceHandler *DeviceHandler
		resourceMgr   *rsrcMgr.OpenOltResourceMgr
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
		name   string
		fields fields
		args   args
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
			flowMgr.checkAndAddFlow(ctx, tt.args.args, tt.args.classifierInfo, tt.args.actionInfo, tt.args.flow,
				tt.args.TpInst, tt.args.gemPorts, tt.args.TpID, tt.args.uni)
		})
	}
}

func TestOpenOltFlowMgr_TestMulticastFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//create group
	group := newGroup(2, []uint32{1})
	flowMgr.AddGroup(ctx, group)

	//create multicast flow
	multicastFlowArgs := &fu.FlowArgs{
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
	ofpStats, _ := fu.MkFlowStat(multicastFlowArgs)
	flowMgr.AddFlow(ctx, ofpStats, &voltha.FlowMetadata{})

	//add bucket to the group
	group = newGroup(2, []uint32{1, 2})

	flowMgr.ModifyGroup(ctx, group)
}
