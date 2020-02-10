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
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/cenkalti/backoff/v3"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/opencord/voltha-lib-go/v3/pkg/adapters/adapterif"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	"github.com/opencord/voltha-lib-go/v3/pkg/pmmetrics"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	"github.com/opencord/voltha-protos/v3/go/common"
	ic "github.com/opencord/voltha-protos/v3/go/inter_container"
	of "github.com/opencord/voltha-protos/v3/go/openflow_13"
	oop "github.com/opencord/voltha-protos/v3/go/openolt"
	"github.com/opencord/voltha-protos/v3/go/voltha"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Constants for number of retries and for timeout
const (
	MaxRetry       = 10
	MaxTimeOutInMs = 500
)

//DeviceHandler will interact with the OLT device.
type DeviceHandler struct {
	deviceID      string
	deviceType    string
	adminState    string
	device        *voltha.Device
	coreProxy     adapterif.CoreProxy
	AdapterProxy  adapterif.AdapterProxy
	EventProxy    adapterif.EventProxy
	openOLT       *OpenOLT
	exitChannel   chan int
	lockDevice    sync.RWMutex
	Client        oop.OpenoltClient
	transitionMap *TransitionMap
	clientCon     *grpc.ClientConn
	flowMgr       *OpenOltFlowMgr
	eventMgr      *OpenOltEventMgr
	resourceMgr   *rsrcMgr.OpenOltResourceMgr

	discOnus           sync.Map
	onus               sync.Map
	portStats          *OpenOltStatisticsMgr
	metrics            *pmmetrics.PmMetrics
	stopCollector      chan bool
	stopHeartbeatCheck chan bool
	activePorts        sync.Map
}

//OnuDevice represents ONU related info
type OnuDevice struct {
	deviceID      string
	deviceType    string
	serialNumber  string
	onuID         uint32
	intfID        uint32
	proxyDeviceID string
	uniPorts      map[uint32]struct{}
}

var pmNames = []string{
	"rx_bytes",
	"rx_packets",
	"rx_mcast_packets",
	"rx_bcast_packets",
	"tx_bytes",
	"tx_packets",
	"tx_mcast_packets",
	"tx_bcast_packets",
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
	device.uniPorts = make(map[uint32]struct{})
	return &device
}

//NewDeviceHandler creates a new device handler
func NewDeviceHandler(cp adapterif.CoreProxy, ap adapterif.AdapterProxy, ep adapterif.EventProxy, device *voltha.Device, adapter *OpenOLT) *DeviceHandler {
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
	dh.lockDevice = sync.RWMutex{}
	dh.stopCollector = make(chan bool, 2)
	dh.stopHeartbeatCheck = make(chan bool, 2)
	dh.metrics = pmmetrics.NewPmMetrics(cloned.Id, pmmetrics.Frequency(150), pmmetrics.FrequencyOverride(false), pmmetrics.Grouped(false), pmmetrics.Metrics(pmNames))
	dh.activePorts = sync.Map{}
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

func macifyIP(ip net.IP) string {
	if len(ip) > 0 {
		oct1 := strconv.FormatInt(int64(ip[12]), 16)
		oct2 := strconv.FormatInt(int64(ip[13]), 16)
		oct3 := strconv.FormatInt(int64(ip[14]), 16)
		oct4 := strconv.FormatInt(int64(ip[15]), 16)
		return fmt.Sprintf("00:00:%02v:%02v:%02v:%02v", oct1, oct2, oct3, oct4)
	}
	return ""
}

func generateMacFromHost(host string) (string, error) {
	var genmac string
	var addr net.IP
	var ips []string
	var err error

	log.Debugw("generating-mac-from-host", log.Fields{"host": host})

	if addr = net.ParseIP(host); addr == nil {
		log.Debugw("looking-up-hostname", log.Fields{"host": host})

		if ips, err = net.LookupHost(host); err == nil {
			log.Debugw("dns-result-ips", log.Fields{"ips": ips})
			if addr = net.ParseIP(ips[0]); addr == nil {
				log.Errorw("unable-to-parse-ip", log.Fields{"ip": ips[0]})
				return "", errors.New("unable-to-parse-ip")
			}
			genmac = macifyIP(addr)
			log.Debugw("using-ip-as-mac", log.Fields{"host": ips[0], "mac": genmac})
			return genmac, nil
		}
		log.Errorw("cannot-resolve-hostname-to-ip", log.Fields{"host": host})
		return "", err
	}

	genmac = macifyIP(addr)
	log.Debugw("using-ip-as-mac", log.Fields{"host": host, "mac": genmac})
	return genmac, nil
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
	var operStatus common.OperStatus_Types
	if state == "up" {
		operStatus = voltha.OperStatus_ACTIVE
		//populating the intfStatus map
		dh.activePorts.Store(intfID, true)
	} else {
		operStatus = voltha.OperStatus_DISCOVERED
		dh.activePorts.Store(intfID, false)
	}
	portNum := IntfIDToPortNo(intfID, portType)
	label := GetportLabel(portNum, portType)
	if len(label) == 0 {
		log.Errorw("Invalid-port-label", log.Fields{"portNum": portNum, "portType": portType})
		return
	}

	device, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		log.Errorw("Failed-to-fetch-device", log.Fields{"err": err})
		return
	}
	if device.Ports != nil {
		for _, dPort := range device.Ports {
			if dPort.Type == portType && dPort.PortNo == portNum {
				log.Debug("port-already-exists-updating-oper-status-of-port")
				if err := dh.coreProxy.PortStateUpdate(context.TODO(), dh.device.Id, portType, portNum, operStatus); err != nil {
					log.Errorw("failed-to-update-port-state", log.Fields{"err": err})
					return
				}
				return
			}
		}
	}
	//    Now create  Port
	port := &voltha.Port{
		PortNo:     portNum,
		Label:      label,
		Type:       portType,
		OperStatus: operStatus,
	}
	log.Debugw("Sending-port-update-to-core", log.Fields{"port": port})
	// Synchronous call to update device - this method is run in its own go routine
	if err := dh.coreProxy.PortCreated(context.TODO(), dh.device.Id, port); err != nil {
		log.Errorw("Error-creating-port", log.Fields{"deviceID": dh.device.Id, "portType": portType, "error": err})
		return
	}
	return
}

// readIndications to read the indications from the OLT device
func (dh *DeviceHandler) readIndications(ctx context.Context) {
	defer log.Errorw("Indications ended", log.Fields{})
	indications, err := dh.Client.EnableIndication(ctx, new(oop.Empty))
	if err != nil {
		log.Errorw("Failed to read indications", log.Fields{"err": err})
		return
	}
	if indications == nil {
		log.Errorw("Indications is nil", log.Fields{})
		return
	}
	/* get device state */
	device, err := dh.coreProxy.GetDevice(ctx, dh.device.Id, dh.device.Id)
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

	// Create an exponential backoff around re-enabling indications. The
	// maximum elapsed time for the back off is set to 0 so that we will
	// continue to retry. The max interval defaults to 1m, but is set
	// here for code clarity
	indicationBackoff := backoff.NewExponentialBackOff()
	indicationBackoff.MaxElapsedTime = 0
	indicationBackoff.MaxInterval = 1 * time.Minute
	for {
		indication, err := indications.Recv()
		if err == io.EOF {
			log.Infow("EOF for  indications", log.Fields{"err": err})
			// Use an exponential back off to prevent getting into a tight loop
			duration := indicationBackoff.NextBackOff()
			if duration == backoff.Stop {
				// If we reach a maximum then warn and reset the backoff
				// timer and keep attempting.
				log.Warnw("Maximum indication backoff reached, resetting backoff timer",
					log.Fields{"max_indication_backoff": indicationBackoff.MaxElapsedTime})
				indicationBackoff.Reset()
			}
			time.Sleep(indicationBackoff.NextBackOff())
			indications, err = dh.Client.EnableIndication(ctx, new(oop.Empty))
			if err != nil {
				log.Errorw("Failed to read indications", log.Fields{"err": err})
				return
			}
			continue
		}
		if err != nil {
			log.Infow("Failed to read from indications", log.Fields{"err": err})
			if dh.adminState == "deleted" {
				log.Debug("Device deleted stoping the read indication thread")
				break
			}
			dh.transitionMap.Handle(ctx, DeviceDownInd)
			dh.transitionMap.Handle(ctx, DeviceInit)
			break
		}
		// Reset backoff if we have a successful receive
		indicationBackoff.Reset()
		dh.lockDevice.RLock()
		adminState := dh.adminState
		dh.lockDevice.RUnlock()
		// When OLT is admin down, ignore all indications.
		if adminState == "down" {

			log.Infow("olt is admin down, ignore indication", log.Fields{"indication": indication})
			continue
		}
		dh.handleIndication(ctx, indication)

	}
}

func (dh *DeviceHandler) handleOltIndication(ctx context.Context, oltIndication *oop.OltIndication) {
	raisedTs := time.Now().UnixNano()
	if oltIndication.OperState == "up" && dh.transitionMap.currentDeviceState != deviceStateUp {
		dh.transitionMap.Handle(ctx, DeviceUpInd)
	} else if oltIndication.OperState == "down" {
		dh.transitionMap.Handle(ctx, DeviceDownInd)
	}
	// Send or clear Alarm
	dh.eventMgr.oltUpDownIndication(oltIndication, dh.deviceID, raisedTs)
}

func (dh *DeviceHandler) handleIndication(ctx context.Context, indication *oop.Indication) {
	raisedTs := time.Now().UnixNano()
	switch indication.Data.(type) {
	case *oop.Indication_OltInd:
		dh.handleOltIndication(ctx, indication.GetOltInd())
	case *oop.Indication_IntfInd:
		intfInd := indication.GetIntfInd()
		go dh.addPort(intfInd.GetIntfId(), voltha.Port_PON_OLT, intfInd.GetOperState())
		log.Infow("Received interface indication ", log.Fields{"InterfaceInd": intfInd})
	case *oop.Indication_IntfOperInd:
		intfOperInd := indication.GetIntfOperInd()
		if intfOperInd.GetType() == "nni" {
			go dh.addPort(intfOperInd.GetIntfId(), voltha.Port_ETHERNET_NNI, intfOperInd.GetOperState())
			dh.resourceMgr.AddNNIToKVStore(ctx, intfOperInd.GetIntfId())
		} else if intfOperInd.GetType() == "pon" {
			// TODO: Check what needs to be handled here for When PON PORT down, ONU will be down
			// Handle pon port update
			go dh.addPort(intfOperInd.GetIntfId(), voltha.Port_PON_OLT, intfOperInd.GetOperState())
			go dh.eventMgr.oltIntfOperIndication(indication.GetIntfOperInd(), dh.deviceID, raisedTs)
		}
		log.Infow("Received interface oper indication ", log.Fields{"InterfaceOperInd": intfOperInd})
	case *oop.Indication_OnuDiscInd:
		onuDiscInd := indication.GetOnuDiscInd()
		log.Infow("Received Onu discovery indication ", log.Fields{"OnuDiscInd": onuDiscInd})
		sn := dh.stringifySerialNumber(onuDiscInd.SerialNumber)
		go dh.onuDiscIndication(ctx, onuDiscInd, sn)
	case *oop.Indication_OnuInd:
		onuInd := indication.GetOnuInd()
		log.Infow("Received Onu indication ", log.Fields{"OnuInd": onuInd})
		go dh.onuIndication(onuInd)
	case *oop.Indication_OmciInd:
		omciInd := indication.GetOmciInd()
		log.Debugw("Received Omci indication ", log.Fields{"IntfId": omciInd.IntfId, "OnuId": omciInd.OnuId, "pkt": hex.EncodeToString(omciInd.Pkt)})
		go dh.omciIndication(omciInd)
	case *oop.Indication_PktInd:
		pktInd := indication.GetPktInd()
		log.Infow("Received pakcet indication ", log.Fields{"PktInd": pktInd})
		go dh.handlePacketIndication(ctx, pktInd)
	case *oop.Indication_PortStats:
		portStats := indication.GetPortStats()
		go dh.portStats.PortStatisticsIndication(portStats, dh.resourceMgr.DevInfo.GetPonPorts())
	case *oop.Indication_FlowStats:
		flowStats := indication.GetFlowStats()
		log.Infow("Received flow stats", log.Fields{"FlowStats": flowStats})
	case *oop.Indication_AlarmInd:
		alarmInd := indication.GetAlarmInd()
		log.Infow("Received alarm indication ", log.Fields{"AlarmInd": alarmInd})
		go dh.eventMgr.ProcessEvents(alarmInd, dh.deviceID, raisedTs)
	}
}

// doStateUp handle the olt up indication and update to voltha core
func (dh *DeviceHandler) doStateUp(ctx context.Context) error {
	// Synchronous call to update device state - this method is run in its own go routine
	if err := dh.coreProxy.DeviceStateUpdate(ctx, dh.device.Id, voltha.ConnectStatus_REACHABLE,
		voltha.OperStatus_ACTIVE); err != nil {
		log.Errorw("Failed to update device with OLT UP indication", log.Fields{"deviceID": dh.device.Id, "error": err})
		return err
	}
	return nil
}

// doStateDown handle the olt down indication
func (dh *DeviceHandler) doStateDown(ctx context.Context) error {
	dh.lockDevice.Lock()
	defer dh.lockDevice.Unlock()
	log.Debug("do-state-down-start")

	device, err := dh.coreProxy.GetDevice(ctx, dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		log.Errorw("Failed to fetch device device", log.Fields{"err": err})
		return errors.New("failed to fetch device device")
	}

	cloned := proto.Clone(device).(*voltha.Device)
	// Update the all ports state on that device to disable
	if er := dh.coreProxy.PortsStateUpdate(ctx, cloned.Id, voltha.OperStatus_UNKNOWN); er != nil {
		log.Errorw("updating-ports-failed", log.Fields{"deviceID": device.Id, "error": er})
		return er
	}

	//Update the device oper state and connection status
	cloned.OperStatus = voltha.OperStatus_UNKNOWN
	cloned.ConnectStatus = common.ConnectStatus_UNREACHABLE
	dh.device = cloned

	if er := dh.coreProxy.DeviceStateUpdate(ctx, cloned.Id, cloned.ConnectStatus, cloned.OperStatus); er != nil {
		log.Errorw("error-updating-device-state", log.Fields{"deviceID": device.Id, "error": er})
		return er
	}

	//get the child device for the parent device
	onuDevices, err := dh.coreProxy.GetChildDevices(ctx, dh.device.Id)
	if err != nil {
		log.Errorw("failed to get child devices information", log.Fields{"deviceID": dh.device.Id, "error": err})
		return err
	}
	for _, onuDevice := range onuDevices.Items {

		// Update onu state as down in onu adapter
		onuInd := oop.OnuIndication{}
		onuInd.OperState = "down"
		er := dh.AdapterProxy.SendInterAdapterMessage(ctx, &onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
			"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		if er != nil {
			log.Errorw("Failed to send inter-adapter-message", log.Fields{"OnuInd": onuInd,
				"From Adapter": "openolt", "DevieType": onuDevice.Type, "DeviceID": onuDevice.Id})
			//Do not return here and continue to process other ONUs
		}
	}
	/* Discovered ONUs entries need to be cleared , since after OLT
	   is up, it starts sending discovery indications again*/
	dh.discOnus = sync.Map{}
	log.Debugw("do-state-down-end", log.Fields{"deviceID": device.Id})
	return nil
}

// doStateInit dial the grpc before going to init state
func (dh *DeviceHandler) doStateInit(ctx context.Context) error {
	var err error
	dh.clientCon, err = grpc.Dial(dh.device.GetHostAndPort(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Errorw("Failed to dial device", log.Fields{"DeviceId": dh.deviceID, "HostAndPort": dh.device.GetHostAndPort(), "err": err})
		return err
	}
	return nil
}

// postInit create olt client instance to invoke RPC on the olt device
func (dh *DeviceHandler) postInit(ctx context.Context) error {
	dh.Client = oop.NewOpenoltClient(dh.clientCon)
	dh.transitionMap.Handle(ctx, GrpcConnected)
	return nil
}

// doStateConnected get the device info and update to voltha core
func (dh *DeviceHandler) doStateConnected(ctx context.Context) error {
	log.Debug("OLT device has been connected")

	// Case where OLT is disabled and then rebooted.
	if dh.adminState == "down" {
		log.Debugln("do-state-connected--device-admin-state-down")
		device, err := dh.coreProxy.GetDevice(ctx, dh.device.Id, dh.device.Id)
		if err != nil || device == nil {
			/*TODO: needs to handle error scenarios */
			log.Errorw("Failed to fetch device device", log.Fields{"err": err})
		}

		cloned := proto.Clone(device).(*voltha.Device)
		cloned.ConnectStatus = voltha.ConnectStatus_REACHABLE
		cloned.OperStatus = voltha.OperStatus_UNKNOWN
		dh.device = cloned
		if er := dh.coreProxy.DeviceStateUpdate(ctx, cloned.Id, cloned.ConnectStatus, cloned.OperStatus); er != nil {
			log.Errorw("error-updating-device-state", log.Fields{"deviceID": dh.device.Id, "error": er})
		}

		// Since the device was disabled before the OLT was rebooted, enforce the OLT to be Disabled after re-connection.
		_, err = dh.Client.DisableOlt(ctx, new(oop.Empty))
		if err != nil {
			log.Errorw("Failed to disable olt ", log.Fields{"err": err})
		}

		// Start reading indications
		go dh.readIndications(ctx)
		return nil
	}

	deviceInfo, err := dh.populateDeviceInfo()
	if err != nil {
		log.Errorw("Unable to populate Device Info", log.Fields{"err": err})
		return err
	}

	device, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		log.Errorw("Failed to fetch device device", log.Fields{"err": err})
		return err
	}
	dh.populateActivePorts(device)
	if err := dh.updatePortAdminState(device); err != nil {
		log.Errorw("Error-on-updating-port-status", log.Fields{"device": device})
		return err
	}

	KVStoreHostPort := fmt.Sprintf("%s:%d", dh.openOLT.KVStoreHost, dh.openOLT.KVStorePort)
	// Instantiate resource manager
	if dh.resourceMgr = rsrcMgr.NewResourceMgr(ctx, dh.deviceID, KVStoreHostPort, dh.openOLT.KVStoreType, dh.deviceType, deviceInfo); dh.resourceMgr == nil {
		log.Error("Error while instantiating resource manager")
		return errors.New("instantiating resource manager failed")
	}
	// Instantiate flow manager
	if dh.flowMgr = NewFlowManager(ctx, dh, dh.resourceMgr); dh.flowMgr == nil {
		log.Error("Error while instantiating flow manager")
		return errors.New("instantiating flow manager failed")
	}
	/* TODO: Instantiate Alarm , stats , BW managers */
	/* Instantiating Event Manager to handle Alarms and KPIs */
	dh.eventMgr = NewEventMgr(dh.EventProxy, dh)
	// Stats config for new device
	dh.portStats = NewOpenOltStatsMgr(dh)

	// Start reading indications
	go dh.readIndications(ctx)
	return nil
}

func (dh *DeviceHandler) populateDeviceInfo() (*oop.DeviceInfo, error) {
	var err error
	var deviceInfo *oop.DeviceInfo

	deviceInfo, err = dh.Client.GetDeviceInfo(context.Background(), new(oop.Empty))

	if err != nil {
		log.Errorw("Failed to fetch device info", log.Fields{"err": err})
		return nil, err
	}
	if deviceInfo == nil {
		log.Errorw("Device info is nil", log.Fields{})
		return nil, errors.New("failed to get device info from OLT")
	}

	log.Debugw("Fetched device info", log.Fields{"deviceInfo": deviceInfo})
	dh.device.Root = true
	dh.device.Vendor = deviceInfo.Vendor
	dh.device.Model = deviceInfo.Model
	dh.device.SerialNumber = deviceInfo.DeviceSerialNumber
	dh.device.HardwareVersion = deviceInfo.HardwareVersion
	dh.device.FirmwareVersion = deviceInfo.FirmwareVersion

	if deviceInfo.DeviceId == "" {
		log.Warnw("no-device-id-provided-using-host", log.Fields{"hostport": dh.device.GetHostAndPort()})
		host := strings.Split(dh.device.GetHostAndPort(), ":")[0]
		genmac, err := generateMacFromHost(host)
		if err != nil {
			return nil, err
		}
		log.Debugw("using-host-for-mac-address", log.Fields{"host": host, "mac": genmac})
		dh.device.MacAddress = genmac
	} else {
		dh.device.MacAddress = deviceInfo.DeviceId
	}

	// Synchronous call to update device - this method is run in its own go routine
	if err := dh.coreProxy.DeviceUpdate(context.TODO(), dh.device); err != nil {
		log.Errorw("error-updating-device", log.Fields{"deviceID": dh.device.Id, "error": err})
		return nil, err
	}

	return deviceInfo, nil
}

func startCollector(dh *DeviceHandler) {
	// Initial delay for OLT initialization
	time.Sleep(1 * time.Minute)
	log.Debugf("Starting-Collector")
	context := make(map[string]string)
	for {
		select {
		case <-dh.stopCollector:
			log.Debugw("Stopping-Collector-for-OLT", log.Fields{"deviceID:": dh.deviceID})
			return
		default:
			freq := dh.metrics.ToPmConfigs().DefaultFreq
			time.Sleep(time.Duration(freq) * time.Second)
			context["oltid"] = dh.deviceID
			context["devicetype"] = dh.deviceType
			// NNI Stats
			cmnni := dh.portStats.collectNNIMetrics(uint32(0))
			log.Debugf("Collect-NNI-Metrics %v", cmnni)
			go dh.portStats.publishMetrics("NNIStats", cmnni, uint32(0), context, dh.deviceID)
			log.Debugf("Publish-NNI-Metrics")
			// PON Stats
			NumPonPORTS := dh.resourceMgr.DevInfo.GetPonPorts()
			for i := uint32(0); i < NumPonPORTS; i++ {
				if val, ok := dh.activePorts.Load(i); ok && val == true {
					cmpon := dh.portStats.collectPONMetrics(i)
					log.Debugf("Collect-PON-Metrics %v", cmpon)

					go dh.portStats.publishMetrics("PONStats", cmpon, i, context, dh.deviceID)
					log.Debugf("Publish-PON-Metrics")
				}
			}
		}
	}
}

//AdoptDevice adopts the OLT device
func (dh *DeviceHandler) AdoptDevice(ctx context.Context, device *voltha.Device) {
	dh.transitionMap = NewTransitionMap(dh)
	log.Infow("Adopt_device", log.Fields{"deviceID": device.Id, "Address": device.GetHostAndPort()})
	dh.transitionMap.Handle(ctx, DeviceInit)

	// Now, set the initial PM configuration for that device
	if err := dh.coreProxy.DevicePMConfigUpdate(nil, dh.metrics.ToPmConfigs()); err != nil {
		log.Errorw("error-updating-PMs", log.Fields{"deviceId": device.Id, "error": err})
	}

	go startCollector(dh)
	go startHeartbeatCheck(dh)
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

func (dh *DeviceHandler) omciIndication(omciInd *oop.OmciIndication) {
	log.Debugw("omci indication", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
	var deviceType string
	var deviceID string
	var proxyDeviceID string

	onuKey := dh.formOnuKey(omciInd.IntfId, omciInd.OnuId)

	if onuInCache, ok := dh.onus.Load(onuKey); !ok {

		log.Debugw("omci indication for a device not in cache.", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
		ponPort := IntfIDToPortNo(omciInd.GetIntfId(), voltha.Port_PON_OLT)
		kwargs := make(map[string]interface{})
		kwargs["onu_id"] = omciInd.OnuId
		kwargs["parent_port_no"] = ponPort

		onuDevice, err := dh.coreProxy.GetChildDevice(context.TODO(), dh.device.Id, kwargs)
		if err != nil {
			log.Errorw("onu not found", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId, "error": err})
			return
		}
		deviceType = onuDevice.Type
		deviceID = onuDevice.Id
		proxyDeviceID = onuDevice.ProxyAddress.DeviceId
		//if not exist in cache, then add to cache.
		dh.onus.Store(onuKey, NewOnuDevice(deviceID, deviceType, onuDevice.SerialNumber, omciInd.OnuId, omciInd.IntfId, proxyDeviceID))
	} else {
		//found in cache
		log.Debugw("omci indication for a device in cache.", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
		deviceType = onuInCache.(*OnuDevice).deviceType
		deviceID = onuInCache.(*OnuDevice).deviceID
		proxyDeviceID = onuInCache.(*OnuDevice).proxyDeviceID
	}

	omciMsg := &ic.InterAdapterOmciMessage{Message: omciInd.Pkt}
	if sendErr := dh.AdapterProxy.SendInterAdapterMessage(context.Background(), omciMsg,
		ic.InterAdapterMessageType_OMCI_REQUEST, dh.deviceType, deviceType,
		deviceID, proxyDeviceID, ""); sendErr != nil {
		log.Errorw("send omci request error", log.Fields{"fromAdapter": dh.deviceType, "toAdapter": deviceType, "onuID": deviceID, "proxyDeviceID": proxyDeviceID, "error": sendErr})
		return
	}
	return
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
	var connectStatus common.ConnectStatus_Types
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

	// TODO: Once we are sure openonu/openomci is sending only binary in omciMsg.Message, we can remove this check
	isHexString := false
	_, decodeerr := hex.DecodeString(string(omciMsg.Message))
	if decodeerr == nil {
		isHexString = true
	}

	// TODO: OpenOLT Agent expects a hex string for OMCI packets rather than binary.  Fix this in the agent and then we can pass binary Pkt: omciMsg.Message.
	var omciMessage *oop.OmciMsg
	if isHexString {
		omciMessage = &oop.OmciMsg{IntfId: intfID, OnuId: onuID, Pkt: omciMsg.Message}
	} else {
		hexPkt := make([]byte, hex.EncodedLen(len(omciMsg.Message)))
		hex.Encode(hexPkt, omciMsg.Message)
		omciMessage = &oop.OmciMsg{IntfId: intfID, OnuId: onuID, Pkt: hexPkt}
	}

	_, err := dh.Client.OmciMsgOut(context.Background(), omciMessage)
	if err != nil {
		log.Errorw("unable to send omci-msg-out", log.Fields{"IntfID": intfID, "OnuID": onuID, "Msg": omciMessage})
		return
	}
	log.Debugw("Sent Omci message", log.Fields{"intfID": intfID, "onuID": onuID, "omciMsg": hex.EncodeToString(omciMsg.Message)})
}

func (dh *DeviceHandler) activateONU(ctx context.Context, intfID uint32, onuID int64, serialNum *oop.SerialNumber, serialNumber string) {
	log.Debugw("activate-onu", log.Fields{"intfID": intfID, "onuID": onuID, "serialNum": serialNum, "serialNumber": serialNumber})
	dh.flowMgr.UpdateOnuInfo(ctx, intfID, uint32(onuID), serialNumber)
	// TODO: need resource manager
	var pir uint32 = 1000000
	Onu := oop.Onu{IntfId: intfID, OnuId: uint32(onuID), SerialNumber: serialNum, Pir: pir}
	if _, err := dh.Client.ActivateOnu(ctx, &Onu); err != nil {
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

func (dh *DeviceHandler) onuDiscIndication(ctx context.Context, onuDiscInd *oop.OnuDiscIndication, sn string) {

	channelID := onuDiscInd.GetIntfId()
	parentPortNo := IntfIDToPortNo(onuDiscInd.GetIntfId(), voltha.Port_PON_OLT)

	log.Infow("new-discovery-indication", log.Fields{"sn": sn})

	if _, ok := dh.discOnus.Load(sn); ok {
		log.Warnw("onu-sn-is-already-being-processed", log.Fields{"sn": sn})
		return
	}

	kwargs := make(map[string]interface{})
	if sn != "" {
		kwargs["serial_number"] = sn
	} else {
		log.Errorw("invalid-onu-serial-number", log.Fields{"sn": sn})
		return
	}

	dh.discOnus.Store(sn, true)
	var onuID uint32

	// check the ONU is already know to the OLT
	// NOTE the second time the ONU is discovered this should return a device
	onuDevice, err := dh.coreProxy.GetChildDevice(ctx, dh.device.Id, kwargs)

	if err != nil {
		log.Errorw("core-proxy-get-child-device-failed", log.Fields{"parentDevice": dh.device.Id, "err": err, "sn": sn})
		if e, ok := status.FromError(err); ok {
			log.Warnw("core-proxy-get-child-device-failed-with-code", log.Fields{"errCode": e.Code(), "sn": sn})
			switch e.Code() {
			case codes.Internal:
				// this probably means NOT FOUND, so just create a new device
				onuDevice = nil
			case codes.DeadlineExceeded:
				// if the call times out, cleanup and exit
				dh.discOnus.Delete(sn)
				return
			}
		}
	}

	if onuDevice == nil {
		// NOTE this should happen a single time, and only if GetChildDevice returns NotFound
		log.Infow("creating-new-onu", log.Fields{"sn": sn})
		// we need to create a new ChildDevice
		ponintfid := onuDiscInd.GetIntfId()
		dh.lockDevice.Lock()
		onuID, err = dh.resourceMgr.GetONUID(ctx, ponintfid)
		dh.lockDevice.Unlock()

		log.Infow("creating-new-onu-got-onu-id", log.Fields{"sn": sn, "onuId": onuID})

		if err != nil {
			// if we can't create an ID in resource manager,
			// cleanup and exit
			log.Warnw("resource-manage-get-onu-id-failed", log.Fields{"pon-intf-id": ponintfid, "err": err, "sn": sn})
			dh.discOnus.Delete(sn)
			return
		}

		if onuDevice, err = dh.coreProxy.ChildDeviceDetected(context.TODO(), dh.device.Id, int(parentPortNo),
			"", int(channelID), string(onuDiscInd.SerialNumber.GetVendorId()), sn, int64(onuID)); err != nil {
			log.Warnw("core-proxy-child-device-detected-failed", log.Fields{"pon-intf-id": ponintfid, "err": err, "sn": sn})
			dh.discOnus.Delete(sn)
			dh.resourceMgr.FreeonuID(ctx, ponintfid, []uint32{onuID}) // NOTE I'm not sure this method is actually cleaning up the right thing
			return
		}

		log.Infow("onu-child-device-added", log.Fields{"onuDevice": onuDevice, "sn": sn})
	}

	// we can now use the existing ONU Id
	onuID = onuDevice.ProxyAddress.OnuId

	//Insert the ONU into cache to use in OnuIndication.
	//TODO: Do we need to remove this from the cache on ONU change, or wait for overwritten on next discovery.
	log.Debugw("onu-discovery-indication-key-create", log.Fields{"onuID": onuID,
		"intfId": onuDiscInd.GetIntfId(), "sn": sn})
	onuKey := dh.formOnuKey(onuDiscInd.GetIntfId(), onuID)

	onuDev := NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuID, onuDiscInd.GetIntfId(), onuDevice.ProxyAddress.DeviceId)
	dh.onus.Store(onuKey, onuDev)
	log.Debugw("new-onu-device-discovered", log.Fields{"onu": onuDev, "sn": sn})

	err = dh.coreProxy.DeviceStateUpdate(ctx, onuDevice.Id, common.ConnectStatus_REACHABLE, common.OperStatus_DISCOVERED)
	if err != nil {
		log.Errorw("failed-to-update-device-state", log.Fields{"DeviceID": onuDevice.Id, "sn": sn, "err": err})
		return
	}
	log.Infow("onu-discovered-reachable", log.Fields{"deviceId": onuDevice.Id, "sn": sn})
	//TODO: We put this sleep here to prevent the race between state update and onuIndication
	//In onuIndication the operStatus of device is checked. If it is still not updated in KV store
	//then the initialisation fails.
	time.Sleep(1 * time.Second)
	dh.activateONU(ctx, onuDiscInd.IntfId, int64(onuID), onuDiscInd.SerialNumber, sn)
	return
}

func (dh *DeviceHandler) onuIndication(onuInd *oop.OnuIndication) {
	serialNumber := dh.stringifySerialNumber(onuInd.SerialNumber)

	kwargs := make(map[string]interface{})
	ponPort := IntfIDToPortNo(onuInd.GetIntfId(), voltha.Port_PON_OLT)
	var onuDevice *voltha.Device
	foundInCache := false
	log.Debugw("ONU indication key create", log.Fields{"onuId": onuInd.OnuId,
		"intfId": onuInd.GetIntfId()})
	onuKey := dh.formOnuKey(onuInd.GetIntfId(), onuInd.OnuId)

	if onuInCache, ok := dh.onus.Load(onuKey); ok {

		//If ONU id is discovered before then use GetDevice to get onuDevice because it is cheaper.
		foundInCache = true
		onuDevice, _ = dh.coreProxy.GetDevice(nil, dh.device.Id, onuInCache.(*OnuDevice).deviceID)
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

			dh.onus.Store(onuKey, NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuInd.GetOnuId(), onuInd.GetIntfId(), onuDevice.ProxyAddress.DeviceId))

		}
		dh.updateOnuStates(onuDevice, onuInd, foundInCache)

	} else {
		log.Errorw("onu not found", log.Fields{"intfID": onuInd.IntfId, "onuID": onuInd.OnuId})
		return
	}

}

func (dh *DeviceHandler) updateOnuStates(onuDevice *voltha.Device, onuInd *oop.OnuIndication, foundInCache bool) {
	ctx := context.TODO()
	log.Debugw("onu-indication-for-state", log.Fields{"onuIndication": onuInd, "DeviceId": onuDevice.Id, "operStatus": onuDevice.OperStatus, "adminStatus": onuDevice.AdminState})
	dh.updateOnuAdminState(onuInd)
	// operState
	if onuInd.OperState == "down" {
		log.Debugw("sending-interadapter-onu-indication", log.Fields{"onuIndication": onuInd, "DeviceId": onuDevice.Id, "operStatus": onuDevice.OperStatus, "adminStatus": onuDevice.AdminState})
		// TODO NEW CORE do not hardcode adapter name. Handler needs Adapter reference
		err := dh.AdapterProxy.SendInterAdapterMessage(ctx, onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
			"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		if err != nil {
			log.Errorw("Failed to send inter-adapter-message", log.Fields{"OnuInd": onuInd,
				"From Adapter": "openolt", "DevieType": onuDevice.Type, "DeviceID": onuDevice.Id})
		}
	} else if onuInd.OperState == "up" {
		// Ignore operstatus if device was found in cache
		if !foundInCache && onuDevice.OperStatus != common.OperStatus_DISCOVERED {
			log.Warnw("ignore-onu-indication", log.Fields{"intfID": onuInd.IntfId, "onuID": onuInd.OnuId, "operStatus": onuDevice.OperStatus, "msgOperStatus": onuInd.OperState})
			return
		}
		log.Debugw("sending-interadapter-onu-indication", log.Fields{"onuIndication": onuInd, "DeviceId": onuDevice.Id, "operStatus": onuDevice.OperStatus, "adminStatus": onuDevice.AdminState})
		// TODO NEW CORE do not hardcode adapter name. Handler needs Adapter reference
		err := dh.AdapterProxy.SendInterAdapterMessage(ctx, onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
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
	log.Debugw("send-packet-in-to-core", log.Fields{
		"port":   logicalPort,
		"packet": hex.EncodeToString(packetPayload),
	})
	if err := dh.coreProxy.SendPacketIn(context.TODO(), dh.device.Id, logicalPort, packetPayload); err != nil {
		log.Errorw("Error sending packetin to core", log.Fields{
			"error":  err,
			"packet": hex.EncodeToString(packetPayload),
		})
		return
	}
	log.Debugw("Sent packet-in to core successfully", log.Fields{
		"packet": hex.EncodeToString(packetPayload),
	})
}

// AddUniPortToOnu adds the uni port to the onu device
func (dh *DeviceHandler) AddUniPortToOnu(intfID, onuID, uniPort uint32) {
	onuKey := dh.formOnuKey(intfID, onuID)

	if onuDevice, ok := dh.onus.Load(onuKey); ok {
		// add it to the uniPort map for the onu device
		if _, ok = onuDevice.(*OnuDevice).uniPorts[uniPort]; !ok {
			onuDevice.(*OnuDevice).uniPorts[uniPort] = struct{}{}
			log.Debugw("adding-uni-port", log.Fields{"port": uniPort, "intfID": intfID, "onuId": onuID})
		}
	}
}

//UpdateFlowsIncrementally updates the device flow
func (dh *DeviceHandler) UpdateFlowsIncrementally(ctx context.Context, device *voltha.Device, flows *of.FlowChanges, groups *of.FlowGroupChanges, flowMetadata *voltha.FlowMetadata) error {
	log.Debugw("Received-incremental-flowupdate-in-device-handler", log.Fields{"deviceID": device.Id, "flows": flows, "groups": groups, "flowMetadata": flowMetadata})
	if flows != nil {
		for _, flow := range flows.ToRemove.Items {
			log.Debug("Removing flow", log.Fields{"deviceId": device.Id, "flowToRemove": flow})
			dh.flowMgr.RemoveFlow(ctx, flow)
		}

		for _, flow := range flows.ToAdd.Items {
			log.Debug("Adding flow", log.Fields{"deviceId": device.Id, "flowToAdd": flow})
			dh.flowMgr.AddFlow(ctx, flow, flowMetadata)
		}
	}
	if groups != nil && flows != nil {
		for _, flow := range flows.ToRemove.Items {
			log.Debug("Removing flow", log.Fields{"deviceID": device.Id, "flowToRemove": flow})
			//  dh.flowMgr.RemoveFlow(flow)
		}
	}

	if groups != nil {
		for _, group := range groups.ToAdd.Items {
			dh.flowMgr.AddGroup(ctx, group)
		}
		for _, group := range groups.ToUpdate.Items {
			dh.flowMgr.ModifyGroup(ctx, group)
		}
		if len(groups.ToRemove.Items) != 0 {
			log.Debug("Group delete operation is not supported for now")
		}
	}
	log.Debug("UpdateFlowsIncrementally done successfully")
	return nil
}

//DisableDevice disables the given device
//It marks the following for the given device:
//Device-Handler Admin-State : down
//Device Port-State: UNKNOWN
//Device Oper-State: UNKNOWN
func (dh *DeviceHandler) DisableDevice(device *voltha.Device) error {
	/* On device disable ,admin state update has to be done prior sending request to agent since
	   the indication thread may processes invalid  indications of ONU and OLT*/
	dh.lockDevice.Lock()
	dh.adminState = "down"
	dh.lockDevice.Unlock()
	if dh.Client != nil {
		if _, err := dh.Client.DisableOlt(context.Background(), new(oop.Empty)); err != nil {
			if e, ok := status.FromError(err); ok && e.Code() == codes.Internal {
				log.Errorw("failed-to-disable-olt ", log.Fields{"err": err, "deviceID": device.Id})
				dh.lockDevice.Lock()
				dh.adminState = "up"
				dh.lockDevice.Unlock()
				return err
			}
		}
	}
	log.Debugw("olt-disabled", log.Fields{"deviceID": device.Id})
	/* Discovered ONUs entries need to be cleared , since on device disable the child devices goes to
	UNREACHABLE state which needs to be configured again*/

	dh.discOnus = sync.Map{}
	dh.onus = sync.Map{}

	go dh.notifyChildDevices("unreachable")
	cloned := proto.Clone(device).(*voltha.Device)
	// Update the all ports state on that device to disable
	if err := dh.coreProxy.PortsStateUpdate(context.TODO(), cloned.Id, voltha.OperStatus_UNKNOWN); err != nil {
		log.Errorw("updating-ports-failed", log.Fields{"deviceID": device.Id, "error": err})
		return err
	}

	log.Debugw("disable-device-end", log.Fields{"deviceID": device.Id})
	return nil
}

func (dh *DeviceHandler) notifyChildDevices(state string) {

	// Update onu state as unreachable in onu adapter
	onuInd := oop.OnuIndication{}
	onuInd.OperState = state
	//get the child device for the parent device
	onuDevices, err := dh.coreProxy.GetChildDevices(context.TODO(), dh.device.Id)
	if err != nil {
		log.Errorw("failed-to-get-child-devices-information", log.Fields{"deviceID": dh.device.Id, "error": err})
	}
	if onuDevices != nil {
		for _, onuDevice := range onuDevices.Items {
			err := dh.AdapterProxy.SendInterAdapterMessage(context.TODO(), &onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
				"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
			if err != nil {
				log.Errorw("failed-to-send-inter-adapter-message", log.Fields{"OnuInd": onuInd,
					"From Adapter": "openolt", "DeviceType": onuDevice.Type, "DeviceID": onuDevice.Id})
			}

		}
	}

}

//ReenableDevice re-enables the olt device after disable
//It marks the following for the given device:
//Device-Handler Admin-State : up
//Device Port-State: ACTIVE
//Device Oper-State: ACTIVE
func (dh *DeviceHandler) ReenableDevice(device *voltha.Device) error {
	dh.lockDevice.Lock()
	dh.adminState = "up"
	dh.lockDevice.Unlock()

	if _, err := dh.Client.ReenableOlt(context.Background(), new(oop.Empty)); err != nil {
		if e, ok := status.FromError(err); ok && e.Code() == codes.Internal {
			log.Errorw("Failed to reenable olt ", log.Fields{"err": err})
			dh.lockDevice.Lock()
			dh.adminState = "down"
			dh.lockDevice.Unlock()
			return err
		}
	}
	log.Debug("olt-reenabled")

	cloned := proto.Clone(device).(*voltha.Device)
	// Update the all ports state on that device to enable

	if err := dh.updatePortAdminState(device); err != nil {
		log.Errorw("Error-on-updating-port-status-after-reenabling-olt", log.Fields{"device": device})
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

func (dh *DeviceHandler) clearUNIData(ctx context.Context, onu *rsrcMgr.OnuGemInfo) error {
	var uniID uint32
	var err error
	for _, port := range onu.UniPorts {
		uniID = UniIDFromPortNum(uint32(port))
		log.Debugw("clearing-resource-data-for-uni-port", log.Fields{"port": port, "uniID": uniID})
		/* Delete tech-profile instance from the KV store */
		if err = dh.flowMgr.DeleteTechProfileInstances(ctx, onu.IntfID, onu.OnuID, uniID, onu.SerialNumber); err != nil {
			log.Debugw("Failed-to-remove-tech-profile-instance-for-onu", log.Fields{"onu-id": onu.OnuID})
		}
		log.Debugw("Deleted-tech-profile-instance-for-onu", log.Fields{"onu-id": onu.OnuID})
		flowIDs := dh.resourceMgr.GetCurrentFlowIDsForOnu(ctx, onu.IntfID, int32(onu.OnuID), int32(uniID))
		for _, flowID := range flowIDs {
			dh.resourceMgr.FreeFlowID(ctx, onu.IntfID, int32(onu.OnuID), int32(uniID), flowID)
		}
		tpIDList := dh.resourceMgr.GetTechProfileIDForOnu(ctx, onu.IntfID, onu.OnuID, uniID)
		for _, tpID := range tpIDList {
			if err = dh.resourceMgr.RemoveMeterIDForOnu(ctx, "upstream", onu.IntfID, onu.OnuID, uniID, tpID); err != nil {
				log.Debugw("Failed-to-remove-meter-id-for-onu-upstream", log.Fields{"onu-id": onu.OnuID})
			}
			log.Debugw("Removed-meter-id-for-onu-upstream", log.Fields{"onu-id": onu.OnuID})
			if err = dh.resourceMgr.RemoveMeterIDForOnu(ctx, "downstream", onu.IntfID, onu.OnuID, uniID, tpID); err != nil {
				log.Debugw("Failed-to-remove-meter-id-for-onu-downstream", log.Fields{"onu-id": onu.OnuID})
			}
			log.Debugw("Removed-meter-id-for-onu-downstream", log.Fields{"onu-id": onu.OnuID})
		}
		dh.resourceMgr.FreePONResourcesForONU(ctx, onu.IntfID, onu.OnuID, uniID)
		if err = dh.resourceMgr.RemoveTechProfileIDsForOnu(ctx, onu.IntfID, onu.OnuID, uniID); err != nil {
			log.Debugw("Failed-to-remove-tech-profile-id-for-onu", log.Fields{"onu-id": onu.OnuID})
		}
		log.Debugw("Removed-tech-profile-id-for-onu", log.Fields{"onu-id": onu.OnuID})
		if err = dh.resourceMgr.DelGemPortPktIn(ctx, onu.IntfID, onu.OnuID, uint32(port)); err != nil {
			log.Debugw("Failed-to-remove-gemport-pkt-in", log.Fields{"intfid": onu.IntfID, "onuid": onu.OnuID, "uniId": uniID})
		}
	}
	return nil
}

func (dh *DeviceHandler) clearNNIData(ctx context.Context) error {
	nniUniID := -1
	nniOnuID := -1

	if dh.resourceMgr == nil {
		return fmt.Errorf("no resource manager for deviceID %s", dh.deviceID)
	}
	//Free the flow-ids for the NNI port
	nni, err := dh.resourceMgr.GetNNIFromKVStore(ctx)
	if err != nil {
		log.Error("Failed to fetch nni from kv store")
		return err
	}
	log.Debugw("NNI are ", log.Fields{"nni": nni})
	for _, nniIntfID := range nni {
		flowIDs := dh.resourceMgr.GetCurrentFlowIDsForOnu(ctx, uint32(nniIntfID), int32(nniOnuID), int32(nniUniID))
		log.Debugw("Current flow ids for nni", log.Fields{"flow-ids": flowIDs})
		for _, flowID := range flowIDs {
			dh.resourceMgr.FreeFlowID(ctx, uint32(nniIntfID), -1, -1, uint32(flowID))
		}
		dh.resourceMgr.RemoveResourceMap(ctx, nniIntfID, int32(nniOnuID), int32(nniUniID))
	}
	if err = dh.resourceMgr.DelNNiFromKVStore(ctx); err != nil {
		log.Error("Failed to clear nni from kv store")
		return err
	}
	return err
}

// DeleteDevice deletes the device instance from openolt handler array.  Also clears allocated resource manager resources.  Also reboots the OLT hardware!
func (dh *DeviceHandler) DeleteDevice(ctx context.Context, device *voltha.Device) error {
	log.Debug("Function entry delete device")
	dh.lockDevice.Lock()
	if dh.adminState == "deleted" {
		dh.lockDevice.Unlock()
		return nil
	}
	dh.adminState = "deleted"
	dh.lockDevice.Unlock()
	/* Clear the KV store data associated with the all the UNI ports
	   This clears up flow data and also resource map data for various
	   other pon resources like alloc_id and gemport_id
	*/
	if dh.resourceMgr != nil {
		noOfPonPorts := dh.resourceMgr.DevInfo.GetPonPorts()
		var ponPort uint32
		for ponPort = 0; ponPort < noOfPonPorts; ponPort++ {
			var onuGemData []rsrcMgr.OnuGemInfo
			err := dh.resourceMgr.ResourceMgrs[ponPort].GetOnuGemInfo(ctx, ponPort, &onuGemData)
			if err != nil {
				log.Errorw("Failed to get onu info for port ", log.Fields{"ponport": ponPort})
				return err
			}
			for _, onu := range onuGemData {
				onuID := make([]uint32, 1)
				log.Debugw("onu data ", log.Fields{"onu": onu})
				if err = dh.clearUNIData(ctx, &onu); err != nil {
					log.Errorw("Failed to clear data for onu", log.Fields{"onu-device": onu})
				}
				// Clear flowids for gem cache.
				for _, gem := range onu.GemPorts {
					dh.resourceMgr.DeleteFlowIDsForGem(ctx, ponPort, gem)
				}
				onuID[0] = onu.OnuID
				dh.resourceMgr.FreeonuID(ctx, ponPort, onuID)
			}
			dh.resourceMgr.DeleteIntfIDGempMapPath(ctx, ponPort)
			onuGemData = nil
			err = dh.resourceMgr.DelOnuGemInfoForIntf(ctx, ponPort)
			if err != nil {
				log.Errorw("Failed to update onugem info", log.Fields{"intfid": ponPort, "onugeminfo": onuGemData})
			}
		}
		/* Clear the flows from KV store associated with NNI port.
		   There are mostly trap rules from NNI port (like LLDP)
		*/
		if err := dh.clearNNIData(ctx); err != nil {
			log.Errorw("Failed to clear data for NNI port", log.Fields{"device-id": dh.deviceID})
		}

		/* Clear the resource pool for each PON port in the background */
		go dh.resourceMgr.Delete(ctx)
	}

	/*Delete ONU map for the device*/
	dh.onus.Range(func(key interface{}, value interface{}) bool {
		dh.onus.Delete(key)
		return true
	})

	log.Debug("Removed-device-from-Resource-manager-KV-store")
	// Stop the Stats collector
	dh.stopCollector <- true
	// stop the heartbeat check routine
	dh.stopHeartbeatCheck <- true
	//Reset the state
	if dh.Client != nil {
		if _, err := dh.Client.Reboot(ctx, new(oop.Empty)); err != nil {
			log.Errorw("Failed-to-reboot-olt ", log.Fields{"deviceID": dh.deviceID, "err": err})
			return err
		}
	}
	cloned := proto.Clone(device).(*voltha.Device)
	cloned.OperStatus = voltha.OperStatus_UNKNOWN
	cloned.ConnectStatus = voltha.ConnectStatus_UNREACHABLE
	if err := dh.coreProxy.DeviceStateUpdate(ctx, cloned.Id, cloned.ConnectStatus, cloned.OperStatus); err != nil {
		log.Errorw("error-updating-device-state", log.Fields{"deviceID": device.Id, "error": err})
		return err
	}
	return nil
}

//RebootDevice reboots the given device
func (dh *DeviceHandler) RebootDevice(device *voltha.Device) error {
	if _, err := dh.Client.Reboot(context.Background(), new(oop.Empty)); err != nil {
		log.Errorw("Failed to reboot olt ", log.Fields{"deviceID": dh.deviceID, "err": err})
		return err
	}

	log.Debugw("rebooted-device-successfully", log.Fields{"deviceID": device.Id})

	return nil
}

func (dh *DeviceHandler) handlePacketIndication(ctx context.Context, packetIn *oop.PacketIndication) {
	log.Debugw("Received packet-in", log.Fields{
		"packet-indication": *packetIn,
		"packet":            hex.EncodeToString(packetIn.Pkt),
	})
	logicalPortNum, err := dh.flowMgr.GetLogicalPortFromPacketIn(ctx, packetIn)
	if err != nil {
		log.Errorw("Error getting logical port from packet-in", log.Fields{
			"error":  err,
			"packet": hex.EncodeToString(packetIn.Pkt),
		})
		return
	}
	log.Debugw("sending packet-in to core", log.Fields{
		"logicalPortNum": logicalPortNum,
		"packet":         hex.EncodeToString(packetIn.Pkt),
	})
	if err := dh.coreProxy.SendPacketIn(context.TODO(), dh.device.Id, logicalPortNum, packetIn.Pkt); err != nil {
		log.Errorw("Error sending packet-in to core", log.Fields{
			"error":  err,
			"packet": hex.EncodeToString(packetIn.Pkt),
		})
		return
	}
	log.Debugw("Success sending packet-in to core!", log.Fields{
		"packet": hex.EncodeToString(packetIn.Pkt),
	})
}

// PacketOut sends packet-out from VOLTHA to OLT on the egress port provided
func (dh *DeviceHandler) PacketOut(ctx context.Context, egressPortNo int, packet *of.OfpPacketOut) error {
	log.Debugw("incoming-packet-out", log.Fields{
		"deviceID":       dh.deviceID,
		"egress_port_no": egressPortNo,
		"pkt-length":     len(packet.Data),
		"packet":         hex.EncodeToString(packet.Data),
	})

	egressPortType := IntfIDToPortTypeName(uint32(egressPortNo))
	if egressPortType == voltha.Port_ETHERNET_UNI {
		outerEthType := (uint16(packet.Data[12]) << 8) | uint16(packet.Data[13])
		innerEthType := (uint16(packet.Data[16]) << 8) | uint16(packet.Data[17])
		if outerEthType == 0x8942 || outerEthType == 0x88cc {
			// Do not packet-out lldp packets on uni port.
			// ONOS has no clue about uni/nni ports, it just packets out on all
			// available ports on the Logical Switch. It should not be interested
			// in the UNI links.
			log.Debug("dropping-lldp-packet-out-on-uni")
			return nil
		}
		if outerEthType == 0x88a8 || outerEthType == 0x8100 {
			if innerEthType == 0x8100 {
				// q-in-q 802.1ad or 802.1q double tagged packet.
				// slice out the outer tag.
				packet.Data = append(packet.Data[:12], packet.Data[16:]...)
				log.Debugw("packet-now-single-tagged", log.Fields{"packetData": hex.EncodeToString(packet.Data)})
			}
		}
		intfID := IntfIDFromUniPortNum(uint32(egressPortNo))
		onuID := OnuIDFromPortNum(uint32(egressPortNo))
		uniID := UniIDFromPortNum(uint32(egressPortNo))

		gemPortID, err := dh.flowMgr.GetPacketOutGemPortID(ctx, intfID, onuID, uint32(egressPortNo))
		if err != nil {
			// In this case the openolt agent will receive the gemPortID as 0.
			// The agent tries to retrieve the gemPortID in this case.
			// This may not always succeed at the agent and packetOut may fail.
			log.Errorw("failed-to-retrieve-gemport-id-for-packet-out", log.Fields{
				"packet": hex.EncodeToString(packet.Data),
			})
		}

		onuPkt := oop.OnuPacket{IntfId: intfID, OnuId: onuID, PortNo: uint32(egressPortNo), GemportId: gemPortID, Pkt: packet.Data}

		log.Debugw("sending-packet-to-onu", log.Fields{
			"egress_port_no": egressPortNo,
			"IntfId":         intfID,
			"onuID":          onuID,
			"uniID":          uniID,
			"gemPortID":      gemPortID,
			"packet":         hex.EncodeToString(packet.Data),
		})

		if _, err := dh.Client.OnuPacketOut(ctx, &onuPkt); err != nil {
			log.Errorw("Error while sending packet-out to ONU", log.Fields{
				"error":  err,
				"packet": hex.EncodeToString(packet.Data),
			})
			return err
		}
	} else if egressPortType == voltha.Port_ETHERNET_NNI {
		uplinkPkt := oop.UplinkPacket{IntfId: IntfIDFromNniPortNum(uint32(egressPortNo)), Pkt: packet.Data}

		log.Debugw("sending-packet-to-nni", log.Fields{
			"uplink_pkt": uplinkPkt,
			"packet":     hex.EncodeToString(packet.Data),
		})

		if _, err := dh.Client.UplinkPacketOut(ctx, &uplinkPkt); err != nil {
			log.Errorw("Error while sending packet-out to NNI", log.Fields{
				"error":  err,
				"packet": hex.EncodeToString(packet.Data),
			})
			return err
		}
	} else {
		log.Warnw("Packet-out-to-this-interface-type-not-implemented", log.Fields{
			"egress_port_no": egressPortNo,
			"egressPortType": egressPortType,
			"packet":         hex.EncodeToString(packet.Data),
		})
	}
	return nil
}

func (dh *DeviceHandler) formOnuKey(intfID, onuID uint32) string {
	return "" + strconv.Itoa(int(intfID)) + "." + strconv.Itoa(int(onuID))
}

func startHeartbeatCheck(dh *DeviceHandler) {
	// start the heartbeat check towards the OLT.
	var timerCheck *time.Timer

	for {
		heartbeatTimer := time.NewTimer(dh.openOLT.HeartbeatCheckInterval)
		select {
		case <-heartbeatTimer.C:
			ctx, cancel := context.WithTimeout(context.Background(), dh.openOLT.GrpcTimeoutInterval)
			if heartBeat, err := dh.Client.HeartbeatCheck(ctx, new(oop.Empty)); err != nil {
				log.Error("Hearbeat failed")
				if timerCheck == nil {
					// start a after func, when expired will update the state to the core
					timerCheck = time.AfterFunc(dh.openOLT.HeartbeatFailReportInterval, dh.updateStateUnreachable)
				}
			} else {
				if timerCheck != nil {
					if timerCheck.Stop() {
						log.Debug("We got hearbeat within the timeout")
					} else {

						log.Debug("We got hearbeat after the timeout expired, changing the states")
						go dh.notifyChildDevices("up")
						if err := dh.coreProxy.DeviceStateUpdate(ctx, dh.device.Id, voltha.ConnectStatus_REACHABLE,
							voltha.OperStatus_ACTIVE); err != nil {
							log.Errorw("Failed to update device state", log.Fields{"deviceID": dh.device.Id, "error": err})
						}
					}
					timerCheck = nil
				}
				log.Debugw("Hearbeat", log.Fields{"signature": heartBeat})
			}
			cancel()
		case <-dh.stopHeartbeatCheck:
			log.Debug("Stopping heart beat check")
			return
		}
	}
}

func (dh *DeviceHandler) updateStateUnreachable() {

	go dh.notifyChildDevices("unreachable")
	if err := dh.coreProxy.DeviceStateUpdate(context.TODO(), dh.device.Id, voltha.ConnectStatus_UNREACHABLE, voltha.OperStatus_UNKNOWN); err != nil {
		log.Errorw("error-updating-device-state", log.Fields{"deviceID": dh.device.Id, "error": err})
		return
	}
}

// EnablePort to enable Pon interface
func (dh *DeviceHandler) EnablePort(port *voltha.Port) error {
	log.Debugw("enable-port", log.Fields{"Device": dh.device, "port": port})
	return dh.invokeDisableorEnablePort(port, true)
}

// DisablePort to disable pon interface
func (dh *DeviceHandler) DisablePort(port *voltha.Port) error {
	log.Debugw("disable-port", log.Fields{"Device": dh.device, "port": port})
	return dh.invokeDisableorEnablePort(port, false)
}

func (dh *DeviceHandler) invokeDisableorEnablePort(port *voltha.Port, enablePort bool) error {
	ctx := context.Background()
	log.Infow("invokeDisableorEnablePort", log.Fields{"port": port, "Enable": enablePort})
	if port.GetType() == voltha.Port_ETHERNET_NNI {
		// Bug is opened for VOL-2505 to support NNI disable feature.
		log.Infow("voltha-supports-single-nni-hence-disable-of-nni-not-allowed",
			log.Fields{"Device": dh.device, "port": port})
		return fmt.Errorf("received-disable-enable-nni-port-request, received-port %s", port.GetType())
	}
	// fetch interfaceid from PortNo
	ponID := PortNoToIntfID(port.GetPortNo(), voltha.Port_PON_OLT)
	ponIntf := &oop.Interface{IntfId: ponID}
	var operStatus voltha.OperStatus_Types
	if enablePort {
		operStatus = voltha.OperStatus_ACTIVE
		out, err := dh.Client.EnablePonIf(ctx, ponIntf)

		if err != nil {
			log.Errorw("error-while-enable-Pon-port", log.Fields{"DeviceID": dh.device, "Port": port, "error": err})
			return err
		}
		// updating interface local cache for collecting stats
		dh.activePorts.Store(ponID, true)
		log.Infow("enabled-pon-port", log.Fields{"out": out, "DeviceID": dh.device, "Port": port})
	} else {
		operStatus = voltha.OperStatus_UNKNOWN
		out, err := dh.Client.DisablePonIf(ctx, ponIntf)
		if err != nil {
			log.Errorw("error-while-disabling-interface", log.Fields{"DeviceID": dh.device, "Port": port})
			return err
		}
		// updating interface local cache for collecting stats
		dh.activePorts.Store(ponID, false)
		log.Infow("disabled-pon-port", log.Fields{"out": out, "DeviceID": dh.device, "Port": port})
	}
	if errs := dh.coreProxy.PortStateUpdate(ctx, dh.deviceID, voltha.Port_PON_OLT, port.PortNo, operStatus); errs != nil {
		log.Errorw("portstate-update-failed", log.Fields{"Device": dh.deviceID, "port": port.PortNo, "error": errs})
		return errs
	}
	return nil
}

//updatePortAdminState update the ports on reboot and re-enable device.
func (dh *DeviceHandler) updatePortAdminState(device *voltha.Device) error {
	cloned := proto.Clone(device).(*voltha.Device)
	// Disable the port and update the oper_port_status to core
	// if the Admin state of the port is disabled on reboot and re-enable device.
	for _, port := range cloned.Ports {
		if port.AdminState == common.AdminState_DISABLED {
			if err := dh.invokeDisableorEnablePort(port, false); err != nil {
				log.Errorw("error-occurred-while-disabling-port", log.Fields{"DeviceId": dh.deviceID, "port": port, "error": err})
				return err
			}
		}
	}
	return nil
}

//populateActivePorts to populate activePorts map
func (dh *DeviceHandler) populateActivePorts(device *voltha.Device) {
	log.Info("populateActiveports", log.Fields{"Device": device})
	for _, port := range device.Ports {
		if port.Type == voltha.Port_ETHERNET_NNI {
			if port.OperStatus == voltha.OperStatus_ACTIVE {
				dh.activePorts.Store(PortNoToIntfID(port.PortNo, voltha.Port_ETHERNET_NNI), true)
			} else {
				dh.activePorts.Store(PortNoToIntfID(port.PortNo, voltha.Port_ETHERNET_NNI), false)
			}
		}
		if port.Type == voltha.Port_PON_OLT {
			if port.OperStatus == voltha.OperStatus_ACTIVE {
				dh.activePorts.Store(PortNoToIntfID(port.PortNo, voltha.Port_PON_OLT), true)
			} else {
				dh.activePorts.Store(PortNoToIntfID(port.PortNo, voltha.Port_PON_OLT), false)
			}
		}
	}
}
