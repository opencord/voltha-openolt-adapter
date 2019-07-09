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
	"google.golang.org/grpc/codes"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/mdlayher/ethernet"
	com "github.com/opencord/voltha-go/adapters/common"
	"github.com/opencord/voltha-go/common/log"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	"github.com/opencord/voltha-protos/go/common"
	ic "github.com/opencord/voltha-protos/go/inter_container"
	of "github.com/opencord/voltha-protos/go/openflow_13"
	oop "github.com/opencord/voltha-protos/go/openolt"
	"github.com/opencord/voltha-protos/go/voltha"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

//DeviceHandler will interact with the OLT device.
type DeviceHandler struct {
	deviceID      string
	deviceType    string
	adminState    string
	device        *voltha.Device
	coreProxy     *com.CoreProxy
	AdapterProxy  *com.AdapterProxy
        EventProxy    *com.EventProxy
	openOLT       *OpenOLT
	exitChannel   chan int
	lockDevice    sync.RWMutex
	Client        oop.OpenoltClient
	transitionMap *TransitionMap
	clientCon     *grpc.ClientConn
	flowMgr       *OpenOltFlowMgr
        eventMgr      *OpenOltEventMgr
	resourceMgr   *rsrcMgr.OpenOltResourceMgr
	discOnus      map[string]bool
	onus          map[string]*OnuDevice
	nniIntfID     int
}

//OnuDevice represents ONU related info
type OnuDevice struct {
	deviceID      string
	deviceType    string
	serialNumber  string
	onuID         uint32
	intfID        uint32
	proxyDeviceID string
}

//NewOnuDevice creates a new Onu Device
func NewOnuDevice(devID, deviceTp, serialNum string, onuID, intfID uint32, proxyDevID string) *OnuDevice {
	var device OnuDevice
	device.deviceID = devID
	device.deviceType = deviceTp
	device.serialNumber = serialNum
	device.onuID = onuID
	device.intfID = intfID
	device.proxyDeviceID = proxyDevID
	return &device
}

//NewDeviceHandler creates a new device handler
func NewDeviceHandler(cp *com.CoreProxy, ap *com.AdapterProxy, ep *com.EventProxy, device *voltha.Device, adapter *OpenOLT) *DeviceHandler {
	var dh DeviceHandler
	dh.coreProxy = cp
	dh.AdapterProxy = ap
        dh.EventProxy = ep
	cloned := (proto.Clone(device)).(*voltha.Device)
	dh.deviceID = cloned.Id
	dh.deviceType = cloned.Type
	dh.adminState = "up"
	dh.device = cloned
	dh.openOLT = adapter
	dh.exitChannel = make(chan int, 1)
	dh.discOnus = make(map[string]bool)
	dh.lockDevice = sync.RWMutex{}
	dh.onus = make(map[string]*OnuDevice)
	// The nniIntfID is initialized to -1 (invalid) and set to right value
	// when the first IntfOperInd with status as "up" is received for
	// any one of the available NNI port on the OLT device.
	dh.nniIntfID = -1

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

//GetportLabel returns the label for the NNI and the PON port based on port number and port type
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

func (dh *DeviceHandler) addPort(intfID uint32, portType voltha.Port_PortType, state string) {
	var operStatus common.OperStatus_OperStatus
	if state == "up" {
		operStatus = voltha.OperStatus_ACTIVE
	} else {
		operStatus = voltha.OperStatus_DISCOVERED
	}
	portNum := IntfIDToPortNo(intfID, portType)
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
	if err := dh.coreProxy.PortCreated(context.TODO(), dh.device.Id, port); err != nil {
		log.Errorw("error-creating-nni-port", log.Fields{"deviceID": dh.device.Id, "portType": portType, "error": err})
	}

	// Once we have successfully added the NNI port to the core, if the
	// locally cached nniIntfID is set to invalid (-1), set it to the right value.
	if portType == voltha.Port_ETHERNET_NNI && dh.nniIntfID == -1 {
		dh.nniIntfID = int(intfID)
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
	device, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		log.Errorw("Failed to fetch device info", log.Fields{"err": err})
		return
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
			dh.transitionMap.Handle(DeviceDownInd)
			dh.transitionMap.Handle(DeviceInit)
			break
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

		dh.handleIndication(indication)

	}
}

func (dh *DeviceHandler) handleOltIndication(oltIndication *oop.OltIndication) {
	if oltIndication.OperState == "up" {
		dh.transitionMap.Handle(DeviceUpInd)
	} else if oltIndication.OperState == "down" {
		dh.transitionMap.Handle(DeviceDownInd)
	}
}

func (dh *DeviceHandler) handleIndication(indication *oop.Indication) {
        raisedTs := time.Now().UnixNano()
	switch indication.Data.(type) {
	case *oop.Indication_OltInd:
		dh.handleOltIndication(indication.GetOltInd())
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
		sn := dh.stringifySerialNumber(onuDiscInd.SerialNumber)
		dh.onuDiscIndication(onuDiscInd, sn)
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
		go dh.handlePacketIndication(pktInd)
	case *oop.Indication_PortStats:
		portStats := indication.GetPortStats()
		log.Infow("Received port stats indication", log.Fields{"PortStats": portStats})
	case *oop.Indication_FlowStats:
		flowStats := indication.GetFlowStats()
		log.Infow("Received flow stats", log.Fields{"FlowStats": flowStats})
	case *oop.Indication_AlarmInd:
		alarmInd := indication.GetAlarmInd()
		log.Infow("Received alarm indication ", log.Fields{"AlarmInd": alarmInd})
                if dh.eventMgr.ProcessEvents(alarmInd, dh.deviceId, raisedTs); err != nil {
                      log.Errorw("Failed to process event indication", log.Fields{"EventInd": alarmInd, "Error": err})
                }

	}
}

// doStateUp handle the olt up indication and update to voltha core
func (dh *DeviceHandler) doStateUp() error {
	// Synchronous call to update device state - this method is run in its own go routine
	if err := dh.coreProxy.DeviceStateUpdate(context.Background(), dh.device.Id, voltha.ConnectStatus_REACHABLE,
		voltha.OperStatus_ACTIVE); err != nil {
		log.Errorw("Failed to update device with OLT UP indication", log.Fields{"deviceID": dh.device.Id, "error": err})
		return err
	}
	return nil
}

// doStateDown handle the olt down indication
func (dh *DeviceHandler) doStateDown() error {
	log.Debug("do-state-down-start")

	device, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		log.Errorw("Failed to fetch device device", log.Fields{"err": err})
		return errors.New("failed to fetch device device")
	}

	cloned := proto.Clone(device).(*voltha.Device)
	// Update the all ports state on that device to disable
	if er := dh.coreProxy.PortsStateUpdate(context.TODO(), cloned.Id, voltha.OperStatus_UNKNOWN); er != nil {
		log.Errorw("updating-ports-failed", log.Fields{"deviceID": device.Id, "error": er})
		return er
	}

	//Update the device oper state and connection status
	cloned.OperStatus = voltha.OperStatus_UNKNOWN
	cloned.ConnectStatus = common.ConnectStatus_UNREACHABLE
	dh.device = cloned

	if er := dh.coreProxy.DeviceStateUpdate(context.TODO(), cloned.Id, cloned.ConnectStatus, cloned.OperStatus); er != nil {
		log.Errorw("error-updating-device-state", log.Fields{"deviceID": device.Id, "error": er})
		return er
	}

	//get the child device for the parent device
	onuDevices, err := dh.coreProxy.GetChildDevices(context.TODO(), dh.device.Id)
	if err != nil {
		log.Errorw("failed to get child devices information", log.Fields{"deviceID": dh.device.Id, "error": err})
		return err
	}
	for _, onuDevice := range onuDevices.Items {

		// Update onu state as down in onu adapter
		onuInd := oop.OnuIndication{}
		onuInd.OperState = "down"
		er := dh.AdapterProxy.SendInterAdapterMessage(context.TODO(), &onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
			"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		if er != nil {
			log.Errorw("Failed to send inter-adapter-message", log.Fields{"OnuInd": onuInd,
				"From Adapter": "openolt", "DevieType": onuDevice.Type, "DeviceID": onuDevice.Id})
			return er
		}
	}
	log.Debugw("do-state-down-end", log.Fields{"deviceID": device.Id})
	return nil
}

// doStateInit dial the grpc before going to init state
func (dh *DeviceHandler) doStateInit() error {
	var err error
	dh.clientCon, err = grpc.Dial(dh.device.GetHostAndPort(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Errorw("Failed to dial device", log.Fields{"DeviceId": dh.deviceID, "HostAndPort": dh.device.GetHostAndPort(), "err": err})
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

	// Case where OLT is disabled and then rebooted.
	if dh.adminState == "down" {
		log.Debugln("do-state-connected--device-admin-state-down")
		device, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, dh.device.Id)
		if err != nil || device == nil {
			/*TODO: needs to handle error scenarios */
			log.Errorw("Failed to fetch device device", log.Fields{"err": err})
		}

		cloned := proto.Clone(device).(*voltha.Device)
		cloned.ConnectStatus = voltha.ConnectStatus_REACHABLE
		cloned.OperStatus = voltha.OperStatus_UNKNOWN
		dh.device = cloned
		if er := dh.coreProxy.DeviceStateUpdate(context.TODO(), cloned.Id, cloned.ConnectStatus, cloned.OperStatus); er != nil {
			log.Errorw("error-updating-device-state", log.Fields{"deviceID": dh.device.Id, "error": er})
		}

		// Since the device was disabled before the OLT was rebooted, enfore the OLT to be Disabled after re-connection.
		_, err = dh.Client.DisableOlt(context.Background(), new(oop.Empty))
		if err != nil {
			log.Errorw("Failed to disable olt ", log.Fields{"err": err})
		}

		// Start reading indications
		go dh.readIndications()
		return nil
	}

	deviceInfo, err := dh.Client.GetDeviceInfo(context.Background(), new(oop.Empty))
	if err != nil {
		log.Errorw("Failed to fetch device info", log.Fields{"err": err})
		return err
	}
	if deviceInfo == nil {
		log.Errorw("Device info is nil", log.Fields{})
		return errors.New("failed to get device info from OLT")
	}
	log.Debugw("Fetched device info", log.Fields{"deviceInfo": deviceInfo})
	dh.device.Root = true
	dh.device.Vendor = deviceInfo.Vendor
	dh.device.Model = deviceInfo.Model
	dh.device.SerialNumber = deviceInfo.DeviceSerialNumber
	dh.device.HardwareVersion = deviceInfo.HardwareVersion
	dh.device.FirmwareVersion = deviceInfo.FirmwareVersion
	// FIXME: Remove Hardcodings
	dh.device.MacAddress = "0a:0b:0c:0d:0e:0f"

	// Synchronous call to update device - this method is run in its own go routine
	if er := dh.coreProxy.DeviceUpdate(context.TODO(), dh.device); er != nil {
		log.Errorw("error-updating-device", log.Fields{"deviceID": dh.device.Id, "error": er})
	}

	device, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		log.Errorw("Failed to fetch device device", log.Fields{"err": err})
		return err
	}
	cloned := proto.Clone(device).(*voltha.Device)
	// Update the all ports (if available) on that device to ACTIVE.
	// The ports do not normally exist, unless the device is coming back from a reboot
	if err := dh.coreProxy.PortsStateUpdate(context.TODO(), cloned.Id, voltha.OperStatus_ACTIVE); err != nil {
		log.Errorw("updating-ports-failed", log.Fields{"deviceID": device.Id, "error": err})
		return err
	}

	KVStoreHostPort := fmt.Sprintf("%s:%d", dh.openOLT.KVStoreHost, dh.openOLT.KVStorePort)
	// Instantiate resource manager
	if dh.resourceMgr = rsrcMgr.NewResourceMgr(dh.deviceID, KVStoreHostPort, dh.openOLT.KVStoreType, dh.deviceType, deviceInfo); dh.resourceMgr == nil {
		log.Error("Error while instantiating resource manager")
		return errors.New("instantiating resource manager failed")
	}
	// Instantiate flow manager
	if dh.flowMgr = NewFlowManager(dh, dh.resourceMgr); dh.flowMgr == nil {
		log.Error("Error while instantiating flow manager")
		return errors.New("instantiating flow manager failed")
	}
	/* TODO: Instantiate Alarm , stats , BW managers */
        /* Instantiating Event Manager to handle Alarms and KPIs */
        dh.eventMgr = NewEventMgr(dh.EventProxy)

	// Start reading indications
	go dh.readIndications()
	return nil
}

//AdoptDevice adopts the OLT device
func (dh *DeviceHandler) AdoptDevice(device *voltha.Device) {
	dh.transitionMap = NewTransitionMap(dh)
	log.Infow("Adopt_device", log.Fields{"deviceID": device.Id, "Address": device.GetHostAndPort()})
	dh.transitionMap.Handle(DeviceInit)
}

//GetOfpDeviceInfo Gets the Ofp information of the given device
func (dh *DeviceHandler) GetOfpDeviceInfo(device *voltha.Device) (*ic.SwitchCapability, error) {
	return &ic.SwitchCapability{
		Desc: &of.OfpDesc{
			MfrDesc:   "VOLTHA Project",
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

//GetOfpPortInfo Get Ofp port information
func (dh *DeviceHandler) GetOfpPortInfo(device *voltha.Device, portNo int64) (*ic.PortCapability, error) {
	capacity := uint32(of.OfpPortFeatures_OFPPF_1GB_FD | of.OfpPortFeatures_OFPPF_FIBER)
	return &ic.PortCapability{
		Port: &voltha.LogicalPort{
			OfpPort: &of.OfpPort{
				HwAddr:     macAddressToUint32Array(dh.device.MacAddress),
				Config:     0,
				State:      uint32(of.OfpPortState_OFPPS_LIVE),
				Curr:       capacity,
				Advertised: capacity,
				Peer:       capacity,
				CurrSpeed:  uint32(of.OfpPortFeatures_OFPPF_1GB_FD),
				MaxSpeed:   uint32(of.OfpPortFeatures_OFPPF_1GB_FD),
			},
			DeviceId:     dh.device.Id,
			DevicePortNo: uint32(portNo),
		},
	}, nil
}

func (dh *DeviceHandler) omciIndication(omciInd *oop.OmciIndication) error {
	log.Debugw("omci indication", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
	var deviceType string
	var deviceID string
	var proxyDeviceID string

	onuKey := dh.formOnuKey(omciInd.IntfId, omciInd.OnuId)
	if onuInCache, ok := dh.onus[onuKey]; !ok {
		log.Debugw("omci indication for a device not in cache.", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
		ponPort := IntfIDToPortNo(omciInd.GetIntfId(), voltha.Port_PON_OLT)
		kwargs := make(map[string]interface{})
		kwargs["onu_id"] = omciInd.OnuId
		kwargs["parent_port_no"] = ponPort

		onuDevice, err := dh.coreProxy.GetChildDevice(context.TODO(), dh.device.Id, kwargs)
		if err != nil {
			log.Errorw("onu not found", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
			return err
		}
		deviceType = onuDevice.Type
		deviceID = onuDevice.Id
		proxyDeviceID = onuDevice.ProxyAddress.DeviceId
		//if not exist in cache, then add to cache.
		dh.onus[onuKey] = NewOnuDevice(deviceID, deviceType, onuDevice.SerialNumber, omciInd.OnuId, omciInd.IntfId, proxyDeviceID)
	} else {
		//found in cache
		log.Debugw("omci indication for a device in cache.", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
		deviceType = onuInCache.deviceType
		deviceID = onuInCache.deviceID
		proxyDeviceID = onuInCache.proxyDeviceID
	}

	omciMsg := &ic.InterAdapterOmciMessage{Message: omciInd.Pkt}
	if sendErr := dh.AdapterProxy.SendInterAdapterMessage(context.Background(), omciMsg,
		ic.InterAdapterMessageType_OMCI_REQUEST, dh.deviceType, deviceType,
		deviceID, proxyDeviceID, ""); sendErr != nil {
		log.Errorw("send omci request error", log.Fields{"fromAdapter": dh.deviceType, "toAdapter": deviceType, "onuID": deviceID, "proxyDeviceID": proxyDeviceID})
		return sendErr
	}
	return nil
}

//ProcessInterAdapterMessage sends the proxied messages to the target device
// If the proxy address is not found in the unmarshalled message, it first fetches the onu device for which the message
// is meant, and then send the unmarshalled omci message to this onu
func (dh *DeviceHandler) ProcessInterAdapterMessage(msg *ic.InterAdapterMessage) error {
	log.Debugw("Process_inter_adapter_message", log.Fields{"msgID": msg.Header.Id})
	if msg.Header.Type == ic.InterAdapterMessageType_OMCI_REQUEST {
		msgID := msg.Header.Id
		fromTopic := msg.Header.FromTopic
		toTopic := msg.Header.ToTopic
		toDeviceID := msg.Header.ToDeviceId
		proxyDeviceID := msg.Header.ProxyDeviceId

		log.Debugw("omci request message header", log.Fields{"msgID": msgID, "fromTopic": fromTopic, "toTopic": toTopic, "toDeviceID": toDeviceID, "proxyDeviceID": proxyDeviceID})

		msgBody := msg.GetBody()

		omciMsg := &ic.InterAdapterOmciMessage{}
		if err := ptypes.UnmarshalAny(msgBody, omciMsg); err != nil {
			log.Warnw("cannot-unmarshal-omci-msg-body", log.Fields{"error": err})
			return err
		}

		if omciMsg.GetProxyAddress() == nil {
			onuDevice, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, toDeviceID)
			if err != nil {
				log.Errorw("onu not found", log.Fields{"onuDeviceId": toDeviceID, "error": err})
				return err
			}
			log.Debugw("device retrieved from core", log.Fields{"msgID": msgID, "fromTopic": fromTopic, "toTopic": toTopic, "toDeviceID": toDeviceID, "proxyDeviceID": proxyDeviceID})
			dh.sendProxiedMessage(onuDevice, omciMsg)

		} else {
			log.Debugw("Proxy Address found in omci message", log.Fields{"msgID": msgID, "fromTopic": fromTopic, "toTopic": toTopic, "toDeviceID": toDeviceID, "proxyDeviceID": proxyDeviceID})
			dh.sendProxiedMessage(nil, omciMsg)
		}

	} else {
		log.Errorw("inter-adapter-unhandled-type", log.Fields{"msgType": msg.Header.Type})
	}
	return nil
}

func (dh *DeviceHandler) sendProxiedMessage(onuDevice *voltha.Device, omciMsg *ic.InterAdapterOmciMessage) {
	var intfID uint32
	var onuID uint32
	var connectStatus common.ConnectStatus_ConnectStatus
	if onuDevice != nil {
		intfID = onuDevice.ProxyAddress.GetChannelId()
		onuID = onuDevice.ProxyAddress.GetOnuId()
		connectStatus = onuDevice.ConnectStatus
	} else {
		intfID = omciMsg.GetProxyAddress().GetChannelId()
		onuID = omciMsg.GetProxyAddress().GetOnuId()
		connectStatus = omciMsg.GetConnectStatus()
	}
	if connectStatus != voltha.ConnectStatus_REACHABLE {
		log.Debugw("ONU is not reachable, cannot send OMCI", log.Fields{"intfID": intfID, "onuID": onuID})
		return
	}

	omciMessage := &oop.OmciMsg{IntfId: intfID, OnuId: onuID, Pkt: omciMsg.Message}

	_, err := dh.Client.OmciMsgOut(context.Background(), omciMessage)
	if err != nil {
		log.Errorw("unable to send omci-msg-out", log.Fields{"IntfID": intfID, "OnuID": onuID, "Msg": omciMessage})
		return
	}
	log.Debugw("omci-message-sent", log.Fields{"intfID": intfID, "onuID": onuID, "omciMsg": string(omciMsg.GetMessage())})
}

func (dh *DeviceHandler) activateONU(intfID uint32, onuID int64, serialNum *oop.SerialNumber, serialNumber string) {
	log.Debugw("activate-onu", log.Fields{"intfID": intfID, "onuID": onuID, "serialNum": serialNum, "serialNumber": serialNumber})
	dh.flowMgr.UpdateOnuInfo(intfID, uint32(onuID), serialNumber)
	// TODO: need resource manager
	var pir uint32 = 1000000
	Onu := oop.Onu{IntfId: intfID, OnuId: uint32(onuID), SerialNumber: serialNum, Pir: pir}
	if _, err := dh.Client.ActivateOnu(context.Background(), &Onu); err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.AlreadyExists {
			log.Debug("ONU activation is in progress", log.Fields{"SerialNumber": serialNumber})
		} else {
			log.Errorw("activate-onu-failed", log.Fields{"Onu": Onu, "err ": err})
		}
	} else {
		log.Infow("activated-onu", log.Fields{"SerialNumber": serialNumber})
	}
}

func (dh *DeviceHandler) onuDiscIndication(onuDiscInd *oop.OnuDiscIndication, sn string) error {
	channelID := onuDiscInd.GetIntfId()
	parentPortNo := IntfIDToPortNo(onuDiscInd.GetIntfId(), voltha.Port_PON_OLT)
	if _, ok := dh.discOnus[sn]; ok {
		log.Debugw("onu-sn-is-already-being-processed", log.Fields{"sn": sn})
		return nil
	}

	dh.lockDevice.Lock()
	dh.discOnus[sn] = true
	dh.lockDevice.Unlock()
	// evict the onu serial number from local cache
	defer func() {
		delete(dh.discOnus, sn)
	}()

	kwargs := make(map[string]interface{})
	if sn != "" {
		kwargs["serial_number"] = sn
	} else {
		log.Error("invalid onu serial number")
		return errors.New("failed to fetch onu serial number")
	}

	onuDevice, err := dh.coreProxy.GetChildDevice(context.TODO(), dh.device.Id, kwargs)
	var onuID uint32
	if onuDevice == nil || err != nil {
		//This is the first time ONU discovered. Create an OnuID for it.
		onuID, err = dh.resourceMgr.GetONUID(onuDiscInd.GetIntfId())
		if err != nil {
			log.Errorw("failed to fetch onuID from resource manager", log.Fields{"err": err})
			return err
		}
		if onuDevice, err = dh.coreProxy.ChildDeviceDetected(context.TODO(), dh.device.Id, int(parentPortNo),
			"brcm_openomci_onu", int(channelID),
			string(onuDiscInd.SerialNumber.GetVendorId()), sn, int64(onuID)); onuDevice == nil {
			log.Errorw("Create onu error",
				log.Fields{"parent_id": dh.device.Id, "ponPort": onuDiscInd.GetIntfId(),
					"onuID": onuID, "sn": sn, "error": err})
			return err
		}

	} else {
		//ONU already discovered before. Use the same OnuID.
		onuID = onuDevice.ProxyAddress.OnuId
	}
	//Insert the ONU into cache to use in OnuIndication.
	//TODO: Do we need to remove this from the cache on ONU change, or wait for overwritten on next discovery.
	onuKey := dh.formOnuKey(onuDiscInd.GetIntfId(), onuID)
	dh.onus[onuKey] = NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuID, onuDiscInd.GetIntfId(), onuDevice.ProxyAddress.DeviceId)

	err = dh.coreProxy.DeviceStateUpdate(context.TODO(), onuDevice.Id, common.ConnectStatus_REACHABLE, common.OperStatus_DISCOVERED)
	if err != nil {
		log.Errorw("failed to update device state", log.Fields{"DeviceID": onuDevice.Id})
		return err
	}
	log.Debugw("onu-discovered-reachable", log.Fields{"deviceId": onuDevice.Id})
	//TODO: We put this sleep here to prevent the race between state update and onuIndication
	//In onuIndication the operStatus of device is checked. If it is still not updated in KV store
	//then the initialisation fails.
	time.Sleep(1 * time.Second)
	dh.activateONU(onuDiscInd.IntfId, int64(onuID), onuDiscInd.SerialNumber, sn)
	return nil
}

func (dh *DeviceHandler) onuIndication(onuInd *oop.OnuIndication) {
	serialNumber := dh.stringifySerialNumber(onuInd.SerialNumber)

	kwargs := make(map[string]interface{})
	ponPort := IntfIDToPortNo(onuInd.GetIntfId(), voltha.Port_PON_OLT)
	var onuDevice *voltha.Device
	foundInCache := false
	onuKey := dh.formOnuKey(onuInd.GetIntfId(), onuInd.OnuId)
	if onuInCache, ok := dh.onus[onuKey]; ok {
		//If ONU id is discovered before then use GetDevice to get onuDevice because it is cheaper.
		foundInCache = true
		onuDevice, _ = dh.coreProxy.GetDevice(nil, dh.device.Id, onuInCache.deviceID)
	} else {
		//If ONU not found in adapter cache then we have to use GetChildDevice to get onuDevice
		if serialNumber != "" {
			kwargs["serial_number"] = serialNumber
		} else {
			kwargs["onu_id"] = onuInd.OnuId
			kwargs["parent_port_no"] = ponPort
		}
		onuDevice, _ = dh.coreProxy.GetChildDevice(context.TODO(), dh.device.Id, kwargs)
	}

	if onuDevice != nil {
		if onuDevice.ParentPortNo != ponPort {
			//log.Warnw("ONU-is-on-a-different-intf-id-now", log.Fields{"previousIntfId": intfIDFromPortNo(onuDevice.ParentPortNo), "currentIntfId": onuInd.GetIntfId()})
			log.Warnw("ONU-is-on-a-different-intf-id-now", log.Fields{"previousIntfId": onuDevice.ParentPortNo, "currentIntfId": ponPort})
		}

		if onuDevice.ProxyAddress.OnuId != onuInd.OnuId {
			log.Warnw("ONU-id-mismatch, can happen if both voltha and the olt rebooted", log.Fields{"expected_onu_id": onuDevice.ProxyAddress.OnuId, "received_onu_id": onuInd.OnuId})
		}
		if !foundInCache {
			onuKey := dh.formOnuKey(onuInd.GetIntfId(), onuInd.GetOnuId())
			dh.onus[onuKey] = NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuInd.GetOnuId(), onuInd.GetIntfId(), onuDevice.ProxyAddress.DeviceId)
		}
		dh.updateOnuStates(onuDevice, onuInd)

	} else {
		log.Errorw("onu not found", log.Fields{"intfID": onuInd.IntfId, "onuID": onuInd.OnuId})
		return
	}

}

func (dh *DeviceHandler) updateOnuStates(onuDevice *voltha.Device, onuInd *oop.OnuIndication) {
	dh.updateOnuAdminState(onuInd)
	// operState
	if onuInd.OperState == "down" {
		if onuDevice.ConnectStatus != common.ConnectStatus_UNREACHABLE {
			err := dh.coreProxy.DeviceStateUpdate(context.TODO(), onuDevice.Id, common.ConnectStatus_UNREACHABLE,
				onuDevice.OperStatus)
			if err != nil {
				log.Errorw("unable to update onu state", log.Fields{"DeviceID": onuDevice.Id})
				return
			}
			log.Debugln("onu-oper-state-is-down")
		}
		if onuDevice.OperStatus != common.OperStatus_DISCOVERED {
			err := dh.coreProxy.DeviceStateUpdate(context.TODO(), onuDevice.Id, common.ConnectStatus_UNREACHABLE,
				common.OperStatus_DISCOVERED)
			if err != nil {
				log.Errorw("unable to update onu state", log.Fields{"DeviceID": onuDevice.Id})
				return
			}
		}
		log.Debugw("inter-adapter-send-onu-ind", log.Fields{"onuIndication": onuInd})

		// TODO NEW CORE do not hardcode adapter name. Handler needs Adapter reference
		err := dh.AdapterProxy.SendInterAdapterMessage(context.TODO(), onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
			"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		if err != nil {
			log.Errorw("Failed to send inter-adapter-message", log.Fields{"OnuInd": onuInd,
				"From Adapter": "openolt", "DevieType": onuDevice.Type, "DeviceID": onuDevice.Id})
		}
	} else if onuInd.OperState == "up" {
		if onuDevice.ConnectStatus != common.ConnectStatus_REACHABLE {
			err := dh.coreProxy.DeviceStateUpdate(context.TODO(), onuDevice.Id, common.ConnectStatus_REACHABLE, onuDevice.OperStatus)
			if err != nil {
				log.Errorw("unable to update onu state", log.Fields{"DeviceID": onuDevice.Id})
				return
			}
		}
		if onuDevice.OperStatus != common.OperStatus_DISCOVERED {
			log.Warnw("ignore onu indication", log.Fields{"intfID": onuInd.IntfId, "onuID": onuInd.OnuId, "operStatus": onuDevice.OperStatus, "msgOperStatus": onuInd.OperState})
			return
		}
		err := dh.AdapterProxy.SendInterAdapterMessage(context.TODO(), onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
			"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		if err != nil {
			log.Errorw("Failed to send inter-adapter-message", log.Fields{"OnuInd": onuInd,
				"From Adapter": "openolt", "DevieType": onuDevice.Type, "DeviceID": onuDevice.Id})
			return
		}
	} else {
		log.Warnw("Not-implemented-or-invalid-value-of-oper-state", log.Fields{"operState": onuInd.OperState})
	}
}

func (dh *DeviceHandler) updateOnuAdminState(onuInd *oop.OnuIndication) {
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
}

func (dh *DeviceHandler) stringifySerialNumber(serialNum *oop.SerialNumber) string {
	if serialNum != nil {
		return string(serialNum.VendorId) + dh.stringifyVendorSpecific(serialNum.VendorSpecific)
	}
	return ""
}

func (dh *DeviceHandler) stringifyVendorSpecific(vendorSpecific []byte) string {
	tmp := fmt.Sprintf("%x", (uint32(vendorSpecific[0])>>4)&0x0f) +
		fmt.Sprintf("%x", uint32(vendorSpecific[0]&0x0f)) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[1])>>4)&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[1]))&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[2])>>4)&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[2]))&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[3])>>4)&0x0f) +
		fmt.Sprintf("%x", (uint32(vendorSpecific[3]))&0x0f)
	return tmp
}

//UpdateFlowsBulk upates the bulk flow
func (dh *DeviceHandler) UpdateFlowsBulk() error {
	return errors.New("unimplemented")
}

//GetChildDevice returns the child device for given parent port and onu id
func (dh *DeviceHandler) GetChildDevice(parentPort, onuID uint32) *voltha.Device {
	log.Debugw("GetChildDevice", log.Fields{"pon port": parentPort, "onuID": onuID})
	kwargs := make(map[string]interface{})
	kwargs["onu_id"] = onuID
	kwargs["parent_port_no"] = parentPort
	onuDevice, err := dh.coreProxy.GetChildDevice(context.TODO(), dh.device.Id, kwargs)
	if err != nil {
		log.Errorw("onu not found", log.Fields{"intfID": parentPort, "onuID": onuID})
		return nil
	}
	log.Debugw("Successfully received child device from core", log.Fields{"child_device": *onuDevice})
	return onuDevice
}

// SendPacketInToCore sends packet-in to core
// For this, it calls SendPacketIn of the core-proxy which uses a device specific topic to send the request.
// The adapter handling the device creates a device specific topic
func (dh *DeviceHandler) SendPacketInToCore(logicalPort uint32, packetPayload []byte) {
	log.Debugw("SendPacketInToCore", log.Fields{"port": logicalPort, "packetPayload": packetPayload})
	if err := dh.coreProxy.SendPacketIn(context.TODO(), dh.device.Id, logicalPort, packetPayload); err != nil {
		log.Errorw("Error sending packetin to core", log.Fields{"error": err})
		return
	}
	log.Debug("Sent packet-in to core successfully")
}

//UpdateFlowsIncrementally updates the device flow
func (dh *DeviceHandler) UpdateFlowsIncrementally(device *voltha.Device, flows *of.FlowChanges, groups *of.FlowGroupChanges) error {
	log.Debugw("In Update_flows_incrementally", log.Fields{"deviceID": device.Id, "flows": flows, "groups": groups})
	if flows != nil {
		for _, flow := range flows.ToAdd.Items {
			log.Debug("Adding flow", log.Fields{"deviceId": device.Id, "flowToAdd": flow})
			dh.flowMgr.AddFlow(flow)
		}
		for _, flow := range flows.ToRemove.Items {
			log.Debug("Removing flow", log.Fields{"deviceId": device.Id, "flowToRemove": flow})
			dh.flowMgr.RemoveFlow(flow)
		}
	}
	if groups != nil && flows != nil {
		for _, flow := range flows.ToRemove.Items {
			log.Debug("Removing flow", log.Fields{"deviceID": device.Id, "flowToRemove": flow})
			//  dh.flowMgr.RemoveFlow(flow)
		}
	}
	return nil
}

//DisableDevice disables the given device
//It marks the following for the given device:
//Device-Handler Admin-State : down
//Device Port-State: UNKNOWN
//Device Oper-State: UNKNOWN
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
	if err := dh.coreProxy.PortsStateUpdate(context.TODO(), cloned.Id, voltha.OperStatus_UNKNOWN); err != nil {
		log.Errorw("updating-ports-failed", log.Fields{"deviceID": device.Id, "error": err})
		return err
	}

	//Update the device oper state
	cloned.OperStatus = voltha.OperStatus_UNKNOWN
	dh.device = cloned

	if err := dh.coreProxy.DeviceStateUpdate(context.TODO(), cloned.Id, cloned.ConnectStatus, cloned.OperStatus); err != nil {
		log.Errorw("error-updating-device-state", log.Fields{"deviceID": device.Id, "error": err})
		return err
	}
	log.Debugw("Disable_device-end", log.Fields{"deviceID": device.Id})
	return nil
}

//ReenableDevice re-enables the olt device after disable
//It marks the following for the given device:
//Device-Handler Admin-State : up
//Device Port-State: ACTIVE
//Device Oper-State: ACTIVE
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
	if err := dh.coreProxy.PortsStateUpdate(context.TODO(), cloned.Id, voltha.OperStatus_ACTIVE); err != nil {
		log.Errorw("updating-ports-failed", log.Fields{"deviceID": device.Id, "error": err})
		return err
	}

	//Update the device oper status as ACTIVE
	cloned.OperStatus = voltha.OperStatus_ACTIVE
	dh.device = cloned

	if err := dh.coreProxy.DeviceStateUpdate(context.TODO(), cloned.Id, cloned.ConnectStatus, cloned.OperStatus); err != nil {
		log.Errorw("error-updating-device-state", log.Fields{"deviceID": device.Id, "error": err})
		return err
	}
	log.Debugw("ReEnableDevice-end", log.Fields{"deviceID": device.Id})

	return nil
}

//RebootDevice reboots the given device
func (dh *DeviceHandler) RebootDevice(device *voltha.Device) error {
	if _, err := dh.Client.Reboot(context.Background(), new(oop.Empty)); err != nil {
		log.Errorw("Failed to reboot olt ", log.Fields{"err": err})
		return err
	}

	log.Debugw("rebooted-device-successfully", log.Fields{"deviceID": device.Id})

	return nil
}

func (dh *DeviceHandler) handlePacketIndication(packetIn *oop.PacketIndication) {
	log.Debugw("Received packet-in", log.Fields{"packet-indication": *packetIn})
	logicalPortNum, err := dh.flowMgr.GetLogicalPortFromPacketIn(packetIn)
	if err != nil {
		log.Errorw("Error getting logical port from packet-in", log.Fields{"error": err})
		return
	}
	log.Debugw("sending packet-in to core", log.Fields{"logicalPortNum": logicalPortNum, "packet": *packetIn})
	if err := dh.coreProxy.SendPacketIn(context.TODO(), dh.device.Id, logicalPortNum, packetIn.Pkt); err != nil {
		log.Errorw("Error sending packet-in to core", log.Fields{"error": err})
		return
	}
	log.Debug("Success sending packet-in to core!")
}

// PacketOut sends packet-out from VOLTHA to OLT on the egress port provided
func (dh *DeviceHandler) PacketOut(egressPortNo int, packet *of.OfpPacketOut) error {
	log.Debugw("PacketOut", log.Fields{"deviceID": dh.deviceID, "egress_port_no": egressPortNo, "pkt-length": len(packet.Data)})
	var etherFrame ethernet.Frame
	err := (&etherFrame).UnmarshalBinary(packet.Data)
	if err != nil {
		log.Errorw("Failed to unmarshal into ethernet frame", log.Fields{"err": err, "pkt-length": len(packet.Data)})
		return err
	}
	log.Debugw("Ethernet Frame", log.Fields{"Frame": etherFrame})
	egressPortType := IntfIDToPortTypeName(uint32(egressPortNo))
	if egressPortType == voltha.Port_ETHERNET_UNI {
		if etherFrame.VLAN != nil { // If double tag, remove the outer tag
			nextEthType := (uint16(packet.Data[16]) << 8) | uint16(packet.Data[17])
			if nextEthType == 0x8100 {
				etherFrame.VLAN = nil
				packet.Data, err = etherFrame.MarshalBinary()
				if err != nil {
					log.Fatalf("failed to marshal frame: %v", err)
					return err
				}
				if err := (&etherFrame).UnmarshalBinary(packet.Data); err != nil {
					log.Fatalf("failed to unmarshal frame: %v", err)
					return err
				}
				log.Debug("Double tagged packet , removed outer vlan", log.Fields{"New frame": etherFrame})
			}
		}
		intfID := IntfIDFromUniPortNum(uint32(egressPortNo))
		onuID := OnuIDFromPortNum(uint32(egressPortNo))
		uniID := UniIDFromPortNum(uint32(egressPortNo))
		/*gemPortId, err := dh.flowMgr.GetPacketOutGemPortId(intfID, onuID, uint32(egress_port_no))
		  if err != nil{
		      log.Errorw("Error while getting gemport to packet-out",log.Fields{"error": err})
		      return err
		  }*/
		onuPkt := oop.OnuPacket{IntfId: intfID, OnuId: onuID, PortNo: uint32(egressPortNo), Pkt: packet.Data}
		log.Debug("sending-packet-to-ONU", log.Fields{"egress_port_no": egressPortNo, "IntfId": intfID, "onuID": onuID,
			"uniID": uniID, "packet": packet.Data})
		if _, err := dh.Client.OnuPacketOut(context.Background(), &onuPkt); err != nil {
			log.Errorw("Error while sending packet-out to ONU", log.Fields{"error": err})
			return err
		}
	} else if egressPortType == voltha.Port_ETHERNET_NNI {
		uplinkPkt := oop.UplinkPacket{IntfId: IntfIDFromNniPortNum(uint32(egressPortNo)), Pkt: packet.Data}
		log.Debug("sending-packet-to-uplink", log.Fields{"uplink_pkt": uplinkPkt})
		if _, err := dh.Client.UplinkPacketOut(context.Background(), &uplinkPkt); err != nil {
			log.Errorw("Error while sending packet-out to uplink", log.Fields{"error": err})
			return err
		}
	} else {
		log.Warnw("Packet-out-to-this-interface-type-not-implemented", log.Fields{"egress_port_no": egressPortNo, "egressPortType": egressPortType})
	}
	return nil
}

func (dh *DeviceHandler) formOnuKey(intfID, onuID uint32) string {
	return "" + strconv.Itoa(int(intfID)) + "." + strconv.Itoa(int(onuID))
}
