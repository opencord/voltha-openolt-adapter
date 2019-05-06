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
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	com "github.com/opencord/voltha-go/adapters/common"
	"github.com/opencord/voltha-go/common/log"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	"github.com/opencord/voltha-protos/go/common"
	ic "github.com/opencord/voltha-protos/go/inter_container"
	of "github.com/opencord/voltha-protos/go/openflow_13"
	oop "github.com/opencord/voltha-protos/go/openolt"
	"github.com/opencord/voltha-protos/go/voltha"
	"google.golang.org/grpc"
)

//DeviceHandler will interact with the OLT device.
type DeviceHandler struct {
	deviceId      string
	deviceType    string
	adminState    string
	device        *voltha.Device
	coreProxy     *com.CoreProxy
	AdapterProxy  *com.AdapterProxy
	openOLT       *OpenOLT
	nniPort       *voltha.Port
	ponPort       *voltha.Port
	exitChannel   chan int
	lockDevice    sync.RWMutex
	Client        oop.OpenoltClient
	transitionMap *TransitionMap
	clientCon     *grpc.ClientConn
	flowMgr       *OpenOltFlowMgr
	resourceMgr   *rsrcMgr.OpenOltResourceMgr
}

//NewDeviceHandler creates a new device handler
func NewDeviceHandler(cp *com.CoreProxy, ap *com.AdapterProxy, device *voltha.Device, adapter *OpenOLT) *DeviceHandler {
	var dh DeviceHandler
	dh.coreProxy = cp
	dh.AdapterProxy = ap
	cloned := (proto.Clone(device)).(*voltha.Device)
	dh.deviceId = cloned.Id
	dh.deviceType = cloned.Type
	dh.adminState = "up"
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

func GetportLabel(portNum uint32, portType voltha.Port_PortType) string {

	if portType == voltha.Port_ETHERNET_NNI {
		return fmt.Sprintf("nni-%d", portNum)
	} else if portType == voltha.Port_PON_OLT {
		return fmt.Sprintf("pon-%d", portNum)
	} else if portType == voltha.Port_ETHERNET_UNI {
		log.Errorw("local UNI management not supported", log.Fields{})
		return ""
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
	// portNum := IntfIdToPortNo(intfId,portType)
	portNum := intfId
	if portType == voltha.Port_ETHERNET_NNI {
		portNum = IntfIdToPortNo(intfId, portType)
	}
	// portNum := IntfIdToPortNo(intfId,portType)
	label := GetportLabel(portNum, portType)
	if len(label) == 0 {
		log.Errorw("Invalid-port-label", log.Fields{"portNum": portNum, "portType": portType})
		return
	}
	//    Now create  Port
	port := &voltha.Port{
		PortNo:     portNum,
		Label:      label,
		Type:       portType,
		OperStatus: operStatus,
	}
	log.Debugw("Sending port update to core", log.Fields{"port": port})
	// Synchronous call to update device - this method is run in its own go routine
	if err := dh.coreProxy.PortCreated(nil, dh.device.Id, port); err != nil {
		log.Errorw("error-creating-nni-port", log.Fields{"deviceId": dh.device.Id, "error": err})
	}
}

// readIndications to read the indications from the OLT device
func (dh *DeviceHandler) readIndications() {
	indications, err := dh.Client.EnableIndication(context.Background(), new(oop.Empty))
	if err != nil {
		log.Errorw("Failed to read indications", log.Fields{"err": err})
		return
	}
	if indications == nil {
		log.Errorw("Indications is nil", log.Fields{})
		return
	}
	/* get device state */
	device, err := dh.coreProxy.GetDevice(nil, dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		log.Errorw("Failed to fetch device info", log.Fields{"err": err})

	}
	// When the device is in DISABLED and Adapter container restarts, we need to
	// rebuild the locally maintained admin state.
	if device.AdminState == voltha.AdminState_DISABLED {
		dh.lockDevice.Lock()
		dh.adminState = "down"
		dh.lockDevice.Unlock()
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
		// When OLT is admin down, allow only NNI operation status change indications.
		if dh.adminState == "down" {
			_, isIntfOperInd := indication.Data.(*oop.Indication_IntfOperInd)
			if isIntfOperInd {
				intfOperInd := indication.GetIntfOperInd()
				if intfOperInd.GetType() == "nni" {
					log.Infow("olt is admin down, allow nni ind", log.Fields{})
				}
			} else {
				log.Infow("olt is admin down, ignore indication", log.Fields{})
				continue
			}
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
			//onuId,err := dh.resourceMgr.GetONUID(onuDiscInd.GetIntfId())
			//onuId,err := dh.resourceMgr.GetONUID(onuDiscInd.GetIntfId())
			// TODO Get onu ID from the resource manager
			var onuId uint32 = 1
			/*if err != nil{
			    log.Errorw("onu-id-unavailable",log.Fields{"intfId":onuDiscInd.GetIntfId()})
			    return
			}*/

			sn := dh.stringifySerialNumber(onuDiscInd.SerialNumber)
			//FIXME: Duplicate child devices being create in go routine
			dh.onuDiscIndication(onuDiscInd, onuId, sn)
		case *oop.Indication_OnuInd:
			onuInd := indication.GetOnuInd()
			log.Infow("Received Onu indication ", log.Fields{"OnuInd": onuInd})
			go dh.onuIndication(onuInd)
		case *oop.Indication_OmciInd:
			omciInd := indication.GetOmciInd()
			log.Infow("Received Omci indication ", log.Fields{"OmciInd": omciInd})
			if err := dh.omciIndication(omciInd); err != nil {
				log.Errorw("send-omci-indication-errr", log.Fields{"error": err, "omciInd": omciInd})
			}
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
	dh.Client = oop.NewOpenoltClient(dh.clientCon)
	dh.transitionMap.Handle(GrpcConnected)
	return nil
}

// doStateConnected get the device info and update to voltha core
func (dh *DeviceHandler) doStateConnected() error {
	log.Debug("OLT device has been connected")
	deviceInfo, err := dh.Client.GetDeviceInfo(context.Background(), new(oop.Empty))
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
	KVStoreHostPort := fmt.Sprintf("%s:%d", dh.openOLT.KVStoreHost, dh.openOLT.KVStorePort)
	// Instantiate resource manager
	if dh.resourceMgr = rsrcMgr.NewResourceMgr(dh.deviceId, KVStoreHostPort, dh.openOLT.KVStoreType, dh.deviceType, deviceInfo); dh.resourceMgr == nil {
		log.Error("Error while instantiating resource manager")
		return errors.New("Instantiating resource manager failed")
	}
	// Instantiate flow manager
	if dh.flowMgr = NewFlowManager(dh, dh.resourceMgr); dh.flowMgr == nil {
		log.Error("Error while instantiating flow manager")
		return errors.New("Instantiating flow manager failed")
	}
	/* TODO: Instantiate Alarm , stats , BW managers */

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

func (dh *DeviceHandler) omciIndication(omciInd *oop.OmciIndication) error {
	log.Debugw("omci indication", log.Fields{"intfId": omciInd.IntfId, "onuId": omciInd.OnuId})

	//        ponPort := IntfIdToPortNo(omciInd.GetIntfId(),voltha.Port_PON_OLT)
	kwargs := make(map[string]interface{})
	kwargs["onu_id"] = omciInd.OnuId
	kwargs["parent_port_no"] = omciInd.GetIntfId()

	if onuDevice, err := dh.coreProxy.GetChildDevice(nil, dh.device.Id, kwargs); err != nil {
		log.Errorw("onu not found", log.Fields{"intfId": omciInd.IntfId, "onuId": omciInd.OnuId})
		return err
	} else {
		omciMsg := &ic.InterAdapterOmciMessage{Message: omciInd.Pkt}
		if sendErr := dh.AdapterProxy.SendInterAdapterMessage(context.Background(), omciMsg,
			ic.InterAdapterMessageType_OMCI_REQUEST, dh.deviceType, onuDevice.Type,
			onuDevice.Id, onuDevice.ProxyAddress.DeviceId, ""); sendErr != nil {
			log.Errorw("send omci request error", log.Fields{"fromAdapter": dh.deviceType, "toAdapter": onuDevice.Type, "onuId": onuDevice.Id, "proxyDeviceId": onuDevice.ProxyAddress.DeviceId})
			return sendErr
		}
		return nil
	}
}

// Process_inter_adapter_message process inter adater message
func (dh *DeviceHandler) Process_inter_adapter_message(msg *ic.InterAdapterMessage) error {
	// TODO
	log.Debugw("Process_inter_adapter_message", log.Fields{"msgId": msg.Header.Id})
	if msg.Header.Type == ic.InterAdapterMessageType_OMCI_REQUEST {
		msgId := msg.Header.Id
		fromTopic := msg.Header.FromTopic
		toTopic := msg.Header.ToTopic
		toDeviceId := msg.Header.ToDeviceId
		proxyDeviceId := msg.Header.ProxyDeviceId

		log.Debugw("omci request message header", log.Fields{"msgId": msgId, "fromTopic": fromTopic, "toTopic": toTopic, "toDeviceId": toDeviceId, "proxyDeviceId": proxyDeviceId})

		msgBody := msg.GetBody()

		omciMsg := &ic.InterAdapterOmciMessage{}
		if err := ptypes.UnmarshalAny(msgBody, omciMsg); err != nil {
			log.Warnw("cannot-unmarshal-omci-msg-body", log.Fields{"error": err})
			return err
		}

		if onuDevice, err := dh.coreProxy.GetDevice(nil, dh.device.Id, toDeviceId); err != nil {
			log.Errorw("onu not found", log.Fields{"onuDeviceId": toDeviceId, "error": err})
			return err
		} else {
			dh.sendProxiedMessage(onuDevice, omciMsg)
		}

	} else {
		log.Errorw("inter-adapter-unhandled-type", log.Fields{"msgType": msg.Header.Type})
	}
	return nil
}

func (dh *DeviceHandler) sendProxiedMessage(onuDevice *voltha.Device, omciMsg *ic.InterAdapterOmciMessage) {
	if onuDevice.ConnectStatus != voltha.ConnectStatus_REACHABLE {
		log.Debugw("ONU is not reachable, cannot send OMCI", log.Fields{"serialNumber": onuDevice.SerialNumber, "intfId": onuDevice.ProxyAddress.GetChannelId(), "onuId": onuDevice.ProxyAddress.GetOnuId()})
		return
	}

	omciMessage := &oop.OmciMsg{IntfId: onuDevice.ProxyAddress.GetChannelId(), OnuId: onuDevice.ProxyAddress.GetOnuId(), Pkt: omciMsg.Message}

	dh.Client.OmciMsgOut(context.Background(), omciMessage)
	log.Debugw("omci-message-sent", log.Fields{"serialNumber": onuDevice.SerialNumber, "intfId": onuDevice.ProxyAddress.GetChannelId(), "omciMsg": string(omciMsg.Message)})
}

func (dh *DeviceHandler) activateONU(intfId uint32, onuId int64, serialNum *oop.SerialNumber, serialNumber string) {
	log.Debugw("activate-onu", log.Fields{"intfId": intfId, "onuId": onuId, "serialNum": serialNum, "serialNumber": serialNumber})
	// TODO: need resource manager
	var pir uint32 = 1000000
	Onu := oop.Onu{IntfId: intfId, OnuId: uint32(onuId), SerialNumber: serialNum, Pir: pir}
	if _, err := dh.Client.ActivateOnu(context.Background(), &Onu); err != nil {
		log.Errorw("activate-onu-failed", log.Fields{"Onu": Onu})
	} else {
		log.Infow("activated-onu", log.Fields{"SerialNumber": serialNumber})
	}
}

func (dh *DeviceHandler) onuDiscIndication(onuDiscInd *oop.OnuDiscIndication, onuId uint32, sn string) error {
	//channelId := MkUniPortNum(onuDiscInd.GetIntfId(), onuId, uint32(0))
	//parentPortNo := IntfIdToPortNo(onuDiscInd.GetIntfId(),voltha.Port_PON_OLT)
	channelId := onuDiscInd.GetIntfId()
	parentPortNo := onuDiscInd.GetIntfId()
	if err := dh.coreProxy.ChildDeviceDetected(nil, dh.device.Id, int(parentPortNo), "brcm_openomci_onu", int(channelId), string(onuDiscInd.SerialNumber.GetVendorId()), sn, int64(onuId)); err != nil {
		log.Errorw("Create onu error", log.Fields{"parent_id": dh.device.Id, "ponPort": onuDiscInd.GetIntfId(), "onuId": onuId, "sn": sn, "error": err})
		return err
	}

	kwargs := make(map[string]interface{})
	kwargs["onu_id"] = onuId
	kwargs["parent_port_no"] = onuDiscInd.GetIntfId()

	for i := 0; i < 10; i++ {
		if onuDevice, _ := dh.coreProxy.GetChildDevice(nil, dh.device.Id, kwargs); onuDevice != nil {
			dh.activateONU(onuDiscInd.IntfId, int64(onuId), onuDiscInd.SerialNumber, sn)
			return nil
		} else {
			time.Sleep(1 * time.Second)
			log.Debugln("Sleep 1 seconds to active onu, retry times ", i+1)
		}
	}
	log.Errorw("Cannot query onu, dont activate it.", log.Fields{"parent_id": dh.device.Id, "ponPort": onuDiscInd.GetIntfId(), "onuId": onuId, "sn": sn})
	return errors.New("Failed to activate onu")
}

func (dh *DeviceHandler) onuIndication(onuInd *oop.OnuIndication) {
	serialNumber := dh.stringifySerialNumber(onuInd.SerialNumber)

	kwargs := make(map[string]interface{})
	//        ponPort := IntfIdToPortNo(onuInd.GetIntfId(),voltha.Port_PON_OLT)

	if serialNumber != "" {
		kwargs["serial_number"] = serialNumber
	} else {
		kwargs["onu_id"] = onuInd.OnuId
		kwargs["parent_port_no"] = onuInd.GetIntfId()
	}
	if onuDevice, _ := dh.coreProxy.GetChildDevice(nil, dh.device.Id, kwargs); onuDevice != nil {
		//if intfIdFromPortNo(onuDevice.ParentPortNo) != onuInd.GetIntfId() {
		if onuDevice.ParentPortNo != onuInd.GetIntfId() {
			//log.Warnw("ONU-is-on-a-different-intf-id-now", log.Fields{"previousIntfId": intfIdFromPortNo(onuDevice.ParentPortNo), "currentIntfId": onuInd.GetIntfId()})
			log.Warnw("ONU-is-on-a-different-intf-id-now", log.Fields{"previousIntfId": onuDevice.ParentPortNo, "currentIntfId": onuInd.GetIntfId()})
		}

		if onuDevice.ProxyAddress.OnuId != onuInd.OnuId {
			log.Warnw("ONU-id-mismatch, can happen if both voltha and the olt rebooted", log.Fields{"expected_onu_id": onuDevice.ProxyAddress.OnuId, "received_onu_id": onuInd.OnuId})
		}

		// adminState
		if onuInd.AdminState == "down" {
			if onuInd.OperState != "down" {
				log.Errorw("ONU-admin-state-down-and-oper-status-not-down", log.Fields{"operState": onuInd.OperState})
				// Forcing the oper state change code to execute
				onuInd.OperState = "down"
			}
			// Port and logical port update is taken care of by oper state block
		} else if onuInd.AdminState == "up" {
			log.Debugln("received-onu-admin-state up")
		} else {
			log.Errorw("Invalid-or-not-implemented-admin-state", log.Fields{"received-admin-state": onuInd.AdminState})
		}
		log.Debugln("admin-state-dealt-with")

		// operState
		if onuInd.OperState == "down" {
			if onuDevice.ConnectStatus != common.ConnectStatus_UNREACHABLE {
				dh.coreProxy.DeviceStateUpdate(nil, onuDevice.Id, common.ConnectStatus_UNREACHABLE, onuDevice.OperStatus)
				log.Debugln("onu-oper-state-is-down")
			}
			if onuDevice.OperStatus != common.OperStatus_DISCOVERED {
				dh.coreProxy.DeviceStateUpdate(nil, onuDevice.Id, common.ConnectStatus_UNREACHABLE, common.OperStatus_DISCOVERED)
			}
			log.Debugw("inter-adapter-send-onu-ind", log.Fields{"onuIndication": onuInd})

			// TODO NEW CORE do not hardcode adapter name. Handler needs Adapter reference
			dh.AdapterProxy.SendInterAdapterMessage(nil, onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST, "openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		} else if onuInd.OperState == "up" {
			if onuDevice.ConnectStatus != common.ConnectStatus_REACHABLE {
				dh.coreProxy.DeviceStateUpdate(nil, onuDevice.Id, common.ConnectStatus_REACHABLE, onuDevice.OperStatus)

			}
			if onuDevice.OperStatus != common.OperStatus_DISCOVERED {
				log.Warnw("ignore onu indication", log.Fields{"intfId": onuInd.IntfId, "onuId": onuInd.OnuId, "operStatus": onuDevice.OperStatus, "msgOperStatus": onuInd.OperState})
				return
			}
			dh.AdapterProxy.SendInterAdapterMessage(nil, onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST, "openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		} else {
			log.Warnw("Not-implemented-or-invalid-value-of-oper-state", log.Fields{"operState": onuInd.OperState})
		}
	} else {
		log.Errorw("onu not found", log.Fields{"intfId": onuInd.IntfId, "onuId": onuInd.OnuId})
		return
	}

}

func (dh *DeviceHandler) stringifySerialNumber(serialNum *oop.SerialNumber) string {
	if serialNum != nil {
		return string(serialNum.VendorId) + dh.stringifyVendorSpecific(serialNum.VendorSpecific)
	} else {
		return ""
	}
}

func (dh *DeviceHandler) stringifyVendorSpecific(vendorSpecific []byte) string {
	tmp := fmt.Sprintf("%x", (uint32(vendorSpecific[0])>>4)&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[0]&0x0f))) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[1])>>4)&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[1]))&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[2])>>4)&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[2]))&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[3])>>4)&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[3]))&0x0f)
	return tmp
}

// flows
func (dh *DeviceHandler) Update_flows_bulk() error {
	return errors.New("UnImplemented")
}
func (dh *DeviceHandler) GetChildDevice(parentPort uint32, onuId uint32) *voltha.Device {
	log.Debugw("GetChildDevice", log.Fields{"pon port": parentPort, "onuId": onuId})
	kwargs := make(map[string]interface{})
	kwargs["onu_id"] = onuId
	kwargs["parent_port_no"] = parentPort
	onuDevice, err := dh.coreProxy.GetChildDevice(nil, dh.device.Id, kwargs)
	if err != nil {
		log.Errorw("onu not found", log.Fields{"intfId": parentPort, "onuId": onuId})
		return nil
	}
	log.Debugw("Successfully received child device from core", log.Fields{"child_device": *onuDevice})
	return onuDevice
}

func (dh *DeviceHandler) UpdateFlowsIncrementally(device *voltha.Device, flows *of.FlowChanges, groups *of.FlowGroupChanges) error {
	log.Debugw("In UpdateFlowsIncrementally", log.Fields{"deviceId": device.Id, "flows": flows, "groups": groups})
	if flows != nil {
		for _, flow := range flows.ToAdd.Items {
			dh.flowMgr.AddFlow(flow)
		}
	}
	if groups != nil {
		for _, flow := range flows.ToRemove.Items {
			log.Debug("Removing flow", log.Fields{"deviceId": device.Id, "flowToRemove": flow})
			//  dh.flowMgr.RemoveFlow(flow)
		}
	}
	return nil
}

func (dh *DeviceHandler) DisableDevice(device *voltha.Device) error {
	if _, err := dh.Client.DisableOlt(context.Background(), new(oop.Empty)); err != nil {
		log.Errorw("Failed to disable olt ", log.Fields{"err": err})
		return err
	}
	dh.lockDevice.Lock()
	dh.adminState = "down"
	dh.lockDevice.Unlock()
	log.Debug("olt-disabled")

	cloned := proto.Clone(device).(*voltha.Device)
	// Update the all ports state on that device to disable
	if err := dh.coreProxy.PortsStateUpdate(nil, cloned.Id, voltha.OperStatus_UNKNOWN); err != nil {
		log.Errorw("updating-ports-failed", log.Fields{"deviceId": device.Id, "error": err})
		return err
	}

	//Update the device oper state
	cloned.OperStatus = voltha.OperStatus_UNKNOWN
	dh.device = cloned

	if err := dh.coreProxy.DeviceStateUpdate(nil, cloned.Id, cloned.ConnectStatus, cloned.OperStatus); err != nil {
		log.Errorw("error-updating-device-state", log.Fields{"deviceId": device.Id, "error": err})
		return err
	}
	log.Debugw("DisableDevice-end", log.Fields{"deviceId": device.Id})
	return nil
}

func (dh *DeviceHandler) ReenableDevice(device *voltha.Device) error {
	if _, err := dh.Client.ReenableOlt(context.Background(), new(oop.Empty)); err != nil {
		log.Errorw("Failed to reenable olt ", log.Fields{"err": err})
		return err
	}

	dh.lockDevice.Lock()
	dh.adminState = "up"
	dh.lockDevice.Unlock()
	log.Debug("olt-reenabled")

	cloned := proto.Clone(device).(*voltha.Device)
	// Update the all ports state on that device to enable
	if err := dh.coreProxy.PortsStateUpdate(nil, cloned.Id, voltha.OperStatus_ACTIVE); err != nil {
		log.Errorw("updating-ports-failed", log.Fields{"deviceId": device.Id, "error": err})
		return err
	}

	//Update the device oper status as ACTIVE
	cloned.OperStatus = voltha.OperStatus_ACTIVE
	dh.device = cloned

	if err := dh.coreProxy.DeviceStateUpdate(nil, cloned.Id, cloned.ConnectStatus, cloned.OperStatus); err != nil {
		log.Errorw("error-updating-device-state", log.Fields{"deviceId": device.Id, "error": err})
		return err
	}
	log.Debugw("ReEnableDevice-end", log.Fields{"deviceId": device.Id})

	return nil
}
