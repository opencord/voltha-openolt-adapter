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
package adaptercore

import (
    "context"
    "errors"
    "fmt"
    "sync"

    com "github.com/opencord/voltha-go/adapters/common"
    "github.com/opencord/voltha-go/common/log"
    "github.com/opencord/voltha-go/kafka"
    ic "github.com/opencord/voltha-go/protos/inter_container"
    "github.com/opencord/voltha-go/protos/openflow_13"
    "github.com/opencord/voltha-go/protos/voltha"
)

type OpenOLT struct {
    deviceHandlers        map[string]*DeviceHandler
    coreProxy             *com.CoreProxy
    kafkaICProxy          *kafka.InterContainerProxy
    numOnus               int
    exitChannel           chan int
    lockDeviceHandlersMap sync.RWMutex
}

func NewOpenOLT(ctx context.Context, kafkaICProxy *kafka.InterContainerProxy, coreProxy *com.CoreProxy, onuNumber int) *OpenOLT {
    var openOLT OpenOLT
    openOLT.exitChannel = make(chan int, 1)
    openOLT.deviceHandlers = make(map[string]*DeviceHandler)
    openOLT.kafkaICProxy = kafkaICProxy
    openOLT.numOnus = onuNumber
    openOLT.coreProxy = coreProxy
    openOLT.lockDeviceHandlersMap = sync.RWMutex{}
    return &openOLT
}

func (oo *OpenOLT) Start(ctx context.Context) error {
    log.Info("starting-device-manager")
    log.Info("device-manager-started")
    return nil
}

func (oo *OpenOLT) Stop(ctx context.Context) error {
    log.Info("stopping-device-manager")
    oo.exitChannel <- 1
    log.Info("device-manager-stopped")
    return nil
}

func sendResponse(ctx context.Context, ch chan interface{}, result interface{}) {
    if ctx.Err() == nil {
        // Returned response only of the ctx has not been cancelled/timeout/etc
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
    if _, exist := oo.deviceHandlers[agent.deviceId]; !exist {
        oo.deviceHandlers[agent.deviceId] = agent
    }
}

func (oo *OpenOLT) deleteDeviceHandlerToMap(agent *DeviceHandler) {
    oo.lockDeviceHandlersMap.Lock()
    defer oo.lockDeviceHandlersMap.Unlock()
    delete(oo.deviceHandlers, agent.deviceId)
}

func (oo *OpenOLT) getDeviceHandler(deviceId string) *DeviceHandler {
    oo.lockDeviceHandlersMap.Lock()
    defer oo.lockDeviceHandlersMap.Unlock()
    if agent, ok := oo.deviceHandlers[deviceId]; ok {
        return agent
    }
    return nil
}

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

func (oo *OpenOLT) Adopt_device(device *voltha.Device) error {
    if device == nil {
        log.Warn("device-is-nil")
        return errors.New("nil-device")
    }
    log.Infow("adopt-device", log.Fields{"deviceId": device.Id})
    var handler *DeviceHandler
    if handler = oo.getDeviceHandler(device.Id); handler == nil {
        handler := NewDeviceHandler(oo.coreProxy, device, oo)
        oo.addDeviceHandlerToMap(handler)
        go handler.AdoptDevice(device)
        // Launch the creation of the device topic
        go oo.createDeviceTopic(device)
    }
    return nil
}

func (oo *OpenOLT) Get_ofp_device_info(device *voltha.Device) (*ic.SwitchCapability, error) {
    log.Infow("Get_ofp_device_info", log.Fields{"deviceId": device.Id})
    if handler := oo.getDeviceHandler(device.Id); handler != nil {
        return handler.GetOfpDeviceInfo(device)
    }
    log.Errorw("device-handler-not-set", log.Fields{"deviceId": device.Id})
    return nil, errors.New("device-handler-not-set")
}

func (oo *OpenOLT) Get_ofp_port_info(device *voltha.Device, port_no int64) (*ic.PortCapability, error) {
    log.Infow("Get_ofp_port_info", log.Fields{"deviceId": device.Id})
    if handler := oo.getDeviceHandler(device.Id); handler != nil {
        return handler.GetOfpPortInfo(device, port_no)
    }
    log.Errorw("device-handler-not-set", log.Fields{"deviceId": device.Id})
    return nil, errors.New("device-handler-not-set")
}

func (oo *OpenOLT) Process_inter_adapter_message(msg *ic.InterAdapterMessage) error {
    log.Infow("Process_inter_adapter_message", log.Fields{"msgId": msg.Header.Id})
    targetDevice := msg.Header.ProxyDeviceId // Request?
    if targetDevice == "" && msg.Header.ToDeviceId != "" {
        // Typical response
        targetDevice = msg.Header.ToDeviceId
    }
    if handler := oo.getDeviceHandler(targetDevice); handler != nil {
        return handler.Process_inter_adapter_message(msg)
    }
    return errors.New(fmt.Sprintf("handler-not-found-%s", targetDevice))
}

func (oo *OpenOLT) Adapter_descriptor() error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Device_types() (*voltha.DeviceTypes, error) {
    return nil, errors.New("UnImplemented")
}

func (oo *OpenOLT) Health() (*voltha.HealthStatus, error) {
    return nil, errors.New("UnImplemented")
}

func (oo *OpenOLT) Reconcile_device(device *voltha.Device) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Abandon_device(device *voltha.Device) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Disable_device(device *voltha.Device) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Reenable_device(device *voltha.Device) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Reboot_device(device *voltha.Device) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Self_test_device(device *voltha.Device) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Gelete_device(device *voltha.Device) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Get_device_details(device *voltha.Device) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Update_flows_bulk(device *voltha.Device, flows *voltha.Flows, groups *voltha.FlowGroups) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Update_flows_incrementally(device *voltha.Device, flows *openflow_13.FlowChanges, groups *openflow_13.FlowGroupChanges) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Update_pm_config(device *voltha.Device, pm_configs *voltha.PmConfigs) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Receive_packet_out(device *voltha.Device, egress_port_no int, msg openflow_13.PacketOut) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Suppress_alarm(filter *voltha.AlarmFilter) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Unsuppress_alarm(filter *voltha.AlarmFilter) error {
    return errors.New("UnImplemented")
}

func (oo *OpenOLT) Download_image(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
    return nil, errors.New("UnImplemented")
}

func (oo *OpenOLT) Get_image_download_status(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
    return nil, errors.New("UnImplemented")
}

func (oo *OpenOLT) Cancel_image_download(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
    return nil, errors.New("UnImplemented")
}

func (oo *OpenOLT) Activate_image_update(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
    return nil, errors.New("UnImplemented")
}

func (oo *OpenOLT) Revert_image_update(device *voltha.Device, request *voltha.ImageDownload) (*voltha.ImageDownload, error) {
    return nil, errors.New("UnImplemented")
}
