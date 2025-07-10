/*
 * Copyright 2018-2024 Open Networking Foundation (ONF) and the ONF Contributors

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

// Package core provides the utility for olt devices, flows and statistics
package core

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencord/voltha-lib-go/v7/pkg/db"
	"github.com/opencord/voltha-lib-go/v7/pkg/db/kvstore"

	vgrpc "github.com/opencord/voltha-lib-go/v7/pkg/grpc"

	"github.com/cenkalti/backoff/v3"
	"github.com/gogo/protobuf/proto"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	"github.com/opencord/voltha-lib-go/v7/pkg/config"
	"github.com/opencord/voltha-lib-go/v7/pkg/events/eventif"
	flow_utils "github.com/opencord/voltha-lib-go/v7/pkg/flows"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	plt "github.com/opencord/voltha-lib-go/v7/pkg/platform"
	"github.com/opencord/voltha-lib-go/v7/pkg/pmmetrics"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	conf "github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"
	"github.com/opencord/voltha-protos/v5/go/common"
	ca "github.com/opencord/voltha-protos/v5/go/core_adapter"
	"github.com/opencord/voltha-protos/v5/go/extension"
	ia "github.com/opencord/voltha-protos/v5/go/inter_adapter"
	"github.com/opencord/voltha-protos/v5/go/onu_inter_adapter_service"
	of "github.com/opencord/voltha-protos/v5/go/openflow_13"
	oop "github.com/opencord/voltha-protos/v5/go/openolt"
	tp_pb "github.com/opencord/voltha-protos/v5/go/tech_profile"
	"github.com/opencord/voltha-protos/v5/go/voltha"
	"google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Constants for number of retries and for timeout
const (
	InvalidPort                  = 0xffffffff
	MaxNumOfGroupHandlerChannels = 256

	McastFlowOrGroupAdd    = "McastFlowOrGroupAdd"
	McastFlowOrGroupModify = "McastFlowOrGroupModify"
	McastFlowOrGroupRemove = "McastFlowOrGroupRemove"
	oltPortInfoTimeout     = 3

	defaultPortSpeedMbps = 1000
	heartbeatPath        = "heartbeat"
)

// DeviceHandler will interact with the OLT device.
type DeviceHandler struct {
	EventProxy          eventif.EventProxy
	Client              oop.OpenoltClient
	cm                  *config.ConfigManager
	device              *voltha.Device
	cfg                 *conf.AdapterFlags
	coreClient          *vgrpc.Client
	childAdapterClients map[string]*vgrpc.Client
	openOLT             *OpenOLT
	exitChannel         chan struct{}
	transitionMap       *TransitionMap
	clientCon           *grpc.ClientConn
	groupMgr            *OpenOltGroupMgr
	eventMgr            *OpenOltEventMgr
	kvStore             *db.Backend // backend kv store connection handle

	deviceInfo *oop.DeviceInfo

	portStats                  *OpenOltStatisticsMgr
	metrics                    *pmmetrics.PmMetrics
	stopCollector              chan bool
	stopHeartbeatCheck         chan bool
	stopIndications            chan bool
	perPonOnuIndicationChannel map[uint32]onuIndicationChannels

	// discOnus (map[onuSn]bool) contains a list of ONUs that have been discovered, indexed by ONU SerialNumber.
	// if the value is true that means the OnuDiscovery indication
	// is currently being processed and thus we can ignore concurrent requests
	// if it's false it means the processing has completed and we shouldn't be receiving a new indication
	// if we do it means something went wrong and we need to retry
	discOnus    sync.Map
	onus        sync.Map
	activePorts sync.Map
	flowMgr     []*OpenOltFlowMgr
	resourceMgr []*rsrcMgr.OpenOltResourceMgr

	// Slice of channels. Each channel in slice, index by (mcast-group-id modulo MaxNumOfGroupHandlerChannels)
	// A go routine per index, waits on a unique channel for incoming mcast flow or group (add/modify/remove).
	incomingMcastFlowOrGroup  []chan McastFlowOrGroupControlBlock
	stopMcastHandlerRoutine   []chan bool
	mcastHandlerRoutineActive []bool

	lockChildAdapterClients        sync.RWMutex
	lockDevice                     sync.RWMutex
	perPonOnuIndicationChannelLock sync.Mutex

	totalPonPorts                 uint32
	heartbeatSignature            uint32
	isCollectorActive             bool
	isHeartbeatCheckActive        bool
	isReadIndicationRoutineActive bool

	adapterPreviouslyConnected bool
	agentPreviouslyConnected   bool

	isDeviceDeletionInProgress bool
	prevOperStatus             common.OperStatus_Types
}

// OnuDevice represents ONU related info
type OnuDevice struct {
	deviceID        string
	deviceType      string
	serialNumber    string
	proxyDeviceID   string
	adapterEndpoint string
	onuID           uint32
	intfID          uint32
	losRaised       bool
	rdiRaised       bool
}

type onuIndicationMsg struct {
	ctx        context.Context
	indication *oop.Indication
}

type onuIndicationChannels struct {
	indicationChannel chan onuIndicationMsg
	stopChannel       chan struct{}
}

// McastFlowOrGroupControlBlock is created per mcast flow/group add/modify/remove and pushed on the incomingMcastFlowOrGroup channel slice
// The McastFlowOrGroupControlBlock is then picked by the mcastFlowOrGroupChannelHandlerRoutine for further processing.
// There are MaxNumOfGroupHandlerChannels number of mcastFlowOrGroupChannelHandlerRoutine routines which monitor for any incoming mcast flow/group messages
// and process them serially. The mcast flow/group are assigned these routines based on formula (group-id modulo MaxNumOfGroupHandlerChannels)
type McastFlowOrGroupControlBlock struct {
	ctx               context.Context   // Flow/group handler context
	flow              *of.OfpFlowStats  // Flow message (can be nil or valid flow)
	group             *of.OfpGroupEntry // Group message (can be nil or valid group)
	errChan           *chan error       // channel to report the mcast Flow/group handling error
	flowOrGroupAction string            // one of McastFlowOrGroupAdd, McastFlowOrGroupModify or McastFlowOrGroupDelete
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

// NewOnuDevice creates a new Onu Device
func NewOnuDevice(devID, deviceTp, serialNum string, onuID, intfID uint32, proxyDevID string, losRaised bool, adapterEndpoint string) *OnuDevice {
	var device OnuDevice
	device.deviceID = devID
	device.deviceType = deviceTp
	device.serialNumber = serialNum
	device.onuID = onuID
	device.intfID = intfID
	device.proxyDeviceID = proxyDevID
	device.losRaised = losRaised
	device.adapterEndpoint = adapterEndpoint
	return &device
}

// NewDeviceHandler creates a new device handler
func NewDeviceHandler(cc *vgrpc.Client, ep eventif.EventProxy, device *voltha.Device, adapter *OpenOLT, cm *config.ConfigManager, cfg *conf.AdapterFlags) *DeviceHandler {
	var dh DeviceHandler
	ctx := context.Background()
	dh.cm = cm
	dh.coreClient = cc
	dh.EventProxy = ep
	cloned := (proto.Clone(device)).(*voltha.Device)
	dh.device = cloned
	dh.openOLT = adapter
	dh.exitChannel = make(chan struct{})
	dh.lockDevice = sync.RWMutex{}
	dh.stopCollector = make(chan bool, 1)      // TODO: Why buffered?
	dh.stopHeartbeatCheck = make(chan bool, 1) // TODO: Why buffered?
	dh.metrics = pmmetrics.NewPmMetrics(cloned.Id, pmmetrics.Frequency(150), pmmetrics.FrequencyOverride(false), pmmetrics.Grouped(false), pmmetrics.Metrics(pmNames))
	dh.activePorts = sync.Map{}
	dh.stopIndications = make(chan bool, 1) // TODO: Why buffered?
	dh.perPonOnuIndicationChannel = make(map[uint32]onuIndicationChannels)
	dh.childAdapterClients = make(map[string]*vgrpc.Client)
	dh.cfg = cfg
	dh.kvStore = SetKVClient(ctx, dh.openOLT.KVStoreType, dh.openOLT.KVStoreAddress, dh.device.Id, dh.cm.Backend.PathPrefix)
	if dh.kvStore == nil {
		logger.Error(ctx, "Failed to setup KV store")
		return nil
	}

	// Create a slice of buffered channels for handling concurrent mcast flow/group.
	dh.incomingMcastFlowOrGroup = make([]chan McastFlowOrGroupControlBlock, MaxNumOfGroupHandlerChannels)
	dh.stopMcastHandlerRoutine = make([]chan bool, MaxNumOfGroupHandlerChannels)
	dh.mcastHandlerRoutineActive = make([]bool, MaxNumOfGroupHandlerChannels)
	for i := range dh.incomingMcastFlowOrGroup {
		dh.incomingMcastFlowOrGroup[i] = make(chan McastFlowOrGroupControlBlock, MaxNumOfGroupHandlerChannels)
		dh.stopMcastHandlerRoutine[i] = make(chan bool)
		// Spin up a go routine to handling incoming mcast flow/group (add/modify/remove).
		// There will be MaxNumOfGroupHandlerChannels number of mcastFlowOrGroupChannelHandlerRoutine go routines.
		// These routines will be blocked on the dh.incomingMcastFlowOrGroup[mcast-group-id modulo MaxNumOfGroupHandlerChannels] channel
		// for incoming mcast flow/group to be processed serially.
		dh.mcastHandlerRoutineActive[i] = true
		go dh.mcastFlowOrGroupChannelHandlerRoutine(i, dh.incomingMcastFlowOrGroup[i], dh.stopMcastHandlerRoutine[i])
	}
	// TODO initialize the support classes.
	return &dh
}

func newKVClient(ctx context.Context, storeType string, address string, timeout time.Duration) (kvstore.Client, error) {
	logger.Infow(ctx, "kv-store-type", log.Fields{"store": storeType})
	switch storeType {
	case "etcd":
		return kvstore.NewEtcdClient(ctx, address, timeout, log.FatalLevel)
	case "redis":
		return kvstore.NewRedisClient(address, timeout, false)
	case "redis-sentinel":
		return kvstore.NewRedisClient(address, timeout, true)
	}
	return nil, errors.New("unsupported-kv-store")
}

// SetKVClient sets the KV client and return a kv backend
func SetKVClient(ctx context.Context, backend string, addr string, DeviceID string, basePathKvStore string) *db.Backend {
	kvClient, err := newKVClient(ctx, backend, addr, rsrcMgr.KvstoreTimeout)
	if err != nil {
		logger.Fatalw(ctx, "Failed to init KV client\n", log.Fields{"err": err})
		return nil
	}
	kvbackend := &db.Backend{
		Client:     kvClient,
		StoreType:  backend,
		Address:    addr,
		Timeout:    rsrcMgr.KvstoreTimeout,
		PathPrefix: fmt.Sprintf(rsrcMgr.BasePathKvStore, basePathKvStore, DeviceID)}

	return kvbackend
}

// CloseKVClient closes open KV clients
func (dh *DeviceHandler) CloseKVClient(ctx context.Context) {
	if dh.resourceMgr != nil {
		for _, rscMgr := range dh.resourceMgr {
			if rscMgr != nil {
				rscMgr.CloseKVClient(ctx)
			}
		}
	}
	if dh.flowMgr != nil {
		for _, flMgr := range dh.flowMgr {
			if flMgr != nil {
				flMgr.CloseKVClient(ctx)
			}
		}
	}
}

// start save the device to the data model
func (dh *DeviceHandler) start(ctx context.Context) {
	dh.lockDevice.Lock()
	defer dh.lockDevice.Unlock()
	logger.Debugw(ctx, "starting-device-agent", log.Fields{"device": dh.device})
	// Add the initial device to the local model
	logger.Debug(ctx, "device-agent-started")
}

// Stop stops the device handler
func (dh *DeviceHandler) Stop(ctx context.Context) {
	dh.lockDevice.Lock()
	defer dh.lockDevice.Unlock()
	logger.Debug(ctx, "stopping-device-agent")
	close(dh.exitChannel)

	// Delete (which will stop also) all grpc connections to the child adapters
	dh.deleteAdapterClients(ctx)
	logger.Debug(ctx, "device-agent-stopped")
}

func (dh *DeviceHandler) getPonTechnology(intfID uint32) string {
	for _, resourceRanges := range dh.deviceInfo.GetRanges() {
		for _, pooledIntfID := range resourceRanges.GetIntfIds() {
			if pooledIntfID == intfID {
				return resourceRanges.GetTechnology()
			}
		}
	}
	return ""
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

func generateMacFromHost(ctx context.Context, host string) (string, error) {
	var genmac string
	var addr net.IP
	var ips []string
	var err error

	logger.Debugw(ctx, "generating-mac-from-host", log.Fields{"host": host})

	if addr = net.ParseIP(host); addr == nil {
		logger.Debugw(ctx, "looking-up-hostname", log.Fields{"host": host})

		if ips, err = net.LookupHost(host); err == nil {
			logger.Debugw(ctx, "dns-result-ips", log.Fields{"ips": ips})
			if addr = net.ParseIP(ips[0]); addr == nil {
				return "", olterrors.NewErrInvalidValue(log.Fields{"ip": ips[0]}, nil)
			}
			genmac = macifyIP(addr)
			logger.Debugw(ctx, "using-ip-as-mac",
				log.Fields{"host": ips[0],
					"mac": genmac})
			return genmac, nil
		}
		return "", olterrors.NewErrAdapter("cannot-resolve-hostname-to-ip", log.Fields{"host": host}, err)
	}

	genmac = macifyIP(addr)
	logger.Debugw(ctx, "using-ip-as-mac",
		log.Fields{"host": host,
			"mac": genmac})
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

// GetportLabel returns the label for the NNI and the PON port based on port number and port type
func GetportLabel(portNum uint32, portType voltha.Port_PortType) (string, error) {
	switch portType {
	case voltha.Port_ETHERNET_NNI:
		return fmt.Sprintf("nni-%d", portNum), nil
	case voltha.Port_PON_OLT:
		return fmt.Sprintf("pon-%d", portNum), nil
	}

	return "", olterrors.NewErrInvalidValue(log.Fields{"port-type": portType}, nil)
}

func makeOfpPort(macAddress string, speedMbps uint32) *of.OfpPort {
	if speedMbps == 0 {
		// In case it was not set in the indication
		// and no other value was provided
		speedMbps = defaultPortSpeedMbps
	}

	ofpPortSpeed := of.OfpPortFeatures_OFPPF_OTHER
	switch speedMbps {
	case 1000000:
		ofpPortSpeed = of.OfpPortFeatures_OFPPF_1TB_FD
	case 100000:
		ofpPortSpeed = of.OfpPortFeatures_OFPPF_100GB_FD
	case 40000:
		ofpPortSpeed = of.OfpPortFeatures_OFPPF_40GB_FD
	case 10000:
		ofpPortSpeed = of.OfpPortFeatures_OFPPF_10GB_FD
	case 1000:
		ofpPortSpeed = of.OfpPortFeatures_OFPPF_1GB_FD
	case 100:
		ofpPortSpeed = of.OfpPortFeatures_OFPPF_100MB_FD
	case 10:
		ofpPortSpeed = of.OfpPortFeatures_OFPPF_10MB_FD
	}

	capacity := uint32(ofpPortSpeed | of.OfpPortFeatures_OFPPF_FIBER)

	port := &of.OfpPort{
		HwAddr:     macAddressToUint32Array(macAddress),
		Config:     0,
		State:      uint32(of.OfpPortState_OFPPS_LIVE),
		Curr:       capacity,
		Advertised: capacity,
		Peer:       capacity,
		CurrSpeed:  speedMbps * 1000, // kbps
		MaxSpeed:   speedMbps * 1000, // kbps
	}

	return port
}

func (dh *DeviceHandler) addPort(ctx context.Context, intfID uint32, portType voltha.Port_PortType, state string, speedMbps uint32) error {
	var operStatus common.OperStatus_Types
	if state == "up" {
		operStatus = voltha.OperStatus_ACTIVE
		// populating the intfStatus map
		dh.activePorts.Store(intfID, true)
	} else {
		operStatus = voltha.OperStatus_DISCOVERED
		dh.activePorts.Store(intfID, false)
	}
	portNum := plt.IntfIDToPortNo(intfID, portType)
	label, err := GetportLabel(intfID, portType)
	if err != nil {
		return olterrors.NewErrNotFound("port-label", log.Fields{"port-number": portNum, "port-type": portType}, err)
	}

	// Check if port exists
	port, err := dh.getPortFromCore(ctx, &ca.PortFilter{
		DeviceId: dh.device.Id,
		Port:     portNum,
	})
	if err == nil && port.Type == portType {
		logger.Debug(ctx, "port-already-exists-updating-oper-status-of-port")
		err = dh.updatePortStateInCore(ctx, &ca.PortState{
			DeviceId:   dh.device.Id,
			PortType:   portType,
			PortNo:     portNum,
			OperStatus: operStatus})
		if err != nil {
			return olterrors.NewErrAdapter("failed-to-update-port-state", log.Fields{
				"device-id":   dh.device.Id,
				"port-type":   portType,
				"port-number": portNum,
				"oper-status": operStatus}, err).Log()
		}
		return nil
	}

	// Now create Port
	port = &voltha.Port{
		DeviceId:   dh.device.Id,
		PortNo:     portNum,
		Label:      label,
		Type:       portType,
		OperStatus: operStatus,
		OfpPort:    makeOfpPort(dh.device.MacAddress, speedMbps),
	}
	logger.Debugw(ctx, "sending-port-update-to-core", log.Fields{"port": port})
	// Synchronous call to update device - this method is run in its own go routine
	err = dh.createPortInCore(ctx, port)
	if err != nil {
		return olterrors.NewErrAdapter("error-creating-port", log.Fields{
			"device-id": dh.device.Id,
			"port-type": portType}, err)
	}
	go dh.updateLocalDevice(ctx)
	return nil
}

func (dh *DeviceHandler) updateLocalDevice(ctx context.Context) {
	device, err := dh.getDeviceFromCore(ctx, dh.device.Id)
	if err != nil || device == nil {
		logger.Errorf(ctx, "device-not-found", log.Fields{"device-id": dh.device.Id}, err)
		return
	}
	dh.lockDevice.Lock()
	defer dh.lockDevice.Unlock()
	dh.device = device
}

// nolint: gocyclo
// readIndications to read the indications from the OLT device
func (dh *DeviceHandler) readIndications(ctx context.Context) error {
	defer logger.Debugw(ctx, "indications-ended", log.Fields{"device-id": dh.device.Id})
	defer func() {
		dh.lockDevice.Lock()
		dh.isReadIndicationRoutineActive = false
		logger.Debugw(ctx, "isReadIndicationRoutineActive set to false", log.Fields{"device-id": dh.device.Id})
		dh.lockDevice.Unlock()
	}()
	indications, err := dh.startOpenOltIndicationStream(ctx)
	if err != nil {
		return err
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
	logger.Debugw(ctx, "isReadIndicationRoutineActive set to true", log.Fields{"device-id": dh.device.Id})
	dh.lockDevice.Unlock()

Loop:
	for {
		select {
		case <-dh.stopIndications:
			logger.Debugw(ctx, "stopping-collecting-indications-for-olt", log.Fields{"device-id": dh.device.Id})
			break Loop
		default:
			indication, err := indications.Recv()

			select {
			case <-indications.Context().Done():
				if err != nil {
					logger.Warnw(ctx, "error-during-enable-indications",
						log.Fields{"err": err,
							"device-id": dh.device.Id})
				}

				// Use an exponential back off to prevent getting into a tight loop
				duration := indicationBackoff.NextBackOff()
				logger.Infow(ctx, "backing-off-enable-indication", log.Fields{
					"device-id": dh.device.Id,
					"duration":  duration,
				})
				if duration == backoff.Stop {
					// If we reach a maximum then warn and reset the backoff
					// timer and keep attempting.
					logger.Warnw(ctx, "maximum-indication-backoff-reached-resetting-backoff-timer",
						log.Fields{"max-indication-backoff": indicationBackoff.MaxElapsedTime,
							"device-id": dh.device.Id})
					indicationBackoff.Reset()
				}

				// On failure process a backoff timer while watching for stopIndications
				// events
				backoffTimer := time.NewTimer(indicationBackoff.NextBackOff())
				select {
				case <-dh.stopIndications:
					logger.Debugw(ctx, "stopping-collecting-indications-for-olt", log.Fields{"device-id": dh.device.Id})
					if !backoffTimer.Stop() {
						<-backoffTimer.C
					}
					break Loop
				case <-backoffTimer.C:
					// backoffTimer expired continue
				}
				if indications, err = dh.startOpenOltIndicationStream(ctx); err != nil {
					return err
				}
				continue
			default:
				if err != nil {
					logger.Errorw(ctx, "read-indication-error",
						log.Fields{"err": err,
							"device-id": dh.device.Id})
					// Close the stream, and re-initialize it
					if err = indications.CloseSend(); err != nil {
						// Ok to ignore here, because we landed here due to a problem on the stream
						// In all probability, the closeSend call may fail
						logger.Debugw(ctx, "error-closing-send stream--error-ignored",
							log.Fields{"err": err,
								"device-id": dh.device.Id})
					}
					if indications, err = dh.startOpenOltIndicationStream(ctx); err != nil {
						return err
					}
					// once we re-initialized the indication stream, continue to read indications
					continue
				}
				// Reset backoff if we have a successful receive
				indicationBackoff.Reset()
				// When OLT is admin down, ignore all indications.
				if dh.device.AdminState == voltha.AdminState_DISABLED && !isIndicationAllowedDuringOltAdminDown(indication) {
					logger.Debugw(ctx, "olt-is-admin-down, ignore indication",
						log.Fields{"indication": indication,
							"device-id": dh.device.Id})
					continue
				}
				dh.handleIndication(ctx, indication)
			}
		}
	}
	// Close the send stream
	_ = indications.CloseSend() // Ok to ignore error, as we stopping the readIndication anyway

	return nil
}

func (dh *DeviceHandler) startOpenOltIndicationStream(ctx context.Context) (oop.Openolt_EnableIndicationClient, error) {
	logger.Infow(ctx, "enabling read indications", log.Fields{"device-id": dh.device.Id})
	indications, err := dh.Client.EnableIndication(ctx, new(oop.Empty))
	if err != nil {
		return nil, olterrors.NewErrCommunication("indication-read-failure", log.Fields{"device-id": dh.device.Id}, err).Log()
	}
	if indications == nil {
		return nil, olterrors.NewErrInvalidValue(log.Fields{"indications": nil, "device-id": dh.device.Id}, nil).Log()
	}
	logger.Infow(ctx, "read indication started successfully", log.Fields{"device-id": dh.device.Id})
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
	raisedTs := time.Now().Unix()
	if oltIndication.OperState == "up" && dh.transitionMap.currentDeviceState != deviceStateUp {
		dh.transitionMap.Handle(ctx, DeviceUpInd)
	} else if oltIndication.OperState == "down" {
		dh.transitionMap.Handle(ctx, DeviceDownInd)
	}
	// Send or clear Alarm
	if err := dh.eventMgr.oltUpDownIndication(ctx, oltIndication, dh.device.Id, raisedTs); err != nil {
		return olterrors.NewErrAdapter("failed-indication", log.Fields{
			"device-id":  dh.device.Id,
			"indication": oltIndication,
			"timestamp":  raisedTs}, err)
	}
	return nil
}

func (dh *DeviceHandler) handleOnuDisableIndication(ctx context.Context, onuIndication *oop.OnuDisabledIndication) error {

	logger.Infow(ctx, "handleOnuDisableIndication", log.Fields{"device-id": dh.device.Id, "onuInd": onuIndication})
	sn := dh.stringifySerialNumber(onuIndication.SerialNumber)
	intfID := onuIndication.GetIntfId()
	ponPort := plt.IntfIDToPortNo(intfID, voltha.Port_PON_OLT)
	onuDev := dh.getChildDevice(ctx, sn, ponPort)
	if onuDev == nil {
		return olterrors.NewErrAdapter("onu-device-fetch-failed", log.Fields{"device-id": dh.device.Id}, nil)
	}
	dh.discOnus.Delete(sn)

	raisedTs := time.Now().Unix()
	if err := dh.eventMgr.onuDisableIndication(ctx, onuIndication, dh.device.Id, onuDev, raisedTs); err != nil {
		return olterrors.NewErrAdapter("failed-indication", log.Fields{
			"device-id":  dh.device.Id,
			"indication": onuIndication,
			"timestamp":  raisedTs}, err)
	}
	OnuIndication := generateOnuIndication(onuDev.intfID, onuDev.onuID, "down", "down")
	err := dh.sendOnuIndicationToChildAdapter(ctx, onuDev.adapterEndpoint, &ia.OnuIndicationMessage{
		DeviceId:      onuDev.deviceID,
		OnuIndication: OnuIndication.GetOnuInd(),
	})
	if err != nil {
		return olterrors.NewErrCommunication("inter-adapter-send-failed", log.Fields{
			"onu-indicator": OnuIndication.GetOnuInd(),
			"source":        dh.openOLT.config.AdapterEndpoint,
			"device-type":   onuDev.deviceType,
			"device-id":     onuDev.deviceID}, err)
	}
	return nil
}

func (dh *DeviceHandler) handleOnuEnableIndication(ctx context.Context, onuIndication *oop.OnuEnabledIndication) error {
	logger.Infow(ctx, "handleOnuEnableIndication", log.Fields{"device-id": dh.device.Id, "onuInd": onuIndication})
	sn := dh.stringifySerialNumber(onuIndication.SerialNumber)
	intfID := onuIndication.GetIntfId()
	ponPort := plt.IntfIDToPortNo(intfID, voltha.Port_PON_OLT)
	onuDev := dh.getChildDevice(ctx, sn, ponPort)
	if onuDev == nil {
		return olterrors.NewErrAdapter("onu-device-fetch-failed", log.Fields{"device-id": dh.device.Id}, nil)
	}
	raisedTs := time.Now().Unix()

	if err := dh.eventMgr.onuEnableIndication(ctx, onuIndication, dh.device.Id, onuDev, raisedTs); err != nil {
		return olterrors.NewErrAdapter("failed-indication", log.Fields{
			"device-id":  dh.device.Id,
			"indication": onuIndication,
			"timestamp":  raisedTs}, err)
	}
	return nil
}

// nolint: gocyclo,govet
func (dh *DeviceHandler) handleIndication(ctx context.Context, indication *oop.Indication) {
	raisedTs := time.Now().Unix()
	switch indication.Data.(type) {
	case *oop.Indication_OltInd:
		span, ctx := log.CreateChildSpan(ctx, "olt-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()
		logger.Infow(ctx, "received olt indication", log.Fields{"device-id": dh.device.Id, "olt-ind": indication.GetOltInd()})
		if err := dh.handleOltIndication(ctx, indication.GetOltInd()); err != nil {
			_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "olt", "device-id": dh.device.Id}, err).Log()
		}
	case *oop.Indication_IntfInd:
		span, ctx := log.CreateChildSpan(ctx, "interface-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()

		intfInd := indication.GetIntfInd()
		go func() {
			if err := dh.addPort(ctx, intfInd.GetIntfId(), voltha.Port_PON_OLT, intfInd.GetOperState(), defaultPortSpeedMbps); err != nil {
				_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "interface", "device-id": dh.device.Id}, err).Log()
			}
		}()
		logger.Infow(ctx, "received-interface-indication", log.Fields{"InterfaceInd": intfInd, "device-id": dh.device.Id})
	case *oop.Indication_IntfOperInd:
		span, ctx := log.CreateChildSpan(ctx, "interface-oper-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()

		intfOperInd := indication.GetIntfOperInd()
		if intfOperInd.GetType() == "nni" {
			go func() {
				if err := dh.addPort(ctx, intfOperInd.GetIntfId(), voltha.Port_ETHERNET_NNI, intfOperInd.GetOperState(), intfOperInd.GetSpeed()); err != nil {
					_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "interface-oper-nni", "device-id": dh.device.Id}, err).Log()
				}
			}()
		} else if intfOperInd.GetType() == "pon" {
			// TODO: Check what needs to be handled here for When PON PORT down, ONU will be down
			// Handle pon port update
			go func() {
				if err := dh.addPort(ctx, intfOperInd.GetIntfId(), voltha.Port_PON_OLT, intfOperInd.GetOperState(), defaultPortSpeedMbps); err != nil {
					_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "interface-oper-pon", "device-id": dh.device.Id}, err).Log()
				}
			}()
			go dh.eventMgr.oltIntfOperIndication(ctx, indication.GetIntfOperInd(), dh.device.Id, raisedTs)
		}
		logger.Infow(ctx, "received-interface-oper-indication",
			log.Fields{"interfaceOperInd": intfOperInd,
				"device-id": dh.device.Id})
	case *oop.Indication_OnuDiscInd:
		span, ctx := log.CreateChildSpan(ctx, "onu-discovery-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()

		onuDiscInd := indication.GetOnuDiscInd()
		logger.Infow(ctx, "received-onu-discovery-indication", log.Fields{"OnuDiscInd": onuDiscInd, "device-id": dh.device.Id})
		// put message  to channel and return immediately
		dh.putOnuIndicationToChannel(ctx, indication, onuDiscInd.GetIntfId())
	case *oop.Indication_OnuInd:
		span, ctx := log.CreateChildSpan(ctx, "onu-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()

		onuInd := indication.GetOnuInd()
		logger.Infow(ctx, "received-onu-indication", log.Fields{"OnuInd": onuInd, "device-id": dh.device.Id})
		// put message  to channel and return immediately
		dh.putOnuIndicationToChannel(ctx, indication, onuInd.GetIntfId())
	case *oop.Indication_OmciInd:
		span, ctx := log.CreateChildSpan(ctx, "omci-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()

		omciInd := indication.GetOmciInd()
		logger.Debugw(ctx, "received-omci-indication", log.Fields{"intf-id": omciInd.IntfId, "onu-id": omciInd.OnuId, "device-id": dh.device.Id})
		go func() {
			if err := dh.omciIndication(ctx, omciInd); err != nil {
				_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "omci", "device-id": dh.device.Id}, err).Log()
			}
		}()
	case *oop.Indication_PktInd:
		span, ctx := log.CreateChildSpan(ctx, "packet-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()

		pktInd := indication.GetPktInd()
		logger.Debugw(ctx, "received-packet-indication", log.Fields{
			"intf-type":   pktInd.IntfId,
			"intf-id":     pktInd.IntfId,
			"gem-port-id": pktInd.GemportId,
			"port-no":     pktInd.PortNo,
			"device-id":   dh.device.Id,
		})

		if logger.V(log.DebugLevel) {
			logger.Debugw(ctx, "received-packet-indication-packet", log.Fields{
				"intf-type":   pktInd.IntfId,
				"intf-id":     pktInd.IntfId,
				"gem-port-id": pktInd.GemportId,
				"port-no":     pktInd.PortNo,
				"packet":      hex.EncodeToString(pktInd.Pkt),
				"device-id":   dh.device.Id,
			})
		}

		go func() {
			if err := dh.handlePacketIndication(ctx, pktInd); err != nil {
				_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "packet", "device-id": dh.device.Id}, err).Log()
			}
		}()
	case *oop.Indication_PortStats:
		span, ctx := log.CreateChildSpan(ctx, "port-statistics-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()

		portStats := indication.GetPortStats()
		go dh.portStats.PortStatisticsIndication(ctx, portStats, dh.totalPonPorts)
	case *oop.Indication_FlowStats:
		span, ctx := log.CreateChildSpan(ctx, "flow-stats-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()

		flowStats := indication.GetFlowStats()
		logger.Infow(ctx, "received-flow-stats", log.Fields{"FlowStats": flowStats, "device-id": dh.device.Id})
	case *oop.Indication_AlarmInd:
		span, ctx := log.CreateChildSpan(ctx, "alarm-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()

		alarmInd := indication.GetAlarmInd()
		logger.Infow(ctx, "received-alarm-indication", log.Fields{"AlarmInd": alarmInd, "device-id": dh.device.Id})
		go dh.eventMgr.ProcessEvents(ctx, alarmInd, dh.device.Id, raisedTs)
	case *oop.Indication_OnuDisabledInd:
		span, ctx := log.CreateChildSpan(ctx, "onu-disable-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()
		logger.Infow(ctx, "received onu-disable-indication", log.Fields{"device-id": dh.device.Id, "onu-ind": indication.GetOnuInd()})
		if err := dh.handleOnuDisableIndication(ctx, indication.GetOnuDisabledInd()); err != nil {
			_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "olt", "device-id": dh.device.Id, "onu-ind": indication.GetOnuInd()}, err).Log()
		}
	case *oop.Indication_OnuEnabledInd:
		span, ctx := log.CreateChildSpan(ctx, "onu-enable-indication", log.Fields{"device-id": dh.device.Id})
		defer span.Finish()
		logger.Infow(ctx, "received onu-enable-indication", log.Fields{"device-id": dh.device.Id, "onu-ind": indication.GetOnuInd()})
		if err := dh.handleOnuEnableIndication(ctx, indication.GetOnuEnabledInd()); err != nil {
			_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "olt", "device-id": dh.device.Id, "onu-ind": indication.GetOnuInd()}, err).Log()
		}
	}
}

func generateOnuIndication(intfID, onuID uint32, operState, adminState string) *oop.Indication {
	onuInd := &oop.OnuIndication{
		IntfId:     intfID,
		OnuId:      onuID,
		OperState:  operState,
		AdminState: adminState,
	}
	indication := &oop.Indication{
		Data: &oop.Indication_OnuInd{
			OnuInd: onuInd,
		},
	}
	return indication
}

func generateOnuAlarmIndication(intfID uint32, onuID uint32, losStatus string) *oop.AlarmIndication {
	onuAlarmInd := &oop.OnuAlarmIndication{
		IntfId:    intfID,
		OnuId:     onuID,
		LosStatus: losStatus,
	}
	alarmInd := &oop.AlarmIndication{
		Data: &oop.AlarmIndication_OnuAlarmInd{
			OnuAlarmInd: onuAlarmInd,
		},
	}
	return alarmInd
}

func generatePonLosAlarmIndication(intfID uint32, losStatus string) *oop.AlarmIndication {
	ponlosAlarmInd := &oop.LosIndication{
		IntfId: intfID,
		Status: losStatus,
	}
	alarmInd := &oop.AlarmIndication{
		Data: &oop.AlarmIndication_LosInd{
			LosInd: ponlosAlarmInd,
		},
	}
	return alarmInd
}

func (dh *DeviceHandler) updateIntfOperStateAndRaiseIndication(ctx context.Context, operState string, intfID uint32) {
	go func() {
		if err := dh.addPort(ctx, intfID, voltha.Port_PON_OLT, operState, defaultPortSpeedMbps); err != nil {
			_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{"type": "interface-oper-pon", "device-id": dh.device.Id}, err).Log()
		}
	}()

	raisedTs := time.Now().Unix()
	go dh.eventMgr.oltIntfOperIndication(ctx, &oop.IntfOperIndication{Type: "pon", IntfId: intfID, OperState: operState}, dh.device.Id, raisedTs)
}

func (dh *DeviceHandler) reconcileOnus(ctx context.Context) error {
	onuDevicesFromCore, err := dh.getChildDevicesFromCore(ctx, dh.device.Id)
	if err != nil {
		logger.Error(ctx, "unable to fetch child device", log.Fields{"eeror": err})
		return err
	}
	for _, onuDeviceFromCore := range onuDevicesFromCore.Items {
		onuOperStatusFromCore := onuDeviceFromCore.OperStatus
		onuConnectStatusFromCore := onuDeviceFromCore.ConnectStatus
		intfID := plt.PortNoToIntfID(onuDeviceFromCore.ParentPortNo, voltha.Port_PON_OLT)

		onuID := onuDeviceFromCore.ProxyAddress.OnuId
		onuDeviceFromOlt, err := dh.getOnuInfo(ctx, intfID, &onuID)
		if err != nil {
			logger.Error(ctx, "unable to get onu object from olt agent", log.Fields{"eeror": err})
		} else {
			onuOperStatusFromOlt := onuDeviceFromOlt.GetState()
			onuLosFromOlt := onuDeviceFromOlt.GetLosi()
			switch {
			case onuOperStatusFromOlt.String() == "ACTIVE" && onuOperStatusFromCore.String() != "ACTIVE":
				OnuIndication := generateOnuIndication(intfID, onuID, "up", "up")
				dh.putOnuIndicationToChannel(ctx, OnuIndication, intfID)

			case onuLosFromOlt.String() == "ON" && onuConnectStatusFromCore.String() == "REACHABLE":
				OnuIndication := generateOnuIndication(intfID, onuID, "down", "down") // check bal cli login notepad
				alarmInd := generateOnuAlarmIndication(intfID, onuID, "on")
				raisedTs := time.Now().Unix()
				go dh.eventMgr.ProcessEvents(ctx, alarmInd, dh.device.Id, raisedTs)

				dh.putOnuIndicationToChannel(ctx, OnuIndication, intfID)
			}
		}
	}
	return nil
}

func (dh *DeviceHandler) reconcilePonPorts(ctx context.Context) error { // need onuid and pon id
	portsFromCore, err := dh.getAllPortsFromCore(ctx, &ca.PortFilter{
		DeviceId: dh.device.Id,
		PortType: voltha.Port_PON_OLT,
	})
	if err != nil {
		logger.Error(ctx, "unable to fetch ports from core", log.Fields{"eeror": err})
		return err
	}
	for _, portFromCore := range portsFromCore.Items {
		portNum := portFromCore.GetPortNo()
		intfID := plt.PortNoToIntfID(portNum, voltha.Port_PON_OLT)
		portOperStatusFromCore := portFromCore.OperStatus
		portAdminStateFromCore := portFromCore.AdminState
		ponPortFromOlt, err := dh.getIntfInfo(ctx, intfID)
		if err != nil {
			logger.Error(ctx, "unable to get pon objects from olt agent", log.Fields{"eeror": err})
		} else {
			portLosFromOlt := ponPortFromOlt.GetLos()
			portStateFromOlt := ponPortFromOlt.GetState()
			if portOperStatusFromCore.String() == "ACTIVE" && portLosFromOlt.String() == "ON" {
				logger.Debug(ctx, "port is active in core but los is fired from olt", log.Fields{
					"portStateFromOlt":       portStateFromOlt.String(),
					"portOperStatusFromCore": portOperStatusFromCore.String(),
					"device-id":              dh.device.Id,
					"port":                   portNum})
				ponLosindication := generatePonLosAlarmIndication(intfID, "on")
				raisedTs := time.Now().Unix()
				go dh.eventMgr.ProcessEvents(ctx, ponLosindication, dh.device.Id, raisedTs)
			}
			switch {
			case portStateFromOlt.String() == "ACTIVE_WORKING" && portOperStatusFromCore.String() != "ACTIVE":
				logger.Debug(ctx, "mismatch between port state in voltha core and raising port up event", log.Fields{
					"portStateFromOlt":       portStateFromOlt.String(),
					"portOperStatusFromCore": portOperStatusFromCore.String(),
					"device-id":              dh.device.Id,
					"port":                   portNum})
				dh.updateIntfOperStateAndRaiseIndication(ctx, "up", intfID)
			case (portStateFromOlt.String() == "INACTIVE" || portStateFromOlt.String() == "UNKNOWN") && portOperStatusFromCore.String() == "ACTIVE":
				logger.Debug(ctx, "mismatch between port state in voltha core and raising port down event", log.Fields{
					"portStateFromOlt":       portStateFromOlt.String(),
					"portOperStatusFromCore": portOperStatusFromCore.String(),
					"device-id":              dh.device.Id,
					"port":                   portNum})
				dh.updateIntfOperStateAndRaiseIndication(ctx, "down", intfID)
			case portStateFromOlt.String() == "DISABLED" && portAdminStateFromCore.String() == "ENABLED":
				logger.Error(ctx, "port enabled in device but disabled at votlha core", log.Fields{
					"device-id": dh.device.Id,
					"port":      portNum})
			default:
				logger.Error(ctx, "mismatch between port state in voltha core and voltha device", log.Fields{
					"portStateFromOlt":       portStateFromOlt.String(),
					"portOperStatusFromCore": portOperStatusFromCore.String(),
					"device-id":              dh.device.Id,
					"port":                   portNum})
			}
		}
	}
	return nil
}

// doStateUp handle the olt up indication and update to voltha core
func (dh *DeviceHandler) doStateUp(ctx context.Context) error {
	//starting the stat collector
	// Declare deviceStateFilter to be used later
	var deviceStateFilter *ca.DeviceStateFilter
	go startCollector(ctx, dh)
	device, err := dh.getDeviceFromCore(ctx, dh.device.Id)
	if err != nil {
		return fmt.Errorf("failed to get device from core: %w", err)
	}
	logger.Info(ctx, "Device state", log.Fields{
		"device-id":      device.Id,
		"CurrOperStatus": device.OperStatus,
		"CurrConnStatus": device.ConnectStatus,
	})
	// Perform cleanup if the device's operational status is REBOOTED
	if device.OperStatus == voltha.OperStatus_RECONCILING && dh.prevOperStatus == voltha.OperStatus_REBOOTED {
		// Log the device's operational status if it's REBOOTED
		logger.Info(ctx, "Device is transitioning from REBOOTED to RECONCILING", log.Fields{
			"device-id":  device.Id,
			"OperStatus": device.OperStatus,
		})
		dh.lockDevice.RLock()
		// Stop the read indication only if it the routine is active
		// The read indication would have already stopped due to failure on the gRPC stream following OLT going unreachable
		// Sending message on the 'stopIndication' channel again will cause the readIndication routine to immediately stop
		// on next execution of the readIndication routine.
		if dh.isHeartbeatCheckActive {
			dh.stopHeartbeatCheck <- true
		}
		if dh.isReadIndicationRoutineActive {
			dh.stopIndications <- true
		}
		dh.lockDevice.RUnlock()
		if err = dh.cleanupDeviceResources(ctx); err != nil {
			logger.Error(ctx, "unable to clean up device resources", log.Fields{"error": err})
			return fmt.Errorf("cleanup device resources failed: %w", err)
		}
		if err = dh.initializeDeviceHandlerModules(ctx); err != nil {
			return olterrors.NewErrAdapter("device-handler-initialization-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}
		go startHeartbeatCheck(ctx, dh)
		//dh.lockDevice.RUnlock()
	} else if device.OperStatus == voltha.OperStatus_RECONCILING {
		// Log the device's operational status if it's RECONCILING
		logger.Info(ctx, "Device is being reconciled", log.Fields{
			"device-id":  device.Id,
			"OperStatus": device.OperStatus,
		})

		// Perform reconciliation steps
		err = dh.reconcileOnus(ctx)
		if err != nil {
			logger.Error(ctx, "unable to reconcile onu", log.Fields{"error": err})
		}
		err = dh.reconcilePonPorts(ctx)
		if err != nil {
			logger.Error(ctx, "unable to reconcile pon ports", log.Fields{"error": err})
		}
	}
	// instantiate the mcast handler routines.
	for i := range dh.incomingMcastFlowOrGroup {
		// We land inside the below "if" code path, after the OLT comes back from a reboot, otherwise the routines
		// are already active when the DeviceHandler module is first instantiated (as part of Adopt_device RPC invocation).
		if !dh.mcastHandlerRoutineActive[i] {
			// Spin up a go routine to handling incoming mcast flow/group (add/modify/remove).
			// There will be MaxNumOfGroupHandlerChannels number of mcastFlowOrGroupChannelHandlerRoutine go routines.
			// These routines will be blocked on the dh.incomingMcastFlowOrGroup[mcast-group-id modulo MaxNumOfGroupHandlerChannels] channel
			// for incoming mcast flow/group to be processed serially.
			dh.mcastHandlerRoutineActive[i] = true
			go dh.mcastFlowOrGroupChannelHandlerRoutine(i, dh.incomingMcastFlowOrGroup[i], dh.stopMcastHandlerRoutine[i])
		}
	}
	// Create DeviceStateFilter with the desired operational and connection statuses
	deviceStateFilter = &ca.DeviceStateFilter{
		DeviceId:   dh.device.Id,
		OperStatus: voltha.OperStatus_ACTIVE,
		ConnStatus: voltha.ConnectStatus_REACHABLE,
	}
	// Log DeviceStateFilter for debugging purposes
	logger.Info(ctx, "DeviceStateFilter", log.Fields{
		"DeviceId":   deviceStateFilter.DeviceId,
		"OperStatus": deviceStateFilter.OperStatus,
		"ConnStatus": deviceStateFilter.ConnStatus,
	})
	// Synchronous call to update device state - this method is run in its own go routine
	if err = dh.updateDeviceStateInCore(ctx, deviceStateFilter); err != nil {
		return olterrors.NewErrAdapter("device-state-update-failed", log.Fields{"device-id": dh.device.Id}, err)
	}

	// Clear olt communication failure event
	dh.device.ConnectStatus = voltha.ConnectStatus_REACHABLE
	dh.device.OperStatus = voltha.OperStatus_ACTIVE

	ports, err := dh.listDevicePortsFromCore(ctx, dh.device.Id)
	if err != nil {
		return olterrors.NewErrAdapter("fetch-ports-failed", log.Fields{"device-id": dh.device.Id}, err)
	}
	dh.populateActivePorts(ctx, ports.Items)
	if err = dh.disableAdminDownPorts(ctx, ports.Items); err != nil {
		logger.Error(ctx, "port-status-update-failed", log.Fields{"error": err, "ports": ports})
	}

	raisedTs := time.Now().Unix()
	go dh.eventMgr.oltCommunicationEvent(ctx, dh.device, raisedTs)

	return nil
}

// doStateDown handle the olt down indication
func (dh *DeviceHandler) doStateDown(ctx context.Context) error {
	logger.Debugw(ctx, "do-state-down-start", log.Fields{"device-id": dh.device.Id})

	device, err := dh.getDeviceFromCore(ctx, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		return olterrors.NewErrNotFound("device", log.Fields{"device-id": dh.device.Id}, err)
	}

	cloned := proto.Clone(device).(*voltha.Device)

	// Update the device oper state and connection status
	cloned.OperStatus = voltha.OperStatus_UNKNOWN
	dh.lockDevice.Lock()
	dh.device = cloned
	dh.lockDevice.Unlock()

	if err = dh.updateDeviceStateInCore(ctx, &ca.DeviceStateFilter{
		DeviceId:   cloned.Id,
		OperStatus: cloned.OperStatus,
		ConnStatus: cloned.ConnectStatus,
	}); err != nil {
		return olterrors.NewErrAdapter("state-update-failed", log.Fields{"device-id": device.Id}, err)
	}

	// get the child device for the parent device
	onuDevices, err := dh.getChildDevicesFromCore(ctx, dh.device.Id)
	if err != nil {
		return olterrors.NewErrAdapter("child-device-fetch-failed", log.Fields{"device-id": dh.device.Id}, err)
	}
	for _, onuDevice := range onuDevices.Items {
		// Update onu state as down in onu adapter
		onuInd := oop.OnuIndication{}
		onuInd.OperState = "down"

		ogClient, err := dh.getChildAdapterServiceClient(onuDevice.AdapterEndpoint)
		if err != nil {
			return err
		}
		subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
		_, err = ogClient.OnuIndication(subCtx, &ia.OnuIndicationMessage{
			DeviceId:      onuDevice.Id,
			OnuIndication: &onuInd,
		})
		cancel()
		if err != nil {
			_ = olterrors.NewErrCommunication("inter-adapter-send-failed", log.Fields{
				"source":        dh.openOLT.config.AdapterEndpoint,
				"onu-indicator": onuInd,
				"device-type":   onuDevice.Type,
				"device-id":     onuDevice.Id}, err).LogAt(log.ErrorLevel)
			// Do not return here and continue to process other ONUs
		} else {
			logger.Debugw(ctx, "sending inter adapter down ind to onu success", log.Fields{"olt-device-id": device.Id, "onu-device-id": onuDevice.Id})
		}
	}
	dh.lockDevice.Lock()
	/* Discovered ONUs entries need to be cleared , since after OLT
	   is up, it starts sending discovery indications again*/
	dh.discOnus = sync.Map{}
	dh.lockDevice.Unlock()

	logger.Debugw(ctx, "do-state-down-end", log.Fields{"device-id": device.Id})
	return nil
}

// doStateInit dial the grpc before going to init state
func (dh *DeviceHandler) doStateInit(ctx context.Context) error {
	var err error

	// if the connection is already available, close the previous connection (olt reboot case)
	if dh.clientCon != nil {
		if err = dh.clientCon.Close(); err != nil {
			logger.Errorw(ctx, "failed-to-close-previous-connection", log.Fields{"device-id": dh.device.Id})
		} else {
			logger.Debugw(ctx, "previous-grpc-channel-closed-successfully", log.Fields{"device-id": dh.device.Id})
		}
	}

	logger.Debugw(ctx, "Dailing grpc", log.Fields{"device-id": dh.device.Id})
	// Use Interceptors to automatically inject and publish Open Tracing Spans by this GRPC client
	dh.clientCon, err = grpc.Dial(dh.device.GetHostAndPort(),
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(
			grpc_opentracing.StreamClientInterceptor(grpc_opentracing.WithTracer(log.ActiveTracerProxy{})),
		)),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(
			grpc_opentracing.UnaryClientInterceptor(grpc_opentracing.WithTracer(log.ActiveTracerProxy{})),
		)))

	if err != nil {
		return olterrors.NewErrCommunication("dial-failure", log.Fields{
			"device-id":     dh.device.Id,
			"host-and-port": dh.device.GetHostAndPort()}, err)
	}
	//Setting oper and connection state to RECONCILING and conn state to reachable
	cgClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil {
		return err
	}

	if dh.device.OperStatus == voltha.OperStatus_RECONCILING {
		subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.openOLT.rpcTimeout)
		defer cancel()
		if _, err := cgClient.DeviceStateUpdate(subCtx, &ca.DeviceStateFilter{
			DeviceId:   dh.device.Id,
			OperStatus: voltha.OperStatus_RECONCILING,
			ConnStatus: voltha.ConnectStatus_REACHABLE,
		}); err != nil {
			return olterrors.NewErrAdapter("device-update-failed", log.Fields{"device-id": dh.device.Id}, err)
		}
		// The OperState and connection state of the device is set to RECONCILING and REACHABLE in the previous section. This also needs to be set on the
		// locally cached copy of the device struct.
		dh.device.OperStatus = voltha.OperStatus_RECONCILING
		dh.device.ConnectStatus = voltha.ConnectStatus_REACHABLE
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
	var err error
	logger.Debugw(ctx, "olt-device-connected", log.Fields{"device-id": dh.device.Id})

	// Case where OLT is disabled and then rebooted.
	device, err := dh.getDeviceFromCore(ctx, dh.device.Id)
	if err != nil || device == nil {
		/*TODO: needs to handle error scenarios */
		return olterrors.NewErrAdapter("device-fetch-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
	}
	if device.AdminState == voltha.AdminState_DISABLED {
		logger.Debugln(ctx, "do-state-connected--device-admin-state-down")

		cloned := proto.Clone(device).(*voltha.Device)
		cloned.ConnectStatus = voltha.ConnectStatus_REACHABLE
		cloned.OperStatus = voltha.OperStatus_UNKNOWN
		dh.device = cloned

		if err = dh.updateDeviceStateInCore(ctx, &ca.DeviceStateFilter{
			DeviceId:   cloned.Id,
			OperStatus: cloned.OperStatus,
			ConnStatus: cloned.ConnectStatus,
		}); err != nil {
			return olterrors.NewErrAdapter("device-state-update-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}

		// Since the device was disabled before the OLT was rebooted, enforce the OLT to be Disabled after re-connection.
		_, err = dh.Client.DisableOlt(ctx, new(oop.Empty))
		if err != nil {
			return olterrors.NewErrAdapter("olt-disable-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}
		// We should still go ahead an initialize various device handler modules so that when OLT is re-enabled, we have
		// all the modules initialized and ready to handle incoming ONUs.

		err = dh.initializeDeviceHandlerModules(ctx)
		if err != nil {
			return olterrors.NewErrAdapter("device-handler-initialization-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}

		go startHeartbeatCheck(ctx, dh)

		return nil
	}

	if err := dh.initializeDeviceHandlerModules(ctx); err != nil {
		return olterrors.NewErrAdapter("device-handler-initialization-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
	}

	go dh.updateLocalDevice(ctx)

	if device.PmConfigs != nil {
		dh.UpdatePmConfig(ctx, device.PmConfigs)
	}

	go startHeartbeatCheck(ctx, dh)

	return nil
}

func (dh *DeviceHandler) initializeDeviceHandlerModules(ctx context.Context) error {
	var err error
	dh.deviceInfo, err = dh.populateDeviceInfo(ctx)

	if dh.flowMgr != nil {
		dh.StopAllFlowRoutines(ctx)
	}

	dh.CloseKVClient(ctx)

	if err != nil {
		return olterrors.NewErrAdapter("populate-device-info-failed", log.Fields{"device-id": dh.device.Id}, err)
	}
	dh.totalPonPorts = dh.deviceInfo.GetPonPorts()
	dh.agentPreviouslyConnected = dh.deviceInfo.PreviouslyConnected
	// +1 is for NNI
	dh.resourceMgr = make([]*rsrcMgr.OpenOltResourceMgr, dh.totalPonPorts+1)
	dh.flowMgr = make([]*OpenOltFlowMgr, dh.totalPonPorts+1)
	var i uint32
	// Index from 0 to until totalPonPorts ( Ex: 0 .. 15 ) 	-> 	PonPort Managers
	// Index totalPonPorts ( Ex: 16 ) 				 	    -> 	NniPort Manager
	// There is only one NNI manager since multiple NNI is not supported for now
	for i = 0; i < dh.totalPonPorts+1; i++ {
		// Instantiate resource manager
		if dh.resourceMgr[i] = rsrcMgr.NewResourceMgr(ctx, i, dh.device.Id, dh.openOLT.KVStoreAddress, dh.openOLT.KVStoreType, dh.device.Type, dh.deviceInfo, dh.cm.Backend.PathPrefix); dh.resourceMgr[i] == nil {
			return olterrors.ErrResourceManagerInstantiating
		}
	}
	// GroupManager instance is per OLT. But it needs a reference to any instance of resourceMgr to interface with
	// the KV store to manage mcast group data. Provide the first instance (0th index)
	if dh.groupMgr = NewGroupManager(ctx, dh, dh.resourceMgr[0]); dh.groupMgr == nil {
		return olterrors.ErrGroupManagerInstantiating
	}
	for i = 0; i < dh.totalPonPorts+1; i++ {
		// Instantiate flow manager
		if dh.flowMgr[i] = NewFlowManager(ctx, dh, dh.resourceMgr[i], dh.groupMgr, i); dh.flowMgr[i] == nil {
			// Continue to check the rest of the ports
			logger.Errorw(ctx, "error-initializing-flow-manager-for-intf", log.Fields{"intfID": i, "device-id": dh.device.Id})
		} else {
			dh.resourceMgr[i].TechprofileRef = dh.flowMgr[i].techprofile
		}
	}
	/* TODO: Instantiate Alarm , stats , BW managers */
	/* Instantiating Event Manager to handle Alarms and KPIs */
	dh.eventMgr = NewEventMgr(dh.EventProxy, dh)

	// Stats config for new device
	dh.portStats = NewOpenOltStatsMgr(ctx, dh)

	return nil
}

func (dh *DeviceHandler) populateDeviceInfo(ctx context.Context) (*oop.DeviceInfo, error) {
	var err error
	var deviceInfo *oop.DeviceInfo
	var genmac string

	deviceInfo, err = dh.Client.GetDeviceInfo(log.WithSpanFromContext(context.Background(), ctx), new(oop.Empty))

	if err != nil {
		return nil, olterrors.NewErrPersistence("get", "device", 0, nil, err)
	}
	if deviceInfo == nil {
		return nil, olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil)
	}

	logger.Debugw(ctx, "fetched-device-info", log.Fields{"deviceInfo": deviceInfo, "device-id": dh.device.Id})
	dh.device.Root = true
	dh.device.Vendor = deviceInfo.Vendor
	dh.device.Model = deviceInfo.Model
	dh.device.SerialNumber = deviceInfo.DeviceSerialNumber
	dh.device.HardwareVersion = deviceInfo.HardwareVersion
	dh.device.FirmwareVersion = deviceInfo.FirmwareVersion

	if deviceInfo.DeviceId == "" {
		logger.Warnw(ctx, "no-device-id-provided-using-host", log.Fields{"hostport": dh.device.GetHostAndPort()})
		host := strings.Split(dh.device.GetHostAndPort(), ":")[0]
		genmac, err = generateMacFromHost(ctx, host)
		if err != nil {
			return nil, olterrors.NewErrAdapter("failed-to-generate-mac-host", log.Fields{"host": host}, err)
		}
		logger.Debugw(ctx, "using-host-for-mac-address", log.Fields{"host": host, "mac": genmac})
		dh.device.MacAddress = genmac
	} else {
		dh.device.MacAddress = deviceInfo.DeviceId
	}

	// Synchronous call to update device - this method is run in its own go routine
	if err = dh.updateDeviceInCore(ctx, dh.device); err != nil {
		return nil, olterrors.NewErrAdapter("device-update-failed", log.Fields{"device-id": dh.device.Id}, err)
	}

	return deviceInfo, nil
}

func startCollector(ctx context.Context, dh *DeviceHandler) {
	logger.Debugw(ctx, "starting-collector", log.Fields{"device-id": dh.device.Id})

	defer func() {
		dh.lockDevice.Lock()
		dh.isCollectorActive = false
		dh.lockDevice.Unlock()
	}()

	dh.lockDevice.Lock()
	dh.isCollectorActive = true
	dh.lockDevice.Unlock()

	for {
		select {
		case <-dh.stopCollector:
			logger.Debugw(ctx, "stopping-collector-for-olt", log.Fields{"device-id": dh.device.Id})
			return
		case <-time.After(time.Duration(dh.metrics.ToPmConfigs().DefaultFreq) * time.Second):

			ports, err := dh.listDevicePortsFromCore(ctx, dh.device.Id)
			if err != nil {
				logger.Warnw(ctx, "failed-to-list-ports", log.Fields{"device-id": dh.device.Id, "err": err})
				continue
			}
			for _, port := range ports.Items {
				// NNI Stats
				if port.Type == voltha.Port_ETHERNET_NNI {
					intfID := plt.PortNoToIntfID(port.PortNo, voltha.Port_ETHERNET_NNI)
					cmnni := dh.portStats.collectNNIMetrics(intfID)
					logger.Debugw(ctx, "collect-nni-metrics", log.Fields{"metrics": cmnni})
					go dh.portStats.publishMetrics(ctx, NNIStats, cmnni, port, dh.device.Id, dh.device.Type)
					logger.Debugw(ctx, "publish-nni-metrics", log.Fields{"nni-port": port.Label})
				}
				// PON Stats
				if port.Type == voltha.Port_PON_OLT {
					intfID := plt.PortNoToIntfID(port.PortNo, voltha.Port_PON_OLT)
					if val, ok := dh.activePorts.Load(intfID); ok && val == true {
						cmpon := dh.portStats.collectPONMetrics(intfID)
						logger.Debugw(ctx, "collect-pon-metrics", log.Fields{"metrics": cmpon})
						go dh.portStats.publishMetrics(ctx, PONStats, cmpon, port, dh.device.Id, dh.device.Type)
					}
					logger.Debugw(ctx, "publish-pon-metrics", log.Fields{"pon-port": port.Label})

					onuGemInfoLst := dh.resourceMgr[intfID].GetOnuGemInfoList(ctx)
					if len(onuGemInfoLst) > 0 {
						go dh.portStats.collectOnuAndGemStats(ctx, onuGemInfoLst)
					}
				}
			}
		}
	}
}

// AdoptDevice adopts the OLT device
func (dh *DeviceHandler) AdoptDevice(ctx context.Context, device *voltha.Device) {
	dh.transitionMap = NewTransitionMap(dh)
	logger.Infow(ctx, "adopt-device", log.Fields{"device-id": device.Id, "Address": device.GetHostAndPort()})
	dh.transitionMap.Handle(ctx, DeviceInit)

	// Now, set the initial PM configuration for that device
	cgClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil {
		logger.Errorw(ctx, "no-core-connection", log.Fields{"device-id": dh.device.Id, "error": err})
		return
	}

	// Now, set the initial PM configuration for that device
	if _, err := cgClient.DevicePMConfigUpdate(ctx, dh.metrics.ToPmConfigs()); err != nil {
		_ = olterrors.NewErrAdapter("error-updating-performance-metrics", log.Fields{"device-id": device.Id}, err).LogAt(log.ErrorLevel)
	}
}

// GetOfpDeviceInfo Gets the Ofp information of the given device
func (dh *DeviceHandler) GetOfpDeviceInfo(device *voltha.Device) (*ca.SwitchCapability, error) {
	return &ca.SwitchCapability{
		Desc: &of.OfpDesc{
			MfrDesc:   "VOLTHA Project",
			HwDesc:    "open_pon",
			SwDesc:    "open_pon",
			SerialNum: device.SerialNumber,
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

// GetTechProfileDownloadMessage fetches the TechProfileDownloadMessage for the caller.
func (dh *DeviceHandler) GetTechProfileDownloadMessage(ctx context.Context, request *ia.TechProfileInstanceRequestMessage) (*ia.TechProfileDownloadMessage, error) {
	ifID, err := plt.IntfIDFromPonPortNum(ctx, request.ParentPonPort)
	if err != nil {
		return nil, err
	}
	if dh.flowMgr == nil || dh.flowMgr[ifID] == nil {
		return nil, olterrors.NewErrNotFound("no-flow-manager-found", log.Fields{"intf-id": ifID, "parent-device-id": request.ParentDeviceId, "child-device-id": request.DeviceId}, nil).Log()
	}
	return dh.flowMgr[ifID].getTechProfileDownloadMessage(ctx, request.TpInstancePath, request.OnuId, request.DeviceId)
}

func (dh *DeviceHandler) omciIndication(ctx context.Context, omciInd *oop.OmciIndication) error {
	logger.Debugw(ctx, "omci-indication", log.Fields{"intf-id": omciInd.IntfId, "onu-id": omciInd.OnuId, "parent-device-id": dh.device.Id})
	var deviceType string
	var deviceID string
	var proxyDeviceID string
	var childAdapterEndpoint string

	transid := extractOmciTransactionID(omciInd.Pkt)
	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "recv-omci-msg", log.Fields{"intf-id": omciInd.IntfId, "onu-id": omciInd.OnuId, "device-id": dh.device.Id,
			"omci-transaction-id": transid, "omci-msg": hex.EncodeToString(omciInd.Pkt)})
	}

	onuKey := dh.formOnuKey(omciInd.IntfId, omciInd.OnuId)

	if onuInCache, ok := dh.onus.Load(onuKey); !ok {
		logger.Debugw(ctx, "omci-indication-for-a-device-not-in-cache.", log.Fields{"intf-id": omciInd.IntfId, "onu-id": omciInd.OnuId, "device-id": dh.device.Id})
		ponPort := plt.IntfIDToPortNo(omciInd.GetIntfId(), voltha.Port_PON_OLT)

		onuDevice, err := dh.getChildDeviceFromCore(ctx, &ca.ChildDeviceFilter{
			ParentId:     dh.device.Id,
			OnuId:        omciInd.OnuId,
			ParentPortNo: ponPort,
		})
		if err != nil {
			return olterrors.NewErrNotFound("onu", log.Fields{
				"intf-id": omciInd.IntfId,
				"onu-id":  omciInd.OnuId}, err)
		}
		deviceType = onuDevice.Type
		deviceID = onuDevice.Id
		proxyDeviceID = onuDevice.ProxyAddress.DeviceId
		childAdapterEndpoint = onuDevice.AdapterEndpoint
		// if not exist in cache, then add to cache.
		dh.onus.Store(onuKey, NewOnuDevice(deviceID, deviceType, onuDevice.SerialNumber, omciInd.OnuId, omciInd.IntfId, proxyDeviceID, false, onuDevice.AdapterEndpoint))
	} else {
		// found in cache
		logger.Debugw(ctx, "omci-indication-for-a-device-in-cache.", log.Fields{"intf-id": omciInd.IntfId, "onu-id": omciInd.OnuId, "device-id": dh.device.Id})
		deviceType = onuInCache.(*OnuDevice).deviceType
		deviceID = onuInCache.(*OnuDevice).deviceID
		proxyDeviceID = onuInCache.(*OnuDevice).proxyDeviceID
		childAdapterEndpoint = onuInCache.(*OnuDevice).adapterEndpoint
	}

	if err := dh.sendOmciIndicationToChildAdapter(ctx, childAdapterEndpoint, &ia.OmciMessage{
		ParentDeviceId: proxyDeviceID,
		ChildDeviceId:  deviceID,
		Message:        omciInd.Pkt,
	}); err != nil {
		return olterrors.NewErrCommunication("omci-request", log.Fields{
			"source":          dh.openOLT.config.AdapterEndpoint,
			"device-type":     deviceType,
			"destination":     childAdapterEndpoint,
			"onu-id":          deviceID,
			"proxy-device-id": proxyDeviceID}, err)
	}
	return nil
}

// //ProcessInterAdapterMessage sends the proxied messages to the target device
// // If the proxy address is not found in the unmarshalled message, it first fetches the onu device for which the message
// // is meant, and then send the unmarshalled omci message to this onu
// func (dh *DeviceHandler) ProcessInterAdapterMessage(ctx context.Context, msg *ca.InterAdapterMessage) error {
// 	logger.Debugw(ctx, "process-inter-adapter-message", log.Fields{"msgID": msg.Header.Id})
// 	if msg.Header.Type == ca.InterAdapterMessageType_OMCI_REQUEST {
// 		return dh.handleInterAdapterOmciMsg(ctx, msg)
// 	}
// 	return olterrors.NewErrInvalidValue(log.Fields{"inter-adapter-message-type": msg.Header.Type}, nil)
// }

// ProxyOmciRequests sends the proxied OMCI message to the target device
func (dh *DeviceHandler) ProxyOmciRequests(ctx context.Context, omciMsgs *ia.OmciMessages) error {
	if DeviceState(dh.device.ConnectStatus) != DeviceState(voltha.ConnectStatus_REACHABLE) {
		return status.Error(codes.Unavailable, "OLT unreachable")
	}
	if omciMsgs.GetProxyAddress() == nil {
		onuDevice, err := dh.getDeviceFromCore(ctx, omciMsgs.ChildDeviceId)
		if err != nil {
			return olterrors.NewErrNotFound("onu", log.Fields{
				"parent-device-id": dh.device.Id,
				"child-device-id":  omciMsgs.ChildDeviceId}, err)
		}
		logger.Debugw(ctx, "device-retrieved-from-core", log.Fields{"onu-device-proxy-address": onuDevice.ProxyAddress})
		if err := dh.sendProxyOmciRequests(log.WithSpanFromContext(context.Background(), ctx), onuDevice, omciMsgs); err != nil {
			return olterrors.NewErrCommunication("send-failed", log.Fields{
				"parent-device-id": dh.device.Id,
				"child-device-id":  omciMsgs.ChildDeviceId}, err)
		}
	} else {
		logger.Debugw(ctx, "proxy-address-found-in-omci-message", log.Fields{"onu-device-proxy-address": omciMsgs.ProxyAddress})
		if err := dh.sendProxyOmciRequests(log.WithSpanFromContext(context.Background(), ctx), nil, omciMsgs); err != nil {
			return olterrors.NewErrCommunication("send-failed", log.Fields{
				"parent-device-id": dh.device.Id,
				"child-device-id":  omciMsgs.ChildDeviceId}, err)
		}
	}
	return nil
}

func (dh *DeviceHandler) sendProxyOmciRequests(ctx context.Context, onuDevice *voltha.Device, omciMsgs *ia.OmciMessages) error {
	var intfID uint32
	var onuID uint32
	var connectStatus common.ConnectStatus_Types
	if onuDevice != nil {
		intfID = onuDevice.ProxyAddress.GetChannelId()
		onuID = onuDevice.ProxyAddress.GetOnuId()
		connectStatus = onuDevice.ConnectStatus
	} else {
		intfID = omciMsgs.GetProxyAddress().GetChannelId()
		onuID = omciMsgs.GetProxyAddress().GetOnuId()
		connectStatus = omciMsgs.GetConnectStatus()
	}
	if connectStatus != voltha.ConnectStatus_REACHABLE {
		logger.Debugw(ctx, "onu-not-reachable--cannot-send-omci", log.Fields{"intf-id": intfID, "onu-id": onuID})

		return olterrors.NewErrCommunication("unreachable", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID}, nil)
	}

	// TODO: OpenOLT Agent oop.OmciMsg expects a hex encoded string for OMCI packets rather than the actual bytes.
	//  Fix this in the agent and then we can pass byte array as Pkt: omciMsg.Message.

	onuSecOmciMsgList := omciMsgs.GetMessages()

	for _, onuSecOmciMsg := range onuSecOmciMsgList {
		var omciMessage *oop.OmciMsg
		hexPkt := make([]byte, hex.EncodedLen(len(onuSecOmciMsg)))
		hex.Encode(hexPkt, onuSecOmciMsg)
		omciMessage = &oop.OmciMsg{IntfId: intfID, OnuId: onuID, Pkt: hexPkt}

		// TODO: Below logging illustrates the "stringify" of the omci Pkt.
		//  once above is fixed this log line can change to just use hex.EncodeToString(omciMessage.Pkt)
		//https://jira.opencord.org/browse/VOL-4604
		transid := extractOmciTransactionID(onuSecOmciMsg)
		logger.Debugw(ctx, "sent-omci-msg", log.Fields{"intf-id": intfID, "onu-id": onuID,
			"omciTransactionID": transid, "omciMsg": string(omciMessage.Pkt)})

		_, err := dh.Client.OmciMsgOut(log.WithSpanFromContext(context.Background(), ctx), omciMessage)
		if err != nil {
			return olterrors.NewErrCommunication("omci-send-failed", log.Fields{
				"intf-id": intfID,
				"onu-id":  onuID,
				"message": omciMessage}, err)
		}
	}
	return nil
}

// ProxyOmciMessage sends the proxied OMCI message to the target device
func (dh *DeviceHandler) ProxyOmciMessage(ctx context.Context, omciMsg *ia.OmciMessage) error {
	logger.Debugw(ctx, "proxy-omci-message", log.Fields{"parent-device-id": omciMsg.ParentDeviceId, "child-device-id": omciMsg.ChildDeviceId, "proxy-address": omciMsg.ProxyAddress, "connect-status": omciMsg.ConnectStatus})

	if DeviceState(dh.device.ConnectStatus) != DeviceState(voltha.ConnectStatus_REACHABLE) {
		return status.Error(codes.Unavailable, "OLT unreachable")
	}
	if omciMsg.GetProxyAddress() == nil {
		onuDevice, err := dh.getDeviceFromCore(ctx, omciMsg.ChildDeviceId)
		if err != nil {
			return olterrors.NewErrNotFound("onu", log.Fields{
				"parent-device-id": dh.device.Id,
				"child-device-id":  omciMsg.ChildDeviceId}, err)
		}
		logger.Debugw(ctx, "device-retrieved-from-core", log.Fields{"onu-device-proxy-address": onuDevice.ProxyAddress})
		if err := dh.sendProxiedMessage(log.WithSpanFromContext(context.Background(), ctx), onuDevice, omciMsg); err != nil {
			return olterrors.NewErrCommunication("send-failed", log.Fields{
				"parent-device-id": dh.device.Id,
				"child-device-id":  omciMsg.ChildDeviceId}, err)
		}
	} else {
		logger.Debugw(ctx, "proxy-address-found-in-omci-message", log.Fields{"onu-device-proxy-address": omciMsg.ProxyAddress})
		if err := dh.sendProxiedMessage(log.WithSpanFromContext(context.Background(), ctx), nil, omciMsg); err != nil {
			return olterrors.NewErrCommunication("send-failed", log.Fields{
				"parent-device-id": dh.device.Id,
				"child-device-id":  omciMsg.ChildDeviceId}, err)
		}
	}
	return nil
}

func (dh *DeviceHandler) sendProxiedMessage(ctx context.Context, onuDevice *voltha.Device, omciMsg *ia.OmciMessage) error {
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
		logger.Debugw(ctx, "onu-not-reachable--cannot-send-omci", log.Fields{"intf-id": intfID, "onu-id": onuID})

		return olterrors.NewErrCommunication("unreachable", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID}, nil)
	}

	// TODO: OpenOLT Agent oop.OmciMsg expects a hex encoded string for OMCI packets rather than the actual bytes.
	//  Fix this in the agent and then we can pass byte array as Pkt: omciMsg.Message.
	// https://jira.opencord.org/browse/VOL-4604
	var omciMessage *oop.OmciMsg
	hexPkt := make([]byte, hex.EncodedLen(len(omciMsg.Message)))
	hex.Encode(hexPkt, omciMsg.Message)
	omciMessage = &oop.OmciMsg{IntfId: intfID, OnuId: onuID, Pkt: hexPkt}

	// TODO: Below logging illustrates the "stringify" of the omci Pkt.
	//  once above is fixed this log line can change to just use hex.EncodeToString(omciMessage.Pkt)
	transid := extractOmciTransactionID(omciMsg.Message)
	logger.Debugw(ctx, "sent-omci-msg", log.Fields{"intf-id": intfID, "onu-id": onuID,
		"omciTransactionID": transid, "omciMsg": string(omciMessage.Pkt)})

	_, err := dh.Client.OmciMsgOut(log.WithSpanFromContext(context.Background(), ctx), omciMessage)
	if err != nil {
		return olterrors.NewErrCommunication("omci-send-failed", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"message": omciMessage}, err)
	}
	return nil
}

func (dh *DeviceHandler) activateONU(ctx context.Context, intfID uint32, onuID int64, serialNum *oop.SerialNumber, serialNumber string) error {
	logger.Debugw(ctx, "activate-onu", log.Fields{"intf-id": intfID, "onu-id": onuID, "serialNum": serialNum, "serialNumber": serialNumber, "device-id": dh.device.Id, "OmccEncryption": dh.openOLT.config.OmccEncryption})
	if err := dh.resourceMgr[intfID].AddNewOnuGemInfoToCacheAndKvStore(ctx, uint32(onuID), serialNumber); err != nil {
		return olterrors.NewErrAdapter("onu-activate-failed", log.Fields{"onu": onuID, "intf-id": intfID}, err)
	}
	var pir uint32 = 1000000
	Onu := oop.Onu{IntfId: intfID, OnuId: uint32(onuID), SerialNumber: serialNum, Pir: pir, OmccEncryption: dh.openOLT.config.OmccEncryption}
	if _, err := dh.Client.ActivateOnu(ctx, &Onu); err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.AlreadyExists {
			logger.Debugw(ctx, "onu-activation-in-progress", log.Fields{"SerialNumber": serialNumber, "onu-id": onuID, "device-id": dh.device.Id})
		} else {
			return olterrors.NewErrAdapter("onu-activate-failed", log.Fields{"onu": Onu, "device-id": dh.device.Id}, err)
		}
	} else {
		logger.Infow(ctx, "activated-onu", log.Fields{"SerialNumber": serialNumber, "device-id": dh.device.Id})
	}
	return nil
}

// getChildDevice function can be used in general to get child device, if not found in cache the function will
// get from core and update the cache and return the child device.
func (dh *DeviceHandler) getChildDevice(ctx context.Context, sn string, parentPortNo uint32) *OnuDevice {
	var InCacheOnuDev *OnuDevice
	dh.onus.Range(func(Onukey interface{}, onuInCache interface{}) bool {
		if onuInCache.(*OnuDevice).serialNumber == sn {
			InCacheOnuDev = onuInCache.(*OnuDevice)
			return false
		}
		return true
	})
	// Got the onu device from cache return
	if InCacheOnuDev != nil {
		logger.Debugw(ctx, "Got child device from cache", log.Fields{"onudev": InCacheOnuDev.serialNumber})
		return InCacheOnuDev
	}
	onuDevice, _ := dh.getChildDeviceFromCore(ctx, &ca.ChildDeviceFilter{
		ParentId:     dh.device.Id,
		SerialNumber: sn,
		ParentPortNo: parentPortNo,
	})
	// No device found in core return nil
	if onuDevice == nil {
		return nil
	}
	onuID := onuDevice.ProxyAddress.OnuId
	intfID := plt.PortNoToIntfID(parentPortNo, voltha.Port_PON_OLT)
	onuKey := dh.formOnuKey(intfID, onuID)

	onuDev := NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuDevice.ProxyAddress.OnuId, intfID, onuDevice.ProxyAddress.DeviceId, false, onuDevice.AdapterEndpoint)
	dh.onus.Store(onuKey, onuDev)
	logger.Debugw(ctx, "got child device from core", log.Fields{"onudev": onuDevice})
	return onuDev
}

func (dh *DeviceHandler) checkForResourceExistance(ctx context.Context, onuDiscInd *oop.OnuDiscIndication, sn string) (bool, error) {
	channelID := onuDiscInd.GetIntfId()
	parentPortNo := plt.IntfIDToPortNo(onuDiscInd.GetIntfId(), voltha.Port_PON_OLT)
	tpInstExists := false

	// CheckOnuDevExistenceAtOnuDiscovery if true , a check will be made for the existence of the onu device. If the onu device
	// still exists , the onu discovery will be ignored, else a check for active techprofiles for ONU is checked.
	if !dh.openOLT.CheckOnuDevExistenceAtOnuDiscovery {
		onuDev := dh.getChildDevice(ctx, sn, parentPortNo)
		if onuDev != nil {
			var onuGemInfo *rsrcMgr.OnuGemInfo
			var err error
			if onuGemInfo, err = dh.resourceMgr[channelID].GetOnuGemInfo(ctx, onuDev.onuID); err != nil {
				logger.Warnw(ctx, "Unable to find onuGemInfo", log.Fields{"onuID": onuDev.onuID})
				return false, err
			}
			if onuGemInfo != nil {
				for _, uni := range onuGemInfo.UniPorts {
					uniID := plt.UniIDFromPortNum(uni)
					tpIDs := dh.resourceMgr[channelID].GetTechProfileIDForOnu(ctx, onuDev.onuID, uniID)
					if len(tpIDs) != 0 {
						logger.Warnw(ctx, "Techprofile present for ONU, ignoring onu discovery", log.Fields{"onuID": onuDev.onuID})
						tpInstExists = true
						break
					}
				}
			}
		}
		return tpInstExists, nil
	}

	onuDevice, _ := dh.getChildDeviceFromCore(ctx, &ca.ChildDeviceFilter{
		ParentId:     dh.device.Id,
		SerialNumber: sn,
		ParentPortNo: parentPortNo,
	})
	if onuDevice != nil {
		logger.Infow(ctx, "Child device still present ignoring discovery indication", log.Fields{"sn": sn})
		return true, nil
	}
	logger.Infow(ctx, "No device present in core , continuing with discovery", log.Fields{"sn": sn})

	return false, nil
}

// processDiscONULOSClear clears the LOS Alarm if it's needed
func (dh *DeviceHandler) processDiscONULOSClear(ctx context.Context, onuDiscInd *oop.OnuDiscIndication, sn string) {
	var alarmInd oop.OnuAlarmIndication
	raisedTs := time.Now().Unix()

	/* When PON cable disconnected and connected back from OLT, it was expected OnuAlarmIndication
	   with "los_status: off" should be raised but BAL does not raise this Alarm hence manually sending
	   OnuLosClear event on receiving OnuDiscoveryIndication for the Onu after checking whether
	   OnuLosRaise event sent for it */
	dh.onus.Range(func(Onukey interface{}, onuInCache interface{}) bool {
		if onuInCache.(*OnuDevice).serialNumber == sn && onuInCache.(*OnuDevice).losRaised {
			if onuDiscInd.GetIntfId() != onuInCache.(*OnuDevice).intfID {
				logger.Warnw(ctx, "onu-is-on-a-different-intf-id-now", log.Fields{
					"previousIntfId": onuInCache.(*OnuDevice).intfID,
					"currentIntfId":  onuDiscInd.GetIntfId()})
				// TODO:: Should we need to ignore raising OnuLosClear event
				// when onu connected to different PON?
			}
			alarmInd.IntfId = onuInCache.(*OnuDevice).intfID
			alarmInd.OnuId = onuInCache.(*OnuDevice).onuID
			alarmInd.LosStatus = statusCheckOff
			go func() {
				if err := dh.eventMgr.onuAlarmIndication(ctx, &alarmInd, onuInCache.(*OnuDevice).deviceID, raisedTs); err != nil {
					logger.Errorw(ctx, "indication-failed", log.Fields{"err": err})
				}
			}()
			// stop iterating
			return false
		}
		return true
	})
}

func (dh *DeviceHandler) handleOnuDiscoveryProcessingError(ctx context.Context, err error, sn string, tpInstExists bool) {
	if err != nil || tpInstExists {
		logger.Infow(ctx, "onu-processing-errored-out-not-adding-to-discovery-map", log.Fields{"sn": sn})
	} else {
		// once the function completes set the value to false so that
		// we know the processing has inProcess.
		// Note that this is done after checking if we are already processing
		// to avoid changing the value from a different thread
		logger.Infow(ctx, "onu-processing-completed", log.Fields{"sn": sn})
		dh.discOnus.Store(sn, false)
	}
}

func (dh *DeviceHandler) onuDiscIndication(ctx context.Context, onuDiscInd *oop.OnuDiscIndication) error {
	var error error
	var tpInstExists bool

	channelID := onuDiscInd.GetIntfId()
	parentPortNo := plt.IntfIDToPortNo(onuDiscInd.GetIntfId(), voltha.Port_PON_OLT)

	sn := dh.stringifySerialNumber(onuDiscInd.SerialNumber)
	defer func() {
		dh.handleOnuDiscoveryProcessingError(ctx, error, sn, tpInstExists)
	}()

	logger.Infow(ctx, "new-discovery-indication", log.Fields{"sn": sn})

	tpInstExists, error = dh.checkForResourceExistance(ctx, onuDiscInd, sn)
	if error != nil {
		return error
	}
	if tpInstExists {
		// ignore the discovery if tpinstance is present.
		logger.Debugw(ctx, "ignoring-onu-indication-as-tp-already-exists", log.Fields{"sn": sn})
		return nil
	}
	inProcess, existing := dh.discOnus.LoadOrStore(sn, true)

	// if the ONU existed, handle the LOS Alarm
	if existing {
		if inProcess.(bool) {
			// if we're currently processing the ONU on a different thread, do nothing
			logger.Warnw(ctx, "onu-sn-is-being-processed", log.Fields{"sn": sn})
			return nil
		}
		// if we had dealt with this ONU before, but the process didn't complete (this happens in case of errors)
		// then continue processing it
		logger.Debugw(ctx, "onu-processing-had-completed-but-new-indication", log.Fields{"sn": sn})

		dh.processDiscONULOSClear(ctx, onuDiscInd, sn)
		return nil
	}
	var onuID uint32

	// check the ONU is already know to the OLT
	// NOTE the second time the ONU is discovered this should return a device
	onuDevice, error := dh.getChildDeviceFromCore(ctx, &ca.ChildDeviceFilter{
		ParentId:     dh.device.Id,
		SerialNumber: sn,
	})

	if error != nil {
		logger.Debugw(ctx, "core-proxy-get-child-device-failed", log.Fields{"parentDevice": dh.device.Id, "err": error, "sn": sn})
		if e, ok := status.FromError(error); ok {
			logger.Debugw(ctx, "core-proxy-get-child-device-failed-with-code", log.Fields{"errCode": e.Code(), "sn": sn})
			switch e.Code() {
			case codes.Internal:
				// this probably means NOT FOUND, so just create a new device
				onuDevice = nil
			case codes.DeadlineExceeded:
				// if the call times out, cleanup and exit
				dh.discOnus.Delete(sn)
				error = olterrors.NewErrTimeout("get-child-device", log.Fields{"device-id": dh.device.Id}, error)
				return error
			}
		}
	}

	if onuDevice == nil {
		// NOTE this should happen a single time, and only if GetChildDevice returns NotFound
		logger.Debugw(ctx, "creating-new-onu", log.Fields{"sn": sn})
		// we need to create a new ChildDevice
		ponintfid := onuDiscInd.GetIntfId()
		onuID, error = dh.resourceMgr[ponintfid].GetONUID(ctx)
		logger.Infow(ctx, "creating-new-onu-got-onu-id", log.Fields{"sn": sn, "onuId": onuID})

		if error != nil {
			// if we can't create an ID in resource manager,
			// cleanup and exit
			dh.discOnus.Delete(sn)

			error = olterrors.NewErrAdapter("resource-manager-get-onu-id-failed", log.Fields{
				"pon-intf-id":   ponintfid,
				"serial-number": sn}, error)
			return error
		}

		if onuDevice, error = dh.sendChildDeviceDetectedToCore(ctx, &ca.DeviceDiscovery{
			ParentId:     dh.device.Id,
			ParentPortNo: parentPortNo,
			ChannelId:    channelID,
			VendorId:     string(onuDiscInd.SerialNumber.GetVendorId()),
			SerialNumber: sn,
			OnuId:        onuID,
		}); error != nil {
			dh.discOnus.Delete(sn)
			dh.resourceMgr[ponintfid].FreeonuID(ctx, []uint32{onuID}) // NOTE I'm not sure this method is actually cleaning up the right thing

			error = olterrors.NewErrAdapter("core-proxy-child-device-detected-failed", log.Fields{
				"pon-intf-id":   ponintfid,
				"serial-number": sn}, error)
			return error
		}
		if error = dh.eventMgr.OnuDiscoveryIndication(ctx, onuDiscInd, dh.device.Id, onuDevice.Id, onuID, sn, time.Now().Unix()); error != nil {
			logger.Error(ctx, "discovery-indication-failed", log.Fields{"err": error})
			error = olterrors.NewErrAdapter("discovery-indication-failed", log.Fields{
				"onu-id":        onuID,
				"device-id":     dh.device.Id,
				"serial-number": sn}, error)
			return error
		}
		logger.Infow(ctx, "onu-child-device-added",
			log.Fields{"onuDevice": onuDevice,
				"sn":        sn,
				"onu-id":    onuID,
				"device-id": dh.device.Id})
	}

	// Setup the gRPC connection to the adapter responsible for that onuDevice, if not setup yet
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	error = dh.setupChildInterAdapterClient(subCtx, onuDevice.AdapterEndpoint)
	cancel()
	if error != nil {
		error = olterrors.NewErrCommunication("no-connection-to-child-adapter", log.Fields{"device-id": onuDevice.Id}, error)
		return error
	}

	// we can now use the existing ONU Id
	onuID = onuDevice.ProxyAddress.OnuId
	// Insert the ONU into cache to use in OnuIndication.
	//TODO: Do we need to remove this from the cache on ONU change, or wait for overwritten on next discovery.
	logger.Debugw(ctx, "onu-discovery-indication-key-create",
		log.Fields{"onu-id": onuID,
			"intfId": onuDiscInd.GetIntfId(),
			"sn":     sn})
	onuKey := dh.formOnuKey(onuDiscInd.GetIntfId(), onuID)

	onuDev := NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuID, onuDiscInd.GetIntfId(), onuDevice.ProxyAddress.DeviceId, false, onuDevice.AdapterEndpoint)
	dh.onus.Store(onuKey, onuDev)
	logger.Debugw(ctx, "new-onu-device-discovered",
		log.Fields{"onu": onuDev,
			"sn": sn})

	if error = dh.updateDeviceStateInCore(ctx, &ca.DeviceStateFilter{
		DeviceId:       onuDevice.Id,
		ParentDeviceId: dh.device.Id,
		OperStatus:     common.OperStatus_DISCOVERED,
		ConnStatus:     common.ConnectStatus_REACHABLE,
	}); error != nil {
		error = olterrors.NewErrAdapter("failed-to-update-device-state", log.Fields{
			"device-id":     onuDevice.Id,
			"serial-number": sn}, error)
		return error
	}

	logger.Infow(ctx, "onu-discovered-reachable", log.Fields{"device-id": onuDevice.Id, "sn": sn})
	if error = dh.activateONU(ctx, onuDiscInd.IntfId, int64(onuID), onuDiscInd.SerialNumber, sn); error != nil {
		error = olterrors.NewErrAdapter("onu-activation-failed", log.Fields{
			"device-id":     onuDevice.Id,
			"serial-number": sn}, error)
		return error
	}
	return nil
}

func (dh *DeviceHandler) onuIndication(ctx context.Context, onuInd *oop.OnuIndication) error {
	ponPort := plt.IntfIDToPortNo(onuInd.GetIntfId(), voltha.Port_PON_OLT)
	var onuDevice *voltha.Device
	var err error
	foundInCache := false
	logger.Debugw(ctx, "onu-indication-key-create",
		log.Fields{"onuId": onuInd.OnuId,
			"intfId":    onuInd.GetIntfId(),
			"device-id": dh.device.Id})
	onuKey := dh.formOnuKey(onuInd.GetIntfId(), onuInd.OnuId)
	serialNumber := dh.stringifySerialNumber(onuInd.SerialNumber)
	errFields := log.Fields{"device-id": dh.device.Id}

	if onuInCache, ok := dh.onus.Load(onuKey); ok {
		// If ONU id is discovered before then use GetDevice to get onuDevice because it is cheaper.
		foundInCache = true
		errFields["onu-id"] = onuInCache.(*OnuDevice).deviceID
		onuDevice, err = dh.getDeviceFromCore(ctx, onuInCache.(*OnuDevice).deviceID)
	} else {
		// If ONU not found in adapter cache then we have to use GetChildDevice to get onuDevice
		if serialNumber != "" {
			errFields["serial-number"] = serialNumber
		} else {
			errFields["onu-id"] = onuInd.OnuId
			errFields["parent-port-no"] = ponPort
		}
		onuDevice, err = dh.getChildDeviceFromCore(ctx, &ca.ChildDeviceFilter{
			ParentId:     dh.device.Id,
			SerialNumber: serialNumber,
			OnuId:        onuInd.OnuId,
			ParentPortNo: ponPort,
		})
	}

	if err != nil || onuDevice == nil {
		return olterrors.NewErrNotFound("onu-device", errFields, err)
	}

	if onuDevice.ParentPortNo != ponPort {
		logger.Warnw(ctx, "onu-is-on-a-different-intf-id-now", log.Fields{
			"previousIntfId": onuDevice.ParentPortNo,
			"currentIntfId":  ponPort})
	}

	if onuDevice.ProxyAddress.OnuId != onuInd.OnuId {
		logger.Warnw(ctx, "onu-id-mismatch-possible-if-voltha-and-olt-rebooted", log.Fields{
			"expected-onu-id": onuDevice.ProxyAddress.OnuId,
			"received-onu-id": onuInd.OnuId,
			"device-id":       dh.device.Id})
	}
	if !foundInCache {
		onuKey := dh.formOnuKey(onuInd.GetIntfId(), onuInd.GetOnuId())
		dh.onus.Store(onuKey, NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuInd.GetOnuId(), onuInd.GetIntfId(), onuDevice.ProxyAddress.DeviceId, false, onuDevice.AdapterEndpoint))
	}
	if onuInd.OperState == "down" && onuInd.FailReason != oop.OnuIndication_ONU_ACTIVATION_FAIL_REASON_NONE {
		if err := dh.eventMgr.onuActivationIndication(ctx, onuActivationFailEvent, onuInd, dh.device.Id, time.Now().Unix()); err != nil {
			logger.Warnw(ctx, "onu-activation-indication-reporting-failed", log.Fields{"err": err})
		}
	}
	if err := dh.updateOnuStates(ctx, onuDevice, onuInd); err != nil {
		return olterrors.NewErrCommunication("state-update-failed", errFields, err)
	}
	return nil
}

func (dh *DeviceHandler) updateOnuStates(ctx context.Context, onuDevice *voltha.Device, onuInd *oop.OnuIndication) error {
	logger.Debugw(ctx, "onu-indication-for-state", log.Fields{"onuIndication": onuInd, "device-id": onuDevice.Id, "operStatus": onuDevice.OperStatus, "adminStatus": onuDevice.AdminState})
	if onuInd.AdminState == "down" || onuInd.OperState == "down" {
		// The ONU has gone admin_state "down" or oper_state "down" - we expect the ONU to send discovery again
		// The ONU admin_state is "up" while "oper_state" is down in cases where ONU activation fails. In this case
		// the ONU sends Discovery again.
		dh.discOnus.Delete(onuDevice.SerialNumber)
		// Tests have shown that we sometimes get OperState as NOT down even if AdminState is down, forcing it
		if onuInd.OperState != "down" {
			logger.Warnw(ctx, "onu-admin-state-down", log.Fields{"operState": onuInd.OperState})
			onuInd.OperState = "down"
		}
	}

	switch onuInd.OperState {
	case "up", "down":
		logger.Debugw(ctx, "sending-interadapter-onu-indication", log.Fields{"onuIndication": onuInd, "device-id": onuDevice.Id, "operStatus": onuDevice.OperStatus, "adminStatus": onuDevice.AdminState})

		err := dh.sendOnuIndicationToChildAdapter(ctx, onuDevice.AdapterEndpoint, &ia.OnuIndicationMessage{
			DeviceId:      onuDevice.Id,
			OnuIndication: onuInd,
		})
		if err != nil {
			return olterrors.NewErrCommunication("inter-adapter-send-failed", log.Fields{
				"onu-indicator": onuInd,
				"source":        dh.openOLT.config.AdapterEndpoint,
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
		VendorSpecific: decodedStr,
	}, nil
}

func (dh *DeviceHandler) stringifyVendorSpecific(vendorSpecific []byte) string {
	if len(vendorSpecific) > 3 {
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
	return ""
}

// UpdateFlowsBulk upates the bulk flow
func (dh *DeviceHandler) UpdateFlowsBulk() error {
	return olterrors.ErrNotImplemented
}

// GetChildDevice returns the child device for given parent port and onu id
func (dh *DeviceHandler) GetChildDevice(ctx context.Context, parentPort, onuID uint32) (*voltha.Device, error) {
	logger.Debugw(ctx, "getchilddevice",
		log.Fields{"pon-port": parentPort,
			"onu-id":    onuID,
			"device-id": dh.device.Id})

	onuDevice, err := dh.getChildDeviceFromCore(ctx, &ca.ChildDeviceFilter{
		ParentId:     dh.device.Id,
		OnuId:        onuID,
		ParentPortNo: parentPort,
	})

	if err != nil {
		return nil, olterrors.NewErrNotFound("onu-device", log.Fields{
			"intf-id": parentPort,
			"onu-id":  onuID}, err)
	}
	logger.Debugw(ctx, "successfully-received-child-device-from-core", log.Fields{"child-device-id": onuDevice.Id, "child-device-sn": onuDevice.SerialNumber})
	return onuDevice, nil
}

// SendPacketInToCore sends packet-in to core
// For this, it calls SendPacketIn of the core-proxy which uses a device specific topic to send the request.
// The adapter handling the device creates a device specific topic
func (dh *DeviceHandler) SendPacketInToCore(ctx context.Context, logicalPort uint32, packetPayload []byte) error {
	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "send-packet-in-to-core", log.Fields{
			"port":      logicalPort,
			"packet":    hex.EncodeToString(packetPayload),
			"device-id": dh.device.Id,
		})
	}

	if err := dh.sendPacketToCore(ctx, &ca.PacketIn{
		DeviceId: dh.device.Id,
		Port:     logicalPort,
		Packet:   packetPayload,
	}); err != nil {
		return olterrors.NewErrCommunication("packet-send-failed", log.Fields{
			"source":       "adapter",
			"destination":  "core",
			"device-id":    dh.device.Id,
			"logical-port": logicalPort,
			"packet":       hex.EncodeToString(packetPayload)}, err)
	}
	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "sent-packet-in-to-core-successfully", log.Fields{
			"packet":    hex.EncodeToString(packetPayload),
			"device-id": dh.device.Id,
		})
	}
	return nil
}

// UpdatePmConfig updates the pm metrics.
func (dh *DeviceHandler) UpdatePmConfig(ctx context.Context, pmConfigs *voltha.PmConfigs) {
	logger.Infow(ctx, "update-pm-configs", log.Fields{"device-id": dh.device.Id, "pm-configs": pmConfigs})

	if pmConfigs.DefaultFreq != dh.metrics.ToPmConfigs().DefaultFreq {
		dh.metrics.UpdateFrequency(pmConfigs.DefaultFreq)
		logger.Debugf(ctx, "frequency-updated")
	}

	if !pmConfigs.Grouped {
		metrics := dh.metrics.GetSubscriberMetrics()
		for _, m := range pmConfigs.Metrics {
			metrics[m.Name].Enabled = m.Enabled
		}
	}
}

func (dh *DeviceHandler) handleFlows(ctx context.Context, device *voltha.Device, flows *of.FlowChanges, flowMetadata *of.FlowMetadata) []error {
	var err error
	var errorsList []error

	if dh.getDeviceDeletionInProgressFlag() {
		// The device itself is going to be reset as part of deletion. So nothing to be done.
		logger.Infow(ctx, "device-deletion-in-progress--not-handling-flows-or-groups", log.Fields{"device-id": device.Id})
		return nil
	}

	if flows != nil {
		for _, flow := range flows.ToRemove.Items {
			intfID := dh.getIntfIDFromFlow(flow)

			logger.Debugw(ctx, "removing-flow",
				log.Fields{"device-id": device.Id,
					"intfId":       intfID,
					"flowToRemove": flow})
			if flow_utils.HasGroup(flow) {
				err = dh.RouteMcastFlowOrGroupMsgToChannel(ctx, flow, nil, McastFlowOrGroupRemove)
			} else {
				if dh.flowMgr == nil || dh.flowMgr[intfID] == nil {
					err = fmt.Errorf("flow-manager-uninitialized-%v", device.Id)
				} else {
					err = dh.flowMgr[intfID].RouteFlowToOnuChannel(ctx, flow, false, nil)
				}
			}
			if err != nil {
				if werr, ok := err.(olterrors.WrappedError); ok && status.Code(werr.Unwrap()) == codes.NotFound {
					// The flow we want to remove is not there, there is no need to throw an error
					logger.Warnw(ctx, "flow-to-remove-not-found",
						log.Fields{
							"ponIf":        intfID,
							"flowToRemove": flow,
							"error":        err,
						})
				} else {
					errorsList = append(errorsList, err)
				}
			}
		}

		for _, flow := range flows.ToAdd.Items {
			intfID := dh.getIntfIDFromFlow(flow)
			logger.Debugw(ctx, "adding-flow",
				log.Fields{"device-id": device.Id,
					"ponIf":     intfID,
					"flowToAdd": flow})
			if flow_utils.HasGroup(flow) {
				err = dh.RouteMcastFlowOrGroupMsgToChannel(ctx, flow, nil, McastFlowOrGroupAdd)
			} else {
				if dh.flowMgr == nil || dh.flowMgr[intfID] == nil {
					// The flow manager module could be uninitialized if the flow arrives too soon before the device has reconciled fully
					logger.Errorw(ctx, "flow-manager-uninitialized", log.Fields{"device-id": device.Id})
					err = fmt.Errorf("flow-manager-uninitialized-%v", device.Id)
				} else {
					err = dh.flowMgr[intfID].RouteFlowToOnuChannel(ctx, flow, true, flowMetadata)
				}
			}
			if err != nil {
				errorsList = append(errorsList, err)
			}
		}
	}

	return errorsList
}

func (dh *DeviceHandler) handleGroups(ctx context.Context, groups *of.FlowGroupChanges) []error {
	var err error
	var errorsList []error

	if dh.getDeviceDeletionInProgressFlag() {
		// The device itself is going to be reset as part of deletion. So nothing to be done.
		logger.Infow(ctx, "device-deletion-in-progress--not-handling-flows-or-groups", log.Fields{"device-id": dh.device.Id})
		return nil
	}

	// Whether we need to synchronize multicast group adds and modifies like flow add and delete needs to be investigated
	if groups != nil {
		for _, group := range groups.ToAdd.Items {
			// err = dh.groupMgr.AddGroup(ctx, group)
			err = dh.RouteMcastFlowOrGroupMsgToChannel(ctx, nil, group, McastFlowOrGroupAdd)
			if err != nil {
				errorsList = append(errorsList, err)
			}
		}
		for _, group := range groups.ToUpdate.Items {
			// err = dh.groupMgr.ModifyGroup(ctx, group)
			err = dh.RouteMcastFlowOrGroupMsgToChannel(ctx, nil, group, McastFlowOrGroupModify)
			if err != nil {
				errorsList = append(errorsList, err)
			}
		}
		for _, group := range groups.ToRemove.Items {
			// err = dh.groupMgr.DeleteGroup(ctx, group)
			err = dh.RouteMcastFlowOrGroupMsgToChannel(ctx, nil, group, McastFlowOrGroupRemove)
			if err != nil {
				errorsList = append(errorsList, err)
			}
		}
	}

	return errorsList
}

// UpdateFlowsIncrementally updates the device flow
func (dh *DeviceHandler) UpdateFlowsIncrementally(ctx context.Context, device *voltha.Device, flows *of.FlowChanges, groups *of.FlowGroupChanges, flowMetadata *of.FlowMetadata) error {
	var errorsList []error

	if dh.getDeviceDeletionInProgressFlag() {
		// The device itself is going to be reset as part of deletion. So nothing to be done.
		logger.Infow(ctx, "device-deletion-in-progress--not-handling-flows-or-groups", log.Fields{"device-id": device.Id})
		return nil
	}
	if dh.transitionMap.currentDeviceState != deviceStateUp {
		logger.Error(ctx, "device-is-not-up--not-handling-flows-or-groups", log.Fields{"device-id": device.Id, "current-device-state": dh.transitionMap.currentDeviceState})
		return fmt.Errorf("device-is-not-up--not-handling-flows-or-groups")
	}
	logger.Debugw(ctx, "received-incremental-flowupdate-in-device-handler", log.Fields{"device-id": device.Id, "flows": flows, "groups": groups, "flowMetadata": flowMetadata})
	errorsList = append(errorsList, dh.handleFlows(ctx, device, flows, flowMetadata)...)
	errorsList = append(errorsList, dh.handleGroups(ctx, groups)...)
	if len(errorsList) > 0 {
		return fmt.Errorf("errors-installing-flows-groups, errors:%v", errorsList)
	}
	logger.Debugw(ctx, "updated-flows-incrementally-successfully", log.Fields{"device-id": dh.device.Id})
	return nil
}

// DisableDevice disables the given device
// It marks the following for the given device:
// Device-Handler Admin-State : down
// Device Port-State: UNKNOWN
// Device Oper-State: UNKNOWN
func (dh *DeviceHandler) DisableDevice(ctx context.Context, device *voltha.Device) error {
	/* On device disable ,admin state update has to be done prior sending request to agent since
	   the indication thread may processes invalid  indications of ONU and OLT*/
	if dh.Client != nil {
		if _, err := dh.Client.DisableOlt(log.WithSpanFromContext(context.Background(), ctx), new(oop.Empty)); err != nil {
			if e, ok := status.FromError(err); ok && e.Code() == codes.Internal {
				return olterrors.NewErrAdapter("olt-disable-failed", log.Fields{"device-id": device.Id}, err)
			}
		}
	}
	logger.Debugw(ctx, "olt-disabled", log.Fields{"device-id": device.Id})
	/* Discovered ONUs entries need to be cleared , since on device disable the child devices goes to
	UNREACHABLE state which needs to be configured again*/

	dh.discOnus = sync.Map{}
	dh.onus = sync.Map{}

	dh.lockDevice.RLock()
	// stopping the stats collector
	if dh.isCollectorActive {
		dh.stopCollector <- true
	}
	dh.lockDevice.RUnlock()

	go dh.notifyChildDevices(ctx, "unreachable")
	cloned := proto.Clone(device).(*voltha.Device)
	// Update device Admin state
	dh.device = cloned

	// Update the all pon ports state on that device to disable and NNI remains active as NNI remains active in openolt agent.
	if err := dh.updatePortsStateInCore(ctx, &ca.PortStateFilter{
		DeviceId:       cloned.Id,
		PortTypeFilter: ^uint32(1 << voltha.Port_PON_OLT),
		OperStatus:     voltha.OperStatus_UNKNOWN,
	}); err != nil {
		return olterrors.NewErrAdapter("ports-state-update-failed", log.Fields{"device-id": device.Id}, err)
	}
	logger.Debugw(ctx, "disable-device-end", log.Fields{"device-id": device.Id})
	return nil
}

func (dh *DeviceHandler) notifyChildDevices(ctx context.Context, state string) {
	// Update onu state as unreachable in onu adapter
	onuInd := oop.OnuIndication{}
	onuInd.OperState = state

	// get the child device for the parent device
	onuDevices, err := dh.getChildDevicesFromCore(ctx, dh.device.Id)
	if err != nil {
		logger.Errorw(ctx, "failed-to-get-child-devices-information", log.Fields{"device-id": dh.device.Id, "err": err})
	}
	if onuDevices != nil {
		for _, onuDevice := range onuDevices.Items {
			err := dh.sendOnuIndicationToChildAdapter(ctx, onuDevice.AdapterEndpoint, &ia.OnuIndicationMessage{
				DeviceId:      onuDevice.Id,
				OnuIndication: &onuInd,
			})
			if err != nil {
				logger.Errorw(ctx, "failed-to-send-inter-adapter-message", log.Fields{"OnuInd": onuInd,
					"From Adapter": dh.openOLT.config.AdapterEndpoint, "DeviceType": onuDevice.Type, "device-id": onuDevice.Id})
			}
		}
	}
}

// ReenableDevice re-enables the olt device after disable
// It marks the following for the given device:
// Device-Handler Admin-State : up
// Device Port-State: ACTIVE
// Device Oper-State: ACTIVE
func (dh *DeviceHandler) ReenableDevice(ctx context.Context, device *voltha.Device) error {
	if dh.Client != nil {
		if _, err := dh.Client.ReenableOlt(log.WithSpanFromContext(context.Background(), ctx), new(oop.Empty)); err != nil {
			if e, ok := status.FromError(err); ok && e.Code() == codes.Internal {
				return olterrors.NewErrAdapter("olt-reenable-failed", log.Fields{"device-id": dh.device.Id}, err)
			}
		}
	} else {
		return olterrors.NewErrAdapter("olt-reenable-failed", log.Fields{"device-id": dh.device.Id}, errors.New("nil device client"))
	}
	logger.Debug(ctx, "olt-reenabled")

	// Update the all ports state on that device to enable
	ports, err := dh.listDevicePortsFromCore(ctx, device.Id)
	var retError error
	if err != nil {
		retError = olterrors.NewErrAdapter("list-ports-failed", log.Fields{"device-id": device.Id}, err)
	} else {
		if err := dh.disableAdminDownPorts(ctx, ports.Items); err != nil {
			retError = olterrors.NewErrAdapter("port-status-update-failed-after-olt-reenable", log.Fields{"device": device}, err)
		}
	}
	if retError == nil {
		// Update the device oper status as ACTIVE
		device.OperStatus = voltha.OperStatus_ACTIVE
	} else {
		// Update the device oper status as FAILED
		device.OperStatus = voltha.OperStatus_FAILED
	}
	dh.device = device

	if err := dh.updateDeviceStateInCore(ctx, &ca.DeviceStateFilter{
		DeviceId:   device.Id,
		OperStatus: device.OperStatus,
		ConnStatus: device.ConnectStatus,
	}); err != nil {
		return olterrors.NewErrAdapter("state-update-failed", log.Fields{
			"device-id":      device.Id,
			"connect-status": device.ConnectStatus,
			"oper-status":    device.OperStatus}, err)
	}

	logger.Debugw(ctx, "reenabledevice-end", log.Fields{"device-id": device.Id})

	return retError
}

func (dh *DeviceHandler) clearUNIData(ctx context.Context, onu *rsrcMgr.OnuGemInfo) error {
	var uniID uint32
	var err error
	var errs []error
	for _, port := range onu.UniPorts {
		uniID = plt.UniIDFromPortNum(port)
		logger.Debugw(ctx, "clearing-resource-data-for-uni-port", log.Fields{"port": port, "uni-id": uniID})
		/* Delete tech-profile instance from the KV store */
		if dh.flowMgr == nil || dh.flowMgr[onu.IntfID] == nil {
			logger.Debugw(ctx, "failed-to-remove-tech-profile-instance-for-onu-no-flowmng", log.Fields{"onu-id": onu.OnuID})
		} else {
			if err = dh.flowMgr[onu.IntfID].DeleteTechProfileInstances(ctx, onu.IntfID, onu.OnuID, uniID); err != nil {
				logger.Debugw(ctx, "failed-to-remove-tech-profile-instance-for-onu", log.Fields{"onu-id": onu.OnuID})
				errs = append(errs, err)
			}
		}
		logger.Debugw(ctx, "deleted-tech-profile-instance-for-onu", log.Fields{"onu-id": onu.OnuID})
		tpIDList := dh.resourceMgr[onu.IntfID].GetTechProfileIDForOnu(ctx, onu.OnuID, uniID)
		for _, tpID := range tpIDList {
			if err = dh.resourceMgr[onu.IntfID].RemoveMeterInfoForOnu(ctx, "upstream", onu.OnuID, uniID, tpID); err != nil {
				logger.Debugw(ctx, "failed-to-remove-meter-id-for-onu-upstream", log.Fields{"onu-id": onu.OnuID})
				errs = append(errs, err)
			}
			logger.Debugw(ctx, "removed-meter-id-for-onu-upstream", log.Fields{"onu-id": onu.OnuID})
			if err = dh.resourceMgr[onu.IntfID].RemoveMeterInfoForOnu(ctx, "downstream", onu.OnuID, uniID, tpID); err != nil {
				logger.Debugw(ctx, "failed-to-remove-meter-id-for-onu-downstream", log.Fields{"onu-id": onu.OnuID})
				errs = append(errs, err)
			}
			logger.Debugw(ctx, "removed-meter-id-for-onu-downstream", log.Fields{"onu-id": onu.OnuID})
		}
		dh.resourceMgr[onu.IntfID].FreePONResourcesForONU(ctx, onu.OnuID, uniID)
		if err = dh.resourceMgr[onu.IntfID].RemoveTechProfileIDsForOnu(ctx, onu.OnuID, uniID); err != nil {
			logger.Debugw(ctx, "failed-to-remove-tech-profile-id-for-onu", log.Fields{"onu-id": onu.OnuID})
			errs = append(errs, err)
		}
		logger.Debugw(ctx, "removed-tech-profile-id-for-onu", log.Fields{"onu-id": onu.OnuID})
		if err = dh.resourceMgr[onu.IntfID].DeletePacketInGemPortForOnu(ctx, onu.OnuID, port); err != nil {
			logger.Debugw(ctx, "failed-to-remove-gemport-pkt-in", log.Fields{"intfid": onu.IntfID, "onuid": onu.OnuID, "uniId": uniID})
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return olterrors.NewErrAdapter(fmt.Errorf("one-or-more-error-during-clear-uni-data, errors:%v",
			errs).Error(), log.Fields{"device-id": dh.device.Id}, nil)
	}
	return nil
}

// DeleteDevice deletes the device instance from openolt handler array.  Also clears allocated resource manager resources.  Also reboots the OLT hardware!
func (dh *DeviceHandler) DeleteDevice(ctx context.Context, device *voltha.Device) error {
	logger.Debugw(ctx, "function-entry-delete-device", log.Fields{"device-id": dh.device.Id})
	/* Clear the KV store data associated with the all the UNI ports
	   This clears up flow data and also resource map data for various
	   other pon resources like alloc_id and gemport_id
	*/

	dh.setDeviceDeletionInProgressFlag(true)
	dh.StopAllFlowRoutines(ctx)

	dh.lockDevice.RLock()
	// Stop the Stats collector
	if dh.isCollectorActive {
		dh.stopCollector <- true
	}
	// stop the heartbeat check routine
	if dh.isHeartbeatCheckActive {
		dh.stopHeartbeatCheck <- true
	}
	// Stop the read indication only if it the routine is active
	if dh.isReadIndicationRoutineActive {
		dh.stopIndications <- true
	}
	dh.lockDevice.RUnlock()

	err := dh.cleanupDeviceResources(ctx)
	if err != nil {
		logger.Errorw(ctx, "could-not-remove-device-from-KV-store", log.Fields{"device-id": dh.device.Id, "err": err})
	} else {
		logger.Debugw(ctx, "successfully-removed-device-from-Resource-manager-KV-store", log.Fields{"device-id": dh.device.Id})
	}

	dh.removeOnuIndicationChannels(ctx)
	// Reset the state
	if dh.Client != nil {
		if _, err = dh.Client.Reboot(ctx, new(oop.Empty)); err != nil {
			go func() {
				failureReason := fmt.Sprintf("Failed to reboot during device delete request with error: %s", err.Error())
				if err1 := dh.eventMgr.oltRebootFailedEvent(ctx, dh.device.Id, failureReason, time.Now().Unix()); err1 != nil {
					logger.Errorw(ctx, "on-olt-reboot-failed", log.Fields{"device-id": dh.device.Id, "err": err1})
				}
			}()
			logger.Errorw(ctx, "olt-reboot-failed", log.Fields{"device-id": dh.device.Id, "err": err})
		}
	}
	// There is no need to update the core about operation status and connection status of the OLT.
	// The OLT is getting deleted anyway and the core might have already cleared the OLT device from its DB.
	// So any attempt to update the operation status and connection status of the OLT will result in core throwing an error back,
	// because the device does not exist in DB.

	// Stop the adapter grpc clients for that parent device
	dh.deleteAdapterClients(ctx)
	return err
}

// StopAllFlowRoutines stops all flow routines
func (dh *DeviceHandler) StopAllFlowRoutines(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1) // for the mcast routine below to finish
	go dh.StopAllMcastHandlerRoutines(ctx, &wg)
	for _, flMgr := range dh.flowMgr {
		if flMgr != nil {
			wg.Add(1) // for the flow handler routine below to finish
			go flMgr.StopAllFlowHandlerRoutines(ctx, &wg)
		}
	}
	if !dh.waitForTimeoutOrCompletion(&wg, time.Second*30) {
		logger.Warnw(ctx, "timed out waiting for stopping flow and group handlers", log.Fields{"deviceID": dh.device.Id})
	} else {
		logger.Infow(ctx, "all flow and group handlers shutdown gracefully", log.Fields{"deviceID": dh.device.Id})
	}
}

func (dh *DeviceHandler) cleanupDeviceResources(ctx context.Context) error {
	var errs []error
	if dh.resourceMgr != nil {
		var ponPort uint32
		for ponPort = 0; ponPort < dh.totalPonPorts; ponPort++ {
			onuGemData := dh.resourceMgr[ponPort].GetOnuGemInfoList(ctx)
			for i, onu := range onuGemData {
				logger.Debugw(ctx, "onu-data", log.Fields{"onu": onu})
				if err := dh.clearUNIData(ctx, &onuGemData[i]); err != nil {
					errs = append(errs, err)
				}
			}
			if err := dh.resourceMgr[ponPort].DeleteAllFlowIDsForGemForIntf(ctx); err != nil {
				errs = append(errs, err)
			}
			if err := dh.resourceMgr[ponPort].DeleteAllOnuGemInfoForIntf(ctx); err != nil {
				errs = append(errs, err)
			}
			if err := dh.resourceMgr[ponPort].DeleteMcastQueueForIntf(ctx); err != nil {
				errs = append(errs, err)
			}
			if err := dh.resourceMgr[ponPort].Delete(ctx, ponPort); err != nil {
				errs = append(errs, err)
			}
		}
	}
	// Clean up NNI manager's data
	if err := dh.resourceMgr[dh.totalPonPorts].DeleteAllFlowIDsForGemForIntf(ctx); err != nil {
		errs = append(errs, err)
	}

	dh.CloseKVClient(ctx)

	// Take one final sweep at cleaning up KV store for the OLT device
	// Clean everything at <base-path-prefix>/openolt/<device-id>
	if err := dh.kvStore.DeleteWithPrefix(ctx, ""); err != nil {
		errs = append(errs, err)
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
	if len(errs) > 0 {
		return olterrors.NewErrAdapter(fmt.Errorf("one-or-more-error-during-device-delete, errors:%v",
			errs).Error(), log.Fields{"device-id": dh.device.Id}, nil)
	}
	return nil
}

// RebootDevice reboots the given device
func (dh *DeviceHandler) RebootDevice(ctx context.Context, device *voltha.Device) error {
	if dh.Client != nil {
		if _, err := dh.Client.Reboot(log.WithSpanFromContext(context.Background(), ctx), new(oop.Empty)); err != nil {
			return olterrors.NewErrAdapter("olt-reboot-failed", log.Fields{"device-id": dh.device.Id}, err)
		}
	} else {
		return olterrors.NewErrAdapter("olt-reboot-failed", log.Fields{"device-id": dh.device.Id}, errors.New("nil device client"))
	}

	logger.Debugw(ctx, "rebooted-device-successfully", log.Fields{"device-id": device.Id})
	return nil
}

func (dh *DeviceHandler) handlePacketIndication(ctx context.Context, packetIn *oop.PacketIndication) error {
	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "received-packet-in", log.Fields{
			"packet-indication": *packetIn,
			"device-id":         dh.device.Id,
			"packet":            hex.EncodeToString(packetIn.Pkt),
		})
	}
	if dh.flowMgr == nil || dh.flowMgr[packetIn.IntfId] == nil {
		return olterrors.NewErrNotFound("flow-manager", log.Fields{"intf-id": packetIn.IntfId, "packet": hex.EncodeToString(packetIn.Pkt)}, nil)
	}
	logicalPortNum, err := dh.flowMgr[packetIn.IntfId].GetLogicalPortFromPacketIn(ctx, packetIn)
	if err != nil {
		return olterrors.NewErrNotFound("logical-port", log.Fields{"packet": hex.EncodeToString(packetIn.Pkt)}, err)
	}
	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "sending-packet-in-to-core", log.Fields{
			"logical-port-num": logicalPortNum,
			"device-id":        dh.device.Id,
			"packet":           hex.EncodeToString(packetIn.Pkt),
		})
	}

	if err := dh.sendPacketToCore(ctx, &ca.PacketIn{
		DeviceId: dh.device.Id,
		Port:     logicalPortNum,
		Packet:   packetIn.Pkt,
	}); err != nil {
		return olterrors.NewErrCommunication("send-packet-in", log.Fields{
			"destination": "core",
			"source":      dh.device.Type,
			"device-id":   dh.device.Id,
			"packet":      hex.EncodeToString(packetIn.Pkt),
		}, err)
	}

	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "success-sending-packet-in-to-core!", log.Fields{
			"packet":    hex.EncodeToString(packetIn.Pkt),
			"device-id": dh.device.Id,
		})
	}
	return nil
}

// PacketOutNNI sends packet-out from VOLTHA to OLT on the NNI provided
func (dh *DeviceHandler) PacketOutNNI(ctx context.Context, egressPortNo uint32, packet *of.OfpPacketOut) error {
	nniIntfID, err := plt.IntfIDFromNniPortNum(ctx, uint32(egressPortNo))
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{
			"egress-nni-port": egressPortNo,
			"device-id":       dh.device.Id,
		}, err)
	}
	uplinkPkt := oop.UplinkPacket{IntfId: nniIntfID, Pkt: packet.Data}

	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "sending-packet-to-nni", log.Fields{
			"uplink-pkt": uplinkPkt,
			"packet":     hex.EncodeToString(packet.Data),
			"device-id":  dh.device.Id,
		})
	}

	if _, err := dh.Client.UplinkPacketOut(ctx, &uplinkPkt); err != nil {
		return olterrors.NewErrCommunication("packet-out-to-nni", log.Fields{
			"packet":    hex.EncodeToString(packet.Data),
			"device-id": dh.device.Id,
		}, err)
	}
	return nil
}

// PacketOutUNI sends packet-out from VOLTHA to OLT on the UNI provided
func (dh *DeviceHandler) PacketOutUNI(ctx context.Context, egressPortNo uint32, packet *of.OfpPacketOut) error {
	outerEthType := (uint16(packet.Data[12]) << 8) | uint16(packet.Data[13])
	innerEthType := (uint16(packet.Data[16]) << 8) | uint16(packet.Data[17])
	if outerEthType == 0x8942 || outerEthType == 0x88cc {
		// Do not packet-out lldp packets on uni port.
		// ONOS has no clue about uni/nni ports, it just packets out on all
		// available ports on the Logical Switch. It should not be interested
		// in the UNI links.
		logger.Debugw(ctx, "dropping-lldp-packet-out-on-uni", log.Fields{
			"device-id": dh.device.Id,
		})
		return nil
	}
	if outerEthType == 0x88a8 || outerEthType == 0x8100 {
		if innerEthType == 0x8100 {
			// q-in-q 802.1ad or 802.1q double tagged packet.
			// slice out the outer tag.
			packet.Data = append(packet.Data[:12], packet.Data[16:]...)
			if logger.V(log.DebugLevel) {
				logger.Debugw(ctx, "packet-now-single-tagged", log.Fields{
					"packet-data": hex.EncodeToString(packet.Data),
					"device-id":   dh.device.Id,
				})
			}
		}
	}
	intfID := plt.IntfIDFromUniPortNum(uint32(egressPortNo))
	onuID := plt.OnuIDFromPortNum(uint32(egressPortNo))
	uniID := plt.UniIDFromPortNum(uint32(egressPortNo))
	var gemPortID uint32
	err := olterrors.NewErrNotFound("no-flow-manager-found-for-packet-out", log.Fields{"device-id": dh.device.Id}, nil).(error)
	if dh.flowMgr != nil && dh.flowMgr[intfID] != nil {
		gemPortID, err = dh.flowMgr[intfID].GetPacketOutGemPortID(ctx, intfID, onuID, uint32(egressPortNo), packet.Data)
	}
	if err != nil {
		// In this case the openolt agent will receive the gemPortID as 0.
		// The agent tries to retrieve the gemPortID in this case.
		// This may not always succeed at the agent and packetOut may fail.
		logger.Errorw(ctx, "failed-to-retrieve-gemport-id-for-packet-out", log.Fields{
			"intf-id":   intfID,
			"onu-id":    onuID,
			"uni-id":    uniID,
			"packet":    hex.EncodeToString(packet.Data),
			"device-id": dh.device.Id,
			"error":     err,
		})
	}

	onuPkt := oop.OnuPacket{IntfId: intfID, OnuId: onuID, PortNo: uint32(egressPortNo), GemportId: gemPortID, Pkt: packet.Data}
	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "sending-packet-to-onu", log.Fields{
			"egress-port-no": egressPortNo,
			"intf-id":        intfID,
			"onu-id":         onuID,
			"uni-id":         uniID,
			"gem-port-id":    gemPortID,
			"packet":         hex.EncodeToString(packet.Data),
			"device-id":      dh.device.Id,
		})
	}

	if _, err := dh.Client.OnuPacketOut(ctx, &onuPkt); err != nil {
		return olterrors.NewErrCommunication("packet-out-send", log.Fields{
			"source":             "adapter",
			"destination":        "onu",
			"egress-port-number": egressPortNo,
			"intf-id":            intfID,
			"oni-id":             onuID,
			"uni-id":             uniID,
			"gem-port-id":        gemPortID,
			"packet":             hex.EncodeToString(packet.Data),
			"device-id":          dh.device.Id,
		}, err)
	}
	return nil
}

// PacketOut sends packet-out from VOLTHA to OLT on the egress port provided
func (dh *DeviceHandler) PacketOut(ctx context.Context, egressPortNo uint32, packet *of.OfpPacketOut) error {
	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "incoming-packet-out", log.Fields{
			"device-id":      dh.device.Id,
			"egress-port-no": egressPortNo,
			"pkt-length":     len(packet.Data),
			"packet":         hex.EncodeToString(packet.Data),
		})
	}

	egressPortType := plt.IntfIDToPortTypeName(uint32(egressPortNo))
	var err error
	if egressPortType == voltha.Port_ETHERNET_UNI {
		err = dh.PacketOutUNI(ctx, egressPortNo, packet)
	} else if egressPortType == voltha.Port_ETHERNET_NNI {
		err = dh.PacketOutNNI(ctx, egressPortNo, packet)
	} else {
		logger.Warnw(ctx, "packet-out-to-this-interface-type-not-implemented", log.Fields{
			"egress-port-no": egressPortNo,
			"egressPortType": egressPortType,
			"packet":         hex.EncodeToString(packet.Data),
			"device-id":      dh.device.Id,
		})
	}
	return err
}

func (dh *DeviceHandler) formOnuKey(intfID, onuID uint32) string {
	return "" + strconv.Itoa(int(intfID)) + "." + strconv.Itoa(int(onuID))
}

func startHeartbeatCheck(ctx context.Context, dh *DeviceHandler) {
	defer func() {
		dh.lockDevice.Lock()
		dh.isHeartbeatCheckActive = false
		dh.lockDevice.Unlock()
	}()

	dh.lockDevice.Lock()
	dh.isHeartbeatCheckActive = true
	dh.lockDevice.Unlock()

	// start the heartbeat check towards the OLT.
	var timerCheck *time.Timer
	dh.heartbeatSignature = dh.getHeartbeatSignature(ctx)

	for {
		heartbeatTimer := time.NewTimer(dh.openOLT.HeartbeatCheckInterval)
		select {
		case <-heartbeatTimer.C:
			ctxWithTimeout, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.openOLT.GrpcTimeoutInterval)
			if heartBeat, err := dh.Client.HeartbeatCheck(ctxWithTimeout, new(oop.Empty)); err != nil {
				logger.Warnw(ctx, "heartbeat-failed", log.Fields{"device-id": dh.device.Id})
				if timerCheck == nil {
					// start a after func, when expired will update the state to the core
					timerCheck = time.AfterFunc(dh.openOLT.HeartbeatFailReportInterval, func() { dh.updateStateUnreachable(ctx) })
				}
			} else {
				if timerCheck != nil {
					if timerCheck.Stop() {
						logger.Debugw(ctx, "got-heartbeat-within-timeout", log.Fields{"device-id": dh.device.Id})
					}
					timerCheck = nil
				}
				if dh.heartbeatSignature == 0 || dh.heartbeatSignature == heartBeat.HeartbeatSignature {
					if dh.heartbeatSignature == 0 {
						// First time the signature will be 0, update the signture to DB when not found.
						dh.updateHeartbeatSignature(ctx, heartBeat.HeartbeatSignature)
						dh.heartbeatSignature = heartBeat.HeartbeatSignature
					}
					logger.Infow(ctx, "heartbeat signature", log.Fields{"sign": dh.heartbeatSignature})

					dh.lockDevice.RLock()
					// Stop the read indication only if it the routine is active
					// The read indication would have already stopped due to failure on the gRPC stream following OLT going unreachable
					// Sending message on the 'stopIndication' channel again will cause the readIndication routine to immediately stop
					// on next execution of the readIndication routine.
					if !dh.isReadIndicationRoutineActive {
						// Start reading indications
						go func() {
							if err = dh.readIndications(ctx); err != nil {
								_ = olterrors.NewErrAdapter("indication-read-failure", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
							}
						}()
					}
					dh.lockDevice.RUnlock()
				} else {
					logger.Warn(ctx, "Heartbeat signature changed, OLT is rebooted. Cleaningup resources.")
					dh.updateHeartbeatSignature(ctx, heartBeat.HeartbeatSignature)
					dh.heartbeatSignature = heartBeat.HeartbeatSignature
					go dh.updateStateRebooted(ctx)
				}
			}
			cancel()
		case <-dh.stopHeartbeatCheck:
			logger.Debugw(ctx, "stopping-heartbeat-check", log.Fields{"device-id": dh.device.Id})
			return
		}
	}
}

func (dh *DeviceHandler) updateStateUnreachable(ctx context.Context) {
	device, err := dh.getDeviceFromCore(ctx, dh.device.Id)
	if err != nil || device == nil {
		// One case where we have seen core returning an error for GetDevice call is after OLT device delete.
		// After OLT delete, the adapter asks for OLT to reboot. When OLT is rebooted, shortly we loose heartbeat.
		// The 'startHeartbeatCheck' then asks the device to be marked unreachable towards the core, but the core
		// has already deleted the device and returns error. In this particular scenario, it is Ok because any necessary
		// cleanup in the adapter was already done during DeleteDevice API handler routine.
		_ = olterrors.NewErrNotFound("device", log.Fields{"device-id": dh.device.Id}, err).Log()
		// Immediately return, otherwise accessing a null 'device' struct would cause panic
		return
	}

	logger.Warnw(ctx, "update-state-unreachable", log.Fields{"device-id": dh.device.Id, "connect-status": device.ConnectStatus,
		"admin-state": device.AdminState, "oper-status": device.OperStatus})
	if device.ConnectStatus == voltha.ConnectStatus_REACHABLE {
		if err = dh.updateDeviceStateInCore(ctx, &ca.DeviceStateFilter{
			DeviceId:   dh.device.Id,
			OperStatus: voltha.OperStatus_UNKNOWN,
			ConnStatus: voltha.ConnectStatus_UNREACHABLE,
		}); err != nil {
			_ = olterrors.NewErrAdapter("device-state-update-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
		}
		/*
			if err = dh.updatePortsStateInCore(ctx, &ca.PortStateFilter{
				DeviceId:       dh.device.Id,
				PortTypeFilter: 0,
				OperStatus:     voltha.OperStatus_UNKNOWN,
			}); err != nil {
				_ = olterrors.NewErrAdapter("port-update-failed", log.Fields{"device-id": dh.device.Id}, err).Log()
			}
		*/

		// raise olt communication failure event
		raisedTs := time.Now().Unix()
		cloned := proto.Clone(device).(*voltha.Device)
		cloned.ConnectStatus = voltha.ConnectStatus_UNREACHABLE
		cloned.OperStatus = voltha.OperStatus_UNKNOWN
		dh.device = cloned // update local copy of the device
		go dh.eventMgr.oltCommunicationEvent(ctx, cloned, raisedTs)

		dh.lockDevice.RLock()
		// Stop the Stats collector
		if dh.isCollectorActive {
			dh.stopCollector <- true
		}
		// stop the heartbeat check routine
		if dh.isHeartbeatCheckActive {
			dh.stopHeartbeatCheck <- true
		}
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

func (dh *DeviceHandler) updateStateRebooted(ctx context.Context) {
	device, err := dh.getDeviceFromCore(ctx, dh.device.Id)
	if err != nil || device == nil {
		// One case where we have seen core returning an error for GetDevice call is after OLT device delete.
		// After OLT delete, the adapter asks for OLT to reboot. When OLT is rebooted, shortly we loose heartbeat.
		// The 'startHeartbeatCheck' then asks the device to be marked unreachable towards the core, but the core
		// has already deleted the device and returns error. In this particular scenario, it is Ok because any necessary
		// cleanup in the adapter was already done during DeleteDevice API handler routine.
		_ = olterrors.NewErrNotFound("device", log.Fields{"device-id": dh.device.Id}, err).Log()
		// Immediately return, otherwise accessing a null 'device' struct would cause panic
		return
	}
	// Starting the cleanup process
	dh.setDeviceDeletionInProgressFlag(true)

	logger.Warnw(ctx, "update-state-rebooted", log.Fields{"device-id": dh.device.Id, "connect-status": device.ConnectStatus,
		"admin-state": device.AdminState, "oper-status": device.OperStatus, "conn-status": voltha.ConnectStatus_UNREACHABLE})
	// First, stop the read indication and heartbeat check routines to prevent any delay
	// in cleanup, which could cause the heartbeat routine to restart the read indication.
	dh.lockDevice.RLock()
	// Stop the read indication only if it the routine is active
	// The read indication would have already stopped due to failure on the gRPC stream following OLT going unreachable
	// Sending message on the 'stopIndication' channel again will cause the readIndication routine to immediately stop
	// on next execution of the readIndication routine.
	if dh.isReadIndicationRoutineActive {
		dh.stopIndications <- true
	}
	dh.lockDevice.RUnlock()

	if err = dh.updateDeviceStateInCore(ctx, &ca.DeviceStateFilter{
		DeviceId:   dh.device.Id,
		OperStatus: voltha.OperStatus_REBOOTED,
		ConnStatus: voltha.ConnectStatus_REACHABLE,
	}); err != nil {
		_ = olterrors.NewErrAdapter("device-state-update-failed", log.Fields{"device-id": dh.device.Id}, err).LogAt(log.ErrorLevel)
	}

	//raise olt communication failure event
	raisedTs := time.Now().Unix()
	cloned := proto.Clone(device).(*voltha.Device)
	cloned.ConnectStatus = voltha.ConnectStatus_UNREACHABLE
	cloned.OperStatus = voltha.OperStatus_UNKNOWN
	dh.device = cloned // update local copy of the device
	go dh.eventMgr.oltCommunicationEvent(ctx, cloned, raisedTs)

	if err := dh.cleanupDeviceResources(ctx); err != nil {
		logger.Errorw(ctx, "failure-in-cleanup-device-resources", log.Fields{"device-id": dh.device.Id, "err": err})
	} else {
		logger.Debugw(ctx, "removed-device-from-Resource-manager-KV-store", log.Fields{"device-id": dh.device.Id})
	}

	dh.lockDevice.RLock()
	// Stop the Stats collector
	if dh.isCollectorActive {
		dh.stopCollector <- true
	}
	// stop the heartbeat check routine
	if dh.isHeartbeatCheckActive {
		dh.stopHeartbeatCheck <- true
	}
	dh.lockDevice.RUnlock()

	dh.StopAllFlowRoutines(ctx)

	// reset adapter reconcile flag
	dh.adapterPreviouslyConnected = false
	for {
		childDevices, err := dh.getChildDevicesFromCore(ctx, dh.device.Id)
		if err != nil || childDevices == nil {
			logger.Errorw(ctx, "Failed to get child devices from core", log.Fields{"deviceID": dh.device.Id})
			continue
		}
		if len(childDevices.Items) == 0 {
			logger.Infow(ctx, "All childDevices cleared from core, proceed with device init", log.Fields{"deviceID": dh.device.Id})
			break
		} else {
			logger.Warn(ctx, "Not all child devices are cleared, continuing to wait")
			time.Sleep(5 * time.Second)
		}
	}
	// Cleanup completed , reset the flag
	dh.setDeviceDeletionInProgressFlag(false)
	logger.Infow(ctx, "cleanup complete after reboot , moving to init", log.Fields{"deviceID": device.Id})
	dh.transitionMap.Handle(ctx, DeviceInit)
}

// EnableOnuSerialNumber to enable onu serial number
func (dh *DeviceHandler) EnableOnuSerialNumber(ctx context.Context, device *voltha.OnuSerialNumberOfOLTPon) error {
	logger.Debugw(ctx, "enable-onu-serial-number", log.Fields{"Device": dh.device, "onu-serial-number": device.SerialNumber, "port": device.Port.PortNo})
	onuSerialNumber := device.SerialNumber

	// fetch interfaceid from PortNo
	ponID := plt.PortNoToIntfID(device.Port.GetPortNo(), voltha.Port_PON_OLT)

	sn, err := dh.deStringifySerialNumber(onuSerialNumber)
	if err != nil {
		return olterrors.NewErrAdapter("failed-to-destringify-serial-number",
			log.Fields{
				"devicer-id":    dh.device.Id,
				"serial-number": onuSerialNumber}, err).Log()
	}

	onuIntf := &oop.InterfaceOnuSerialNumber{
		IntfId:          ponID,
		OnuSerialNumber: sn,
	}
	_, err = dh.Client.EnableOnuSerialNumber(ctx, onuIntf)

	if err != nil {
		logger.Errorw(ctx, "failed to enable onu serial number", log.Fields{"onudev": onuSerialNumber, "error": err})
		return olterrors.NewErrAdapter("onu-serial-number-enable-failed", log.Fields{
			"device-id":         dh.device.Id,
			"onu-serial-number": onuSerialNumber}, err)
	}
	return nil
}

// DisableOnuSerialNumber to disable onu serial number
func (dh *DeviceHandler) DisableOnuSerialNumber(ctx context.Context, device *voltha.OnuSerialNumberOfOLTPon) error {
	logger.Debugw(ctx, "disable-onu-serial-number", log.Fields{"Device": dh.device, "onu-serial-number": device.SerialNumber, "port": device.Port.PortNo})
	onuSerialNumber := device.SerialNumber
	ponID := plt.PortNoToIntfID(device.Port.GetPortNo(), voltha.Port_PON_OLT)
	sn, err := dh.deStringifySerialNumber(onuSerialNumber)
	if err != nil {
		return olterrors.NewErrAdapter("failed-to-destringify-serial-number",
			log.Fields{
				"devicer-id":    dh.device.Id,
				"serial-number": onuSerialNumber}, err).Log()
	}

	onuIntf := &oop.InterfaceOnuSerialNumber{
		OnuSerialNumber: sn,
		IntfId:          ponID,
	}
	_, err = dh.Client.DisableOnuSerialNumber(ctx, onuIntf)

	if err != nil {
		logger.Errorw(ctx, "failed to disable onu serial number", log.Fields{"onudev": onuSerialNumber, "error": err})
		return olterrors.NewErrAdapter("onu-serial-number-disable-failed", log.Fields{
			"device-id":         dh.device.Id,
			"onu-serial-number": onuSerialNumber}, err)
	}
	return nil
}

// EnableChildDevice to enable onu
func (dh *DeviceHandler) EnableChildDevice(ctx context.Context, device *voltha.Device) error {
	logger.Debugw(ctx, "enable-onu", log.Fields{"Device": dh.device, "onu-serial-number": device.SerialNumber})
	onuSerialNumber := device.SerialNumber
	InCacheOnuDev := dh.getChildDevice(ctx, onuSerialNumber, dh.device.ParentPortNo)
	if InCacheOnuDev == nil {
		logger.Errorw(ctx, "failed to get child device from cache", log.Fields{"onudev": onuSerialNumber})
		return olterrors.NewErrAdapter("failed to get child device from cache", log.Fields{
			"device-id":         dh.device.Id,
			"onu-serial-number": onuSerialNumber}, nil)
	}
	logger.Debugw(ctx, "successfully-received-child-device-from-cache", log.Fields{"child-device-intfid": InCacheOnuDev.intfID, "child-device-sn": InCacheOnuDev.serialNumber, "child-onuid": InCacheOnuDev.onuID})

	sn, err := dh.deStringifySerialNumber(onuSerialNumber)
	if err != nil {
		return olterrors.NewErrAdapter("failed-to-destringify-serial-number",
			log.Fields{
				"devicer-id":    dh.device.Id,
				"serial-number": onuSerialNumber}, err).Log()
	}

	onuIntf := &oop.InterfaceOnuSerialNumber{
		OnuSerialNumber: sn,
		IntfId:          InCacheOnuDev.intfID,
	}
	onuIntfReq := &oop.InterfaceOnuSerialNumberOnuId{
		OnuId:           InCacheOnuDev.onuID,
		IntfIdSerialNum: onuIntf,
	}
	_, err = dh.Client.EnableChildDevice(ctx, onuIntfReq)

	if err != nil {
		logger.Errorw(ctx, "failed to enable onu ", log.Fields{"onudev": onuSerialNumber, "error": err})
		return olterrors.NewErrAdapter("onu-enable-failed", log.Fields{
			"olt-device-id":     dh.device.Id,
			"onu-serial-number": onuSerialNumber}, err)
	}
	return nil
}

// DisableChildDevice to disable onu
func (dh *DeviceHandler) DisableChildDevice(ctx context.Context, device *voltha.Device) error {
	logger.Debugw(ctx, "disable-onu", log.Fields{"Device": dh.device, "onu-serial-number": device.SerialNumber})
	onuSerialNumber := device.SerialNumber
	InCacheOnuDev := dh.getChildDevice(ctx, onuSerialNumber, dh.device.ParentPortNo)
	if InCacheOnuDev == nil {
		logger.Errorw(ctx, "failed to get child device from cache", log.Fields{"onudev": onuSerialNumber})
		return olterrors.NewErrAdapter("failed to get child device from cache", log.Fields{
			"device-id":         dh.device.Id,
			"onu-serial-number": onuSerialNumber}, nil)
	}
	logger.Debugw(ctx, "successfully-received-child-device-from-cache", log.Fields{"child-device-intfid": InCacheOnuDev.intfID, "child-device-sn": InCacheOnuDev.serialNumber, "child-onuid": InCacheOnuDev.onuID})

	sn, err := dh.deStringifySerialNumber(onuSerialNumber)
	if err != nil {
		return olterrors.NewErrAdapter("failed-to-destringify-serial-number",
			log.Fields{
				"devicer-id":    dh.device.Id,
				"serial-number": onuSerialNumber}, err).Log()
	}

	onuIntf := &oop.InterfaceOnuSerialNumber{
		OnuSerialNumber: sn,
		IntfId:          InCacheOnuDev.intfID,
	}
	onuIntfReq := &oop.InterfaceOnuSerialNumberOnuId{
		OnuId:           InCacheOnuDev.onuID,
		IntfIdSerialNum: onuIntf,
	}
	_, err = dh.Client.DisableChildDevice(ctx, onuIntfReq)

	if err != nil {
		logger.Errorw(ctx, "failed to disable onu ", log.Fields{"onudev": onuSerialNumber, "error": err})
		return olterrors.NewErrAdapter("onu-disable-failed", log.Fields{
			"olt-device-id":     dh.device.Id,
			"onu-serial-number": onuSerialNumber}, err)
	}
	return nil
}

// EnablePort to enable Pon interface
func (dh *DeviceHandler) EnablePort(ctx context.Context, port *voltha.Port) error {
	logger.Debugw(ctx, "enable-port", log.Fields{"Device": dh.device, "port": port})
	return dh.modifyPhyPort(ctx, port, true)
}

// DisablePort to disable pon interface
func (dh *DeviceHandler) DisablePort(ctx context.Context, port *voltha.Port) error {
	logger.Debugw(ctx, "disable-port", log.Fields{"Device": dh.device, "port": port})
	return dh.modifyPhyPort(ctx, port, false)
}

// modifyPhyPort is common function to enable and disable the port. parm :enablePort, true to enablePort and false to disablePort.
func (dh *DeviceHandler) modifyPhyPort(ctx context.Context, port *voltha.Port, enablePort bool) error {
	logger.Infow(ctx, "modifyPhyPort", log.Fields{"port": port, "Enable": enablePort, "device-id": dh.device.Id})
	if port.GetType() == voltha.Port_ETHERNET_NNI {
		// Bug is opened for VOL-2505 to support NNI disable feature.
		logger.Infow(ctx, "voltha-supports-single-nni-hence-disable-of-nni-not-allowed",
			log.Fields{"device": dh.device, "port": port})
		return olterrors.NewErrAdapter("illegal-port-request", log.Fields{
			"port-type":    port.GetType,
			"enable-state": enablePort}, nil)
	}
	// fetch interfaceid from PortNo
	ponID := plt.PortNoToIntfID(port.GetPortNo(), voltha.Port_PON_OLT)
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
		logger.Infow(ctx, "enabled-pon-port", log.Fields{"out": out, "device-id": dh.device, "Port": port})
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
		logger.Infow(ctx, "disabled-pon-port", log.Fields{"out": out, "device-id": dh.device, "Port": port})
	}
	if err := dh.updatePortStateInCore(ctx, &ca.PortState{
		DeviceId:   dh.device.Id,
		PortType:   voltha.Port_PON_OLT,
		PortNo:     port.PortNo,
		OperStatus: operStatus,
	}); err != nil {
		return olterrors.NewErrAdapter("port-state-update-failed", log.Fields{
			"device-id": dh.device.Id,
			"port":      port.PortNo}, err)
	}
	return nil
}

// disableAdminDownPorts disables the ports, if the corresponding port Adminstate is disabled on reboot and Renable device.
func (dh *DeviceHandler) disableAdminDownPorts(ctx context.Context, ports []*voltha.Port) error {
	// Disable the port and update the oper_port_status to core
	// if the Admin state of the port is disabled on reboot and re-enable device.
	for _, port := range ports {
		if port.AdminState == common.AdminState_DISABLED {
			if err := dh.DisablePort(ctx, port); err != nil {
				return olterrors.NewErrAdapter("port-disable-failed", log.Fields{
					"device-id": dh.device.Id,
					"port":      port}, err)
			}
		}
	}
	return nil
}

// populateActivePorts to populate activePorts map
func (dh *DeviceHandler) populateActivePorts(ctx context.Context, ports []*voltha.Port) {
	logger.Infow(ctx, "populateActivePorts", log.Fields{"device-id": dh.device.Id})
	for _, port := range ports {
		if port.Type == voltha.Port_ETHERNET_NNI {
			if port.OperStatus == voltha.OperStatus_ACTIVE {
				dh.activePorts.Store(plt.PortNoToIntfID(port.PortNo, voltha.Port_ETHERNET_NNI), true)
			} else {
				dh.activePorts.Store(plt.PortNoToIntfID(port.PortNo, voltha.Port_ETHERNET_NNI), false)
			}
		}
		if port.Type == voltha.Port_PON_OLT {
			if port.OperStatus == voltha.OperStatus_ACTIVE {
				dh.activePorts.Store(plt.PortNoToIntfID(port.PortNo, voltha.Port_PON_OLT), true)
			} else {
				dh.activePorts.Store(plt.PortNoToIntfID(port.PortNo, voltha.Port_PON_OLT), false)
			}
		}
	}
}

// ChildDeviceLost deletes ONU and clears pon resources related to it.
func (dh *DeviceHandler) ChildDeviceLost(ctx context.Context, pPortNo uint32, onuID uint32, onuSn string) error {
	logger.Debugw(ctx, "child-device-lost", log.Fields{"parent-device-id": dh.device.Id})
	if dh.getDeviceDeletionInProgressFlag() {
		// Given that the OLT device itself is getting deleted, everything will be cleaned up in the DB and the OLT
		// will reboot, so everything will be reset on the pOLT too.
		logger.Infow(ctx, "olt-device-delete-in-progress-not-handling-child-device-lost",
			log.Fields{"parent-device-id": dh.device.Id, "pon-port": pPortNo, "onuID": onuID, "onuSN": onuSn})
		return nil
	}

	if dh.transitionMap.currentDeviceState != deviceStateUp {
		logger.Warnw(ctx, "device-is-not-up--not-handling-child-device-lost", log.Fields{"device-id": dh.device.Id, "current-device-state": dh.transitionMap.currentDeviceState})
		return fmt.Errorf("device-is-not-up--not-handling-child-device-lost")
	}

	intfID := plt.PortNoToIntfID(pPortNo, voltha.Port_PON_OLT)
	onuKey := dh.formOnuKey(intfID, onuID)

	var sn *oop.SerialNumber
	var err error
	if sn, err = dh.deStringifySerialNumber(onuSn); err != nil {
		return olterrors.NewErrAdapter("failed-to-destringify-serial-number",
			log.Fields{
				"devicer-id":    dh.device.Id,
				"serial-number": onuSn}, err).Log()
	}

	onu := &oop.Onu{IntfId: intfID, OnuId: onuID, SerialNumber: sn}
	// clear PON resources associated with ONU
	onuGem, err := dh.resourceMgr[intfID].GetOnuGemInfo(ctx, onuID)
	if err != nil || onuGem == nil || onuGem.OnuID != onuID {
		logger.Warnw(ctx, "failed-to-get-onu-info-for-pon-port", log.Fields{
			"device-id": dh.device.Id,
			"intf-id":   intfID,
			"onuID":     onuID,
			"err":       err})
	} else {
		logger.Debugw(ctx, "onu-data", log.Fields{"onu": onu})
		// Delete flows from device before schedulers and queue
		// Clear flowids for gem cache.
		removedFlows := []uint64{}
		for _, gem := range onuGem.GemPorts {
			if flowIDs, err := dh.resourceMgr[intfID].GetFlowIDsForGem(ctx, gem); err == nil {
				for _, flowID := range flowIDs {
					// multiple gem port can have the same flow id
					// it is better to send only one flowRemove request to the agent
					var alreadyRemoved bool
					for _, removedFlowID := range removedFlows {
						if removedFlowID == flowID {
							logger.Debugw(ctx, "flow-is-already-removed-due-to-another-gem", log.Fields{"flowID": flowID})
							alreadyRemoved = true
							break
						}
					}
					if !alreadyRemoved {
						dh.removeFlowFromDevice(ctx, flowID, intfID)
						removedFlows = appendUnique64bit(removedFlows, flowID)
					}
				}
			}
			_ = dh.resourceMgr[intfID].DeleteFlowIDsForGem(ctx, gem)
		}
		if err := dh.clearUNIData(ctx, onuGem); err != nil {
			logger.Warnw(ctx, "failed-to-clear-uni-data-for-onu", log.Fields{
				"device-id":  dh.device.Id,
				"onu-device": onu,
				"err":        err})
		}
		if err := dh.resourceMgr[intfID].DelOnuGemInfo(ctx, onuID); err != nil {
			logger.Warnw(ctx, "persistence-update-onu-gem-info-failed", log.Fields{
				"intf-id":    intfID,
				"onu-device": onu,
				"onu-gem":    onuGem,
				"err":        err})
			// Not returning error on cleanup.
		}
		logger.Debugw(ctx, "removed-onu-gem-info", log.Fields{"intf": intfID, "onu-device": onu, "onugem": onuGem})
	}
	dh.resourceMgr[intfID].FreeonuID(ctx, []uint32{onuID})
	dh.onus.Delete(onuKey)
	dh.discOnus.Delete(onuSn)

	// Now clear the ONU on the OLT
	if _, err := dh.Client.DeleteOnu(log.WithSpanFromContext(context.Background(), ctx), onu); err != nil {
		return olterrors.NewErrAdapter("failed-to-delete-onu", log.Fields{
			"device-id": dh.device.Id,
			"onu-id":    onuID}, err).Log()
	}

	return nil
}
func (dh *DeviceHandler) removeFlowFromDevice(ctx context.Context, flowID uint64, intfID uint32) {
	flow := &oop.Flow{FlowId: flowID}
	if dh.flowMgr == nil || dh.flowMgr[intfID] == nil {
		logger.Warnw(ctx, "failed-to-get-flow-mgr-to-remove-flow-from-device", log.Fields{
			"device-id": dh.device.Id})
	} else {
		if err := dh.flowMgr[intfID].removeFlowFromDevice(ctx, flow, flowID); err != nil {
			logger.Warnw(ctx, "failed-to-remove-flow-from-device", log.Fields{
				"device-id": dh.device.Id,
				"err":       err})
		}
	}
}

func getInPortFromFlow(flow *of.OfpFlowStats) uint32 {
	for _, field := range flow_utils.GetOfbFields(flow) {
		if field.Type == flow_utils.IN_PORT {
			return field.GetPort()
		}
	}
	return InvalidPort
}

func getOutPortFromFlow(flow *of.OfpFlowStats) uint32 {
	for _, action := range flow_utils.GetActions(flow) {
		if action.Type == flow_utils.OUTPUT {
			if out := action.GetOutput(); out != nil {
				return out.GetPort()
			}
		}
	}
	return InvalidPort
}

func getPorts(flow *of.OfpFlowStats) (uint32, uint32) {
	inPort := getInPortFromFlow(flow)
	outPort := getOutPortFromFlow(flow)

	if inPort == InvalidPort || outPort == InvalidPort {
		return inPort, outPort
	}

	if isControllerFlow := plt.IsControllerBoundFlow(outPort); isControllerFlow {
		/* Get UNI port/ IN Port from tunnel ID field for upstream controller bound flows  */
		if portType := plt.IntfIDToPortTypeName(inPort); portType == voltha.Port_PON_OLT {
			if uniPort := flow_utils.GetChildPortFromTunnelId(flow); uniPort != 0 {
				return uniPort, outPort
			}
		}
	} else {
		// Downstream flow from NNI to PON port , Use tunnel ID as new OUT port / UNI port
		if portType := plt.IntfIDToPortTypeName(outPort); portType == voltha.Port_PON_OLT {
			if uniPort := flow_utils.GetChildPortFromTunnelId(flow); uniPort != 0 {
				return inPort, uniPort
			}
			// Upstream flow from PON to NNI port , Use tunnel ID as new IN port / UNI port
		} else if portType := plt.IntfIDToPortTypeName(inPort); portType == voltha.Port_PON_OLT {
			if uniPort := flow_utils.GetChildPortFromTunnelId(flow); uniPort != 0 {
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

func (dh *DeviceHandler) getExtValue(ctx context.Context, device *voltha.Device, value extension.ValueType_Type) (*extension.ReturnValues, error) {
	var err error
	var sn *oop.SerialNumber
	var ID uint32
	resp := new(extension.ReturnValues)
	valueparam := new(oop.ValueParam)
	ctx = log.WithSpanFromContext(context.Background(), ctx)
	logger.Infow(ctx, "getExtValue", log.Fields{"onu-id": device.Id, "pon-intf": device.ParentPortNo})
	if sn, err = dh.deStringifySerialNumber(device.SerialNumber); err != nil {
		return nil, err
	}
	ID = device.ProxyAddress.GetOnuId()
	Onu := oop.Onu{IntfId: device.ParentPortNo, OnuId: ID, SerialNumber: sn}
	valueparam.Onu = &Onu
	valueparam.Value = value

	// This API is unsupported until agent patch is added
	resp.Unsupported = uint32(value)
	_ = ctx

	// Uncomment this code once agent changes are complete and tests
	/*
		resp, err = dh.Client.GetValue(ctx, valueparam)
		if err != nil {
			logger.Errorw("error-while-getValue", log.Fields{"DeviceID": dh.device, "onu-id": onuid, "err": err})
			return nil, err
		}
	*/

	logger.Infow(ctx, "get-ext-value", log.Fields{"resp": resp, "device-id": dh.device, "onu-id": device.Id, "pon-intf": device.ParentPortNo})
	return resp, nil
}

func (dh *DeviceHandler) getIntfIDFromFlow(flow *of.OfpFlowStats) uint32 {
	// Default to NNI
	var intfID = dh.totalPonPorts
	inPort, outPort := getPorts(flow)
	if inPort != InvalidPort && outPort != InvalidPort {
		_, intfID, _, _ = plt.ExtractAccessFromFlow(inPort, outPort)
	}
	return intfID
}

func (dh *DeviceHandler) getOnuIndicationChannel(intfID uint32) chan onuIndicationMsg {
	dh.perPonOnuIndicationChannelLock.Lock()
	if ch, ok := dh.perPonOnuIndicationChannel[intfID]; ok {
		dh.perPonOnuIndicationChannelLock.Unlock()
		return ch.indicationChannel
	}
	channels := onuIndicationChannels{
		// We create a buffered channel here to avoid calling function to be  blocked
		// in case of multiple indications from the ONUs,
		//especially in the case where indications are buffered in  OLT.
		indicationChannel: make(chan onuIndicationMsg, 500),
		stopChannel:       make(chan struct{}),
	}
	dh.perPonOnuIndicationChannel[intfID] = channels
	dh.perPonOnuIndicationChannelLock.Unlock()
	go dh.onuIndicationsRoutine(&channels)
	return channels.indicationChannel
}

func (dh *DeviceHandler) removeOnuIndicationChannels(ctx context.Context) {
	logger.Debug(ctx, "remove-onu-indication-channels", log.Fields{"device-id": dh.device.Id})
	dh.perPonOnuIndicationChannelLock.Lock()
	defer dh.perPonOnuIndicationChannelLock.Unlock()
	for _, v := range dh.perPonOnuIndicationChannel {
		close(v.stopChannel)
	}
	dh.perPonOnuIndicationChannel = make(map[uint32]onuIndicationChannels)
}

func (dh *DeviceHandler) putOnuIndicationToChannel(ctx context.Context, indication *oop.Indication, intfID uint32) {
	ind := onuIndicationMsg{
		ctx:        ctx,
		indication: indication,
	}
	logger.Debugw(ctx, "put-onu-indication-to-channel", log.Fields{"indication": indication, "intfID": intfID})
	// Send the onuIndication on the ONU channel
	dh.getOnuIndicationChannel(intfID) <- ind
}

func (dh *DeviceHandler) onuIndicationsRoutine(onuChannels *onuIndicationChannels) {
	for {
		select {
		// process one indication per onu, before proceeding to the next one
		case onuInd := <-onuChannels.indicationChannel:
			indication := *(proto.Clone(onuInd.indication)).(*oop.Indication)
			logger.Debugw(onuInd.ctx, "calling-indication", log.Fields{"device-id": dh.device.Id,
				"ind": indication})
			switch indication.Data.(type) {
			case *oop.Indication_OnuInd:
				if err := dh.onuIndication(onuInd.ctx, indication.GetOnuInd()); err != nil {
					_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{
						"type":      "onu-indication",
						"device-id": dh.device.Id}, err).Log()
				}
			case *oop.Indication_OnuDiscInd:
				if err := dh.onuDiscIndication(onuInd.ctx, indication.GetOnuDiscInd()); err != nil {
					_ = olterrors.NewErrAdapter("handle-indication-error", log.Fields{
						"type":      "onu-discovery",
						"device-id": dh.device.Id}, err).Log()
				}
			}
		case <-onuChannels.stopChannel:
			logger.Debugw(context.Background(), "stop-signal-received-for-onu-channel", log.Fields{"device-id": dh.device.Id})
			close(onuChannels.indicationChannel)
			return
		}
	}
}

// RouteMcastFlowOrGroupMsgToChannel routes incoming mcast flow or group to a channel to be handled by the a specific
// instance of mcastFlowOrGroupChannelHandlerRoutine meant to handle messages for that group.
func (dh *DeviceHandler) RouteMcastFlowOrGroupMsgToChannel(ctx context.Context, flow *of.OfpFlowStats, group *of.OfpGroupEntry, action string) error {
	if dh.getDeviceDeletionInProgressFlag() {
		// The device itself is going to be reset as part of deletion. So nothing to be done.
		logger.Infow(ctx, "device-deletion-in-progress--not-handling-flows-or-groups", log.Fields{"device-id": dh.device.Id})
		return nil
	}

	// Step1 : Fill McastFlowOrGroupControlBlock
	// Step2 : Push the McastFlowOrGroupControlBlock to appropriate channel
	// Step3 : Wait on response channel for response
	// Step4 : Return error value
	startTime := time.Now()
	logger.Debugw(ctx, "process-flow-or-group", log.Fields{"flow": flow, "group": group, "action": action})
	errChan := make(chan error)
	var groupID uint32
	mcastFlowOrGroupCb := McastFlowOrGroupControlBlock{
		ctx:               ctx,
		flowOrGroupAction: action,
		flow:              flow,
		group:             group,
		errChan:           &errChan,
	}
	if flow != nil {
		groupID = flow_utils.GetGroup(flow)
	} else if group != nil {
		groupID = group.Desc.GroupId
	} else {
		return errors.New("flow-and-group-both-nil")
	}
	mcastRoutineIdx := groupID % MaxNumOfGroupHandlerChannels
	if dh.mcastHandlerRoutineActive[mcastRoutineIdx] {
		// Derive the appropriate go routine to handle the request by a simple module operation.
		// There are only MaxNumOfGroupHandlerChannels number of channels to handle the mcast flow or group
		dh.incomingMcastFlowOrGroup[groupID%MaxNumOfGroupHandlerChannels] <- mcastFlowOrGroupCb
		// Wait for handler to return error value
		err := <-errChan
		logger.Debugw(ctx, "process-flow-or-group--received-resp", log.Fields{"err": err, "totalTimeInSeconds": time.Since(startTime).Milliseconds()})
		return err
	}
	logger.Errorw(ctx, "mcast handler routine not active for onu", log.Fields{"mcastRoutineIdx": mcastRoutineIdx})
	return fmt.Errorf("mcast-handler-routine-not-active-for-index-%v", mcastRoutineIdx)
}

// mcastFlowOrGroupChannelHandlerRoutine routine to handle incoming mcast flow/group message
func (dh *DeviceHandler) mcastFlowOrGroupChannelHandlerRoutine(routineIndex int, mcastFlowOrGroupChannel chan McastFlowOrGroupControlBlock, stopHandler chan bool) {
	for {
		select {
		// block on the channel to receive an incoming mcast flow/group
		// process the flow completely before proceeding to handle the next flow
		case mcastFlowOrGroupCb := <-mcastFlowOrGroupChannel:
			if mcastFlowOrGroupCb.flow != nil {
				if mcastFlowOrGroupCb.flowOrGroupAction == McastFlowOrGroupAdd {
					logger.Debugw(mcastFlowOrGroupCb.ctx, "adding-mcast-flow",
						log.Fields{"device-id": dh.device.Id,
							"flowToAdd": mcastFlowOrGroupCb.flow})
					err := olterrors.NewErrNotFound("no-flow-manager-found-to-add-mcast-flow", log.Fields{"device-id": dh.device.Id}, nil).(error)
					// The mcast flow is not unique to any particular PON port, so it is OK to default to first non nil PON
					for _, flMgr := range dh.flowMgr {
						if flMgr != nil {
							err = flMgr.AddFlow(mcastFlowOrGroupCb.ctx, mcastFlowOrGroupCb.flow, nil)
							break
						}
					}
					// Pass the return value over the return channel
					*mcastFlowOrGroupCb.errChan <- err
				} else { // flow remove
					logger.Debugw(mcastFlowOrGroupCb.ctx, "removing-mcast-flow",
						log.Fields{"device-id": dh.device.Id,
							"flowToRemove": mcastFlowOrGroupCb.flow})
					// The mcast flow is not unique to any particular PON port, so it is OK to default to first non nil PON
					err := olterrors.NewErrNotFound("no-flow-manager-found-to-remove-mcast-flow", log.Fields{"device-id": dh.device.Id}, nil).(error)
					for _, flMgr := range dh.flowMgr {
						if flMgr != nil {
							err = flMgr.RemoveFlow(mcastFlowOrGroupCb.ctx, mcastFlowOrGroupCb.flow)
							break
						}
					}
					// Pass the return value over the return channel
					*mcastFlowOrGroupCb.errChan <- err
				}
			} else { // mcast group
				if mcastFlowOrGroupCb.flowOrGroupAction == McastFlowOrGroupAdd {
					logger.Debugw(mcastFlowOrGroupCb.ctx, "adding-mcast-group",
						log.Fields{"device-id": dh.device.Id,
							"groupToAdd": mcastFlowOrGroupCb.group})
					err := dh.groupMgr.AddGroup(mcastFlowOrGroupCb.ctx, mcastFlowOrGroupCb.group)
					// Pass the return value over the return channel
					*mcastFlowOrGroupCb.errChan <- err
				} else if mcastFlowOrGroupCb.flowOrGroupAction == McastFlowOrGroupModify { // group modify
					logger.Debugw(mcastFlowOrGroupCb.ctx, "modifying-mcast-group",
						log.Fields{"device-id": dh.device.Id,
							"groupToModify": mcastFlowOrGroupCb.group})
					err := dh.groupMgr.ModifyGroup(mcastFlowOrGroupCb.ctx, mcastFlowOrGroupCb.group)
					// Pass the return value over the return channel
					*mcastFlowOrGroupCb.errChan <- err
				} else { // group remove
					logger.Debugw(mcastFlowOrGroupCb.ctx, "removing-mcast-group",
						log.Fields{"device-id": dh.device.Id,
							"groupToRemove": mcastFlowOrGroupCb.group})
					err := dh.groupMgr.DeleteGroup(mcastFlowOrGroupCb.ctx, mcastFlowOrGroupCb.group)
					// Pass the return value over the return channel
					*mcastFlowOrGroupCb.errChan <- err
				}
			}
		case <-stopHandler:
			dh.mcastHandlerRoutineActive[routineIndex] = false
			return
		}
	}
}

// StopAllMcastHandlerRoutines stops all flow handler routines. Call this when device is being rebooted or deleted
func (dh *DeviceHandler) StopAllMcastHandlerRoutines(ctx context.Context, wg *sync.WaitGroup) {
	for i, v := range dh.stopMcastHandlerRoutine {
		if dh.mcastHandlerRoutineActive[i] {
			select {
			case v <- true:
			case <-time.After(time.Second * 5):
				logger.Warnw(ctx, "timeout stopping mcast handler routine", log.Fields{"idx": i, "deviceID": dh.device.Id})
			}
		}
	}

	if dh.incomingMcastFlowOrGroup != nil {
		for k := range dh.incomingMcastFlowOrGroup {
			if dh.incomingMcastFlowOrGroup[k] != nil {
				dh.incomingMcastFlowOrGroup[k] = nil
			}
		}
		dh.incomingMcastFlowOrGroup = nil
	}

	wg.Done()
	logger.Debug(ctx, "stopped all mcast handler routines")
}

// nolint: unparam
func (dh *DeviceHandler) getOltPortCounters(ctx context.Context, oltPortInfo *extension.GetOltPortCounters) *extension.SingleGetValueResponse {
	singleValResp := extension.SingleGetValueResponse{
		Response: &extension.GetValueResponse{
			Response: &extension.GetValueResponse_PortCoutners{
				PortCoutners: &extension.GetOltPortCountersResponse{},
			},
		},
	}

	errResp := func(status extension.GetValueResponse_Status, reason extension.GetValueResponse_ErrorReason) *extension.SingleGetValueResponse {
		return &extension.SingleGetValueResponse{
			Response: &extension.GetValueResponse{
				Status:    status,
				ErrReason: reason,
			},
		}
	}

	if oltPortInfo.PortType != extension.GetOltPortCounters_Port_ETHERNET_NNI &&
		oltPortInfo.PortType != extension.GetOltPortCounters_Port_PON_OLT {
		// send error response
		logger.Debugw(ctx, "getOltPortCounters invalid portType", log.Fields{"oltPortInfo": oltPortInfo.PortType})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_PORT_TYPE)
	}
	statIndChn := make(chan bool, 1)
	dh.portStats.RegisterForStatIndication(ctx, portStatsType, statIndChn, oltPortInfo.PortNo, oltPortInfo.PortType)
	defer dh.portStats.DeRegisterFromStatIndication(ctx, portStatsType, statIndChn)
	// request openOlt agent to send the the port statistics indication

	go func() {
		_, err := dh.Client.CollectStatistics(ctx, new(oop.Empty))
		if err != nil {
			logger.Errorw(ctx, "getOltPortCounters CollectStatistics failed ", log.Fields{"err": err})
		}
	}()
	select {
	case <-statIndChn:
		// indication received for ports stats
		logger.Debugw(ctx, "getOltPortCounters recvd statIndChn", log.Fields{"oltPortInfo": oltPortInfo})
	case <-time.After(oltPortInfoTimeout * time.Second):
		logger.Debugw(ctx, "getOltPortCounters timeout happened", log.Fields{"oltPortInfo": oltPortInfo})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_TIMEOUT)
	case <-ctx.Done():
		logger.Debugw(ctx, "getOltPortCounters ctx Done ", log.Fields{"oltPortInfo": oltPortInfo})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_TIMEOUT)
	}
	if oltPortInfo.PortType == extension.GetOltPortCounters_Port_ETHERNET_NNI {
		// get nni stats
		intfID := plt.PortNoToIntfID(oltPortInfo.PortNo, voltha.Port_ETHERNET_NNI)
		logger.Debugw(ctx, "getOltPortCounters intfID  ", log.Fields{"intfID": intfID})
		cmnni := dh.portStats.collectNNIMetrics(intfID)
		if cmnni == nil {
			// TODO define the error reason
			return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INTERNAL_ERROR)
		}
		dh.portStats.updateGetOltPortCountersResponse(ctx, &singleValResp, cmnni)
		return &singleValResp
	} else if oltPortInfo.PortType == extension.GetOltPortCounters_Port_PON_OLT {
		// get pon stats
		intfID := plt.PortNoToIntfID(oltPortInfo.PortNo, voltha.Port_PON_OLT)
		if val, ok := dh.activePorts.Load(intfID); ok && val == true {
			cmpon := dh.portStats.collectPONMetrics(intfID)
			if cmpon == nil {
				// TODO define the error reason
				return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INTERNAL_ERROR)
			}
			dh.portStats.updateGetOltPortCountersResponse(ctx, &singleValResp, cmpon)
			return &singleValResp
		}
	}
	return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INTERNAL_ERROR)
}

//nolint:unparam
func (dh *DeviceHandler) getOltOffloadStats(ctx context.Context, oltPortInfo *extension.GetOffloadedAppsStatisticsRequest) *extension.SingleGetValueResponse {
	singleValResp := extension.SingleGetValueResponse{
		Response: &extension.GetValueResponse{
			Status: extension.GetValueResponse_OK,
			Response: &extension.GetValueResponse_OffloadedAppsStats{
				OffloadedAppsStats: &extension.GetOffloadedAppsStatisticsResponse{},
			},
		},
	}

	return &singleValResp
}

//nolint:unparam
func (dh *DeviceHandler) setOltOffload(ctx context.Context, congig *extension.AppOffloadConfig) *extension.SingleSetValueResponse {
	singleValResp := extension.SingleSetValueResponse{
		Response: &extension.SetValueResponse{
			Status: extension.SetValueResponse_OK,
		},
	}

	return &singleValResp
}

//nolint:unparam
func (dh *DeviceHandler) setOnuOffload(ctx context.Context, config *extension.AppOffloadOnuConfig) *extension.SingleSetValueResponse {
	singleValResp := extension.SingleSetValueResponse{
		Response: &extension.SetValueResponse{
			Status: extension.SetValueResponse_OK,
		},
	}

	return &singleValResp
}

func (dh *DeviceHandler) getOnuPonCounters(ctx context.Context, onuPonInfo *extension.GetOnuCountersRequest) *extension.SingleGetValueResponse {
	singleValResp := extension.SingleGetValueResponse{
		Response: &extension.GetValueResponse{
			Response: &extension.GetValueResponse_OnuPonCounters{
				OnuPonCounters: &extension.GetOnuCountersResponse{},
			},
		},
	}

	errResp := func(status extension.GetValueResponse_Status, reason extension.GetValueResponse_ErrorReason) *extension.SingleGetValueResponse {
		return &extension.SingleGetValueResponse{
			Response: &extension.GetValueResponse{
				Status:    status,
				ErrReason: reason,
			},
		}
	}
	intfID := onuPonInfo.IntfId
	onuID := onuPonInfo.OnuId
	onuKey := dh.formOnuKey(intfID, onuID)

	if _, ok := dh.onus.Load(onuKey); !ok {
		logger.Errorw(ctx, "get-onui-pon-counters-request-invalid-request-received", log.Fields{"intfID": intfID, "onuID": onuID})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_DEVICE)
	}
	logger.Debugw(ctx, "get-onui-pon-counters-request-received", log.Fields{"intfID": intfID, "onuID": onuID})
	cmnni := dh.portStats.collectOnDemandOnuStats(ctx, intfID, onuID)
	if cmnni == nil {
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INTERNAL_ERROR)
	}
	dh.portStats.updateGetOnuPonCountersResponse(ctx, &singleValResp, cmnni)
	return &singleValResp
}

func (dh *DeviceHandler) CheckOnuGemInfo(ctx context.Context, intfID uint32, onuID uint32) (*rsrcMgr.OnuGemInfo, error) {
	onuGemInfo, err := dh.resourceMgr[intfID].GetOnuGemInfo(ctx, onuID)
	if err != nil {
		logger.Errorw(ctx, "Unable to find onuGemInfo", log.Fields{"onuID": onuID})
		return nil, err
	}
	if onuGemInfo != nil {
		if len(onuGemInfo.UniPorts) == 0 {
			logger.Errorw(ctx, "No uniports present for the ONU", log.Fields{"onuID": onuID})
			return nil, err
		}
		logger.Debugw(ctx, "Received onuGemInfo", log.Fields{"onuID": onuID, "onuGemInfo": onuGemInfo})
		return onuGemInfo, nil
	}
	logger.Errorw(ctx, "Received nil onuGemInfo", log.Fields{"onuID": onuID})
	return nil, fmt.Errorf("onuGemInfo received as null, onuID : %d", onuID)
}

func (dh *DeviceHandler) getAllocGemStats(ctx context.Context, intfID uint32, onuID uint32, onuGemInfo *rsrcMgr.OnuGemInfo, singleValResp *extension.SingleGetValueResponse) error {
	var err error
	var allocStats *oop.OnuAllocIdStatistics
	var onuGemStats *oop.GemPortStatistics
	for _, uni := range onuGemInfo.UniPorts {
		uniID := plt.UniIDFromPortNum(uni)
		uniPath := getUniPortPath(dh.device.Id, intfID, int32(onuID), int32(uniID))
		tpIDs := dh.resourceMgr[intfID].GetTechProfileIDForOnu(ctx, onuID, uniID)
		if len(tpIDs) == 0 {
			logger.Errorw(ctx, "No TPIDs present for the Uni", log.Fields{"onuID": onuID, "uniPort": uni})
			continue
		}
		for _, tpId := range tpIDs {
			tpPath := dh.flowMgr[intfID].getTPpath(ctx, uniPath, uint32(tpId))
			techProfileInstance, _ := dh.flowMgr[intfID].techprofile.GetTPInstance(ctx, tpPath)
			onuStatsFromOlt := extension.OnuAllocGemStatsFromOltResponse{}
			if techProfileInstance != nil {
				switch tpInst := techProfileInstance.(type) {
				case *tp_pb.TechProfileInstance:
					allocId := tpInst.UsScheduler.AllocId
					onuAllocPkt := oop.OnuPacket{IntfId: intfID, OnuId: onuID, AllocId: allocId}
					allocStats, err = dh.Client.GetAllocIdStatistics(ctx, &onuAllocPkt)
					if err != nil {
						logger.Errorw(ctx, "Error received from openolt for GetAllocIdStatistics", log.Fields{"onuID": onuID, "AllodId": allocId, "Error": err})
						return err
					}
					allocIdInfo := extension.OnuAllocIdInfoFromOlt{}
					allocIdInfo.AllocId = allocStats.AllocId
					allocIdInfo.RxBytes = allocStats.RxBytes

					onuStatsFromOlt.AllocIdInfo = &allocIdInfo

					gemPorts := tpInst.UpstreamGemPortAttributeList
					for _, gem := range gemPorts {
						onuGemPkt := oop.OnuPacket{IntfId: intfID, OnuId: onuID, GemportId: gem.GemportId}
						onuGemStats, err = dh.Client.GetGemPortStatistics(ctx, &onuGemPkt)
						if err != nil {
							logger.Errorw(ctx, "Error received from openolt for GetGemPortStatistics", log.Fields{"onuID": onuID, "GemPortId": gem.GemportId, "Error": err})
							return err
						}
						gemStatsInfo := extension.OnuGemPortInfoFromOlt{}
						gemStatsInfo.GemId = onuGemStats.GemportId
						gemStatsInfo.RxBytes = onuGemStats.RxBytes
						gemStatsInfo.RxPackets = onuGemStats.RxPackets
						gemStatsInfo.TxBytes = onuGemStats.TxBytes
						gemStatsInfo.TxPackets = onuGemStats.TxPackets

						onuStatsFromOlt.GemPortInfo = append(onuStatsFromOlt.GemPortInfo, &gemStatsInfo)
					}
					singleValResp.Response.GetOnuStatsFromOltResponse().AllocGemStatsInfo = append(singleValResp.Response.GetOnuStatsFromOltResponse().AllocGemStatsInfo, &onuStatsFromOlt)

				default:
					logger.Errorw(ctx, "Invalid Techprofile type received", log.Fields{"onuID": onuID, "TpId": tpId})
					return err
				}
			} else {
				logger.Errorw(ctx, "No TpInstance found for the TpID", log.Fields{"onuID": onuID, "TpId": tpId})
				continue
			}
		}
	}
	return err
}

//nolint:unparam
func (dh *DeviceHandler) getOnuStatsFromOlt(ctx context.Context, onuInfo *extension.GetOnuStatsFromOltRequest, onuDevice *voltha.Device) *extension.SingleGetValueResponse {
	singleValResp := extension.SingleGetValueResponse{
		Response: &extension.GetValueResponse{
			Status: extension.GetValueResponse_OK,
			Response: &extension.GetValueResponse_OnuStatsFromOltResponse{
				OnuStatsFromOltResponse: &extension.GetOnuStatsFromOltResponse{},
			},
		},
	}
	errResp := func(status extension.GetValueResponse_Status,
		reason extension.GetValueResponse_ErrorReason) *extension.SingleGetValueResponse {
		return &extension.SingleGetValueResponse{
			Response: &extension.GetValueResponse{
				Status:    status,
				ErrReason: reason,
			},
		}
	}

	var intfID, onuID uint32
	if onuDevice != nil {
		onuID = onuDevice.ProxyAddress.OnuId
		intfID = plt.PortNoToIntfID(onuDevice.ParentPortNo, voltha.Port_PON_OLT)
	}

	onuKey := dh.formOnuKey(intfID, onuID)
	if _, ok := dh.onus.Load(onuKey); !ok {
		logger.Errorw(ctx, "getOnuStatsFromOlt request received for invalid device", log.Fields{"deviceId": onuDevice.Id, "intfID": intfID, "onuID": onuID})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_DEVICE)
	}
	logger.Debugw(ctx, "getOnuStatsFromOlt request received", log.Fields{"intfID": intfID, "onuID": onuID})

	onuGemInfo, err := dh.CheckOnuGemInfo(ctx, intfID, onuID)
	if err == nil {
		err = dh.getAllocGemStats(ctx, intfID, onuID, onuGemInfo, &singleValResp)
		if err != nil {
			return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INTERNAL_ERROR)
		}
	} else {
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_DEVICE)
	}
	return &singleValResp
}

func (dh *DeviceHandler) getOnuInfo(ctx context.Context, intfID uint32, onuID *uint32) (*oop.OnuInfo, error) {
	Onu := oop.Onu{IntfId: intfID, OnuId: *onuID}
	OnuInfo, err := dh.Client.GetOnuInfo(ctx, &Onu)
	if err != nil {
		return nil, err
	}
	return OnuInfo, nil
}

func (dh *DeviceHandler) getIntfInfo(ctx context.Context, intfID uint32) (*oop.PonIntfInfo, error) {
	Intf := oop.Interface{IntfId: intfID}
	IntfInfo, err := dh.Client.GetPonInterfaceInfo(ctx, &Intf)
	if err != nil {
		return nil, err
	}
	return IntfInfo, nil
}

func (dh *DeviceHandler) getRxPower(ctx context.Context, rxPowerRequest *extension.GetRxPowerRequest) *extension.SingleGetValueResponse {
	Onu := oop.Onu{IntfId: rxPowerRequest.IntfId, OnuId: rxPowerRequest.OnuId}
	rxPower, err := dh.Client.GetPonRxPower(ctx, &Onu)
	if err != nil {
		logger.Errorw(ctx, "error-while-getting-rx-power", log.Fields{"Onu": Onu, "err": err})
		return generateSingleGetValueErrorResponse(err)
	}
	return &extension.SingleGetValueResponse{
		Response: &extension.GetValueResponse{
			Status: extension.GetValueResponse_OK,
			Response: &extension.GetValueResponse_RxPower{
				RxPower: &extension.GetRxPowerResponse{
					IntfId:     rxPowerRequest.IntfId,
					OnuId:      rxPowerRequest.OnuId,
					Status:     rxPower.Status,
					FailReason: rxPower.FailReason.String(),
					RxPower:    rxPower.RxPowerMeanDbm,
				},
			},
		},
	}
}

func (dh *DeviceHandler) getPONRxPower(ctx context.Context, OltRxPowerRequest *extension.GetOltRxPowerRequest) *extension.SingleGetValueResponse {
	errResp := func(status extension.GetValueResponse_Status, reason extension.GetValueResponse_ErrorReason) *extension.SingleGetValueResponse {
		return &extension.SingleGetValueResponse{
			Response: &extension.GetValueResponse{
				Status:    status,
				ErrReason: reason,
			},
		}
	}

	resp := extension.SingleGetValueResponse{
		Response: &extension.GetValueResponse{
			Status: extension.GetValueResponse_OK,
			Response: &extension.GetValueResponse_OltRxPower{
				OltRxPower: &extension.GetOltRxPowerResponse{},
			},
		},
	}

	logger.Debugw(ctx, "getPONRxPower", log.Fields{"portLabel": OltRxPowerRequest.PortLabel, "OnuSerialNumber": OltRxPowerRequest.OnuSn, "device-id": dh.device.Id})
	portLabel := OltRxPowerRequest.PortLabel
	serialNumber := OltRxPowerRequest.OnuSn

	portInfo := strings.Split(portLabel, "-")
	portNumber, err := strconv.ParseUint(portInfo[1], 10, 32)

	if err != nil {
		logger.Errorw(ctx, "getPONRxPower invalid portNumber ", log.Fields{"oltPortNumber": portInfo[1]})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_REQ_TYPE)
	}

	if portInfo[0] != "pon" {
		logger.Errorw(ctx, "getPONRxPower invalid portType", log.Fields{"oltPortType": portInfo[0]})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_PORT_TYPE)
	}

	if serialNumber != "" {
		onuDev := dh.getChildDevice(ctx, serialNumber, (uint32)(portNumber))
		if onuDev != nil {
			Onu := oop.Onu{IntfId: uint32(portNumber), OnuId: onuDev.onuID}
			rxPower, err := dh.Client.GetPonRxPower(ctx, &Onu)
			if err != nil {
				logger.Errorw(ctx, "error-while-getting-rx-power", log.Fields{"Onu": Onu, "err": err})
				return generateSingleGetValueErrorResponse(err)
			}

			rxPowerValue := extension.RxPower{}
			rxPowerValue.OnuSn = onuDev.serialNumber
			rxPowerValue.Status = rxPower.GetStatus()
			rxPowerValue.RxPower = rxPower.GetRxPowerMeanDbm()
			rxPowerValue.FailReason = rxPower.GetFailReason().String()

			resp.Response.GetOltRxPower().RxPower = append(resp.Response.GetOltRxPower().RxPower, &rxPowerValue)
		} else {
			logger.Errorw(ctx, "getPONRxPower invalid Device", log.Fields{"portLabel": portLabel, "serialNumber": serialNumber})
			return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_DEVICE)
		}
	} else {
		dh.onus.Range(func(Onukey interface{}, onuInCache interface{}) bool {
			if onuInCache.(*OnuDevice).intfID == (uint32)(portNumber) {
				Onu := oop.Onu{IntfId: (uint32)(portNumber), OnuId: onuInCache.(*OnuDevice).onuID}
				rxPower, err := dh.Client.GetPonRxPower(ctx, &Onu)
				if err != nil {
					logger.Errorw(ctx, "error-while-getting-rx-power, however considering to proceed further with other ONUs on PON", log.Fields{"Onu": Onu, "err": err})
				} else {
					rxPowerValue := extension.RxPower{}
					rxPowerValue.OnuSn = onuInCache.(*OnuDevice).serialNumber
					rxPowerValue.Status = rxPower.GetStatus()
					rxPowerValue.RxPower = rxPower.GetRxPowerMeanDbm()
					rxPowerValue.FailReason = rxPower.GetFailReason().String()

					resp.Response.GetOltRxPower().RxPower = append(resp.Response.GetOltRxPower().RxPower, &rxPowerValue)
				}
			}
			logger.Infow(ctx, "getPONRxPower response ", log.Fields{"Response": resp})
			return true
		})
	}
	logger.Infow(ctx, "getPONRxPower response ", log.Fields{"Response": resp})
	return &resp
}

func (dh *DeviceHandler) getPonPortStats(ctx context.Context, ponStatsRequest *extension.GetPonStatsRequest) *extension.SingleGetValueResponse {
	errResp := func(status extension.GetValueResponse_Status,
		reason extension.GetValueResponse_ErrorReason) *extension.SingleGetValueResponse {
		return &extension.SingleGetValueResponse{
			Response: &extension.GetValueResponse{
				Status:    status,
				ErrReason: reason,
			},
		}
	}

	resp := extension.SingleGetValueResponse{
		Response: &extension.GetValueResponse{
			Status: extension.GetValueResponse_OK,
			Response: &extension.GetValueResponse_OltPonStatsResponse{
				OltPonStatsResponse: &extension.GetPonStatsResponse{},
			},
		},
	}

	portLabel := ponStatsRequest.GetPortLabel()
	logger.Debugw(ctx, "getPonPortStats", log.Fields{"portLabel": portLabel, "device-id": dh.device.Id})

	portInfo := strings.Split(portLabel, "-")
	portNumber, err := strconv.ParseUint(portInfo[1], 10, 32)

	if err != nil {
		logger.Errorw(ctx, "getPonPortStats invalid portNumber ", log.Fields{"oltPortNumber": portInfo[1]})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_REQ_TYPE)
	}

	if portInfo[0] != "pon" {
		logger.Errorw(ctx, "getPonPortStats invalid portType", log.Fields{"oltPortType": portInfo[0]})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_PORT_TYPE)
	}

	Interface := oop.Interface{IntfId: uint32(portNumber)}
	ponStats, err := dh.Client.GetPonPortStatistics(ctx, &Interface)
	if err != nil {
		logger.Errorw(ctx, "error-while-getting-pon-port-stats", log.Fields{"IntfId": portNumber, "err": err})
		return generateSingleGetValueErrorResponse(err)
	}

	ponPortStats := resp.Response.GetOltPonStatsResponse()
	ponPortStats.PonPort = uint32(portNumber)
	ponPortStats.PortStatistics = ponStats

	logger.Infow(ctx, "getPonPortStats response ", log.Fields{"Response": resp})
	return &resp
}

func (dh *DeviceHandler) getNniPortStats(ctx context.Context, nniStatsRequest *extension.GetNNIStatsRequest) *extension.SingleGetValueResponse {
	errResp := func(status extension.GetValueResponse_Status,
		reason extension.GetValueResponse_ErrorReason) *extension.SingleGetValueResponse {
		return &extension.SingleGetValueResponse{
			Response: &extension.GetValueResponse{
				Status:    status,
				ErrReason: reason,
			},
		}
	}

	resp := extension.SingleGetValueResponse{
		Response: &extension.GetValueResponse{
			Status: extension.GetValueResponse_OK,
			Response: &extension.GetValueResponse_OltNniStatsResponse{
				OltNniStatsResponse: &extension.GetNNIStatsResponse{},
			},
		},
	}

	portLabel := nniStatsRequest.GetPortLabel()
	logger.Debugw(ctx, "getNniPortStats", log.Fields{"portLabel": portLabel, "device-id": dh.device.Id})

	portInfo := strings.Split(portLabel, "-")
	portNumber, err := strconv.ParseUint(portInfo[1], 10, 32)

	if err != nil {
		logger.Errorw(ctx, "getNniPortStats invalid portNumber ", log.Fields{"oltPortNumber": portInfo[1]})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_REQ_TYPE)
	}

	if portInfo[0] != "nni" {
		logger.Errorw(ctx, "getNniPortStats invalid portType", log.Fields{"oltPortType": portInfo[0]})
		return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_PORT_TYPE)
	}

	Interface := oop.Interface{IntfId: uint32(portNumber)}
	nniStats, err := dh.Client.GetNniPortStatistics(ctx, &Interface)
	if err != nil {
		logger.Errorw(ctx, "error-while-getting-nni-port-stats", log.Fields{"PortNo": portNumber, "err": err})
		return generateSingleGetValueErrorResponse(err)
	}

	nniPortStats := resp.Response.GetOltNniStatsResponse()
	nniPortStats.NniPort = uint32(portNumber)
	nniPortStats.PortStatistics = nniStats

	logger.Infow(ctx, "getNniPortStats response ", log.Fields{"Response": resp})
	return &resp
}

// nolint: unparam
func generateSingleGetValueErrorResponse(err error) *extension.SingleGetValueResponse {
	errResp := func(status extension.GetValueResponse_Status, reason extension.GetValueResponse_ErrorReason) *extension.SingleGetValueResponse {
		return &extension.SingleGetValueResponse{
			Response: &extension.GetValueResponse{
				Status:    status,
				ErrReason: reason,
			},
		}
	}

	if err != nil {
		if e, ok := status.FromError(err); ok {
			switch e.Code() {
			case codes.Internal:
				return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INTERNAL_ERROR)
			case codes.DeadlineExceeded:
				return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_TIMEOUT)
			case codes.Unimplemented:
				return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_UNSUPPORTED)
			case codes.NotFound:
				return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_DEVICE)
			}
		}
	}

	return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_REASON_UNDEFINED)
}

/*
Helper functions to communicate with Core
*/

func (dh *DeviceHandler) getDeviceFromCore(ctx context.Context, deviceID string) (*voltha.Device, error) {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return nil, err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	return cClient.GetDevice(subCtx, &common.ID{Id: deviceID})
}

func (dh *DeviceHandler) getChildDeviceFromCore(ctx context.Context, childDeviceFilter *ca.ChildDeviceFilter) (*voltha.Device, error) {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return nil, err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	return cClient.GetChildDevice(subCtx, childDeviceFilter)
}

func (dh *DeviceHandler) updateDeviceStateInCore(ctx context.Context, deviceStateFilter *ca.DeviceStateFilter) error {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = cClient.DeviceStateUpdate(subCtx, deviceStateFilter)
	return err
}

func (dh *DeviceHandler) getChildDevicesFromCore(ctx context.Context, deviceID string) (*voltha.Devices, error) {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return nil, err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	return cClient.GetChildDevices(subCtx, &common.ID{Id: deviceID})
}

func (dh *DeviceHandler) listDevicePortsFromCore(ctx context.Context, deviceID string) (*voltha.Ports, error) {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return nil, err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	return cClient.ListDevicePorts(subCtx, &common.ID{Id: deviceID})
}

func (dh *DeviceHandler) updateDeviceInCore(ctx context.Context, device *voltha.Device) error {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = cClient.DeviceUpdate(subCtx, device)
	return err
}

func (dh *DeviceHandler) sendChildDeviceDetectedToCore(ctx context.Context, deviceDiscoveryInfo *ca.DeviceDiscovery) (*voltha.Device, error) {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return nil, err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	return cClient.ChildDeviceDetected(subCtx, deviceDiscoveryInfo)
}

func (dh *DeviceHandler) sendPacketToCore(ctx context.Context, pkt *ca.PacketIn) error {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = cClient.SendPacketIn(subCtx, pkt)
	return err
}

func (dh *DeviceHandler) createPortInCore(ctx context.Context, port *voltha.Port) error {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = cClient.PortCreated(subCtx, port)
	return err
}

func (dh *DeviceHandler) updatePortsStateInCore(ctx context.Context, portFilter *ca.PortStateFilter) error {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = cClient.PortsStateUpdate(subCtx, portFilter)
	return err
}

func (dh *DeviceHandler) updatePortStateInCore(ctx context.Context, portState *ca.PortState) error {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = cClient.PortStateUpdate(subCtx, portState)
	return err
}

func (dh *DeviceHandler) getPortFromCore(ctx context.Context, portFilter *ca.PortFilter) (*voltha.Port, error) {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return nil, err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	return cClient.GetDevicePort(subCtx, portFilter)
}

func (dh *DeviceHandler) getAllPortsFromCore(ctx context.Context, portFilter *ca.PortFilter) (*voltha.Ports, error) {
	cClient, err := dh.coreClient.GetCoreServiceClient()
	if err != nil || cClient == nil {
		return nil, err
	}
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	return cClient.GetPorts(subCtx, portFilter)
}

/*
Helper functions to communicate with child adapter
*/

func (dh *DeviceHandler) sendOmciIndicationToChildAdapter(ctx context.Context, childEndpoint string, response *ia.OmciMessage) error {
	aClient, err := dh.getChildAdapterServiceClient(childEndpoint)
	if err != nil || aClient == nil {
		return err
	}
	logger.Debugw(ctx, "sending-omci-response", log.Fields{"response": response, "child-endpoint": childEndpoint})
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = aClient.OmciIndication(subCtx, response)
	return err
}

func (dh *DeviceHandler) sendOnuIndicationToChildAdapter(ctx context.Context, childEndpoint string, onuInd *ia.OnuIndicationMessage) error {
	aClient, err := dh.getChildAdapterServiceClient(childEndpoint)
	if err != nil || aClient == nil {
		return err
	}
	logger.Debugw(ctx, "sending-onu-indication", log.Fields{"onu-indication": onuInd, "child-endpoint": childEndpoint})
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = aClient.OnuIndication(subCtx, onuInd)
	return err
}

func (dh *DeviceHandler) sendDeleteTContToChildAdapter(ctx context.Context, childEndpoint string, tContInfo *ia.DeleteTcontMessage) error {
	aClient, err := dh.getChildAdapterServiceClient(childEndpoint)
	if err != nil || aClient == nil {
		return err
	}
	logger.Debugw(ctx, "sending-delete-tcont", log.Fields{"tcont": tContInfo, "child-endpoint": childEndpoint})
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = aClient.DeleteTCont(subCtx, tContInfo)
	return err
}

func (dh *DeviceHandler) sendDeleteGemPortToChildAdapter(ctx context.Context, childEndpoint string, gemPortInfo *ia.DeleteGemPortMessage) error {
	aClient, err := dh.getChildAdapterServiceClient(childEndpoint)
	if err != nil || aClient == nil {
		return err
	}
	logger.Debugw(ctx, "sending-delete-gem-port", log.Fields{"gem-port-info": gemPortInfo, "child-endpoint": childEndpoint})
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = aClient.DeleteGemPort(subCtx, gemPortInfo)
	return err
}

func (dh *DeviceHandler) sendDownloadTechProfileToChildAdapter(ctx context.Context, childEndpoint string, tpDownloadInfo *ia.TechProfileDownloadMessage) error {
	aClient, err := dh.getChildAdapterServiceClient(childEndpoint)
	if err != nil || aClient == nil {
		return err
	}
	logger.Debugw(ctx, "sending-tech-profile-download", log.Fields{"tp-download-info": tpDownloadInfo, "child-endpoint": childEndpoint})
	subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), dh.cfg.RPCTimeout)
	defer cancel()
	_, err = aClient.DownloadTechProfile(subCtx, tpDownloadInfo)
	return err
}

/*
Helper functions for remote communication
*/

// TODO: Use a connection tracker such that the adapter connection is stopped when the last device that adapter
// supports is deleted
func (dh *DeviceHandler) setupChildInterAdapterClient(ctx context.Context, endpoint string) error {
	logger.Infow(ctx, "setting-child-adapter-connection", log.Fields{"child-endpoint": endpoint})

	dh.lockChildAdapterClients.Lock()
	defer dh.lockChildAdapterClients.Unlock()
	if _, ok := dh.childAdapterClients[endpoint]; ok {
		// Already set
		return nil
	}

	// Setup child's adapter grpc connection
	var err error
	if dh.childAdapterClients[endpoint], err = vgrpc.NewClient(
		dh.cfg.AdapterEndpoint,
		endpoint,
		"onu_inter_adapter_service.OnuInterAdapterService",
		dh.onuInterAdapterRestarted,
		vgrpc.ClientContextData(dh.device.Id)); err != nil {
		logger.Errorw(ctx, "grpc-client-not-created", log.Fields{"error": err, "endpoint": endpoint})
		return err
	}
	retryCodes := []codes.Code{
		codes.Unavailable,      // server is currently unavailable
		codes.DeadlineExceeded, // deadline for the operation was exceeded
	}
	backoffCtxOption := grpc_retry.WithBackoff(grpc_retry.BackoffLinearWithJitter(dh.cfg.PerRPCRetryTimeout, 0.2))
	grpcRetryOptions := grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(dh.cfg.MaxRetries), grpc_retry.WithPerRetryTimeout(dh.cfg.PerRPCRetryTimeout), grpc_retry.WithCodes(retryCodes...), backoffCtxOption)

	go dh.childAdapterClients[endpoint].Start(log.WithSpanFromContext(context.TODO(), ctx), dh.getOnuInterAdapterServiceClientHandler, grpcRetryOptions)
	// Wait until we have a connection to the child adapter.
	// Unlimited retries or until context expires
	subCtx := log.WithSpanFromContext(context.TODO(), ctx)
	backoff := vgrpc.NewBackoff(dh.cfg.MinBackoffRetryDelay, dh.cfg.MaxBackoffRetryDelay, 0)
	for {
		client, err := dh.childAdapterClients[endpoint].GetOnuInterAdapterServiceClient()
		if err == nil && client != nil {
			logger.Infow(subCtx, "connected-to-child-adapter", log.Fields{"child-endpoint": endpoint})
			break
		}
		logger.Warnw(subCtx, "connection-to-child-adapter-not-ready", log.Fields{"error": err, "child-endpoint": endpoint})
		// Backoff
		if err = backoff.Backoff(subCtx); err != nil {
			logger.Errorw(subCtx, "received-error-on-backoff", log.Fields{"error": err, "child-endpoint": endpoint})
			break
		}
	}
	return nil
}

func (dh *DeviceHandler) getChildAdapterServiceClient(endpoint string) (onu_inter_adapter_service.OnuInterAdapterServiceClient, error) {
	// First check from cache
	dh.lockChildAdapterClients.RLock()
	if cgClient, ok := dh.childAdapterClients[endpoint]; ok {
		dh.lockChildAdapterClients.RUnlock()
		return cgClient.GetOnuInterAdapterServiceClient()
	}
	dh.lockChildAdapterClients.RUnlock()

	// Set the child connection - can occur on restarts
	ctx, cancel := context.WithTimeout(context.Background(), dh.cfg.RPCTimeout)
	err := dh.setupChildInterAdapterClient(ctx, endpoint)
	cancel()
	if err != nil {
		return nil, err
	}

	// Get the child client now
	dh.lockChildAdapterClients.RLock()
	defer dh.lockChildAdapterClients.RUnlock()
	if cgClient, ok := dh.childAdapterClients[endpoint]; ok {
		return cgClient.GetOnuInterAdapterServiceClient()
	}
	return nil, fmt.Errorf("no-client-for-endpoint-%s", endpoint)
}

func (dh *DeviceHandler) deleteAdapterClients(ctx context.Context) {
	dh.lockChildAdapterClients.Lock()
	defer dh.lockChildAdapterClients.Unlock()
	for key, client := range dh.childAdapterClients {
		client.Stop(ctx)
		delete(dh.childAdapterClients, key)
	}
}

// TODO:  Any action the adapter needs to do following a onu adapter inter adapter service restart?
func (dh *DeviceHandler) onuInterAdapterRestarted(ctx context.Context, endPoint string) error {
	logger.Warnw(ctx, "onu-inter-adapter-service-reconnected", log.Fields{"endpoint": endPoint})
	return nil
}

// getOnuInterAdapterServiceClientHandler is used to setup the remote gRPC service
func (dh *DeviceHandler) getOnuInterAdapterServiceClientHandler(ctx context.Context, conn *grpc.ClientConn) interface{} {
	if conn == nil {
		return nil
	}
	return onu_inter_adapter_service.NewOnuInterAdapterServiceClient(conn)
}

func (dh *DeviceHandler) setDeviceDeletionInProgressFlag(flag bool) {
	dh.lockDevice.Lock()
	defer dh.lockDevice.Unlock()
	dh.isDeviceDeletionInProgress = flag
}

func (dh *DeviceHandler) getDeviceDeletionInProgressFlag() bool {
	dh.lockDevice.RLock()
	defer dh.lockDevice.RUnlock()
	return dh.isDeviceDeletionInProgress
}

// waitForTimeoutOrCompletion waits for the waitgroup for the specified max timeout.
// Returns false if waiting timed out.
func (dh *DeviceHandler) waitForTimeoutOrCompletion(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return true // completed normally
	case <-time.After(timeout):
		return false // timed out
	}
}

func (dh *DeviceHandler) updateHeartbeatSignature(ctx context.Context, signature uint32) {
	val, err := json.Marshal(signature)
	if err != nil {
		logger.Error(ctx, "failed-to-marshal")
		return
	}
	if err = dh.kvStore.Put(ctx, heartbeatPath, val); err != nil {
		logger.Error(ctx, "failed-to-store-hearbeat-signature")
	}
}

func (dh *DeviceHandler) getHeartbeatSignature(ctx context.Context) uint32 {
	var signature uint32

	Value, er := dh.kvStore.Get(ctx, heartbeatPath)
	if er == nil {
		if Value != nil {
			Val, er := kvstore.ToByte(Value.Value)
			if er != nil {
				logger.Errorw(ctx, "Failed to convert into byte array", log.Fields{"err": er})
				return signature
			}
			if er = json.Unmarshal(Val, &signature); er != nil {
				logger.Error(ctx, "Failed to unmarshal signature", log.Fields{"err": er})
				return signature
			}
		}
	}
	return signature
}
