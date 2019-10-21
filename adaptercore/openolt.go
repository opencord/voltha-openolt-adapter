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
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/opencord/voltha-lib-go/pkg/adapters/adapterif"
	"github.com/opencord/voltha-lib-go/pkg/kafka"
	"github.com/opencord/voltha-lib-go/pkg/log"
	ic "github.com/opencord/voltha-protos/go/inter_container"
	"github.com/opencord/voltha-protos/go/openflow_13"
	"github.com/opencord/voltha-protos/go/voltha"
)

//OpenOLT structure holds the OLT information
type OpenOLT struct {
	deviceHandlers        map[string]*DeviceHandler
	coreProxy             adapterif.CoreProxy
	adapterProxy          adapterif.AdapterProxy
	eventProxy            adapterif.EventProxy
	kafkaICProxy          *kafka.InterContainerProxy
	numOnus               int
	KVStoreHost           string
	KVStorePort           int
	KVStoreType           string
	exitChannel           chan int
	lockDeviceHandlersMap sync.RWMutex
}

//NewOpenOLT returns a new instance of OpenOLT
func NewOpenOLT(ctx context.Context, kafkaICProxy *kafka.InterContainerProxy,
	coreProxy adapterif.CoreProxy, adapterProxy adapterif.AdapterProxy,
	eventProxy adapterif.EventProxy, onuNumber int, kvStoreHost string,
	kvStorePort int, KVStoreType string) *OpenOLT {
	var openOLT OpenOLT
	openOLT.exitChannel = make(chan int, 1)
	openOLT.deviceHandlers = make(map[string]*DeviceHandler)
	openOLT.kafkaICProxy = kafkaICProxy
	openOLT.numOnus = onuNumber
	openOLT.coreProxy = coreProxy
	openOLT.adapterProxy = adapterProxy
	openOLT.eventProxy = eventProxy
	openOLT.KVStoreHost = kvStoreHost
	openOLT.KVStorePort = kvStorePort
	openOLT.KVStoreType = KVStoreType
	openOLT.lockDeviceHandlersMap = sync.RWMutex{}
	return &openOLT
}

//Start starts (logs) the device manager
func (oo *OpenOLT) Start(ctx context.Context) error {
	log.Info("starting-device-manager")
	log.Info("device-manager-started")
	return nil
}

//Stop terminates the session
func (oo *OpenOLT) Stop(ctx context.Context) error {
	log.Info("stopping-device-manager")
	oo.exitChannel <- 1
	log.Info("device-manager-stopped")
	return nil
}

func sendResponse(ctx context.Context, ch chan interface{}, result interface{}) {
	if ctx.Err() == nil {
		// Returned response only of the ctx has not been canceled/timeout/etc
		// Channel is automatically closed when a context is Done
		ch <- result
		log.Debugw("sendResponse", log.Fields{"result": result})
	} else {
		// Should the transaction be reverted back?
		log.Debugw("sendResponse-context-error", log.Fields{"context-error": ctx.Err()})
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
	log.Infow("create-device-topic", log.Fields{"deviceId": device.Id})
	deviceTopic := kafka.Topic{Name: oo.kafkaICProxy.DefaultTopic.Name + "_" + device.Id}
	// TODO for the offset
	if err := oo.kafkaICProxy.SubscribeWithDefaultRequestHandler(deviceTopic, 0); err != nil {
		log.Infow("create-device-topic-failed", log.Fields{"deviceId": device.Id, "error": err})
		return err
	}
	return nil
}

// Adopt_device creates a new device handler if not present already and then adopts the device
func (oo *OpenOLT) Adopt_device(device *voltha.Device) error {
	if device == nil {
		log.Warn("device-is-nil")
		return errors.New("nil-device")
	}
	log.Infow("adopt-device", log.Fields{"deviceId": device.Id})
	var handler *DeviceHandler
	if handler = oo.getDeviceHandler(device.Id); handler == nil {
		handler := NewDeviceHandler(oo.coreProxy, oo.adapterProxy, oo.eventProxy, device, oo)
		oo.addDeviceHandlerToMap(handler)
		go handler.AdoptDevice(device)
		// Launch the creation of the device topic
		// go oo.createDeviceTopic(device)
	}
	return nil
}

//Get_ofp_device_info returns OFP information for the given device
func (oo *OpenOLT) Get_ofp_device_info(device *voltha.Device) (*ic.SwitchCapability, error) {
	log.Infow("Get_ofp_device_info", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.GetOfpDeviceInfo(device)
	}
	log.Errorw("device-handler-not-set", log.Fields{"deviceId": device.Id})
	return nil, errors.New("device-handler-not-set")
}

//Get_ofp_port_info returns OFP port information for the given device
func (oo *OpenOLT) Get_ofp_port_info(device *voltha.Device, portNo int64) (*ic.PortCapability, error) {
	log.Infow("Get_ofp_port_info", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.GetOfpPortInfo(device, portNo)
	}
	log.Errorw("device-handler-not-set", log.Fields{"deviceId": device.Id})
	return nil, errors.New("device-handler-not-set")
}

//Process_inter_adapter_message sends messages to a target device (between adapters)
func (oo *OpenOLT) Process_inter_adapter_message(msg *ic.InterAdapterMessage) error {
	log.Infow("Process_inter_adapter_message", log.Fields{"msgId": msg.Header.Id})
	targetDevice := msg.Header.ProxyDeviceId // Request?
	if targetDevice == "" && msg.Header.ToDeviceId != "" {
		// Typical response
		targetDevice = msg.Header.ToDeviceId
	}
	if handler := oo.getDeviceHandler(targetDevice); handler != nil {
		return handler.ProcessInterAdapterMessage(msg)
	}
	return fmt.Errorf(fmt.Sprintf("handler-not-found-%s", targetDevice))
}

//Adapter_descriptor not implemented
func (oo *OpenOLT) Adapter_descriptor() error {
	return errors.New("unImplemented")
}

//Device_types unimplemented
func (oo *OpenOLT) Device_types() (*voltha.DeviceTypes, error) {
	return nil, errors.New("unImplemented")
}

//Health  returns unimplemented
func (oo *OpenOLT) Health() (*voltha.HealthStatus, error) {
	return nil, errors.New("unImplemented")
}

//Reconcile_device unimplemented
func (oo *OpenOLT) Reconcile_device(device *voltha.Device) error {
	if device == nil {
               log.Warn("device-is-nil")
               return errors.New("nil-device")
	}
        log.Infow("reconcile-device", log.Fields{"deviceId": device.Id})
        var handler *DeviceHandler
        if handler = oo.getDeviceHandler(device.Id); handler == nil {
                handler := NewDeviceHandler(oo.coreProxy, oo.adapterProxy, oo.eventProxy, device, oo)
                oo.addDeviceHandlerToMap(handler)
		handler.transitionMap = NewTransitionMap(handler)
                handler.transitionMap.Handle(DeviceInit)
        }
        return nil
}

//Abandon_device unimplemented
func (oo *OpenOLT) Abandon_device(device *voltha.Device) error {
	return errors.New("unImplemented")
}

//Disable_device disables the given device
func (oo *OpenOLT) Disable_device(device *voltha.Device) error {
	log.Infow("disable-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.DisableDevice(device)
	}
	log.Errorw("device-handler-not-set", log.Fields{"deviceId": device.Id})
	return errors.New("device-handler-not-found")
}

//Reenable_device enables the olt device after disable
func (oo *OpenOLT) Reenable_device(device *voltha.Device) error {
	log.Infow("reenable-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.ReenableDevice(device)
	}
	log.Errorw("device-handler-not-set", log.Fields{"deviceId": device.Id})
	return errors.New("device-handler-not-found")
}

//Reboot_device reboots the given device
func (oo *OpenOLT) Reboot_device(device *voltha.Device) error {
	log.Infow("reboot-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.RebootDevice(device)
	}
	log.Errorw("device-handler-not-set", log.Fields{"deviceId": device.Id})
	return errors.New("device-handler-not-found")

}

//Self_test_device unimplented
func (oo *OpenOLT) Self_test_device(device *voltha.Device) error {
	return errors.New("unImplemented")
}

//Delete_device unimplemented
func (oo *OpenOLT) Delete_device(device *voltha.Device) error {
	log.Infow("delete-device", log.Fields{"deviceId": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		if err := handler.DeleteDevice(device); err != nil {
			log.Errorw("failed-to-handle-delete-device", log.Fields{"device-id": device.Id})
		}
		oo.deleteDeviceHandlerToMap(handler)
		return nil
	}
	log.Errorw("device-handler-not-set", log.Fields{"deviceId": device.Id})
	return errors.New("device-handler-not-found")
}

//Get_device_details unimplemented
func (oo *OpenOLT) Get_device_details(device *voltha.Device) error {
	return errors.New("unImplemented")
}

//Update_flows_bulk returns
func (oo *OpenOLT) Update_flows_bulk(device *voltha.Device, flows *voltha.Flows, groups *voltha.FlowGroups, flowMetadata *voltha.FlowMetadata) error {
	return errors.New("unImplemented")
}

//Update_flows_incrementally updates (add/remove) the flows on a given device
func (oo *OpenOLT) Update_flows_incrementally(device *voltha.Device, flows *openflow_13.FlowChanges, groups *openflow_13.FlowGroupChanges, flowMetadata *voltha.FlowMetadata) error {
	log.Debugw("Update_flows_incrementally", log.Fields{"deviceId": device.Id, "flows": flows, "flowMetadata": flowMetadata})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.UpdateFlowsIncrementally(device, flows, groups, flowMetadata)
	}
	log.Errorw("Update_flows_incrementally failed-device-handler-not-set", log.Fields{"deviceId": device.Id})
	return errors.New("device-handler-not-set")
}

//Update_pm_config returns PmConfigs nil or error
func (oo *OpenOLT) Update_pm_config(device *voltha.Device, pmConfigs *voltha.PmConfigs) error {
	return errors.New("unImplemented")
}

//Receive_packet_out sends packet out to the device
func (oo *OpenOLT) Receive_packet_out(deviceID string, egressPortNo int, packet *openflow_13.OfpPacketOut) error {
	log.Debugw("Receive_packet_out", log.Fields{"deviceId": deviceID, "egress_port_no": egressPortNo, "pkt": packet})
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		return handler.PacketOut(egressPortNo, packet)
	}
	log.Errorw("Receive_packet_out failed-device-handler-not-set", log.Fields{"deviceId": deviceID, "egressport": egressPortNo, "packet": packet})
	return errors.New("device-handler-not-set")
}

//Suppress_alarm unimplemented
func (oo *OpenOLT) Suppress_alarm(filter *voltha.AlarmFilter) error {
	return errors.New("unImplemented")
}

//Unsuppress_alarm  unimplemented
func (oo *OpenOLT) Unsuppress_alarm(filter *voltha.AlarmFilter) error {
	return errors.New("unImplemented")
}

//Download_image unimplemented
func (oo *OpenOLT) Download_image(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, errors.New("unImplemented")
}

//Get_image_download_status unimplemented
func (oo *OpenOLT) Get_image_download_status(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, errors.New("unImplemented")
}

//Cancel_image_download unimplemented
func (oo *OpenOLT) Cancel_image_download(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, errors.New("unImplemented")
}

//Activate_image_update unimplemented
func (oo *OpenOLT) Activate_image_update(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, errors.New("unImplemented")
}

//Revert_image_update unimplemented
func (oo *OpenOLT) Revert_image_update(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
	return nil, errors.New("unImplemented")
}
