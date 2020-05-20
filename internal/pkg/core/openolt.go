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

	"github.com/opencord/voltha-lib-go/v3/pkg/adapters/adapterif"
	"github.com/opencord/voltha-lib-go/v3/pkg/kafka"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	ic "github.com/opencord/voltha-protos/v3/go/inter_container"
	"github.com/opencord/voltha-protos/v3/go/openflow_13"
	"github.com/opencord/voltha-protos/v3/go/voltha"
)

//OpenOLT structure holds the OLT information
type OpenOLT struct {
	deviceHandlers              map[string]*DeviceHandler
	coreProxy                   adapterif.CoreProxy
	adapterProxy                adapterif.AdapterProxy
	eventProxy                  adapterif.EventProxy
	kafkaICProxy                kafka.InterContainerProxy
	config                      *config.AdapterFlags
	numOnus                     int
	KVStoreHost                 string
	KVStorePort                 int
	KVStoreType                 string
	exitChannel                 chan int
	HeartbeatCheckInterval      time.Duration
	HeartbeatFailReportInterval time.Duration
	GrpcTimeoutInterval         time.Duration
	lockDeviceHandlersMap       sync.RWMutex
}

//NewOpenOLT returns a new instance of OpenOLT
func NewOpenOLT(ctx context.Context, kafkaICProxy kafka.InterContainerProxy,
	coreProxy adapterif.CoreProxy, adapterProxy adapterif.AdapterProxy,
	eventProxy adapterif.EventProxy, cfg *config.AdapterFlags) *OpenOLT {
	var openOLT OpenOLT
	openOLT.exitChannel = make(chan int, 1)
	openOLT.deviceHandlers = make(map[string]*DeviceHandler)
	openOLT.kafkaICProxy = kafkaICProxy
	openOLT.config = cfg
	openOLT.numOnus = cfg.OnuNumber
	openOLT.coreProxy = coreProxy
	openOLT.adapterProxy = adapterProxy
	openOLT.eventProxy = eventProxy
	openOLT.KVStoreHost = cfg.KVStoreHost
	openOLT.KVStorePort = cfg.KVStorePort
	openOLT.KVStoreType = cfg.KVStoreType
	openOLT.HeartbeatCheckInterval = cfg.HeartbeatCheckInterval
	openOLT.HeartbeatFailReportInterval = cfg.HeartbeatFailReportInterval
	openOLT.GrpcTimeoutInterval = cfg.GrpcTimeoutInterval
	openOLT.lockDeviceHandlersMap = sync.RWMutex{}
	return &openOLT
}

//Start starts (logs) the device manager
func (oo *OpenOLT) Start(ctx context.Context) error {
	logger.Info("starting-device-manager")
	logger.Info("device-manager-started")
	return nil
}

//Stop terminates the session
func (oo *OpenOLT) Stop(ctx context.Context) error {
	logger.Info("stopping-device-manager")
	oo.exitChannel <- 1
	logger.Info("device-manager-stopped")
	return nil
}

func sendResponse(ctx context.Context, ch chan interface{}, result interface{}) {
	if ctx.Err() == nil {
		// Returned response only of the ctx has not been canceled/timeout/etc
		// Channel is automatically closed when a context is Done
		ch <- result
		logger.Debugw("sendResponse", log.Fields{"result": result})
	} else {
		// Should the transaction be reverted back?
		logger.Debugw("sendResponse-context-error", log.Fields{"context-error": ctx.Err()})
	}
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

//createDeviceTopic returns
func (oo *OpenOLT) createDeviceTopic(device *voltha.Device) error {
	logger.Infow("create-device-topic", log.Fields{"deviceId": device.Id})
	defaultTopic := oo.kafkaICProxy.GetDefaultTopic()
	deviceTopic := kafka.Topic{Name: defaultTopic.Name + "_" + device.Id}
	// TODO for the offset
	if err := oo.kafkaICProxy.SubscribeWithDefaultRequestHandler(deviceTopic, 0); err != nil {
		return olterrors.NewErrAdapter("subscribe-for-device-topic-failed", log.Fields{"device-topic": deviceTopic}, err)
	}
	return nil
}

// Adopt_device creates a new device handler if not present already and then adopts the device
func (oo *OpenOLT) Adopt_device(device *voltha.Device) error {
	ctx := context.Background()
	if device == nil {
		return olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil).Log()
	}
	logger.Infow("adopt-device", log.Fields{"deviceId": device.Id})
	var handler *DeviceHandler
	if handler = oo.getDeviceHandler(device.Id); handler == nil {
		handler := NewDeviceHandler(oo.coreProxy, oo.adapterProxy, oo.eventProxy, device, oo)
		oo.addDeviceHandlerToMap(handler)
		go handler.AdoptDevice(ctx, device)
		// Launch the creation of the device topic
		// go oo.createDeviceTopic(device)
	}
	return nil
}

//Get_ofp_device_info returns OFP information for the given device
func (oo *OpenOLT) Get_ofp_device_info(device *voltha.Device) (*ic.SwitchCapability, error) {
	logger.Infow("Get_ofp_device_info", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.GetOfpDeviceInfo(device)
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Get_ofp_port_info returns OFP port information for the given device
func (oo *OpenOLT) Get_ofp_port_info(device *voltha.Device, portNo int64) (*ic.PortCapability, error) {
	logger.Infow("Get_ofp_port_info", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.GetOfpPortInfo(device, portNo)
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Process_inter_adapter_message sends messages to a target device (between adapters)
func (oo *OpenOLT) Process_inter_adapter_message(msg *ic.InterAdapterMessage) error {
	logger.Debugw("Process_inter_adapter_message", log.Fields{"msgId": msg.Header.Id})
	targetDevice := msg.Header.ProxyDeviceId // Request?
	if targetDevice == "" && msg.Header.ToDeviceId != "" {
		// Typical response
		targetDevice = msg.Header.ToDeviceId
	}
	if handler := oo.getDeviceHandler(targetDevice); handler != nil {
		return handler.ProcessInterAdapterMessage(msg)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": targetDevice}, nil)
}

//Adapter_descriptor not implemented
func (oo *OpenOLT) Adapter_descriptor() error {
	return olterrors.ErrNotImplemented
}

//Device_types unimplemented
func (oo *OpenOLT) Device_types() (*voltha.DeviceTypes, error) {
	return nil, olterrors.ErrNotImplemented
}

//Health  returns unimplemented
func (oo *OpenOLT) Health() (*voltha.HealthStatus, error) {
	return nil, olterrors.ErrNotImplemented
}

//Reconcile_device unimplemented
func (oo *OpenOLT) Reconcile_device(device *voltha.Device) error {
	ctx := context.Background()
	if device == nil {
		return olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil)
	}
	logger.Infow("reconcile-device", log.Fields{"deviceId": device.Id})
	var handler *DeviceHandler
	if handler = oo.getDeviceHandler(device.Id); handler == nil {
		handler := NewDeviceHandler(oo.coreProxy, oo.adapterProxy, oo.eventProxy, device, oo)
		oo.addDeviceHandlerToMap(handler)
		handler.transitionMap = NewTransitionMap(handler)
		handler.transitionMap.Handle(ctx, DeviceInit)
	}
	return nil
}

//Abandon_device unimplemented
func (oo *OpenOLT) Abandon_device(device *voltha.Device) error {
	return olterrors.ErrNotImplemented
}

//Disable_device disables the given device
func (oo *OpenOLT) Disable_device(device *voltha.Device) error {
	logger.Infow("disable-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.DisableDevice(device)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Reenable_device enables the olt device after disable
func (oo *OpenOLT) Reenable_device(device *voltha.Device) error {
	logger.Infow("reenable-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.ReenableDevice(device)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Reboot_device reboots the given device
func (oo *OpenOLT) Reboot_device(device *voltha.Device) error {
	logger.Infow("reboot-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.RebootDevice(device)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Self_test_device unimplented
func (oo *OpenOLT) Self_test_device(device *voltha.Device) error {
	return olterrors.ErrNotImplemented
}

//Delete_device unimplemented
func (oo *OpenOLT) Delete_device(device *voltha.Device) error {
	logger.Infow("delete-device", log.Fields{"deviceId": device.Id})
	ctx := context.Background()
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		if err := handler.DeleteDevice(ctx, device); err != nil {
			logger.Errorw("failed-to-handle-delete-device", log.Fields{"device-id": device.Id})
		}
		oo.deleteDeviceHandlerToMap(handler)
		return nil
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Get_device_details unimplemented
func (oo *OpenOLT) Get_device_details(device *voltha.Device) error {
	return olterrors.ErrNotImplemented
}

//Update_flows_bulk returns
func (oo *OpenOLT) Update_flows_bulk(device *voltha.Device, flows *voltha.Flows, groups *voltha.FlowGroups, flowMetadata *voltha.FlowMetadata) error {
	return olterrors.ErrNotImplemented
}

//Update_flows_incrementally updates (add/remove) the flows on a given device
func (oo *OpenOLT) Update_flows_incrementally(device *voltha.Device, flows *openflow_13.FlowChanges, groups *openflow_13.FlowGroupChanges, flowMetadata *voltha.FlowMetadata) error {
	logger.Debugw("Update_flows_incrementally", log.Fields{"deviceId": device.Id, "flows": flows, "flowMetadata": flowMetadata})
	ctx := context.Background()
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.UpdateFlowsIncrementally(ctx, device, flows, groups, flowMetadata)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Update_pm_config returns PmConfigs nil or error
func (oo *OpenOLT) Update_pm_config(device *voltha.Device, pmConfigs *voltha.PmConfigs) error {
	logger.Debugw("Update_pm_config", log.Fields{"deviceId": device.Id, "pmConfigs": pmConfigs})
        if handler := oo.getDeviceHandler(device.Id); handler != nil {
                handler.UpdatePmConfig(pmConfigs)
                return nil
        }
        return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//Receive_packet_out sends packet out to the device
func (oo *OpenOLT) Receive_packet_out(deviceID string, egressPortNo int, packet *openflow_13.OfpPacketOut) error {
	logger.Debugw("Receive_packet_out", log.Fields{"deviceId": deviceID, "egress_port_no": egressPortNo, "pkt": packet})
	ctx := context.Background()
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		return handler.PacketOut(ctx, egressPortNo, packet)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": deviceID}, nil)
}

//Suppress_event unimplemented
func (oo *OpenOLT) Suppress_event(filter *voltha.EventFilter) error {
	return olterrors.ErrNotImplemented
}

//Unsuppress_event  unimplemented
func (oo *OpenOLT) Unsuppress_event(filter *voltha.EventFilter) error {
	return olterrors.ErrNotImplemented
}

//Download_image unimplemented
func (oo *OpenOLT) Download_image(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//Get_image_download_status unimplemented
func (oo *OpenOLT) Get_image_download_status(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//Cancel_image_download unimplemented
func (oo *OpenOLT) Cancel_image_download(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//Activate_image_update unimplemented
func (oo *OpenOLT) Activate_image_update(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//Revert_image_update unimplemented
func (oo *OpenOLT) Revert_image_update(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

// Enable_port to Enable PON/NNI interface
func (oo *OpenOLT) Enable_port(deviceID string, port *voltha.Port) error {
	logger.Infow("Enable_port", log.Fields{"deviceId": deviceID, "port": port})
	return oo.enableDisablePort(deviceID, port, true)
}

// Disable_port to Disable pon/nni interface
func (oo *OpenOLT) Disable_port(deviceID string, port *voltha.Port) error {
	logger.Infow("Disable_port", log.Fields{"deviceId": deviceID, "port": port})
	return oo.enableDisablePort(deviceID, port, false)
}

// enableDisablePort to Disable pon or Enable PON interface
func (oo *OpenOLT) enableDisablePort(deviceID string, port *voltha.Port, enablePort bool) error {
	logger.Infow("enableDisablePort", log.Fields{"deviceId": deviceID, "port": port})
	if port == nil {
		return olterrors.NewErrInvalidValue(log.Fields{
			"reason":    "port cannot be nil",
			"device-id": deviceID,
			"port":      nil}, nil)
	}
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		logger.Debugw("Enable_Disable_Port", log.Fields{"deviceId": deviceID, "port": port})
		if enablePort {
			if err := handler.EnablePort(port); err != nil {
				return olterrors.NewErrAdapter("error-occurred-during-enable-port", log.Fields{"deviceID": deviceID, "port": port}, err)
			}
		} else {
			if err := handler.DisablePort(port); err != nil {
				return olterrors.NewErrAdapter("error-occurred-during-disable-port", log.Fields{"deviceID": deviceID, "port": port}, err)
			}
		}
	}
	return nil
}

//Child_device_lost deletes the ONU and its references from PONResources
func (oo *OpenOLT) Child_device_lost(deviceID string, pPortNo uint32, onuID uint32) error {
	logger.Infow("Child-device-lost", log.Fields{"parentId": deviceID})
	ctx := context.Background()
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		return handler.ChildDeviceLost(ctx, pPortNo, onuID)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": deviceID}, nil).Log()
}

//Start_omci_test not implemented
func (oo *OpenOLT) Start_omci_test(device *voltha.Device, request *voltha.OmciTestRequest) (*voltha.TestResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

//Get_ext_value retrieves a value on a particular ONU
func (oo *OpenOLT) Get_ext_value(deviceID string, device *voltha.Device, valueparam voltha.ValueType_Type) (*voltha.ReturnValues, error) {
	var err error
	resp := new(voltha.ReturnValues)
	log.Infow("Get_ext_value", log.Fields{"device-id": deviceID, "onu-id": device.Id})
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		if resp, err = handler.getExtValue(device, valueparam); err != nil {
			log.Errorw("error-occurred-during-get-ext-value", log.Fields{"device-id": deviceID, "onu-id": device.Id,
				"error": err})
			return nil, err
		}
	}
	return resp, nil
}
