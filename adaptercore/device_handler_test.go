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
	"net"
	"testing"

	"github.com/opencord/voltha-openolt-adapter/mocks"
	"github.com/opencord/voltha-protos/go/voltha"
)

func newMockDeviceDeviceHandler() *DeviceHandler {
	device := &voltha.Device{
		Id:       "olt",
		Root:     true,
		ParentId: "logical_device",
		Ports: []*voltha.Port{
			{PortNo: 1, Label: "pon"},
			{PortNo: 2, Label: "nni"},
		},
	}
	return &DeviceHandler{
		deviceID: device.GetId(),

		device:       device,
		coreProxy:    &mocks.MockCoreProxy{},
		AdapterProxy: &mocks.MockAdapterProxy{},
	}
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
		{"test1", args{host: "localhost"}, "00:00:7f:00:00:01", false},
		{"test2", args{host: "10.10.10.10"}, "00:00:0a:0a:0a:0a", false},
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
		"test1",
		args{ip: net.ParseIP("10.10.10.10")},
		"00:00:0a:0a:0a:0a",
	},
		{
			"test3",
			args{ip: net.ParseIP("127.0.0.1")},
			"00:00:7f:00:00:01",
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := macifyIP(tt.args.ip); got != tt.want {
				t.Errorf("macifyIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceHandler_GetOfpDeviceInfo(t *testing.T) {
	dh := newMockDeviceDeviceHandler()
	device := &voltha.Device{}
	got, err := dh.GetOfpDeviceInfo(device)
	if err != nil {
		t.Errorf("DeviceHandler.GetOfpDeviceInfo() error = %v", err)
		return
	}
	t.Logf("ofpDeviceInfo %v", got)
}

func TestDeviceHandler_GetOfpPortInfo(t *testing.T) {
	dh := newMockDeviceDeviceHandler()
	device := &voltha.Device{}
	got, err := dh.GetOfpPortInfo(device, 1)
	if err != nil {
		t.Errorf("DeviceHandler.GetOfpPortInfo() error = %v", err)
		return
	}
	t.Logf("ofpDeviceInfo %v", got)
}
func TestDeviceHandler_GetChildDevice(t *testing.T) {
	dh := newMockDeviceDeviceHandler()
	type args struct {
		parentPort uint32
		onuID      uint32
	}
	tests := []struct {
		name string
		args args
		want *voltha.Device
	}{
		{"test1",
			args{parentPort: 1,
				onuID: 1},
			&voltha.Device{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dh.GetChildDevice(tt.args.parentPort, tt.args.onuID)
			t.Log("onu device id", got)
		})
	}
}
