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
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/opencord/voltha-lib-go/v3/pkg/flows"
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
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"
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
	InvalidPort    = 0xffffffff
)

// pendingFlowRemoveDataKey is key to pendingFlowRemoveDataPerSubscriber map
type pendingFlowRemoveDataKey struct {
	intfID uint32
	onuID  uint32
	uniID  uint32
}

// pendingFlowRemoveData is value stored in pendingFlowRemoveDataPerSubscriber map
// This holds the number of pending flow removes and also a signal channel to
// to indicate the receiver when all flow removes are handled
type pendingFlowRemoveData struct {
	pendingFlowRemoveCount uint32
	allFlowsRemoved        chan struct{}
}

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

	discOnus                      sync.Map
	onus                          sync.Map
	portStats                     *OpenOltStatisticsMgr
	metrics                       *pmmetrics.PmMetrics
	stopCollector                 chan bool
	stopHeartbeatCheck            chan bool
	activePorts                   sync.Map
	stopIndications               chan bool
	isReadIndicationRoutineActive bool

	// pendingFlowRemoveDataPerSubscriber map is used to maintain the context on a per
	// subscriber basis for the number of pending flow removes. This data is used
	// to process all the flow removes for a subscriber before handling flow adds.
	// Interleaving flow delete and flow add processing has known to cause PON resource
	// management contentions on a per subscriber bases, so we need ensure ordering.
	pendingFlowRemoveDataPerSubscriber map[pendingFlowRemoveDataKey]pendingFlowRemoveData
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
	losRaised     bool
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
func NewOnuDevice(devID, deviceTp, serialNum string, onuID, intfID uint32, proxyDevID string, losRaised bool) *OnuDevice {
	var device OnuDevice
	device.deviceID = devID
	device.deviceType = deviceTp
	device.serialNumber = serialNum
	device.onuID = onuID
	device.intfID = intfID
	device.proxyDeviceID = proxyDevID
	device.uniPorts = make(map[uint32]struct{})
	device.losRaised = losRaised
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
	dh.stopIndications = make(chan bool, 1)
	dh.pendingFlowRemoveDataPerSubscriber = make(map[pendingFlowRemoveDataKey]pendingFlowRemoveData)

	//TODO initialize the support classes.
	return &dh
}

// start save the device to the data model
func (dh *DeviceHandler) start(ctx context.Context) {
	dh.lockDevice.Lock()
	defer dh.lockDevice.Unlock()
	logger.Debugw("starting-device-agent", log.Fields{"device": dh.device})
	// Add the initial device to the local model
	logger.Debug("device-agent-started")
}

// stop stops the device dh.  Not much to do for now
func (dh *DeviceHandler) stop(ctx context.Context) {
	dh.lockDevice.Lock()
	defer dh.lockDevice.Unlock()
	logger.Debug("stopping-device-agent")
	dh.exitChannel <- 1
	logger.Debug("device-agent-stopped")
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

	logger.Debugw("generating-mac-from-host", log.Fields{"host": host})

	if addr = net.ParseIP(host); addr == nil {
		logger.Debugw("looking-up-hostname", log.Fields{"host": host})

		if ips, err = net.LookupHost(host); err == nil {
			logger.Debugw("dns-result-ips", log.Fields{"ips": ips})
			if addr = net.ParseIP(ips[0]); addr == nil {
				return "", olterrors.NewErrInvalidValue(log.Fields{"ip": ips[0]}, nil)
			}
			genmac = macifyIP(addr)
			logger.Debugw("using-ip-as-mac", log.Fields{"host": ips[0], "mac": genmac})
			return genmac, nil
		}
		return "", olterrors.NewErrAdapter("cannot-resolve-hostname-to-ip", log.Fields{"host": host}, err)
	}

	genmac = macifyIP(addr)
	logger.Debugw("using-ip-as-mac", log.Fields{"host": host, "mac": genmac})
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
func GetportLabel(portNum uint32, portType voltha.Port_PortType) (string, error) {

	switch portType {
	case voltha.Port_ETHERNET_NNI:
		return fmt.Sprintf("nni-%d", portNum), nil
	case voltha.Port_PON_OLT:
		return fmt.Sprintf("pon-%d", portNum), nil
	}

	return "", olterrors.NewErrInvalidValue(log.Fields{"port-type": portType}, nil)
}

func (dh *DeviceHandler) addPort(intfID uint32, portType voltha.Port_PortType, state string) error {
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
	label, err := GetportLabel(portNum, portType)
	if err != nil {
		return olterrors.NewErrNotFound("port-label", log.Fields{"port-number": portNum, "port-type": portType}, err)
	}

	device, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		return olterrors.NewErrNotFound("device", log.Fields{"device-id": dh.device.Id}, err)
	}
	if device.Ports != nil {
		for _, dPort := range device.Ports {
			if dPort.Type == portType && dPort.PortNo == portNum {
				logger.Debug("port-already-exists-updating-oper-status-of-port")
				if err := dh.coreProxy.PortStateUpdate(context.TODO(), dh.device.Id, portType, portNum, operStatus); err != nil {
					return olterrors.NewErrAdapter("failed-to-update-port-state", log.Fields{
						"device-id":   dh.device.Id,
						"port-type":   portType,
						"port-number": portNum,
						"oper-status": operStatus}, err)

				}
				return nil
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
	logger.Debugw("Sending-port-update-to-core", log.Fields{"port": port})
	// Synchronous call to update device - this method is run in its own go routine
	if err := dh.coreProxy.PortCreated(context.TODO(), dh.device.Id, port); err != nil {
		return olterrors.NewErrAdapter("error-creating-port", log.Fields{
			"device-id": dh.device.Id,
			"port-type": portType}, err)
	}
	return nil
}

// readIndications to read the indications from the OLT device
func (dh *DeviceHandler) readIndications(ctx context.Context) error {
	defer logger.Debugw("indications-ended", log.Fields{"device-id": dh.device.Id})
	defer func() {
		dh.lockDevice.Lock()
		dh.isReadIndicationRoutineActive = false
		dh.lockDevice.Unlock()
	}()
	indications, err := dh.startOpenOltIndicationStream(ctx)
	if err != nil {
		return err
	}
	/* get device state */
	device, err := dh.coreProxy.GetDevice(ctx, dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		return olterrors.NewErrNotFound("device", log.Fields{"device-id": dh.device.Id}, err)
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

	dh.lockDevice.Lock()
	dh.isReadIndicationRoutineActive = true
	dh.lockDevice.Unlock()

Loop:
	for {
		select {
		case <-dh.stopIndications:
			logger.Debugw("Stopping-collecting-indications-for-OLT", log.Fields{"deviceID:": dh.deviceID})
			break Loop
		default:
			indication, err := indications.Recv()
			if err == io.EOF {
				logger.Infow("EOF for  indications", log.Fields{"err": err})
				// Use an exponential back off to prevent getting into a tight loop
				duration := indicationBackoff.NextBackOff()
				if duration == backoff.Stop {
					// If we reach a maximum then warn and reset the backoff
					// timer and keep attempting.
					logger.Warnw("Maximum indication backoff reached, resetting backoff timer",
						log.Fields{"max_indication_backoff": indicationBackoff.MaxElapsedTime})
					indicationBackoff.Reset()
				}
				time.Sleep(indicationBackoff.NextBackOff())
				if indications, err = dh.startOpenOltIndicationStream(ctx); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				logger.Errorw("Read indication error", log.Fields{"err": err})
				if dh.adminState == "deleted" {
					logger.Debug("Device deleted stopping the read indication thread")
					break Loop
				}
				// Close the stream, and re-initialize it
				if err = indications.CloseSend(); err != nil {
					// Ok to ignore here, because we landed here due to a problem on the stream
					// In all probability, the closeSend call may fail
					logger.Debugw("error closing send stream, error ignored", log.Fields{"err": err})
				}
				if indications, err = dh.startOpenOltIndicationStream(ctx); err != nil {
					return err
				}
				// once we re-initialized the indication stream, continue to read indications
				continue
			}
			// Reset backoff if we have a successful receive
			indicationBackoff.Reset()
			dh.lockDevice.RLock()
			adminState := dh.adminState
			dh.lockDevice.RUnlock()
			// When OLT is admin down, ignore all indications.
			if adminState == "down" && !isIndicationAllowedDuringOltAdminDown(indication) {
				logger.Debugw("olt is admin down, ignore indication", log.Fields{"indication": indication})
				continue
			}
			dh.handleIndication(ctx, indication)
		}
	}
	// Close the send stream
	_ = indications.CloseSend() // Ok to ignore error, as we stopping the readIndication anyway

	return nil
}

func (dh *DeviceHandler) startOpenOltIndicationStream(ctx context.Context) (oop.Openolt_EnableIndicationClient, error) {

	indications, err := dh.Client.EnableIndication(ctx, new(oop.Empty))
	if err != nil {
		return nil, olterrors.NewErrCommunication("indication-read-failure", log.Fields{"device-id": dh.device.Id}, err).Log()
	}
	if indications == nil {
		return nil, olterrors.NewErrInvalidValue(log.Fields{"indications": nil, "device-id": dh.device.Id}, nil).Log()
	}

	return indications, nil
}

// isIndicationAllowedDuringOltAdminDown returns true if the indication is allowed during OLT Admin down, else false
func isIndicationAllowedDuringOltAdminDown(indication *oop.Indication) bool {
	switch indication.Data.(type) {
	case *oop.Indication_OltInd, *oop.Indication_IntfInd, *oop.Indication_IntfOperInd:
		return true

	default:
		return false
	}
}

func (dh *DeviceHandler) handleOltIndication(ctx context.Context, oltIndication *oop.OltIndication) error {
	raisedTs := time.Now().UnixNano()
	if oltIndication.OperState == "up" && dh.transitionMap.currentDeviceState != deviceStateUp {
		dh.transitionMap.Handle(ctx, DeviceUpInd)
	} else if oltIndication.OperState == "down" {
		dh.transitionMap.Handle(ctx, DeviceDownInd)
	}
	// Send or clear Alarm
	if err := dh.eventMgr.oltUpDownIndication(oltIndication, dh.deviceID, raisedTs); err != nil {
		return olterrors.NewErrAdapter("failed-indication", log.Fields{
			"device_id":  dh.deviceID,
			"indication": oltIndication,
			"timestamp":  raisedTs}, err)
	}
	return nil
}

// nolint: gocyclo
func (dh *DeviceHandler) handleIndication(ctx context.Context, indication *oop.Indication) {
	raisedTs := time.Now().UnixNano()
	switch indication.Data.(type) {
	case *oop.Indication_OltInd:
		if err := dh.handleOltIndication(ctx, indication.GetOltInd()); err != nil {
			olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "olt"}, err).Log()
		}
	case *oop.Indication_IntfInd:
		intfInd := indication.GetIntfInd()
		go func() {
			if err := dh.addPort(intfInd.GetIntfId(), voltha.Port_PON_OLT, intfInd.GetOperState()); err != nil {
				olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "interface"}, err).Log()
			}
		}()
		logger.Infow("Received interface indication ", log.Fields{"InterfaceInd": intfInd})
	case *oop.Indication_IntfOperInd:
		intfOperInd := indication.GetIntfOperInd()
		if intfOperInd.GetType() == "nni" {
			go func() {
				if err := dh.addPort(intfOperInd.GetIntfId(), voltha.Port_ETHERNET_NNI, intfOperInd.GetOperState()); err != nil {
					olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "interface-oper-nni"}, err).Log()
				}
			}()
			dh.resourceMgr.AddNNIToKVStore(ctx, intfOperInd.GetIntfId())
		} else if intfOperInd.GetType() == "pon" {
			// TODO: Check what needs to be handled here for When PON PORT down, ONU will be down
			// Handle pon port update
			go func() {
				if err := dh.addPort(intfOperInd.GetIntfId(), voltha.Port_PON_OLT, intfOperInd.GetOperState()); err != nil {
					olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "interface-oper-pon"}, err).Log()
				}
			}()
			go dh.eventMgr.oltIntfOperIndication(indication.GetIntfOperInd(), dh.deviceID, raisedTs)
		}
		logger.Infow("Received interface oper indication ", log.Fields{"InterfaceOperInd": intfOperInd})
	case *oop.Indication_OnuDiscInd:
		onuDiscInd := indication.GetOnuDiscInd()
		logger.Infow("Received Onu discovery indication ", log.Fields{"OnuDiscInd": onuDiscInd})
		sn := dh.stringifySerialNumber(onuDiscInd.SerialNumber)
		go func() {
			if err := dh.onuDiscIndication(ctx, onuDiscInd, sn); err != nil {
				olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "onu-discovery"}, err).Log()
			}
		}()
	case *oop.Indication_OnuInd:
		onuInd := indication.GetOnuInd()
		logger.Infow("Received Onu indication ", log.Fields{"OnuInd": onuInd})
		go func() {
			if err := dh.onuIndication(onuInd); err != nil {
				olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "onu"}, err).Log()
			}
		}()
	case *oop.Indication_OmciInd:
		omciInd := indication.GetOmciInd()
		logger.Debugw("Received Omci indication ", log.Fields{"IntfId": omciInd.IntfId, "OnuId": omciInd.OnuId})
		go func() {
			if err := dh.omciIndication(omciInd); err != nil {
				olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "omci"}, err).Log()
			}
		}()
	case *oop.Indication_PktInd:
		pktInd := indication.GetPktInd()
		logger.Infow("Received packet indication ", log.Fields{"PktInd": pktInd, "deviceId": dh.device.Id})
		go func() {
			if err := dh.handlePacketIndication(ctx, pktInd); err != nil {
				olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "packet"}, err).Log()
			}
		}()
	case *oop.Indication_PortStats:
		portStats := indication.GetPortStats()
		go dh.portStats.PortStatisticsIndication(portStats, dh.resourceMgr.DevInfo.GetPonPorts())
	case *oop.Indication_FlowStats:
		flowStats := indication.GetFlowStats()
		logger.Infow("Received flow stats", log.Fields{"FlowStats": flowStats})
	case *oop.Indication_AlarmInd:
		alarmInd := indication.GetAlarmInd()
		logger.Infow("Received alarm indication ", log.Fields{"AlarmInd": alarmInd})
		go dh.eventMgr.ProcessEvents(alarmInd, dh.deviceID, raisedTs)
	}
}

// doStateUp handle the olt up indication and update to voltha core
func (dh *DeviceHandler) doStateUp(ctx context.Context) error {
	//starting the stat collector
	go startCollector(dh)

	// Synchronous call to update device state - this method is run in its own go routine
	if err := dh.coreProxy.DeviceStateUpdate(ctx, dh.device.Id, voltha.ConnectStatus_REACHABLE,
		voltha.OperStatus_ACTIVE); err != nil {
		return olterrors.NewErrAdapter("device-update-failed", log.Fields{"device-id": dh.device.Id}, err)
	}
	return nil
}

// doStateDown handle the olt down indication
func (dh *DeviceHandler) doStateDown(ctx context.Context) error {
	dh.lockDevice.Lock()
	defer dh.lockDevice.Unlock()
	logger.Debug("do-state-down-start")

	device, err := dh.coreProxy.GetDevice(ctx, dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		return olterrors.NewErrNotFound("device", log.Fields{"device-id": dh.device.Id}, err)
	}

	cloned := proto.Clone(device).(*voltha.Device)

	//Update the device oper state and connection status
	cloned.OperStatus = voltha.OperStatus_UNKNOWN
	dh.device = cloned

	if err = dh.coreProxy.DeviceStateUpdate(ctx, cloned.Id, cloned.ConnectStatus, cloned.OperStatus); err != nil {
		return olterrors.NewErrAdapter("state-update-failed", log.Fields{"device-id": device.Id}, err)
	}

	//get the child device for the parent device
	onuDevices, err := dh.coreProxy.GetChildDevices(ctx, dh.device.Id)
	if err != nil {
		return olterrors.NewErrAdapter("child-device-fetch-failed", log.Fields{"device-id": dh.device.Id}, err)
	}
	for _, onuDevice := range onuDevices.Items {

		// Update onu state as down in onu adapter
		onuInd := oop.OnuIndication{}
		onuInd.OperState = "down"
		err := dh.AdapterProxy.SendInterAdapterMessage(ctx, &onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
			"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		if err != nil {
			olterrors.NewErrCommunication("inter-adapter-send-failed", log.Fields{
				"source":        "openolt",
				"onu-indicator": onuInd,
				"device-type":   onuDevice.Type,
				"device-id":     onuDevice.Id}, err).LogAt(log.ErrorLevel)
			//Do not return here and continue to process other ONUs
		}
	}
	/* Discovered ONUs entries need to be cleared , since after OLT
	   is up, it starts sending discovery indications again*/
	dh.discOnus = sync.Map{}
	logger.Debugw("do-state-down-end", log.Fields{"device-id": device.Id})
	return nil
}

// doStateInit dial the grpc before going to init state
func (dh *DeviceHandler) doStateInit(ctx context.Context) error {
	var err error
	if dh.clientCon, err = grpc.Dial(dh.device.GetHostAndPort(), grpc.WithInsecure(), grpc.WithBlock()); err != nil {
		return olterrors.NewErrCommunication("dial-failure", log.Fields{
			"device-id":     dh.deviceID,
			"host-and-port": dh.device.GetHostAndPort()}, err)
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
	logger.Debug("OLT device has been connected")

	// Case where OLT is disabled and then rebooted.
	if dh.adminState == "down" {
		logger.Debugln("do-state-connected--device-admin-state-down")
		device, err := dh.coreProxy.GetDevice(ctx, dh.device.Id, dh.device.Id)
		if err != nil || device == nil {
			/*TODO: needs to handle error scenarios */
			olterrors.NewErrAdapter("device-fetch-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}

		cloned := proto.Clone(device).(*voltha.Device)
		cloned.ConnectStatus = voltha.ConnectStatus_REACHABLE
		cloned.OperStatus = voltha.OperStatus_UNKNOWN
		dh.device = cloned
		if er := dh.coreProxy.DeviceStateUpdate(ctx, cloned.Id, cloned.ConnectStatus, cloned.OperStatus); er != nil {
			olterrors.NewErrAdapter("device-state-update-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}

		// Since the device was disabled before the OLT was rebooted, enforce the OLT to be Disabled after re-connection.
		_, err = dh.Client.DisableOlt(ctx, new(oop.Empty))
		if err != nil {
			olterrors.NewErrAdapter("olt-disable-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}
		// We should still go ahead an initialize various device handler modules so that when OLT is re-enabled, we have
		// all the modules initialized and ready to handle incoming ONUs.

		if err := dh.initializeDeviceHandlerModules(ctx); err != nil {
			olterrors.NewErrAdapter("device-handler-initialization-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}

		// Start reading indications
		go func() {
			if err := dh.readIndications(ctx); err != nil {
				olterrors.NewErrAdapter("indication-read-failure", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
			}
		}()
		return nil
	}

	device, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		return olterrors.NewErrAdapter("fetch-device-failed", log.Fields{"device-id": dh.device.Id}, err)
	}
	dh.populateActivePorts(device)
	if err := dh.disableAdminDownPorts(device); err != nil {
		return olterrors.NewErrAdapter("port-status-update-failed", log.Fields{"device": device}, err)
	}

	if err := dh.initializeDeviceHandlerModules(ctx); err != nil {
		olterrors.NewErrAdapter("device-handler-initialization-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
	}

	// Start reading indications
	go func() {
		if err := dh.readIndications(ctx); err != nil {
			olterrors.NewErrAdapter("read-indications-failure", log.Fields{"device-id": dh.device.Id}, err).Log()
		}
	}()
	return nil
}

func (dh *DeviceHandler) initializeDeviceHandlerModules(ctx context.Context) error {
	deviceInfo, err := dh.populateDeviceInfo()

	if err != nil {
		return olterrors.NewErrAdapter("populate-device-info-failed", log.Fields{"device-id": dh.device.Id}, err)
	}
	KVStoreHostPort := fmt.Sprintf("%s:%d", dh.openOLT.KVStoreHost, dh.openOLT.KVStorePort)
	// Instantiate resource manager
	if dh.resourceMgr = rsrcMgr.NewResourceMgr(ctx, dh.deviceID, KVStoreHostPort, dh.openOLT.KVStoreType, dh.deviceType, deviceInfo); dh.resourceMgr == nil {
		return olterrors.ErrResourceManagerInstantiating
	}

	// Instantiate flow manager
	if dh.flowMgr = NewFlowManager(ctx, dh, dh.resourceMgr); dh.flowMgr == nil {
		return olterrors.ErrResourceManagerInstantiating

	}
	/* TODO: Instantiate Alarm , stats , BW managers */
	/* Instantiating Event Manager to handle Alarms and KPIs */
	dh.eventMgr = NewEventMgr(dh.EventProxy, dh)

	// Stats config for new device
	dh.portStats = NewOpenOltStatsMgr(dh)

	return nil

}

func (dh *DeviceHandler) populateDeviceInfo() (*oop.DeviceInfo, error) {
	var err error
	var deviceInfo *oop.DeviceInfo

	deviceInfo, err = dh.Client.GetDeviceInfo(context.Background(), new(oop.Empty))

	if err != nil {
		return nil, olterrors.NewErrPersistence("get", "device", 0, nil, err)
	}
	if deviceInfo == nil {
		return nil, olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil)
	}

	logger.Debugw("Fetched device info", log.Fields{"deviceInfo": deviceInfo})
	dh.device.Root = true
	dh.device.Vendor = deviceInfo.Vendor
	dh.device.Model = deviceInfo.Model
	dh.device.SerialNumber = deviceInfo.DeviceSerialNumber
	dh.device.HardwareVersion = deviceInfo.HardwareVersion
	dh.device.FirmwareVersion = deviceInfo.FirmwareVersion

	if deviceInfo.DeviceId == "" {
		logger.Warnw("no-device-id-provided-using-host", log.Fields{"hostport": dh.device.GetHostAndPort()})
		host := strings.Split(dh.device.GetHostAndPort(), ":")[0]
		genmac, err := generateMacFromHost(host)
		if err != nil {
			return nil, olterrors.NewErrAdapter("failed-to-generate-mac-host", log.Fields{"host": host}, err)
		}
		logger.Debugw("using-host-for-mac-address", log.Fields{"host": host, "mac": genmac})
		dh.device.MacAddress = genmac
	} else {
		dh.device.MacAddress = deviceInfo.DeviceId
	}

	// Synchronous call to update device - this method is run in its own go routine
	if err := dh.coreProxy.DeviceUpdate(context.TODO(), dh.device); err != nil {
		return nil, olterrors.NewErrAdapter("device-update-failed", log.Fields{"device-id": dh.device.Id}, err)
	}

	return deviceInfo, nil
}

func startCollector(dh *DeviceHandler) {
	logger.Debugf("Starting-Collector")
	context := make(map[string]string)
	for {
		select {
		case <-dh.stopCollector:
			logger.Debugw("Stopping-Collector-for-OLT", log.Fields{"deviceID:": dh.deviceID})
			return
		default:
			freq := dh.metrics.ToPmConfigs().DefaultFreq
			time.Sleep(time.Duration(freq) * time.Second)
			context["oltid"] = dh.deviceID
			context["devicetype"] = dh.deviceType
			// NNI Stats
			cmnni := dh.portStats.collectNNIMetrics(uint32(0))
			logger.Debugf("Collect-NNI-Metrics %v", cmnni)
			go dh.portStats.publishMetrics("NNIStats", cmnni, uint32(0), context, dh.deviceID)
			logger.Debugf("Publish-NNI-Metrics")
			// PON Stats
			NumPonPORTS := dh.resourceMgr.DevInfo.GetPonPorts()
			for i := uint32(0); i < NumPonPORTS; i++ {
				if val, ok := dh.activePorts.Load(i); ok && val == true {
					cmpon := dh.portStats.collectPONMetrics(i)
					logger.Debugf("Collect-PON-Metrics %v", cmpon)
					go dh.portStats.publishMetrics("PONStats", cmpon, i, context, dh.deviceID)
					logger.Debugf("Publish-PON-Metrics")
				}
			}
		}
	}
}

//AdoptDevice adopts the OLT device
func (dh *DeviceHandler) AdoptDevice(ctx context.Context, device *voltha.Device) {
	dh.transitionMap = NewTransitionMap(dh)
	logger.Infow("Adopt_device", log.Fields{"deviceID": device.Id, "Address": device.GetHostAndPort()})
	dh.transitionMap.Handle(ctx, DeviceInit)

	// Now, set the initial PM configuration for that device
	if err := dh.coreProxy.DevicePMConfigUpdate(nil, dh.metrics.ToPmConfigs()); err != nil {
		olterrors.NewErrAdapter("error-updating-performance-metrics", log.Fields{"device-id": device.Id}, err).LogAt(log.ErrorLevel)
	}

	go startHeartbeatCheck(ctx, dh)
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
	logger.Debugw("omci indication", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
	var deviceType string
	var deviceID string
	var proxyDeviceID string

	transid := extractOmciTransactionID(omciInd.Pkt)
	logger.Debugw("recv-omci-msg", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId,
		"omciTransactionID": transid, "omciMsg": hex.EncodeToString(omciInd.Pkt)})

	onuKey := dh.formOnuKey(omciInd.IntfId, omciInd.OnuId)

	if onuInCache, ok := dh.onus.Load(onuKey); !ok {

		logger.Debugw("omci indication for a device not in cache.", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
		ponPort := IntfIDToPortNo(omciInd.GetIntfId(), voltha.Port_PON_OLT)
		kwargs := make(map[string]interface{})
		kwargs["onu_id"] = omciInd.OnuId
		kwargs["parent_port_no"] = ponPort

		onuDevice, err := dh.coreProxy.GetChildDevice(context.TODO(), dh.device.Id, kwargs)
		if err != nil {
			return olterrors.NewErrNotFound("onu", log.Fields{
				"interface-id": omciInd.IntfId,
				"onu-id":       omciInd.OnuId}, err)
		}
		deviceType = onuDevice.Type
		deviceID = onuDevice.Id
		proxyDeviceID = onuDevice.ProxyAddress.DeviceId
		//if not exist in cache, then add to cache.
		dh.onus.Store(onuKey, NewOnuDevice(deviceID, deviceType, onuDevice.SerialNumber, omciInd.OnuId, omciInd.IntfId, proxyDeviceID, false))
	} else {
		//found in cache
		logger.Debugw("omci indication for a device in cache.", log.Fields{"intfID": omciInd.IntfId, "onuID": omciInd.OnuId})
		deviceType = onuInCache.(*OnuDevice).deviceType
		deviceID = onuInCache.(*OnuDevice).deviceID
		proxyDeviceID = onuInCache.(*OnuDevice).proxyDeviceID
	}

	omciMsg := &ic.InterAdapterOmciMessage{Message: omciInd.Pkt}
	if err := dh.AdapterProxy.SendInterAdapterMessage(context.Background(), omciMsg,
		ic.InterAdapterMessageType_OMCI_REQUEST, dh.deviceType, deviceType,
		deviceID, proxyDeviceID, ""); err != nil {
		return olterrors.NewErrCommunication("omci-request", log.Fields{
			"source":          dh.deviceType,
			"destination":     deviceType,
			"onu-id":          deviceID,
			"proxy-device-id": proxyDeviceID}, err)
	}
	return nil
}

//ProcessInterAdapterMessage sends the proxied messages to the target device
// If the proxy address is not found in the unmarshalled message, it first fetches the onu device for which the message
// is meant, and then send the unmarshalled omci message to this onu
func (dh *DeviceHandler) ProcessInterAdapterMessage(msg *ic.InterAdapterMessage) error {
	logger.Debugw("Process_inter_adapter_message", log.Fields{"msgID": msg.Header.Id})
	if msg.Header.Type == ic.InterAdapterMessageType_OMCI_REQUEST {
		msgID := msg.Header.Id
		fromTopic := msg.Header.FromTopic
		toTopic := msg.Header.ToTopic
		toDeviceID := msg.Header.ToDeviceId
		proxyDeviceID := msg.Header.ProxyDeviceId

		logger.Debugw("omci request message header", log.Fields{"msgID": msgID, "fromTopic": fromTopic, "toTopic": toTopic, "toDeviceID": toDeviceID, "proxyDeviceID": proxyDeviceID})

		msgBody := msg.GetBody()

		omciMsg := &ic.InterAdapterOmciMessage{}
		if err := ptypes.UnmarshalAny(msgBody, omciMsg); err != nil {
			return olterrors.NewErrAdapter("cannot-unmarshal-omci-msg-body", log.Fields{"msgbody": msgBody}, err)
		}

		if omciMsg.GetProxyAddress() == nil {
			onuDevice, err := dh.coreProxy.GetDevice(context.TODO(), dh.device.Id, toDeviceID)
			if err != nil {
				return olterrors.NewErrNotFound("onu", log.Fields{
					"device-id":     dh.device.Id,
					"onu-device-id": toDeviceID}, err)
			}
			logger.Debugw("device retrieved from core", log.Fields{"msgID": msgID, "fromTopic": fromTopic, "toTopic": toTopic, "toDeviceID": toDeviceID, "proxyDeviceID": proxyDeviceID})
			if err := dh.sendProxiedMessage(onuDevice, omciMsg); err != nil {
				return olterrors.NewErrCommunication("send-failed", log.Fields{
					"device-id":     dh.device.Id,
					"onu-device-id": toDeviceID}, err)
			}
		} else {
			logger.Debugw("Proxy Address found in omci message", log.Fields{"msgID": msgID, "fromTopic": fromTopic, "toTopic": toTopic, "toDeviceID": toDeviceID, "proxyDeviceID": proxyDeviceID})
			if err := dh.sendProxiedMessage(nil, omciMsg); err != nil {
				return olterrors.NewErrCommunication("send-failed", log.Fields{
					"device-id":     dh.device.Id,
					"onu-device-id": toDeviceID}, err)
			}
		}

	} else {
		return olterrors.NewErrInvalidValue(log.Fields{"inter-adapter-message-type": msg.Header.Type}, nil)
	}
	return nil
}

func (dh *DeviceHandler) sendProxiedMessage(onuDevice *voltha.Device, omciMsg *ic.InterAdapterOmciMessage) error {
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
		logger.Debugw("ONU is not reachable, cannot send OMCI", log.Fields{"intfID": intfID, "onuID": onuID})

		return olterrors.NewErrCommunication("unreachable", log.Fields{
			"interface-id": intfID,
			"onu-id":       onuID}, nil)
	}

	// TODO: OpenOLT Agent oop.OmciMsg expects a hex encoded string for OMCI packets rather than the actual bytes.
	//  Fix this in the agent and then we can pass byte array as Pkt: omciMsg.Message.
	var omciMessage *oop.OmciMsg
	hexPkt := make([]byte, hex.EncodedLen(len(omciMsg.Message)))
	hex.Encode(hexPkt, omciMsg.Message)
	omciMessage = &oop.OmciMsg{IntfId: intfID, OnuId: onuID, Pkt: hexPkt}

	// TODO: Below logging illustrates the "stringify" of the omci Pkt.
	//  once above is fixed this log line can change to just use hex.EncodeToString(omciMessage.Pkt)
	transid := extractOmciTransactionID(omciMsg.Message)
	logger.Debugw("sent-omci-msg", log.Fields{"intfID": intfID, "onuID": onuID,
		"omciTransactionID": transid, "omciMsg": string(omciMessage.Pkt)})

	_, err := dh.Client.OmciMsgOut(context.Background(), omciMessage)
	if err != nil {
		return olterrors.NewErrCommunication("omci-send-failed", log.Fields{
			"interface-id": intfID,
			"onu-id":       onuID,
			"message":      omciMessage}, err)
	}
	return nil
}

func (dh *DeviceHandler) activateONU(ctx context.Context, intfID uint32, onuID int64, serialNum *oop.SerialNumber, serialNumber string) error {
	logger.Debugw("activate-onu", log.Fields{"intfID": intfID, "onuID": onuID, "serialNum": serialNum, "serialNumber": serialNumber})
	if err := dh.flowMgr.UpdateOnuInfo(ctx, intfID, uint32(onuID), serialNumber); err != nil {
		return olterrors.NewErrAdapter("onu-activate-failed", log.Fields{"onu": onuID, "intfID": intfID}, err)
	}
	// TODO: need resource manager
	var pir uint32 = 1000000
	Onu := oop.Onu{IntfId: intfID, OnuId: uint32(onuID), SerialNumber: serialNum, Pir: pir}
	if _, err := dh.Client.ActivateOnu(ctx, &Onu); err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.AlreadyExists {
			logger.Debug("ONU activation is in progress", log.Fields{"SerialNumber": serialNumber})
		} else {
			return olterrors.NewErrAdapter("onu-activate-failed", log.Fields{"onu": Onu}, err)
		}
	} else {
		logger.Infow("activated-onu", log.Fields{"SerialNumber": serialNumber})
	}
	return nil
}

func (dh *DeviceHandler) onuDiscIndication(ctx context.Context, onuDiscInd *oop.OnuDiscIndication, sn string) error {

	channelID := onuDiscInd.GetIntfId()
	parentPortNo := IntfIDToPortNo(onuDiscInd.GetIntfId(), voltha.Port_PON_OLT)

	logger.Infow("new-discovery-indication", log.Fields{"sn": sn})

	kwargs := make(map[string]interface{})
	if sn != "" {
		kwargs["serial_number"] = sn
	} else {
		return olterrors.NewErrInvalidValue(log.Fields{"serial-number": sn}, nil)
	}

	var alarmInd oop.OnuAlarmIndication
	raisedTs := time.Now().UnixNano()
	if _, loaded := dh.discOnus.LoadOrStore(sn, true); loaded {

		/* When PON cable disconnected and connected back from OLT, it was expected OnuAlarmIndication
		   with "los_status: off" should be raised but BAL does not raise this Alarm hence manually sending
		   OnuLosClear event on receiving OnuDiscoveryIndication for the Onu after checking whether
		   OnuLosRaise event sent for it */
		dh.onus.Range(func(Onukey interface{}, onuInCache interface{}) bool {
			if onuInCache.(*OnuDevice).serialNumber == sn && onuInCache.(*OnuDevice).losRaised {
				if onuDiscInd.GetIntfId() != onuInCache.(*OnuDevice).intfID {
					logger.Warnw("ONU-is-on-a-different-intf-id-now", log.Fields{
						"previousIntfId": onuInCache.(*OnuDevice).intfID,
						"currentIntfId":  onuDiscInd.GetIntfId()})
					// TODO:: Should we need to ignore raising OnuLosClear event
					// when onu connected to different PON?
				}
				alarmInd.IntfId = onuInCache.(*OnuDevice).intfID
				alarmInd.OnuId = onuInCache.(*OnuDevice).onuID
				alarmInd.LosStatus = statusCheckOff
				go dh.eventMgr.onuAlarmIndication(&alarmInd, onuInCache.(*OnuDevice).deviceID, raisedTs)
			}
			return true
		})

		logger.Warnw("onu-sn-is-already-being-processed", log.Fields{"sn": sn})
		return nil
	}

	var onuID uint32

	// check the ONU is already know to the OLT
	// NOTE the second time the ONU is discovered this should return a device
	onuDevice, err := dh.coreProxy.GetChildDevice(ctx, dh.device.Id, kwargs)

	if err != nil {
		logger.Warnw("core-proxy-get-child-device-failed", log.Fields{"parentDevice": dh.device.Id, "err": err, "sn": sn})
		if e, ok := status.FromError(err); ok {
			logger.Warnw("core-proxy-get-child-device-failed-with-code", log.Fields{"errCode": e.Code(), "sn": sn})
			switch e.Code() {
			case codes.Internal:
				// this probably means NOT FOUND, so just create a new device
				onuDevice = nil
			case codes.DeadlineExceeded:
				// if the call times out, cleanup and exit
				dh.discOnus.Delete(sn)
				return olterrors.NewErrTimeout("get-child-device", log.Fields{"device-id": dh.device.Id}, err)
			}
		}
	}

	if onuDevice == nil {
		// NOTE this should happen a single time, and only if GetChildDevice returns NotFound
		logger.Infow("creating-new-onu", log.Fields{"sn": sn})
		// we need to create a new ChildDevice
		ponintfid := onuDiscInd.GetIntfId()
		dh.lockDevice.Lock()
		onuID, err = dh.resourceMgr.GetONUID(ctx, ponintfid)
		dh.lockDevice.Unlock()

		logger.Infow("creating-new-onu-got-onu-id", log.Fields{"sn": sn, "onuId": onuID})

		if err != nil {
			// if we can't create an ID in resource manager,
			// cleanup and exit
			dh.discOnus.Delete(sn)
			return olterrors.NewErrAdapter("resource-manager-get-onu-id-failed", log.Fields{
				"pon-interface-id": ponintfid,
				"serial-number":    sn}, err)
		}

		if onuDevice, err = dh.coreProxy.ChildDeviceDetected(context.TODO(), dh.device.Id, int(parentPortNo),
			"", int(channelID), string(onuDiscInd.SerialNumber.GetVendorId()), sn, int64(onuID)); err != nil {
			dh.discOnus.Delete(sn)
			dh.resourceMgr.FreeonuID(ctx, ponintfid, []uint32{onuID}) // NOTE I'm not sure this method is actually cleaning up the right thing
			return olterrors.NewErrAdapter("core-proxy-child-device-detected-failed", log.Fields{
				"pon-interface-id": ponintfid,
				"serial-number":    sn}, err)
		}
		dh.eventMgr.OnuDiscoveryIndication(onuDiscInd, onuDevice.Id, onuID, sn, time.Now().UnixNano())
		logger.Infow("onu-child-device-added", log.Fields{"onuDevice": onuDevice, "sn": sn})
	}

	// we can now use the existing ONU Id
	onuID = onuDevice.ProxyAddress.OnuId
	//Insert the ONU into cache to use in OnuIndication.
	//TODO: Do we need to remove this from the cache on ONU change, or wait for overwritten on next discovery.
	logger.Debugw("onu-discovery-indication-key-create", log.Fields{"onuID": onuID,
		"intfId": onuDiscInd.GetIntfId(), "sn": sn})
	onuKey := dh.formOnuKey(onuDiscInd.GetIntfId(), onuID)

	onuDev := NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuID, onuDiscInd.GetIntfId(), onuDevice.ProxyAddress.DeviceId, false)
	dh.onus.Store(onuKey, onuDev)
	logger.Debugw("new-onu-device-discovered", log.Fields{"onu": onuDev, "sn": sn})

	if err = dh.coreProxy.DeviceStateUpdate(ctx, onuDevice.Id, common.ConnectStatus_REACHABLE, common.OperStatus_DISCOVERED); err != nil {
		return olterrors.NewErrAdapter("failed-to-update-device-state", log.Fields{
			"device-id":     onuDevice.Id,
			"serial-number": sn}, err)
	}
	logger.Infow("onu-discovered-reachable", log.Fields{"deviceId": onuDevice.Id, "sn": sn})
	if err = dh.activateONU(ctx, onuDiscInd.IntfId, int64(onuID), onuDiscInd.SerialNumber, sn); err != nil {
		return olterrors.NewErrAdapter("onu-activation-failed", log.Fields{
			"device-id":     onuDevice.Id,
			"serial-number": sn}, err)
	}
	return nil
}

func (dh *DeviceHandler) onuIndication(onuInd *oop.OnuIndication) error {
	serialNumber := dh.stringifySerialNumber(onuInd.SerialNumber)

	kwargs := make(map[string]interface{})
	ponPort := IntfIDToPortNo(onuInd.GetIntfId(), voltha.Port_PON_OLT)
	var onuDevice *voltha.Device
	var err error
	foundInCache := false
	logger.Debugw("ONU indication key create", log.Fields{"onuId": onuInd.OnuId,
		"intfId": onuInd.GetIntfId()})
	onuKey := dh.formOnuKey(onuInd.GetIntfId(), onuInd.OnuId)

	errFields := log.Fields{"device-id": dh.device.Id}

	if onuInCache, ok := dh.onus.Load(onuKey); ok {

		//If ONU id is discovered before then use GetDevice to get onuDevice because it is cheaper.
		foundInCache = true
		errFields["onu-id"] = onuInCache.(*OnuDevice).deviceID
		onuDevice, err = dh.coreProxy.GetDevice(nil, dh.device.Id, onuInCache.(*OnuDevice).deviceID)
	} else {
		//If ONU not found in adapter cache then we have to use GetChildDevice to get onuDevice
		if serialNumber != "" {
			kwargs["serial_number"] = serialNumber
			errFields["serial-number"] = serialNumber
		} else {
			kwargs["onu_id"] = onuInd.OnuId
			kwargs["parent_port_no"] = ponPort
			errFields["onu-id"] = onuInd.OnuId
			errFields["parent-port-no"] = ponPort
		}
		onuDevice, err = dh.coreProxy.GetChildDevice(context.TODO(), dh.device.Id, kwargs)
	}

	if err != nil || onuDevice == nil {
		return olterrors.NewErrNotFound("onu-device", errFields, err)
	}

	if onuDevice.ParentPortNo != ponPort {
		logger.Warnw("ONU-is-on-a-different-intf-id-now", log.Fields{
			"previousIntfId": onuDevice.ParentPortNo,
			"currentIntfId":  ponPort})
	}

	if onuDevice.ProxyAddress.OnuId != onuInd.OnuId {
		logger.Warnw("ONU-id-mismatch, can happen if both voltha and the olt rebooted", log.Fields{
			"expected_onu_id": onuDevice.ProxyAddress.OnuId,
			"received_onu_id": onuInd.OnuId})
	}
	if !foundInCache {
		onuKey := dh.formOnuKey(onuInd.GetIntfId(), onuInd.GetOnuId())

		dh.onus.Store(onuKey, NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuInd.GetOnuId(), onuInd.GetIntfId(), onuDevice.ProxyAddress.DeviceId, false))

	}
	if err := dh.updateOnuStates(onuDevice, onuInd); err != nil {
		return olterrors.NewErrCommunication("state-update-failed", errFields, err)
	}
	return nil
}

func (dh *DeviceHandler) updateOnuStates(onuDevice *voltha.Device, onuInd *oop.OnuIndication) error {
	ctx := context.TODO()
	logger.Debugw("onu-indication-for-state", log.Fields{"onuIndication": onuInd, "DeviceId": onuDevice.Id, "operStatus": onuDevice.OperStatus, "adminStatus": onuDevice.AdminState})
	if onuInd.AdminState == "down" {
		// Tests have shown that we sometimes get OperState as NOT down even if AdminState is down, forcing it
		if onuInd.OperState != "down" {
			logger.Warnw("ONU-admin-state-down", log.Fields{"operState": onuInd.OperState})
			onuInd.OperState = "down"
		}
	}

	switch onuInd.OperState {
	case "down":
		logger.Debugw("sending-interadapter-onu-indication", log.Fields{"onuIndication": onuInd, "DeviceId": onuDevice.Id, "operStatus": onuDevice.OperStatus, "adminStatus": onuDevice.AdminState})
		// TODO NEW CORE do not hardcode adapter name. Handler needs Adapter reference
		err := dh.AdapterProxy.SendInterAdapterMessage(ctx, onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
			"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		if err != nil {
			return olterrors.NewErrCommunication("inter-adapter-send-failed", log.Fields{
				"onu-indicator": onuInd,
				"source":        "openolt",
				"device-type":   onuDevice.Type,
				"device-id":     onuDevice.Id}, err)
		}
	case "up":
		logger.Debugw("sending-interadapter-onu-indication", log.Fields{"onuIndication": onuInd, "DeviceId": onuDevice.Id, "operStatus": onuDevice.OperStatus, "adminStatus": onuDevice.AdminState})
		// TODO NEW CORE do not hardcode adapter name. Handler needs Adapter reference
		err := dh.AdapterProxy.SendInterAdapterMessage(ctx, onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
			"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
		if err != nil {
			return olterrors.NewErrCommunication("inter-adapter-send-failed", log.Fields{
				"onu-indicator": onuInd,
				"source":        "openolt",
				"device-type":   onuDevice.Type,
				"device-id":     onuDevice.Id}, err)
		}
	default:
		return olterrors.NewErrInvalidValue(log.Fields{"oper-state": onuInd.OperState}, nil)
	}
	return nil
}

func (dh *DeviceHandler) stringifySerialNumber(serialNum *oop.SerialNumber) string {
	if serialNum != nil {
		return string(serialNum.VendorId) + dh.stringifyVendorSpecific(serialNum.VendorSpecific)
	}
	return ""
}
func (dh *DeviceHandler) deStringifySerialNumber(serialNum string) (*oop.SerialNumber, error) {
	decodedStr, err := hex.DecodeString(serialNum[4:])
	if err != nil {
		return nil, err
	}
	return &oop.SerialNumber{
		VendorId:       []byte(serialNum[:4]),
		VendorSpecific: []byte(decodedStr),
	}, nil
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
	return olterrors.ErrNotImplemented
}

//GetChildDevice returns the child device for given parent port and onu id
func (dh *DeviceHandler) GetChildDevice(parentPort, onuID uint32) (*voltha.Device, error) {
	logger.Debugw("GetChildDevice", log.Fields{"pon port": parentPort, "onuID": onuID})
	kwargs := make(map[string]interface{})
	kwargs["onu_id"] = onuID
	kwargs["parent_port_no"] = parentPort
	onuDevice, err := dh.coreProxy.GetChildDevice(context.TODO(), dh.device.Id, kwargs)
	if err != nil {
		return nil, olterrors.NewErrNotFound("onu-device", log.Fields{
			"interface-id": parentPort,
			"onu-id":       onuID}, err)
	}
	logger.Debugw("Successfully received child device from core", log.Fields{"child_device_id": onuDevice.Id, "child_device_sn": onuDevice.SerialNumber})
	return onuDevice, nil
}

// SendPacketInToCore sends packet-in to core
// For this, it calls SendPacketIn of the core-proxy which uses a device specific topic to send the request.
// The adapter handling the device creates a device specific topic
func (dh *DeviceHandler) SendPacketInToCore(logicalPort uint32, packetPayload []byte) error {
	logger.Debugw("send-packet-in-to-core", log.Fields{
		"port":   logicalPort,
		"packet": hex.EncodeToString(packetPayload),
	})
	if err := dh.coreProxy.SendPacketIn(context.TODO(), dh.device.Id, logicalPort, packetPayload); err != nil {
		return olterrors.NewErrCommunication("packet-send-failed", log.Fields{
			"source":       "adapter",
			"destination":  "core",
			"device-id":    dh.device.Id,
			"logical-port": logicalPort,
			"packet":       hex.EncodeToString(packetPayload)}, err)
	}
	logger.Debugw("Sent packet-in to core successfully", log.Fields{
		"packet": hex.EncodeToString(packetPayload),
	})
	return nil
}

// AddUniPortToOnu adds the uni port to the onu device
func (dh *DeviceHandler) AddUniPortToOnu(intfID, onuID, uniPort uint32) {
	onuKey := dh.formOnuKey(intfID, onuID)

	if onuDevice, ok := dh.onus.Load(onuKey); ok {
		// add it to the uniPort map for the onu device
		if _, ok = onuDevice.(*OnuDevice).uniPorts[uniPort]; !ok {
			onuDevice.(*OnuDevice).uniPorts[uniPort] = struct{}{}
			logger.Debugw("adding-uni-port", log.Fields{"port": uniPort, "intfID": intfID, "onuId": onuID})
		}
	}
}

//UpdateFlowsIncrementally updates the device flow
func (dh *DeviceHandler) UpdateFlowsIncrementally(ctx context.Context, device *voltha.Device, flows *of.FlowChanges, groups *of.FlowGroupChanges, flowMetadata *voltha.FlowMetadata) error {
	logger.Debugw("Received-incremental-flowupdate-in-device-handler", log.Fields{"deviceID": device.Id, "flows": flows, "groups": groups, "flowMetadata": flowMetadata})

	var errorsList []error

	if flows != nil {
		for _, flow := range flows.ToRemove.Items {
			dh.incrementActiveFlowRemoveCount(flow)

			logger.Debug("Removing flow", log.Fields{"deviceId": device.Id, "flowToRemove": flow})
			err := dh.flowMgr.RemoveFlow(ctx, flow)
			if err != nil {
				errorsList = append(errorsList, err)
			}

			dh.decrementActiveFlowRemoveCount(flow)
		}

		for _, flow := range flows.ToAdd.Items {
			logger.Debug("Adding flow", log.Fields{"deviceId": device.Id, "flowToAdd": flow})
			// If there are active Flow Remove in progress for a given subscriber, wait until it completes
			dh.waitForFlowRemoveToFinish(flow)
			err := dh.flowMgr.AddFlow(ctx, flow, flowMetadata)
			if err != nil {
				errorsList = append(errorsList, err)
			}
		}
	}
	if groups != nil && flows != nil {
		for _, flow := range flows.ToRemove.Items {
			logger.Debug("Removing flow", log.Fields{"deviceID": device.Id, "flowToRemove": flow})
			//  dh.flowMgr.RemoveFlow(flow)
		}
	}

	// Whether we need to synchronize multicast group adds and modifies like flow add and delete needs to be investigated
	if groups != nil {
		for _, group := range groups.ToAdd.Items {
			err := dh.flowMgr.AddGroup(ctx, group)
			if err != nil {
				errorsList = append(errorsList, err)
			}
		}
		for _, group := range groups.ToUpdate.Items {
			err := dh.flowMgr.ModifyGroup(ctx, group)
			if err != nil {
				errorsList = append(errorsList, err)
			}
		}
		if len(groups.ToRemove.Items) != 0 {
			logger.Debug("Group delete operation is not supported for now")
		}
	}
	if len(errorsList) > 0 {
		return fmt.Errorf("errors-installing-flows-groups, errors:%v", errorsList)
	}
	logger.Debug("UpdateFlowsIncrementally done successfully")
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
				dh.lockDevice.Lock()
				dh.adminState = "up"
				dh.lockDevice.Unlock()
				return olterrors.NewErrAdapter("olt-disable-failed", log.Fields{"device-id": device.Id}, err)
			}
		}
	}
	logger.Debugw("olt-disabled", log.Fields{"deviceID": device.Id})
	/* Discovered ONUs entries need to be cleared , since on device disable the child devices goes to
	UNREACHABLE state which needs to be configured again*/

	dh.discOnus = sync.Map{}
	dh.onus = sync.Map{}

	//stopping the stats collector
	dh.stopCollector <- true

	go dh.notifyChildDevices("unreachable")
	cloned := proto.Clone(device).(*voltha.Device)
	// Update the all pon ports state on that device to disable and NNI remains active as NNI remains active in openolt agent.
	for _, port := range cloned.Ports {
		if port.GetType() == voltha.Port_PON_OLT {
			if err := dh.coreProxy.PortStateUpdate(context.TODO(), cloned.Id,
				voltha.Port_PON_OLT, port.GetPortNo(), voltha.OperStatus_UNKNOWN); err != nil {
				return olterrors.NewErrAdapter("port-state-update-failed", log.Fields{"device-id": device.Id, "port-number": port.GetPortNo()}, err)
			}
		}
	}

	logger.Debugw("disable-device-end", log.Fields{"deviceID": device.Id})
	return nil
}

func (dh *DeviceHandler) notifyChildDevices(state string) {

	// Update onu state as unreachable in onu adapter
	onuInd := oop.OnuIndication{}
	onuInd.OperState = state
	//get the child device for the parent device
	onuDevices, err := dh.coreProxy.GetChildDevices(context.TODO(), dh.device.Id)
	if err != nil {
		logger.Errorw("failed-to-get-child-devices-information", log.Fields{"deviceID": dh.device.Id, "error": err})
	}
	if onuDevices != nil {
		for _, onuDevice := range onuDevices.Items {
			err := dh.AdapterProxy.SendInterAdapterMessage(context.TODO(), &onuInd, ic.InterAdapterMessageType_ONU_IND_REQUEST,
				"openolt", onuDevice.Type, onuDevice.Id, onuDevice.ProxyAddress.DeviceId, "")
			if err != nil {
				logger.Errorw("failed-to-send-inter-adapter-message", log.Fields{"OnuInd": onuInd,
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
			dh.lockDevice.Lock()
			dh.adminState = "down"
			dh.lockDevice.Unlock()
			return olterrors.NewErrAdapter("olt-reenable-failed", log.Fields{"device-id": dh.device.Id}, err)
		}
	}
	logger.Debug("olt-reenabled")

	cloned := proto.Clone(device).(*voltha.Device)
	// Update the all ports state on that device to enable

	if err := dh.disableAdminDownPorts(device); err != nil {
		return olterrors.NewErrAdapter("port-status-update-failed-after-olt-reenable", log.Fields{"device": device}, err)
	}
	//Update the device oper status as ACTIVE
	cloned.OperStatus = voltha.OperStatus_ACTIVE
	dh.device = cloned

	if err := dh.coreProxy.DeviceStateUpdate(context.TODO(), cloned.Id, cloned.ConnectStatus, cloned.OperStatus); err != nil {
		return olterrors.NewErrAdapter("state-update-failed", log.Fields{
			"device-id":      device.Id,
			"connect-status": cloned.ConnectStatus,
			"oper-status":    cloned.OperStatus}, err)
	}

	logger.Debugw("ReEnableDevice-end", log.Fields{"deviceID": device.Id})

	return nil
}

func (dh *DeviceHandler) clearUNIData(ctx context.Context, onu *rsrcMgr.OnuGemInfo) error {
	var uniID uint32
	var err error
	for _, port := range onu.UniPorts {
		uniID = UniIDFromPortNum(uint32(port))
		logger.Debugw("clearing-resource-data-for-uni-port", log.Fields{"port": port, "uniID": uniID})
		/* Delete tech-profile instance from the KV store */
		if err = dh.flowMgr.DeleteTechProfileInstances(ctx, onu.IntfID, onu.OnuID, uniID, onu.SerialNumber); err != nil {
			logger.Debugw("Failed-to-remove-tech-profile-instance-for-onu", log.Fields{"onu-id": onu.OnuID})
		}
		logger.Debugw("Deleted-tech-profile-instance-for-onu", log.Fields{"onu-id": onu.OnuID})
		flowIDs := dh.resourceMgr.GetCurrentFlowIDsForOnu(ctx, onu.IntfID, int32(onu.OnuID), int32(uniID))
		for _, flowID := range flowIDs {
			dh.resourceMgr.FreeFlowID(ctx, onu.IntfID, int32(onu.OnuID), int32(uniID), flowID)
		}
		tpIDList := dh.resourceMgr.GetTechProfileIDForOnu(ctx, onu.IntfID, onu.OnuID, uniID)
		for _, tpID := range tpIDList {
			if err = dh.resourceMgr.RemoveMeterIDForOnu(ctx, "upstream", onu.IntfID, onu.OnuID, uniID, tpID); err != nil {
				logger.Debugw("Failed-to-remove-meter-id-for-onu-upstream", log.Fields{"onu-id": onu.OnuID})
			}
			logger.Debugw("Removed-meter-id-for-onu-upstream", log.Fields{"onu-id": onu.OnuID})
			if err = dh.resourceMgr.RemoveMeterIDForOnu(ctx, "downstream", onu.IntfID, onu.OnuID, uniID, tpID); err != nil {
				logger.Debugw("Failed-to-remove-meter-id-for-onu-downstream", log.Fields{"onu-id": onu.OnuID})
			}
			logger.Debugw("Removed-meter-id-for-onu-downstream", log.Fields{"onu-id": onu.OnuID})
		}
		dh.resourceMgr.FreePONResourcesForONU(ctx, onu.IntfID, onu.OnuID, uniID)
		if err = dh.resourceMgr.RemoveTechProfileIDsForOnu(ctx, onu.IntfID, onu.OnuID, uniID); err != nil {
			logger.Debugw("Failed-to-remove-tech-profile-id-for-onu", log.Fields{"onu-id": onu.OnuID})
		}
		logger.Debugw("Removed-tech-profile-id-for-onu", log.Fields{"onu-id": onu.OnuID})
		if err = dh.resourceMgr.DelGemPortPktIn(ctx, onu.IntfID, onu.OnuID, uint32(port)); err != nil {
			logger.Debugw("Failed-to-remove-gemport-pkt-in", log.Fields{"intfid": onu.IntfID, "onuid": onu.OnuID, "uniId": uniID})
		}
	}
	return nil
}

func (dh *DeviceHandler) clearNNIData(ctx context.Context) error {
	nniUniID := -1
	nniOnuID := -1

	if dh.resourceMgr == nil {
		return olterrors.NewErrNotFound("resource-manager", log.Fields{"device-id": dh.deviceID}, nil)
	}
	//Free the flow-ids for the NNI port
	nni, err := dh.resourceMgr.GetNNIFromKVStore(ctx)
	if err != nil {
		return olterrors.NewErrPersistence("get", "nni", 0, nil, err)
	}
	logger.Debugw("NNI are ", log.Fields{"nni": nni})
	for _, nniIntfID := range nni {
		flowIDs := dh.resourceMgr.GetCurrentFlowIDsForOnu(ctx, uint32(nniIntfID), int32(nniOnuID), int32(nniUniID))
		logger.Debugw("Current flow ids for nni", log.Fields{"flow-ids": flowIDs})
		for _, flowID := range flowIDs {
			dh.resourceMgr.FreeFlowID(ctx, uint32(nniIntfID), -1, -1, uint32(flowID))
		}
		dh.resourceMgr.RemoveResourceMap(ctx, nniIntfID, int32(nniOnuID), int32(nniUniID))
	}
	if err = dh.resourceMgr.DelNNiFromKVStore(ctx); err != nil {
		return olterrors.NewErrPersistence("clear", "nni", 0, nil, err)
	}
	return nil
}

// DeleteDevice deletes the device instance from openolt handler array.  Also clears allocated resource manager resources.  Also reboots the OLT hardware!
func (dh *DeviceHandler) DeleteDevice(ctx context.Context, device *voltha.Device) error {
	logger.Debug("Function entry delete device")
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
	go dh.cleanupDeviceResources(ctx)
	logger.Debug("Removed-device-from-Resource-manager-KV-store")
	// Stop the Stats collector
	dh.stopCollector <- true
	// stop the heartbeat check routine
	dh.stopHeartbeatCheck <- true
	//Reset the state
	if dh.Client != nil {
		if _, err := dh.Client.Reboot(ctx, new(oop.Empty)); err != nil {
			return olterrors.NewErrAdapter("olt-reboot-failed", log.Fields{"device-id": dh.deviceID}, err).Log()
		}
	}
	cloned := proto.Clone(device).(*voltha.Device)
	cloned.OperStatus = voltha.OperStatus_UNKNOWN
	cloned.ConnectStatus = voltha.ConnectStatus_UNREACHABLE
	if err := dh.coreProxy.DeviceStateUpdate(ctx, cloned.Id, cloned.ConnectStatus, cloned.OperStatus); err != nil {
		return olterrors.NewErrAdapter("device-state-update-failed", log.Fields{
			"device-id":      device.Id,
			"connect-status": cloned.ConnectStatus,
			"oper-status":    cloned.OperStatus}, err).Log()
	}
	return nil
}
func (dh *DeviceHandler) cleanupDeviceResources(ctx context.Context) error {
	if dh.resourceMgr != nil {
		noOfPonPorts := dh.resourceMgr.DevInfo.GetPonPorts()
		var ponPort uint32
		for ponPort = 0; ponPort < noOfPonPorts; ponPort++ {
			var onuGemData []rsrcMgr.OnuGemInfo
			err := dh.resourceMgr.ResourceMgrs[ponPort].GetOnuGemInfo(ctx, ponPort, &onuGemData)
			if err != nil {
				return olterrors.NewErrNotFound("onu", log.Fields{
					"device-id": dh.device.Id,
					"pon-port":  ponPort}, err)
			}
			for _, onu := range onuGemData {
				onuID := make([]uint32, 1)
				logger.Debugw("onu data ", log.Fields{"onu": onu})
				if err = dh.clearUNIData(ctx, &onu); err != nil {
					logger.Errorw("Failed to clear data for onu", log.Fields{"onu-device": onu})
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
				logger.Errorw("Failed to update onugem info", log.Fields{"intfid": ponPort, "onugeminfo": onuGemData})
			}
		}
		/* Clear the flows from KV store associated with NNI port.
		   There are mostly trap rules from NNI port (like LLDP)
		*/
		if err := dh.clearNNIData(ctx); err != nil {
			logger.Errorw("Failed to clear data for NNI port", log.Fields{"device-id": dh.deviceID})
		}

		/* Clear the resource pool for each PON port in the background */
		go dh.resourceMgr.Delete(ctx)
	}

	/*Delete ONU map for the device*/
	dh.onus.Range(func(key interface{}, value interface{}) bool {
		dh.onus.Delete(key)
		return true
	})

	/*Delete discovered ONU map for the device*/
	dh.discOnus.Range(func(key interface{}, value interface{}) bool {
		dh.discOnus.Delete(key)
		return true
	})

	return nil
}

//RebootDevice reboots the given device
func (dh *DeviceHandler) RebootDevice(device *voltha.Device) error {
	if _, err := dh.Client.Reboot(context.Background(), new(oop.Empty)); err != nil {
		return olterrors.NewErrAdapter("olt-reboot-failed", log.Fields{"device-id": dh.deviceID}, err)
	}
	logger.Debugw("rebooted-device-successfully", log.Fields{"deviceID": device.Id})
	return nil
}

func (dh *DeviceHandler) handlePacketIndication(ctx context.Context, packetIn *oop.PacketIndication) error {
	logger.Debugw("Received packet-in", log.Fields{
		"deviceId":          dh.device.Id,
		"packet-indication": *packetIn,
		"device-id": dh.device.Id,
		"packet":            hex.EncodeToString(packetIn.Pkt),
	})
	logicalPortNum, err := dh.flowMgr.GetLogicalPortFromPacketIn(ctx, packetIn)
	if err != nil {
		return olterrors.NewErrNotFound("logical-port", log.Fields{"packet": hex.EncodeToString(packetIn.Pkt)}, err)
	}
	logger.Debugw("sending packet-in to core", log.Fields{
		"logicalPortNum": logicalPortNum,
		"device-id": dh.device.Id,
		"packet":         hex.EncodeToString(packetIn.Pkt),
	})
	if err := dh.coreProxy.SendPacketIn(context.TODO(), dh.device.Id, logicalPortNum, packetIn.Pkt); err != nil {
		return olterrors.NewErrCommunication("send-packet-in", log.Fields{
			"destination": "core",
			"source":      dh.deviceType,
			"device-id": dh.device.Id,
			"packet":      hex.EncodeToString(packetIn.Pkt),
		}, err)
	}
	logger.Debugw("Success sending packet-in to core!", log.Fields{
		"packet": hex.EncodeToString(packetIn.Pkt),
		"device-id": dh.device.Id,
	})
	return nil
}

// PacketOut sends packet-out from VOLTHA to OLT on the egress port provided
func (dh *DeviceHandler) PacketOut(ctx context.Context, egressPortNo int, packet *of.OfpPacketOut) error {
	logger.Debugw("incoming-packet-out", log.Fields{
		"deviceID":       dh.deviceID,
		"egress_port_no": egressPortNo,
		"pkt-length":     len(packet.Data),
		"device-id": dh.device.Id,
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
			logger.Debugw("dropping-lldp-packet-out-on-uni", log.Fields{
				"device-id": dh.device.Id,
			})
			return nil
		}
		if outerEthType == 0x88a8 || outerEthType == 0x8100 {
			if innerEthType == 0x8100 {
				// q-in-q 802.1ad or 802.1q double tagged packet.
				// slice out the outer tag.
				packet.Data = append(packet.Data[:12], packet.Data[16:]...)
				logger.Debugw("packet-now-single-tagged", log.Fields{
					"packetData": hex.EncodeToString(packet.Data),
					"device-id": dh.device.Id,
				})
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
			logger.Errorw("failed-to-retrieve-gemport-id-for-packet-out", log.Fields{
				"packet": hex.EncodeToString(packet.Data),
				"device-id": dh.device.Id,
			})
		}

		onuPkt := oop.OnuPacket{IntfId: intfID, OnuId: onuID, PortNo: uint32(egressPortNo), GemportId: gemPortID, Pkt: packet.Data}

		logger.Debugw("sending-packet-to-onu", log.Fields{
			"egress_port_no": egressPortNo,
			"IntfId":         intfID,
			"onuID":          onuID,
			"uniID":          uniID,
			"gemPortID":      gemPortID,
			"packet":         hex.EncodeToString(packet.Data),
			"device-id": dh.device.Id,
		})

		if _, err := dh.Client.OnuPacketOut(ctx, &onuPkt); err != nil {
			return olterrors.NewErrCommunication("packet-out-send", log.Fields{
				"source":             "adapter",
				"destination":        "onu",
				"egress-port-number": egressPortNo,
				"interface-id":       intfID,
				"oni-id":             onuID,
				"uni-id":             uniID,
				"gem-port-id":        gemPortID,
				"packet":             hex.EncodeToString(packet.Data),
				"device-id": dh.device.Id,
			}, err)
		}
	} else if egressPortType == voltha.Port_ETHERNET_NNI {
		nniIntfID, err := IntfIDFromNniPortNum(uint32(egressPortNo))
		if err != nil {
			return olterrors.NewErrInvalidValue(log.Fields{
				"egress-nni-port": egressPortNo,
				"device-id": dh.device.Id,
			}, err)
		}
		uplinkPkt := oop.UplinkPacket{IntfId: nniIntfID, Pkt: packet.Data}

		logger.Debugw("sending-packet-to-nni", log.Fields{
			"uplink_pkt": uplinkPkt,
			"packet":     hex.EncodeToString(packet.Data),
			"device-id": dh.device.Id,
		})

		if _, err := dh.Client.UplinkPacketOut(ctx, &uplinkPkt); err != nil {
			return olterrors.NewErrCommunication("packet-out-to-nni", log.Fields{
				"packet": hex.EncodeToString(packet.Data),
				"device-id": dh.device.Id,
			}, err)
		}
	} else {
		logger.Warnw("Packet-out-to-this-interface-type-not-implemented", log.Fields{
			"egress_port_no": egressPortNo,
			"egressPortType": egressPortType,
			"packet":         hex.EncodeToString(packet.Data),
			"device-id": dh.device.Id,
		})
	}
	return nil
}

func (dh *DeviceHandler) formOnuKey(intfID, onuID uint32) string {
	return "" + strconv.Itoa(int(intfID)) + "." + strconv.Itoa(int(onuID))
}

func startHeartbeatCheck(ctx context.Context, dh *DeviceHandler) {
	// start the heartbeat check towards the OLT.
	var timerCheck *time.Timer

	for {
		heartbeatTimer := time.NewTimer(dh.openOLT.HeartbeatCheckInterval)
		select {
		case <-heartbeatTimer.C:
			ctxWithTimeout, cancel := context.WithTimeout(context.Background(), dh.openOLT.GrpcTimeoutInterval)
			if heartBeat, err := dh.Client.HeartbeatCheck(ctxWithTimeout, new(oop.Empty)); err != nil {
				logger.Error("Hearbeat failed")
				if timerCheck == nil {
					// start a after func, when expired will update the state to the core
					timerCheck = time.AfterFunc(dh.openOLT.HeartbeatFailReportInterval, func() { dh.updateStateUnreachable(ctx) })
				}
			} else {
				if timerCheck != nil {
					if timerCheck.Stop() {
						logger.Debug("We got hearbeat within the timeout")
					}
					timerCheck = nil
				}
				logger.Debugw("Hearbeat", log.Fields{"signature": heartBeat})
			}
			cancel()
		case <-dh.stopHeartbeatCheck:
			logger.Debug("Stopping heart beat check")
			return
		}
	}
}

func (dh *DeviceHandler) updateStateUnreachable(ctx context.Context) {
	device, err := dh.coreProxy.GetDevice(ctx, dh.device.Id, dh.device.Id)
	if err != nil || device == nil {
		olterrors.NewErrNotFound("device", log.Fields{"device-id": dh.device.Id}, err).Log()
	}

	if device.ConnectStatus == voltha.ConnectStatus_REACHABLE {
		if err = dh.coreProxy.DeviceStateUpdate(ctx, dh.device.Id, voltha.ConnectStatus_UNREACHABLE, voltha.OperStatus_UNKNOWN); err != nil {
			olterrors.NewErrAdapter("device-state-update-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}
		if err = dh.coreProxy.PortsStateUpdate(ctx, dh.device.Id, voltha.OperStatus_UNKNOWN); err != nil {
			olterrors.NewErrAdapter("port-update-failed", log.Fields{"device-id": dh.device.Id}, err).Log()
		}
		go dh.cleanupDeviceResources(ctx)

		dh.lockDevice.RLock()
		// Stop the read indication only if it the routine is active
		// The read indication would have already stopped due to failure on the gRPC stream following OLT going unreachable
		// Sending message on the 'stopIndication' channel again will cause the readIndication routine to immediately stop
		// on next execution of the readIndication routine.
		if dh.isReadIndicationRoutineActive {
			dh.stopIndications <- true
		}
		dh.lockDevice.RUnlock()

		dh.transitionMap.Handle(ctx, DeviceInit)

	}
}

// EnablePort to enable Pon interface
func (dh *DeviceHandler) EnablePort(port *voltha.Port) error {
	logger.Debugw("enable-port", log.Fields{"Device": dh.device, "port": port})
	return dh.modifyPhyPort(port, true)
}

// DisablePort to disable pon interface
func (dh *DeviceHandler) DisablePort(port *voltha.Port) error {
	logger.Debugw("disable-port", log.Fields{"Device": dh.device, "port": port})
	return dh.modifyPhyPort(port, false)
}

//modifyPhyPort is common function to enable and disable the port. parm :enablePort, true to enablePort and false to disablePort.
func (dh *DeviceHandler) modifyPhyPort(port *voltha.Port, enablePort bool) error {
	ctx := context.Background()
	logger.Infow("modifyPhyPort", log.Fields{"port": port, "Enable": enablePort})
	if port.GetType() == voltha.Port_ETHERNET_NNI {
		// Bug is opened for VOL-2505 to support NNI disable feature.
		logger.Infow("voltha-supports-single-nni-hence-disable-of-nni-not-allowed",
			log.Fields{"Device": dh.device, "port": port})
		return olterrors.NewErrAdapter("illegal-port-request", log.Fields{
			"port-type":    port.GetType,
			"enable-state": enablePort}, nil)
	}
	// fetch interfaceid from PortNo
	ponID := PortNoToIntfID(port.GetPortNo(), voltha.Port_PON_OLT)
	ponIntf := &oop.Interface{IntfId: ponID}
	var operStatus voltha.OperStatus_Types
	if enablePort {
		operStatus = voltha.OperStatus_ACTIVE
		out, err := dh.Client.EnablePonIf(ctx, ponIntf)

		if err != nil {
			return olterrors.NewErrAdapter("pon-port-enable-failed", log.Fields{
				"device-id": dh.device.Id,
				"port":      port}, err)
		}
		// updating interface local cache for collecting stats
		dh.activePorts.Store(ponID, true)
		logger.Infow("enabled-pon-port", log.Fields{"out": out, "DeviceID": dh.device, "Port": port})
	} else {
		operStatus = voltha.OperStatus_UNKNOWN
		out, err := dh.Client.DisablePonIf(ctx, ponIntf)
		if err != nil {
			return olterrors.NewErrAdapter("pon-port-disable-failed", log.Fields{
				"device-id": dh.device.Id,
				"port":      port}, err)
		}
		// updating interface local cache for collecting stats
		dh.activePorts.Store(ponID, false)
		logger.Infow("disabled-pon-port", log.Fields{"out": out, "DeviceID": dh.device, "Port": port})
	}
	if err := dh.coreProxy.PortStateUpdate(ctx, dh.deviceID, voltha.Port_PON_OLT, port.PortNo, operStatus); err != nil {
		return olterrors.NewErrAdapter("port-state-update-failed", log.Fields{
			"device-id": dh.deviceID,
			"port":      port.PortNo}, err)
	}
	return nil
}

//disableAdminDownPorts disables the ports, if the corresponding port Adminstate is disabled on reboot and Renable device.
func (dh *DeviceHandler) disableAdminDownPorts(device *voltha.Device) error {
	cloned := proto.Clone(device).(*voltha.Device)
	// Disable the port and update the oper_port_status to core
	// if the Admin state of the port is disabled on reboot and re-enable device.
	for _, port := range cloned.Ports {
		if port.AdminState == common.AdminState_DISABLED {
			if err := dh.DisablePort(port); err != nil {
				return olterrors.NewErrAdapter("port-disable-failed", log.Fields{
					"device-id": dh.deviceID,
					"port":      port}, err)
			}
		}
	}
	return nil
}

//populateActivePorts to populate activePorts map
func (dh *DeviceHandler) populateActivePorts(device *voltha.Device) {
	logger.Info("populateActiveports", log.Fields{"Device": device})
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

// ChildDeviceLost deletes ONU and clears pon resources related to it.
func (dh *DeviceHandler) ChildDeviceLost(ctx context.Context, pPortNo uint32, onuID uint32) error {
	logger.Debugw("child-device-lost", log.Fields{"pdeviceID": dh.device.Id})
	IntfID := PortNoToIntfID(pPortNo, voltha.Port_PON_OLT)
	onuKey := dh.formOnuKey(IntfID, onuID)
	onuDevice, ok := dh.onus.Load(onuKey)
	if !ok {
		return olterrors.NewErrAdapter("failed-to-load-onu-details",
			log.Fields{
				"device-id":    dh.deviceID,
				"onu-id":       onuID,
				"interface-id": IntfID}, nil).Log()
	}
	var sn *oop.SerialNumber
	var err error
	if sn, err = dh.deStringifySerialNumber(onuDevice.(*OnuDevice).serialNumber); err != nil {
		return olterrors.NewErrAdapter("failed-to-destringify-serial-number",
			log.Fields{
				"devicer-id":    dh.deviceID,
				"serial-number": onuDevice.(*OnuDevice).serialNumber}, err).Log()
	}
	onu := &oop.Onu{IntfId: IntfID, OnuId: onuID, SerialNumber: sn}
	if _, err := dh.Client.DeleteOnu(context.Background(), onu); err != nil {
		return olterrors.NewErrAdapter("failed-to-delete-onu", log.Fields{
			"device-id": dh.deviceID,
			"onu-id":    onuID}, err).Log()
	}
	//clear PON resources associated with ONU
	var onuGemData []rsrcMgr.OnuGemInfo
	if onuMgr, ok := dh.resourceMgr.ResourceMgrs[IntfID]; !ok {
		logger.Warnw("failed-to-get-resource-manager-for-interface-Id", log.Fields{
			"device-id":    dh.deviceID,
			"interface-id": IntfID})
	} else {
		if err := onuMgr.GetOnuGemInfo(ctx, IntfID, &onuGemData); err != nil {
			logger.Warnw("failed-to-get-onu-info-for-pon-port ", log.Fields{
				"device-id":    dh.deviceID,
				"interface-id": IntfID,
				"error":        err})
		} else {
			for i, onu := range onuGemData {
				if onu.OnuID == onuID && onu.SerialNumber == onuDevice.(*OnuDevice).serialNumber {
					logger.Debugw("onu-data ", log.Fields{"onu": onu})
					if err := dh.clearUNIData(ctx, &onu); err != nil {
						logger.Warnw("failed-to-clear-uni-data-for-onu", log.Fields{
							"device-id":  dh.deviceID,
							"onu-device": onu,
							"error":      err})
					}
					// Clear flowids for gem cache.
					for _, gem := range onu.GemPorts {
						dh.resourceMgr.DeleteFlowIDsForGem(ctx, IntfID, gem)
					}
					onuGemData = append(onuGemData[:i], onuGemData[i+1:]...)
					err := onuMgr.AddOnuGemInfo(ctx, IntfID, onuGemData)
					if err != nil {
						logger.Warnw("persistence-update-onu-gem-info-failed", log.Fields{
							"interface-id": IntfID,
							"onu-device":   onu,
							"onu-gem":      onuGemData,
							"error":        err})
						//Not returning error on cleanup.
					}
					logger.Debugw("removed-onu-gem-info", log.Fields{"intf": IntfID, "onu-device": onu, "onugem": onuGemData})
					dh.resourceMgr.FreeonuID(ctx, IntfID, []uint32{onu.OnuID})
					break
				}
			}
		}
	}
	dh.onus.Delete(onuKey)
	dh.discOnus.Delete(onuDevice.(*OnuDevice).serialNumber)
	return nil
}

func getInPortFromFlow(flow *of.OfpFlowStats) uint32 {
	for _, field := range flows.GetOfbFields(flow) {
		if field.Type == flows.IN_PORT {
			return field.GetPort()
		}
	}
	return InvalidPort
}

func getOutPortFromFlow(flow *of.OfpFlowStats) uint32 {
	for _, action := range flows.GetActions(flow) {
		if action.Type == flows.OUTPUT {
			if out := action.GetOutput(); out != nil {
				return out.GetPort()
			}
		}
	}
	return InvalidPort
}

func (dh *DeviceHandler) incrementActiveFlowRemoveCount(flow *of.OfpFlowStats) {
	inPort, outPort := getPorts(flow)
	logger.Debugw("increment flow remove count for inPort outPort", log.Fields{"inPort": inPort, "outPort": outPort})
	if inPort != InvalidPort && outPort != InvalidPort {
		_, intfID, onuID, uniID := ExtractAccessFromFlow(inPort, outPort)
		key := pendingFlowRemoveDataKey{intfID: intfID, onuID: onuID, uniID: uniID}
		logger.Debugw("increment flow remove count for subscriber", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID})

		dh.lockDevice.Lock()
		defer dh.lockDevice.Unlock()
		flowRemoveData, ok := dh.pendingFlowRemoveDataPerSubscriber[key]
		if !ok {
			flowRemoveData = pendingFlowRemoveData{
				pendingFlowRemoveCount: 0,
				allFlowsRemoved:        make(chan struct{}),
			}
		}
		flowRemoveData.pendingFlowRemoveCount++
		dh.pendingFlowRemoveDataPerSubscriber[key] = flowRemoveData

		logger.Debugw("current flow remove count after increment",
			log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID,
				"currCnt": dh.pendingFlowRemoveDataPerSubscriber[key].pendingFlowRemoveCount})
	}
}

func (dh *DeviceHandler) decrementActiveFlowRemoveCount(flow *of.OfpFlowStats) {
	inPort, outPort := getPorts(flow)
	logger.Debugw("decrement flow remove count for inPort outPort", log.Fields{"inPort": inPort, "outPort": outPort})
	if inPort != InvalidPort && outPort != InvalidPort {
		_, intfID, onuID, uniID := ExtractAccessFromFlow(uint32(inPort), uint32(outPort))
		key := pendingFlowRemoveDataKey{intfID: intfID, onuID: onuID, uniID: uniID}
		logger.Debugw("decrement flow remove count for subscriber", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID})

		dh.lockDevice.Lock()
		defer dh.lockDevice.Unlock()
		if val, ok := dh.pendingFlowRemoveDataPerSubscriber[key]; !ok {
			logger.Fatalf("flow remove key not found", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID})
		} else {
			if val.pendingFlowRemoveCount > 0 {
				val.pendingFlowRemoveCount--
			}
			logger.Debugw("current flow remove count after decrement",
				log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID,
					"currCnt": dh.pendingFlowRemoveDataPerSubscriber[key].pendingFlowRemoveCount})
			// If all flow removes have finished, then close the channel to signal the receiver
			// to go ahead with flow adds.
			if val.pendingFlowRemoveCount == 0 {
				close(val.allFlowsRemoved)
				delete(dh.pendingFlowRemoveDataPerSubscriber, key)
				return
			}
			dh.pendingFlowRemoveDataPerSubscriber[key] = val
		}
	}
}

func (dh *DeviceHandler) waitForFlowRemoveToFinish(flow *of.OfpFlowStats) {
	var flowRemoveData pendingFlowRemoveData
	var ok bool
	inPort, outPort := getPorts(flow)
	logger.Debugw("wait for flow remove to finish for inPort outPort", log.Fields{"inPort": inPort, "outPort": outPort})
	if inPort != InvalidPort && outPort != InvalidPort {
		_, intfID, onuID, uniID := ExtractAccessFromFlow(inPort, outPort)
		key := pendingFlowRemoveDataKey{intfID: intfID, onuID: onuID, uniID: uniID}
		logger.Debugw("wait for flow remove to finish for subscriber", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID})

		dh.lockDevice.RLock()
		if flowRemoveData, ok = dh.pendingFlowRemoveDataPerSubscriber[key]; !ok {
			logger.Debugw("no pending flow to remove", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID})
			dh.lockDevice.RUnlock()
			return
		}
		dh.lockDevice.RUnlock()

		// Wait for all flow removes to finish first
		<-flowRemoveData.allFlowsRemoved

		logger.Debugw("all flows cleared, handling flow add now", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID})
	}
}

func getPorts(flow *of.OfpFlowStats) (uint32, uint32) {
	inPort := getInPortFromFlow(flow)
	outPort := getOutPortFromFlow(flow)

	if inPort == InvalidPort || outPort == InvalidPort {
		return inPort, outPort
	}

	if isControllerFlow := IsControllerBoundFlow(outPort); isControllerFlow {
		/* Get UNI port/ IN Port from tunnel ID field for upstream controller bound flows  */
		if portType := IntfIDToPortTypeName(inPort); portType == voltha.Port_PON_OLT {
			if uniPort := flows.GetChildPortFromTunnelId(flow); uniPort != 0 {
				return uniPort, outPort
			}
		}
	} else {
		// Downstream flow from NNI to PON port , Use tunnel ID as new OUT port / UNI port
		if portType := IntfIDToPortTypeName(outPort); portType == voltha.Port_PON_OLT {
			if uniPort := flows.GetChildPortFromTunnelId(flow); uniPort != 0 {
				return inPort, uniPort
			}
			// Upstream flow from PON to NNI port , Use tunnel ID as new IN port / UNI port
		} else if portType := IntfIDToPortTypeName(inPort); portType == voltha.Port_PON_OLT {
			if uniPort := flows.GetChildPortFromTunnelId(flow); uniPort != 0 {
				return uniPort, outPort
			}
		}
	}

	return InvalidPort, InvalidPort
}

func extractOmciTransactionID(omciPkt []byte) uint16 {
	if len(omciPkt) > 3 {
		d := omciPkt[0:2]
		transid := binary.BigEndian.Uint16(d)
		return transid
	}
	return 0
}

// StoreOnuDevice stores the onu parameters to the local cache.
func (dh *DeviceHandler) StoreOnuDevice(onuDevice *OnuDevice) {
	onuKey := dh.formOnuKey(onuDevice.intfID, onuDevice.onuID)
	dh.onus.Store(onuKey, onuDevice)
}
