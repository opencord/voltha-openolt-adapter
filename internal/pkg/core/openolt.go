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
	"sync"
	"time"

	"github.com/opencord/voltha-lib-go/v5/pkg/adapters/adapterif"
	conf "github.com/opencord/voltha-lib-go/v5/pkg/config"
	"github.com/opencord/voltha-lib-go/v5/pkg/events/eventif"
	"github.com/opencord/voltha-lib-go/v5/pkg/kafka"
	"github.com/opencord/voltha-lib-go/v5/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	"github.com/opencord/voltha-protos/v4/go/extension"
	ic "github.com/opencord/voltha-protos/v4/go/inter_container"
	"github.com/opencord/voltha-protos/v4/go/openflow_13"
	"github.com/opencord/voltha-protos/v4/go/voltha"
)

//OpenOLT structure holds the OLT information
type OpenOLT struct {
	configManager               *conf.ConfigManager
	deviceHandlers              map[string]*DeviceHandler
	coreProxy                   adapterif.CoreProxy
	adapterProxy                adapterif.AdapterProxy
	eventProxy                  eventif.EventProxy
	kafkaICProxy                kafka.InterContainerProxy
	config                      *config.AdapterFlags
	numOnus                     int
	KVStoreAddress              string
	KVStoreType                 string
	exitChannel                 chan int
	HeartbeatCheckInterval      time.Duration
	HeartbeatFailReportInterval time.Duration
	GrpcTimeoutInterval         time.Duration
	lockDeviceHandlersMap       sync.RWMutex
	enableONUStats              bool
	enableGemStats              bool
}

//NewOpenOLT returns a new instance of OpenOLT
func NewOpenOLT(ctx context.Context, kafkaICProxy kafka.InterContainerProxy,
	coreProxy adapterif.CoreProxy, adapterProxy adapterif.AdapterProxy,
	eventProxy eventif.EventProxy, cfg *config.AdapterFlags, cm *conf.ConfigManager) *OpenOLT {
	var openOLT OpenOLT
	openOLT.exitChannel = make(chan int, 1)
	openOLT.deviceHandlers = make(map[string]*DeviceHandler)
	openOLT.kafkaICProxy = kafkaICProxy
	openOLT.config = cfg
	openOLT.numOnus = cfg.OnuNumber
	openOLT.coreProxy = coreProxy
	openOLT.adapterProxy = adapterProxy
	openOLT.eventProxy = eventProxy
	openOLT.KVStoreAddress = cfg.KVStoreAddress
	openOLT.KVStoreType = cfg.KVStoreType
	openOLT.HeartbeatCheckInterval = cfg.HeartbeatCheckInterval
	openOLT.HeartbeatFailReportInterval = cfg.HeartbeatFailReportInterval
	openOLT.GrpcTimeoutInterval = cfg.GrpcTimeoutInterval
	openOLT.lockDeviceHandlersMap = sync.RWMutex{}
	openOLT.configManager = cm
	openOLT.enableONUStats = cfg.EnableONUStats
	openOLT.enableGemStats = cfg.EnableGEMStats
	return &openOLT
}

//Start starts (logs) the device manager
func (oo *OpenOLT) Start(ctx context.Context) error {
	logger.Info(ctx, "starting-device-manager")
	logger.Info(ctx, "device-manager-started")
	return nil
}

//Stop terminates the session
func (oo *OpenOLT) Stop(ctx context.Context) error {
	logger.Info(ctx, "stopping-device-manager")
	oo.exitChannel <- 1
	logger.Info(ctx, "device-manager-stopped")
	return nil
}

func (oo *OpenOLT) addDeviceHandlerToMap(agent *DeviceHandler) {
	oo.lockDeviceHandlersMap.Lock()
	defer oo.lockDeviceHandlersMap.Unlock()
	if _, exist := oo.deviceHandlers[agent.device.Id]; !exist {
		oo.deviceHandlers[agent.device.Id] = agent
	}
}

func (oo *OpenOLT) deleteDeviceHandlerToMap(agent *DeviceHandler) {
	oo.lockDeviceHandlersMap.Lock()
	defer oo.lockDeviceHandlersMap.Unlock()
	delete(oo.deviceHandlers, agent.device.Id)
}

func (oo *OpenOLT) getDeviceHandler(deviceID string) *DeviceHandler {
	oo.lockDeviceHandlersMap.Lock()
	defer oo.lockDeviceHandlersMap.Unlock()
	if agent, ok := oo.deviceHandlers[deviceID]; ok {
		return agent
	}
	return nil
}

// Adopt_device creates a new device handler if not present already and then adopts the device
func (oo *OpenOLT) Adopt_device(ctx context.Context, device *voltha.Device) error {
	if device == nil {
		return olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil).Log()
	}
	logger.Infow(ctx, "adopt-device", log.Fields{"device-id": device.Id})
	var handler *DeviceHandler
	if handler = oo.getDeviceHandler(device.Id); handler == nil {
		handler := NewDeviceHandler(oo.coreProxy, oo.adapterProxy, oo.eventProxy, device, oo, oo.configManager)
		oo.addDeviceHandlerToMap(handler)
		go handler.AdoptDevice(ctx, device)
		// Launch the creation of the device topic
		// go oo.createDeviceTopic(device)
	}
	return nil
}

//Get_ofp_device_info returns OFP information for the given device
func (oo *OpenOLT) Get_ofp_device_info(ctx context.Context, device *voltha.Device) (*ic.SwitchCapability, error) {
	logger.Infow(ctx, "Get_ofp_device_info", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.GetOfpDeviceInfo(device)
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Process_inter_adapter_message sends messages to a target device (between adapters)
func (oo *OpenOLT) Process_inter_adapter_message(ctx context.Context, msg *ic.InterAdapterMessage) error {
	logger.Debugw(ctx, "Process_inter_adapter_message", log.Fields{"msgId": msg.Header.Id})
	targetDevice := msg.Header.ProxyDeviceId // Request?
	if targetDevice == "" && msg.Header.ToDeviceId != "" {
		// Typical response
		targetDevice = msg.Header.ToDeviceId
	}
	if handler := oo.getDeviceHandler(targetDevice); handler != nil {
		return handler.ProcessInterAdapterMessage(ctx, msg)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": targetDevice}, nil)
}

//Process_tech_profile_instance_request processes tech profile request message from onu adapter
func (oo *OpenOLT) Process_tech_profile_instance_request(ctx context.Context, msg *ic.InterAdapterTechProfileInstanceRequestMessage) *ic.InterAdapterTechProfileDownloadMessage {
	logger.Debugw(ctx, "Process_tech_profile_instance_request", log.Fields{"tpPath": msg.TpInstancePath})
	targetDeviceID := msg.ParentDeviceId // Request?
	if targetDeviceID == "" {
		logger.Error(ctx, "device-id-nil")
		return nil
	}
	if handler := oo.getDeviceHandler(targetDeviceID); handler != nil {
		return handler.GetInterAdapterTechProfileDownloadMessage(ctx, msg.TpInstancePath, msg.ParentPonPort, msg.OnuId, msg.UniId)
	}
	return nil
}

//Adapter_descriptor not implemented
func (oo *OpenOLT) Adapter_descriptor(ctx context.Context) error {
	return olterrors.ErrNotImplemented
}

//Device_types unimplemented
func (oo *OpenOLT) Device_types(ctx context.Context) (*voltha.DeviceTypes, error) {
	return nil, olterrors.ErrNotImplemented
}

//Health  returns unimplemented
func (oo *OpenOLT) Health(ctx context.Context) (*voltha.HealthStatus, error) {
	return nil, olterrors.ErrNotImplemented
}

//Reconcile_device unimplemented
func (oo *OpenOLT) Reconcile_device(ctx context.Context, device *voltha.Device) error {
	if device == nil {
		return olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil)
	}
	logger.Infow(ctx, "reconcile-device", log.Fields{"device-id": device.Id})
	var handler *DeviceHandler
	if handler = oo.getDeviceHandler(device.Id); handler == nil {
		handler := NewDeviceHandler(oo.coreProxy, oo.adapterProxy, oo.eventProxy, device, oo, oo.configManager)
		handler.adapterPreviouslyConnected = true
		oo.addDeviceHandlerToMap(handler)
		handler.transitionMap = NewTransitionMap(handler)
		//Setting state to RECONCILING
		err := handler.coreProxy.DeviceStateUpdate(ctx, device.Id, device.ConnectStatus, voltha.OperStatus_RECONCILING)
		if err != nil {
			return err
		}
		handler.transitionMap.Handle(ctx, DeviceInit)
	}
	return nil
}

//Abandon_device unimplemented
func (oo *OpenOLT) Abandon_device(ctx context.Context, device *voltha.Device) error {
	return olterrors.ErrNotImplemented
}

//Disable_device disables the given device
func (oo *OpenOLT) Disable_device(ctx context.Context, device *voltha.Device) error {
	logger.Infow(ctx, "disable-device", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.DisableDevice(ctx, device)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Reenable_device enables the olt device after disable
func (oo *OpenOLT) Reenable_device(ctx context.Context, device *voltha.Device) error {
	logger.Infow(ctx, "reenable-device", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.ReenableDevice(ctx, device)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Reboot_device reboots the given device
func (oo *OpenOLT) Reboot_device(ctx context.Context, device *voltha.Device) error {
	logger.Infow(ctx, "reboot-device", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.RebootDevice(ctx, device)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Self_test_device unimplented
func (oo *OpenOLT) Self_test_device(ctx context.Context, device *voltha.Device) error {
	return olterrors.ErrNotImplemented
}

//Delete_device unimplemented
func (oo *OpenOLT) Delete_device(ctx context.Context, device *voltha.Device) error {
	logger.Infow(ctx, "delete-device", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		if err := handler.DeleteDevice(ctx, device); err != nil {
			logger.Errorw(ctx, "failed-to-handle-delete-device", log.Fields{"device-id": device.Id})
		}
		oo.deleteDeviceHandlerToMap(handler)
		return nil
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Get_device_details unimplemented
func (oo *OpenOLT) Get_device_details(ctx context.Context, device *voltha.Device) error {
	return olterrors.ErrNotImplemented
}

//Update_flows_bulk returns
func (oo *OpenOLT) Update_flows_bulk(ctx context.Context, device *voltha.Device, flows *voltha.Flows, groups *voltha.FlowGroups, flowMetadata *voltha.FlowMetadata) error {
	return olterrors.ErrNotImplemented
}

//Update_flows_incrementally updates (add/remove) the flows on a given device
func (oo *OpenOLT) Update_flows_incrementally(ctx context.Context, device *voltha.Device, flows *openflow_13.FlowChanges, groups *openflow_13.FlowGroupChanges, flowMetadata *voltha.FlowMetadata) error {
	logger.Infow(ctx, "Update_flows_incrementally", log.Fields{"device-id": device.Id, "flows": flows, "flowMetadata": flowMetadata})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.UpdateFlowsIncrementally(ctx, device, flows, groups, flowMetadata)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Update_pm_config returns PmConfigs nil or error
func (oo *OpenOLT) Update_pm_config(ctx context.Context, device *voltha.Device, pmConfigs *voltha.PmConfigs) error {
	logger.Debugw(ctx, "Update_pm_config", log.Fields{"device-id": device.Id, "pm-configs": pmConfigs})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		handler.UpdatePmConfig(ctx, pmConfigs)
		return nil
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Receive_packet_out sends packet out to the device
func (oo *OpenOLT) Receive_packet_out(ctx context.Context, deviceID string, egressPortNo int, packet *openflow_13.OfpPacketOut) error {
	logger.Debugw(ctx, "Receive_packet_out", log.Fields{"device-id": deviceID, "egress_port_no": egressPortNo, "pkt": packet})
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		return handler.PacketOut(ctx, egressPortNo, packet)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": deviceID}, nil)
}

//Suppress_event unimplemented
func (oo *OpenOLT) Suppress_event(ctx context.Context, filter *voltha.EventFilter) error {
	return olterrors.ErrNotImplemented
}

//Unsuppress_event  unimplemented
func (oo *OpenOLT) Unsuppress_event(ctx context.Context, filter *voltha.EventFilter) error {
	return olterrors.ErrNotImplemented
}

//Download_image unimplemented
func (oo *OpenOLT) Download_image(ctx context.Context, device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//Get_image_download_status unimplemented
func (oo *OpenOLT) Get_image_download_status(ctx context.Context, device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//Cancel_image_download unimplemented
func (oo *OpenOLT) Cancel_image_download(ctx context.Context, device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//Activate_image_update unimplemented
func (oo *OpenOLT) Activate_image_update(ctx context.Context, device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//Revert_image_update unimplemented
func (oo *OpenOLT) Revert_image_update(ctx context.Context, device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//Download_onu_image unimplemented
func (oo *OpenOLT) Download_onu_image(ctx context.Context, request *voltha.DeviceImageDownloadRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

//Get_onu_image_status unimplemented
func (oo *OpenOLT) Get_onu_image_status(ctx context.Context, in *voltha.DeviceImageRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

//Abort_onu_image_upgrade unimplemented
func (oo *OpenOLT) Abort_onu_image_upgrade(ctx context.Context, in *voltha.DeviceImageRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

//Get_onu_images unimplemented
func (oo *OpenOLT) Get_onu_images(ctx context.Context, deviceID string) (*voltha.OnuImages, error) {
	return nil, olterrors.ErrNotImplemented
}

//Activate_onu_image unimplemented
func (oo *OpenOLT) Activate_onu_image(ctx context.Context, in *voltha.DeviceImageRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

//Commit_onu_image unimplemented
func (oo *OpenOLT) Commit_onu_image(ctx context.Context, in *voltha.DeviceImageRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

// Enable_port to Enable PON/NNI interface
func (oo *OpenOLT) Enable_port(ctx context.Context, deviceID string, port *voltha.Port) error {
	logger.Infow(ctx, "Enable_port", log.Fields{"device-id": deviceID, "port": port})
	return oo.enableDisablePort(ctx, deviceID, port, true)
}

// Disable_port to Disable pon/nni interface
func (oo *OpenOLT) Disable_port(ctx context.Context, deviceID string, port *voltha.Port) error {
	logger.Infow(ctx, "Disable_port", log.Fields{"device-id": deviceID, "port": port})
	return oo.enableDisablePort(ctx, deviceID, port, false)
}

// enableDisablePort to Disable pon or Enable PON interface
func (oo *OpenOLT) enableDisablePort(ctx context.Context, deviceID string, port *voltha.Port, enablePort bool) error {
	logger.Infow(ctx, "enableDisablePort", log.Fields{"device-id": deviceID, "port": port})
	if port == nil {
		return olterrors.NewErrInvalidValue(log.Fields{
			"reason":    "port cannot be nil",
			"device-id": deviceID,
			"port":      nil}, nil)
	}
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		logger.Debugw(ctx, "Enable_Disable_Port", log.Fields{"device-id": deviceID, "port": port})
		if enablePort {
			if err := handler.EnablePort(ctx, port); err != nil {
				return olterrors.NewErrAdapter("error-occurred-during-enable-port", log.Fields{"device-id": deviceID, "port": port}, err)
			}
		} else {
			if err := handler.DisablePort(ctx, port); err != nil {
				return olterrors.NewErrAdapter("error-occurred-during-disable-port", log.Fields{"device-id": deviceID, "port": port}, err)
			}
		}
	}
	return nil
}

//Child_device_lost deletes the ONU and its references from PONResources
func (oo *OpenOLT) Child_device_lost(ctx context.Context, childDevice *voltha.Device) error {
	logger.Infow(ctx, "Child-device-lost", log.Fields{"parent-device-id": childDevice.ParentId, "child-device-id": childDevice.Id})
	if handler := oo.getDeviceHandler(childDevice.ParentId); handler != nil {
		return handler.ChildDeviceLost(ctx, childDevice.ParentPortNo, childDevice.ProxyAddress.OnuId, childDevice.SerialNumber)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": childDevice.ParentId}, nil).Log()
}

//Start_omci_test not implemented
func (oo *OpenOLT) Start_omci_test(ctx context.Context, device *voltha.Device, request *voltha.OmciTestRequest) (*voltha.TestResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

//Get_ext_value retrieves a value on a particular ONU
func (oo *OpenOLT) Get_ext_value(ctx context.Context, deviceID string, device *voltha.Device, valueparam voltha.ValueType_Type) (*voltha.ReturnValues, error) {
	var err error
	resp := new(voltha.ReturnValues)
	logger.Infow(ctx, "Get_ext_value", log.Fields{"device-id": deviceID, "onu-id": device.Id})
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		if resp, err = handler.getExtValue(ctx, device, valueparam); err != nil {
			logger.Errorw(ctx, "error-occurred-during-get-ext-value", log.Fields{"device-id": deviceID, "onu-id": device.Id,
				"error": err})
			return nil, err
		}
	}
	return resp, nil
}

//Single_get_value_request handles get uni status on ONU and ondemand metric on OLT
func (oo *OpenOLT) Single_get_value_request(ctx context.Context, request extension.SingleGetValueRequest) (*extension.SingleGetValueResponse, error) {
	logger.Infow(ctx, "Single_get_value_request", log.Fields{"request": request})

	errResp := func(status extension.GetValueResponse_Status,
		reason extension.GetValueResponse_ErrorReason) *extension.SingleGetValueResponse {
		return &extension.SingleGetValueResponse{
			Response: &extension.GetValueResponse{
				Status:    status,
				ErrReason: reason,
			},
		}
	}
	if handler := oo.getDeviceHandler(request.TargetId); handler != nil {
		switch reqType := request.GetRequest().GetRequest().(type) {
		case *extension.GetValueRequest_OltPortInfo:
			return handler.getOltPortCounters(ctx, reqType.OltPortInfo), nil
		case *extension.GetValueRequest_OnuPonInfo:
			return handler.getOnuPonCounters(ctx, reqType.OnuPonInfo), nil
		case *extension.GetValueRequest_RxPower:
			return handler.getRxPower(ctx, reqType.RxPower), nil
		default:
			return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_UNSUPPORTED), nil
		}
	}

	logger.Infow(ctx, "Single_get_value_request failed ", log.Fields{"request": request})
	return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_DEVICE_ID), nil
}
