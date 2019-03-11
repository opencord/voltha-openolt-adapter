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
    "io"
    "strconv"
    "strings"
    "sync"

    "github.com/gogo/protobuf/proto"
    com "github.com/opencord/voltha-go/adapters/common"
    "github.com/opencord/voltha-go/common/log"
    "github.com/opencord/voltha-go/protos/common"
    ic "github.com/opencord/voltha-go/protos/inter_container"
    of "github.com/opencord/voltha-go/protos/openflow_13"
    oop "github.com/opencord/voltha-go/protos/openolt"
    "github.com/opencord/voltha-go/protos/voltha"
    "google.golang.org/grpc"
)

//DeviceHandler will interact with the OLT device.
type DeviceHandler struct {
    deviceId      string
    deviceType    string
    device        *voltha.Device
    coreProxy     *com.CoreProxy
    openOLT       *OpenOLT
    nniPort       *voltha.Port
    ponPort       *voltha.Port
    exitChannel   chan int
    lockDevice    sync.RWMutex
    client        oop.OpenoltClient
    transitionMap *TransitionMap
    clientCon     *grpc.ClientConn
}

//NewDeviceHandler creates a new device handler
func NewDeviceHandler(cp *com.CoreProxy, device *voltha.Device, adapter *OpenOLT) *DeviceHandler {
    var dh DeviceHandler
    dh.coreProxy = cp
    cloned := (proto.Clone(device)).(*voltha.Device)
    dh.deviceId = cloned.Id
    dh.deviceType = cloned.Type
    dh.device = cloned
    dh.openOLT = adapter
    dh.exitChannel = make(chan int, 1)
    dh.lockDevice = sync.RWMutex{}

    //TODO initialize the support classes.
    return &dh
}

// start save the device to the data model
func (dh *DeviceHandler) start(ctx context.Context) {
    dh.lockDevice.Lock()
    defer dh.lockDevice.Unlock()
    log.Debugw("starting-device-agent", log.Fields{"device": dh.device})
    // Add the initial device to the local model
    log.Debug("device-agent-started")
}

// stop stops the device dh.  Not much to do for now
func (dh *DeviceHandler) stop(ctx context.Context) {
    dh.lockDevice.Lock()
    defer dh.lockDevice.Unlock()
    log.Debug("stopping-device-agent")
    dh.exitChannel <- 1
    log.Debug("device-agent-stopped")
}

func macAddressToUint32Array(mac string) []uint32 {
    slist := strings.Split(mac, ":")
    result := make([]uint32, len(slist))
    var err error
    var tmp int64
    for index, val := range slist {
        if tmp, err = strconv.ParseInt(val, 16, 32); err != nil {
            return []uint32{1, 2, 3, 4, 5, 6}
        }
        result[index] = uint32(tmp)
    }
    return result
}

func portName(portNum uint32, portType voltha.Port_PortType, intfId uint32) string {

    if portType == voltha.Port_PON_OLT {
        return "pon-" + string(portNum)
    } else if portType == voltha.Port_ETHERNET_NNI {
        return "nni-" + string(intfId)
    } else if portType == voltha.Port_ETHERNET_UNI {
        log.Errorw("local UNI management not supported", log.Fields{})
    }
    return ""
}

func (dh *DeviceHandler) addPort(intfId uint32, portType voltha.Port_PortType, state string) {
    var operStatus common.OperStatus_OperStatus
    if state == "up" {
        operStatus = voltha.OperStatus_ACTIVE
    } else {
        operStatus = voltha.OperStatus_DISCOVERED
    }

    // TODO
    //portNum := platform.intfIdToPortNo(intfId, portType)
    portNum := intfId

    label := portName(portNum, portType, intfId)
    //    Now create the PON Port
    ponPort := &voltha.Port{
        PortNo:     portNum,
        Label:      label,
        Type:       portType,
        OperStatus: operStatus,
    }

    // Synchronous call to update device - this method is run in its own go routine
    if err := dh.coreProxy.PortCreated(nil, dh.device.Id, ponPort); err != nil {
        log.Errorw("error-creating-nni-port", log.Fields{"deviceId": dh.device.Id, "error": err})
    }
}

// readIndications to read the indications from the OLT device
func (dh *DeviceHandler) readIndications() {
    indications, err := dh.client.EnableIndication(context.Background(), new(oop.Empty))
    if err != nil {
        log.Errorw("Failed to read indications", log.Fields{"err": err})
        return
    }
    if indications == nil {
        log.Errorw("Indications is nil", log.Fields{})
        return
    }
    for {
        indication, err := indications.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Infow("Failed to read from indications", log.Fields{"err": err})
            continue
        }
        switch indication.Data.(type) {
        case *oop.Indication_OltInd:
            oltInd := indication.GetOltInd()
            if oltInd.OperState == "up" {
                dh.transitionMap.Handle(DeviceUpInd)
            } else if oltInd.OperState == "down" {
                dh.transitionMap.Handle(DeviceDownInd)
            }
        case *oop.Indication_IntfInd:

            intfInd := indication.GetIntfInd()
            go dh.addPort(intfInd.GetIntfId(), voltha.Port_PON_OLT, intfInd.GetOperState())
            log.Infow("Received interface indication ", log.Fields{"InterfaceInd": intfInd})
        case *oop.Indication_IntfOperInd:
            intfOperInd := indication.GetIntfOperInd()
            if intfOperInd.GetType() == "nni" {
                go dh.addPort(intfOperInd.GetIntfId(), voltha.Port_ETHERNET_NNI, intfOperInd.GetOperState())
            } else if intfOperInd.GetType() == "pon" {
                // TODO: Check what needs to be handled here for When PON PORT down, ONU will be down
                // Handle pon port update
            }
            log.Infow("Received interface oper indication ", log.Fields{"InterfaceOperInd": intfOperInd})
        case *oop.Indication_OnuDiscInd:
            onuDiscInd := indication.GetOnuDiscInd()
            log.Infow("Received Onu discovery indication ", log.Fields{"OnuDiscInd": onuDiscInd})
            // TODO Get onu ID from the resource manager
            onuId := 0
            go dh.coreProxy.ChildDeviceDetected(nil, dh.device.Id, int(onuDiscInd.GetIntfId()),
                   "brcm_openomci_onu", int(onuDiscInd.GetIntfId()), string(onuDiscInd.SerialNumber.GetVendorId()), 
                   string(onuDiscInd.SerialNumber.GetVendorSpecific()), int64(onuId))
        case *oop.Indication_OnuInd:
            onuInd := indication.GetOnuInd()
            log.Infow("Received Onu indication ", log.Fields{"OnuInd": onuInd})
        case *oop.Indication_OmciInd:
            omciInd := indication.GetOmciInd()
            log.Infow("Received Omci indication ", log.Fields{"OmciInd": omciInd})
        case *oop.Indication_PktInd:
            pktInd := indication.GetPktInd()
            log.Infow("Received pakcet indication ", log.Fields{"PktInd": pktInd})
        case *oop.Indication_PortStats:
            portStats := indication.GetPortStats()
            log.Infow("Received port stats indication", log.Fields{"PortStats": portStats})
        case *oop.Indication_FlowStats:
            flowStats := indication.GetFlowStats()
            log.Infow("Received flow stats", log.Fields{"FlowStats": flowStats})
        case *oop.Indication_AlarmInd:
            alarmInd := indication.GetAlarmInd()
            log.Infow("Received alarm indication ", log.Fields{"AlarmInd": alarmInd})
        }
    }
}

// doStateUp handle the olt up indication and update to voltha core
func (dh *DeviceHandler) doStateUp() error {
    // Synchronous call to update device state - this method is run in its own go routine
    if err := dh.coreProxy.DeviceStateUpdate(context.Background(), dh.device.Id, voltha.ConnectStatus_REACHABLE,
        voltha.OperStatus_ACTIVE); err != nil {
        log.Errorw("Failed to update device with OLT UP indication", log.Fields{"deviceId": dh.device.Id, "error": err})
        return err
    }
    return nil
}

// doStateDown handle the olt down indication
func (dh *DeviceHandler) doStateDown() error {
    //TODO Handle oper state down
    return nil
}

// doStateInit dial the grpc before going to init state
func (dh *DeviceHandler) doStateInit() error {
    var err error
    dh.clientCon, err = grpc.Dial(dh.device.GetHostAndPort(), grpc.WithInsecure())
    if err != nil {
        log.Errorw("Failed to dial device", log.Fields{"DeviceId": dh.deviceId, "HostAndPort": dh.device.GetHostAndPort(), "err": err})
        return err
    }
    return nil
}

// postInit create olt client instance to invoke RPC on the olt device
func (dh *DeviceHandler) postInit() error {
    dh.client = oop.NewOpenoltClient(dh.clientCon)
    dh.transitionMap.Handle(GrpcConnected)
    return nil
}

// doStateConnected get the device info and update to voltha core
func (dh *DeviceHandler) doStateConnected() error {
    deviceInfo, err := dh.client.GetDeviceInfo(context.Background(), new(oop.Empty))
    if err != nil {
        log.Errorw("Failed to fetch device info", log.Fields{"err": err})
        return err
    }
    if deviceInfo == nil {
        log.Errorw("Device info is nil", log.Fields{})
        return errors.New("Failed to get device info from OLT")
    }

    dh.device.Root = true
    dh.device.Vendor = deviceInfo.Vendor
    dh.device.Model = deviceInfo.Model
    dh.device.ConnectStatus = voltha.ConnectStatus_REACHABLE
    dh.device.SerialNumber = deviceInfo.DeviceSerialNumber
    dh.device.HardwareVersion = deviceInfo.HardwareVersion
    dh.device.FirmwareVersion = deviceInfo.FirmwareVersion
    // TODO : Check whether this MAC address is learnt from SDPON or need to send from device
    dh.device.MacAddress = "0a:0b:0c:0d:0e:0f"

    // Synchronous call to update device - this method is run in its own go routine
    if err := dh.coreProxy.DeviceUpdate(nil, dh.device); err != nil {
        log.Errorw("error-updating-device", log.Fields{"deviceId": dh.device.Id, "error": err})
    }

    // Start reading indications
    go dh.readIndications()
    return nil
}

// AdoptDevice adopts the OLT device
func (dh *DeviceHandler) AdoptDevice(device *voltha.Device) {
    dh.transitionMap = NewTransitionMap(dh)
    log.Infow("AdoptDevice", log.Fields{"deviceId": device.Id, "Address": device.GetHostAndPort()})
    dh.transitionMap.Handle(DeviceInit)
}

// GetOfpDeviceInfo Get the Ofp device information
func (dh *DeviceHandler) GetOfpDeviceInfo(device *voltha.Device) (*ic.SwitchCapability, error) {
    return &ic.SwitchCapability{
        Desc: &of.OfpDesc{
            HwDesc:    "open_pon",
            SwDesc:    "open_pon",
            SerialNum: dh.device.SerialNumber,
        },
        SwitchFeatures: &of.OfpSwitchFeatures{
            NBuffers: 256,
            NTables:  2,
            Capabilities: uint32(of.OfpCapabilities_OFPC_FLOW_STATS |
                of.OfpCapabilities_OFPC_TABLE_STATS |
                of.OfpCapabilities_OFPC_PORT_STATS |
                of.OfpCapabilities_OFPC_GROUP_STATS),
        },
    }, nil
}

// GetOfpPortInfo Get Ofp port information
func (dh *DeviceHandler) GetOfpPortInfo(device *voltha.Device, portNo int64) (*ic.PortCapability, error) {
    cap := uint32(of.OfpPortFeatures_OFPPF_1GB_FD | of.OfpPortFeatures_OFPPF_FIBER)
    return &ic.PortCapability{
        Port: &voltha.LogicalPort{
            OfpPort: &of.OfpPort{
                HwAddr:     macAddressToUint32Array(dh.device.MacAddress),
                Config:     0,
                State:      uint32(of.OfpPortState_OFPPS_LIVE),
                Curr:       cap,
                Advertised: cap,
                Peer:       cap,
                CurrSpeed:  uint32(of.OfpPortFeatures_OFPPF_1GB_FD),
                MaxSpeed:   uint32(of.OfpPortFeatures_OFPPF_1GB_FD),
            },
            DeviceId:     dh.device.Id,
            DevicePortNo: uint32(portNo),
        },
    }, nil
}

// Process_inter_adapter_message process inter adater message
func (dh *DeviceHandler) Process_inter_adapter_message(msg *ic.InterAdapterMessage) error {
    // TODO
    log.Debugw("Process_inter_adapter_message", log.Fields{"msgId": msg.Header.Id})
    return nil
}

