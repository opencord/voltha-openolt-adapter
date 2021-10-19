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
	"reflect"
	"testing"

	conf "github.com/opencord/voltha-lib-go/v7/pkg/config"
	vgrpc "github.com/opencord/voltha-lib-go/v7/pkg/grpc"

	"github.com/opencord/voltha-lib-go/v7/pkg/events"
	fu "github.com/opencord/voltha-lib-go/v7/pkg/flows"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	ca "github.com/opencord/voltha-protos/v5/go/core_adapter"
	"github.com/opencord/voltha-protos/v5/go/openflow_13"
	ofp "github.com/opencord/voltha-protos/v5/go/openflow_13"
	"github.com/opencord/voltha-protos/v5/go/voltha"
)

// mocks the OpenOLT struct.
type fields struct {
	deviceHandlers map[string]*DeviceHandler
	coreClient     *vgrpc.Client
	eventProxy     *events.EventProxy
	numOnus        int
	KVStoreAddress string
	KVStoreType    string
	exitChannel    chan int
	ctx            context.Context
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
		eventProxy:     testOlt.eventProxy,
		numOnus:        testOlt.numOnus,
		KVStoreAddress: testOlt.KVStoreAddress,
		KVStoreType:    testOlt.KVStoreType,
		exitChannel:    testOlt.exitChannel,
	}
}

// mockDevice mocks Device.
func mockDevice() *voltha.Device {
	return &voltha.Device{
		Id:       "olt",
		Root:     true,
		ParentId: "logical_device",
		ProxyAddress: &voltha.Device_ProxyAddress{
			DeviceId:       "olt",
			DeviceType:     "onu",
			ChannelId:      1,
			ChannelGroupId: 1,
		},
		ConnectStatus: 1,
	}
}

func TestNewOpenOLT(t *testing.T) {
	tests := []struct {
		name        string
		fields      *fields
		configFlags *config.AdapterFlags
		cm          *conf.ConfigManager
		want        *OpenOLT
	}{
		{"newopenolt-1", &fields{}, &config.AdapterFlags{OnuNumber: 1, KVStoreAddress: "1.1.1.1:1", KVStoreType: "etcd"}, &conf.ConfigManager{},
			&OpenOLT{numOnus: 1, KVStoreAddress: "1.1.1.1:1", KVStoreType: "etcd"}},
		{"newopenolt-2", &fields{}, &config.AdapterFlags{OnuNumber: 2, KVStoreAddress: "2.2.2.2:2", KVStoreType: "etcd"}, &conf.ConfigManager{},
			&OpenOLT{numOnus: 2, KVStoreAddress: "2.2.2.2:2", KVStoreType: "etcd"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewOpenOLT(tt.fields.ctx, tt.fields.coreClient,
				tt.fields.eventProxy, tt.configFlags, tt.cm); reflect.TypeOf(got) != reflect.TypeOf(tt.want) && got != nil {
				t.Errorf("NewOpenOLT() error = %v, wantErr %v", got, tt.want)
			}
		})
	}
}

func TestOpenOLT_ActivateImageUpdate(t *testing.T) {
	type args struct {
		request *ca.ImageDownloadMessage
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
			got, err := oo.ActivateImageUpdate(context.Background(), tt.args.request)
			if err != tt.wantErr && got == nil {
				t.Errorf("Activate_image_update() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_AdoptDevice(t *testing.T) {
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
			_, err := oo.AdoptDevice(context.Background(), tt.args.device)
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

func TestOpenOLT_CancelImageDownload(t *testing.T) {
	type args struct {
		request *ca.ImageDownloadMessage
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
			got, err := oo.CancelImageDownload(context.Background(), tt.args.request)
			if err != tt.wantErr && got == nil {
				t.Errorf("Cancel_image_download() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_DeleteDevice(t *testing.T) {
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
			if _, err := oo.DeleteDevice(context.Background(), tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("Delete_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_DisableDevice(t *testing.T) {
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
			if _, err := oo.DisableDevice(context.Background(), tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("Disable_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_DownloadImage(t *testing.T) {
	type args struct {
		request *ca.ImageDownloadMessage
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
			got, err := oo.DownloadImage(context.Background(), tt.args.request)
			if err != tt.wantErr && got == nil {
				t.Errorf("Download_image() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_GetImageDownloadStatus(t *testing.T) {
	type args struct {
		request *ca.ImageDownloadMessage
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
			got, err := oo.GetImageDownloadStatus(context.Background(), tt.args.request)
			if err != tt.wantErr && got == nil {
				t.Errorf("Get_image_download_status() got = %v want = %v error = %v, wantErr %v",
					got, tt.want, err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_GetOfpDeviceInfo(t *testing.T) {
	type args struct {
		device *voltha.Device
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		want    *ca.SwitchCapability
		wantErr error
	}{
		{"get_ofp_device_info-1", mockOlt(), args{mockDevice()}, &ca.SwitchCapability{
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
			got, err := oo.GetOfpDeviceInfo(context.Background(), tt.args.device)
			if !reflect.DeepEqual(err, tt.wantErr) || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Get_ofp_device_info() got = %v want = %v error = %v, wantErr = %v",
					got, tt.want, err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_RebootDevice(t *testing.T) {
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
			if _, err := oo.RebootDevice(context.Background(), tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("Reboot_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_SendPacketOut(t *testing.T) {
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
			if _, err := oo.SendPacketOut(context.Background(), &ca.PacketOut{
				DeviceId:     tt.args.deviceID,
				EgressPortNo: uint32(tt.args.egressPortNo),
				Packet:       tt.args.packet}); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("Receive_packet_out() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_ReconcileDevice(t *testing.T) {
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
			if _, err := oo.ReconcileDevice(context.Background(), tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("Reconcile_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_ReEnableDevice(t *testing.T) {
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
			if _, err := oo.ReEnableDevice(context.Background(), tt.args.device); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("Reenable_device() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_RevertImageUpdate(t *testing.T) {
	type args struct {
		request *ca.ImageDownloadMessage
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
			got, err := oo.RevertImageUpdate(context.Background(), tt.args.request)
			if err != tt.wantErr && got == nil {
				t.Log("error :", err)
			}
		})
	}
}

func TestOpenOLT_SelfTestDevice(t *testing.T) {
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
			if _, err := oo.SelfTestDevice(context.Background(), tt.args.device); err != tt.wantErr {
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
			if err := oo.Start(tt.args.ctx); err != nil {
				t.Error(err)
			}
			if err := oo.Stop(tt.args.ctx); err != nil {
				t.Errorf("Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_SuppressEvent(t *testing.T) {
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
			if _, err := oo.SuppressEvent(context.Background(), tt.args.filter); err != tt.wantErr {
				t.Errorf("Suppress_event() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_UnSuppressEvent(t *testing.T) {
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
			if _, err := oo.UnSuppressEvent(context.Background(), tt.args.filter); err != tt.wantErr {
				t.Errorf("Unsuppress_event() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_UpdateFlowsBulk(t *testing.T) {
	type args struct {
		device       *voltha.Device
		flows        *ofp.Flows
		groups       *ofp.FlowGroups
		flowMetadata *ofp.FlowMetadata
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
			if _, err := oo.UpdateFlowsBulk(context.Background(), &ca.BulkFlows{
				Device:       tt.args.device,
				Flows:        tt.args.flows,
				Groups:       tt.args.groups,
				FlowMetadata: tt.args.flowMetadata,
			}); err != tt.wantErr {
				t.Errorf("Update_flows_bulk() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_UpdateFlowsIncrementally(t *testing.T) {
	type args struct {
		device       *voltha.Device
		flows        *openflow_13.FlowChanges
		groups       *openflow_13.FlowGroupChanges
		flowMetadata *ofp.FlowMetadata
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
			if _, err := oo.UpdateFlowsIncrementally(context.Background(), &ca.IncrementalFlows{
				Device:       tt.args.device,
				Flows:        tt.args.flows,
				Groups:       tt.args.groups,
				FlowMetadata: tt.args.flowMetadata}); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("Update_flows_incrementally() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_UpdatePmConfig(t *testing.T) {
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
		{"update_pm_config-1", mockOlt(), args{device: mockDevice(), pmConfigs: &voltha.PmConfigs{DefaultFreq: 150, Grouped: false, FreqOverride: false}}, nil},
		{"update_pm_config-2", &fields{}, args{device: mockDevice(), pmConfigs: &voltha.PmConfigs{DefaultFreq: 150, Grouped: false, FreqOverride: false}}, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": "olt"}, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if _, err := oo.UpdatePmConfig(context.Background(), &ca.PmConfigsInfo{DeviceId: tt.args.device.Id, PmConfigs: tt.args.pmConfigs}); !reflect.DeepEqual(err, tt.wantErr) {
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

func TestOpenOLT_EnablePort(t *testing.T) {
	type args struct {
		port *voltha.Port
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{"Enable_port-1", mockOlt(), args{port: &voltha.Port{Type: voltha.Port_PON_OLT, PortNo: 1, DeviceId: "olt"}}, false},
		{"Enable_port-2", mockOlt(), args{port: &voltha.Port{Type: voltha.Port_ETHERNET_NNI, PortNo: 1, DeviceId: "olt"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if _, err := oo.EnablePort(context.Background(), tt.args.port); (err != nil) != tt.wantErr {
				t.Errorf("OpenOLT.Enable_port() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenOLT_DisablePort(t *testing.T) {
	type args struct {
		port *voltha.Port
	}
	tests := []struct {
		name    string
		fields  *fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{"Disable_port-1", mockOlt(), args{port: &voltha.Port{Type: voltha.Port_PON_OLT, PortNo: 1, DeviceId: "olt"}}, false},
		{"Disable_port-2", mockOlt(), args{port: &voltha.Port{Type: voltha.Port_ETHERNET_NNI, PortNo: 1, DeviceId: "olt"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oo := testOltObject(tt.fields)
			if _, err := oo.DisablePort(context.Background(), tt.args.port); (err != nil) != tt.wantErr {
				t.Errorf("OpenOLT.Disable_port() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
