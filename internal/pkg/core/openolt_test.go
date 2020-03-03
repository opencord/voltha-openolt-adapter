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

//Package core provides the utility for olt devices, flows and statistics
package core

import (
	"context"
	"errors"
	com "github.com/opencord/voltha-lib-go/v3/pkg/adapters/common"
	fu "github.com/opencord/voltha-lib-go/v3/pkg/flows"
	"github.com/opencord/voltha-lib-go/v3/pkg/kafka"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	ic "github.com/opencord/voltha-protos/v3/go/inter_container"
	"github.com/opencord/voltha-protos/v3/go/openflow_13"
	ofp "github.com/opencord/voltha-protos/v3/go/openflow_13"
	"github.com/opencord/voltha-protos/v3/go/voltha"
	"reflect"
	"sync"
	"testing"
)

// mocks the OpenOLT struct.
type fields struct {
	deviceHandlers        map[string]*DeviceHandler
	coreProxy             *com.CoreProxy
	adapterProxy          *com.AdapterProxy
	eventProxy            *com.EventProxy
	kafkaICProxy          kafka.InterContainerProxy
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
		name        string
		fields      *fields
		configFlags *config.AdapterFlags
		want        *OpenOLT
	}{
		{"newopenolt-1", &fields{}, &config.AdapterFlags{OnuNumber: 1, KVStorePort: 1, KVStoreType: "consul", KVStoreHost: "1.1.1.1"},
			&OpenOLT{numOnus: 1, KVStorePort: 1, KVStoreType: "consul", KVStoreHost: "1.1.1.1"}},
		{"newopenolt-2", &fields{}, &config.AdapterFlags{OnuNumber: 2, KVStorePort: 2, KVStoreType: "etcd", KVStoreHost: "2.2.2.2"},
			&OpenOLT{numOnus: 2, KVStorePort: 2, KVStoreType: "etcd", KVStoreHost: "2.2.2.2"}},
		{"newopenolt-3", &fields{}, &config.AdapterFlags{OnuNumber: 3, KVStorePort: 3, KVStoreType: "consul", KVStoreHost: "3.3.3.3"},
			&OpenOLT{numOnus: 3, KVStorePort: 3, KVStoreType: "consul", KVStoreHost: "3.3.3.3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewOpenOLT(tt.fields.ctx, tt.fields.kafkaICProxy, tt.fields.coreProxy, tt.fields.adapterProxy,
				tt.fields.eventProxy, tt.configFlags); reflect.TypeOf(got) != reflect.TypeOf(tt.want) && got != nil {
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
		{"abandon_device-1", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"abandon_device-2", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"abandon_device-3", &fields{}, args{}, olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Abandon_device(tt.args.device); err != tt.wantErr {
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
			olterrors.ErrNotImplemented},
		{"activate_image_upate-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123CDE"},
			olterrors.ErrNotImplemented},
		{"activate_image_upate-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image3-ABC123EFG"},
			olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Activate_image_update(tt.args.device, tt.args.request)
			if err != tt.wantErr && got == nil {
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
		{"adapter_descriptor-1", &fields{}, olterrors.ErrNotImplemented},
		{"adapter_descriptor-2", &fields{}, olterrors.ErrNotImplemented},
		{"adapter_descriptor-3", &fields{}, olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Adapter_descriptor(); err != tt.wantErr {
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
	device.Id = "olt"
	nilDevice := olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil)
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"adopt_device-1", mockOlt(), args{}, nilDevice},
		{"adopt_device-2", mockOlt(), args{device}, nilDevice},
		{"adopt_device-3", mockOlt(), args{mockDevice()}, nil},
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
			olterrors.ErrNotImplemented},
		{"cancel_image_download-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123IJK"},
			olterrors.ErrNotImplemented},
		{"cancel_image_download-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image3-ABC123KLM"},
			olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Cancel_image_download(tt.args.device, tt.args.request)
			if err != tt.wantErr && got == nil {
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
		{"delete_device-1", &fields{}, args{mockDevice()},
			olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": "olt"}, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Delete_device(tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
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
			olterrors.ErrNotImplemented},
		{"device_types-2", &fields{}, &voltha.DeviceTypes{},
			olterrors.ErrNotImplemented},
		{"device_types-3", &fields{}, &voltha.DeviceTypes{},
			olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Device_types()
			if err != tt.wantErr && got == nil {
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
		{"disable_device-1", mockOlt(), args{mockDevice()}, nil},
		{"disable_device-2", &fields{}, args{mockDevice()},
			olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": "olt"}, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Disable_device(tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
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
			olterrors.ErrNotImplemented},
		{"download_image-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123LKJ"},
			olterrors.ErrNotImplemented},
		{"download_image-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image1-ABC123RTY"},
			olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Download_image(tt.args.device, tt.args.request)
			if err != tt.wantErr && got == nil {
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
		{"get_device_details-1", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"get_device_details-2", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"get_device_details-3", &fields{}, args{}, olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Get_device_details(tt.args.device); err != tt.wantErr {
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
			olterrors.ErrNotImplemented},
		{"get_image_download_status-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123LKJ"},
			olterrors.ErrNotImplemented},
		{"get_image_download_status-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image1-ABC123DFG"},
			olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Get_image_download_status(tt.args.device, tt.args.request)
			if err != tt.wantErr && got == nil {
				t.Errorf("Get_image_download_status() got = %v want = %v error = %v, wantErr %v",
					got, tt.want, err, tt.wantErr)
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
		{"get_ofp_device_info-1", mockOlt(), args{mockDevice()}, &ic.SwitchCapability{
			Desc: &openflow_13.OfpDesc{
				MfrDesc: "VOLTHA Project",
				HwDesc:  "open_pon",
				SwDesc:  "open_pon",
			},
			SwitchFeatures: &openflow_13.OfpSwitchFeatures{
				NBuffers:     uint32(256),
				NTables:      uint32(2),
				Capabilities: uint32(15),
			},
		}, nil},
		{"get_ofp_device_info-2", &fields{}, args{mockDevice()}, nil,
			olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": "olt"}, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Get_ofp_device_info(tt.args.device)
			if !reflect.DeepEqual(err, tt.wantErr) || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Get_ofp_device_info() got = %v want = %v error = %v, wantErr = %v",
					got, tt.want, err, tt.wantErr)
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
		{"get_ofp_port_info-1", mockOlt(), args{mockDevice(), 1}, &ic.PortCapability{
			Port: &voltha.LogicalPort{
				DeviceId:     "olt",
				DevicePortNo: uint32(1),
				OfpPort: &openflow_13.OfpPort{
					HwAddr:     []uint32{1, 2, 3, 4, 5, 6},
					State:      uint32(4),
					Curr:       uint32(4128),
					Advertised: uint32(4128),
					Peer:       uint32(4128),
					CurrSpeed:  uint32(32),
					MaxSpeed:   uint32(32),
				},
			},
		}, nil},
		{"get_ofp_port_info-2", &fields{}, args{mockDevice(), 1}, nil,
			olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": "olt"}, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Get_ofp_port_info(tt.args.device, tt.args.portNo)
			if !reflect.DeepEqual(err, tt.wantErr) || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Get_ofp_port_info() got = %v want = %v error = %v, wantErr = %v",
					got, tt.want, err, tt.wantErr)
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
		{"health-1", &fields{}, &voltha.HealthStatus{}, olterrors.ErrNotImplemented},
		{"health-2", &fields{}, &voltha.HealthStatus{}, olterrors.ErrNotImplemented},
		{"health-3", &fields{}, &voltha.HealthStatus{}, olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Health()
			if err != tt.wantErr && got == nil {
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
				Type:          ic.InterAdapterMessageType_OMCI_REQUEST,
			},
		},
	}
	var message3 = args{
		msg: &ic.InterAdapterMessage{
			Header: &ic.InterAdapterHeader{
				Id:            "olt",
				ProxyDeviceId: "olt",
				ToDeviceId:    "olt",
				Type:          ic.InterAdapterMessageType_FLOW_REQUEST,
			},
		},
	}
	tests := []struct {
		name        string
		fields      *fields
		args        args
		wantErrType reflect.Type
	}{
		{"process_inter_adaptor_messgae-1", mockOlt(), message1,
			reflect.TypeOf(&olterrors.ErrNotFound{})},
		{"process_inter_adaptor_messgae-2", mockOlt(), message2,
			reflect.TypeOf(errors.New("message is nil"))},
		{"process_inter_adaptor_messgae-3", mockOlt(), message3,
			reflect.TypeOf(&olterrors.ErrInvalidValue{})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Process_inter_adapter_message(tt.args.msg); reflect.TypeOf(err) != tt.wantErrType {
				t.Errorf("Process_inter_adapter_message() error = %v, wantErr %v",
					reflect.TypeOf(err), tt.wantErrType)
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
		{"reboot_device-1", mockOlt(), args{mockDevice()}, nil},
		{"reboot_device-2", &fields{}, args{mockDevice()},
			olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": "olt"}, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Reboot_device(tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
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
		{"receive_packet_out-1", mockOlt(), args{mockDevice().Id, 1, pktout}, nil},
		{"receive_packet_out-2", mockOlt(), args{"1234", 1, pktout},
			olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": "1234"}, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Receive_packet_out(tt.args.deviceID, tt.args.egressPortNo, tt.args.packet); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("Receive_packet_out() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Reconcile_device(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	expectedError := olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil)
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"reconcile_device-1", &fields{}, args{}, expectedError},
		{"reconcile_device-2", &fields{}, args{}, expectedError},
		{"reconcile_device-3", &fields{}, args{}, expectedError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Reconcile_device(tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
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
		{"reenable_device-1", mockOlt(), args{mockDevice()}, nil},
		{"reenable_device-2", &fields{}, args{mockDevice()},
			olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": "olt"}, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Reenable_device(tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
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
			olterrors.ErrNotImplemented},
		{"revert_image_update-2", &fields{}, args{}, &voltha.ImageDownload{Id: "Image2-ABC123TYU"},
			olterrors.ErrNotImplemented},
		{"revert_image_update-3", &fields{}, args{}, &voltha.ImageDownload{Id: "Image3-ABC123GTH"},
			olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			got, err := oo.Revert_image_update(tt.args.device, tt.args.request)
			if err != tt.wantErr && got == nil {
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
		{"self_test_device-1", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"self_test_device-2", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"self_test_device-3", &fields{}, args{}, olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Self_test_device(tt.args.device); err != tt.wantErr {
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

func TestOpenOLT_Suppress_event(t *testing.T) {
	type args struct {
		filter *voltha.EventFilter
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"suppress_event-1", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"suppress_event-2", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"suppress_event-3", &fields{}, args{}, olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Suppress_event(tt.args.filter); err != tt.wantErr {
				t.Errorf("Suppress_event() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Unsuppress_event(t *testing.T) {
	type args struct {
		filter *voltha.EventFilter
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr error
	}{
		{"unsupress_event-1", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"unsupress_event-2", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"unsupress_event-3", &fields{}, args{}, olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Unsuppress_event(tt.args.filter); err != tt.wantErr {
				t.Errorf("Unsuppress_event() error = %v, wantErr %v", err, tt.wantErr)
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
		{"update_flows_bulk-1", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"update_flows_bulk-2", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"update_flows_bulk-3", &fields{}, args{}, olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Update_flows_bulk(tt.args.device, tt.args.flows, tt.args.groups, tt.args.flowMetadata); err != tt.wantErr {
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
			olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": "olt"}, nil)},
		{"update_flows_incrementally-2", mockOlt(), args{device: mockDevice()}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Update_flows_incrementally(tt.args.device, tt.args.flows, tt.args.groups, tt.args.flowMetadata); !reflect.DeepEqual(err, tt.wantErr) {
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
		{"update_pm_config-1", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"update_pm_config-2", &fields{}, args{}, olterrors.ErrNotImplemented},
		{"update_pm_config-3", &fields{}, args{}, olterrors.ErrNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Update_pm_config(tt.args.device, tt.args.pmConfigs); err != tt.wantErr {
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

func TestOpenOLT_Enable_port(t *testing.T) {
	type args struct {
		deviceID string
		port     *voltha.Port
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{"Enable_port-1", mockOlt(), args{deviceID: "olt", port: &voltha.Port{Type: voltha.Port_PON_OLT, PortNo: 1}}, false},
		{"Enable_port-2", mockOlt(), args{deviceID: "olt", port: &voltha.Port{Type: voltha.Port_ETHERNET_NNI, PortNo: 1}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Enable_port(tt.args.deviceID, tt.args.port); (err != nil) != tt.wantErr {
				t.Errorf("OpenOLT.Enable_port() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_Disable_port(t *testing.T) {
	type args struct {
		deviceID string
		port     *voltha.Port
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{"Disable_port-1", mockOlt(), args{deviceID: "olt", port: &voltha.Port{Type: voltha.Port_PON_OLT, PortNo: 1}}, false},
		{"Disable_port-2", mockOlt(), args{deviceID: "olt", port: &voltha.Port{Type: voltha.Port_ETHERNET_NNI, PortNo: 1}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if err := oo.Disable_port(tt.args.deviceID, tt.args.port); (err != nil) != tt.wantErr {
				t.Errorf("OpenOLT.Disable_port() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
