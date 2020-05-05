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
	if _, exist := oo.deviceHandlers[agent.deviceID]; !exist {
		oo.deviceHandlers[agent.deviceID] = agent
	}
}

func (oo *OpenOLT) deleteDeviceHandlerToMap(agent *DeviceHandler) {
	oo.lockDeviceHandlersMap.Lock()
	defer oo.lockDeviceHandlersMap.Unlock()
	delete(oo.deviceHandlers, agent.deviceID)
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

// AdoptDevice creates a new device handler if not present already and then adopts the device
func (oo *OpenOLT) AdoptDevice(device *voltha.Device) error {
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

//GetOfpDeviceInfo returns OFP information for the given device
func (oo *OpenOLT) GetOfpDeviceInfo(device *voltha.Device) (*ic.SwitchCapability, error) {
	logger.Infow("GetOfpDeviceInfo", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.GetOfpDeviceInfo(device)
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//GetOfpPortInfo returns OFP port information for the given device
func (oo *OpenOLT) GetOfpPortInfo(device *voltha.Device, portNo int64) (*ic.PortCapability, error) {
	logger.Infow("GetOfpPortInfo", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.GetOfpPortInfo(device, portNo)
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//ProcessInterAdapterMessage sends messages to a target device (between adapters)
func (oo *OpenOLT) ProcessInterAdapterMessage(msg *ic.InterAdapterMessage) error {
	logger.Debugw("ProcessInterAdapterMessage", log.Fields{"msgId": msg.Header.Id})
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

//AdapterDescriptor not implemented
func (oo *OpenOLT) AdapterDescriptor() error {
	return olterrors.ErrNotImplemented
}

//DeviceTypes unimplemented
func (oo *OpenOLT) DeviceTypes() (*voltha.DeviceTypes, error) {
	return nil, olterrors.ErrNotImplemented
}

//Health  returns unimplemented
func (oo *OpenOLT) Health() (*voltha.HealthStatus, error) {
	return nil, olterrors.ErrNotImplemented
}

//ReconcileDevice unimplemented
func (oo *OpenOLT) ReconcileDevice(device *voltha.Device) error {
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

//AbandonDevice unimplemented
func (oo *OpenOLT) AbandonDevice(device *voltha.Device) error {
	return olterrors.ErrNotImplemented
}

//DisableDevice disables the given device
func (oo *OpenOLT) DisableDevice(device *voltha.Device) error {
	logger.Infow("disable-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.DisableDevice(device)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//ReenableDevice enables the olt device after disable
func (oo *OpenOLT) ReenableDevice(device *voltha.Device) error {
	logger.Infow("reenable-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.ReenableDevice(device)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//RebootDevice reboots the given device
func (oo *OpenOLT) RebootDevice(device *voltha.Device) error {
	logger.Infow("reboot-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.RebootDevice(device)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//SelfTestDevice unimplented
func (oo *OpenOLT) SelfTestDevice(device *voltha.Device) error {
	return olterrors.ErrNotImplemented
}

//DeleteDevice unimplemented
func (oo *OpenOLT) DeleteDevice(device *voltha.Device) error {
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

//GetDeviceDetails unimplemented
func (oo *OpenOLT) GetDeviceDetails(device *voltha.Device) error {
	return olterrors.ErrNotImplemented
}

//UpdateFlowsBulk returns
func (oo *OpenOLT) UpdateFlowsBulk(device *voltha.Device, flows *voltha.Flows, groups *voltha.FlowGroups, flowMetadata *voltha.FlowMetadata) error {
	return olterrors.ErrNotImplemented
}

//UpdateFlowsIncrementally updates (add/remove) the flows on a given device
func (oo *OpenOLT) UpdateFlowsIncrementally(device *voltha.Device, flows *openflow_13.FlowChanges, groups *openflow_13.FlowGroupChanges, flowMetadata *voltha.FlowMetadata) error {
	logger.Debugw("UpdateFlowsIncrementally", log.Fields{"deviceId": device.Id, "flows": flows, "flowMetadata": flowMetadata})
	ctx := context.Background()
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.UpdateFlowsIncrementally(ctx, device, flows, groups, flowMetadata)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

//UpdatePmConfig returns PmConfigs nil or error
func (oo *OpenOLT) UpdatePmConfig(device *voltha.Device, pmConfigs *voltha.PmConfigs) error {
	return olterrors.ErrNotImplemented
}

//ReceivePacketOut sends packet out to the device
func (oo *OpenOLT) ReceivePacketOut(deviceID string, egressPortNo int, packet *openflow_13.OfpPacketOut) error {
	logger.Debugw("ReceivePacketOut", log.Fields{"deviceId": deviceID, "egress_port_no": egressPortNo, "pkt": packet})
	ctx := context.Background()
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		return handler.PacketOut(ctx, egressPortNo, packet)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": deviceID}, nil)
}

//SuppressEvent unimplemented
func (oo *OpenOLT) SuppressEvent(filter *voltha.EventFilter) error {
	return olterrors.ErrNotImplemented
}

//UnsuppressEvent  unimplemented
func (oo *OpenOLT) UnsuppressEvent(filter *voltha.EventFilter) error {
	return olterrors.ErrNotImplemented
}

//DownloadImage unimplemented
func (oo *OpenOLT) DownloadImage(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//GetImageDownloadStatus unimplemented
func (oo *OpenOLT) GetImageDownloadStatus(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//CancelImageDownload unimplemented
func (oo *OpenOLT) CancelImageDownload(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//ActivateImageUpdate unimplemented
func (oo *OpenOLT) ActivateImageUpdate(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

//RevertImageUpdate unimplemented
func (oo *OpenOLT) RevertImageUpdate(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

// EnablePort to Enable PON/NNI interface
func (oo *OpenOLT) EnablePort(deviceID string, port *voltha.Port) error {
	logger.Infow("EnablePort", log.Fields{"deviceId": deviceID, "port": port})
	return oo.enableDisablePort(deviceID, port, true)
}

// DisablePort to Disable pon/nni interface
func (oo *OpenOLT) DisablePort(deviceID string, port *voltha.Port) error {
	logger.Infow("DisablePort", log.Fields{"deviceId": deviceID, "port": port})
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

//ChildDeviceLost deletes the ONU and its references from PONResources
func (oo *OpenOLT) ChildDeviceLost(deviceID string, pPortNo uint32, onuID uint32) error {
	logger.Infow("Child-device-lost", log.Fields{"parentId": deviceID})
	ctx := context.Background()
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		return handler.ChildDeviceLost(ctx, pPortNo, onuID)
	}
	return olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": deviceID}, nil).Log()
}

//StartOmciTest not implemented
func (oo *OpenOLT) StartOmciTest(device *voltha.Device, request *voltha.OmciTestRequest) (*voltha.TestResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

//GetExtValue retrieves a value on a particular ONU
func (oo *OpenOLT) GetExtValue(deviceID string, device *voltha.Device, valueparam voltha.ValueType_Type) (*voltha.ReturnValues, error) {
	var err error
	resp := new(voltha.ReturnValues)
	log.Infow("GetExtValue", log.Fields{"device-id": deviceID, "onu-id": device.Id})
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		if resp, err = handler.getExtValue(device, valueparam); err != nil {
			log.Errorw("error-occurred-during-get-ext-value", log.Fields{"device-id": deviceID, "onu-id": device.Id,
				"error": err})
			return nil, err
		}
	}
	return resp, nil
}
