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
	"testing"

	"github.com/opencord/voltha-lib-go/pkg/common/log"
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

			StatMgr.PortStatisticsIndication(tt.args.PortStats)
		})
	}
}
