/*
 * Copyright 2019-present Open Networking Foundation

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
	"reflect"
	"testing"

	"github.com/opencord/voltha-lib-go/v2/pkg/log"
	"github.com/opencord/voltha-protos/go/openolt"
	"github.com/opencord/voltha-protos/go/voltha"
)

func init() {
	_, _ = log.AddPackage(log.JSON, log.DebugLevel, nil)
}
func TestOpenOltStatisticsMgr_PortStatisticsIndication(t *testing.T) {
	device := &voltha.Device{
		Id:       "olt",
		Root:     true,
		ParentId: "logical_device",
		Ports: []*voltha.Port{
			{PortNo: 1, Label: "pon", Type: voltha.Port_ETHERNET_UNI},
			{PortNo: 2, Label: "nni", Type: voltha.Port_ETHERNET_NNI},
		},
		ProxyAddress: &voltha.Device_ProxyAddress{
			DeviceId:       "olt",
			DeviceType:     "onu",
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
	}
	dh := newMockDeviceHandler()
	dh.device = device
	StatMgr := NewOpenOltStatsMgr(dh)

	type args struct {
		PortStats *openolt.PortStatistics
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"PortStatisticsIndication", args{PortStats: &openolt.PortStatistics{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			StatMgr.PortStatisticsIndication(tt.args.PortStats, 16)
		})
	}
}

func TestOpenOltStatisticsMgr_publishMetrics(t *testing.T) {
	type fields struct {
		Device         *DeviceHandler
		NorthBoundPort map[uint32]*NniPort
		SouthBoundPort map[uint32]*PonPort
	}
	type args struct {
		portType string
		val      map[string]float32
		portnum  uint32
		context  map[string]string
	}
	ctx := map[string]string{}
	ctx["deviceID"] = "Test"
	ponmap := map[uint32]*PonPort{}
	ponmap[0] = &PonPort{
		PONID:          0,
		DeviceID:       "onu1",
		IntfID:         0,
		PortNum:        0,
		PortID:         0,
		Label:          "",
		ONUs:           nil,
		ONUsByID:       nil,
		RxBytes:        0,
		RxPackets:      0,
		RxMcastPackets: 0,
		RxBcastPackets: 0,
		RxErrorPackets: 0,
		TxBytes:        0,
		TxPackets:      0,
		TxUcastPackets: 0,
		TxMcastPackets: 0,
		TxBcastPackets: 0,
		TxErrorPackets: 0,
	}
	nnimap := map[uint32]*NniPort{}
	nnimap[0] = &NniPort{
		PortNum:        0,
		Name:           "olt1",
		LogicalPort:    0,
		IntfID:         0,
		RxBytes:        0,
		RxPackets:      0,
		RxMcastPackets: uint64(1111),
		RxBcastPackets: 0,
		RxErrorPackets: 0,
		TxBytes:        0,
		TxPackets:      0,
		TxUcastPackets: 0,
		TxMcastPackets: 0,
		TxBcastPackets: 0,
		TxErrorPackets: 0,
	}
	pval := make(map[string]float32)
	pval["rx_bytes"] = float32(111)
	nval := make(map[string]float32)
	nval["rx_bytes"] = float32(111)
	dhandlerNNI := newMockDeviceHandler()
	dhandlerNNI.portStats = &OpenOltStatisticsMgr{Device: nil, SouthBoundPort: nil, NorthBoundPort: nnimap}
	dhandlerPON := newMockDeviceHandler()
	dhandlerPON.portStats = &OpenOltStatisticsMgr{Device: nil, SouthBoundPort: ponmap, NorthBoundPort: nil}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "PublishNNIMetrics-1",
			fields: fields{
				Device:         dhandlerNNI,
				NorthBoundPort: nnimap,
				SouthBoundPort: nil,
			},
			args: args{
				portType: "NNIStats",
				val:      nval,
				portnum:  0,
				context:  ctx,
			},
		},
		{
			name: "PublishPONMetrics-1",
			fields: fields{
				Device:         dhandlerPON,
				NorthBoundPort: nil,
				SouthBoundPort: ponmap,
			},
			args: args{
				portType: "PONStats",
				val:      pval,
				portnum:  0,
				context:  ctx,
			},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			StatMgr := &OpenOltStatisticsMgr{
				Device:         tt.fields.Device,
				NorthBoundPort: tt.fields.NorthBoundPort,
				SouthBoundPort: tt.fields.SouthBoundPort,
			}
			StatMgr.publishMetrics(tt.args.portType, tt.args.val, tt.args.portnum, tt.args.context, "onu1")

		})
	}
}

func TestOpenOltStatisticsMgr_collectNNIMetrics(t *testing.T) {
	type fields struct {
		Device         *DeviceHandler
		NorthBoundPort map[uint32]*NniPort
		SouthBoundPort map[uint32]*PonPort
	}
	type args struct {
		nniID uint32
	}
	dhandler := newMockDeviceHandler()
	pmconfig := make(map[string]*voltha.PmConfig)
	pmconfig["rx_bytes"] = &voltha.PmConfig{Name: "olt"}

	var res map[string]float32
	nnimap := map[uint32]*NniPort{}
	nnimap[0] = &NniPort{Name: "olt"}
	nnimap[1] = &NniPort{Name: "olt"}
	dh := &DeviceHandler{portStats: &OpenOltStatisticsMgr{Device: dhandler, SouthBoundPort: nil, NorthBoundPort: nnimap}}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]float32
	}{
		{"CollectNNIMetrics-1", fields{
			Device:         dh,
			NorthBoundPort: nnimap,
			SouthBoundPort: nil,
		}, args{0}, res},
		{"CollectNNIMetrics-2", fields{
			Device:         dh,
			NorthBoundPort: nnimap,
			SouthBoundPort: nil,
		}, args{1}, res},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			StatMgr := &OpenOltStatisticsMgr{
				Device:         tt.fields.Device,
				NorthBoundPort: tt.fields.NorthBoundPort,
				SouthBoundPort: tt.fields.SouthBoundPort,
			}
			got := StatMgr.collectNNIMetrics(tt.args.nniID)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("collectNNIMetrics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenOltStatisticsMgr_collectPONMetrics(t *testing.T) {
	type fields struct {
		Device         *DeviceHandler
		NorthBoundPort map[uint32]*NniPort
		SouthBoundPort map[uint32]*PonPort
	}
	type args struct {
		pID uint32
	}
	dhandler := newMockDeviceHandler()
	pmconfig := make(map[string]*voltha.PmConfig)
	pmconfig["rx_bytes"] = &voltha.PmConfig{Name: "olt"}

	var res map[string]float32
	ponmap := map[uint32]*PonPort{}
	ponmap[0] = &PonPort{DeviceID: "olt"}
	ponmap[1] = &PonPort{DeviceID: "olt"}
	dh := &DeviceHandler{portStats: &OpenOltStatisticsMgr{Device: dhandler, SouthBoundPort: ponmap, NorthBoundPort: nil}}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]float32
	}{
		{"CollectPONMetrics-1", fields{
			Device:         dh,
			NorthBoundPort: nil,
			SouthBoundPort: ponmap,
		}, args{0}, res},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			StatMgr := &OpenOltStatisticsMgr{
				Device:         tt.fields.Device,
				NorthBoundPort: tt.fields.NorthBoundPort,
				SouthBoundPort: tt.fields.SouthBoundPort,
			}
			got := StatMgr.collectPONMetrics(tt.args.pID)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("collectPONMetrics() = %v, want %v", got, tt.want)
			}
		})
	}
}
