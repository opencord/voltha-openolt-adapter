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

	"github.com/opencord/voltha-go/adapters/adapterif"

	"github.com/opencord/voltha-go/common/log"
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
	dh := &DeviceHandler{}
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

func Test_collectNNIMetrics(t *testing.T) {
	type args struct {
		dh    *DeviceHandler
		nniID uint32
	}
	var res NniPort
	nnimap := map[uint32]*NniPort{}
	nnimap[0] = &NniPort{Name: "olt"}
	nnimap[1] = &NniPort{Name: "olt"}
	tests := []struct {
		name string
		args args
		want NniPort
	}{
		{"CollectNNIMetrics-1", args{&DeviceHandler{portStats: &OpenOltStatisticsMgr{Device: nil, NorthBoundPort: nnimap, SouthBoundPort: nil}}, 0}, res},
		{"CollectNNIMetrics-2", args{&DeviceHandler{portStats: &OpenOltStatisticsMgr{Device: nil, NorthBoundPort: nnimap, SouthBoundPort: nil}}, 1}, res},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectNNIMetrics(tt.args.dh, tt.args.nniID)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("collectNNIMetrics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_collectPONMetrics(t *testing.T) {
	var res PonPort
	ponmap := map[uint32]*PonPort{}
	ponmap[0] = &PonPort{DeviceID: "onu1"}
	type args struct {
		dh    *DeviceHandler
		ponID uint32
	}
	tests := []struct {
		name string
		args args
		want PonPort
	}{
		{"CollectPONMetrics-1", args{&DeviceHandler{portStats: &OpenOltStatisticsMgr{Device: nil, SouthBoundPort: ponmap, NorthBoundPort: nil}}, 0}, res},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectPONMetrics(tt.args.dh, tt.args.ponID)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("collectPONMetrics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_publishNNIMetrics(t *testing.T) {
	type args struct {
		cm      NniPort
		ep      adapterif.EventProxy
		devid   string
		context map[string]string
	}
	ctx := map[string]string{}

	tests := []struct {
		name string
		args args
	}{
		{"PublishNNIMetrics-1", args{NniPort{Name: "olt"}, nil, "olt", ctx}},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_publishPONMetrics(t *testing.T) {
	type args struct {
		cm      PonPort
		ep      adapterif.EventProxy
		devid   string
		context map[string]string
	}
	ctx := map[string]string{}
	tests := []struct {
		name string
		args args
	}{
		{"PublishPONMetrics-1", args{PonPort{DeviceID: "onu1"}, nil, "onu1", ctx}},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}
