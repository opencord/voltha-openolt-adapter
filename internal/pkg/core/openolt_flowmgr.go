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
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencord/voltha-lib-go/v7/pkg/meters"

	"github.com/opencord/voltha-lib-go/v7/pkg/flows"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	plt "github.com/opencord/voltha-lib-go/v7/pkg/platform"
	tp "github.com/opencord/voltha-lib-go/v7/pkg/techprofile"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"
	"github.com/opencord/voltha-protos/v5/go/common"
	ia "github.com/opencord/voltha-protos/v5/go/inter_adapter"
	ofp "github.com/opencord/voltha-protos/v5/go/openflow_13"
	openoltpb2 "github.com/opencord/voltha-protos/v5/go/openolt"
	tp_pb "github.com/opencord/voltha-protos/v5/go/tech_profile"
	"github.com/opencord/voltha-protos/v5/go/voltha"

	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	//IPProtoDhcp flow category
	IPProtoDhcp = 17

	//IgmpProto proto value
	IgmpProto = 2

	//EapEthType eapethtype value
	EapEthType = 0x888e
	//LldpEthType lldp ethtype value
	LldpEthType = 0x88cc
	//IPv4EthType IPv4 ethernet type value
	IPv4EthType = 0x800
	//PPPoEDEthType PPPoE discovery ethernet type value
	PPPoEDEthType = 0x8863

	//ReservedVlan Transparent Vlan (Masked Vlan, VLAN_ANY in ONOS Flows)
	ReservedVlan = 4096

	//DefaultMgmtVlan default vlan value
	DefaultMgmtVlan = 4091

	// Openolt Flow

	//Upstream constant
	Upstream = "upstream"
	//Downstream constant
	Downstream = "downstream"
	//Multicast constant
	Multicast = "multicast"
	//PacketTagType constant
	PacketTagType = "pkt_tag_type"
	//Untagged constant
	Untagged = "untagged"
	//SingleTag constant
	SingleTag = "single_tag"
	//DoubleTag constant
	DoubleTag = "double_tag"

	// classifierInfo

	//EthType constant
	EthType = "eth_type"
	//EthDst constant
	EthDst = "eth_dst"
	//TPID constant
	TPID = "tpid"
	//IPProto constant
	IPProto = "ip_proto"
	//InPort constant
	InPort = "in_port"
	//VlanVid constant
	VlanVid = "vlan_vid"
	//VlanPcp constant
	VlanPcp = "vlan_pcp"

	//UDPDst constant
	UDPDst = "udp_dst"
	//UDPSrc constant
	UDPSrc = "udp_src"
	//Ipv4Dst constant
	Ipv4Dst = "ipv4_dst"
	//Ipv4Src constant
	Ipv4Src = "ipv4_src"
	//Metadata constant
	Metadata = "metadata"
	//TunnelID constant
	TunnelID = "tunnel_id"
	//Output constant
	Output = "output"
	//GroupID constant
	GroupID = "group_id"
	// Actions

	//PopVlan constant
	PopVlan = "pop_vlan"
	//PushVlan constant
	PushVlan = "push_vlan"
	//TrapToHost constant
	TrapToHost = "trap_to_host"
	//MaxMeterBand constant
	MaxMeterBand = 2
	//VlanPCPMask contant
	VlanPCPMask = 0xFF
	//VlanvIDMask constant
	VlanvIDMask = 0xFFF
	//IntfID constant
	IntfID = "intfId"
	//OnuID constant
	OnuID = "onuId"
	//UniID constant
	UniID = "uniId"
	//PortNo constant
	PortNo = "portNo"
	//AllocID constant
	AllocID = "allocId"
	//GemID constant
	GemID = "gemId"

	//NoneOnuID constant
	NoneOnuID = -1
	//NoneUniID constant
	NoneUniID = -1

	// Max number of flows that can be queued per ONU
	maxConcurrentFlowsPerOnu = 20

	bitMapPrefix = "0b"
	pbit1        = '1'
)

type schedQueue struct {
	direction    tp_pb.Direction
	intfID       uint32
	onuID        uint32
	uniID        uint32
	tpID         uint32
	uniPort      uint32
	tpInst       interface{}
	meterID      uint32
	flowMetadata *ofp.FlowMetadata
}

type flowContext struct {
	intfID      uint32
	onuID       uint32
	uniID       uint32
	portNo      uint32
	classifier  map[string]interface{}
	action      map[string]interface{}
	logicalFlow *ofp.OfpFlowStats
	allocID     uint32
	gemPortID   uint32
	tpID        uint32
	pbitToGem   map[uint32]uint32
	gemToAes    map[uint32]bool
}

// This control block is created per flow add/remove and pushed on the incomingFlows channel slice
// The flowControlBlock is then picked by the perOnuFlowHandlerRoutine for further processing.
// There is on perOnuFlowHandlerRoutine routine per ONU that constantly monitors for any incoming
// flow and processes it serially
type flowControlBlock struct {
	ctx          context.Context   // Flow handler context
	addFlow      bool              // if true flow to be added, else removed
	flow         *ofp.OfpFlowStats // Flow message
	flowMetadata *ofp.FlowMetadata // FlowMetadata that contains flow meter information. This can be nil for Flow remove
	errChan      *chan error       // channel to report the Flow handling error
}

//OpenOltFlowMgr creates the Structure of OpenOltFlowMgr obj
type OpenOltFlowMgr struct {
	ponPortIdx    uint32 // Pon Port this FlowManager is responsible for
	techprofile   tp.TechProfileIf
	deviceHandler *DeviceHandler
	grpMgr        *OpenOltGroupMgr
	resourceMgr   *rsrcMgr.OpenOltResourceMgr

	gemToFlowIDs    map[uint32][]uint64 // gem port id to flow ids
	gemToFlowIDsKey sync.RWMutex        // lock to be used to access the gemToFlowIDs map

	packetInGemPort     map[rsrcMgr.PacketInInfoKey]uint32 //packet in gem port local cache
	packetInGemPortLock sync.RWMutex

	// TODO create a type rsrcMgr.OnuGemInfos to be used instead of []rsrcMgr.OnuGemInfo
	onuGemInfoMap map[uint32]*rsrcMgr.OnuGemInfo //onu, gem and uni info local cache -> map of onuID to OnuGemInfo
	// We need to have a global lock on the onuGemInfo map
	onuGemInfoLock sync.RWMutex

	flowIDToGems     map[uint64][]uint32
	flowIDToGemsLock sync.RWMutex

	// Slice of channels. Each channel in slice, index by ONU ID, queues flows per ONU.
	// A go routine per ONU, waits on the unique channel (indexed by ONU ID) for incoming flows (add/remove)
	incomingFlows            []chan flowControlBlock
	stopFlowHandlerRoutine   []chan bool
	flowHandlerRoutineActive []bool
}

//NewFlowManager creates OpenOltFlowMgr object and initializes the parameters
func NewFlowManager(ctx context.Context, dh *DeviceHandler, rMgr *rsrcMgr.OpenOltResourceMgr, grpMgr *OpenOltGroupMgr, ponPortIdx uint32) *OpenOltFlowMgr {
	logger.Infow(ctx, "initializing-flow-manager", log.Fields{"device-id": dh.device.Id})
	var flowMgr OpenOltFlowMgr
	var err error

	flowMgr.deviceHandler = dh
	flowMgr.ponPortIdx = ponPortIdx
	flowMgr.grpMgr = grpMgr
	flowMgr.resourceMgr = rMgr
	if err = flowMgr.populateTechProfileForCurrentPonPort(ctx); err != nil {
		logger.Errorw(ctx, "error-while-populating-tech-profile-mgr", log.Fields{"err": err})
		return nil
	}
	flowMgr.gemToFlowIDs = make(map[uint32][]uint64)
	flowMgr.packetInGemPort = make(map[rsrcMgr.PacketInInfoKey]uint32)
	flowMgr.flowIDToGems = make(map[uint64][]uint32)

	// Create a slice of buffered channels for handling concurrent flows per ONU.
	// The additional entry (+1) is to handle the NNI trap flows on a separate channel from individual ONUs channel
	flowMgr.incomingFlows = make([]chan flowControlBlock, plt.MaxOnusPerPon+1)
	flowMgr.stopFlowHandlerRoutine = make([]chan bool, plt.MaxOnusPerPon+1)
	flowMgr.flowHandlerRoutineActive = make([]bool, plt.MaxOnusPerPon+1)
	for i := range flowMgr.incomingFlows {
		flowMgr.incomingFlows[i] = make(chan flowControlBlock, maxConcurrentFlowsPerOnu)
		flowMgr.stopFlowHandlerRoutine[i] = make(chan bool)
		// Spin up a go routine to handling incoming flows (add/remove).
		// There will be on go routine per ONU.
		// This routine will be blocked on the flowMgr.incomingFlows[onu-id] channel for incoming flows.
		flowMgr.flowHandlerRoutineActive[i] = true
		go flowMgr.perOnuFlowHandlerRoutine(i, flowMgr.incomingFlows[i], flowMgr.stopFlowHandlerRoutine[i])
	}
	flowMgr.onuGemInfoMap = make(map[uint32]*rsrcMgr.OnuGemInfo)
	//Load the onugem info cache from kv store on flowmanager start
	onuIDStart := flowMgr.deviceHandler.deviceInfo.OnuIdStart
	onuIDEnd := flowMgr.deviceHandler.deviceInfo.OnuIdEnd
	for onuID := onuIDStart; onuID <= onuIDEnd; onuID++ {
		// check for a valid serial number in onuGem as GetOnuGemInfo can return nil error in case of nothing found in the path.
		onugem, err := rMgr.GetOnuGemInfo(ctx, ponPortIdx, onuID)
		if err == nil && onugem != nil && onugem.SerialNumber != "" {
			flowMgr.onuGemInfoMap[onuID] = onugem
		}
	}

	//Load flowID list per gem map And gemIDs per flow per interface from the kvstore.
	flowMgr.loadFlowIDsForGemAndGemIDsForFlow(ctx)

	//load interface to multicast queue map from kv store
	flowMgr.grpMgr.LoadInterfaceToMulticastQueueMap(ctx)
	logger.Info(ctx, "initialization-of-flow-manager-success")
	return &flowMgr
}

func (f *OpenOltFlowMgr) registerFlow(ctx context.Context, flowFromCore *ofp.OfpFlowStats, deviceFlow *openoltpb2.Flow) error {
	if !deviceFlow.ReplicateFlow && deviceFlow.GemportId > 0 {
		// Flow is not replicated in this case, we need to register the flow for a single gem-port
		return f.registerFlowIDForGemAndGemIDForFlow(ctx, uint32(deviceFlow.AccessIntfId), uint32(deviceFlow.GemportId), flowFromCore)
	} else if deviceFlow.ReplicateFlow && len(deviceFlow.PbitToGemport) > 0 {
		// Flow is replicated in this case. We need to register the flow for all the gem-ports it is replicated to.
		for _, gemPort := range deviceFlow.PbitToGemport {
			if err := f.registerFlowIDForGemAndGemIDForFlow(ctx, uint32(deviceFlow.AccessIntfId), gemPort, flowFromCore); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *OpenOltFlowMgr) registerFlowIDForGemAndGemIDForFlow(ctx context.Context, accessIntfID uint32, gemPortID uint32, flowFromCore *ofp.OfpFlowStats) error {
	// update gem->flows map
	f.gemToFlowIDsKey.Lock()
	flowIDList, ok := f.gemToFlowIDs[gemPortID]
	if !ok {
		flowIDList = []uint64{flowFromCore.Id}
	} else {
		flowIDList = appendUnique64bit(flowIDList, flowFromCore.Id)
	}
	f.gemToFlowIDs[gemPortID] = flowIDList
	f.gemToFlowIDsKey.Unlock()

	// update flow->gems map
	f.flowIDToGemsLock.Lock()
	if _, ok := f.flowIDToGems[flowFromCore.Id]; !ok {
		f.flowIDToGems[flowFromCore.Id] = []uint32{gemPortID}
	} else {
		f.flowIDToGems[flowFromCore.Id] = appendUnique32bit(f.flowIDToGems[flowFromCore.Id], gemPortID)
	}
	f.flowIDToGemsLock.Unlock()

	// update the flowids for a gem to the KVstore
	return f.resourceMgr.UpdateFlowIDsForGem(ctx, accessIntfID, gemPortID, flowIDList)
}

func (f *OpenOltFlowMgr) processAddFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32,
	classifierInfo map[string]interface{}, actionInfo map[string]interface{}, flow *ofp.OfpFlowStats, TpID uint32,
	UsMeterID uint32, DsMeterID uint32, flowMetadata *ofp.FlowMetadata) error {
	var allocID uint32
	var gemPorts []uint32
	var TpInst interface{}

	logger.Infow(ctx, "dividing-flow", log.Fields{
		"device-id":  f.deviceHandler.device.Id,
		"intf-id":    intfID,
		"onu-id":     onuID,
		"uni-id":     uniID,
		"port-no":    portNo,
		"classifier": classifierInfo,
		"action":     actionInfo,
		"usmeter-iD": UsMeterID,
		"dsmeter-iD": DsMeterID,
		"tp-id":      TpID})
	// only create tcont/gemports if there is actually an onu id.  otherwise BAL throws an error.  Usually this
	// is because the flow is an NNI flow and there would be no onu resources associated with it
	// TODO: properly deal with NNI flows
	if onuID == 0 {
		cause := "no-onu-id-for-flow"
		fields := log.Fields{
			"onu":       onuID,
			"port-no":   portNo,
			"classifer": classifierInfo,
			"action":    actionInfo,
			"device-id": f.deviceHandler.device.Id}
		logger.Errorw(ctx, cause, fields)
		return olterrors.NewErrNotFound(cause, fields, nil)
	}

	uni := getUniPortPath(f.deviceHandler.device.Id, intfID, int32(onuID), int32(uniID))
	logger.Debugw(ctx, "uni-port-path", log.Fields{
		"uni":       uni,
		"device-id": f.deviceHandler.device.Id})

	logger.Debugw(ctx, "dividing-flow-create-tcont-gem-ports", log.Fields{
		"device-id":  f.deviceHandler.device.Id,
		"intf-id":    intfID,
		"onu-id":     onuID,
		"uni-id":     uniID,
		"port-no":    portNo,
		"classifier": classifierInfo,
		"action":     actionInfo,
		"usmeter-id": UsMeterID,
		"dsmeter-id": DsMeterID,
		"tp-id":      TpID})
	allocID, gemPorts, TpInst = f.createTcontGemports(ctx, intfID, onuID, uniID, uni, portNo, TpID, UsMeterID, DsMeterID, flowMetadata)
	if allocID == 0 || gemPorts == nil || TpInst == nil {
		logger.Error(ctx, "alloc-id-gem-ports-tp-unavailable")
		return olterrors.NewErrNotFound(
			"alloc-id-gem-ports-tp-unavailable",
			nil, nil)
	}
	args := make(map[string]uint32)
	args[IntfID] = intfID
	args[OnuID] = onuID
	args[UniID] = uniID
	args[PortNo] = portNo
	args[AllocID] = allocID

	/* Flows can be added specific to gemport if p-bits are received.
	 * If no pbit mentioned then adding flows for all gemports
	 */
	return f.checkAndAddFlow(ctx, args, classifierInfo, actionInfo, flow, TpInst, gemPorts, TpID, uni)
}

// CreateSchedulerQueues creates traffic schedulers on the device with the given scheduler configuration and traffic shaping info
func (f *OpenOltFlowMgr) CreateSchedulerQueues(ctx context.Context, sq schedQueue) error {

	logger.Debugw(ctx, "CreateSchedulerQueues",
		log.Fields{"dir": sq.direction,
			"intf-id":      sq.intfID,
			"onu-id":       sq.onuID,
			"uni-id":       sq.uniID,
			"tp-id":        sq.tpID,
			"meter-id":     sq.meterID,
			"tp-inst":      sq.tpInst,
			"flowmetadata": sq.flowMetadata,
			"device-id":    f.deviceHandler.device.Id})

	direction, err := verifyMeterIDAndGetDirection(sq.meterID, sq.direction)
	if err != nil {
		return err
	}

	/* Lets make a simple assumption that if the meter-id is present on the KV store,
	 * then the scheduler and queues configuration is applied on the OLT device
	 * in the given direction.
	 */

	var SchedCfg *tp_pb.SchedulerConfig
	meterInfo, err := f.resourceMgr.GetMeterInfoForOnu(ctx, direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID)
	if err != nil {
		return olterrors.NewErrNotFound("meter",
			log.Fields{"intf-id": sq.intfID,
				"onu-id":    sq.onuID,
				"uni-id":    sq.uniID,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	// update referernce count and return if the meter was already installed before
	if meterInfo != nil && meterInfo.MeterID == sq.meterID {
		logger.Debugw(ctx, "scheduler-already-created-for-direction",
			log.Fields{"device-id": f.deviceHandler.device.Id, "direction": direction, "meter-id": sq.meterID})
		return f.resourceMgr.HandleMeterInfoRefCntUpdate(ctx, direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID, true)
	}

	logger.Debugw(ctx, "meter-does-not-exist-creating-new",
		log.Fields{
			"meter-id":  sq.meterID,
			"direction": direction,
			"device-id": f.deviceHandler.device.Id})

	if sq.direction == tp_pb.Direction_UPSTREAM {
		SchedCfg = f.techprofile.GetUsScheduler(sq.tpInst.(*tp_pb.TechProfileInstance))
	} else if sq.direction == tp_pb.Direction_DOWNSTREAM {
		SchedCfg = f.techprofile.GetDsScheduler(sq.tpInst.(*tp_pb.TechProfileInstance))
	}

	found := false
	meterInfo = &rsrcMgr.MeterInfo{}
	if sq.flowMetadata != nil {
		for _, meter := range sq.flowMetadata.Meters {
			if sq.meterID == meter.MeterId {
				meterInfo.MeterID = meter.MeterId
				meterInfo.RefCnt = 1 // initialize it to 1, since this is the first flow that referenced the meter id.
				logger.Debugw(ctx, "found-meter-config-from-flowmetadata",
					log.Fields{"meter": meter,
						"device-id": f.deviceHandler.device.Id})
				found = true
				break
			}
		}
	} else {
		logger.Errorw(ctx, "flow-metadata-not-present-in-flow", log.Fields{"device-id": f.deviceHandler.device.Id})
	}
	if !found {
		return olterrors.NewErrNotFound("meterbands", log.Fields{
			"reason":        "Could-not-get-meterbands-from-flowMetadata",
			"flow-metadata": sq.flowMetadata,
			"meter-id":      sq.meterID,
			"device-id":     f.deviceHandler.device.Id}, nil)
	}

	var TrafficShaping *tp_pb.TrafficShapingInfo
	if TrafficShaping, err = meters.GetTrafficShapingInfo(ctx, sq.flowMetadata.Meters[0]); err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{
			"reason":    "invalid-meter-config",
			"meter-id":  sq.meterID,
			"device-id": f.deviceHandler.device.Id}, nil)
	}

	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile.GetTrafficScheduler(sq.tpInst.(*tp_pb.TechProfileInstance), SchedCfg, TrafficShaping)}
	TrafficSched[0].TechProfileId = sq.tpID

	if err := f.pushSchedulerQueuesToDevice(ctx, sq, TrafficSched); err != nil {
		return olterrors.NewErrAdapter("failure-pushing-traffic-scheduler-and-queues-to-device",
			log.Fields{"intf-id": sq.intfID,
				"direction": sq.direction,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	/* After we successfully applied the scheduler configuration on the OLT device,
	 * store the meter id on the KV store, for further reference.
	 */
	if err := f.resourceMgr.StoreMeterInfoForOnu(ctx, direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID, meterInfo); err != nil {
		return olterrors.NewErrAdapter("failed-updating-meter-id",
			log.Fields{"onu-id": sq.onuID,
				"meter-id":  sq.meterID,
				"device-id": f.deviceHandler.device.Id}, err)
	}
	logger.Infow(ctx, "updated-meter-info-into-kv-store-successfully",
		log.Fields{"direction": direction,
			"meter-info": meterInfo,
			"device-id":  f.deviceHandler.device.Id})
	return nil
}

func (f *OpenOltFlowMgr) pushSchedulerQueuesToDevice(ctx context.Context, sq schedQueue, TrafficSched []*tp_pb.TrafficScheduler) error {
	trafficQueues, err := f.techprofile.GetTrafficQueues(ctx, sq.tpInst.(*tp_pb.TechProfileInstance), sq.direction)

	if err != nil {
		return olterrors.NewErrAdapter("unable-to-construct-traffic-queue-configuration",
			log.Fields{"intf-id": sq.intfID,
				"direction": sq.direction,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	if allocExists := f.isAllocUsedByAnotherUNI(ctx, sq); !allocExists {
		logger.Debugw(ctx, "sending-traffic-scheduler-create-to-device",
			log.Fields{
				"direction":     sq.direction,
				"TrafficScheds": TrafficSched,
				"device-id":     f.deviceHandler.device.Id,
				"intfID":        sq.intfID,
				"onuID":         sq.onuID,
				"uniID":         sq.uniID})
		if _, err := f.deviceHandler.Client.CreateTrafficSchedulers(ctx, &tp_pb.TrafficSchedulers{
			IntfId: sq.intfID, OnuId: sq.onuID,
			UniId: sq.uniID, PortNo: sq.uniPort,
			TrafficScheds: TrafficSched}); err != nil {
			return olterrors.NewErrAdapter("failed-to-create-traffic-schedulers-in-device", log.Fields{"TrafficScheds": TrafficSched}, err)
		}
		logger.Infow(ctx, "successfully-created-traffic-schedulers", log.Fields{
			"direction":      sq.direction,
			"traffic-queues": trafficQueues,
			"device-id":      f.deviceHandler.device.Id})
	}

	// On receiving the CreateTrafficQueues request, the driver should create corresponding
	// downstream queues.
	logger.Debugw(ctx, "sending-traffic-queues-create-to-device",
		log.Fields{"direction": sq.direction,
			"traffic-queues": trafficQueues,
			"device-id":      f.deviceHandler.device.Id})
	queues := &tp_pb.TrafficQueues{IntfId: sq.intfID, OnuId: sq.onuID,
		UniId: sq.uniID, PortNo: sq.uniPort,
		TrafficQueues: trafficQueues,
		TechProfileId: TrafficSched[0].TechProfileId}
	if _, err := f.deviceHandler.Client.CreateTrafficQueues(ctx, queues); err != nil {
		if len(queues.TrafficQueues) > 1 {
			logger.Debug(ctx, "removing-queues-for-1tcont-multi-gem", log.Fields{"intfID": sq.intfID, "onuID": sq.onuID, "dir": sq.direction})
			_, _ = f.deviceHandler.Client.RemoveTrafficQueues(ctx, queues)
		}
		f.revertScheduler(ctx, sq, TrafficSched)
		return olterrors.NewErrAdapter("failed-to-create-traffic-queues-in-device", log.Fields{"traffic-queues": trafficQueues}, err)
	}
	logger.Infow(ctx, "successfully-created-traffic-schedulers", log.Fields{
		"direction":      sq.direction,
		"traffic-queues": trafficQueues,
		"device-id":      f.deviceHandler.device.Id})

	if sq.direction == tp_pb.Direction_DOWNSTREAM {
		multicastTrafficQueues := f.techprofile.GetMulticastTrafficQueues(ctx, sq.tpInst.(*tp_pb.TechProfileInstance))
		if len(multicastTrafficQueues) > 0 {
			if _, present := f.grpMgr.GetInterfaceToMcastQueueMap(sq.intfID); !present { //assumed that there is only one queue per PON for the multicast service
				//the default queue with multicastQueuePerPonPort.Priority per a pon interface is used for multicast service
				//just put it in interfaceToMcastQueueMap to use for building group members
				logger.Debugw(ctx, "multicast-traffic-queues", log.Fields{"device-id": f.deviceHandler.device.Id})
				multicastQueuePerPonPort := multicastTrafficQueues[0]
				val := &QueueInfoBrief{
					gemPortID:       multicastQueuePerPonPort.GemportId,
					servicePriority: multicastQueuePerPonPort.Priority,
				}
				f.grpMgr.UpdateInterfaceToMcastQueueMap(sq.intfID, val)
				//also store the queue info in kv store
				if err := f.resourceMgr.AddMcastQueueForIntf(ctx, sq.intfID, multicastQueuePerPonPort.GemportId, multicastQueuePerPonPort.Priority); err != nil {
					logger.Errorw(ctx, "failed-to-add-mcast-queue", log.Fields{"err": err})
					return err
				}

				logger.Infow(ctx, "multicast-queues-successfully-updated", log.Fields{"device-id": f.deviceHandler.device.Id})
			}
		}
	}
	return nil
}

// RemoveSchedulerQueues removes the traffic schedulers from the device based on the given scheduler configuration and traffic shaping info
func (f *OpenOltFlowMgr) RemoveSchedulerQueues(ctx context.Context, sq schedQueue) error {

	var Direction string
	var SchedCfg *tp_pb.SchedulerConfig
	var err error
	logger.Infow(ctx, "removing-schedulers-and-queues-in-olt",
		log.Fields{
			"direction": sq.direction,
			"intf-id":   sq.intfID,
			"onu-id":    sq.onuID,
			"uni-id":    sq.uniID,
			"uni-port":  sq.uniPort,
			"device-id": f.deviceHandler.device.Id})
	if sq.direction == tp_pb.Direction_UPSTREAM {
		SchedCfg = f.techprofile.GetUsScheduler(sq.tpInst.(*tp_pb.TechProfileInstance))
		Direction = "upstream"
	} else if sq.direction == tp_pb.Direction_DOWNSTREAM {
		SchedCfg = f.techprofile.GetDsScheduler(sq.tpInst.(*tp_pb.TechProfileInstance))
		Direction = "downstream"
	}

	TrafficShaping := &tp_pb.TrafficShapingInfo{} // this info is not really useful for the agent during flow removal. Just use default values.

	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile.GetTrafficScheduler(sq.tpInst.(*tp_pb.TechProfileInstance), SchedCfg, TrafficShaping)}
	TrafficSched[0].TechProfileId = sq.tpID

	TrafficQueues, err := f.techprofile.GetTrafficQueues(ctx, sq.tpInst.(*tp_pb.TechProfileInstance), sq.direction)
	if err != nil {
		return olterrors.NewErrAdapter("unable-to-construct-traffic-queue-configuration",
			log.Fields{
				"intf-id":   sq.intfID,
				"direction": sq.direction,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	if _, err = f.deviceHandler.Client.RemoveTrafficQueues(ctx,
		&tp_pb.TrafficQueues{IntfId: sq.intfID, OnuId: sq.onuID,
			UniId: sq.uniID, PortNo: sq.uniPort,
			TrafficQueues: TrafficQueues,
			TechProfileId: TrafficSched[0].TechProfileId}); err != nil {
		return olterrors.NewErrAdapter("unable-to-remove-traffic-queues-from-device",
			log.Fields{
				"intf-id":        sq.intfID,
				"traffic-queues": TrafficQueues,
				"device-id":      f.deviceHandler.device.Id}, err)
	}
	logger.Infow(ctx, "removed-traffic-queues-successfully", log.Fields{"device-id": f.deviceHandler.device.Id, "trafficQueues": TrafficQueues})

	if allocExists := f.isAllocUsedByAnotherUNI(ctx, sq); !allocExists {
		if _, err = f.deviceHandler.Client.RemoveTrafficSchedulers(ctx, &tp_pb.TrafficSchedulers{
			IntfId: sq.intfID, OnuId: sq.onuID,
			UniId: sq.uniID, PortNo: sq.uniPort,
			TrafficScheds: TrafficSched}); err != nil {
			return olterrors.NewErrAdapter("unable-to-remove-traffic-schedulers-from-device",
				log.Fields{
					"intf-id":            sq.intfID,
					"traffic-schedulers": TrafficSched,
					"onu-id":             sq.onuID,
					"uni-id":             sq.uniID,
					"uni-port":           sq.uniPort}, err)
		}

		logger.Infow(ctx, "removed-traffic-schedulers-successfully",
			log.Fields{"device-id": f.deviceHandler.device.Id,
				"intf-id":  sq.intfID,
				"onu-id":   sq.onuID,
				"uni-id":   sq.uniID,
				"uni-port": sq.uniPort})

		if sq.direction == tp_pb.Direction_UPSTREAM {
			allocID := sq.tpInst.(*tp_pb.TechProfileInstance).UsScheduler.AllocId
			// Delete the TCONT on the ONU.
			uni := getUniPortPath(f.deviceHandler.device.Id, sq.intfID, int32(sq.onuID), int32(sq.uniID))
			tpPath := f.getTPpath(ctx, sq.intfID, uni, sq.tpID)
			if err := f.sendDeleteTcontToChild(ctx, sq.intfID, sq.onuID, sq.uniID, allocID, tpPath); err != nil {
				logger.Errorw(ctx, "error-processing-delete-tcont-towards-onu",
					log.Fields{
						"intf":      sq.intfID,
						"onu-id":    sq.onuID,
						"uni-id":    sq.uniID,
						"device-id": f.deviceHandler.device.Id,
						"alloc-id":  allocID})
			}
		}
	}

	/* After we successfully remove the scheduler configuration on the OLT device,
	 * delete the meter id on the KV store.
	 */
	err = f.resourceMgr.RemoveMeterInfoForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID)
	if err != nil {
		return olterrors.NewErrAdapter("unable-to-remove-meter",
			log.Fields{
				"onu":       sq.onuID,
				"device-id": f.deviceHandler.device.Id,
				"intf-id":   sq.intfID,
				"onu-id":    sq.onuID,
				"uni-id":    sq.uniID,
				"uni-port":  sq.uniPort}, err)
	}
	logger.Infow(ctx, "removed-meter-from-KV-store-successfully",
		log.Fields{
			"dir":       Direction,
			"device-id": f.deviceHandler.device.Id,
			"intf-id":   sq.intfID,
			"onu-id":    sq.onuID,
			"uni-id":    sq.uniID,
			"uni-port":  sq.uniPort})
	return err
}

// We are trying to force remove the schedulers and queues here if one exists for the given key.
// We ignore any errors encountered in the process. The errors most likely are encountered when
// the schedulers and queues are already cleared for the given key.
func (f *OpenOltFlowMgr) forceRemoveSchedulerQueues(ctx context.Context, sq schedQueue) {

	var schedCfg *tp_pb.SchedulerConfig
	var err error
	logger.Infow(ctx, "removing-schedulers-and-queues-in-olt",
		log.Fields{
			"direction": sq.direction,
			"intf-id":   sq.intfID,
			"onu-id":    sq.onuID,
			"uni-id":    sq.uniID,
			"uni-port":  sq.uniPort,
			"tp-id":     sq.tpID,
			"device-id": f.deviceHandler.device.Id})
	if sq.direction == tp_pb.Direction_UPSTREAM {
		schedCfg = f.techprofile.GetUsScheduler(sq.tpInst.(*tp_pb.TechProfileInstance))
	} else if sq.direction == tp_pb.Direction_DOWNSTREAM {
		schedCfg = f.techprofile.GetDsScheduler(sq.tpInst.(*tp_pb.TechProfileInstance))
	}

	TrafficShaping := &tp_pb.TrafficShapingInfo{} // this info is not really useful for the agent during flow removal. Just use default values.
	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile.GetTrafficScheduler(sq.tpInst.(*tp_pb.TechProfileInstance), schedCfg, TrafficShaping)}
	TrafficSched[0].TechProfileId = sq.tpID

	// Remove traffic queues. Ignore any errors, just log them.
	if TrafficQueues, err := f.techprofile.GetTrafficQueues(ctx, sq.tpInst.(*tp_pb.TechProfileInstance), sq.direction); err != nil {
		logger.Errorw(ctx, "error retrieving traffic queue", log.Fields{
			"direction": sq.direction,
			"intf-id":   sq.intfID,
			"onu-id":    sq.onuID,
			"uni-id":    sq.uniID,
			"uni-port":  sq.uniPort,
			"tp-id":     sq.tpID,
			"device-id": f.deviceHandler.device.Id,
			"err":       err})
	} else {
		if _, err = f.deviceHandler.Client.RemoveTrafficQueues(ctx,
			&tp_pb.TrafficQueues{IntfId: sq.intfID, OnuId: sq.onuID,
				UniId: sq.uniID, PortNo: sq.uniPort,
				TrafficQueues: TrafficQueues,
				TechProfileId: TrafficSched[0].TechProfileId}); err != nil {
			logger.Warnw(ctx, "error removing traffic queue", log.Fields{
				"direction": sq.direction,
				"intf-id":   sq.intfID,
				"onu-id":    sq.onuID,
				"uni-id":    sq.uniID,
				"uni-port":  sq.uniPort,
				"tp-id":     sq.tpID,
				"device-id": f.deviceHandler.device.Id,
				"err":       err})

		} else {
			logger.Infow(ctx, "removed-traffic-queues-successfully", log.Fields{"device-id": f.deviceHandler.device.Id,
				"direction": sq.direction,
				"intf-id":   sq.intfID,
				"onu-id":    sq.onuID,
				"uni-id":    sq.uniID,
				"uni-port":  sq.uniPort,
				"tp-id":     sq.tpID})
		}
	}

	// Remove traffic schedulers. Ignore any errors, just log them.
	if _, err = f.deviceHandler.Client.RemoveTrafficSchedulers(ctx, &tp_pb.TrafficSchedulers{
		IntfId: sq.intfID, OnuId: sq.onuID,
		UniId: sq.uniID, PortNo: sq.uniPort,
		TrafficScheds: TrafficSched}); err != nil {
		logger.Warnw(ctx, "error removing traffic scheduler", log.Fields{
			"direction": sq.direction,
			"intf-id":   sq.intfID,
			"onu-id":    sq.onuID,
			"uni-id":    sq.uniID,
			"uni-port":  sq.uniPort,
			"tp-id":     sq.tpID,
			"device-id": f.deviceHandler.device.Id,
			"err":       err})
	} else {
		logger.Infow(ctx, "removed-traffic-schedulers-successfully", log.Fields{"device-id": f.deviceHandler.device.Id,
			"direction": sq.direction,
			"intf-id":   sq.intfID,
			"onu-id":    sq.onuID,
			"uni-id":    sq.uniID,
			"uni-port":  sq.uniPort,
			"tp-id":     sq.tpID})
	}
}

// This function allocates tconts and GEM ports for an ONU
func (f *OpenOltFlowMgr) createTcontGemports(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, uni string, uniPort uint32, TpID uint32, UsMeterID uint32, DsMeterID uint32, flowMetadata *ofp.FlowMetadata) (uint32, []uint32, interface{}) {
	var allocIDs []uint32
	var allgemPortIDs []uint32
	var gemPortIDs []uint32
	tpInstanceExists := false
	var err error
	allocIDs = f.resourceMgr.GetCurrentAllocIDsForOnu(ctx, intfID, onuID, uniID)
	allgemPortIDs = f.resourceMgr.GetCurrentGEMPortIDsForOnu(ctx, intfID, onuID, uniID)
	tpPath := f.getTPpath(ctx, intfID, uni, TpID)

	logger.Debugw(ctx, "creating-new-tcont-and-gem", log.Fields{
		"intf-id":   intfID,
		"onu-id":    onuID,
		"uni-id":    uniID,
		"device-id": f.deviceHandler.device.Id,
		"tp-id":     TpID})

	// Check tech profile instance already exists for derived port name
	techProfileInstance, _ := f.techprofile.GetTPInstance(ctx, tpPath)
	if techProfileInstance == nil {
		logger.Infow(ctx, "tp-instance-not-found--creating-new",
			log.Fields{
				"path":      tpPath,
				"device-id": f.deviceHandler.device.Id})
		techProfileInstance, err = f.techprofile.CreateTechProfileInstance(ctx, TpID, uni, intfID)
		if err != nil {
			// This should not happen, something wrong in KV backend transaction
			logger.Errorw(ctx, "tp-instance-create-failed",
				log.Fields{
					"err":       err,
					"tp-id":     TpID,
					"device-id": f.deviceHandler.device.Id})
			return 0, nil, nil
		}
		if err := f.resourceMgr.UpdateTechProfileIDForOnu(ctx, intfID, onuID, uniID, TpID); err != nil {
			logger.Warnw(ctx, "failed-to-update-tech-profile-id", log.Fields{"err": err})
		}
	} else {
		logger.Debugw(ctx, "tech-profile-instance-already-exist-for-given port-name",
			log.Fields{
				"uni":       uni,
				"device-id": f.deviceHandler.device.Id})
		tpInstanceExists = true
	}

	switch tpInst := techProfileInstance.(type) {
	case *tp_pb.TechProfileInstance:
		if UsMeterID != 0 {
			sq := schedQueue{direction: tp_pb.Direction_UPSTREAM, intfID: intfID, onuID: onuID, uniID: uniID, tpID: TpID,
				uniPort: uniPort, tpInst: techProfileInstance, meterID: UsMeterID, flowMetadata: flowMetadata}
			if err := f.CreateSchedulerQueues(ctx, sq); err != nil {
				logger.Errorw(ctx, "CreateSchedulerQueues-failed-upstream",
					log.Fields{
						"err":       err,
						"onu-id":    onuID,
						"uni-id":    uniID,
						"intf-id":   intfID,
						"meter-id":  UsMeterID,
						"device-id": f.deviceHandler.device.Id})
				f.revertTechProfileInstance(ctx, sq)
				return 0, nil, nil
			}
		}
		if DsMeterID != 0 {
			sq := schedQueue{direction: tp_pb.Direction_DOWNSTREAM, intfID: intfID, onuID: onuID, uniID: uniID, tpID: TpID,
				uniPort: uniPort, tpInst: techProfileInstance, meterID: DsMeterID, flowMetadata: flowMetadata}
			if err := f.CreateSchedulerQueues(ctx, sq); err != nil {
				logger.Errorw(ctx, "CreateSchedulerQueues-failed-downstream",
					log.Fields{
						"err":       err,
						"onu-id":    onuID,
						"uni-id":    uniID,
						"intf-id":   intfID,
						"meter-id":  DsMeterID,
						"device-id": f.deviceHandler.device.Id})
				f.revertTechProfileInstance(ctx, sq)
				return 0, nil, nil
			}
		}
		allocID := tpInst.UsScheduler.AllocId
		for _, gem := range tpInst.UpstreamGemPortAttributeList {
			gemPortIDs = append(gemPortIDs, gem.GemportId)
		}
		allocIDs = appendUnique32bit(allocIDs, allocID)

		if tpInstanceExists {
			return allocID, gemPortIDs, techProfileInstance
		}

		for _, gemPortID := range gemPortIDs {
			allgemPortIDs = appendUnique32bit(allgemPortIDs, gemPortID)
		}
		logger.Infow(ctx, "allocated-tcont-and-gem-ports",
			log.Fields{
				"intf-id":   intfID,
				"onu-id":    onuID,
				"uni-id":    uniID,
				"alloc-ids": allocIDs,
				"gemports":  allgemPortIDs,
				"device-id": f.deviceHandler.device.Id})
		// Send Tconts and GEM ports to KV store
		f.storeTcontsGEMPortsIntoKVStore(ctx, intfID, onuID, uniID, allocIDs, allgemPortIDs)
		return allocID, gemPortIDs, techProfileInstance
	case *tp_pb.EponTechProfileInstance:
		// CreateSchedulerQueues for EPON needs to be implemented here
		// when voltha-protos for EPON is completed.
		allocID := tpInst.AllocId
		for _, gem := range tpInst.UpstreamQueueAttributeList {
			gemPortIDs = append(gemPortIDs, gem.GemportId)
		}
		allocIDs = appendUnique32bit(allocIDs, allocID)

		if tpInstanceExists {
			return allocID, gemPortIDs, techProfileInstance
		}

		for _, gemPortID := range gemPortIDs {
			allgemPortIDs = appendUnique32bit(allgemPortIDs, gemPortID)
		}
		logger.Infow(ctx, "allocated-tcont-and-gem-ports",
			log.Fields{
				"alloc-ids": allocIDs,
				"gemports":  allgemPortIDs,
				"device-id": f.deviceHandler.device.Id})
		// Send Tconts and GEM ports to KV store
		f.storeTcontsGEMPortsIntoKVStore(ctx, intfID, onuID, uniID, allocIDs, allgemPortIDs)
		return allocID, gemPortIDs, techProfileInstance
	default:
		logger.Errorw(ctx, "unknown-tech",
			log.Fields{
				"tpInst": tpInst})
		return 0, nil, nil
	}
}

func (f *OpenOltFlowMgr) storeTcontsGEMPortsIntoKVStore(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, allocID []uint32, gemPortIDs []uint32) {

	logger.Debugw(ctx, "storing-allocated-tconts-and-gem-ports-into-KV-store",
		log.Fields{
			"intf-id":     intfID,
			"onu-id":      onuID,
			"uni-id":      uniID,
			"alloc-id":    allocID,
			"gemport-ids": gemPortIDs,
			"device-id":   f.deviceHandler.device.Id})
	/* Update the allocated alloc_id and gem_port_id for the ONU/UNI to KV store  */
	if err := f.resourceMgr.UpdateAllocIdsForOnu(ctx, intfID, onuID, uniID, allocID); err != nil {
		logger.Errorw(ctx, "error-while-uploading-allocid-to-kv-store", log.Fields{"device-id": f.deviceHandler.device.Id})
	}
	if err := f.resourceMgr.UpdateGEMPortIDsForOnu(ctx, intfID, onuID, uniID, gemPortIDs); err != nil {
		logger.Errorw(ctx, "error-while-uploading-gemports-to-kv-store", log.Fields{"device-id": f.deviceHandler.device.Id})
	}

	logger.Infow(ctx, "stored-tconts-and-gem-into-kv-store-successfully", log.Fields{"device-id": f.deviceHandler.device.Id})
	for _, gemPort := range gemPortIDs {
		f.addGemPortToOnuInfoMap(ctx, intfID, onuID, gemPort)
	}
}

func (f *OpenOltFlowMgr) populateTechProfileForCurrentPonPort(ctx context.Context) error {
	for _, techRange := range f.resourceMgr.DevInfo.Ranges {
		for _, intfID := range techRange.IntfIds {
			if intfID == f.ponPortIdx { // initialize only for the pon port that this flow manager is managing
				var err error
				f.techprofile, err = tp.NewTechProfile(ctx, f.resourceMgr.PonRsrMgr, f.resourceMgr.PonRsrMgr.Backend,
					f.resourceMgr.PonRsrMgr.Address, f.deviceHandler.cm.Backend.PathPrefix)
				if err != nil || f.techprofile == nil {
					logger.Errorw(ctx, "failed-to-allocate-to-techprofile-for-pon-port", log.Fields{"intfID": intfID, "err": err})
					return fmt.Errorf("failed-to-allocate-tech-profile-for-pon-port--pon-%v-err-%v", intfID, err)
				}
				logger.Debugw(ctx, "init-tech-profile-done",
					log.Fields{
						"intf-id":   intfID,
						"device-id": f.deviceHandler.device.Id})
				return nil
			}
		}
	}
	logger.Errorw(ctx, "pon port not found in the the device pon port range", log.Fields{"intfID": f.ponPortIdx})
	return fmt.Errorf("pon-port-idx-not-found-in-the-device-info-pon-port-range-%v", f.ponPortIdx)
}

func (f *OpenOltFlowMgr) addUpstreamDataPathFlow(ctx context.Context, flowContext *flowContext) error {
	flowContext.classifier[PacketTagType] = SingleTag // FIXME: This hardcoding needs to be removed.
	logger.Debugw(ctx, "adding-upstream-data-flow",
		log.Fields{
			"uplinkClassifier": flowContext.classifier,
			"uplinkAction":     flowContext.action})
	return f.addSymmetricDataPathFlow(ctx, flowContext, Upstream)
	/* TODO: Install Secondary EAP on the subscriber vlan */
}

func (f *OpenOltFlowMgr) addDownstreamDataPathFlow(ctx context.Context, flowContext *flowContext) error {
	downlinkClassifier := flowContext.classifier
	downlinkAction := flowContext.action

	// TODO: For now mark the PacketTagType as SingleTag when OLT is transparent to VLAN
	// Per some deployment models, it is also possible that ONU operates on double tagged packets,
	// so this hardcoding of SingeTag packet-tag-type may be problem for such deployment models.
	// Need a better way for detection of packet tag type from OpenFlow message.
	if _, ok := downlinkClassifier[VlanVid]; !ok {
		downlinkClassifier[PacketTagType] = SingleTag
	} else {
		downlinkClassifier[PacketTagType] = DoubleTag
		downlinkAction[PopVlan] = true
	}
	logger.Debugw(ctx, "adding-downstream-data-flow",
		log.Fields{
			"downlinkClassifier": downlinkClassifier,
			"downlinkAction":     downlinkAction})
	// Ignore Downlink trap flow given by core, cannot do anything with this flow */
	if vlan, exists := downlinkClassifier[VlanVid]; exists {
		if vlan.(uint32) == (uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 4000) { //private VLAN given by core
			if metadata, exists := downlinkClassifier[Metadata]; exists { // inport is filled in metadata by core
				if uint32(metadata.(uint64)) == plt.MkUniPortNum(ctx, flowContext.intfID, flowContext.onuID, flowContext.uniID) {
					logger.Infow(ctx, "ignoring-dl-trap-device-flow-from-core",
						log.Fields{
							"flow":      flowContext.logicalFlow,
							"device-id": f.deviceHandler.device.Id,
							"onu-id":    flowContext.onuID,
							"intf-id":   flowContext.intfID})
					return nil
				}
			}
		}
	}

	// vlan_vid is a uint32.  must be type asserted as such or conversion fails
	dlClVid, ok := downlinkClassifier[VlanVid].(uint32)
	if ok {
		downlinkAction[VlanVid] = dlClVid & 0xfff
	}

	return f.addSymmetricDataPathFlow(ctx, flowContext, Downstream)
}

func (f *OpenOltFlowMgr) addSymmetricDataPathFlow(ctx context.Context, flowContext *flowContext, direction string) error {

	intfID := flowContext.intfID
	onuID := flowContext.onuID
	uniID := flowContext.uniID
	classifier := flowContext.classifier
	action := flowContext.action
	allocID := flowContext.allocID
	gemPortID := flowContext.gemPortID
	tpID := flowContext.tpID
	logicalFlow := flowContext.logicalFlow
	logger.Infow(ctx, "adding-hsia-flow",
		log.Fields{
			"intf-id":     intfID,
			"onu-id":      onuID,
			"uni-id":      uniID,
			"device-id":   f.deviceHandler.device.Id,
			"classifier":  classifier,
			"action":      action,
			"direction":   direction,
			"alloc-id":    allocID,
			"gemport-id":  gemPortID,
			"logicalflow": *logicalFlow})

	if present := f.resourceMgr.IsFlowOnKvStore(ctx, intfID, int32(onuID), int32(uniID), logicalFlow.Id); present {
		logger.Infow(ctx, "flow-already-exists",
			log.Fields{
				"device-id": f.deviceHandler.device.Id,
				"intf-id":   intfID,
				"onu-id":    onuID})
		return nil
	}
	classifierProto, err := makeOpenOltClassifierField(classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"classifier": classifier, "device-id": f.deviceHandler.device.Id}, err).Log()
	}
	logger.Debugw(ctx, "created-classifier-proto",
		log.Fields{
			"classifier": *classifierProto,
			"device-id":  f.deviceHandler.device.Id})
	actionProto, err := makeOpenOltActionField(action, classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"action": action, "device-id": f.deviceHandler.device.Id}, err).Log()
	}
	logger.Debugw(ctx, "created-action-proto",
		log.Fields{
			"action":    *actionProto,
			"device-id": f.deviceHandler.device.Id})
	networkIntfID, err := getNniIntfID(ctx, classifier, action)
	if err != nil {
		return olterrors.NewErrNotFound("nni-interface-id",
			log.Fields{
				"classifier": classifier,
				"action":     action,
				"device-id":  f.deviceHandler.device.Id,
			}, err).Log()
	}

	flow := openoltpb2.Flow{AccessIntfId: int32(intfID),
		OnuId:         int32(onuID),
		UniId:         int32(uniID),
		FlowId:        logicalFlow.Id,
		FlowType:      direction,
		AllocId:       int32(allocID),
		NetworkIntfId: int32(networkIntfID),
		GemportId:     int32(gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        flowContext.portNo,
		TechProfileId: tpID,
		ReplicateFlow: len(flowContext.pbitToGem) > 0,
		PbitToGemport: flowContext.pbitToGem,
		GemportToAes:  flowContext.gemToAes,
	}
	if err := f.addFlowToDevice(ctx, logicalFlow, &flow); err != nil {
		return olterrors.NewErrFlowOp("add", logicalFlow.Id, nil, err).Log()
	}
	logger.Infow(ctx, "hsia-flow-added-to-device-successfully",
		log.Fields{"direction": direction,
			"device-id": f.deviceHandler.device.Id,
			"flow":      flow,
			"intf-id":   intfID,
			"onu-id":    onuID})

	return nil
}

func (f *OpenOltFlowMgr) addDHCPTrapFlow(ctx context.Context, flowContext *flowContext) error {

	intfID := flowContext.intfID
	onuID := flowContext.onuID
	uniID := flowContext.uniID
	logicalFlow := flowContext.logicalFlow
	classifier := flowContext.classifier
	action := flowContext.action

	networkIntfID, err := getNniIntfID(ctx, classifier, action)
	if err != nil {
		return olterrors.NewErrNotFound("nni-interface-id", log.Fields{
			"classifier": classifier,
			"action":     action,
			"device-id":  f.deviceHandler.device.Id},
			err).Log()
	}

	// Clear the action map
	for k := range action {
		delete(action, k)
	}

	action[TrapToHost] = true
	classifier[UDPSrc] = uint32(68)
	classifier[UDPDst] = uint32(67)
	classifier[PacketTagType] = SingleTag

	if present := f.resourceMgr.IsFlowOnKvStore(ctx, intfID, int32(onuID), int32(uniID), logicalFlow.Id); present {
		logger.Infow(ctx, "flow-exists--not-re-adding",
			log.Fields{
				"device-id": f.deviceHandler.device.Id,
				"intf-id":   intfID,
				"onu-id":    onuID})
		return nil
	}

	logger.Debugw(ctx, "creating-ul-dhcp-flow",
		log.Fields{
			"ul_classifier": classifier,
			"ul_action":     action,
			"uplinkFlowId":  logicalFlow.Id,
			"intf-id":       intfID,
			"onu-id":        onuID,
			"device-id":     f.deviceHandler.device.Id})

	classifierProto, err := makeOpenOltClassifierField(classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"classifier": classifier}, err).Log()
	}
	logger.Debugw(ctx, "created-classifier-proto", log.Fields{"classifier": *classifierProto})
	actionProto, err := makeOpenOltActionField(action, classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"action": action}, err).Log()
	}

	dhcpFlow := openoltpb2.Flow{AccessIntfId: int32(intfID),
		OnuId:         int32(onuID),
		UniId:         int32(uniID),
		FlowId:        logicalFlow.Id,
		FlowType:      Upstream,
		AllocId:       int32(flowContext.allocID),
		NetworkIntfId: int32(networkIntfID),
		GemportId:     int32(flowContext.gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        flowContext.portNo,
		TechProfileId: flowContext.tpID,
		ReplicateFlow: len(flowContext.pbitToGem) > 0,
		PbitToGemport: flowContext.pbitToGem,
		GemportToAes:  flowContext.gemToAes,
	}
	if err := f.addFlowToDevice(ctx, logicalFlow, &dhcpFlow); err != nil {
		return olterrors.NewErrFlowOp("add", logicalFlow.Id, log.Fields{"dhcp-flow": dhcpFlow}, err).Log()
	}
	logger.Infow(ctx, "dhcp-ul-flow-added-to-device-successfully",
		log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"flow-id":   logicalFlow.Id,
			"intf-id":   intfID,
			"onu-id":    onuID})

	return nil
}

//addIGMPTrapFlow creates IGMP trap-to-host flow
func (f *OpenOltFlowMgr) addIGMPTrapFlow(ctx context.Context, flowContext *flowContext) error {
	delete(flowContext.classifier, VlanVid)
	return f.addUpstreamTrapFlow(ctx, flowContext)
}

//addUpstreamTrapFlow creates a trap-to-host flow
func (f *OpenOltFlowMgr) addUpstreamTrapFlow(ctx context.Context, flowContext *flowContext) error {

	intfID := flowContext.intfID
	onuID := flowContext.onuID
	uniID := flowContext.uniID
	logicalFlow := flowContext.logicalFlow
	classifier := flowContext.classifier
	action := flowContext.action

	networkIntfID, err := getNniIntfID(ctx, classifier, action)
	if err != nil {
		return olterrors.NewErrNotFound("nni-interface-id",
			log.Fields{
				"classifier": classifier,
				"action":     action,
				"device-id":  f.deviceHandler.device.Id},
			err).Log()
	}

	// Clear the action map
	for k := range action {
		delete(action, k)
	}

	action[TrapToHost] = true
	classifier[PacketTagType] = SingleTag

	if present := f.resourceMgr.IsFlowOnKvStore(ctx, networkIntfID, int32(onuID), int32(uniID), logicalFlow.Id); present {
		logger.Infow(ctx, "flow-exists-not-re-adding", log.Fields{"device-id": f.deviceHandler.device.Id})
		return nil
	}

	logger.Debugw(ctx, "creating-upstream-trap-flow",
		log.Fields{
			"ul_classifier": classifier,
			"ul_action":     action,
			"uplinkFlowId":  logicalFlow.Id,
			"device-id":     f.deviceHandler.device.Id,
			"intf-id":       intfID,
			"onu-id":        onuID})

	classifierProto, err := makeOpenOltClassifierField(classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"classifier": classifier, "device-id": f.deviceHandler.device.Id}, err).Log()
	}
	logger.Debugw(ctx, "created-classifier-proto",
		log.Fields{
			"classifier": *classifierProto,
			"device-id":  f.deviceHandler.device.Id})
	actionProto, err := makeOpenOltActionField(action, classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"action": action, "device-id": f.deviceHandler.device.Id}, err).Log()
	}

	flow := openoltpb2.Flow{AccessIntfId: int32(intfID),
		OnuId:         int32(onuID),
		UniId:         int32(uniID),
		FlowId:        logicalFlow.Id,
		FlowType:      Upstream,
		AllocId:       int32(flowContext.allocID),
		NetworkIntfId: int32(networkIntfID),
		GemportId:     int32(flowContext.gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        flowContext.portNo,
		TechProfileId: flowContext.tpID,
		ReplicateFlow: len(flowContext.pbitToGem) > 0,
		PbitToGemport: flowContext.pbitToGem,
		GemportToAes:  flowContext.gemToAes,
	}

	if err := f.addFlowToDevice(ctx, logicalFlow, &flow); err != nil {
		return olterrors.NewErrFlowOp("add", logicalFlow.Id, log.Fields{"flow": flow, "device-id": f.deviceHandler.device.Id}, err).Log()
	}

	return nil
}

// Add EthType flow to  device with mac, vlanId as classifier for upstream and downstream
func (f *OpenOltFlowMgr) addEthTypeBasedFlow(ctx context.Context, flowContext *flowContext, vlanID uint32, ethType uint32) error {
	intfID := flowContext.intfID
	onuID := flowContext.onuID
	uniID := flowContext.uniID
	portNo := flowContext.portNo
	allocID := flowContext.allocID
	gemPortID := flowContext.gemPortID
	logicalFlow := flowContext.logicalFlow
	classifier := flowContext.classifier
	action := flowContext.action

	logger.Infow(ctx, "adding-ethType-flow-to-device",
		log.Fields{
			"intf-id":    intfID,
			"onu-id":     onuID,
			"port-no":    portNo,
			"alloc-id":   allocID,
			"gemport-id": gemPortID,
			"vlan-id":    vlanID,
			"flow":       logicalFlow,
			"ethType":    ethType})

	uplinkClassifier := make(map[string]interface{})
	uplinkAction := make(map[string]interface{})

	// Fill Classfier
	uplinkClassifier[EthType] = ethType
	uplinkClassifier[PacketTagType] = SingleTag
	uplinkClassifier[VlanVid] = vlanID
	uplinkClassifier[VlanPcp] = classifier[VlanPcp]
	// Fill action
	uplinkAction[TrapToHost] = true
	if present := f.resourceMgr.IsFlowOnKvStore(ctx, intfID, int32(onuID), int32(uniID), logicalFlow.Id); present {
		logger.Infow(ctx, "flow-exists-not-re-adding", log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"onu-id":    onuID,
			"intf-id":   intfID,
			"ethType":   ethType})
		return nil
	}
	//Add Uplink EthType Flow
	logger.Debugw(ctx, "creating-ul-ethType-flow",
		log.Fields{
			"ul_classifier": uplinkClassifier,
			"ul_action":     uplinkAction,
			"uplinkFlowId":  logicalFlow.Id,
			"device-id":     f.deviceHandler.device.Id,
			"intf-id":       intfID,
			"onu-id":        onuID})

	classifierProto, err := makeOpenOltClassifierField(uplinkClassifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{
			"classifier": uplinkClassifier,
			"device-id":  f.deviceHandler.device.Id}, err).Log()
	}
	logger.Debugw(ctx, "created-classifier-proto",
		log.Fields{
			"classifier": *classifierProto,
			"device-id":  f.deviceHandler.device.Id})
	actionProto, err := makeOpenOltActionField(uplinkAction, uplinkClassifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"action": uplinkAction, "device-id": f.deviceHandler.device.Id}, err).Log()
	}
	logger.Debugw(ctx, "created-action-proto",
		log.Fields{
			"action":    *actionProto,
			"device-id": f.deviceHandler.device.Id})
	networkIntfID, err := getNniIntfID(ctx, classifier, action)
	if err != nil {
		return olterrors.NewErrNotFound("nni-interface-id", log.Fields{
			"classifier": classifier,
			"action":     action,
			"device-id":  f.deviceHandler.device.Id},
			err).Log()
	}

	upstreamFlow := openoltpb2.Flow{AccessIntfId: int32(intfID),
		OnuId:         int32(onuID),
		UniId:         int32(uniID),
		FlowId:        logicalFlow.Id,
		FlowType:      Upstream,
		AllocId:       int32(allocID),
		NetworkIntfId: int32(networkIntfID),
		GemportId:     int32(gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo,
		TechProfileId: flowContext.tpID,
		ReplicateFlow: len(flowContext.pbitToGem) > 0,
		PbitToGemport: flowContext.pbitToGem,
		GemportToAes:  flowContext.gemToAes,
	}
	if err := f.addFlowToDevice(ctx, logicalFlow, &upstreamFlow); err != nil {
		return olterrors.NewErrFlowOp("add", logicalFlow.Id, log.Fields{"flow": upstreamFlow}, err).Log()
	}
	logger.Infow(ctx, "ethType-ul-flow-added-to-device-successfully",
		log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"onu-id":    onuID,
			"intf-id":   intfID,
			"ethType":   ethType,
		})

	return nil
}

func makeOpenOltClassifierField(classifierInfo map[string]interface{}) (*openoltpb2.Classifier, error) {
	var classifier openoltpb2.Classifier

	classifier.EthType, _ = classifierInfo[EthType].(uint32)
	classifier.IpProto, _ = classifierInfo[IPProto].(uint32)
	if vlanID, ok := classifierInfo[VlanVid].(uint32); ok {
		if vlanID != ReservedVlan {
			vid := vlanID & VlanvIDMask
			classifier.OVid = vid
		}
	}
	// The classifierInfo[Metadata] carries the vlan that the OLT see when it receives packet from the ONU
	if metadata, ok := classifierInfo[Metadata].(uint64); ok {
		vid := uint32(metadata)
		// Set the OVid or IVid classifier based on the whether OLT is using a transparent tag or not
		// If OLT is using transparent tag mechanism, then it classifies whatever tag it sees to/from ONU which
		//is OVid from the perspective of the OLT. When OLT also places or pops the outer tag, then classifierInfo[Metadata]
		// becomes the IVid.
		if classifier.OVid != 0 && classifier.OVid != ReservedVlan { // This is case when classifier.OVid is not set
			if vid != ReservedVlan {
				classifier.IVid = vid
			}
		} else {
			if vid != ReservedVlan {
				classifier.OVid = vid
			}
		}
	}
	// Use VlanPCPMask (0xff) to signify NO PCP. Else use valid PCP (0 to 7)
	if vlanPcp, ok := classifierInfo[VlanPcp].(uint32); ok {
		classifier.OPbits = vlanPcp
	} else {
		classifier.OPbits = VlanPCPMask
	}
	classifier.SrcPort, _ = classifierInfo[UDPSrc].(uint32)
	classifier.DstPort, _ = classifierInfo[UDPDst].(uint32)
	classifier.DstIp, _ = classifierInfo[Ipv4Dst].(uint32)
	classifier.SrcIp, _ = classifierInfo[Ipv4Src].(uint32)
	classifier.DstMac, _ = classifierInfo[EthDst].([]uint8)
	if pktTagType, ok := classifierInfo[PacketTagType].(string); ok {
		classifier.PktTagType = pktTagType

		switch pktTagType {
		case SingleTag:
		case DoubleTag:
		case Untagged:
		default:
			return nil, olterrors.NewErrInvalidValue(log.Fields{"packet-tag-type": pktTagType}, nil)
		}
	}
	return &classifier, nil
}

func makeOpenOltActionField(actionInfo map[string]interface{}, classifierInfo map[string]interface{}) (*openoltpb2.Action, error) {
	var actionCmd openoltpb2.ActionCmd
	var action openoltpb2.Action
	action.Cmd = &actionCmd
	if _, ok := actionInfo[PopVlan]; ok {
		action.Cmd.RemoveOuterTag = true
		if _, ok := actionInfo[VlanPcp]; ok {
			action.Cmd.RemarkInnerPbits = true
			action.IPbits = actionInfo[VlanPcp].(uint32)
			if _, ok := actionInfo[VlanVid]; ok {
				action.Cmd.TranslateInnerTag = true
				action.IVid = actionInfo[VlanVid].(uint32)
			}
		}
	} else if _, ok := actionInfo[PushVlan]; ok {
		action.OVid = actionInfo[VlanVid].(uint32)
		action.Cmd.AddOuterTag = true
		if _, ok := actionInfo[VlanPcp]; ok {
			action.OPbits = actionInfo[VlanPcp].(uint32)
			action.Cmd.RemarkOuterPbits = true
			if _, ok := classifierInfo[VlanVid]; ok {
				action.IVid = classifierInfo[VlanVid].(uint32)
				action.Cmd.TranslateInnerTag = true
			}
		}
	} else if _, ok := actionInfo[TrapToHost]; ok {
		action.Cmd.TrapToHost = actionInfo[TrapToHost].(bool)
	}
	// When OLT is transparent to vlans no-action is valid.
	/*
		else {
			return nil, olterrors.NewErrInvalidValue(log.Fields{"action-command": actionInfo}, nil)
		}
	*/
	return &action, nil
}

// getTPpath return the ETCD path for a given UNI port
func (f *OpenOltFlowMgr) getTPpath(ctx context.Context, intfID uint32, uniPath string, TpID uint32) string {
	return f.techprofile.GetTechProfileInstanceKey(ctx, TpID, uniPath)
}

// DeleteTechProfileInstances removes the tech profile instances from persistent storage
// We also force release scheduler and queues associated with the tp instance. Theoretically there could be
// an issue if the upstream scheduler (DBA) is shared across multiple UNI and we force release it, given that
// this function is only meant to clean up TP instances of a given UNI. But in practicality this  routine
// is only meant to be called when the clean up of resource for the whole ONU is taking place.
// The reason for introducing the force cleanup of scheduler and queues (on the OLT) was introduced here
// because it was observed that if the ONU device was deleted too soon after the flows were
// unprovisioned on that ONU, the scheduler and queue removal pertinent to that ONU would remain
// uncleaned on the OLT. So we force clean up here and ignore any error that OLT returns during the
// force cleanup (possible if the OLT has already cleared those resources).
func (f *OpenOltFlowMgr) DeleteTechProfileInstances(ctx context.Context, intfID uint32, onuID uint32, uniID uint32) error {
	tpIDList := f.resourceMgr.GetTechProfileIDForOnu(ctx, intfID, onuID, uniID)
	uniPortName := getUniPortPath(f.deviceHandler.device.Id, intfID, int32(onuID), int32(uniID))

	for _, tpID := range tpIDList {

		// Force cleanup scheduler/queues -- start
		uniPortNum := plt.MkUniPortNum(ctx, intfID, onuID, uniID)
		uni := getUniPortPath(f.deviceHandler.device.Id, intfID, int32(onuID), int32(uniID))
		tpPath := f.getTPpath(ctx, intfID, uni, tpID)
		tpInst, err := f.techprofile.GetTPInstance(ctx, tpPath)
		if err != nil || tpInst == nil { // This should not happen, something wrong in KV backend transaction
			logger.Warnw(ctx, "tech-profile-not-in-kv-store",
				log.Fields{
					"tp-id": tpID,
					"path":  tpPath})
		}
		switch tpInstance := tpInst.(type) {
		case *tp_pb.TechProfileInstance:
			f.forceRemoveSchedulerQueues(ctx, schedQueue{direction: tp_pb.Direction_UPSTREAM, intfID: intfID, onuID: onuID, uniID: uniID, tpID: tpID, uniPort: uniPortNum, tpInst: tpInstance})
			f.forceRemoveSchedulerQueues(ctx, schedQueue{direction: tp_pb.Direction_DOWNSTREAM, intfID: intfID, onuID: onuID, uniID: uniID, tpID: tpID, uniPort: uniPortNum, tpInst: tpInstance})
		}
		// Force cleanup scheduler/queues -- end

		// Now remove the tp instance
		if err := f.DeleteTechProfileInstance(ctx, intfID, onuID, uniID, uniPortName, tpID); err != nil {
			logger.Errorw(ctx, "delete-tech-profile-failed", log.Fields{"err": err, "device-id": f.deviceHandler.device.Id})
			// return err
			// We should continue to delete tech-profile instances for other TP IDs
		}
		logger.Debugw(ctx, "tech-profile-instance-deleted", log.Fields{"device-id": f.deviceHandler.device.Id, "uniPortName": uniPortName, "tp-id": tpID})
	}
	return nil
}

// DeleteTechProfileInstance removes the tech profile instance from persistent storage
func (f *OpenOltFlowMgr) DeleteTechProfileInstance(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, uniPortName string, tpID uint32) error {
	if uniPortName == "" {
		uniPortName = getUniPortPath(f.deviceHandler.device.Id, intfID, int32(onuID), int32(uniID))
	}
	if err := f.techprofile.DeleteTechProfileInstance(ctx, tpID, uniPortName); err != nil {
		return olterrors.NewErrAdapter("failed-to-delete-tp-instance-from-kv-store",
			log.Fields{
				"tp-id":         tpID,
				"uni-port-name": uniPortName,
				"device-id":     f.deviceHandler.device.Id}, err)
	}
	return nil
}

func (f *OpenOltFlowMgr) addFlowToDevice(ctx context.Context, logicalFlow *ofp.OfpFlowStats, deviceFlow *openoltpb2.Flow) error {

	var intfID uint32
	/* For flows which trap out of the NNI, the AccessIntfId is invalid
	   (set to -1). In such cases, we need to refer to the NetworkIntfId .
	*/
	if deviceFlow.AccessIntfId != -1 {
		intfID = uint32(deviceFlow.AccessIntfId)
	} else {
		// We need to log the valid interface ID.
		// For trap-on-nni flows, the access_intf_id is invalid (-1), so choose the network_intf_id.
		intfID = uint32(deviceFlow.NetworkIntfId)
	}

	logger.Debugw(ctx, "sending-flow-to-device-via-grpc", log.Fields{
		"flow":      *deviceFlow,
		"device-id": f.deviceHandler.device.Id,
		"intf-id":   intfID})
	_, err := f.deviceHandler.Client.FlowAdd(log.WithSpanFromContext(context.Background(), ctx), deviceFlow)

	st, _ := status.FromError(err)
	if st.Code() == codes.AlreadyExists {
		logger.Debug(ctx, "flow-already-exists", log.Fields{
			"err":        err,
			"deviceFlow": deviceFlow,
			"device-id":  f.deviceHandler.device.Id,
			"intf-id":    intfID})
		return nil
	}

	if err != nil {
		logger.Errorw(ctx, "failed-to-add-flow-to-device",
			log.Fields{"err": err,
				"device-flow": deviceFlow,
				"device-id":   f.deviceHandler.device.Id,
				"intf-id":     intfID})
		return err
	}
	logger.Infow(ctx, "flow-added-to-device-successfully ",
		log.Fields{
			"flow":      *deviceFlow,
			"device-id": f.deviceHandler.device.Id,
			"intf-id":   intfID})

	// Case of trap-on-nni flow when deviceFlow.AccessIntfId is invalid (-1)
	if deviceFlow.AccessIntfId != -1 {
		// No need to register the flow if it is a trap on nni flow.
		if err := f.registerFlow(ctx, logicalFlow, deviceFlow); err != nil {
			logger.Errorw(ctx, "failed-to-register-flow", log.Fields{"err": err})
			return err
		}
	}
	return nil
}

func (f *OpenOltFlowMgr) removeFlowFromDevice(ctx context.Context, deviceFlow *openoltpb2.Flow, ofFlowID uint64) error {
	logger.Debugw(ctx, "sending-flow-to-device-via-grpc",
		log.Fields{
			"flow":      *deviceFlow,
			"device-id": f.deviceHandler.device.Id})
	_, err := f.deviceHandler.Client.FlowRemove(log.WithSpanFromContext(context.Background(), ctx), deviceFlow)
	if err != nil {
		if f.deviceHandler.device.ConnectStatus == common.ConnectStatus_UNREACHABLE {
			logger.Warnw(ctx, "can-not-remove-flow-from-device--unreachable",
				log.Fields{
					"err":        err,
					"deviceFlow": deviceFlow,
					"device-id":  f.deviceHandler.device.Id})
			//Assume the flow is removed
			return nil
		}
		return olterrors.NewErrFlowOp("remove", deviceFlow.FlowId, log.Fields{"deviceFlow": deviceFlow}, err)

	}
	logger.Infow(ctx, "flow-removed-from-device-successfully", log.Fields{
		"of-flow-id": ofFlowID,
		"flow":       *deviceFlow,
		"device-id":  f.deviceHandler.device.Id,
	})
	return nil
}

func (f *OpenOltFlowMgr) addLLDPFlow(ctx context.Context, flow *ofp.OfpFlowStats, portNo uint32) error {

	classifierInfo := make(map[string]interface{})
	actionInfo := make(map[string]interface{})

	classifierInfo[EthType] = uint32(LldpEthType)
	classifierInfo[PacketTagType] = Untagged
	actionInfo[TrapToHost] = true

	// LLDP flow is installed to trap LLDP packets on the NNI port.
	// We manage flow_id resource pool on per PON port basis.
	// Since this situation is tricky, as a hack, we pass the NNI port
	// index (network_intf_id) as PON port Index for the flow_id resource
	// pool. Also, there is no ONU Id available for trapping LLDP packets
	// on NNI port, use onu_id as -1 (invalid)
	// ****************** CAVEAT *******************
	// This logic works if the NNI Port Id falls within the same valid
	// range of PON Port Ids. If this doesn't work for some OLT Vendor
	// we need to have a re-look at this.
	// *********************************************

	var onuID = -1
	var uniID = -1
	var gemPortID = -1

	networkInterfaceID, err := plt.IntfIDFromNniPortNum(ctx, portNo)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"nni-port-number": portNo}, err).Log()
	}
	if present := f.resourceMgr.IsFlowOnKvStore(ctx, networkInterfaceID, int32(onuID), int32(uniID), flow.Id); present {
		logger.Infow(ctx, "flow-exists--not-re-adding", log.Fields{"device-id": f.deviceHandler.device.Id})
		return nil
	}

	classifierProto, err := makeOpenOltClassifierField(classifierInfo)
	if err != nil {
		return olterrors.NewErrInvalidValue(
			log.Fields{
				"classifier": classifierInfo,
				"device-id":  f.deviceHandler.device.Id}, err)
	}
	logger.Debugw(ctx, "created-classifier-proto",
		log.Fields{
			"classifier": *classifierProto,
			"device-id":  f.deviceHandler.device.Id})
	actionProto, err := makeOpenOltActionField(actionInfo, classifierInfo)
	if err != nil {
		return olterrors.NewErrInvalidValue(
			log.Fields{
				"action":    actionInfo,
				"device-id": f.deviceHandler.device.Id}, err)
	}
	logger.Debugw(ctx, "created-action-proto",
		log.Fields{
			"action":    *actionProto,
			"device-id": f.deviceHandler.device.Id})

	downstreamflow := openoltpb2.Flow{AccessIntfId: int32(-1), // AccessIntfId not required
		OnuId:         int32(onuID), // OnuId not required
		UniId:         int32(uniID), // UniId not used
		FlowId:        flow.Id,
		FlowType:      Downstream,
		NetworkIntfId: int32(networkInterfaceID),
		GemportId:     int32(gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(flow.Priority),
		Cookie:        flow.Cookie,
		PortNo:        portNo}
	if err := f.addFlowToDevice(ctx, flow, &downstreamflow); err != nil {
		return olterrors.NewErrFlowOp("add", flow.Id,
			log.Fields{
				"flow":      downstreamflow,
				"device-id": f.deviceHandler.device.Id}, err)
	}
	logger.Infow(ctx, "lldp-trap-on-nni-flow-added-to-device-successfully",
		log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"onu-id":    onuID,
			"flow-id":   flow.Id})

	return nil
}

func getUniPortPath(oltID string, intfID uint32, onuID int32, uniID int32) string {
	return fmt.Sprintf("olt-{%s}/pon-{%d}/onu-{%d}/uni-{%d}", oltID, intfID, onuID, uniID)
}

//getOnuDevice to fetch onu from cache or core.
func (f *OpenOltFlowMgr) getOnuDevice(ctx context.Context, intfID uint32, onuID uint32) (*OnuDevice, error) {
	onuKey := f.deviceHandler.formOnuKey(intfID, onuID)
	onuDev, ok := f.deviceHandler.onus.Load(onuKey)
	if !ok {
		logger.Debugw(ctx, "couldnt-find-onu-in-cache",
			log.Fields{
				"intf-id":   intfID,
				"onu-id":    onuID,
				"device-id": f.deviceHandler.device.Id})
		onuDevice, err := f.getChildDevice(ctx, intfID, onuID)
		if err != nil {
			return nil, olterrors.NewErrNotFound("onu-child-device",
				log.Fields{
					"onu-id":    onuID,
					"intf-id":   intfID,
					"device-id": f.deviceHandler.device.Id}, err)
		}
		onuDev = NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuDevice.ProxyAddress.OnuId, onuDevice.ProxyAddress.ChannelId, onuDevice.ProxyAddress.DeviceId, false, onuDevice.AdapterEndpoint)
		//better to ad the device to cache here.
		f.deviceHandler.StoreOnuDevice(onuDev.(*OnuDevice))
	} else {
		logger.Debugw(ctx, "found-onu-in-cache",
			log.Fields{
				"intf-id":   intfID,
				"onu-id":    onuID,
				"device-id": f.deviceHandler.device.Id})
	}

	return onuDev.(*OnuDevice), nil
}

//getChildDevice to fetch onu
func (f *OpenOltFlowMgr) getChildDevice(ctx context.Context, intfID uint32, onuID uint32) (*voltha.Device, error) {
	logger.Infow(ctx, "GetChildDevice",
		log.Fields{
			"pon-port":  intfID,
			"onu-id":    onuID,
			"device-id": f.deviceHandler.device.Id})
	parentPortNo := plt.IntfIDToPortNo(intfID, voltha.Port_PON_OLT)
	onuDevice, err := f.deviceHandler.GetChildDevice(ctx, parentPortNo, onuID)
	if err != nil {
		return nil, olterrors.NewErrNotFound("onu",
			log.Fields{
				"interface-id": parentPortNo,
				"onu-id":       onuID,
				"device-id":    f.deviceHandler.device.Id},
			err)
	}
	logger.Infow(ctx, "successfully-received-child-device-from-core",
		log.Fields{
			"device-id":       f.deviceHandler.device.Id,
			"child_device_id": onuDevice.Id,
			"child_device_sn": onuDevice.SerialNumber})
	return onuDevice, nil
}

func (f *OpenOltFlowMgr) sendDeleteGemPortToChild(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, gemPortID uint32, tpPath string) error {
	onuDev, err := f.getOnuDevice(ctx, intfID, onuID)
	if err != nil {
		logger.Debugw(ctx, "couldnt-find-onu-child-device",
			log.Fields{
				"intf-id":   intfID,
				"onu-id":    onuID,
				"uni-id":    uniID,
				"device-id": f.deviceHandler.device.Id})
		return err
	}

	delGemPortMsg := &ia.DeleteGemPortMessage{
		DeviceId:       onuDev.deviceID,
		UniId:          uniID,
		TpInstancePath: tpPath,
		GemPortId:      gemPortID,
	}
	logger.Debugw(ctx, "sending-gem-port-delete-to-openonu-adapter", log.Fields{"msg": *delGemPortMsg, "child-device-id": onuDev.deviceID})

	if err := f.deviceHandler.sendDeleteGemPortToChildAdapter(ctx, onuDev.adapterEndpoint, delGemPortMsg); err != nil {
		return olterrors.NewErrCommunication("send-delete-gem-port-to-onu-adapter",
			log.Fields{
				"from-adapter":  f.deviceHandler.openOLT.config.AdapterEndpoint,
				"to-adapter":    onuDev.adapterEndpoint,
				"onu-id":        onuDev.deviceID,
				"proxyDeviceID": onuDev.proxyDeviceID,
				"device-id":     f.deviceHandler.device.Id}, err)
	}

	logger.Infow(ctx, "success-sending-del-gem-port-to-onu-adapter",
		log.Fields{
			"msg":             delGemPortMsg,
			"from-adapter":    f.deviceHandler.device.Type,
			"to-adapter":      onuDev.deviceType,
			"device-id":       f.deviceHandler.device.Id,
			"child-device-id": onuDev.deviceID})
	return nil
}

func (f *OpenOltFlowMgr) sendDeleteTcontToChild(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, allocID uint32, tpPath string) error {
	onuDev, err := f.getOnuDevice(ctx, intfID, onuID)
	if err != nil {
		logger.Warnw(ctx, "couldnt-find-onu-child-device",
			log.Fields{
				"intf-id":   intfID,
				"onu-id":    onuID,
				"uni-id":    uniID,
				"device-id": f.deviceHandler.device.Id})
		return err
	}

	delTcontMsg := &ia.DeleteTcontMessage{
		DeviceId:       onuDev.deviceID,
		UniId:          uniID,
		TpInstancePath: tpPath,
		AllocId:        allocID,
	}

	logger.Debugw(ctx, "sending-tcont-delete-to-openonu-adapter",
		log.Fields{
			"msg":       *delTcontMsg,
			"device-id": f.deviceHandler.device.Id})

	if err := f.deviceHandler.sendDeleteTContToChildAdapter(ctx, onuDev.adapterEndpoint, delTcontMsg); err != nil {
		return olterrors.NewErrCommunication("send-delete-tcont-to-onu-adapter",
			log.Fields{
				"from-adapter":  f.deviceHandler.openOLT.config.AdapterEndpoint,
				"to-adapter":    onuDev.adapterEndpoint,
				"onu-id":        onuDev.deviceID,
				"proxyDeviceID": onuDev.proxyDeviceID,
				"device-id":     f.deviceHandler.device.Id}, err)

	}
	logger.Infow(ctx, "success-sending-del-tcont-to-onu-adapter",
		log.Fields{
			"msg":             delTcontMsg,
			"device-id":       f.deviceHandler.device.Id,
			"child-device-id": onuDev.deviceID})
	return nil
}

// Once the gemport is released for a given onu, it also has to be cleared from local cache
// which was used for deriving the gemport->logicalPortNo during packet-in.
// Otherwise stale info continues to exist after gemport is freed and wrong logicalPortNo
// is conveyed to ONOS during packet-in OF message.
func (f *OpenOltFlowMgr) deleteGemPortFromLocalCache(ctx context.Context, intfID uint32, onuID uint32, gemPortID uint32) {
	logger.Infow(ctx, "deleting-gem-from-local-cache",
		log.Fields{
			"gem-port-id": gemPortID,
			"intf-id":     intfID,
			"onu-id":      onuID,
			"device-id":   f.deviceHandler.device.Id})
	f.onuGemInfoLock.RLock()
	onugem, ok := f.onuGemInfoMap[onuID]
	f.onuGemInfoLock.RUnlock()
	if !ok {
		logger.Warnw(ctx, "onu gem info already cleared from cache", log.Fields{
			"gem-port-id": gemPortID,
			"intf-id":     intfID,
			"onu-id":      onuID,
			"device-id":   f.deviceHandler.device.Id})
		return
	}
deleteLoop:
	for j, gem := range onugem.GemPorts {
		// If the gemport is found, delete it from local cache.
		if gem == gemPortID {
			onugem.GemPorts = append(onugem.GemPorts[:j], onugem.GemPorts[j+1:]...)
			f.onuGemInfoLock.Lock()
			f.onuGemInfoMap[onuID] = onugem
			f.onuGemInfoLock.Unlock()
			logger.Infow(ctx, "removed-gemport-from-local-cache",
				log.Fields{
					"intf-id":           intfID,
					"onu-id":            onuID,
					"deletedgemport-id": gemPortID,
					"gemports":          onugem.GemPorts,
					"device-id":         f.deviceHandler.device.Id})
			break deleteLoop
		}
	}
}

//clearResources clears pon resources in kv store and the device
// nolint: gocyclo
func (f *OpenOltFlowMgr) clearResources(ctx context.Context, intfID uint32, onuID int32, uniID int32,
	flowID uint64, portNum uint32, tpID uint32, sendDeleteGemRequest bool) error {

	logger.Debugw(ctx, "clearing-resources", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID, "tpID": tpID})

	uni := getUniPortPath(f.deviceHandler.device.Id, intfID, onuID, uniID)
	tpPath := f.getTPpath(ctx, intfID, uni, tpID)
	logger.Debugw(ctx, "getting-techprofile-instance-for-subscriber",
		log.Fields{
			"tpPath":    tpPath,
			"device-id": f.deviceHandler.device.Id})

	techprofileInst, err := f.techprofile.GetTPInstance(ctx, tpPath)
	if err != nil || techprofileInst == nil {
		// The child device is possibly deleted which in turn had cleaned up all the resources (including tp instances), check..
		childDevice, _ := f.getChildDevice(ctx, intfID, uint32(onuID)) // do not care about the error code
		if childDevice == nil {
			// happens when subscriber un-provision is immediately followed by child device delete
			// before all the flow removes are processed, the child device delete has already arrived and cleaned up all the resources
			logger.Warnw(ctx, "child device and its associated resources are already cleared", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID})
			return nil
		}
		return olterrors.NewErrNotFound("tech-profile-in-kv-store",
			log.Fields{
				"tp-id": tpID,
				"path":  tpPath}, err)
	}

	var allGemPortsFree = true
	switch techprofileInst := techprofileInst.(type) {
	case *tp_pb.TechProfileInstance:
		for _, gemPort := range techprofileInst.UpstreamGemPortAttributeList {
			gemPortID := gemPort.GemportId
			used := f.isGemPortUsedByAnotherFlow(gemPortID, flowID)
			if used {
				f.gemToFlowIDsKey.RLock()
				flowIDs := f.gemToFlowIDs[gemPortID]
				f.gemToFlowIDsKey.RUnlock()

				for i, flowIDinMap := range flowIDs {
					if flowIDinMap == flowID {
						flowIDs = append(flowIDs[:i], flowIDs[i+1:]...)
						f.gemToFlowIDsKey.Lock()
						f.gemToFlowIDs[gemPortID] = flowIDs
						f.gemToFlowIDsKey.Unlock()
						// everytime gemToFlowIDs cache is updated the same should be updated
						// in kv store by calling UpdateFlowIDsForGem
						if err := f.resourceMgr.UpdateFlowIDsForGem(ctx, intfID, gemPortID, flowIDs); err != nil {
							return err
						}
						break
					}
				}
				logger.Debugw(ctx, "gem-port-id-is-still-used-by-other-flows",
					log.Fields{
						"gemport-id":  gemPortID,
						"usedByFlows": flowIDs,
						"currentFlow": flowID,
						"device-id":   f.deviceHandler.device.Id})
				allGemPortsFree = false
			}
		}
		if !allGemPortsFree {
			return nil
		}
	}

	logger.Debugw(ctx, "all-gem-ports-are-free-to-be-deleted", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID, "tpID": tpID})
	switch techprofileInst := techprofileInst.(type) {
	case *tp_pb.TechProfileInstance:
		schedQueue := schedQueue{direction: tp_pb.Direction_UPSTREAM, intfID: intfID, onuID: uint32(onuID), uniID: uint32(uniID), tpID: tpID, uniPort: portNum, tpInst: techprofileInst}
		allocExists := f.isAllocUsedByAnotherUNI(ctx, schedQueue)
		if !allocExists {
			if err := f.resourceMgr.RemoveTechProfileIDForOnu(ctx, intfID, uint32(onuID), uint32(uniID), tpID); err != nil {
				logger.Warn(ctx, err)
			}
			if err := f.DeleteTechProfileInstance(ctx, intfID, uint32(onuID), uint32(uniID), "", tpID); err != nil {
				logger.Warn(ctx, err)
			}

			if KvStoreMeter, _ := f.resourceMgr.GetMeterInfoForOnu(ctx, "upstream", intfID, uint32(onuID), uint32(uniID), tpID); KvStoreMeter != nil {
				if err := f.RemoveSchedulerQueues(ctx, schedQueue); err != nil {
					logger.Warn(ctx, err)
				}
			}
			f.resourceMgr.FreeAllocID(ctx, intfID, uint32(onuID), uint32(uniID), techprofileInst.UsScheduler.AllocId)

			schedQueue.direction = tp_pb.Direction_DOWNSTREAM
			if KvStoreMeter, _ := f.resourceMgr.GetMeterInfoForOnu(ctx, "downstream", intfID, uint32(onuID), uint32(uniID), tpID); KvStoreMeter != nil {
				if err := f.RemoveSchedulerQueues(ctx, schedQueue); err != nil {
					logger.Warn(ctx, err)
				}
			}

			for _, gemPort := range techprofileInst.UpstreamGemPortAttributeList {
				gemPortID := gemPort.GemportId
				f.deleteGemPortFromLocalCache(ctx, intfID, uint32(onuID), gemPortID)
				_ = f.resourceMgr.RemoveGemFromOnuGemInfo(ctx, intfID, uint32(onuID), gemPortID) // ignore error and proceed.

				if err := f.resourceMgr.DeleteFlowIDsForGem(ctx, intfID, gemPortID); err == nil {
					//everytime an entry is deleted from gemToFlowIDs cache, the same should be updated in kv as well
					// by calling DeleteFlowIDsForGem
					f.gemToFlowIDsKey.Lock()
					delete(f.gemToFlowIDs, gemPortID)
					f.gemToFlowIDsKey.Unlock()
				} else {
					logger.Errorw(ctx, "error-removing-flow-ids-of-gem-port",
						log.Fields{
							"err":        err,
							"intf":       intfID,
							"onu-id":     onuID,
							"uni-id":     uniID,
							"device-id":  f.deviceHandler.device.Id,
							"gemport-id": gemPortID})
				}

				f.resourceMgr.FreeGemPortID(ctx, intfID, uint32(onuID), uint32(uniID), gemPortID)

				// Delete the gem port on the ONU.
				if sendDeleteGemRequest {
					if err := f.sendDeleteGemPortToChild(ctx, intfID, uint32(onuID), uint32(uniID), gemPortID, tpPath); err != nil {
						logger.Errorw(ctx, "error-processing-delete-gem-port-towards-onu",
							log.Fields{
								"err":        err,
								"intfID":     intfID,
								"onu-id":     onuID,
								"uni-id":     uniID,
								"device-id":  f.deviceHandler.device.Id,
								"gemport-id": gemPortID})
					}
				}
			}
		}
	case *tp_pb.EponTechProfileInstance:
		if err := f.resourceMgr.RemoveTechProfileIDForOnu(ctx, intfID, uint32(onuID), uint32(uniID), tpID); err != nil {
			logger.Warn(ctx, err)
		}
		if err := f.DeleteTechProfileInstance(ctx, intfID, uint32(onuID), uint32(uniID), "", tpID); err != nil {
			logger.Warn(ctx, err)
		}
		f.resourceMgr.FreeAllocID(ctx, intfID, uint32(onuID), uint32(uniID), techprofileInst.AllocId)
		// Delete the TCONT on the ONU.
		if err := f.sendDeleteTcontToChild(ctx, intfID, uint32(onuID), uint32(uniID), techprofileInst.AllocId, tpPath); err != nil {
			logger.Errorw(ctx, "error-processing-delete-tcont-towards-onu",
				log.Fields{
					"intf":      intfID,
					"onu-id":    onuID,
					"uni-id":    uniID,
					"device-id": f.deviceHandler.device.Id,
					"alloc-id":  techprofileInst.AllocId,
					"error":     err})
		}
	default:
		logger.Errorw(ctx, "error-unknown-tech",
			log.Fields{
				"techprofileInst": techprofileInst})
	}
	return nil
}

// nolint: gocyclo
func (f *OpenOltFlowMgr) clearFlowFromDeviceAndResourceManager(ctx context.Context, flow *ofp.OfpFlowStats, flowDirection string) error {
	logger.Infow(ctx, "clear-flow-from-resource-manager",
		log.Fields{
			"flowDirection": flowDirection,
			"flow":          *flow,
			"device-id":     f.deviceHandler.device.Id})

	if flowDirection == Multicast {
		return f.clearMulticastFlowFromResourceManager(ctx, flow)
	}

	var ethType, ipProto, inPort uint32
	for _, field := range flows.GetOfbFields(flow) {
		if field.Type == flows.IP_PROTO {
			ipProto = field.GetIpProto()
			logger.Debugw(ctx, "field-type-ip-proto", log.Fields{"ipProto": ipProto})
		} else if field.Type == flows.ETH_TYPE {
			ethType = field.GetEthType()
			logger.Debugw(ctx, "field-type-eth-type", log.Fields{"ethType": ethType})
		} else if field.Type == flows.IN_PORT {
			inPort = field.GetPort()
			logger.Debugw(ctx, "field-type-in-port", log.Fields{"inPort": inPort})
		}
	}
	portType := plt.IntfIDToPortTypeName(inPort)
	if (ethType == uint32(LldpEthType) || ipProto == uint32(IPProtoDhcp) || ipProto == uint32(IgmpProto)) &&
		(portType == voltha.Port_ETHERNET_NNI) {
		removeFlowMessage := openoltpb2.Flow{FlowId: flow.Id, AccessIntfId: -1, OnuId: -1, UniId: -1, TechProfileId: 0, FlowType: Downstream}
		logger.Debugw(ctx, "nni-trap-flow-to-be-deleted", log.Fields{"flow": flow})
		return f.removeFlowFromDevice(ctx, &removeFlowMessage, flow.Id)
		// No more processing needed for trap from nni flows.
	}

	portNum, Intf, onu, uni, _, _, err := plt.FlowExtractInfo(ctx, flow, flowDirection)
	if err != nil {
		logger.Error(ctx, err)
		return err
	}
	onuID := int32(onu)
	uniID := int32(uni)
	tpID, err := getTpIDFromFlow(ctx, flow)
	if err != nil {
		return olterrors.NewErrNotFound("tp-id",
			log.Fields{
				"flow":      flow,
				"intf-id":   Intf,
				"onu-id":    onuID,
				"uni-id":    uniID,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	logger.Infow(ctx, "extracted-access-info-from-flow-to-be-deleted",
		log.Fields{
			"flow-id": flow.Id,
			"intf-id": Intf,
			"onu-id":  onuID,
			"uni-id":  uniID})

	removeFlowMessage := openoltpb2.Flow{FlowId: flow.Id, AccessIntfId: int32(Intf), OnuId: onuID, UniId: uniID, TechProfileId: tpID, FlowType: flowDirection}
	logger.Debugw(ctx, "flow-to-be-deleted", log.Fields{"flow": flow})
	if err = f.removeFlowFromDevice(ctx, &removeFlowMessage, flow.Id); err != nil {
		return err
	}

	// Delete the flow-id to gemport list entry from the map now the flow is deleted.
	f.flowIDToGemsLock.Lock()
	delete(f.flowIDToGems, flow.Id)
	f.flowIDToGemsLock.Unlock()

	if err = f.clearResources(ctx, Intf, onuID, uniID, flow.Id, portNum, tpID, true); err != nil {
		logger.Errorw(ctx, "failed-to-clear-resources-for-flow", log.Fields{
			"flow-id":   flow.Id,
			"device-id": f.deviceHandler.device.Id,
			"onu-id":    onuID,
			"intf":      Intf,
			"err":       err,
		})
		return err
	}

	// Decrement reference count for the meter associated with the given <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
	if err := f.resourceMgr.HandleMeterInfoRefCntUpdate(ctx, flowDirection, Intf, uint32(onuID), uint32(uniID), tpID, false); err != nil {
		return err
	}
	return nil
}

//RemoveFlow removes the flow from the device
func (f *OpenOltFlowMgr) RemoveFlow(ctx context.Context, flow *ofp.OfpFlowStats) error {

	logger.Infow(ctx, "removing-flow", log.Fields{"flow": *flow})
	var direction string
	actionInfo := make(map[string]interface{})

	for _, action := range flows.GetActions(flow) {
		if action.Type == flows.OUTPUT {
			if out := action.GetOutput(); out != nil {
				actionInfo[Output] = out.GetPort()
				logger.Debugw(ctx, "action-type-output", log.Fields{"out_port": actionInfo[Output].(uint32)})
			} else {
				logger.Error(ctx, "invalid-output-port-in-action")
				return olterrors.NewErrInvalidValue(log.Fields{"invalid-out-port-action": 0}, nil)
			}
		}
	}

	if flows.HasGroup(flow) {
		direction = Multicast
		return f.clearFlowFromDeviceAndResourceManager(ctx, flow, direction)
	} else if plt.IsUpstream(actionInfo[Output].(uint32)) {
		direction = Upstream
	} else {
		direction = Downstream
	}

	// Serialize flow removes on a per subscriber basis
	err := f.clearFlowFromDeviceAndResourceManager(ctx, flow, direction)

	return err
}

//isIgmpTrapDownstreamFlow return true if the flow is a downsteam IGMP trap-to-host flow; false otherwise
func isIgmpTrapDownstreamFlow(classifierInfo map[string]interface{}) bool {
	if portType := plt.IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_ETHERNET_NNI {
		if ethType, ok := classifierInfo[EthType]; ok {
			if ethType.(uint32) == IPv4EthType {
				if ipProto, ok := classifierInfo[IPProto]; ok {
					if ipProto.(uint32) == IgmpProto {
						return true
					}
				}
			}
		}
	}
	return false
}

// RouteFlowToOnuChannel routes incoming flow to ONU specific channel
func (f *OpenOltFlowMgr) RouteFlowToOnuChannel(ctx context.Context, flow *ofp.OfpFlowStats, addFlow bool, flowMetadata *ofp.FlowMetadata) error {
	if f.deviceHandler.getDeviceDeletionInProgressFlag() {
		// The device itself is going to be reset as part of deletion. So nothing to be done.
		logger.Infow(ctx, "device-deletion-in-progress--not-handling-flows-or-groups", log.Fields{"device-id": f.deviceHandler.device.Id})
		return nil
	}
	// Step1 : Fill flowControlBlock
	// Step2 : Push the flowControlBlock to ONU channel
	// Step3 : Wait on response channel for response
	// Step4 : Return error value
	startTime := time.Now()
	logger.Infow(ctx, "process-flow", log.Fields{"flow": flow, "addFlow": addFlow})
	errChan := make(chan error)
	flowCb := flowControlBlock{
		ctx:          ctx,
		addFlow:      addFlow,
		flow:         flow,
		flowMetadata: flowMetadata,
		errChan:      &errChan,
	}
	inPort, outPort := getPorts(flow)
	var onuID uint32
	if inPort != InvalidPort && outPort != InvalidPort {
		_, _, onuID, _ = plt.ExtractAccessFromFlow(inPort, outPort)
	}
	if f.flowHandlerRoutineActive[onuID] {
		// inPort or outPort is InvalidPort for trap-from-nni flows.
		// In the that case onuID is 0 which is the reserved index for trap-from-nni flows in the f.incomingFlows slice
		// Send the flowCb on the ONU flow channel
		f.incomingFlows[onuID] <- flowCb
		// Wait on the channel for flow handlers return value
		err := <-errChan
		logger.Infow(ctx, "process-flow-received-resp", log.Fields{"err": err, "totalTimeSeconds": time.Since(startTime).Seconds()})
		return err
	}
	logger.Errorw(ctx, "flow handler routine not active for onu", log.Fields{"onuID": onuID, "ponPortIdx": f.ponPortIdx})
	return fmt.Errorf("flow-handler-routine-not-active-for-onu-%v-pon-%d", onuID, f.ponPortIdx)
}

// This routine is unique per ONU ID and blocks on flowControlBlock channel for incoming flows
// Each incoming flow is processed in a synchronous manner, i.e., the flow is processed to completion before picking another
func (f *OpenOltFlowMgr) perOnuFlowHandlerRoutine(handlerRoutineIndex int, subscriberFlowChannel chan flowControlBlock, stopHandler chan bool) {
	var flowCb flowControlBlock
	for {
		select {
		// block on the channel to receive an incoming flow
		// process the flow completely before proceeding to handle the next flow
		case flowCb = <-subscriberFlowChannel:
			if flowCb.addFlow {
				logger.Info(flowCb.ctx, "adding-flow-start")
				startTime := time.Now()
				err := f.AddFlow(flowCb.ctx, flowCb.flow, flowCb.flowMetadata)
				logger.Infow(flowCb.ctx, "adding-flow-complete", log.Fields{"processTimeSecs": time.Since(startTime).Seconds()})
				// Pass the return value over the return channel
				*flowCb.errChan <- err
			} else {
				logger.Info(flowCb.ctx, "removing-flow-start")
				startTime := time.Now()
				err := f.RemoveFlow(flowCb.ctx, flowCb.flow)
				logger.Infow(flowCb.ctx, "removing-flow-complete", log.Fields{"processTimeSecs": time.Since(startTime).Seconds()})
				// Pass the return value over the return channel
				*flowCb.errChan <- err
			}
		case <-stopHandler:
			f.flowHandlerRoutineActive[handlerRoutineIndex] = false
			return
		}
	}
}

// StopAllFlowHandlerRoutines stops all flow handler routines. Call this when device is being rebooted or deleted
func (f *OpenOltFlowMgr) StopAllFlowHandlerRoutines(ctx context.Context, wg *sync.WaitGroup) {
	for i, v := range f.stopFlowHandlerRoutine {
		if f.flowHandlerRoutineActive[i] {
			select {
			case v <- true:
			case <-time.After(time.Second * 5):
				logger.Warnw(ctx, "timeout stopping flow handler routine", log.Fields{"onuID": i, "deviceID": f.deviceHandler.device.Id})
			}
		}
	}
	wg.Done()
	logger.Debugw(ctx, "stopped all flow handler routines", log.Fields{"ponPortIdx": f.ponPortIdx})
}

// AddFlow add flow to device
// nolint: gocyclo
func (f *OpenOltFlowMgr) AddFlow(ctx context.Context, flow *ofp.OfpFlowStats, flowMetadata *ofp.FlowMetadata) error {
	classifierInfo := make(map[string]interface{})
	actionInfo := make(map[string]interface{})
	var UsMeterID uint32
	var DsMeterID uint32

	logger.Infow(ctx, "adding-flow",
		log.Fields{
			"flow":         flow,
			"flowmetadata": flowMetadata})
	formulateClassifierInfoFromFlow(ctx, classifierInfo, flow)

	err := formulateActionInfoFromFlow(ctx, actionInfo, classifierInfo, flow)
	if err != nil {
		// Error logging is already done in the called function
		// So just return in case of error
		return err
	}

	if flows.HasGroup(flow) {
		// handle multicast flow
		return f.handleFlowWithGroup(ctx, actionInfo, classifierInfo, flow)
	}

	/* Controller bound trap flows */
	err = formulateControllerBoundTrapFlowInfo(ctx, actionInfo, classifierInfo, flow)
	if err != nil {
		// error if any, already logged in the called function
		return err
	}

	logger.Debugw(ctx, "flow-ports",
		log.Fields{
			"classifierinfo_inport": classifierInfo[InPort],
			"action_output":         actionInfo[Output]})
	portNo, intfID, onuID, uniID := plt.ExtractAccessFromFlow(classifierInfo[InPort].(uint32), actionInfo[Output].(uint32))

	if ethType, ok := classifierInfo[EthType]; ok {
		if ethType.(uint32) == LldpEthType {
			logger.Info(ctx, "adding-lldp-flow")
			return f.addLLDPFlow(ctx, flow, portNo)
		}
		if ethType.(uint32) == PPPoEDEthType {
			if voltha.Port_ETHERNET_NNI == plt.IntfIDToPortTypeName(classifierInfo[InPort].(uint32)) {
				logger.Debug(ctx, "trap-pppoed-from-nni-flow")
				return f.addTrapFlowOnNNI(ctx, flow, classifierInfo, portNo)
			}
		}
	}
	if ipProto, ok := classifierInfo[IPProto]; ok {
		if ipProto.(uint32) == IPProtoDhcp {
			if udpSrc, ok := classifierInfo[UDPSrc]; ok {
				if udpSrc.(uint32) == uint32(67) || udpSrc.(uint32) == uint32(546) {
					logger.Debug(ctx, "trap-dhcp-from-nni-flow")
					return f.addTrapFlowOnNNI(ctx, flow, classifierInfo, portNo)
				}
			}
		}
	}
	if isIgmpTrapDownstreamFlow(classifierInfo) {
		logger.Debug(ctx, "trap-igmp-from-nni-flow")
		return f.addIgmpTrapFlowOnNNI(ctx, flow, classifierInfo, portNo)
	}

	f.resourceMgr.AddUniPortToOnuInfo(ctx, intfID, onuID, portNo)
	// also update flowmgr cache
	f.onuGemInfoLock.Lock()
	onugem, ok := f.onuGemInfoMap[onuID]
	if ok {
		found := false
		for _, uni := range onugem.UniPorts {
			if uni == portNo {
				found = true
				break
			}
		}
		if !found {
			onugem.UniPorts = append(onugem.UniPorts, portNo)
			f.onuGemInfoMap[onuID] = onugem
			logger.Infow(ctx, "added uni port to onugem cache", log.Fields{"uni": portNo})
		}
	}
	f.onuGemInfoLock.Unlock()

	tpID, err := getTpIDFromFlow(ctx, flow)
	if err != nil {
		return olterrors.NewErrNotFound("tpid-for-flow",
			log.Fields{
				"flow":    flow,
				"intf-id": IntfID,
				"onu-id":  onuID,
				"uni-id":  uniID}, err)
	}
	logger.Debugw(ctx, "tpid-for-this-subcriber",
		log.Fields{
			"tp-id":   tpID,
			"intf-id": intfID,
			"onu-id":  onuID,
			"uni-id":  uniID})
	if plt.IsUpstream(actionInfo[Output].(uint32)) {
		UsMeterID = flows.GetMeterIdFromFlow(flow)
		logger.Debugw(ctx, "upstream-flow-meter-id", log.Fields{"us-meter-id": UsMeterID})
		if err := f.validateMeter(ctx, Upstream, UsMeterID, intfID, onuID, uniID, tpID); err != nil {
			logger.Errorw(ctx, "meter-validation-failed", log.Fields{"err": err})
			return err
		}
	} else {
		DsMeterID = flows.GetMeterIdFromFlow(flow)
		logger.Debugw(ctx, "downstream-flow-meter-id", log.Fields{"ds-meter-id": DsMeterID})
		if err := f.validateMeter(ctx, Downstream, DsMeterID, intfID, onuID, uniID, tpID); err != nil {
			logger.Errorw(ctx, "meter-validation-failed", log.Fields{"err": err})
			return err
		}
	}
	return f.processAddFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, tpID, UsMeterID, DsMeterID, flowMetadata)
}

// handleFlowWithGroup adds multicast flow to the device.
func (f *OpenOltFlowMgr) handleFlowWithGroup(ctx context.Context, actionInfo, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) error {
	classifierInfo[PacketTagType] = getPacketTypeFromClassifiers(classifierInfo)
	logger.Debugw(ctx, "add-multicast-flow", log.Fields{
		"classifier-info": classifierInfo,
		"actionInfo":      actionInfo})

	networkInterfaceID, err := f.getNNIInterfaceIDOfMulticastFlow(ctx, classifierInfo)
	if err != nil {
		return olterrors.NewErrNotFound("multicast-in-port", log.Fields{"classifier": classifierInfo}, err)
	}

	delete(classifierInfo, EthType)

	onuID := NoneOnuID
	uniID := NoneUniID

	if present := f.resourceMgr.IsFlowOnKvStore(ctx, networkInterfaceID, int32(onuID), int32(uniID), flow.Id); present {
		logger.Infow(ctx, "multicast-flow-exists-not-re-adding", log.Fields{"classifier-info": classifierInfo})
		return nil
	}
	classifierProto, err := makeOpenOltClassifierField(classifierInfo)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"classifier": classifierInfo}, err)
	}
	groupID := actionInfo[GroupID].(uint32)
	multicastFlow := openoltpb2.Flow{
		FlowId:        flow.Id,
		FlowType:      Multicast,
		NetworkIntfId: int32(networkInterfaceID),
		GroupId:       groupID,
		Classifier:    classifierProto,
		Priority:      int32(flow.Priority),
		Cookie:        flow.Cookie}

	if err := f.addFlowToDevice(ctx, flow, &multicastFlow); err != nil {
		return olterrors.NewErrFlowOp("add", flow.Id, log.Fields{"flow": multicastFlow}, err)
	}
	logger.Info(ctx, "multicast-flow-added-to-device-successfully")
	//get cached group
	if group, _, err := f.grpMgr.getFlowGroupFromKVStore(ctx, groupID, true); err == nil {
		//calling groupAdd to set group members after multicast flow creation
		if err := f.grpMgr.ModifyGroup(ctx, group); err != nil {
			return olterrors.NewErrGroupOp("modify", groupID, log.Fields{"group": group}, err)
		}
		//cached group can be removed now
		if err := f.resourceMgr.RemoveFlowGroupFromKVStore(ctx, groupID, true); err != nil {
			logger.Warnw(ctx, "failed-to-remove-flow-group", log.Fields{"group-id": groupID, "err": err})
		}
	}

	return nil
}

//getNNIInterfaceIDOfMulticastFlow returns associated NNI interface id of the inPort criterion if exists; returns the first NNI interface of the device otherwise
func (f *OpenOltFlowMgr) getNNIInterfaceIDOfMulticastFlow(ctx context.Context, classifierInfo map[string]interface{}) (uint32, error) {
	if inPort, ok := classifierInfo[InPort]; ok {
		nniInterfaceID, err := plt.IntfIDFromNniPortNum(ctx, inPort.(uint32))
		if err != nil {
			return 0, olterrors.NewErrInvalidValue(log.Fields{"nni-in-port-number": inPort}, err)
		}
		return nniInterfaceID, nil
	}

	// TODO: For now we support only one NNI port in VOLTHA. We shall use only the first NNI port, i.e., interface-id 0.
	return 0, nil
}

//sendTPDownloadMsgToChild send payload
func (f *OpenOltFlowMgr) sendTPDownloadMsgToChild(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, uni string, TpID uint32, tpInst tp_pb.TechProfileInstance) error {

	onuDev, err := f.getOnuDevice(ctx, intfID, onuID)
	if err != nil {
		logger.Errorw(ctx, "couldnt-find-onu-child-device",
			log.Fields{
				"intf-id": intfID,
				"onu-id":  onuID,
				"uni-id":  uniID})
		return err
	}
	logger.Debugw(ctx, "got-child-device-from-olt-device-handler", log.Fields{"onu-id": onuDev.deviceID})

	tpPath := f.getTPpath(ctx, intfID, uni, TpID)
	tpDownloadMsg := &ia.TechProfileDownloadMessage{
		DeviceId:       onuDev.deviceID,
		UniId:          uniID,
		TpInstancePath: tpPath,
		TechTpInstance: &ia.TechProfileDownloadMessage_TpInstance{TpInstance: &tpInst},
	}
	logger.Debugw(ctx, "sending-load-tech-profile-request-to-brcm-onu-adapter", log.Fields{"tpDownloadMsg": *tpDownloadMsg})

	err = f.deviceHandler.sendDownloadTechProfileToChildAdapter(ctx, onuDev.adapterEndpoint, tpDownloadMsg)
	if err != nil {
		return olterrors.NewErrCommunication("send-techprofile-download-request",
			log.Fields{
				"from-adapter":  f.deviceHandler.openOLT.config.AdapterEndpoint,
				"to-adapter":    onuDev.deviceType,
				"onu-id":        onuDev.deviceID,
				"proxyDeviceID": onuDev.proxyDeviceID}, err)
	}
	logger.Infow(ctx, "success-sending-load-tech-profile-request-to-brcm-onu-adapter", log.Fields{"tpDownloadMsg": *tpDownloadMsg})
	return nil
}

//AddOnuInfoToFlowMgrCacheAndKvStore function adds onu info to cache and kvstore
func (f *OpenOltFlowMgr) AddOnuInfoToFlowMgrCacheAndKvStore(ctx context.Context, intfID uint32, onuID uint32, serialNum string) error {

	f.onuGemInfoLock.RLock()
	_, ok := f.onuGemInfoMap[onuID]
	f.onuGemInfoLock.RUnlock()
	// If the ONU already exists in onuGemInfo list, nothing to do
	if ok {
		logger.Debugw(ctx, "onu-id-already-exists-in-cache",
			log.Fields{"onuID": onuID,
				"serialNum": serialNum})
		return nil
	}

	onuGemInfo := rsrcMgr.OnuGemInfo{OnuID: onuID, SerialNumber: serialNum, IntfID: intfID}
	f.onuGemInfoLock.Lock()
	f.onuGemInfoMap[onuID] = &onuGemInfo
	f.onuGemInfoLock.Unlock()
	if err := f.resourceMgr.AddOnuGemInfo(ctx, intfID, onuID, onuGemInfo); err != nil {
		return err
	}
	logger.Infow(ctx, "added-onuinfo",
		log.Fields{
			"intf-id":    intfID,
			"onu-id":     onuID,
			"serial-num": serialNum,
			"onu":        onuGemInfo,
			"device-id":  f.deviceHandler.device.Id})
	return nil
}

//RemoveOnuInfoFromFlowMgrCacheAndKvStore function adds onu info to cache and kvstore
func (f *OpenOltFlowMgr) RemoveOnuInfoFromFlowMgrCacheAndKvStore(ctx context.Context, intfID uint32, onuID uint32) error {

	f.onuGemInfoLock.Lock()
	delete(f.onuGemInfoMap, onuID)
	f.onuGemInfoLock.Unlock()

	if err := f.resourceMgr.DelOnuGemInfo(ctx, intfID, onuID); err != nil {
		return err
	}
	logger.Infow(ctx, "deleted-onuinfo",
		log.Fields{
			"intf-id":   intfID,
			"onu-id":    onuID,
			"device-id": f.deviceHandler.device.Id})
	return nil
}

//addGemPortToOnuInfoMap function adds GEMport to ONU map
func (f *OpenOltFlowMgr) addGemPortToOnuInfoMap(ctx context.Context, intfID uint32, onuID uint32, gemPort uint32) {

	logger.Infow(ctx, "adding-gem-to-onu-info-map",
		log.Fields{
			"gem-port-id": gemPort,
			"intf-id":     intfID,
			"onu-id":      onuID,
			"device-id":   f.deviceHandler.device.Id})
	f.onuGemInfoLock.RLock()
	onugem, ok := f.onuGemInfoMap[onuID]
	f.onuGemInfoLock.RUnlock()
	if !ok {
		logger.Warnw(ctx, "onu gem info is missing", log.Fields{
			"gem-port-id": gemPort,
			"intf-id":     intfID,
			"onu-id":      onuID,
			"device-id":   f.deviceHandler.device.Id})
		return
	}

	if onugem.OnuID == onuID {
		// check if gem already exists , else update the cache and kvstore
		for _, gem := range onugem.GemPorts {
			if gem == gemPort {
				logger.Debugw(ctx, "gem-already-in-cache-no-need-to-update-cache-and-kv-store",
					log.Fields{
						"gem":       gemPort,
						"device-id": f.deviceHandler.device.Id})
				return
			}
		}
		onugem.GemPorts = append(onugem.GemPorts, gemPort)
		f.onuGemInfoLock.Lock()
		f.onuGemInfoMap[onuID] = onugem
		f.onuGemInfoLock.Unlock()
		logger.Debugw(ctx, "updated onu gem info from cache", log.Fields{"onugem": onugem})
	} else {
		logger.Warnw(ctx, "mismatched onu id", log.Fields{
			"gem-port-id": gemPort,
			"intf-id":     intfID,
			"onu-id":      onuID,
			"device-id":   f.deviceHandler.device.Id})
		return
	}
	err := f.resourceMgr.AddGemToOnuGemInfo(ctx, intfID, onuID, gemPort)
	if err != nil {
		logger.Errorw(ctx, "failed-to-add-gem-to-onu",
			log.Fields{
				"intf-id":   intfID,
				"onu-id":    onuID,
				"gemPort":   gemPort,
				"device-id": f.deviceHandler.device.Id})
		return
	}
	logger.Infow(ctx, "gem-added-to-onu-info-map",
		log.Fields{
			"gem-port-id": gemPort,
			"intf-id":     intfID,
			"onu-id":      onuID,
			"device-id":   f.deviceHandler.device.Id})
}

//GetLogicalPortFromPacketIn function computes logical port UNI/NNI port from packet-in indication and returns the same
func (f *OpenOltFlowMgr) GetLogicalPortFromPacketIn(ctx context.Context, packetIn *openoltpb2.PacketIndication) (uint32, error) {
	var logicalPortNum uint32

	if packetIn.IntfType == "pon" {
		// packet indication does not have serial number , so sending as nil
		// get onu and uni ids associated with the given pon and gem ports
		onuID, uniID := packetIn.OnuId, packetIn.UniId
		logger.Debugf(ctx, "retrieved ONU and UNI IDs [%d, %d] by interface:%d, gem:%d", packetIn.OnuId, packetIn.UniId, packetIn.GemportId)

		if packetIn.PortNo != 0 {
			logicalPortNum = packetIn.PortNo
		} else {
			logicalPortNum = plt.MkUniPortNum(ctx, packetIn.IntfId, onuID, uniID)
		}
		// Store the gem port through which the packet_in came. Use the same gem port for packet_out
		f.UpdateGemPortForPktIn(ctx, packetIn.IntfId, onuID, logicalPortNum, packetIn.GemportId, packetIn.Pkt)
	} else if packetIn.IntfType == "nni" {
		logicalPortNum = plt.IntfIDToPortNo(packetIn.IntfId, voltha.Port_ETHERNET_NNI)
	}

	if logger.V(log.DebugLevel) {
		logger.Debugw(ctx, "retrieved-logicalport-from-packet-in",
			log.Fields{
				"logical-port-num": logicalPortNum,
				"intf-type":        packetIn.IntfType,
				"packet":           hex.EncodeToString(packetIn.Pkt),
			})
	}
	return logicalPortNum, nil
}

//GetPacketOutGemPortID returns gemPortId
func (f *OpenOltFlowMgr) GetPacketOutGemPortID(ctx context.Context, intfID uint32, onuID uint32, portNum uint32, packet []byte) (uint32, error) {
	var gemPortID uint32

	ctag, priority, err := getCTagFromPacket(ctx, packet)
	if err != nil {
		return 0, err
	}

	pktInkey := rsrcMgr.PacketInInfoKey{IntfID: intfID, OnuID: onuID, LogicalPort: portNum, VlanID: ctag, Priority: priority}
	var ok bool
	f.packetInGemPortLock.RLock()
	gemPortID, ok = f.packetInGemPort[pktInkey]
	f.packetInGemPortLock.RUnlock()
	if ok {
		logger.Debugw(ctx, "found-gemport-for-pktin-key",
			log.Fields{
				"pktinkey": pktInkey,
				"gem":      gemPortID})

		return gemPortID, nil
	}
	//If gem is not found in cache try to get it from kv store, if found in kv store, update the cache and return.
	gemPortID, err = f.resourceMgr.GetGemPortFromOnuPktIn(ctx, pktInkey)
	if err == nil {
		if gemPortID != 0 {
			f.packetInGemPortLock.Lock()
			f.packetInGemPort[pktInkey] = gemPortID
			f.packetInGemPortLock.Unlock()
			logger.Infow(ctx, "found-gem-port-from-kv-store-and-updating-cache-with-gemport",
				log.Fields{
					"pktinkey": pktInkey,
					"gem":      gemPortID})
			return gemPortID, nil
		}
	}
	return uint32(0), olterrors.NewErrNotFound("gem-port",
		log.Fields{
			"pktinkey": pktInkey,
			"gem":      gemPortID}, err)

}

func (f *OpenOltFlowMgr) addTrapFlowOnNNI(ctx context.Context, logicalFlow *ofp.OfpFlowStats, classifier map[string]interface{}, portNo uint32) error {
	logger.Debug(ctx, "adding-trap-of-nni-flow")
	action := make(map[string]interface{})
	classifier[PacketTagType] = DoubleTag
	action[TrapToHost] = true
	/* We manage flowId resource pool on per PON port basis.
	   Since this situation is tricky, as a hack, we pass the NNI port
	   index (network_intf_id) as PON port Index for the flowId resource
	   pool. Also, there is no ONU Id available for trapping packets
	   on NNI port, use onu_id as -1 (invalid)
	   ****************** CAVEAT *******************
	   This logic works if the NNI Port Id falls within the same valid
	   range of PON Port Ids. If this doesn't work for some OLT Vendor
	   we need to have a re-look at this.
	   *********************************************
	*/
	onuID := -1
	uniID := -1
	gemPortID := -1
	allocID := -1
	networkInterfaceID, err := getNniIntfID(ctx, classifier, action)
	if err != nil {
		return olterrors.NewErrNotFound("nni-intreface-id",
			log.Fields{
				"classifier": classifier,
				"action":     action},
			err)
	}

	if present := f.resourceMgr.IsFlowOnKvStore(ctx, networkInterfaceID, int32(onuID), int32(uniID), logicalFlow.Id); present {
		logger.Info(ctx, "flow-exists-not-re-adding")
		return nil
	}

	logger.Debugw(ctx, "creating-trap-of-nni-flow",
		log.Fields{
			"classifier": classifier,
			"action":     action,
			"flowId":     logicalFlow.Id,
			"intf-id":    networkInterfaceID})

	classifierProto, err := makeOpenOltClassifierField(classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"classifier": classifier}, err)
	}
	logger.Debugw(ctx, "created-classifier-proto", log.Fields{"classifier": *classifierProto})
	actionProto, err := makeOpenOltActionField(action, classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"action": action}, err)
	}
	logger.Debugw(ctx, "created-action-proto", log.Fields{"action": *actionProto})
	downstreamflow := openoltpb2.Flow{AccessIntfId: int32(-1), // AccessIntfId not required
		OnuId:         int32(onuID), // OnuId not required
		UniId:         int32(uniID), // UniId not used
		FlowId:        logicalFlow.Id,
		FlowType:      Downstream,
		AllocId:       int32(allocID), // AllocId not used
		NetworkIntfId: int32(networkInterfaceID),
		GemportId:     int32(gemPortID), // GemportId not used
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo}
	if err := f.addFlowToDevice(ctx, logicalFlow, &downstreamflow); err != nil {
		return olterrors.NewErrFlowOp("add", logicalFlow.Id, log.Fields{"flow": downstreamflow}, err)
	}
	logger.Info(ctx, "trap-on-nni-flow-added–to-device-successfully")
	return nil
}

//getPacketTypeFromClassifiers finds and returns packet type of a flow by checking flow classifiers
func getPacketTypeFromClassifiers(classifierInfo map[string]interface{}) string {
	var packetType string
	ovid, ivid := false, false
	if vlanID, ok := classifierInfo[VlanVid].(uint32); ok {
		vid := vlanID & VlanvIDMask
		if vid != ReservedVlan {
			ovid = true
		}
	}
	if metadata, ok := classifierInfo[Metadata].(uint64); ok {
		vid := uint32(metadata)
		if vid != ReservedVlan {
			ivid = true
		}
	}
	if ovid && ivid {
		packetType = DoubleTag
	} else if !ovid && !ivid {
		packetType = Untagged
	} else {
		packetType = SingleTag
	}
	return packetType
}

//addIgmpTrapFlowOnNNI adds a trap-to-host flow on NNI
func (f *OpenOltFlowMgr) addIgmpTrapFlowOnNNI(ctx context.Context, logicalFlow *ofp.OfpFlowStats, classifier map[string]interface{}, portNo uint32) error {
	logger.Infow(ctx, "adding-igmp-trap-of-nni-flow", log.Fields{"classifier-info": classifier})
	action := make(map[string]interface{})
	classifier[PacketTagType] = getPacketTypeFromClassifiers(classifier)
	action[TrapToHost] = true
	/* We manage flowId resource pool on per PON port basis.
	   Since this situation is tricky, as a hack, we pass the NNI port
	   index (network_intf_id) as PON port Index for the flowId resource
	   pool. Also, there is no ONU Id available for trapping packets
	   on NNI port, use onu_id as -1 (invalid)
	   ****************** CAVEAT *******************
	   This logic works if the NNI Port Id falls within the same valid
	   range of PON Port Ids. If this doesn't work for some OLT Vendor
	   we need to have a re-look at this.
	   *********************************************
	*/
	onuID := -1
	uniID := -1
	gemPortID := -1
	allocID := -1
	networkInterfaceID, err := getNniIntfID(ctx, classifier, action)
	if err != nil {
		return olterrors.NewErrNotFound("nni-interface-id", log.Fields{
			"classifier": classifier,
			"action":     action},
			err)
	}
	if present := f.resourceMgr.IsFlowOnKvStore(ctx, networkInterfaceID, int32(onuID), int32(uniID), logicalFlow.Id); present {
		logger.Info(ctx, "igmp-flow-exists-not-re-adding")
		return nil
	}

	classifierProto, err := makeOpenOltClassifierField(classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"classifier": classifier}, err)
	}
	logger.Debugw(ctx, "created-classifier-proto-for-the-igmp-flow", log.Fields{"classifier": *classifierProto})
	actionProto, err := makeOpenOltActionField(action, classifier)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"action": action}, err)
	}
	logger.Debugw(ctx, "created-action-proto-for-the-igmp-flow", log.Fields{"action": *actionProto})
	downstreamflow := openoltpb2.Flow{AccessIntfId: int32(-1), // AccessIntfId not required
		OnuId:         int32(onuID), // OnuId not required
		UniId:         int32(uniID), // UniId not used
		FlowId:        logicalFlow.Id,
		FlowType:      Downstream,
		AllocId:       int32(allocID), // AllocId not used
		NetworkIntfId: int32(networkInterfaceID),
		GemportId:     int32(gemPortID), // GemportId not used
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo}
	if err := f.addFlowToDevice(ctx, logicalFlow, &downstreamflow); err != nil {
		return olterrors.NewErrFlowOp("add", logicalFlow.Id, log.Fields{"flow": downstreamflow}, err)
	}
	logger.Info(ctx, "igmp-trap-on-nni-flow-added-to-device-successfully")

	return nil
}

func verifyMeterIDAndGetDirection(MeterID uint32, Dir tp_pb.Direction) (string, error) {
	if MeterID == 0 { // This should never happen
		return "", olterrors.NewErrInvalidValue(log.Fields{"meter-id": MeterID}, nil).Log()
	}
	if Dir == tp_pb.Direction_UPSTREAM {
		return "upstream", nil
	} else if Dir == tp_pb.Direction_DOWNSTREAM {
		return "downstream", nil
	}
	return "", nil
}

// nolint: gocyclo
func (f *OpenOltFlowMgr) checkAndAddFlow(ctx context.Context, args map[string]uint32, classifierInfo map[string]interface{},
	actionInfo map[string]interface{}, flow *ofp.OfpFlowStats, TpInst interface{}, gemPorts []uint32,
	tpID uint32, uni string) error {
	var gemPortID uint32
	intfID := args[IntfID]
	onuID := args[OnuID]
	uniID := args[UniID]
	portNo := args[PortNo]
	allocID := args[AllocID]
	pbitToGem := make(map[uint32]uint32)
	gemToAes := make(map[uint32]bool)

	var attributes []*tp_pb.GemPortAttributes
	var direction = tp_pb.Direction_UPSTREAM
	switch TpInst := TpInst.(type) {
	case *tp_pb.TechProfileInstance:
		if plt.IsUpstream(actionInfo[Output].(uint32)) {
			attributes = TpInst.UpstreamGemPortAttributeList
		} else {
			attributes = TpInst.DownstreamGemPortAttributeList
			direction = tp_pb.Direction_DOWNSTREAM
		}
	default:
		logger.Errorw(ctx, "unsupported-tech", log.Fields{"tpInst": TpInst})
		return olterrors.NewErrInvalidValue(log.Fields{"tpInst": TpInst}, nil)
	}

	if len(gemPorts) == 1 {
		// If there is only single gemport use that and do not populate pbitToGem map
		gemPortID = gemPorts[0]
		gemToAes[gemPortID], _ = strconv.ParseBool(attributes[0].AesEncryption)
	} else if pcp, ok := classifierInfo[VlanPcp]; !ok {
		for idx, gemID := range gemPorts {
			pBitMap := attributes[idx].PbitMap
			// Trim the bitMapPrefix form the binary string and then iterate each character in the binary string.
			// If the character is set to pbit1, extract the pcp value from the position of this character in the string.
			// Update the pbitToGem map with key being the pcp bit and the value being the gemPortID that consumes
			// this pcp bit traffca.
			for pos, pbitSet := range strings.TrimPrefix(pBitMap, bitMapPrefix) {
				if pbitSet == pbit1 {
					pcp := uint32(len(strings.TrimPrefix(pBitMap, bitMapPrefix))) - 1 - uint32(pos)
					pbitToGem[pcp] = gemID
					gemToAes[gemID], _ = strconv.ParseBool(attributes[idx].AesEncryption)
				}
			}
		}
	} else { // Extract the exact gemport which maps to the PCP classifier in the flow
		if gem := f.techprofile.GetGemportForPbit(ctx, TpInst, direction, pcp.(uint32)); gem != nil {
			gemPortID = gem.(*tp_pb.GemPortAttributes).GemportId
			gemToAes[gemPortID], _ = strconv.ParseBool(gem.(*tp_pb.GemPortAttributes).AesEncryption)
		}
	}

	flowContext := &flowContext{intfID, onuID, uniID, portNo, classifierInfo, actionInfo,
		flow, allocID, gemPortID, tpID, pbitToGem, gemToAes}

	if ipProto, ok := classifierInfo[IPProto]; ok {
		if ipProto.(uint32) == IPProtoDhcp {
			logger.Infow(ctx, "adding-dhcp-flow", log.Fields{
				"tp-id":    tpID,
				"alloc-id": allocID,
				"intf-id":  intfID,
				"onu-id":   onuID,
				"uni-id":   uniID,
			})
			//Adding DHCP upstream flow
			if err := f.addDHCPTrapFlow(ctx, flowContext); err != nil {
				logger.Warn(ctx, err)
				logger.Errorw(ctx, "reverting-scheduler-and-queue-for-onu", log.Fields{"intf-id": intfID, "onu-id": onuID, "uni-id": uniID, "flow-id": flow.Id, "tp-id": tpID})
				_ = f.clearResources(ctx, intfID, int32(onuID), int32(uniID), flow.Id, portNo, tpID, false)
				return err
			}

		} else if ipProto.(uint32) == IgmpProto {
			logger.Infow(ctx, "adding-us-igmp-flow",
				log.Fields{
					"intf-id":          intfID,
					"onu-id":           onuID,
					"uni-id":           uniID,
					"classifier-info:": classifierInfo})
			if err := f.addIGMPTrapFlow(ctx, flowContext); err != nil {
				logger.Warn(ctx, err)
				logger.Errorw(ctx, "reverting-scheduler-and-queue-for-onu", log.Fields{"intf-id": intfID, "onu-id": onuID, "uni-id": uniID, "flow-id": flow.Id, "tp-id": tpID})
				_ = f.clearResources(ctx, intfID, int32(onuID), int32(uniID), flow.Id, portNo, tpID, false)
				return err
			}
		} else {
			logger.Errorw(ctx, "invalid-classifier-to-handle", log.Fields{"classifier": classifierInfo, "action": actionInfo})
			return olterrors.NewErrInvalidValue(log.Fields{"classifier": classifierInfo, "action": actionInfo}, nil)
		}
	} else if ethType, ok := classifierInfo[EthType]; ok {
		if ethType.(uint32) == EapEthType {
			logger.Infow(ctx, "adding-eapol-flow", log.Fields{
				"intf-id": intfID,
				"onu-id":  onuID,
				"uni-id":  uniID,
				"ethType": ethType,
			})
			var vlanID uint32
			if val, ok := classifierInfo[VlanVid]; ok {
				vlanID = (val.(uint32)) & VlanvIDMask
			} else {
				vlanID = DefaultMgmtVlan
			}
			if err := f.addEthTypeBasedFlow(ctx, flowContext, vlanID, ethType.(uint32)); err != nil {
				logger.Warn(ctx, err)
				logger.Errorw(ctx, "reverting-scheduler-and-queue-for-onu", log.Fields{"intf-id": intfID, "onu-id": onuID, "uni-id": uniID, "flow-id": flow.Id, "tp-id": tpID})
				_ = f.clearResources(ctx, intfID, int32(onuID), int32(uniID), flow.Id, portNo, tpID, false)
				return err
			}
		} else if ethType.(uint32) == PPPoEDEthType {
			logger.Infow(ctx, "adding-pppoed-flow", log.Fields{
				"tp-id":    tpID,
				"alloc-id": allocID,
				"intf-id":  intfID,
				"onu-id":   onuID,
				"uni-id":   uniID,
			})
			//Adding PPPOED upstream flow
			if err := f.addUpstreamTrapFlow(ctx, flowContext); err != nil {
				logger.Warn(ctx, err)
				logger.Errorw(ctx, "reverting-scheduler-and-queue-for-onu", log.Fields{"intf-id": intfID, "onu-id": onuID, "uni-id": uniID, "flow-id": flow.Id, "tp-id": tpID})
				_ = f.clearResources(ctx, intfID, int32(onuID), int32(uniID), flow.Id, portNo, tpID, false)
				return err
			}
		}
	} else if direction == tp_pb.Direction_UPSTREAM {
		logger.Infow(ctx, "adding-upstream-data-rule", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"uni-id":  uniID,
		})
		//Adding HSIA upstream flow
		if err := f.addUpstreamDataPathFlow(ctx, flowContext); err != nil {
			logger.Warn(ctx, err)
			logger.Errorw(ctx, "reverting-scheduler-and-queue-for-onu", log.Fields{"intf-id": intfID, "onu-id": onuID, "uni-id": uniID, "flow-id": flow.Id, "tp-id": tpID})
			_ = f.clearResources(ctx, intfID, int32(onuID), int32(uniID), flow.Id, portNo, tpID, false)
			return err
		}
	} else if direction == tp_pb.Direction_DOWNSTREAM {
		logger.Infow(ctx, "adding-downstream-data-rule", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"uni-id":  uniID,
		})
		//Adding HSIA downstream flow
		if err := f.addDownstreamDataPathFlow(ctx, flowContext); err != nil {
			logger.Warn(ctx, err)
			logger.Errorw(ctx, "reverting-scheduler-and-queue-for-onu", log.Fields{"intf-id": intfID, "onu-id": onuID, "uni-id": uniID, "flow-id": flow.Id, "tp-id": tpID})
			_ = f.clearResources(ctx, intfID, int32(onuID), int32(uniID), flow.Id, portNo, tpID, false)
			return err
		}
	} else {
		return olterrors.NewErrInvalidValue(log.Fields{
			"intf-id":    intfID,
			"onu-id":     onuID,
			"uni-id":     uniID,
			"classifier": classifierInfo,
			"action":     actionInfo,
			"flow":       flow},
			nil).Log()
	}
	// Send Techprofile download event to child device in go routine as it takes time
	go func() {
		if err := f.sendTPDownloadMsgToChild(ctx, intfID, onuID, uniID, uni, tpID, *(TpInst.(*tp_pb.TechProfileInstance))); err != nil {
			logger.Warn(ctx, err)
		}
	}()
	return nil
}

func (f *OpenOltFlowMgr) isGemPortUsedByAnotherFlow(gemPortID uint32, flowID uint64) bool {
	f.gemToFlowIDsKey.RLock()
	flowIDList := f.gemToFlowIDs[gemPortID]
	f.gemToFlowIDsKey.RUnlock()
	if len(flowIDList) > 0 {
		for _, id := range flowIDList {
			if flowID != id {
				return true
			}
		}
	}
	return false
}

func (f *OpenOltFlowMgr) isAllocUsedByAnotherUNI(ctx context.Context, sq schedQueue) bool {
	tpInst := sq.tpInst.(*tp_pb.TechProfileInstance)
	if tpInst.InstanceControl.Onu == "single-instance" && sq.direction == tp_pb.Direction_UPSTREAM {
		tpInstances := f.techprofile.FindAllTpInstances(ctx, f.deviceHandler.device.Id, sq.tpID, sq.intfID, sq.onuID).([]tp_pb.TechProfileInstance)
		logger.Debugw(ctx, "got-single-instance-tp-instances", log.Fields{"tp-instances": tpInstances})
		for i := 0; i < len(tpInstances); i++ {
			tpI := tpInstances[i]
			if tpI.SubscriberIdentifier != tpInst.SubscriberIdentifier &&
				tpI.UsScheduler.AllocId == tpInst.UsScheduler.AllocId {
				logger.Debugw(ctx, "alloc-is-in-use",
					log.Fields{
						"device-id": f.deviceHandler.device.Id,
						"intfID":    sq.intfID,
						"onuID":     sq.onuID,
						"uniID":     sq.uniID,
						"allocID":   tpI.UsScheduler.AllocId,
					})
				return true
			}
		}
	}
	return false
}

func formulateClassifierInfoFromFlow(ctx context.Context, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) {
	for _, field := range flows.GetOfbFields(flow) {
		if field.Type == flows.ETH_TYPE {
			classifierInfo[EthType] = field.GetEthType()
			logger.Debug(ctx, "field-type-eth-type", log.Fields{"classifierInfo[ETH_TYPE]": classifierInfo[EthType].(uint32)})
		} else if field.Type == flows.ETH_DST {
			classifierInfo[EthDst] = field.GetEthDst()
			logger.Debug(ctx, "field-type-eth-type", log.Fields{"classifierInfo[ETH_DST]": classifierInfo[EthDst].([]uint8)})
		} else if field.Type == flows.IP_PROTO {
			classifierInfo[IPProto] = field.GetIpProto()
			logger.Debug(ctx, "field-type-ip-proto", log.Fields{"classifierInfo[IP_PROTO]": classifierInfo[IPProto].(uint32)})
		} else if field.Type == flows.IN_PORT {
			classifierInfo[InPort] = field.GetPort()
			logger.Debug(ctx, "field-type-in-port", log.Fields{"classifierInfo[IN_PORT]": classifierInfo[InPort].(uint32)})
		} else if field.Type == flows.VLAN_VID {
			// The ReservedVlan is used to signify transparent vlan. Do not do any classification when we see ReservedVlan
			if field.GetVlanVid() != ReservedVlan {
				classifierInfo[VlanVid] = field.GetVlanVid() & 0xfff
				logger.Debug(ctx, "field-type-vlan-vid", log.Fields{"classifierInfo[VLAN_VID]": classifierInfo[VlanVid].(uint32)})
			}
		} else if field.Type == flows.VLAN_PCP {
			classifierInfo[VlanPcp] = field.GetVlanPcp()
			logger.Debug(ctx, "field-type-vlan-pcp", log.Fields{"classifierInfo[VLAN_PCP]": classifierInfo[VlanPcp].(uint32)})
		} else if field.Type == flows.UDP_DST {
			classifierInfo[UDPDst] = field.GetUdpDst()
			logger.Debug(ctx, "field-type-udp-dst", log.Fields{"classifierInfo[UDP_DST]": classifierInfo[UDPDst].(uint32)})
		} else if field.Type == flows.UDP_SRC {
			classifierInfo[UDPSrc] = field.GetUdpSrc()
			logger.Debug(ctx, "field-type-udp-src", log.Fields{"classifierInfo[UDP_SRC]": classifierInfo[UDPSrc].(uint32)})
		} else if field.Type == flows.IPV4_DST {
			classifierInfo[Ipv4Dst] = field.GetIpv4Dst()
			logger.Debug(ctx, "field-type-ipv4-dst", log.Fields{"classifierInfo[IPV4_DST]": classifierInfo[Ipv4Dst].(uint32)})
		} else if field.Type == flows.IPV4_SRC {
			classifierInfo[Ipv4Src] = field.GetIpv4Src()
			logger.Debug(ctx, "field-type-ipv4-src", log.Fields{"classifierInfo[IPV4_SRC]": classifierInfo[Ipv4Src].(uint32)})
		} else if field.Type == flows.METADATA {
			classifierInfo[Metadata] = field.GetTableMetadata()
			logger.Debug(ctx, "field-type-metadata", log.Fields{"classifierInfo[Metadata]": classifierInfo[Metadata].(uint64)})
		} else if field.Type == flows.TUNNEL_ID {
			classifierInfo[TunnelID] = field.GetTunnelId()
			logger.Debug(ctx, "field-type-tunnelId", log.Fields{"classifierInfo[TUNNEL_ID]": classifierInfo[TunnelID].(uint64)})
		} else {
			logger.Errorw(ctx, "un-supported-field-type", log.Fields{"type": field.Type})
			return
		}
	}
}

func formulateActionInfoFromFlow(ctx context.Context, actionInfo, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) error {
	for _, action := range flows.GetActions(flow) {
		if action.Type == flows.OUTPUT {
			if out := action.GetOutput(); out != nil {
				actionInfo[Output] = out.GetPort()
				logger.Debugw(ctx, "action-type-output", log.Fields{"out-port": actionInfo[Output].(uint32)})
			} else {
				return olterrors.NewErrInvalidValue(log.Fields{"output-port": nil}, nil)
			}
		} else if action.Type == flows.POP_VLAN {
			actionInfo[PopVlan] = true
			logger.Debugw(ctx, "action-type-pop-vlan", log.Fields{"in_port": classifierInfo[InPort].(uint32)})
		} else if action.Type == flows.PUSH_VLAN {
			if out := action.GetPush(); out != nil {
				if tpid := out.GetEthertype(); tpid != 0x8100 {
					logger.Errorw(ctx, "invalid ethertype in push action", log.Fields{"ethertype": actionInfo[PushVlan].(int32)})
				} else {
					actionInfo[PushVlan] = true
					actionInfo[TPID] = tpid
					logger.Debugw(ctx, "action-type-push-vlan",
						log.Fields{
							"push-tpid": actionInfo[TPID].(uint32),
							"in-port":   classifierInfo[InPort].(uint32)})
				}
			}
		} else if action.Type == flows.SET_FIELD {
			if out := action.GetSetField(); out != nil {
				if field := out.GetField(); field != nil {
					if ofClass := field.GetOxmClass(); ofClass != ofp.OfpOxmClass_OFPXMC_OPENFLOW_BASIC {
						return olterrors.NewErrInvalidValue(log.Fields{"openflow-class": ofClass}, nil)
					}
					/*logger.Debugw(ctx, "action-type-set-field",log.Fields{"field": field, "in_port": classifierInfo[IN_PORT].(uint32)})*/
					formulateSetFieldActionInfoFromFlow(ctx, field, actionInfo)
				}
			}
		} else if action.Type == flows.GROUP {
			formulateGroupActionInfoFromFlow(ctx, action, actionInfo)
		} else {
			return olterrors.NewErrInvalidValue(log.Fields{"action-type": action.Type}, nil)
		}
	}
	return nil
}

func formulateSetFieldActionInfoFromFlow(ctx context.Context, field *ofp.OfpOxmField, actionInfo map[string]interface{}) {
	if ofbField := field.GetOfbField(); ofbField != nil {
		fieldtype := ofbField.GetType()
		if fieldtype == ofp.OxmOfbFieldTypes_OFPXMT_OFB_VLAN_VID {
			if vlan := ofbField.GetVlanVid(); vlan != 0 {
				actionInfo[VlanVid] = vlan & 0xfff
				logger.Debugw(ctx, "action-set-vlan-vid", log.Fields{"actionInfo[VLAN_VID]": actionInfo[VlanVid].(uint32)})
			} else {
				logger.Error(ctx, "no-invalid-vlan-id-in-set-vlan-vid-action")
			}
		} else if fieldtype == ofp.OxmOfbFieldTypes_OFPXMT_OFB_VLAN_PCP {
			pcp := ofbField.GetVlanPcp()
			actionInfo[VlanPcp] = pcp
			logger.Debugw(ctx, "action-set-vlan-pcp", log.Fields{"actionInfo[VLAN_PCP]": actionInfo[VlanPcp].(uint32)})
		} else {
			logger.Errorw(ctx, "unsupported-action-set-field-type", log.Fields{"type": fieldtype})
		}
	}
}

func formulateGroupActionInfoFromFlow(ctx context.Context, action *ofp.OfpAction, actionInfo map[string]interface{}) {
	if action.GetGroup() == nil {
		logger.Warn(ctx, "no-group-entry-found-in-the-group-action")
	} else {
		actionInfo[GroupID] = action.GetGroup().GroupId
		logger.Debugw(ctx, "action-group-id", log.Fields{"actionInfo[GroupID]": actionInfo[GroupID].(uint32)})
	}
}

func formulateControllerBoundTrapFlowInfo(ctx context.Context, actionInfo, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) error {
	if isControllerFlow := plt.IsControllerBoundFlow(actionInfo[Output].(uint32)); isControllerFlow {
		logger.Debug(ctx, "controller-bound-trap-flows--getting-inport-from-tunnelid")
		/* Get UNI port/ IN Port from tunnel ID field for upstream controller bound flows  */
		if portType := plt.IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := flows.GetChildPortFromTunnelId(flow); uniPort != 0 {
				classifierInfo[InPort] = uniPort
				logger.Debugw(ctx, "upstream-pon-to-controller-flow--inport-in-tunnelid",
					log.Fields{
						"newinport": classifierInfo[InPort].(uint32),
						"outport":   actionInfo[Output].(uint32)})
			} else {
				return olterrors.NewErrNotFound("child-in-port",
					log.Fields{
						"reason": "upstream-pon-to-controller-flow--no-inport-in-tunnelid",
						"flow":   flow}, nil)
			}
		}
	} else {
		logger.Debug(ctx, "non-controller-flows--getting-uniport-from-tunnelid")
		// Downstream flow from NNI to PON port , Use tunnel ID as new OUT port / UNI port
		if portType := plt.IntfIDToPortTypeName(actionInfo[Output].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := flows.GetChildPortFromTunnelId(flow); uniPort != 0 {
				actionInfo[Output] = uniPort
				logger.Debugw(ctx, "downstream-nni-to-pon-port-flow, outport-in-tunnelid",
					log.Fields{
						"newoutport": actionInfo[Output].(uint32),
						"outport":    actionInfo[Output].(uint32)})
			} else {
				return olterrors.NewErrNotFound("out-port",
					log.Fields{
						"reason": "downstream-nni-to-pon-port-flow--no-outport-in-tunnelid",
						"flow":   flow}, nil)
			}
			// Upstream flow from PON to NNI port , Use tunnel ID as new IN port / UNI port
		} else if portType := plt.IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := flows.GetChildPortFromTunnelId(flow); uniPort != 0 {
				classifierInfo[InPort] = uniPort
				logger.Debugw(ctx, "upstream-pon-to-nni-port-flow, inport-in-tunnelid",
					log.Fields{
						"newinport": actionInfo[Output].(uint32),
						"outport":   actionInfo[Output].(uint32)})
			} else {
				return olterrors.NewErrNotFound("nni-port",
					log.Fields{
						"reason":   "upstream-pon-to-nni-port-flow--no-inport-in-tunnelid",
						"in-port":  classifierInfo[InPort].(uint32),
						"out-port": actionInfo[Output].(uint32),
						"flow":     flow}, nil)
			}
		}
	}
	return nil
}

func getTpIDFromFlow(ctx context.Context, flow *ofp.OfpFlowStats) (uint32, error) {
	/*     Metadata 8 bytes:
		   Most Significant 2 Bytes = Inner VLAN
		   Next 2 Bytes = Tech Profile ID(TPID)
		   Least Significant 4 Bytes = Port ID
	       Flow Metadata carries Tech-Profile (TP) ID and is mandatory in all
	       subscriber related flows.
	*/
	metadata := flows.GetMetadataFromWriteMetadataAction(ctx, flow)
	if metadata == 0 {
		return 0, olterrors.NewErrNotFound("metadata", log.Fields{"flow": flow}, nil)
	}
	TpID := flows.GetTechProfileIDFromWriteMetaData(ctx, metadata)
	return uint32(TpID), nil
}

func appendUnique64bit(slice []uint64, item uint64) []uint64 {
	for _, sliceElement := range slice {
		if sliceElement == item {
			return slice
		}
	}
	return append(slice, item)
}

func appendUnique32bit(slice []uint32, item uint32) []uint32 {
	for _, sliceElement := range slice {
		if sliceElement == item {
			return slice
		}
	}
	return append(slice, item)
}

// getNniIntfID gets nni intf id from the flow classifier/action
func getNniIntfID(ctx context.Context, classifier map[string]interface{}, action map[string]interface{}) (uint32, error) {

	portType := plt.IntfIDToPortTypeName(classifier[InPort].(uint32))
	if portType == voltha.Port_PON_OLT {
		intfID, err := plt.IntfIDFromNniPortNum(ctx, action[Output].(uint32))
		if err != nil {
			logger.Debugw(ctx, "invalid-action-port-number",
				log.Fields{
					"port-number": action[Output].(uint32),
					"err":         err})
			return uint32(0), err
		}
		logger.Infow(ctx, "output-nni-intfId-is", log.Fields{"intf-id": intfID})
		return intfID, nil
	} else if portType == voltha.Port_ETHERNET_NNI {
		intfID, err := plt.IntfIDFromNniPortNum(ctx, classifier[InPort].(uint32))
		if err != nil {
			logger.Debugw(ctx, "invalid-classifier-port-number",
				log.Fields{
					"port-number": action[Output].(uint32),
					"err":         err})
			return uint32(0), err
		}
		logger.Infow(ctx, "input-nni-intfId-is", log.Fields{"intf-id": intfID})
		return intfID, nil
	}
	return uint32(0), nil
}

// UpdateGemPortForPktIn updates gemport for packet-in in to the cache and to the kv store as well.
func (f *OpenOltFlowMgr) UpdateGemPortForPktIn(ctx context.Context, intfID uint32, onuID uint32, logicalPort uint32, gemPort uint32, pkt []byte) {
	cTag, priority, err := getCTagFromPacket(ctx, pkt)
	if err != nil {
		logger.Errorw(ctx, "unable-to-update-gem-port-for-packet-in",
			log.Fields{"intfID": intfID, "onuID": onuID, "logicalPort": logicalPort, "gemPort": gemPort, "err": err})
		return
	}
	pktInkey := rsrcMgr.PacketInInfoKey{IntfID: intfID, OnuID: onuID, LogicalPort: logicalPort, VlanID: cTag, Priority: priority}

	f.packetInGemPortLock.RLock()
	lookupGemPort, ok := f.packetInGemPort[pktInkey]
	f.packetInGemPortLock.RUnlock()

	if ok {
		if lookupGemPort == gemPort {
			logger.Infow(ctx, "pktin-key/value-found-in-cache--no-need-to-update-kv--assume-both-in-sync",
				log.Fields{
					"pktinkey": pktInkey,
					"gem":      gemPort})
			return
		}
	}
	f.packetInGemPortLock.Lock()
	f.packetInGemPort[pktInkey] = gemPort
	f.packetInGemPortLock.Unlock()

	f.resourceMgr.UpdateGemPortForPktIn(ctx, pktInkey, gemPort)
	logger.Infow(ctx, "pktin-key-not-found-in-local-cache-value-is-different--updating-cache-and-kv-store",
		log.Fields{
			"pktinkey": pktInkey,
			"gem":      gemPort})
}

//getCTagFromPacket retrieves and returns c-tag and priority value from a packet.
func getCTagFromPacket(ctx context.Context, packet []byte) (uint16, uint8, error) {
	if packet == nil || len(packet) < 18 {
		logger.Error(ctx, "unable-get-c-tag-from-the-packet--invalid-packet-length ")
		return 0, 0, errors.New("invalid packet length")
	}
	outerEthType := (uint16(packet[12]) << 8) | uint16(packet[13])
	innerEthType := (uint16(packet[16]) << 8) | uint16(packet[17])

	var index int8
	if outerEthType == 0x8100 {
		if innerEthType == 0x8100 {
			// q-in-q 802.1ad or 802.1q double tagged packet.
			// get the inner vlanId
			index = 18
		} else {
			index = 14
		}
		priority := (packet[index] >> 5) & 0x7
		//13 bits composes vlanId value
		vlan := ((uint16(packet[index]) << 8) & 0x0fff) | uint16(packet[index+1])
		return vlan, priority, nil
	}
	logger.Debugf(ctx, "No vlanId found in the packet. Returning zero as c-tag")
	return 0, 0, nil
}

func (f *OpenOltFlowMgr) loadFlowIDsForGemAndGemIDsForFlow(ctx context.Context) {
	logger.Debug(ctx, "loadFlowIDsForGemAndGemIDsForFlow - start")
	f.onuGemInfoLock.RLock()
	f.gemToFlowIDsKey.Lock()
	f.flowIDToGemsLock.Lock()
	for _, og := range f.onuGemInfoMap {
		for _, gem := range og.GemPorts {
			flowIDs, err := f.resourceMgr.GetFlowIDsForGem(ctx, f.ponPortIdx, gem)
			if err == nil {
				f.gemToFlowIDs[gem] = flowIDs
				for _, flowID := range flowIDs {
					if _, ok := f.flowIDToGems[flowID]; !ok {
						f.flowIDToGems[flowID] = []uint32{gem}
					} else {
						f.flowIDToGems[flowID] = appendUnique32bit(f.flowIDToGems[flowID], gem)
					}
				}
			}
		}
	}
	f.flowIDToGemsLock.Unlock()
	f.gemToFlowIDsKey.Unlock()
	f.onuGemInfoLock.RUnlock()
	logger.Debug(ctx, "loadFlowIDsForGemAndGemIDsForFlow - end")
}

//clearMulticastFlowFromResourceManager  removes a multicast flow from the KV store and
// clears resources reserved for this multicast flow
func (f *OpenOltFlowMgr) clearMulticastFlowFromResourceManager(ctx context.Context, flow *ofp.OfpFlowStats) error {
	removeFlowMessage := openoltpb2.Flow{FlowId: flow.Id, FlowType: Multicast}
	logger.Debugw(ctx, "multicast-flow-to-be-deleted",
		log.Fields{
			"flow":      flow,
			"flow-id":   flow.Id,
			"device-id": f.deviceHandler.device.Id})
	// Remove from device
	if err := f.removeFlowFromDevice(ctx, &removeFlowMessage, flow.Id); err != nil {
		// DKB
		logger.Errorw(ctx, "failed-to-remove-multicast-flow",
			log.Fields{
				"flow-id": flow.Id,
				"err":     err})
		return err
	}

	return nil
}

func (f *OpenOltFlowMgr) getTechProfileDownloadMessage(ctx context.Context, tpPath string, uniID uint32, onuDeviceID string) (*ia.TechProfileDownloadMessage, error) {
	tpInst, err := f.techprofile.GetTPInstance(ctx, tpPath)
	if err != nil {
		logger.Errorw(ctx, "error-fetching-tp-instance", log.Fields{"tpPath": tpPath})
		return nil, err
	}

	switch tpInst := tpInst.(type) {
	case *tp_pb.TechProfileInstance:
		logger.Debugw(ctx, "fetched-tp-instance-successfully-formulating-tp-download-msg", log.Fields{"tpPath": tpPath})
		return &ia.TechProfileDownloadMessage{
			DeviceId:       onuDeviceID,
			UniId:          uniID,
			TpInstancePath: tpPath,
			TechTpInstance: &ia.TechProfileDownloadMessage_TpInstance{TpInstance: tpInst},
		}, nil
	case *tp_pb.EponTechProfileInstance:
		return &ia.TechProfileDownloadMessage{
			DeviceId:       onuDeviceID,
			UniId:          uniID,
			TpInstancePath: tpPath,
			TechTpInstance: &ia.TechProfileDownloadMessage_EponTpInstance{EponTpInstance: tpInst},
		}, nil
	default:
		logger.Errorw(ctx, "unknown-tech", log.Fields{"tpPath": tpPath})
	}
	return &ia.TechProfileDownloadMessage{
		DeviceId:       onuDeviceID,
		UniId:          uniID,
		TpInstancePath: tpPath,
		TechTpInstance: nil,
	}, nil
}

func (f *OpenOltFlowMgr) getOnuGemInfoList(ctx context.Context) []rsrcMgr.OnuGemInfo {
	var onuGemInfoLst []rsrcMgr.OnuGemInfo
	f.onuGemInfoLock.RLock()
	defer f.onuGemInfoLock.RUnlock()
	for _, v := range f.onuGemInfoMap {
		onuGemInfoLst = append(onuGemInfoLst, *v)
	}
	return onuGemInfoLst
}

// revertTechProfileInstance is called when CreateScheduler or CreateQueues request fails
func (f *OpenOltFlowMgr) revertTechProfileInstance(ctx context.Context, sq schedQueue) {

	intfID := sq.intfID
	onuID := sq.onuID
	uniID := sq.uniID
	tpID := sq.tpID

	var reverseDirection string
	if sq.direction == tp_pb.Direction_UPSTREAM {
		reverseDirection = "downstream"
	} else {
		reverseDirection = "upstream"
	}

	// check reverse direction - if reverse meter exists, tech profile instance is in use - do not delete
	if KvStoreMeter, _ := f.resourceMgr.GetMeterInfoForOnu(ctx, reverseDirection, intfID, onuID, uniID, tpID); KvStoreMeter != nil {
		return
	}

	// revert-delete tech-profile instance and delete tech profile id for onu
	logger.Warnw(ctx, "reverting-tech-profile-instance-and-tech-profile-id-for-onu", log.Fields{"intf-id": intfID, "onu-id": onuID, "uni-id": uniID, "tp-id": tpID})
	uniPortName := getUniPortPath(f.deviceHandler.device.Id, intfID, int32(onuID), int32(uniID))
	_ = f.DeleteTechProfileInstance(ctx, intfID, onuID, uniID, uniPortName, tpID)
	_ = f.resourceMgr.RemoveTechProfileIDForOnu(ctx, intfID, onuID, uniID, tpID)

	// free gem/alloc
	switch techprofileInst := sq.tpInst.(type) {
	case *tp_pb.TechProfileInstance:
		for _, gem := range techprofileInst.UpstreamGemPortAttributeList {
			f.resourceMgr.FreeGemPortID(ctx, intfID, onuID, uniID, gem.GemportId)
		}
		f.resourceMgr.FreeAllocID(ctx, intfID, onuID, uniID, techprofileInst.UsScheduler.AllocId)
	}
}

// revertSchduler is called when CreateQueues request fails
func (f *OpenOltFlowMgr) revertScheduler(ctx context.Context, sq schedQueue, TrafficSched []*tp_pb.TrafficScheduler) {
	// revert scheduler
	logger.Warnw(ctx, "reverting-scheduler-for-onu", log.Fields{"intf-id": sq.intfID, "onu-id": sq.onuID, "uni-id": sq.uniID, "tp-id": sq.tpID})
	_, _ = f.deviceHandler.Client.RemoveTrafficSchedulers(ctx, &tp_pb.TrafficSchedulers{
		IntfId: sq.intfID, OnuId: sq.onuID,
		UniId: sq.uniID, PortNo: sq.uniPort,
		TrafficScheds: TrafficSched})
}

// validateMeter validates if there is a meter mismatch for the given direction. It also clears the stale meter if the reference count is zero
func (f *OpenOltFlowMgr) validateMeter(ctx context.Context, direction string, meterID uint32, intfID uint32, onuID uint32, uniID uint32, tpID uint32) error {
	meterInfo, err := f.resourceMgr.GetMeterInfoForOnu(ctx, direction, intfID, onuID, uniID, tpID)
	if err != nil {
		return olterrors.NewErrNotFound("meter",
			log.Fields{"intf-id": intfID,
				"onu-id":    onuID,
				"uni-id":    uniID,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	if meterInfo != nil {
		// If RefCnt become 0 clear the meter information from the DB.
		if meterInfo.MeterID != meterID && meterInfo.RefCnt == 0 {
			if err := f.resourceMgr.RemoveMeterInfoForOnu(ctx, direction, intfID, onuID, uniID, tpID); err != nil {
				return err
			}
		} else if meterInfo.MeterID != meterID {
			logger.Errorw(ctx, "meter-mismatch-for-direction",
				log.Fields{"direction": direction,
					"kv-store-meter-id": meterInfo.MeterID,
					"meter-id-in-flow":  meterID,
					"device-id":         f.deviceHandler.device.Id})
			return olterrors.NewErrInvalidValue(log.Fields{
				"unsupported":       "meter-id",
				"kv-store-meter-id": meterInfo.MeterID,
				"meter-id-in-flow":  meterID,
				"device-id":         f.deviceHandler.device.Id}, nil)
		}
	}
	return nil
}
