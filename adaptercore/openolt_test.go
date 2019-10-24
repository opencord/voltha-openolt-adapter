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
This file contains unit test cases for functions in the file openolt.go.
This file also implements the fields struct to mock the Openolt and few utility functions.
*/

//Package adaptercore provides the utility for olt devices, flows and statistics
package adaptercore

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	com "github.com/opencord/voltha-lib-go/v2/pkg/adapters/common"
	fu "github.com/opencord/voltha-lib-go/v2/pkg/flows"
	"github.com/opencord/voltha-lib-go/v2/pkg/kafka"
	ic "github.com/opencord/voltha-protos/go/inter_container"
	"github.com/opencord/voltha-protos/go/openflow_13"
	ofp "github.com/opencord/voltha-protos/go/openflow_13"
	"github.com/opencord/voltha-protos/go/voltha"
)

// mocks the OpenOLT struct.
type fields struct {
	deviceHandlers        map[string]*DeviceHandler
	coreProxy             *com.CoreProxy
	adapterProxy          *com.AdapterProxy
	eventProxy            *com.EventProxy
	kafkaICProxy          *kafka.InterContainerProxy
	numOnus               int
	KVStoreHost           string
	KVStorePort           int
	KVStoreType           string
	exitChannel           chan int
	lockDeviceHandlersMap sync.RWMutex
	ctx                   context.Context
}

// mockOlt mocks OpenOLT struct.
func mockOlt() *fields {
	dh := newMockDeviceHandler()
	newOlt := &fields{}
	newOlt.deviceHandlers = map[string]*DeviceHandler{}
	newOlt.deviceHandlers[dh.device.Id] = dh
	return newOlt
}

// testOltObject maps fields type to OpenOLt type.
func testOltObject(testOlt *fields) *OpenOLT {
	return &OpenOLT{
		deviceHandlers: testOlt.deviceHandlers,
		coreProxy:      testOlt.coreProxy,
		adapterProxy:   testOlt.adapterProxy,
		eventProxy:     testOlt.eventProxy,
		kafkaICProxy:   testOlt.kafkaICProxy,
		numOnus:        testOlt.numOnus,
		KVStoreHost:    testOlt.KVStoreHost,
		KVStorePort:    testOlt.KVStorePort,
		KVStoreType:    testOlt.KVStoreType,
		exitChannel:    testOlt.exitChannel,
	}
}

// mockDevice mocks Device.
func mockDevice() *voltha.Device {
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
	return device
}

func TestNewOpenOLT(t *testing.T) {
	tests := []struct {
		name   string
		fields *fields
		want   *OpenOLT
	}{
		{"newopenolt-1", &fields{numOnus: 1, KVStorePort: 1, KVStoreType: "consul", KVStoreHost: "1.1.1.1"},
			&OpenOLT{numOnus: 1, KVStorePort: 1, KVStoreType: "consul", KVStoreHost: "1.1.1.1"}},
		{"newopenolt-2", &fields{numOnus: 2, KVStorePort: 2, KVStoreType: "etcd", KVStoreHost: "2.2.2.2"},
			&OpenOLT{numOnus: 2, KVStorePort: 2, KVStoreType: "etcd", KVStoreHost: "2.2.2.2"}},
		{"newopenolt-3", &fields{numOnus: 3, KVStorePort: 3, KVStoreType: "consul", KVStoreHost: "3.3.3.3"},
			&OpenOLT{numOnus: 3, KVStorePort: 3, KVStoreType: "consul", KVStoreHost: "3.3.3.3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewOpenOLT(tt.fields.ctx, tt.fields.kafkaICProxy, tt.fields.coreProxy, tt.fields.adapterProxy,
				tt.fields.eventProxy, tt.fields.numOnus, tt.fields.KVStoreHost, tt.fields.KVStorePort,
				tt.fields.KVStoreType); reflect.TypeOf(got) != reflect.TypeOf(tt.want) && got != nil {
				t.Errorf("NewOpenOLT() error = %v, wantErr %v", got, tt.want)
			}
		})
	}
}

func TestOpenOLT_Abandon_device(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"abandon_device-1", &fields{}, args{}, errors.New("unImplemented")},
		{"abandon_device-2", &fields{}, args{}, errors.New("unImplemented")},
		{"abandon_device-3", &fields{}, args{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Abandon_device(tt.args.device); (err != nil) && (reflect.TypeOf(err) !=
				reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Abandon_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Activate_image_update(t *testing.T) {
	type args struct {
		device  *voltha.Device
		request *voltha.ImageDownload
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    *voltha.ImageDownload
		wantErr error
	}{
		{"activate_image_upate-1", &fields{}, args{}, &voltha.ImageDownload{Id: "Image1-ABC123XYZ"},
			errors.New("unImplemented")},
		{"activate_image_upate-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123CDE"},
			errors.New("unImplemented")},
		{"activate_image_upate-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image3-ABC123EFG"},
			errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Activate_image_update(tt.args.device, tt.args.request)
			if (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && (got == nil) {
				t.Errorf("Activate_image_update() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Adapter_descriptor(t *testing.T) {
	tests := []struct {
		name    string
		fields  *fields
		wantErr error
	}{
		{"adapter_descriptor-1", &fields{}, errors.New("unImplemented")},
		{"adapter_descriptor-2", &fields{}, errors.New("unImplemented")},
		{"adapter_descriptor-3", &fields{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Adapter_descriptor(); (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Adapter_descriptor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Adopt_device(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	var device = mockDevice()
	device.Id = "openolt"
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"adopt_device-1", mockOlt(), args{}, errors.New("nil-device")},
		{"adopt_device-2", mockOlt(), args{device}, errors.New("nil-device")},
		{"adopt_device-3", mockOlt(),
			args{mockDevice()}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			err := oo.Adopt_device(tt.args.device)
			if (err != nil) && (reflect.TypeOf(err) !=
				reflect.TypeOf(tt.wantErr)) && (tt.args.device == nil) {
				t.Errorf("Adopt_device() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				t.Log("return'd nil")
			}
		})
	}
}

func TestOpenOLT_Cancel_image_download(t *testing.T) {
	type args struct {
		device  *voltha.Device
		request *voltha.ImageDownload
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    *voltha.ImageDownload
		wantErr error
	}{
		{"cancel_image_download-1", &fields{}, args{}, &voltha.ImageDownload{Id: "Image1-ABC123XYZ"},
			errors.New("unImplemented")},
		{"cancel_image_download-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123IJK"},
			errors.New("unImplemented")},
		{"cancel_image_download-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image3-ABC123KLM"},
			errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Cancel_image_download(tt.args.device, tt.args.request)
			if (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && got == nil {
				t.Errorf("Cancel_image_download() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Delete_device(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"delete_device-1", &fields{}, args{mockDevice()}, errors.New("device-handler-not-found")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Delete_device(tt.args.device); (err != nil) && (reflect.TypeOf(err) !=
				reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Delete_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Device_types(t *testing.T) {
	tests := []struct {
		name    string
		fields  *fields
		want    *voltha.DeviceTypes
		wantErr error
	}{
		{"device_types-1", &fields{}, &voltha.DeviceTypes{},
			errors.New("unImplemented")},
		{"device_types-2", &fields{}, &voltha.DeviceTypes{},
			errors.New("unImplemented")},
		{"device_types-3", &fields{}, &voltha.DeviceTypes{},
			errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Device_types()
			if (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && got == nil {
				t.Errorf("Device_types() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Disable_device(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"disable_device-1", mockOlt(), args{mockDevice()}, errors.New("device-handler-not-found")},
		{"disable_device-2", &fields{}, args{mockDevice()}, errors.New("device-handler-not-found")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Disable_device(tt.args.device); (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && err != nil {
				t.Errorf("Disable_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Download_image(t *testing.T) {
	type args struct {
		device  *voltha.Device
		request *voltha.ImageDownload
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    *voltha.ImageDownload
		wantErr error
	}{
		{"download_image-1", &fields{}, args{}, &voltha.ImageDownload{Id: "Image1-ABC123XYZ"},
			errors.New("unImplemented")},
		{"download_image-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123LKJ"},
			errors.New("unImplemented")},
		{"download_image-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image1-ABC123RTY"},
			errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Download_image(tt.args.device, tt.args.request)
			if (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && got == nil {
				t.Errorf("Download_image() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Get_device_details(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"get_device_details-1", &fields{}, args{}, errors.New("unImplemented")},
		{"get_device_details-2", &fields{}, args{}, errors.New("unImplemented")},
		{"get_device_details-3", &fields{}, args{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Get_device_details(tt.args.device); (err != nil) &&
				(reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Get_device_details() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Get_image_download_status(t *testing.T) {
	type args struct {
		device  *voltha.Device
		request *voltha.ImageDownload
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    *voltha.ImageDownload
		wantErr error
	}{
		{"get_image_download_status-1", &fields{}, args{}, &voltha.ImageDownload{Id: "Image1-ABC123XYZ"},
			errors.New("unImplemented")},
		{"get_image_download_status-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123LKJ"},
			errors.New("unImplemented")},
		{"get_image_download_status-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image1-ABC123DFG"},
			errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Get_image_download_status(tt.args.device, tt.args.request)
			if (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && got == nil {
				t.Errorf("Get_image_download_status() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Get_ofp_device_info(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    *ic.SwitchCapability
		wantErr error
	}{
		{"get_ofp_device_info-1", mockOlt(), args{mockDevice()}, &ic.SwitchCapability{},
			errors.New("device-handler-not-set")},
		{"get_ofp_device_info-2", &fields{}, args{mockDevice()}, &ic.SwitchCapability{},
			errors.New("device-handler-not-set")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Get_ofp_device_info(tt.args.device)
			if (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && got == nil {
				t.Errorf("Get_ofp_device_info() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (err == nil) && got != nil {
				t.Log("got :", got)
			}
		})
	}
}

func TestOpenOLT_Get_ofp_port_info(t *testing.T) {
	type args struct {
		device *voltha.Device
		portNo int64
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    *ic.PortCapability
		wantErr error
	}{
		{"get_ofp_port_info-1", mockOlt(), args{mockDevice(), 1}, &ic.PortCapability{},
			errors.New("device-handler-not-set")},
		{"get_ofp_port_info-2", &fields{}, args{mockDevice(), 1}, &ic.PortCapability{},
			errors.New("device-handler-not-set")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Get_ofp_port_info(tt.args.device, tt.args.portNo)
			if (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && got == nil {
				t.Errorf("Get_ofp_port_info() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (err == nil) && got != nil {
				t.Log("got :", got)
			}
		})
	}
}

func TestOpenOLT_Health(t *testing.T) {
	tests := []struct {
		name    string
		fields  *fields
		want    *voltha.HealthStatus
		wantErr error
	}{
		{"health-1", &fields{}, &voltha.HealthStatus{}, errors.New("unImplemented")},
		{"health-2", &fields{}, &voltha.HealthStatus{}, errors.New("unImplemented")},
		{"health-3", &fields{}, &voltha.HealthStatus{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Health()
			if (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && got == nil {
				t.Errorf("Get_ofp_port_info() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Process_inter_adapter_message(t *testing.T) {
	type args struct {
		msg *ic.InterAdapterMessage
	}
	var message1 = args{
		msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:            "olt",
				ProxyDeviceId: "",
				ToDeviceId:    "onu1",
			},
		},
	}
	var message2 = args{
		msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:            "olt",
				ProxyDeviceId: "olt",
				ToDeviceId:    "olt",
			},
		},
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"process_inter_adaptor_messgae-1", mockOlt(), message1, errors.New("handler-not-found")},
		{"process_inter_adaptor_messgae-2", mockOlt(), message2, errors.New("handler-not-found")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Process_inter_adapter_message(tt.args.msg); (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && err != nil {
				t.Errorf("Process_inter_adapter_message() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Reboot_device(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"reboot_device-1", mockOlt(), args{mockDevice()}, errors.New("device-handler-not-found")},
		{"reboot_device-2", &fields{}, args{mockDevice()}, errors.New("device-handler-not-found")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Reboot_device(tt.args.device); (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && err != nil {
				t.Errorf("Reboot_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Receive_packet_out(t *testing.T) {
	acts := []*ofp.OfpAction{
		fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA))),
		fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 101)),
		fu.Output(1),
	}
	type args struct {
		deviceID     string
		egressPortNo int
		packet       *openflow_13.OfpPacketOut
	}
	pktout := &ofp.OfpPacketOut{BufferId: 0, InPort: 1, Actions: acts, Data: []byte("AYDCAAAOAODsSE5TiMwCBwQA4OxITlIEBQUwLzUx" +
		"BgIAFAgEMC81MQoJbG9jYWxob3N0EBwFAawbqqACAAAAoRAxLjMuNi4xLjQuMS40NDEz/gYAgMILAgD+GQCAwgkDAAAAAGQAAAAAAAAAAgICAgICAgL+" +
		"GQCAwgoDAAAAAGQAAAAAAAAAAgICAgICAgIAAA==")}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"receive_packet_out-1", mockOlt(), args{mockDevice().Id, 1, pktout},
			errors.New("device-handler-not-set")},
		{"receive_packet_out-2", mockOlt(), args{"1234", 1, pktout},
			errors.New("device-handler-not-set")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Receive_packet_out(tt.args.deviceID, tt.args.egressPortNo, tt.args.packet); (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Receive_packet_out() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Reconcile_device(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"reconcile_device-1", &fields{}, args{}, errors.New("unImplemented")},
		{"reconcile_device-2", &fields{}, args{}, errors.New("unImplemented")},
		{"reconcile_device-3", &fields{}, args{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Reconcile_device(tt.args.device); (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Reconcile_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Reenable_device(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"reenable_device-1", mockOlt(), args{mockDevice()}, errors.New("device-handler-not-found")},
		{"reenable_device-2", &fields{}, args{mockDevice()}, errors.New("device-handler-not-found")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Reenable_device(tt.args.device); (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && err != nil {
				t.Errorf("Reenable_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Revert_image_update(t *testing.T) {
	type args struct {
		device  *voltha.Device
		request *voltha.ImageDownload
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    *voltha.ImageDownload
		wantErr error
	}{
		{"revert_image_update-1", &fields{}, args{}, &voltha.ImageDownload{Id: "Image1-ABC123XYZ"},
			errors.New("unImplemented")},
		{"revert_image_update-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123TYU"},
			errors.New("unImplemented")},
		{"revert_image_update-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image3-ABC123GTH"},
			errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Revert_image_update(tt.args.device, tt.args.request)
			if (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) && got == nil {
				t.Log("error :", err)
			}
		})
	}
}

func TestOpenOLT_Self_test_device(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"self_test_device-1", &fields{}, args{}, errors.New("unImplemented")},
		{"self_test_device-2", &fields{}, args{}, errors.New("unImplemented")},
		{"self_test_device-3", &fields{}, args{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Self_test_device(tt.args.device); (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Self_test_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Start(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"start-1", &fields{}, args{}, errors.New("start error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Start(tt.args.ctx); err != nil {
				t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Stop(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"stop-1", &fields{exitChannel: make(chan int, 1)}, args{}, errors.New("stop error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			oo.Start(tt.args.ctx)
			if err := oo.Stop(tt.args.ctx); err != nil {
				t.Errorf("Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Suppress_alarm(t *testing.T) {
	type args struct {
		filter *voltha.AlarmFilter
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"suppress_alarm-1", &fields{}, args{}, errors.New("unImplemented")},
		{"suppress_alarm-2", &fields{}, args{}, errors.New("unImplemented")},
		{"suppress_alarm-3", &fields{}, args{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Suppress_alarm(tt.args.filter); (err != nil) &&
				(reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Suppress_alarm() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Unsuppress_alarm(t *testing.T) {
	type args struct {
		filter *voltha.AlarmFilter
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"unsupress_alarm-1", &fields{}, args{}, errors.New("unImplemented")},
		{"unsupress_alarm-2", &fields{}, args{}, errors.New("unImplemented")},
		{"unsupress_alarm-3", &fields{}, args{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Unsuppress_alarm(tt.args.filter); (err != nil) &&
				(reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Unsuppress_alarm() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Update_flows_bulk(t *testing.T) {
	type args struct {
		device       *voltha.Device
		flows        *voltha.Flows
		groups       *voltha.FlowGroups
		flowMetadata *voltha.FlowMetadata
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"update_flows_bulk-1", &fields{}, args{}, errors.New("unImplemented")},
		{"update_flows_bulk-2", &fields{}, args{}, errors.New("unImplemented")},
		{"update_flows_bulk-3", &fields{}, args{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Update_flows_bulk(tt.args.device, tt.args.flows, tt.args.groups, tt.args.flowMetadata); (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Update_flows_bulk() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Update_flows_incrementally(t *testing.T) {
	type args struct {
		device       *voltha.Device
		flows        *openflow_13.FlowChanges
		groups       *openflow_13.FlowGroupChanges
		flowMetadata *voltha.FlowMetadata
	}

	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"update_flows_incrementally-1", &fields{}, args{device: mockDevice()},
			errors.New("device-handler-not-set")},
		{"update_flows_incrementally-1", mockOlt(), args{device: mockDevice()},
			errors.New("device-handler-not-set")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Update_flows_incrementally(tt.args.device, tt.args.flows, tt.args.groups, tt.args.flowMetadata); (err != nil) && (reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Update_flows_incrementally() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Update_pm_config(t *testing.T) {
	type args struct {
		device    *voltha.Device
		pmConfigs *voltha.PmConfigs
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"update_pm_config-1", &fields{}, args{}, errors.New("unImplemented")},
		{"update_pm_config-2", &fields{}, args{}, errors.New("unImplemented")},
		{"update_pm_config-3", &fields{}, args{}, errors.New("unImplemented")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Update_pm_config(tt.args.device, tt.args.pmConfigs); (err != nil) &&
				(reflect.TypeOf(err) != reflect.TypeOf(tt.wantErr)) {
				t.Errorf("Update_pm_config() error = %v, wantErr %v", err, tt.wantErr)
			}

		})
	}
}

func TestOpenOLT_deleteDeviceHandlerToMap(t *testing.T) {
	type args struct {
		agent *DeviceHandler
	}
	tests := []struct {
		name   string
		fields *fields
		args   args
	}{
		{"delete_device_handler_map-1", mockOlt(), args{newMockDeviceHandler()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			oo.deleteDeviceHandlerToMap(tt.args.agent)
			if len(oo.deviceHandlers) > 0 {
				t.Errorf("delete device manager failed")
			}
		})
	}
}
