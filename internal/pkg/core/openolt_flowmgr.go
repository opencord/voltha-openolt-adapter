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
	"github.com/opencord/voltha-lib-go/v4/pkg/meters"
	"strconv"
	"strings"
	"sync"

	"github.com/opencord/voltha-lib-go/v4/pkg/flows"
	"github.com/opencord/voltha-lib-go/v4/pkg/log"
	tp "github.com/opencord/voltha-lib-go/v4/pkg/techprofile"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"
	"github.com/opencord/voltha-protos/v4/go/common"
	ic "github.com/opencord/voltha-protos/v4/go/inter_container"
	ofp "github.com/opencord/voltha-protos/v4/go/openflow_13"
	openoltpb2 "github.com/opencord/voltha-protos/v4/go/openolt"
	tp_pb "github.com/opencord/voltha-protos/v4/go/tech_profile"
	"github.com/opencord/voltha-protos/v4/go/voltha"

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
	flowMetadata *voltha.FlowMetadata
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
	ctx          context.Context      // Flow handler context
	addFlow      bool                 // if true flow to be added, else removed
	flow         *voltha.OfpFlowStats // Flow message
	flowMetadata *voltha.FlowMetadata // FlowMetadata that contains flow meter information. This can be nil for Flow remove
	errChan      *chan error          // channel to report the Flow handling error
}

//OpenOltFlowMgr creates the Structure of OpenOltFlowMgr obj
type OpenOltFlowMgr struct {
	ponPortIdx    uint32 // Pon Port this FlowManager is responsible for
	techprofile   map[uint32]tp.TechProfileIf
	deviceHandler *DeviceHandler
	grpMgr        *OpenOltGroupMgr
	resourceMgr   *rsrcMgr.OpenOltResourceMgr

	onuIdsLock sync.RWMutex // TODO: Do we need this?

	gemToFlowIDs    map[uint32][]uint64 // gem port id to flow ids
	gemToFlowIDsKey sync.RWMutex        // lock to be used to access the gemToFlowIDs map

	packetInGemPort     map[rsrcMgr.PacketInInfoKey]uint32 //packet in gem port local cache
	packetInGemPortLock sync.RWMutex

	// TODO create a type rsrcMgr.OnuGemInfos to be used instead of []rsrcMgr.OnuGemInfo
	onuGemInfo []rsrcMgr.OnuGemInfo //onu, gem and uni info local cache
	// We need to have a global lock on the onuGemInfo map
	onuGemInfoLock sync.RWMutex

	flowIDToGems     map[uint64][]uint32
	flowIDToGemsLock sync.RWMutex

	// Slice of channels. Each channel in slice, index by ONU ID, queues flows per ONU.
	// A go routine per ONU, waits on the unique channel (indexed by ONU ID) for incoming flows (add/remove)
	incomingFlows []chan flowControlBlock
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
	flowMgr.techprofile = make(map[uint32]tp.TechProfileIf)
	if err = flowMgr.populateTechProfilePerPonPort(ctx); err != nil {
		logger.Errorw(ctx, "error-while-populating-tech-profile-mgr", log.Fields{"error": err})
		return nil
	}
	flowMgr.gemToFlowIDs = make(map[uint32][]uint64)
	flowMgr.packetInGemPort = make(map[rsrcMgr.PacketInInfoKey]uint32)
	flowMgr.flowIDToGems = make(map[uint64][]uint32)

	// Create a slice of buffered channels for handling concurrent flows per ONU.
	// The additional entry (+1) is to handle the NNI trap flows on a separate channel from individual ONUs channel
	flowMgr.incomingFlows = make([]chan flowControlBlock, MaxOnusPerPon+1)
	for i := range flowMgr.incomingFlows {
		flowMgr.incomingFlows[i] = make(chan flowControlBlock, maxConcurrentFlowsPerOnu)
		// Spin up a go routine to handling incoming flows (add/remove).
		// There will be on go routine per ONU.
		// This routine will be blocked on the flowMgr.incomingFlows[onu-id] channel for incoming flows.
		go flowMgr.perOnuFlowHandlerRoutine(flowMgr.incomingFlows[i])
	}

	//Load the onugem info cache from kv store on flowmanager start
	if flowMgr.onuGemInfo, err = rMgr.GetOnuGemInfo(ctx, ponPortIdx); err != nil {
		logger.Error(ctx, "failed-to-load-onu-gem-info-cache")
	}

	//Load flowID list per gem map And gemIDs per flow per interface from the kvstore.
	flowMgr.loadFlowIDlistForGemAndGemIDsForFlow(ctx)

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
	UsMeterID uint32, DsMeterID uint32, flowMetadata *voltha.FlowMetadata) error {
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
	f.checkAndAddFlow(ctx, args, classifierInfo, actionInfo, flow, TpInst, gemPorts, TpID, uni)

	return nil
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

	Direction, err := verifyMeterIDAndGetDirection(sq.meterID, sq.direction)
	if err != nil {
		return err
	}

	/* Lets make a simple assumption that if the meter-id is present on the KV store,
	 * then the scheduler and queues configuration is applied on the OLT device
	 * in the given direction.
	 */

	var SchedCfg *tp_pb.SchedulerConfig
	meterInfo, err := f.resourceMgr.GetMeterInfoForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID)
	if err != nil {
		return olterrors.NewErrNotFound("meter",
			log.Fields{"intf-id": sq.intfID,
				"onu-id":    sq.onuID,
				"uni-id":    sq.uniID,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	if meterInfo != nil {
		logger.Debugw(ctx, "scheduler-already-created-for-upstream", log.Fields{"device-id": f.deviceHandler.device.Id, "meter-id": sq.meterID})
		if meterInfo.MeterID == sq.meterID {
			if err := f.resourceMgr.HandleMeterInfoRefCntUpdate(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID, true); err != nil {
				return err
			}
			return nil
		}
		return olterrors.NewErrInvalidValue(log.Fields{
			"unsupported":       "meter-id",
			"kv-store-meter-id": meterInfo.MeterID,
			"meter-id-in-flow":  sq.meterID,
			"device-id":         f.deviceHandler.device.Id}, nil)
	}

	logger.Debugw(ctx, "meter-does-not-exist-creating-new",
		log.Fields{
			"meter-id":  sq.meterID,
			"direction": Direction,
			"device-id": f.deviceHandler.device.Id})

	if sq.direction == tp_pb.Direction_UPSTREAM {
		SchedCfg, err = f.techprofile[sq.intfID].GetUsScheduler(ctx, sq.tpInst.(*tp.TechProfile))
	} else if sq.direction == tp_pb.Direction_DOWNSTREAM {
		SchedCfg, err = f.techprofile[sq.intfID].GetDsScheduler(ctx, sq.tpInst.(*tp.TechProfile))
	}

	if err != nil {
		return olterrors.NewErrNotFound("scheduler-config",
			log.Fields{
				"intf-id":   sq.intfID,
				"direction": sq.direction,
				"tp-inst":   sq.tpInst,
				"device-id": f.deviceHandler.device.Id}, err)
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

	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile[sq.intfID].GetTrafficScheduler(sq.tpInst.(*tp.TechProfile), SchedCfg, TrafficShaping)}
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
	if err := f.resourceMgr.StoreMeterInfoForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID, meterInfo); err != nil {
		return olterrors.NewErrAdapter("failed-updating-meter-id",
			log.Fields{"onu-id": sq.onuID,
				"meter-id":  sq.meterID,
				"device-id": f.deviceHandler.device.Id}, err)
	}
	logger.Infow(ctx, "updated-meter-info-into-kv-store-successfully",
		log.Fields{"direction": Direction,
			"meter-info": meterInfo,
			"device-id":  f.deviceHandler.device.Id})
	return nil
}

func (f *OpenOltFlowMgr) pushSchedulerQueuesToDevice(ctx context.Context, sq schedQueue, TrafficSched []*tp_pb.TrafficScheduler) error {
	trafficQueues, err := f.techprofile[sq.intfID].GetTrafficQueues(ctx, sq.tpInst.(*tp.TechProfile), sq.direction)

	if err != nil {
		return olterrors.NewErrAdapter("unable-to-construct-traffic-queue-configuration",
			log.Fields{"intf-id": sq.intfID,
				"direction": sq.direction,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	logger.Debugw(ctx, "sending-traffic-scheduler-create-to-device",
		log.Fields{
			"direction":     sq.direction,
			"TrafficScheds": TrafficSched,
			"device-id":     f.deviceHandler.device.Id})
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

	// On receiving the CreateTrafficQueues request, the driver should create corresponding
	// downstream queues.
	logger.Debugw(ctx, "sending-traffic-queues-create-to-device",
		log.Fields{"direction": sq.direction,
			"traffic-queues": trafficQueues,
			"device-id":      f.deviceHandler.device.Id})
	if _, err := f.deviceHandler.Client.CreateTrafficQueues(ctx,
		&tp_pb.TrafficQueues{IntfId: sq.intfID, OnuId: sq.onuID,
			UniId: sq.uniID, PortNo: sq.uniPort,
			TrafficQueues: trafficQueues,
			TechProfileId: TrafficSched[0].TechProfileId}); err != nil {
		return olterrors.NewErrAdapter("failed-to-create-traffic-queues-in-device", log.Fields{"traffic-queues": trafficQueues}, err)
	}
	logger.Infow(ctx, "successfully-created-traffic-schedulers", log.Fields{
		"direction":      sq.direction,
		"traffic-queues": trafficQueues,
		"device-id":      f.deviceHandler.device.Id})

	if sq.direction == tp_pb.Direction_DOWNSTREAM {
		multicastTrafficQueues := f.techprofile[sq.intfID].GetMulticastTrafficQueues(ctx, sq.tpInst.(*tp.TechProfile))
		if len(multicastTrafficQueues) > 0 {
			if _, present := f.grpMgr.GetInterfaceToMcastQueueMap(sq.intfID); !present {
				//assumed that there is only one queue per PON for the multicast service
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
					logger.Errorw(ctx, "failed-to-add-mcast-queue", log.Fields{"error": err})
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
		SchedCfg, err = f.techprofile[sq.intfID].GetUsScheduler(ctx, sq.tpInst.(*tp.TechProfile))
		Direction = "upstream"
	} else if sq.direction == tp_pb.Direction_DOWNSTREAM {
		SchedCfg, err = f.techprofile[sq.intfID].GetDsScheduler(ctx, sq.tpInst.(*tp.TechProfile))
		Direction = "downstream"
	}

	if err != nil {
		return olterrors.NewErrNotFound("scheduler-config",
			log.Fields{
				"int-id":    sq.intfID,
				"direction": sq.direction,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	TrafficShaping := &tp_pb.TrafficShapingInfo{} // this info is not really useful for the agent during flow removal. Just use default values.

	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile[sq.intfID].GetTrafficScheduler(sq.tpInst.(*tp.TechProfile), SchedCfg, TrafficShaping)}
	TrafficSched[0].TechProfileId = sq.tpID

	TrafficQueues, err := f.techprofile[sq.intfID].GetTrafficQueues(ctx, sq.tpInst.(*tp.TechProfile), sq.direction)
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
	logger.Infow(ctx, "removed-traffic-queues-successfully", log.Fields{"device-id": f.deviceHandler.device.Id})
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

// This function allocates tconts and GEM ports for an ONU
func (f *OpenOltFlowMgr) createTcontGemports(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, uni string, uniPort uint32, TpID uint32, UsMeterID uint32, DsMeterID uint32, flowMetadata *voltha.FlowMetadata) (uint32, []uint32, interface{}) {
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
	techProfileInstance, _ := f.techprofile[intfID].GetTPInstanceFromKVStore(ctx, TpID, tpPath)
	if techProfileInstance == nil {
		logger.Infow(ctx, "tp-instance-not-found--creating-new",
			log.Fields{
				"path":      tpPath,
				"device-id": f.deviceHandler.device.Id})
		techProfileInstance, err = f.techprofile[intfID].CreateTechProfInstance(ctx, TpID, uni, intfID)
		if err != nil {
			// This should not happen, something wrong in KV backend transaction
			logger.Errorw(ctx, "tp-instance-create-failed",
				log.Fields{
					"error":     err,
					"tp-id":     TpID,
					"device-id": f.deviceHandler.device.Id})
			return 0, nil, nil
		}
		if err := f.resourceMgr.UpdateTechProfileIDForOnu(ctx, intfID, onuID, uniID, TpID); err != nil {
			logger.Warnw(ctx, "failed-to-update-tech-profile-id", log.Fields{"error": err})
		}
	} else {
		logger.Debugw(ctx, "tech-profile-instance-already-exist-for-given port-name",
			log.Fields{
				"uni":       uni,
				"device-id": f.deviceHandler.device.Id})
		tpInstanceExists = true
	}

	switch tpInst := techProfileInstance.(type) {
	case *tp.TechProfile:
		if UsMeterID != 0 {
			sq := schedQueue{direction: tp_pb.Direction_UPSTREAM, intfID: intfID, onuID: onuID, uniID: uniID, tpID: TpID,
				uniPort: uniPort, tpInst: techProfileInstance, meterID: UsMeterID, flowMetadata: flowMetadata}
			if err := f.CreateSchedulerQueues(ctx, sq); err != nil {
				logger.Errorw(ctx, "CreateSchedulerQueues-failed-upstream",
					log.Fields{
						"error":     err,
						"onu-id":    onuID,
						"uni-id":    uniID,
						"intf-id":   intfID,
						"meter-id":  UsMeterID,
						"device-id": f.deviceHandler.device.Id})
				return 0, nil, nil
			}
		}
		if DsMeterID != 0 {
			sq := schedQueue{direction: tp_pb.Direction_DOWNSTREAM, intfID: intfID, onuID: onuID, uniID: uniID, tpID: TpID,
				uniPort: uniPort, tpInst: techProfileInstance, meterID: DsMeterID, flowMetadata: flowMetadata}
			if err := f.CreateSchedulerQueues(ctx, sq); err != nil {
				logger.Errorw(ctx, "CreateSchedulerQueues-failed-downstream",
					log.Fields{
						"error":     err,
						"onu-id":    onuID,
						"uni-id":    uniID,
						"intf-id":   intfID,
						"meter-id":  DsMeterID,
						"device-id": f.deviceHandler.device.Id})
				return 0, nil, nil
			}
		}
		allocID := tpInst.UsScheduler.AllocID
		for _, gem := range tpInst.UpstreamGemPortAttributeList {
			gemPortIDs = append(gemPortIDs, gem.GemportID)
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
	case *tp.EponProfile:
		// CreateSchedulerQueues for EPON needs to be implemented here
		// when voltha-protos for EPON is completed.
		allocID := tpInst.AllocID
		for _, gem := range tpInst.UpstreamQueueAttributeList {
			gemPortIDs = append(gemPortIDs, gem.GemportID)
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

func (f *OpenOltFlowMgr) populateTechProfilePerPonPort(ctx context.Context) error {
	var tpCount int
	for _, techRange := range f.resourceMgr.DevInfo.Ranges {
		for _, intfID := range techRange.IntfIds {
			f.techprofile[intfID] = f.resourceMgr.ResourceMgrs[intfID].TechProfileMgr
			tpCount++
			logger.Debugw(ctx, "init-tech-profile-done",
				log.Fields{
					"intf-id":   intfID,
					"device-id": f.deviceHandler.device.Id})
		}
	}
	//Make sure we have as many tech_profiles as there are pon ports on the device
	if tpCount != int(f.resourceMgr.DevInfo.GetPonPorts()) {
		return olterrors.NewErrInvalidValue(log.Fields{
			"reason":             "tP-count-does-not-match-number-of-pon-ports",
			"tech-profile-count": tpCount,
			"pon-port-count":     f.resourceMgr.DevInfo.GetPonPorts(),
			"device-id":          f.deviceHandler.device.Id}, nil)
	}
	logger.Infow(ctx, "populated-techprofile-for-ponports-successfully",
		log.Fields{
			"numofTech":   tpCount,
			"numPonPorts": f.resourceMgr.DevInfo.GetPonPorts(),
			"device-id":   f.deviceHandler.device.Id})
	return nil
}

func (f *OpenOltFlowMgr) addUpstreamDataPathFlow(ctx context.Context, flowContext *flowContext) error {
	flowContext.classifier[PacketTagType] = SingleTag
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

	downlinkClassifier[PacketTagType] = DoubleTag
	logger.Debugw(ctx, "adding-downstream-data-flow",
		log.Fields{
			"downlinkClassifier": downlinkClassifier,
			"downlinkAction":     downlinkAction})
	// Ignore Downlink trap flow given by core, cannot do anything with this flow */
	if vlan, exists := downlinkClassifier[VlanVid]; exists {
		if vlan.(uint32) == (uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 4000) { //private VLAN given by core
			if metadata, exists := downlinkClassifier[Metadata]; exists { // inport is filled in metadata by core
				if uint32(metadata.(uint64)) == MkUniPortNum(ctx, flowContext.intfID, flowContext.onuID, flowContext.uniID) {
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

	/* Already this info available classifier? */
	downlinkAction[PopVlan] = true
	// vlan_vid is a uint32.  must be type asserted as such or conversion fails
	dlClVid, ok := downlinkClassifier[VlanVid].(uint32)
	if ok {
		downlinkAction[VlanVid] = dlClVid & 0xfff
	} else {
		return olterrors.NewErrInvalidValue(log.Fields{
			"reason":    "failed-to-convert-vlanid-classifier",
			"vlan-id":   VlanVid,
			"device-id": f.deviceHandler.device.Id}, nil).Log()
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
	if metadata, ok := classifierInfo[Metadata].(uint64); ok {
		vid := uint32(metadata)
		if vid != ReservedVlan {
			classifier.IVid = vid
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
	} else {
		return nil, olterrors.NewErrInvalidValue(log.Fields{"action-command": actionInfo}, nil)
	}
	return &action, nil
}

// getTPpath return the ETCD path for a given UNI port
func (f *OpenOltFlowMgr) getTPpath(ctx context.Context, intfID uint32, uniPath string, TpID uint32) string {
	return f.techprofile[intfID].GetTechProfileInstanceKVPath(ctx, TpID, uniPath)
}

// DeleteTechProfileInstances removes the tech profile instances from persistent storage
func (f *OpenOltFlowMgr) DeleteTechProfileInstances(ctx context.Context, intfID uint32, onuID uint32, uniID uint32) error {
	tpIDList := f.resourceMgr.GetTechProfileIDForOnu(ctx, intfID, onuID, uniID)
	uniPortName := getUniPortPath(f.deviceHandler.device.Id, intfID, int32(onuID), int32(uniID))

	for _, tpID := range tpIDList {
		if err := f.DeleteTechProfileInstance(ctx, intfID, onuID, uniID, uniPortName, tpID); err != nil {
			_ = olterrors.NewErrAdapter("delete-tech-profile-failed", log.Fields{"device-id": f.deviceHandler.device.Id}, err).Log()
			// return err
			// We should continue to delete tech-profile instances for other TP IDs
		}
		logger.Debugw(ctx, "tech-profile-deleted", log.Fields{"device-id": f.deviceHandler.device.Id, "tp-id": tpID})
	}
	return nil
}

// DeleteTechProfileInstance removes the tech profile instance from persistent storage
func (f *OpenOltFlowMgr) DeleteTechProfileInstance(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, uniPortName string, tpID uint32) error {
	if uniPortName == "" {
		uniPortName = getUniPortPath(f.deviceHandler.device.Id, intfID, int32(onuID), int32(uniID))
	}
	if err := f.techprofile[intfID].DeleteTechProfileInstance(ctx, tpID, uniPortName); err != nil {
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

	networkInterfaceID, err := IntfIDFromNniPortNum(ctx, portNo)
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
		onuDev = NewOnuDevice(onuDevice.Id, onuDevice.Type, onuDevice.SerialNumber, onuDevice.ProxyAddress.OnuId, onuDevice.ProxyAddress.ChannelId, onuDevice.ProxyAddress.DeviceId, false)
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
	parentPortNo := IntfIDToPortNo(intfID, voltha.Port_PON_OLT)
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

	delGemPortMsg := &ic.InterAdapterDeleteGemPortMessage{UniId: uniID, TpPath: tpPath, GemPortId: gemPortID}
	logger.Debugw(ctx, "sending-gem-port-delete-to-openonu-adapter",
		log.Fields{
			"msg":       *delGemPortMsg,
			"device-id": f.deviceHandler.device.Id})
	if sendErr := f.deviceHandler.AdapterProxy.SendInterAdapterMessage(log.WithSpanFromContext(context.Background(), ctx),
		delGemPortMsg,
		ic.InterAdapterMessageType_DELETE_GEM_PORT_REQUEST,
		f.deviceHandler.openOLT.config.Topic,
		onuDev.deviceType,
		onuDev.deviceID,
		onuDev.proxyDeviceID, ""); sendErr != nil {
		return olterrors.NewErrCommunication("send-delete-gem-port-to-onu-adapter",
			log.Fields{
				"from-adapter":  f.deviceHandler.openOLT.config.Topic,
				"to-adapter":    onuDev.deviceType,
				"onu-id":        onuDev.deviceID,
				"proxyDeviceID": onuDev.proxyDeviceID,
				"device-id":     f.deviceHandler.device.Id}, sendErr)
	}
	logger.Infow(ctx, "success-sending-del-gem-port-to-onu-adapter",
		log.Fields{
			"msg":          delGemPortMsg,
			"from-adapter": f.deviceHandler.device.Type,
			"to-adapter":   onuDev.deviceType,
			"device-id":    f.deviceHandler.device.Id})
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

	delTcontMsg := &ic.InterAdapterDeleteTcontMessage{UniId: uniID, TpPath: tpPath, AllocId: allocID}
	logger.Debugw(ctx, "sending-tcont-delete-to-openonu-adapter",
		log.Fields{
			"msg":       *delTcontMsg,
			"device-id": f.deviceHandler.device.Id})
	if sendErr := f.deviceHandler.AdapterProxy.SendInterAdapterMessage(log.WithSpanFromContext(context.Background(), ctx),
		delTcontMsg,
		ic.InterAdapterMessageType_DELETE_TCONT_REQUEST,
		f.deviceHandler.openOLT.config.Topic,
		onuDev.deviceType,
		onuDev.deviceID,
		onuDev.proxyDeviceID, ""); sendErr != nil {
		return olterrors.NewErrCommunication("send-delete-tcont-to-onu-adapter",
			log.Fields{
				"from-adapter": f.deviceHandler.openOLT.config.Topic,
				"to-adapter":   onuDev.deviceType, "onu-id": onuDev.deviceID,
				"proxyDeviceID": onuDev.proxyDeviceID,
				"device-id":     f.deviceHandler.device.Id}, sendErr)
	}
	logger.Infow(ctx, "success-sending-del-tcont-to-onu-adapter",
		log.Fields{
			"msg":       delTcontMsg,
			"device-id": f.deviceHandler.device.Id})
	return nil
}

// Once the gemport is released for a given onu, it also has to be cleared from local cache
// which was used for deriving the gemport->logicalPortNo during packet-in.
// Otherwise stale info continues to exist after gemport is freed and wrong logicalPortNo
// is conveyed to ONOS during packet-in OF message.
func (f *OpenOltFlowMgr) deleteGemPortFromLocalCache(ctx context.Context, intfID uint32, onuID uint32, gemPortID uint32) {

	f.onuGemInfoLock.Lock()
	defer f.onuGemInfoLock.Unlock()

	logger.Infow(ctx, "deleting-gem-from-local-cache",
		log.Fields{
			"gem-port-id": gemPortID,
			"intf-id":     intfID,
			"onu-id":      onuID,
			"device-id":   f.deviceHandler.device.Id,
			"onu-gem":     f.onuGemInfo})

	onugem := f.onuGemInfo
deleteLoop:
	for i, onu := range onugem {
		if onu.OnuID == onuID {
			for j, gem := range onu.GemPorts {
				// If the gemport is found, delete it from local cache.
				if gem == gemPortID {
					onu.GemPorts = append(onu.GemPorts[:j], onu.GemPorts[j+1:]...)
					onugem[i] = onu
					logger.Infow(ctx, "removed-gemport-from-local-cache",
						log.Fields{
							"intf-id":           intfID,
							"onu-id":            onuID,
							"deletedgemport-id": gemPortID,
							"gemports":          onu.GemPorts,
							"device-id":         f.deviceHandler.device.Id})
					break deleteLoop
				}
			}
			break deleteLoop
		}
	}
}

//clearResources clears pon resources in kv store and the device
// nolint: gocyclo
func (f *OpenOltFlowMgr) clearResources(ctx context.Context, intfID uint32, onuID int32, uniID int32,
	gemPortID int32, flowID uint64, portNum uint32, tpID uint32) error {

	uni := getUniPortPath(f.deviceHandler.device.Id, intfID, onuID, uniID)
	tpPath := f.getTPpath(ctx, intfID, uni, tpID)
	logger.Debugw(ctx, "getting-techprofile-instance-for-subscriber",
		log.Fields{
			"tpPath":    tpPath,
			"device-id": f.deviceHandler.device.Id})

	used := f.isGemPortUsedByAnotherFlow(uint32(gemPortID))

	if used {
		f.gemToFlowIDsKey.Lock()
		defer f.gemToFlowIDsKey.Unlock()

		flowIDs := f.gemToFlowIDs[uint32(gemPortID)]
		for i, flowIDinMap := range flowIDs {
			if flowIDinMap == flowID {
				flowIDs = append(flowIDs[:i], flowIDs[i+1:]...)
				// everytime gemToFlowIDs cache is updated the same should be updated
				// in kv store by calling UpdateFlowIDsForGem
				f.gemToFlowIDs[uint32(gemPortID)] = flowIDs
				if err := f.resourceMgr.UpdateFlowIDsForGem(ctx, intfID, uint32(gemPortID), flowIDs); err != nil {
					return err
				}
				break
			}
		}
		logger.Debugw(ctx, "gem-port-id-is-still-used-by-other-flows",
			log.Fields{
				"gemport-id":  gemPortID,
				"usedByFlows": flowIDs,
				"device-id":   f.deviceHandler.device.Id})

		return nil
	}
	logger.Debugf(ctx, "gem-port-id %d is-not-used-by-another-flow--releasing-the-gem-port", gemPortID)
	f.resourceMgr.RemoveGemPortIDForOnu(ctx, intfID, uint32(onuID), uint32(uniID), uint32(gemPortID))
	f.deleteGemPortFromLocalCache(ctx, intfID, uint32(onuID), uint32(gemPortID))

	f.onuIdsLock.Lock() // TODO: What is this lock?

	//everytime an entry is deleted from gemToFlowIDs cache, the same should be updated in kv as well
	// by calling DeleteFlowIDsForGem
	f.gemToFlowIDsKey.Lock()
	delete(f.gemToFlowIDs, uint32(gemPortID))
	f.gemToFlowIDsKey.Unlock()
	f.resourceMgr.DeleteFlowIDsForGem(ctx, intfID, uint32(gemPortID))
	f.resourceMgr.FreeGemPortID(ctx, intfID, uint32(onuID), uint32(uniID), uint32(gemPortID))

	f.onuIdsLock.Unlock()

	// Delete the gem port on the ONU.
	if err := f.sendDeleteGemPortToChild(ctx, intfID, uint32(onuID), uint32(uniID), uint32(gemPortID), tpPath); err != nil {
		logger.Errorw(ctx, "error-processing-delete-gem-port-towards-onu",
			log.Fields{
				"err":        err,
				"intfID":     intfID,
				"onu-id":     onuID,
				"uni-id":     uniID,
				"device-id":  f.deviceHandler.device.Id,
				"gemport-id": gemPortID})
	}
	techprofileInst, err := f.techprofile[intfID].GetTPInstanceFromKVStore(ctx, tpID, tpPath)
	if err != nil || techprofileInst == nil { // This should not happen, something wrong in KV backend transaction
		return olterrors.NewErrNotFound("tech-profile-in-kv-store",
			log.Fields{
				"tp-id": tpID,
				"path":  tpPath}, err)
	}
	switch techprofileInst := techprofileInst.(type) {
	case *tp.TechProfile:
		techprofileInst, err := f.techprofile[intfID].GetTPInstanceFromKVStore(ctx, tpID, tpPath)
		if err != nil || techprofileInst == nil { // This should not happen, something wrong in KV backend transaction
			return olterrors.NewErrNotFound("tech-profile-in-kv-store",
				log.Fields{
					"tp-id": tpID,
					"path":  tpPath}, err)
		}
		ok, _ := f.isTechProfileUsedByAnotherGem(ctx, intfID, uint32(onuID), uint32(uniID), tpID, techprofileInst, uint32(gemPortID))
		if !ok {
			if err := f.resourceMgr.RemoveTechProfileIDForOnu(ctx, intfID, uint32(onuID), uint32(uniID), tpID); err != nil {
				logger.Warn(ctx, err)
			}
			if err := f.DeleteTechProfileInstance(ctx, intfID, uint32(onuID), uint32(uniID), "", tpID); err != nil {
				logger.Warn(ctx, err)
			}
			if err := f.RemoveSchedulerQueues(ctx, schedQueue{direction: tp_pb.Direction_UPSTREAM, intfID: intfID, onuID: uint32(onuID), uniID: uint32(uniID), tpID: tpID, uniPort: portNum, tpInst: techprofileInst}); err != nil {
				logger.Warn(ctx, err)
			}
			if err := f.RemoveSchedulerQueues(ctx, schedQueue{direction: tp_pb.Direction_DOWNSTREAM, intfID: intfID, onuID: uint32(onuID), uniID: uint32(uniID), tpID: tpID, uniPort: portNum, tpInst: techprofileInst}); err != nil {
				logger.Warn(ctx, err)
			}
			f.resourceMgr.FreeAllocID(ctx, intfID, uint32(onuID), uint32(uniID), techprofileInst.UsScheduler.AllocID)
			// Delete the TCONT on the ONU.
			if err := f.sendDeleteTcontToChild(ctx, intfID, uint32(onuID), uint32(uniID), techprofileInst.UsScheduler.AllocID, tpPath); err != nil {
				logger.Errorw(ctx, "error-processing-delete-tcont-towards-onu",
					log.Fields{
						"intf":      intfID,
						"onu-id":    onuID,
						"uni-id":    uniID,
						"device-id": f.deviceHandler.device.Id,
						"alloc-id":  techprofileInst.UsScheduler.AllocID})
			}
		}
	case *tp.EponProfile:
		if err := f.resourceMgr.RemoveTechProfileIDForOnu(ctx, intfID, uint32(onuID), uint32(uniID), tpID); err != nil {
			logger.Warn(ctx, err)
		}
		if err := f.DeleteTechProfileInstance(ctx, intfID, uint32(onuID), uint32(uniID), "", tpID); err != nil {
			logger.Warn(ctx, err)
		}
		f.resourceMgr.FreeAllocID(ctx, intfID, uint32(onuID), uint32(uniID), techprofileInst.AllocID)
		// Delete the TCONT on the ONU.
		if err := f.sendDeleteTcontToChild(ctx, intfID, uint32(onuID), uint32(uniID), techprofileInst.AllocID, tpPath); err != nil {
			logger.Errorw(ctx, "error-processing-delete-tcont-towards-onu",
				log.Fields{
					"intf":      intfID,
					"onu-id":    onuID,
					"uni-id":    uniID,
					"device-id": f.deviceHandler.device.Id,
					"alloc-id":  techprofileInst.AllocID})
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

	classifierInfo := make(map[string]interface{})

	portNum, Intf, onu, uni, inPort, ethType, err := FlowExtractInfo(ctx, flow, flowDirection)
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

	for _, field := range flows.GetOfbFields(flow) {
		if field.Type == flows.IP_PROTO {
			classifierInfo[IPProto] = field.GetIpProto()
			logger.Debugw(ctx, "field-type-ip-proto", log.Fields{"classifierInfo[IP_PROTO]": classifierInfo[IPProto].(uint32)})
		}
	}
	logger.Infow(ctx, "extracted-access-info-from-flow-to-be-deleted",
		log.Fields{
			"flow-id": flow.Id,
			"intf-id": Intf,
			"onu-id":  onuID,
			"uni-id":  uniID})

	if ethType == LldpEthType || ((classifierInfo[IPProto] == IPProtoDhcp) && (flowDirection == "downstream")) {
		onuID = -1
		uniID = -1
		logger.Debug(ctx, "trap-on-nni-flow-set-oni--uni-to- -1")
		Intf, err = IntfIDFromNniPortNum(ctx, inPort)
		if err != nil {
			logger.Errorw(ctx, "invalid-in-port-number",
				log.Fields{
					"port-number": inPort,
					"error":       err})
			return err
		}
	}

	removeFlowMessage := openoltpb2.Flow{FlowId: flow.Id, AccessIntfId: int32(Intf), OnuId: onuID, UniId: uniID, TechProfileId: tpID, FlowType: flowDirection}
	logger.Debugw(ctx, "flow-to-be-deleted", log.Fields{"flow": flow})
	if err = f.removeFlowFromDevice(ctx, &removeFlowMessage, flow.Id); err != nil {
		return err
	}

	f.flowIDToGemsLock.Lock()
	gems, ok := f.flowIDToGems[flow.Id]
	if !ok {
		logger.Errorw(ctx, "flow-id-to-gem-map-not-found", log.Fields{"flowID": flow.Id})
		f.flowIDToGemsLock.Unlock()
		return olterrors.NewErrNotFound("flow-id-to-gem-map-not-found", log.Fields{"flowID": flow.Id}, nil)
	}
	copyOfGems := make([]uint32, len(gems))
	_ = copy(copyOfGems, gems)
	// Delete the flow-id to gemport list entry from the map now the flow is deleted.
	delete(f.flowIDToGems, flow.Id)
	f.flowIDToGemsLock.Unlock()

	logger.Debugw(ctx, "gems-to-be-cleared", log.Fields{"gems": copyOfGems})
	for _, gem := range copyOfGems {
		if err = f.clearResources(ctx, Intf, onuID, uniID, int32(gem), flow.Id, portNum, tpID); err != nil {
			logger.Errorw(ctx, "failed-to-clear-resources-for-flow", log.Fields{
				"flow-id":   flow.Id,
				"device-id": f.deviceHandler.device.Id,
				"onu-id":    onuID,
				"intf":      Intf,
				"gem":       gem,
				"err":       err,
			})
			return err
		}
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
	} else if IsUpstream(actionInfo[Output].(uint32)) {
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
	if portType := IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_ETHERNET_NNI {
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
func (f *OpenOltFlowMgr) RouteFlowToOnuChannel(ctx context.Context, flow *voltha.OfpFlowStats, addFlow bool, flowMetadata *voltha.FlowMetadata) error {
	// Step1 : Fill flowControlBlock
	// Step2 : Push the flowControlBlock to ONU channel
	// Step3 : Wait on response channel for response
	// Step4 : Return error value
	logger.Debugw(ctx, "process-flow", log.Fields{"flow": flow, "addFlow": addFlow})
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
		_, _, onuID, _ = ExtractAccessFromFlow(inPort, outPort)
	}
	// inPort or outPort is InvalidPort for trap-from-nni flows.
	// In the that case onuID is 0 which is the reserved index for trap-from-nni flows in the f.incomingFlows slice
	// Send the flowCb on the ONU flow channel
	f.incomingFlows[onuID] <- flowCb
	// Wait on the channel for flow handlers return value
	err := <-errChan
	logger.Debugw(ctx, "process-flow--received-resp", log.Fields{"flow": flow, "addFlow": addFlow, "err": err})
	return err
}

// This routine is unique per ONU ID and blocks on flowControlBlock channel for incoming flows
// Each incoming flow is processed in a synchronous manner, i.e., the flow is processed to completion before picking another
func (f *OpenOltFlowMgr) perOnuFlowHandlerRoutine(subscriberFlowChannel chan flowControlBlock) {
	for {
		// block on the channel to receive an incoming flow
		// process the flow completely before proceeding to handle the next flow
		flowCb := <-subscriberFlowChannel
		if flowCb.addFlow {
			logger.Debugw(flowCb.ctx, "adding-flow",
				log.Fields{"device-id": f.deviceHandler.device.Id,
					"flowToAdd": flowCb.flow})
			err := f.AddFlow(flowCb.ctx, flowCb.flow, flowCb.flowMetadata)
			// Pass the return value over the return channel
			*flowCb.errChan <- err
		} else {
			logger.Debugw(flowCb.ctx, "removing-flow",
				log.Fields{"device-id": f.deviceHandler.device.Id,
					"flowToRemove": flowCb.flow})
			err := f.RemoveFlow(flowCb.ctx, flowCb.flow)
			// Pass the return value over the return channel
			*flowCb.errChan <- err
		}
	}
}

// AddFlow add flow to device
// nolint: gocyclo
func (f *OpenOltFlowMgr) AddFlow(ctx context.Context, flow *ofp.OfpFlowStats, flowMetadata *voltha.FlowMetadata) error {
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
	portNo, intfID, onuID, uniID := ExtractAccessFromFlow(classifierInfo[InPort].(uint32), actionInfo[Output].(uint32))

	if ethType, ok := classifierInfo[EthType]; ok {
		if ethType.(uint32) == LldpEthType {
			logger.Info(ctx, "adding-lldp-flow")
			return f.addLLDPFlow(ctx, flow, portNo)
		}
		if ethType.(uint32) == PPPoEDEthType {
			if voltha.Port_ETHERNET_NNI == IntfIDToPortTypeName(classifierInfo[InPort].(uint32)) {
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

	TpID, err := getTpIDFromFlow(ctx, flow)
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
			"tp-id":   TpID,
			"intf-id": intfID,
			"onu-id":  onuID,
			"uni-id":  uniID})
	if IsUpstream(actionInfo[Output].(uint32)) {
		UsMeterID = flows.GetMeterIdFromFlow(flow)
		logger.Debugw(ctx, "upstream-flow-meter-id", log.Fields{"us-meter-id": UsMeterID})
	} else {
		DsMeterID = flows.GetMeterIdFromFlow(flow)
		logger.Debugw(ctx, "downstream-flow-meter-id", log.Fields{"ds-meter-id": DsMeterID})

	}
	return f.processAddFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, TpID, UsMeterID, DsMeterID, flowMetadata)
}

// handleFlowWithGroup adds multicast flow to the device.
func (f *OpenOltFlowMgr) handleFlowWithGroup(ctx context.Context, actionInfo, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) error {
	classifierInfo[PacketTagType] = DoubleTag
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
			logger.Warnw(ctx, "failed-to-remove-flow-group", log.Fields{"group-id": groupID, "error": err})
		}
	}

	return nil
}

//getNNIInterfaceIDOfMulticastFlow returns associated NNI interface id of the inPort criterion if exists; returns the first NNI interface of the device otherwise
func (f *OpenOltFlowMgr) getNNIInterfaceIDOfMulticastFlow(ctx context.Context, classifierInfo map[string]interface{}) (uint32, error) {
	if inPort, ok := classifierInfo[InPort]; ok {
		nniInterfaceID, err := IntfIDFromNniPortNum(ctx, inPort.(uint32))
		if err != nil {
			return 0, olterrors.NewErrInvalidValue(log.Fields{"nni-in-port-number": inPort}, err)
		}
		return nniInterfaceID, nil
	}

	// TODO: For now we support only one NNI port in VOLTHA. We shall use only the first NNI port, i.e., interface-id 0.
	return 0, nil
}

//sendTPDownloadMsgToChild send payload
func (f *OpenOltFlowMgr) sendTPDownloadMsgToChild(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, uni string, TpID uint32) error {

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
	tpDownloadMsg := &ic.InterAdapterTechProfileDownloadMessage{UniId: uniID, Path: tpPath}
	logger.Debugw(ctx, "sending-load-tech-profile-request-to-brcm-onu-adapter", log.Fields{"tpDownloadMsg": *tpDownloadMsg})
	sendErr := f.deviceHandler.AdapterProxy.SendInterAdapterMessage(log.WithSpanFromContext(context.Background(), ctx),
		tpDownloadMsg,
		ic.InterAdapterMessageType_TECH_PROFILE_DOWNLOAD_REQUEST,
		f.deviceHandler.openOLT.config.Topic,
		onuDev.deviceType,
		onuDev.deviceID,
		onuDev.proxyDeviceID, "")
	if sendErr != nil {
		return olterrors.NewErrCommunication("send-techprofile-download-request",
			log.Fields{
				"from-adapter":  f.deviceHandler.openOLT.config.Topic,
				"to-adapter":    onuDev.deviceType,
				"onu-id":        onuDev.deviceID,
				"proxyDeviceID": onuDev.proxyDeviceID}, sendErr)
	}
	logger.Infow(ctx, "success-sending-load-tech-profile-request-to-brcm-onu-adapter", log.Fields{"tpDownloadMsg": *tpDownloadMsg})
	return nil
}

//UpdateOnuInfo function adds onu info to cache and kvstore
func (f *OpenOltFlowMgr) UpdateOnuInfo(ctx context.Context, intfID uint32, onuID uint32, serialNum string) error {

	f.onuGemInfoLock.Lock()
	defer f.onuGemInfoLock.Unlock()
	onugem := f.onuGemInfo
	// If the ONU already exists in onuGemInfo list, nothing to do
	for _, onu := range onugem {
		if onu.OnuID == onuID && onu.SerialNumber == serialNum {
			logger.Debugw(ctx, "onu-id-already-exists-in-cache",
				log.Fields{"onuID": onuID,
					"serialNum": serialNum})
			return nil
		}
	}

	onu := rsrcMgr.OnuGemInfo{OnuID: onuID, SerialNumber: serialNum, IntfID: intfID}
	f.onuGemInfo = append(f.onuGemInfo, onu)
	if err := f.resourceMgr.AddOnuGemInfo(ctx, intfID, onu); err != nil {
		return err
	}
	logger.Infow(ctx, "updated-onuinfo",
		log.Fields{
			"intf-id":    intfID,
			"onu-id":     onuID,
			"serial-num": serialNum,
			"onu":        onu,
			"device-id":  f.deviceHandler.device.Id})
	return nil
}

//addGemPortToOnuInfoMap function adds GEMport to ONU map
func (f *OpenOltFlowMgr) addGemPortToOnuInfoMap(ctx context.Context, intfID uint32, onuID uint32, gemPort uint32) {

	f.onuGemInfoLock.Lock()
	defer f.onuGemInfoLock.Unlock()

	logger.Infow(ctx, "adding-gem-to-onu-info-map",
		log.Fields{
			"gem-port-id": gemPort,
			"intf-id":     intfID,
			"onu-id":      onuID,
			"device-id":   f.deviceHandler.device.Id,
			"onu-gem":     f.onuGemInfo})
	onugem := f.onuGemInfo
	// update the gem to the local cache as well as to kv strore
	for idx, onu := range onugem {
		if onu.OnuID == onuID {
			// check if gem already exists , else update the cache and kvstore
			for _, gem := range onu.GemPorts {
				if gem == gemPort {
					logger.Debugw(ctx, "gem-already-in-cache-no-need-to-update-cache-and-kv-store",
						log.Fields{
							"gem":       gemPort,
							"device-id": f.deviceHandler.device.Id})
					return
				}
			}
			onugem[idx].GemPorts = append(onugem[idx].GemPorts, gemPort)
			f.onuGemInfo = onugem
			break
		}
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
			"device-id":   f.deviceHandler.device.Id,
			"onu-gem":     f.onuGemInfo})
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
			logicalPortNum = MkUniPortNum(ctx, packetIn.IntfId, onuID, uniID)
		}
		// Store the gem port through which the packet_in came. Use the same gem port for packet_out
		f.UpdateGemPortForPktIn(ctx, packetIn.IntfId, onuID, logicalPortNum, packetIn.GemportId, packetIn.Pkt)
	} else if packetIn.IntfType == "nni" {
		logicalPortNum = IntfIDToPortNo(packetIn.IntfId, voltha.Port_ETHERNET_NNI)
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
	tpID uint32, uni string) {
	var gemPortID uint32
	intfID := args[IntfID]
	onuID := args[OnuID]
	uniID := args[UniID]
	portNo := args[PortNo]
	allocID := args[AllocID]
	pbitToGem := make(map[uint32]uint32)
	gemToAes := make(map[uint32]bool)

	var attributes []tp.IGemPortAttribute
	var direction = tp_pb.Direction_UPSTREAM
	switch TpInst := TpInst.(type) {
	case *tp.TechProfile:
		if IsUpstream(actionInfo[Output].(uint32)) {
			attributes = TpInst.UpstreamGemPortAttributeList
		} else {
			attributes = TpInst.DownstreamGemPortAttributeList
			direction = tp_pb.Direction_DOWNSTREAM
		}
	default:
		logger.Errorw(ctx, "unsupported-tech", log.Fields{"tpInst": TpInst})
		return
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
			// this pcp bit traffic.
			for pos, pbitSet := range strings.TrimPrefix(pBitMap, bitMapPrefix) {
				if pbitSet == pbit1 {
					pcp := uint32(len(strings.TrimPrefix(pBitMap, bitMapPrefix))) - 1 - uint32(pos)
					pbitToGem[pcp] = gemID
					gemToAes[gemID], _ = strconv.ParseBool(attributes[idx].AesEncryption)
				}
			}
		}
	} else { // Extract the exact gemport which maps to the PCP classifier in the flow
		if gem := f.techprofile[intfID].GetGemportForPbit(ctx, TpInst, direction, pcp.(uint32)); gem != nil {
			gemPortID = gem.(tp.IGemPortAttribute).GemportID
			gemToAes[gemPortID], _ = strconv.ParseBool(gem.(tp.IGemPortAttribute).AesEncryption)
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
			}
		} else {
			logger.Errorw(ctx, "invalid-classifier-to-handle", log.Fields{"classifier": classifierInfo, "action": actionInfo})
			return
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
		}
	} else {
		logger.Errorw(ctx, "invalid-flow-type-to-handle",
			log.Fields{
				"intf-id":    intfID,
				"onu-id":     onuID,
				"uni-id":     uniID,
				"classifier": classifierInfo,
				"action":     actionInfo,
				"flow":       flow})
		return
	}
	// Send Techprofile download event to child device in go routine as it takes time
	go func() {
		if err := f.sendTPDownloadMsgToChild(ctx, intfID, onuID, uniID, uni, tpID); err != nil {
			logger.Warn(ctx, err)
		}
	}()
}

func (f *OpenOltFlowMgr) isGemPortUsedByAnotherFlow(gemPortID uint32) bool {
	f.gemToFlowIDsKey.RLock()
	flowIDList := f.gemToFlowIDs[gemPortID]
	f.gemToFlowIDsKey.RUnlock()
	return len(flowIDList) > 1

}

func (f *OpenOltFlowMgr) isTechProfileUsedByAnotherGem(ctx context.Context, ponIntf uint32, onuID uint32, uniID uint32, tpID uint32, tpInst *tp.TechProfile, gemPortID uint32) (bool, uint32) {
	currentGemPorts := f.resourceMgr.GetCurrentGEMPortIDsForOnu(ctx, ponIntf, onuID, uniID)
	tpGemPorts := tpInst.UpstreamGemPortAttributeList
	for _, currentGemPort := range currentGemPorts {
		for _, tpGemPort := range tpGemPorts {
			if (currentGemPort == tpGemPort.GemportID) && (currentGemPort != gemPortID) {
				return true, currentGemPort
			}
		}
	}
	if tpInst.InstanceCtrl.Onu == "single-instance" {
		// The TP information for the given TP ID, PON ID, ONU ID, UNI ID should be removed.
		if err := f.resourceMgr.RemoveTechProfileIDForOnu(ctx, ponIntf, onuID, uniID, tpID); err != nil {
			logger.Warn(ctx, err)
		}
		if err := f.DeleteTechProfileInstance(ctx, ponIntf, onuID, uniID, "", tpID); err != nil {
			logger.Warn(ctx, err)
		}

		// Although we cleaned up TP Instance for the given (PON ID, ONU ID, UNI ID), the TP might
		// still be used on other uni ports.
		// So, we need to check and make sure that no other gem port is referring to the given TP ID
		// on any other uni port.
		tpInstances := f.techprofile[ponIntf].FindAllTpInstances(ctx, tpID, ponIntf, onuID).([]tp.TechProfile)
		logger.Debugw(ctx, "got-single-instance-tp-instances", log.Fields{"tp-instances": tpInstances})
		for i := 0; i < len(tpInstances); i++ {
			tpI := tpInstances[i]
			tpGemPorts := tpI.UpstreamGemPortAttributeList
			for _, tpGemPort := range tpGemPorts {
				if tpGemPort.GemportID != gemPortID {
					logger.Debugw(ctx, "single-instance-tp-is-in-use-by-gem", log.Fields{"gemPort": tpGemPort.GemportID})
					return true, tpGemPort.GemportID
				}
			}
		}
	}
	logger.Debug(ctx, "tech-profile-is-not-in-use-by-any-gem")
	return false, 0
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
			classifierInfo[VlanVid] = field.GetVlanVid() & 0xfff
			logger.Debug(ctx, "field-type-vlan-vid", log.Fields{"classifierInfo[VLAN_VID]": classifierInfo[VlanVid].(uint32)})
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
	if isControllerFlow := IsControllerBoundFlow(actionInfo[Output].(uint32)); isControllerFlow {
		logger.Debug(ctx, "controller-bound-trap-flows--getting-inport-from-tunnelid")
		/* Get UNI port/ IN Port from tunnel ID field for upstream controller bound flows  */
		if portType := IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
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
		if portType := IntfIDToPortTypeName(actionInfo[Output].(uint32)); portType == voltha.Port_PON_OLT {
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
		} else if portType := IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
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

	portType := IntfIDToPortTypeName(classifier[InPort].(uint32))
	if portType == voltha.Port_PON_OLT {
		intfID, err := IntfIDFromNniPortNum(ctx, action[Output].(uint32))
		if err != nil {
			logger.Debugw(ctx, "invalid-action-port-number",
				log.Fields{
					"port-number": action[Output].(uint32),
					"error":       err})
			return uint32(0), err
		}
		logger.Infow(ctx, "output-nni-intfId-is", log.Fields{"intf-id": intfID})
		return intfID, nil
	} else if portType == voltha.Port_ETHERNET_NNI {
		intfID, err := IntfIDFromNniPortNum(ctx, classifier[InPort].(uint32))
		if err != nil {
			logger.Debugw(ctx, "invalid-classifier-port-number",
				log.Fields{
					"port-number": action[Output].(uint32),
					"error":       err})
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

func (f *OpenOltFlowMgr) loadFlowIDlistForGemAndGemIDsForFlow(ctx context.Context) {
	logger.Debug(ctx, "loadFlowIDlistForGemAndGemIDsForFlow - start")
	f.onuGemInfoLock.RLock()
	f.gemToFlowIDsKey.Lock()
	f.flowIDToGemsLock.Lock()
	for _, og := range f.onuGemInfo {
		for _, gem := range og.GemPorts {
			flowIDs, err := f.resourceMgr.GetFlowIDsForGem(ctx, f.ponPortIdx, gem)
			if err != nil {
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
	logger.Debug(ctx, "loadFlowIDlistForGemAndGemIDsForFlow - end")
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
				"error":   err})
		return err
	}

	return nil
}
