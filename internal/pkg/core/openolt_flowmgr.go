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
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/opencord/voltha-lib-go/v3/pkg/flows"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	tp "github.com/opencord/voltha-lib-go/v3/pkg/techprofile"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"
	"github.com/opencord/voltha-protos/v3/go/common"
	ic "github.com/opencord/voltha-protos/v3/go/inter_container"
	ofp "github.com/opencord/voltha-protos/v3/go/openflow_13"
	openoltpb2 "github.com/opencord/voltha-protos/v3/go/openolt"
	tp_pb "github.com/opencord/voltha-protos/v3/go/tech_profile"
	"github.com/opencord/voltha-protos/v3/go/voltha"

	//deepcopy "github.com/getlantern/deepcopy"
	"github.com/EagleChen/mapmutex"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// Flow categories

	//HsiaFlow flow category
	HsiaFlow = "HSIA_FLOW"

	//EapolFlow flow category
	EapolFlow = "EAPOL_FLOW"

	//DhcpFlow flow category
	DhcpFlow = "DHCP_FLOW"

	//MulticastFlow flow category
	MulticastFlow = "MULTICAST_FLOW"

	//IgmpFlow flow category
	IgmpFlow = "IGMP_FLOW"

	//IPProtoDhcp flow category
	IPProtoDhcp = 17

	//IPProtoIgmp flow category
	IPProtoIgmp = 2

	//EapEthType eapethtype value
	EapEthType = 0x888e
	//LldpEthType lldp ethtype value
	LldpEthType = 0x88cc
	//IPv4EthType IPv4 ethernet type value
	IPv4EthType = 0x800

	//IgmpProto proto value
	IgmpProto = 2

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

	//NoneOnuID constant
	NoneOnuID = -1
	//NoneUniID constant
	NoneUniID = -1
	//NoneGemPortID constant
	NoneGemPortID = -1

	// BinaryStringPrefix is binary string prefix
	BinaryStringPrefix = "0b"
	// BinaryBit1 is binary bit 1 expressed as a character
	BinaryBit1 = '1'

	// MapMutex
	maxRetry  = 300
	maxDelay  = 100000000
	baseDelay = 10000000
	factor    = 1.1
	jitter    = 0.2
)

type gemPortKey struct {
	intfID  uint32
	gemPort uint32
}

type pendingFlowDeleteKey struct {
	intfID uint32
	onuID  uint32
	uniID  uint32
}

type tpLockKey struct {
	intfID uint32
	onuID  uint32
	uniID  uint32
}

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

type queueInfoBrief struct {
	gemPortID       uint32
	servicePriority uint32
}

//OpenOltFlowMgr creates the Structure of OpenOltFlowMgr obj
type OpenOltFlowMgr struct {
	techprofile        map[uint32]tp.TechProfileIf
	deviceHandler      *DeviceHandler
	resourceMgr        *rsrcMgr.OpenOltResourceMgr
	onuIdsLock         sync.RWMutex
	perGemPortLock     *mapmutex.Mutex                    // lock to be used to access the flowsUsedByGemPort map
	flowsUsedByGemPort map[gemPortKey][]uint32            //gem port id to flow ids
	packetInGemPort    map[rsrcMgr.PacketInInfoKey]uint32 //packet in gem port local cache
	// TODO create a type rsrcMgr.OnuGemInfos to be used instead of []rsrcMgr.OnuGemInfo
	onuGemInfo map[uint32][]rsrcMgr.OnuGemInfo //onu, gem and uni info local cache, indexed by IntfId
	// We need to have a global lock on the onuGemInfo map
	onuGemInfoLock    sync.RWMutex
	pendingFlowDelete sync.Map
	// The mapmutex.Mutex can be fine tuned to use mapmutex.NewCustomizedMapMutex
	perUserFlowHandleLock    *mapmutex.Mutex
	interfaceToMcastQueueMap map[uint32]*queueInfoBrief /*pon interface -> multicast queue map. Required to assign GEM to a bucket during group population*/
}

//NewFlowManager creates OpenOltFlowMgr object and initializes the parameters
func NewFlowManager(ctx context.Context, dh *DeviceHandler, rMgr *rsrcMgr.OpenOltResourceMgr) *OpenOltFlowMgr {
	logger.Infow(ctx, "initializing-flow-manager", log.Fields{"device-id": dh.device.Id})
	var flowMgr OpenOltFlowMgr
	var err error
	var idx uint32

	flowMgr.deviceHandler = dh
	flowMgr.resourceMgr = rMgr
	flowMgr.techprofile = make(map[uint32]tp.TechProfileIf)
	if err = flowMgr.populateTechProfilePerPonPort(ctx); err != nil {
		logger.Errorw(ctx, "error-while-populating-tech-profile-mgr", log.Fields{"error": err})
		return nil
	}
	flowMgr.onuIdsLock = sync.RWMutex{}
	flowMgr.flowsUsedByGemPort = make(map[gemPortKey][]uint32)
	flowMgr.packetInGemPort = make(map[rsrcMgr.PacketInInfoKey]uint32)
	ponPorts := rMgr.DevInfo.GetPonPorts()
	flowMgr.onuGemInfo = make(map[uint32][]rsrcMgr.OnuGemInfo, ponPorts)
	//Load the onugem info cache from kv store on flowmanager start
	for idx = 0; idx < ponPorts; idx++ {
		if flowMgr.onuGemInfo[idx], err = rMgr.GetOnuGemInfo(ctx, idx); err != nil {
			logger.Error(ctx, "failed-to-load-onu-gem-info-cache")
		}
		//Load flowID list per gem map per interface from the kvstore.
		flowMgr.loadFlowIDlistForGem(ctx, idx)
	}
	flowMgr.onuGemInfoLock = sync.RWMutex{}
	flowMgr.pendingFlowDelete = sync.Map{}
	flowMgr.perUserFlowHandleLock = mapmutex.NewCustomizedMapMutex(maxRetry, maxDelay, baseDelay, factor, jitter)
	flowMgr.perGemPortLock = mapmutex.NewCustomizedMapMutex(maxRetry, maxDelay, baseDelay, factor, jitter)
	flowMgr.interfaceToMcastQueueMap = make(map[uint32]*queueInfoBrief)
	//load interface to multicast queue map from kv store
	flowMgr.loadInterfaceToMulticastQueueMap(ctx)
	logger.Info(ctx, "initialization-of-flow-manager-success")
	return &flowMgr
}

func (f *OpenOltFlowMgr) registerFlow(ctx context.Context, flowFromCore *ofp.OfpFlowStats, deviceFlow *openoltpb2.Flow) error {
	gemPK := gemPortKey{uint32(deviceFlow.AccessIntfId), uint32(deviceFlow.GemportId)}
	if f.perGemPortLock.TryLock(gemPK) {
		logger.Debugw(ctx, "registering-flow-for-device ",
			log.Fields{
				"flow":      flowFromCore,
				"device-id": f.deviceHandler.device.Id})
		flowIDList, ok := f.flowsUsedByGemPort[gemPK]
		if !ok {
			flowIDList = []uint32{deviceFlow.FlowId}
		}
		flowIDList = appendUnique(flowIDList, deviceFlow.FlowId)
		f.flowsUsedByGemPort[gemPK] = flowIDList

		f.perGemPortLock.Unlock(gemPK)

		// update the flowids for a gem to the KVstore
		return f.resourceMgr.UpdateFlowIDsForGem(ctx, uint32(deviceFlow.AccessIntfId), uint32(deviceFlow.GemportId), flowIDList)
	}
	logger.Error(ctx, "failed-to-acquire-per-gem-port-lock",
		log.Fields{
			"flow-from-core": flowFromCore,
			"device-id":      f.deviceHandler.device.Id,
			"key":            gemPK,
		})
	return olterrors.NewErrAdapter("failed-to-acquire-per-gem-port-lock", log.Fields{
		"flow-from-core": flowFromCore,
		"device-id":      f.deviceHandler.device.Id,
		"key":            gemPK,
	}, nil)
}

func (f *OpenOltFlowMgr) divideAndAddFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32,
	classifierInfo map[string]interface{}, actionInfo map[string]interface{}, flow *ofp.OfpFlowStats, TpID uint32,
	UsMeterID uint32, DsMeterID uint32, flowMetadata *voltha.FlowMetadata) {
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
		logger.Errorw(ctx, "no-onu-id-for-flow",
			log.Fields{
				"port-no":   portNo,
				"classifer": classifierInfo,
				"action":    actionInfo,
				"device-id": f.deviceHandler.device.Id})
		return
	}

	uni := getUniPortPath(f.deviceHandler.device.Id, intfID, int32(onuID), int32(uniID))
	logger.Debugw(ctx, "uni-port-path", log.Fields{
		"uni":       uni,
		"device-id": f.deviceHandler.device.Id})

	tpLockMapKey := tpLockKey{intfID, onuID, uniID}
	if f.perUserFlowHandleLock.TryLock(tpLockMapKey) {
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
			f.perUserFlowHandleLock.Unlock(tpLockMapKey)
			return
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
		f.perUserFlowHandleLock.Unlock(tpLockMapKey)
	} else {
		logger.Errorw(ctx, "failed-to-acquire-per-user-flow-handle-lock",
			log.Fields{
				"intf-id":     intfID,
				"onu-id":      onuID,
				"uni-id":      uniID,
				"flow-id":     flow.Id,
				"flow-cookie": flow.Cookie,
				"device-id":   f.deviceHandler.device.Id})
		return
	}
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
	KvStoreMeter, err := f.resourceMgr.GetMeterIDForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID)
	if err != nil {
		return olterrors.NewErrNotFound("meter",
			log.Fields{"intf-id": sq.intfID,
				"onu-id":    sq.onuID,
				"uni-id":    sq.uniID,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	if KvStoreMeter != nil {
		if KvStoreMeter.MeterId == sq.meterID {
			logger.Debugw(ctx, "scheduler-already-created-for-upstream", log.Fields{"device-id": f.deviceHandler.device.Id})
			return nil
		}
		return olterrors.NewErrInvalidValue(log.Fields{
			"unsupported":       "meter-id",
			"kv-store-meter-id": KvStoreMeter.MeterId,
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

	var meterConfig *ofp.OfpMeterConfig
	if sq.flowMetadata != nil {
		for _, meter := range sq.flowMetadata.Meters {
			if sq.meterID == meter.MeterId {
				meterConfig = meter
				logger.Debugw(ctx, "found-meter-config-from-flowmetadata",
					log.Fields{"meterConfig": meterConfig,
						"device-id": f.deviceHandler.device.Id})
				break
			}
		}
	} else {
		logger.Errorw(ctx, "flow-metadata-not-present-in-flow", log.Fields{"device-id": f.deviceHandler.device.Id})
	}
	if meterConfig == nil {
		return olterrors.NewErrNotFound("meterbands", log.Fields{
			"reason":        "Could-not-get-meterbands-from-flowMetadata",
			"flow-metadata": sq.flowMetadata,
			"meter-id":      sq.meterID,
			"device-id":     f.deviceHandler.device.Id}, nil)
	} else if len(meterConfig.Bands) < MaxMeterBand {
		logger.Errorw(ctx, "invalid-number-of-bands-in-meter",
			log.Fields{"Bands": meterConfig.Bands,
				"meter-id":  sq.meterID,
				"device-id": f.deviceHandler.device.Id})
		return olterrors.NewErrInvalidValue(log.Fields{
			"reason":          "Invalid-number-of-bands-in-meter",
			"meterband-count": len(meterConfig.Bands),
			"metabands":       meterConfig.Bands,
			"meter-id":        sq.meterID,
			"device-id":       f.deviceHandler.device.Id}, nil)
	}
	cir := meterConfig.Bands[0].Rate
	cbs := meterConfig.Bands[0].BurstSize
	eir := meterConfig.Bands[1].Rate
	ebs := meterConfig.Bands[1].BurstSize
	pir := cir + eir
	pbs := cbs + ebs
	TrafficShaping := &tp_pb.TrafficShapingInfo{Cir: cir, Cbs: cbs, Pir: pir, Pbs: pbs}

	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile[sq.intfID].GetTrafficScheduler(sq.tpInst.(*tp.TechProfile), SchedCfg, TrafficShaping)}
	TrafficSched[0].TechProfileId = sq.tpID

	if err := f.pushSchedulerQueuesToDevice(ctx, sq, TrafficShaping, TrafficSched); err != nil {
		return olterrors.NewErrAdapter("failure-pushing-traffic-scheduler-and-queues-to-device",
			log.Fields{"intf-id": sq.intfID,
				"direction": sq.direction,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	/* After we successfully applied the scheduler configuration on the OLT device,
	 * store the meter id on the KV store, for further reference.
	 */
	if err := f.resourceMgr.UpdateMeterIDForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID, meterConfig); err != nil {
		return olterrors.NewErrAdapter("failed-updating-meter-id",
			log.Fields{"onu-id": sq.onuID,
				"meter-id":  sq.meterID,
				"device-id": f.deviceHandler.device.Id}, err)
	}
	logger.Infow(ctx, "updated-meter-info-into-kv-store-successfully",
		log.Fields{"direction": Direction,
			"Meter":     meterConfig,
			"device-id": f.deviceHandler.device.Id})
	return nil
}

func (f *OpenOltFlowMgr) pushSchedulerQueuesToDevice(ctx context.Context, sq schedQueue, TrafficShaping *tp_pb.TrafficShapingInfo, TrafficSched []*tp_pb.TrafficScheduler) error {
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
			if _, present := f.interfaceToMcastQueueMap[sq.intfID]; !present {
				//assumed that there is only one queue per PON for the multicast service
				//the default queue with multicastQueuePerPonPort.Priority per a pon interface is used for multicast service
				//just put it in interfaceToMcastQueueMap to use for building group members
				logger.Debugw(ctx, "multicast-traffic-queues", log.Fields{"device-id": f.deviceHandler.device.Id})
				multicastQueuePerPonPort := multicastTrafficQueues[0]
				f.interfaceToMcastQueueMap[sq.intfID] = &queueInfoBrief{
					gemPortID:       multicastQueuePerPonPort.GemportId,
					servicePriority: multicastQueuePerPonPort.Priority,
				}
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

	KVStoreMeter, err := f.resourceMgr.GetMeterIDForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID)
	if err != nil {
		return olterrors.NewErrNotFound("meter",
			log.Fields{
				"onu-id":    sq.onuID,
				"device-id": f.deviceHandler.device.Id}, err)
	}
	if KVStoreMeter == nil {
		logger.Warnw(ctx, "no-meter-installed-yet",
			log.Fields{
				"direction": Direction,
				"intf-id":   sq.intfID,
				"onu-id":    sq.onuID,
				"uni-id":    sq.uniID,
				"device-id": f.deviceHandler.device.Id})
		return nil
	}
	cir := KVStoreMeter.Bands[0].Rate
	cbs := KVStoreMeter.Bands[0].BurstSize
	eir := KVStoreMeter.Bands[1].Rate
	ebs := KVStoreMeter.Bands[1].BurstSize
	pir := cir + eir
	pbs := cbs + ebs

	TrafficShaping := &tp_pb.TrafficShapingInfo{Cir: cir, Cbs: cbs, Pir: pir, Pbs: pbs}

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
				"traffic-schedulers": TrafficSched}, err)
	}

	logger.Infow(ctx, "removed-traffic-schedulers-successfully", log.Fields{"device-id": f.deviceHandler.device.Id})

	/* After we successfully remove the scheduler configuration on the OLT device,
	 * delete the meter id on the KV store.
	 */
	err = f.resourceMgr.RemoveMeterIDForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID)
	if err != nil {
		return olterrors.NewErrAdapter("unable-to-remove-meter",
			log.Fields{
				"onu":       sq.onuID,
				"meter":     KVStoreMeter.MeterId,
				"device-id": f.deviceHandler.device.Id}, err)
	}
	logger.Infow(ctx, "removed-meter-from-KV-store-successfully",
		log.Fields{
			"meter-id":  KVStoreMeter.MeterId,
			"dir":       Direction,
			"device-id": f.deviceHandler.device.Id})
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
						"meter-id":  DsMeterID,
						"device-id": f.deviceHandler.device.Id})
				return 0, nil, nil
			}
		}
		allocID := tpInst.UsScheduler.AllocID
		for _, gem := range tpInst.UpstreamGemPortAttributeList {
			gemPortIDs = append(gemPortIDs, gem.GemportID)
		}
		allocIDs = appendUnique(allocIDs, allocID)

		if tpInstanceExists {
			return allocID, gemPortIDs, techProfileInstance
		}

		for _, gemPortID := range gemPortIDs {
			allgemPortIDs = appendUnique(allgemPortIDs, gemPortID)
		}
		logger.Infow(ctx, "allocated-tcont-and-gem-ports",
			log.Fields{
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
		allocIDs = appendUnique(allocIDs, allocID)

		if tpInstanceExists {
			return allocID, gemPortIDs, techProfileInstance
		}

		for _, gemPortID := range gemPortIDs {
			allgemPortIDs = appendUnique(allgemPortIDs, gemPortID)
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
	if err := f.resourceMgr.UpdateGEMportsPonportToOnuMapOnKVStore(ctx, gemPortIDs, intfID, onuID, uniID); err != nil {
		logger.Error(ctx, "error-while-uploading-gemtopon-map-to-kv-store", log.Fields{"device-id": f.deviceHandler.device.Id})
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
			f.techprofile[intfID] = f.resourceMgr.ResourceMgrs[uint32(intfID)].TechProfileMgr
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

func (f *OpenOltFlowMgr) addUpstreamDataFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32,
	portNo uint32, uplinkClassifier map[string]interface{},
	uplinkAction map[string]interface{}, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemportID uint32, tpID uint32) error {
	uplinkClassifier[PacketTagType] = SingleTag
	logger.Debugw(ctx, "adding-upstream-data-flow",
		log.Fields{
			"uplinkClassifier": uplinkClassifier,
			"uplinkAction":     uplinkAction})
	return f.addHSIAFlow(ctx, intfID, onuID, uniID, portNo, uplinkClassifier, uplinkAction,
		Upstream, logicalFlow, allocID, gemportID, tpID)
	/* TODO: Install Secondary EAP on the subscriber vlan */
}

func (f *OpenOltFlowMgr) addDownstreamDataFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32,
	portNo uint32, downlinkClassifier map[string]interface{},
	downlinkAction map[string]interface{}, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemportID uint32, tpID uint32) error {
	downlinkClassifier[PacketTagType] = DoubleTag
	logger.Debugw(ctx, "adding-downstream-data-flow",
		log.Fields{
			"downlinkClassifier": downlinkClassifier,
			"downlinkAction":     downlinkAction})
	// Ignore Downlink trap flow given by core, cannot do anything with this flow */
	if vlan, exists := downlinkClassifier[VlanVid]; exists {
		if vlan.(uint32) == (uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 4000) { //private VLAN given by core
			if metadata, exists := downlinkClassifier[Metadata]; exists { // inport is filled in metadata by core
				if uint32(metadata.(uint64)) == MkUniPortNum(ctx, intfID, onuID, uniID) {
					logger.Infow(ctx, "ignoring-dl-trap-device-flow-from-core",
						log.Fields{
							"flow":      logicalFlow,
							"device-id": f.deviceHandler.device.Id,
							"onu-id":    onuID,
							"intf-id":   intfID})
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

	return f.addHSIAFlow(ctx, intfID, onuID, uniID, portNo, downlinkClassifier, downlinkAction,
		Downstream, logicalFlow, allocID, gemportID, tpID)
}

func (f *OpenOltFlowMgr) addHSIAFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifier map[string]interface{},
	action map[string]interface{}, direction string, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemPortID uint32, tpID uint32) error {
	/* One of the OLT platform (Broadcom BAL) requires that symmetric
	   flows require the same flow_id to be used across UL and DL.
	   Since HSIA flow is the only symmetric flow currently, we need to
	   re-use the flow_id across both direction. The 'flow_category'
	   takes priority over flow_cookie to find any available HSIA_FLOW
	   id for the ONU.
	*/
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
	var vlanPbit uint32 = 0xff // means no pbit
	var vlanVid uint32
	if _, ok := classifier[VlanPcp]; ok {
		vlanPbit = classifier[VlanPcp].(uint32)
		logger.Debugw(ctx, "found-pbit-in-flow",
			log.Fields{
				"vlan-pbit": vlanPbit,
				"intf-id":   intfID,
				"onu-id":    onuID,
				"device-id": f.deviceHandler.device.Id})
	} else {
		logger.Debugw(ctx, "pbit-not-found-in-flow",
			log.Fields{
				"vlan-pcp":  VlanPcp,
				"intf-id":   intfID,
				"onu-id":    onuID,
				"device-id": f.deviceHandler.device.Id})
	}
	if _, ok := classifier[VlanVid]; ok {
		vlanVid = classifier[VlanVid].(uint32)
		log.Debugw("found-vlan-in-the-flow",
			log.Fields{
				"vlan-vid":  vlanVid,
				"intf-id":   intfID,
				"onu-id":    onuID,
				"device-id": f.deviceHandler.device.Id})
	}
	flowStoreCookie := getFlowStoreCookie(ctx, classifier, gemPortID)
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(intfID), int32(onuID), int32(uniID), flowStoreCookie); present {
		logger.Infow(ctx, "flow-already-exists",
			log.Fields{
				"device-id": f.deviceHandler.device.Id,
				"intf-id":   intfID,
				"onu-id":    onuID})
		return nil
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, intfID, int32(onuID), int32(uniID), gemPortID, flowStoreCookie, HsiaFlow, vlanVid, vlanPbit)
	if err != nil {
		return olterrors.NewErrNotFound("hsia-flow-id",
			log.Fields{
				"direction": direction,
				"device-id": f.deviceHandler.device.Id,
				"intf-id":   intfID,
				"onu-id":    onuID,
			}, err).Log()
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
		FlowId:        flowID,
		FlowType:      direction,
		AllocId:       int32(allocID),
		NetworkIntfId: int32(networkIntfID),
		GemportId:     int32(gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo,
		TechProfileId: tpID,
	}
	if err := f.addFlowToDevice(ctx, logicalFlow, &flow); err != nil {
		return olterrors.NewErrFlowOp("add", flowID, nil, err).Log()
	}
	logger.Infow(ctx, "hsia-flow-added-to-device-successfully",
		log.Fields{"direction": direction,
			"device-id": f.deviceHandler.device.Id,
			"flow":      flow,
			"intf-id":   intfID,
			"onu-id":    onuID})
	flowsToKVStore := f.getUpdatedFlowInfo(ctx, &flow, flowStoreCookie, HsiaFlow, flowID, logicalFlow.Id)
	if err := f.updateFlowInfoToKVStore(ctx, flow.AccessIntfId,
		flow.OnuId,
		flow.UniId,
		flow.FlowId /*flowCategory,*/, flowsToKVStore); err != nil {
		return olterrors.NewErrPersistence("update", "flow", flowID,
			log.Fields{
				"flow":      flow,
				"device-id": f.deviceHandler.device.Id,
				"intf-id":   intfID,
				"onu-id":    onuID}, err).Log()
	}
	return nil
}

func (f *OpenOltFlowMgr) addDHCPTrapFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32,
	classifier map[string]interface{}, action map[string]interface{}, logicalFlow *ofp.OfpFlowStats, allocID uint32,
	gemPortID uint32, tpID uint32) error {

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

	flowStoreCookie := getFlowStoreCookie(ctx, classifier, gemPortID)
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(intfID), int32(onuID), int32(uniID), flowStoreCookie); present {
		logger.Infow(ctx, "flow-exists--not-re-adding",
			log.Fields{
				"device-id": f.deviceHandler.device.Id,
				"intf-id":   intfID,
				"onu-id":    onuID})
		return nil
	}

	flowID, err := f.resourceMgr.GetFlowID(ctx, intfID, int32(onuID), int32(uniID), gemPortID, flowStoreCookie, DhcpFlow, 0 /*classifier[VLAN_PCP].(uint32)*/)

	if err != nil {
		return olterrors.NewErrNotFound("flow",
			log.Fields{
				"interface-id": intfID,
				"gem-port":     gemPortID,
				"cookie":       flowStoreCookie,
				"device-id":    f.deviceHandler.device.Id},
			err).Log()
	}

	logger.Debugw(ctx, "creating-ul-dhcp-flow",
		log.Fields{
			"ul_classifier": classifier,
			"ul_action":     action,
			"uplinkFlowId":  flowID,
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
		FlowId:        flowID,
		FlowType:      Upstream,
		AllocId:       int32(allocID),
		NetworkIntfId: int32(networkIntfID),
		GemportId:     int32(gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo,
		TechProfileId: tpID,
	}
	if err := f.addFlowToDevice(ctx, logicalFlow, &dhcpFlow); err != nil {
		return olterrors.NewErrFlowOp("add", flowID, log.Fields{"dhcp-flow": dhcpFlow}, err).Log()
	}
	logger.Infow(ctx, "dhcp-ul-flow-added-to-device-successfully",
		log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"flow-id":   flowID,
			"intf-id":   intfID,
			"onu-id":    onuID})
	flowsToKVStore := f.getUpdatedFlowInfo(ctx, &dhcpFlow, flowStoreCookie, "DHCP", flowID, logicalFlow.Id)
	if err := f.updateFlowInfoToKVStore(ctx, dhcpFlow.AccessIntfId,
		dhcpFlow.OnuId,
		dhcpFlow.UniId,
		dhcpFlow.FlowId, flowsToKVStore); err != nil {
		return olterrors.NewErrPersistence("update", "flow", dhcpFlow.FlowId,
			log.Fields{
				"flow":      dhcpFlow,
				"device-id": f.deviceHandler.device.Id}, err).Log()
	}

	return nil
}

//addIGMPTrapFlow creates IGMP trap-to-host flow
func (f *OpenOltFlowMgr) addIGMPTrapFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifier map[string]interface{},
	action map[string]interface{}, logicalFlow *ofp.OfpFlowStats, allocID uint32, gemPortID uint32, tpID uint32) error {
	return f.addUpstreamTrapFlow(ctx, intfID, onuID, uniID, portNo, classifier, action, logicalFlow, allocID, gemPortID, IgmpFlow, tpID)
}

//addUpstreamTrapFlow creates a trap-to-host flow
func (f *OpenOltFlowMgr) addUpstreamTrapFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifier map[string]interface{},
	action map[string]interface{}, logicalFlow *ofp.OfpFlowStats, allocID uint32, gemPortID uint32, flowType string, tpID uint32) error {

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
	delete(classifier, VlanVid)

	flowStoreCookie := getFlowStoreCookie(ctx, classifier, gemPortID)
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkIntfID), int32(onuID), int32(uniID), flowStoreCookie); present {
		logger.Infow(ctx, "flow-exists-not-re-adding", log.Fields{"device-id": f.deviceHandler.device.Id})
		return nil
	}

	flowID, err := f.resourceMgr.GetFlowID(ctx, intfID, int32(onuID), int32(uniID), gemPortID, flowStoreCookie, flowType, 0, 0 /*classifier[VLAN_PCP].(uint32)*/)

	if err != nil {
		return olterrors.NewErrNotFound("flow-id",
			log.Fields{
				"intf-id":   intfID,
				"oni-id":    onuID,
				"cookie":    flowStoreCookie,
				"flow-type": flowType,
				"device-id": f.deviceHandler.device.Id,
				"onu-id":    onuID},
			err).Log()
	}

	logger.Debugw(ctx, "creating-upstream-trap-flow",
		log.Fields{
			"ul_classifier": classifier,
			"ul_action":     action,
			"uplinkFlowId":  flowID,
			"flowType":      flowType,
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
		FlowId:        flowID,
		FlowType:      Upstream,
		AllocId:       int32(allocID),
		NetworkIntfId: int32(networkIntfID),
		GemportId:     int32(gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo,
		TechProfileId: tpID,
	}

	if err := f.addFlowToDevice(ctx, logicalFlow, &flow); err != nil {
		return olterrors.NewErrFlowOp("add", flowID, log.Fields{"flow": flow, "device-id": f.deviceHandler.device.Id}, err).Log()
	}
	logger.Infof(ctx, "%s ul-flow-added-to-device-successfully", flowType)

	flowsToKVStore := f.getUpdatedFlowInfo(ctx, &flow, flowStoreCookie, flowType, flowID, logicalFlow.Id)
	if err := f.updateFlowInfoToKVStore(ctx, flow.AccessIntfId,
		flow.OnuId,
		flow.UniId,
		flow.FlowId, flowsToKVStore); err != nil {
		return olterrors.NewErrPersistence("update", "flow", flow.FlowId, log.Fields{"flow": flow, "device-id": f.deviceHandler.device.Id}, err).Log()
	}

	return nil
}

// Add EAPOL flow to  device with mac, vlanId as classifier for upstream and downstream
func (f *OpenOltFlowMgr) addEAPOLFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32,
	classifier map[string]interface{}, action map[string]interface{}, logicalFlow *ofp.OfpFlowStats, allocID uint32,
	gemPortID uint32, vlanID uint32, tpID uint32) error {
	logger.Infow(ctx, "adding-eapol-to-device",
		log.Fields{
			"intf-id":    intfID,
			"onu-id":     onuID,
			"port-no":    portNo,
			"alloc-id":   allocID,
			"gemport-id": gemPortID,
			"vlan-id":    vlanID,
			"flow":       logicalFlow})

	uplinkClassifier := make(map[string]interface{})
	uplinkAction := make(map[string]interface{})

	// Fill Classfier
	uplinkClassifier[EthType] = uint32(EapEthType)
	uplinkClassifier[PacketTagType] = SingleTag
	uplinkClassifier[VlanVid] = vlanID
	uplinkClassifier[VlanPcp] = classifier[VlanPcp]
	// Fill action
	uplinkAction[TrapToHost] = true
	flowStoreCookie := getFlowStoreCookie(ctx, uplinkClassifier, gemPortID)
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(intfID), int32(onuID), int32(uniID), flowStoreCookie); present {
		logger.Infow(ctx, "flow-exists-not-re-adding", log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"onu-id":    onuID,
			"intf-id":   intfID})
		return nil
	}
	//Add Uplink EAPOL Flow
	uplinkFlowID, err := f.resourceMgr.GetFlowID(ctx, intfID, int32(onuID), int32(uniID), gemPortID, flowStoreCookie, "", 0, 0)
	if err != nil {
		return olterrors.NewErrNotFound("flow-id",
			log.Fields{
				"intf-id":   intfID,
				"onu-id":    onuID,
				"coookie":   flowStoreCookie,
				"device-id": f.deviceHandler.device.Id},
			err).Log()
	}
	logger.Debugw(ctx, "creating-ul-eapol-flow",
		log.Fields{
			"ul_classifier": uplinkClassifier,
			"ul_action":     uplinkAction,
			"uplinkFlowId":  uplinkFlowID,
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
		FlowId:        uplinkFlowID,
		FlowType:      Upstream,
		AllocId:       int32(allocID),
		NetworkIntfId: int32(networkIntfID),
		GemportId:     int32(gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo,
		TechProfileId: tpID,
	}
	if err := f.addFlowToDevice(ctx, logicalFlow, &upstreamFlow); err != nil {
		return olterrors.NewErrFlowOp("add", uplinkFlowID, log.Fields{"flow": upstreamFlow}, err).Log()
	}
	logger.Infow(ctx, "eapol-ul-flow-added-to-device-successfully",
		log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"onu-id":    onuID,
			"intf-id":   intfID,
		})
	flowCategory := "EAPOL"
	flowsToKVStore := f.getUpdatedFlowInfo(ctx, &upstreamFlow, flowStoreCookie, flowCategory, uplinkFlowID, logicalFlow.Id)
	if err := f.updateFlowInfoToKVStore(ctx, upstreamFlow.AccessIntfId,
		upstreamFlow.OnuId,
		upstreamFlow.UniId,
		upstreamFlow.FlowId,
		/* lowCategory, */
		flowsToKVStore); err != nil {
		return olterrors.NewErrPersistence("update", "flow", upstreamFlow.FlowId,
			log.Fields{
				"flow":      upstreamFlow,
				"device-id": f.deviceHandler.device.Id}, err).Log()
	}
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
func (f *OpenOltFlowMgr) DeleteTechProfileInstances(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, sn string) error {
	tpIDList := f.resourceMgr.GetTechProfileIDForOnu(ctx, intfID, onuID, uniID)
	uniPortName := getUniPortPath(f.deviceHandler.device.Id, intfID, int32(onuID), int32(uniID))

	for _, tpID := range tpIDList {
		if err := f.DeleteTechProfileInstance(ctx, intfID, onuID, uniID, uniPortName, tpID); err != nil {
			_ = olterrors.NewErrAdapter("delete-tech-profile-failed", log.Fields{"device-id": f.deviceHandler.device.Id}, err).Log()
			// return err
			// We should continue to delete tech-profile instances for other TP IDs
		}
		log.Debugw("tech-profile-deleted", log.Fields{"device-id": f.deviceHandler.device.Id, "tp-id": tpID})
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

func getFlowStoreCookie(ctx context.Context, classifier map[string]interface{}, gemPortID uint32) uint64 {
	if len(classifier) == 0 { // should never happen
		logger.Error(ctx, "invalid-classfier-object")
		return 0
	}
	logger.Debugw(ctx, "generating-flow-store-cookie",
		log.Fields{
			"classifier": classifier,
			"gemport-id": gemPortID})
	var jsonData []byte
	var flowString string
	var err error
	// TODO: Do we need to marshall ??
	if jsonData, err = json.Marshal(classifier); err != nil {
		logger.Error(ctx, "failed-to-encode-classifier")
		return 0
	}
	flowString = string(jsonData)
	if gemPortID != 0 {
		flowString = fmt.Sprintf("%s%s", string(jsonData), string(gemPortID))
	}
	h := md5.New()
	_, _ = h.Write([]byte(flowString))
	hash := big.NewInt(0)
	hash.SetBytes(h.Sum(nil))
	generatedHash := hash.Uint64()
	logger.Debugw(ctx, "hash-generated", log.Fields{"hash": generatedHash})
	return generatedHash
}

func (f *OpenOltFlowMgr) getUpdatedFlowInfo(ctx context.Context, flow *openoltpb2.Flow, flowStoreCookie uint64, flowCategory string, deviceFlowID uint32, logicalFlowID uint64) *[]rsrcMgr.FlowInfo {
	var flows = []rsrcMgr.FlowInfo{{Flow: flow, FlowCategory: flowCategory, FlowStoreCookie: flowStoreCookie, LogicalFlowID: logicalFlowID}}
	var intfID uint32
	/* For flows which trap out of the NNI, the AccessIntfId is invalid
	   (set to -1). In such cases, we need to refer to the NetworkIntfId .
	*/
	if flow.AccessIntfId != -1 {
		intfID = uint32(flow.AccessIntfId)
	} else {
		intfID = uint32(flow.NetworkIntfId)
	}
	// Get existing flows matching flowid for given subscriber from KV store
	existingFlows := f.resourceMgr.GetFlowIDInfo(ctx, intfID, flow.OnuId, flow.UniId, flow.FlowId)
	if existingFlows != nil {
		logger.Debugw(ctx, "flow-exists-for-given-flowID--appending-it-to-current-flow",
			log.Fields{
				"flow-id":   flow.FlowId,
				"device-id": f.deviceHandler.device.Id,
				"intf-id":   intfID,
				"onu-id":    flow.OnuId})
		//for _, f := range *existingFlows {
		//	flows = append(flows, f)
		//}
		flows = append(flows, *existingFlows...)
	}
	logger.Debugw(ctx, "updated-flows-for-given-flowID-and-onuid",
		log.Fields{
			"updatedflow": flows,
			"flow-id":     flow.FlowId,
			"onu-id":      flow.OnuId,
			"device-id":   f.deviceHandler.device.Id})
	return &flows
}

func (f *OpenOltFlowMgr) updateFlowInfoToKVStore(ctx context.Context, intfID int32, onuID int32, uniID int32, flowID uint32, flows *[]rsrcMgr.FlowInfo) error {
	logger.Debugw(ctx, "storing-flow(s)-into-kv-store", log.Fields{
		"flow-id":   flowID,
		"device-id": f.deviceHandler.device.Id,
		"intf-id":   intfID,
		"onu-id":    onuID})
	if err := f.resourceMgr.UpdateFlowIDInfo(ctx, intfID, onuID, uniID, flowID, flows); err != nil {
		logger.Warnw(ctx, "error-while-storing-flow-into-kv-store", log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"onu-id":    onuID,
			"intf-id":   intfID,
			"flow-id":   flowID})
		return err
	}
	logger.Infow(ctx, "stored-flow(s)-into-kv-store-successfully!", log.Fields{
		"device-id": f.deviceHandler.device.Id,
		"onu-id":    onuID,
		"intf-id":   intfID,
		"flow-id":   flowID})
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
		// REVIST : Why ponport is given as network port?
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
		f.resourceMgr.FreeFlowID(ctx, intfID, deviceFlow.OnuId, deviceFlow.UniId, deviceFlow.FlowId)
		return err
	}
	if deviceFlow.GemportId != -1 {
		// No need to register the flow if it is a trap on nni flow.
		if err := f.registerFlow(ctx, logicalFlow, deviceFlow); err != nil {
			logger.Errorw(ctx, "failed-to-register-flow", log.Fields{"err": err})
			return err
		}
	}
	logger.Infow(ctx, "flow-added-to-device-successfully ",
		log.Fields{
			"flow":      *deviceFlow,
			"device-id": f.deviceHandler.device.Id,
			"intf-id":   intfID})
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
	var flowStoreCookie = getFlowStoreCookie(ctx, classifierInfo, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), flowStoreCookie); present {
		logger.Infow(ctx, "flow-exists--not-re-adding", log.Fields{"device-id": f.deviceHandler.device.Id})
		return nil
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), uint32(gemPortID), flowStoreCookie, "", 0)

	if err != nil {
		return olterrors.NewErrNotFound("flow-id",
			log.Fields{
				"interface-id": networkInterfaceID,
				"onu-id":       onuID,
				"uni-id":       uniID,
				"gem-port-id":  gemPortID,
				"cookie":       flowStoreCookie,
				"device-id":    f.deviceHandler.device.Id},
			err)
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
		FlowId:        flowID,
		FlowType:      Downstream,
		NetworkIntfId: int32(networkInterfaceID),
		GemportId:     int32(gemPortID),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(flow.Priority),
		Cookie:        flow.Cookie,
		PortNo:        portNo}
	if err := f.addFlowToDevice(ctx, flow, &downstreamflow); err != nil {
		return olterrors.NewErrFlowOp("add", flowID,
			log.Fields{
				"flow":      downstreamflow,
				"device-id": f.deviceHandler.device.Id}, err)
	}
	logger.Infow(ctx, "lldp-trap-on-nni-flow-added-to-device-successfully",
		log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"onu-id":    onuID,
			"flow-id":   flowID})
	flowsToKVStore := f.getUpdatedFlowInfo(ctx, &downstreamflow, flowStoreCookie, "", flowID, flow.Id)
	if err := f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID),
		int32(onuID),
		int32(uniID),
		flowID, flowsToKVStore); err != nil {
		return olterrors.NewErrPersistence("update", "flow", flowID,
			log.Fields{
				"flow":      downstreamflow,
				"device-id": f.deviceHandler.device.Id}, err)
	}
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
		f.deviceHandler.device.Type,
		onuDev.deviceType,
		onuDev.deviceID,
		onuDev.proxyDeviceID, ""); sendErr != nil {
		return olterrors.NewErrCommunication("send-delete-gem-port-to-onu-adapter",
			log.Fields{
				"from-adapter":  f.deviceHandler.device.Type,
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
		f.deviceHandler.device.Type,
		onuDev.deviceType,
		onuDev.deviceID,
		onuDev.proxyDeviceID, ""); sendErr != nil {
		return olterrors.NewErrCommunication("send-delete-tcont-to-onu-adapter",
			log.Fields{
				"from-adapter": f.deviceHandler.device.Type,
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

func (f *OpenOltFlowMgr) deletePendingFlows(ctx context.Context, Intf uint32, onuID int32, uniID int32) {
	pnFlDelKey := pendingFlowDeleteKey{Intf, uint32(onuID), uint32(uniID)}
	if val, ok := f.pendingFlowDelete.Load(pnFlDelKey); ok {
		if val.(int) > 0 {
			pnFlDels := val.(int) - 1
			if pnFlDels > 0 {
				logger.Debugw(ctx, "flow-delete-succeeded--more-pending",
					log.Fields{
						"intf":               Intf,
						"onu-id":             onuID,
						"uni-id":             uniID,
						"currpendingflowcnt": pnFlDels,
						"device-id":          f.deviceHandler.device.Id})
				f.pendingFlowDelete.Store(pnFlDelKey, pnFlDels)
			} else {
				logger.Debugw(ctx, "all-pending-flow-deletes-handled--removing-entry-from-map",
					log.Fields{
						"intf":      Intf,
						"onu-id":    onuID,
						"uni-id":    uniID,
						"device-id": f.deviceHandler.device.Id})
				f.pendingFlowDelete.Delete(pnFlDelKey)
			}
		}
	} else {
		logger.Debugw(ctx, "no-pending-delete-flows-found",
			log.Fields{
				"intf":      Intf,
				"onu-id":    onuID,
				"uni-id":    uniID,
				"device-id": f.deviceHandler.device.Id})

	}

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
			"onu-gem":     f.onuGemInfo[intfID]})
	onugem := f.onuGemInfo[intfID]
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
func (f *OpenOltFlowMgr) clearResources(ctx context.Context, flow *ofp.OfpFlowStats, Intf uint32, onuID int32, uniID int32,
	gemPortID int32, flowID uint32, flowDirection string,
	portNum uint32, updatedFlows []rsrcMgr.FlowInfo) error {

	tpID, err := getTpIDFromFlow(ctx, flow)
	if err != nil {
		return olterrors.NewErrNotFound("tp-id",
			log.Fields{
				"flow":      flow,
				"intf":      Intf,
				"onu-id":    onuID,
				"uni-id":    uniID,
				"device-id": f.deviceHandler.device.Id}, err)
	}

	if len(updatedFlows) >= 0 {
		// There are still flows referencing the same flow_id.
		// So the flow should not be freed yet.
		// For ex: Case of HSIA where same flow is shared
		// between DS and US.
		if err := f.updateFlowInfoToKVStore(ctx, int32(Intf), int32(onuID), int32(uniID), flowID, &updatedFlows); err != nil {
			_ = olterrors.NewErrPersistence("update", "flow", flowID,
				log.Fields{
					"flow":      updatedFlows,
					"device-id": f.deviceHandler.device.Id}, err).Log()
		}
		if len(updatedFlows) == 0 {
			// Do this for subscriber flows only (not trap from NNI flows)
			if onuID != -1 && uniID != -1 {
				pnFlDelKey := pendingFlowDeleteKey{Intf, uint32(onuID), uint32(uniID)}
				if val, ok := f.pendingFlowDelete.Load(pnFlDelKey); !ok {
					logger.Debugw(ctx, "creating-entry-for-pending-flow-delete",
						log.Fields{
							"flow-id":   flowID,
							"intf":      Intf,
							"onu-id":    onuID,
							"uni-id":    uniID,
							"device-id": f.deviceHandler.device.Id})
					f.pendingFlowDelete.Store(pnFlDelKey, 1)
				} else {
					pnFlDels := val.(int) + 1
					logger.Debugw(ctx, "updating-flow-delete-entry",
						log.Fields{
							"flow-id":            flowID,
							"intf":               Intf,
							"onu-id":             onuID,
							"uni-id":             uniID,
							"currPendingFlowCnt": pnFlDels,
							"device-id":          f.deviceHandler.device.Id})
					f.pendingFlowDelete.Store(pnFlDelKey, pnFlDels)
				}

				defer f.deletePendingFlows(ctx, Intf, onuID, uniID)
			}

			logger.Debugw(ctx, "releasing-flow-id-to-resource-manager",
				log.Fields{
					"Intf":      Intf,
					"onu-id":    onuID,
					"uni-id":    uniID,
					"flow-id":   flowID,
					"device-id": f.deviceHandler.device.Id})
			f.resourceMgr.FreeFlowID(ctx, Intf, int32(onuID), int32(uniID), flowID)

			uni := getUniPortPath(f.deviceHandler.device.Id, Intf, onuID, uniID)
			tpPath := f.getTPpath(ctx, Intf, uni, tpID)
			logger.Debugw(ctx, "getting-techprofile-instance-for-subscriber",
				log.Fields{
					"TP-PATH":   tpPath,
					"device-id": f.deviceHandler.device.Id})
			techprofileInst, err := f.techprofile[Intf].GetTPInstanceFromKVStore(ctx, tpID, tpPath)
			if err != nil || techprofileInst == nil { // This should not happen, something wrong in KV backend transaction
				return olterrors.NewErrNotFound("tech-profile-in-kv-store",
					log.Fields{
						"tp-id": tpID,
						"path":  tpPath}, err)
			}

			gemPK := gemPortKey{Intf, uint32(gemPortID)}
			used, err := f.isGemPortUsedByAnotherFlow(ctx, gemPK)
			if err != nil {
				return err
			}
			if used {
				if f.perGemPortLock.TryLock(gemPK) {
					flowIDs := f.flowsUsedByGemPort[gemPK]
					for i, flowIDinMap := range flowIDs {
						if flowIDinMap == flowID {
							flowIDs = append(flowIDs[:i], flowIDs[i+1:]...)
							// everytime flowsUsedByGemPort cache is updated the same should be updated
							// in kv store by calling UpdateFlowIDsForGem
							f.flowsUsedByGemPort[gemPK] = flowIDs
							if err := f.resourceMgr.UpdateFlowIDsForGem(ctx, Intf, uint32(gemPortID), flowIDs); err != nil {
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
					f.perGemPortLock.Unlock(gemPK)
					return nil
				}

				logger.Error(ctx, "failed-to-acquire-per-gem-port-lock",
					log.Fields{
						"gemport-id": gemPortID,
						"device-id":  f.deviceHandler.device.Id,
						"key":        gemPK,
					})
				return olterrors.NewErrAdapter("failed-to-acquire-per-gem-port-lock", log.Fields{
					"gemport-id": gemPortID,
					"device-id":  f.deviceHandler.device.Id,
					"key":        gemPK,
				}, nil)
			}
			logger.Debugf(ctx, "gem-port-id %d is-not-used-by-another-flow--releasing-the-gem-port", gemPortID)
			f.resourceMgr.RemoveGemPortIDForOnu(ctx, Intf, uint32(onuID), uint32(uniID), uint32(gemPortID))
			// TODO: The TrafficQueue corresponding to this gem-port also should be removed immediately.
			// But it is anyway eventually  removed later when the TechProfile is freed, so not a big issue for now.
			f.resourceMgr.RemoveGEMportPonportToOnuMapOnKVStore(ctx, uint32(gemPortID), Intf)
			f.deleteGemPortFromLocalCache(ctx, Intf, uint32(onuID), uint32(gemPortID))
			f.onuIdsLock.Lock()
			//everytime an entry is deleted from flowsUsedByGemPort cache, the same should be updated in kv as well
			// by calling DeleteFlowIDsForGem
			if f.perGemPortLock.TryLock(gemPK) {
				delete(f.flowsUsedByGemPort, gemPK)
				f.perGemPortLock.Unlock(gemPK)
			} else {
				logger.Error(ctx, "failed-to-acquire-per-gem-port-lock",
					log.Fields{
						"device-id": f.deviceHandler.device.Id,
						"key":       gemPK,
					})
			}
			f.resourceMgr.DeleteFlowIDsForGem(ctx, Intf, uint32(gemPortID))
			f.resourceMgr.FreeGemPortID(ctx, Intf, uint32(onuID), uint32(uniID), uint32(gemPortID))
			f.onuIdsLock.Unlock()
			// Delete the gem port on the ONU.
			if err := f.sendDeleteGemPortToChild(ctx, Intf, uint32(onuID), uint32(uniID), uint32(gemPortID), tpPath); err != nil {
				logger.Errorw(ctx, "error-processing-delete-gem-port-towards-onu",
					log.Fields{
						"err":        err,
						"intf":       Intf,
						"onu-id":     onuID,
						"uni-id":     uniID,
						"device-id":  f.deviceHandler.device.Id,
						"gemport-id": gemPortID})
			}
			switch techprofileInst := techprofileInst.(type) {
			case *tp.TechProfile:
				ok, _ := f.isTechProfileUsedByAnotherGem(ctx, Intf, uint32(onuID), uint32(uniID), tpID, techprofileInst, uint32(gemPortID))
				if !ok {
					if err := f.resourceMgr.RemoveTechProfileIDForOnu(ctx, Intf, uint32(onuID), uint32(uniID), tpID); err != nil {
						logger.Warn(ctx, err)
					}
					if err := f.DeleteTechProfileInstance(ctx, Intf, uint32(onuID), uint32(uniID), "", tpID); err != nil {
						logger.Warn(ctx, err)
					}
					if err := f.RemoveSchedulerQueues(ctx, schedQueue{direction: tp_pb.Direction_UPSTREAM, intfID: Intf, onuID: uint32(onuID), uniID: uint32(uniID), tpID: tpID, uniPort: portNum, tpInst: techprofileInst}); err != nil {
						logger.Warn(ctx, err)
					}
					if err := f.RemoveSchedulerQueues(ctx, schedQueue{direction: tp_pb.Direction_DOWNSTREAM, intfID: Intf, onuID: uint32(onuID), uniID: uint32(uniID), tpID: tpID, uniPort: portNum, tpInst: techprofileInst}); err != nil {
						logger.Warn(ctx, err)
					}
					f.resourceMgr.FreeAllocID(ctx, Intf, uint32(onuID), uint32(uniID), techprofileInst.UsScheduler.AllocID)
					// Delete the TCONT on the ONU.
					if err := f.sendDeleteTcontToChild(ctx, Intf, uint32(onuID), uint32(uniID), uint32(techprofileInst.UsScheduler.AllocID), tpPath); err != nil {
						logger.Errorw(ctx, "error-processing-delete-tcont-towards-onu",
							log.Fields{
								"intf":      Intf,
								"onu-id":    onuID,
								"uni-id":    uniID,
								"device-id": f.deviceHandler.device.Id,
								"alloc-id":  techprofileInst.UsScheduler.AllocID})
					}
				}
			case *tp.EponProfile:
				if err := f.resourceMgr.RemoveTechProfileIDForOnu(ctx, Intf, uint32(onuID), uint32(uniID), tpID); err != nil {
					logger.Warn(ctx, err)
				}
				if err := f.DeleteTechProfileInstance(ctx, Intf, uint32(onuID), uint32(uniID), "", tpID); err != nil {
					logger.Warn(ctx, err)
				}
				f.resourceMgr.FreeAllocID(ctx, Intf, uint32(onuID), uint32(uniID), techprofileInst.AllocID)
				// Delete the TCONT on the ONU.
				if err := f.sendDeleteTcontToChild(ctx, Intf, uint32(onuID), uint32(uniID), uint32(techprofileInst.AllocID), tpPath); err != nil {
					logger.Errorw(ctx, "error-processing-delete-tcont-towards-onu",
						log.Fields{
							"intf":      Intf,
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
		}
	}
	return nil
}

// nolint: gocyclo
func (f *OpenOltFlowMgr) clearFlowFromResourceManager(ctx context.Context, flow *ofp.OfpFlowStats, flowDirection string) {

	logger.Infow(ctx, "clear-flow-from-resource-manager",
		log.Fields{
			"flowDirection": flowDirection,
			"flow":          *flow,
			"device-id":     f.deviceHandler.device.Id})

	if flowDirection == Multicast {
		f.clearMulticastFlowFromResourceManager(ctx, flow)
		return
	}

	classifierInfo := make(map[string]interface{})

	portNum, Intf, onu, uni, inPort, ethType, err := FlowExtractInfo(ctx, flow, flowDirection)
	if err != nil {
		logger.Error(ctx, err)
		return
	}

	onuID := int32(onu)
	uniID := int32(uni)

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
			return
		}
	}
	flowIds := f.resourceMgr.GetCurrentFlowIDsForOnu(ctx, Intf, onuID, uniID)
	for _, flowID := range flowIds {
		flowInfo := f.resourceMgr.GetFlowIDInfo(ctx, Intf, onuID, uniID, flowID)
		if flowInfo == nil {
			logger.Debugw(ctx, "no-flowinfo-found-in-kv-store",
				log.Fields{
					"intf":    Intf,
					"onu-id":  onuID,
					"uni-id":  uniID,
					"flow-id": flowID})
			return
		}

		updatedFlows := *flowInfo
		for i, storedFlow := range updatedFlows {
			if flow.Id == storedFlow.LogicalFlowID {
				removeFlowMessage := openoltpb2.Flow{FlowId: storedFlow.Flow.FlowId, FlowType: storedFlow.Flow.FlowType}
				logger.Debugw(ctx, "flow-to-be-deleted", log.Fields{"flow": storedFlow})
				// DKB
				if err = f.removeFlowFromDevice(ctx, &removeFlowMessage, flow.Id); err != nil {
					logger.Errorw(ctx, "failed-to-remove-flow", log.Fields{"error": err})
					return
				}
				logger.Info(ctx, "flow-removed-from-device-successfully", log.Fields{
					"flow-id":        flow.Id,
					"stored-flow":    storedFlow,
					"device-id":      f.deviceHandler.device.Id,
					"stored-flow-id": flowID,
					"onu-id":         onuID,
					"intf":           Intf,
				})
				//Remove the Flow from FlowInfo
				updatedFlows = append(updatedFlows[:i], updatedFlows[i+1:]...)
				if err = f.clearResources(ctx, flow, Intf, onuID, uniID, storedFlow.Flow.GemportId,
					flowID, flowDirection, portNum, updatedFlows); err != nil {
					logger.Error(ctx, "failed-to-clear-resources-for-flow", log.Fields{
						"flow-id":        flow.Id,
						"stored-flow":    storedFlow,
						"device-id":      f.deviceHandler.device.Id,
						"stored-flow-id": flowID,
						"onu-id":         onuID,
						"intf":           Intf,
					})
					return
				}
			}
		}
	}
}

//clearMulticastFlowFromResourceManager  removes a multicast flow from the KV store and
// clears resources reserved for this multicast flow
func (f *OpenOltFlowMgr) clearMulticastFlowFromResourceManager(ctx context.Context, flow *ofp.OfpFlowStats) {
	classifierInfo := make(map[string]interface{})
	formulateClassifierInfoFromFlow(ctx, classifierInfo, flow)
	networkInterfaceID, err := f.getNNIInterfaceIDOfMulticastFlow(ctx, classifierInfo)

	if err != nil {
		logger.Warnw(ctx, "no-inport-found--cannot-release-resources-of-the-multicast-flow", log.Fields{"flowId:": flow.Id})
		return
	}

	var onuID = int32(NoneOnuID)
	var uniID = int32(NoneUniID)
	var flowID uint32

	flowIds := f.resourceMgr.GetCurrentFlowIDsForOnu(ctx, networkInterfaceID, onuID, uniID)

	for _, flowID = range flowIds {
		flowInfo := f.resourceMgr.GetFlowIDInfo(ctx, networkInterfaceID, onuID, uniID, flowID)
		if flowInfo == nil {
			logger.Debugw(ctx, "no-multicast-flowinfo-found-in-the-kv-store",
				log.Fields{
					"intf":    networkInterfaceID,
					"onu-id":  onuID,
					"uni-id":  uniID,
					"flow-id": flowID})
			continue
		}
		updatedFlows := *flowInfo
		for i, storedFlow := range updatedFlows {
			if flow.Id == storedFlow.LogicalFlowID {
				removeFlowMessage := openoltpb2.Flow{FlowId: storedFlow.Flow.FlowId, FlowType: storedFlow.Flow.FlowType}
				logger.Debugw(ctx, "multicast-flow-to-be-deleted",
					log.Fields{
						"flow":      storedFlow,
						"flow-id":   flow.Id,
						"device-id": f.deviceHandler.device.Id})
				//remove from device
				if err := f.removeFlowFromDevice(ctx, &removeFlowMessage, flow.Id); err != nil {
					// DKB
					logger.Errorw(ctx, "failed-to-remove-multicast-flow",
						log.Fields{
							"flow-id": flow.Id,
							"error":   err})
					return
				}
				logger.Infow(ctx, "multicast-flow-removed-from-device-successfully", log.Fields{"flow-id": flow.Id})
				//Remove the Flow from FlowInfo
				updatedFlows = append(updatedFlows[:i], updatedFlows[i+1:]...)
				if err := f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID), NoneOnuID, NoneUniID, flowID, &updatedFlows); err != nil {
					logger.Errorw(ctx, "failed-to-delete-multicast-flow-from-the-kv-store",
						log.Fields{"flow": storedFlow,
							"err": err})
					return
				}
				//release flow id
				logger.Debugw(ctx, "releasing-multicast-flow-id",
					log.Fields{"flow-id": flowID,
						"interfaceID": networkInterfaceID})
				f.resourceMgr.FreeFlowID(ctx, uint32(networkInterfaceID), NoneOnuID, NoneUniID, flowID)
			}
		}
	}
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
		f.clearFlowFromResourceManager(ctx, flow, direction)
		return nil
	} else if IsUpstream(actionInfo[Output].(uint32)) {
		direction = Upstream
	} else {
		direction = Downstream
	}

	_, intfID, onuID, uniID, _, _, err := FlowExtractInfo(ctx, flow, direction)
	if err != nil {
		return err
	}

	userKey := tpLockKey{intfID, onuID, uniID}

	// Serialize flow removes on a per subscriber basis
	if f.perUserFlowHandleLock.TryLock(userKey) {
		f.clearFlowFromResourceManager(ctx, flow, direction) //TODO: Take care of the limitations
		f.perUserFlowHandleLock.Unlock(userKey)
	} else {
		// Ideally this should never happen
		logger.Errorw(ctx, "failed-to-acquire-lock-to-remove-flow--remove-aborted", log.Fields{"flow": flow})
		return errors.New("failed-to-acquire-per-user-lock")
	}

	return nil
}

func (f *OpenOltFlowMgr) waitForFlowDeletesToCompleteForOnu(ctx context.Context, intfID uint32, onuID uint32,
	uniID uint32, ch chan bool) {
	pnFlDelKey := pendingFlowDeleteKey{intfID, onuID, uniID}
	for {
		select {
		case <-time.After(20 * time.Millisecond):
			if flowDelRefCnt, ok := f.pendingFlowDelete.Load(pnFlDelKey); !ok || flowDelRefCnt == 0 {
				logger.Debug(ctx, "pending-flow-deletes-completed")
				ch <- true
				return
			}
		case <-ctx.Done():
			logger.Error(ctx, "flow-delete-wait-handler-routine-canceled")
			return
		}
	}
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
	}
	if ipProto, ok := classifierInfo[IPProto]; ok {
		if ipProto.(uint32) == IPProtoDhcp {
			if udpSrc, ok := classifierInfo[UDPSrc]; ok {
				if udpSrc.(uint32) == uint32(67) || udpSrc.(uint32) == uint32(546) {
					logger.Debug(ctx, "trap-dhcp-from-nni-flow")
					return f.addDHCPTrapFlowOnNNI(ctx, flow, classifierInfo, portNo)
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

	pnFlDelKey := pendingFlowDeleteKey{intfID, onuID, uniID}
	if _, ok := f.pendingFlowDelete.Load(pnFlDelKey); !ok {
		logger.Debugw(ctx, "no-pending-flows-found--going-ahead-with-flow-install",
			log.Fields{
				"intf-id": intfID,
				"onu-id":  onuID,
				"uni-id":  uniID})
		f.divideAndAddFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, uint32(TpID), UsMeterID, DsMeterID, flowMetadata)
	} else {
		pendingFlowDelComplete := make(chan bool)
		go f.waitForFlowDeletesToCompleteForOnu(ctx, intfID, onuID, uniID, pendingFlowDelComplete)
		select {
		case <-pendingFlowDelComplete:
			logger.Debugw(ctx, "all-pending-flow-deletes-completed",
				log.Fields{
					"intf-id": intfID,
					"onu-id":  onuID,
					"uni-id":  uniID})
			f.divideAndAddFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, uint32(TpID), UsMeterID, DsMeterID, flowMetadata)

		case <-time.After(10 * time.Second):
			return olterrors.NewErrTimeout("pending-flow-deletes",
				log.Fields{
					"intf-id": intfID,
					"onu-id":  onuID,
					"uni-id":  uniID}, nil)
		}
	}
	return nil
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
	//this variable acts like a switch. When it is set, multicast flows are classified by eth_dst.
	//otherwise, classification is based on ipv4_dst by default.
	//the variable can be configurable in the future; it can be read from a configuration path in the kv store.
	mcastFlowClassificationByEthDst := false

	if mcastFlowClassificationByEthDst {
		//replace ipDst with ethDst
		if ipv4Dst, ok := classifierInfo[Ipv4Dst]; ok &&
			flows.IsMulticastIp(ipv4Dst.(uint32)) {
			// replace ipv4_dst classifier with eth_dst
			multicastMac := flows.ConvertToMulticastMacBytes(ipv4Dst.(uint32))
			delete(classifierInfo, Ipv4Dst)
			classifierInfo[EthDst] = multicastMac
			logger.Debugw(ctx, "multicast-ip-to-mac-conversion-success",
				log.Fields{
					"ip:":  ipv4Dst.(uint32),
					"mac:": multicastMac})
		}
	}
	delete(classifierInfo, EthType)

	onuID := NoneOnuID
	uniID := NoneUniID
	gemPortID := NoneGemPortID

	flowStoreCookie := getFlowStoreCookie(ctx, classifierInfo, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), flowStoreCookie); present {
		logger.Infow(ctx, "multicast-flow-exists-not-re-adding", log.Fields{"classifier-info": classifierInfo})
		return nil
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), uint32(gemPortID), flowStoreCookie, "", 0, 0)
	if err != nil {
		return olterrors.NewErrNotFound("multicast-flow-id",
			log.Fields{
				"interface-id": networkInterfaceID,
				"onu-id":       onuID,
				"uni-id":       uniID,
				"gem-port-id":  gemPortID,
				"cookie":       flowStoreCookie},
			err)
	}
	classifierProto, err := makeOpenOltClassifierField(classifierInfo)
	if err != nil {
		return olterrors.NewErrInvalidValue(log.Fields{"classifier": classifierInfo}, err)
	}
	groupID := actionInfo[GroupID].(uint32)
	multicastFlow := openoltpb2.Flow{
		FlowId:        flowID,
		FlowType:      Multicast,
		NetworkIntfId: int32(networkInterfaceID),
		GroupId:       groupID,
		Classifier:    classifierProto,
		Priority:      int32(flow.Priority),
		Cookie:        flow.Cookie}

	if err := f.addFlowToDevice(ctx, flow, &multicastFlow); err != nil {
		return olterrors.NewErrFlowOp("add", flowID, log.Fields{"flow": multicastFlow}, err)
	}
	logger.Info(ctx, "multicast-flow-added-to-device-successfully")
	//get cached group
	if group, _, err := f.GetFlowGroupFromKVStore(ctx, groupID, true); err == nil {
		//calling groupAdd to set group members after multicast flow creation
		if err := f.ModifyGroup(ctx, group); err != nil {
			return olterrors.NewErrGroupOp("modify", groupID, log.Fields{"group": group}, err)
		}
		//cached group can be removed now
		if err := f.resourceMgr.RemoveFlowGroupFromKVStore(ctx, groupID, true); err != nil {
			logger.Warnw(ctx, "failed-to-remove-flow-group", log.Fields{"group-id": groupID, "error": err})
		}
	}

	flowsToKVStore := f.getUpdatedFlowInfo(ctx, &multicastFlow, flowStoreCookie, MulticastFlow, flowID, flow.Id)
	if err = f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID),
		int32(onuID),
		int32(uniID),
		flowID, flowsToKVStore); err != nil {
		return olterrors.NewErrPersistence("update", "flow", flowID, log.Fields{"flow": multicastFlow}, err)
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
	// find the first NNI interface id of the device
	nniPorts, e := f.resourceMgr.GetNNIFromKVStore(ctx)
	if e == nil && len(nniPorts) > 0 {
		return nniPorts[0], nil
	}
	return 0, olterrors.NewErrNotFound("nni-port", nil, e).Log()
}

// AddGroup add or update the group
func (f *OpenOltFlowMgr) AddGroup(ctx context.Context, group *ofp.OfpGroupEntry) error {
	logger.Infow(ctx, "add-group", log.Fields{"group": group})
	if group == nil {
		return olterrors.NewErrInvalidValue(log.Fields{"group": group}, nil)
	}

	groupToOlt := openoltpb2.Group{
		GroupId: group.Desc.GroupId,
		Command: openoltpb2.Group_SET_MEMBERS,
		Action:  f.buildGroupAction(),
	}

	logger.Debugw(ctx, "sending-group-to-device", log.Fields{"groupToOlt": groupToOlt})
	_, err := f.deviceHandler.Client.PerformGroupOperation(ctx, &groupToOlt)
	if err != nil {
		return olterrors.NewErrAdapter("add-group-operation-failed", log.Fields{"groupToOlt": groupToOlt}, err)
	}
	// group members not created yet. So let's store the group
	if err := f.resourceMgr.AddFlowGroupToKVStore(ctx, group, true); err != nil {
		return olterrors.NewErrPersistence("add", "flow-group", group.Desc.GroupId, log.Fields{"group": group}, err)
	}
	logger.Infow(ctx, "add-group-operation-performed-on-the-device-successfully ", log.Fields{"groupToOlt": groupToOlt})
	return nil
}

// DeleteGroup deletes a group from the device
func (f *OpenOltFlowMgr) DeleteGroup(ctx context.Context, group *ofp.OfpGroupEntry) error {
	logger.Debugw(ctx, "delete-group", log.Fields{"group": group})
	if group == nil {
		logger.Error(ctx, "unable-to-delete-group--invalid-argument--group-is-nil")
		return olterrors.NewErrInvalidValue(log.Fields{"group": group}, nil)
	}

	groupToOlt := openoltpb2.Group{
		GroupId: group.Desc.GroupId,
	}

	logger.Debugw(ctx, "deleting-group-from-device", log.Fields{"groupToOlt": groupToOlt})
	_, err := f.deviceHandler.Client.DeleteGroup(ctx, &groupToOlt)
	if err != nil {
		logger.Errorw(ctx, "delete-group-failed-on-dev", log.Fields{"groupToOlt": groupToOlt, "err": err})
		return olterrors.NewErrAdapter("delete-group-operation-failed", log.Fields{"groupToOlt": groupToOlt}, err)
	}
	//remove group from the store
	if err := f.resourceMgr.RemoveFlowGroupFromKVStore(ctx, group.Desc.GroupId, false); err != nil {
		return olterrors.NewErrPersistence("delete", "flow-group", group.Desc.GroupId, log.Fields{"group": group}, err)
	}
	logger.Debugw(ctx, "delete-group-operation-performed-on-the-device-successfully ", log.Fields{"groupToOlt": groupToOlt})
	return nil
}

//buildGroupAction creates and returns a group action
func (f *OpenOltFlowMgr) buildGroupAction() *openoltpb2.Action {
	var actionCmd openoltpb2.ActionCmd
	var action openoltpb2.Action
	action.Cmd = &actionCmd
	//pop outer vlan
	action.Cmd.RemoveOuterTag = true
	return &action
}

// ModifyGroup updates the group
func (f *OpenOltFlowMgr) ModifyGroup(ctx context.Context, group *ofp.OfpGroupEntry) error {
	logger.Infow(ctx, "modify-group", log.Fields{"group": group})
	if group == nil || group.Desc == nil {
		return olterrors.NewErrInvalidValue(log.Fields{"group": group}, nil)
	}

	newGroup := f.buildGroup(ctx, group.Desc.GroupId, group.Desc.Buckets)
	//get existing members of the group
	val, groupExists, err := f.GetFlowGroupFromKVStore(ctx, group.Desc.GroupId, false)

	if err != nil {
		return olterrors.NewErrNotFound("flow-group-in-kv-store", log.Fields{"groupId": group.Desc.GroupId}, err)
	}

	var current *openoltpb2.Group // represents the group on the device
	if groupExists {
		// group already exists
		current = f.buildGroup(ctx, group.Desc.GroupId, val.Desc.GetBuckets())
		logger.Debugw(ctx, "modify-group--group exists",
			log.Fields{
				"group on the device": val,
				"new":                 group})
	} else {
		current = f.buildGroup(ctx, group.Desc.GroupId, nil)
	}

	logger.Debugw(ctx, "modify-group--comparing-current-and-new",
		log.Fields{
			"group on the device": current,
			"new":                 newGroup})
	// get members to be added
	membersToBeAdded := f.findDiff(current, newGroup)
	// get members to be removed
	membersToBeRemoved := f.findDiff(newGroup, current)

	logger.Infow(ctx, "modify-group--differences found", log.Fields{
		"membersToBeAdded":   membersToBeAdded,
		"membersToBeRemoved": membersToBeRemoved,
		"groupId":            group.Desc.GroupId})

	groupToOlt := openoltpb2.Group{
		GroupId: group.Desc.GroupId,
	}
	var errAdd, errRemoved error
	if len(membersToBeAdded) > 0 {
		groupToOlt.Command = openoltpb2.Group_ADD_MEMBERS
		groupToOlt.Members = membersToBeAdded
		//execute addMembers
		errAdd = f.callGroupAddRemove(ctx, &groupToOlt)
	}
	if len(membersToBeRemoved) > 0 {
		groupToOlt.Command = openoltpb2.Group_REMOVE_MEMBERS
		groupToOlt.Members = membersToBeRemoved
		//execute removeMembers
		errRemoved = f.callGroupAddRemove(ctx, &groupToOlt)
	}

	//save the modified group
	if errAdd == nil && errRemoved == nil {
		if err := f.resourceMgr.AddFlowGroupToKVStore(ctx, group, false); err != nil {
			return olterrors.NewErrPersistence("add", "flow-group", group.Desc.GroupId, log.Fields{"group": group}, err)
		}
		logger.Infow(ctx, "modify-group-was-success--storing-group",
			log.Fields{
				"group":         group,
				"existingGroup": current})
	} else {
		logger.Warnw(ctx, "one-of-the-group-add/remove-operations-failed--cannot-save-group-modifications",
			log.Fields{"group": group})
		if errAdd != nil {
			return errAdd
		}
		return errRemoved
	}
	return nil
}

//callGroupAddRemove performs add/remove buckets operation for the indicated group
func (f *OpenOltFlowMgr) callGroupAddRemove(ctx context.Context, group *openoltpb2.Group) error {
	if err := f.performGroupOperation(ctx, group); err != nil {
		st, _ := status.FromError(err)
		//ignore already exists error code
		if st.Code() != codes.AlreadyExists {
			return olterrors.NewErrGroupOp("groupAddRemove", group.GroupId, log.Fields{"status": st}, err)
		}
	}
	return nil
}

//findDiff compares group members and finds members which only exists in groups2
func (f *OpenOltFlowMgr) findDiff(group1 *openoltpb2.Group, group2 *openoltpb2.Group) []*openoltpb2.GroupMember {
	var members []*openoltpb2.GroupMember
	for _, bucket := range group2.Members {
		if !f.contains(group1.Members, bucket) {
			// bucket does not exist and must be added
			members = append(members, bucket)
		}
	}
	return members
}

//contains returns true if the members list contains the given member; false otherwise
func (f *OpenOltFlowMgr) contains(members []*openoltpb2.GroupMember, member *openoltpb2.GroupMember) bool {
	for _, groupMember := range members {
		if groupMember.InterfaceId == member.InterfaceId {
			return true
		}
	}
	return false
}

//performGroupOperation call performGroupOperation operation of openolt proto
func (f *OpenOltFlowMgr) performGroupOperation(ctx context.Context, group *openoltpb2.Group) error {
	logger.Debugw(ctx, "sending-group-to-device",
		log.Fields{
			"groupToOlt": group,
			"command":    group.Command})
	_, err := f.deviceHandler.Client.PerformGroupOperation(log.WithSpanFromContext(context.Background(), ctx), group)
	if err != nil {
		return olterrors.NewErrAdapter("group-operation-failed", log.Fields{"groupToOlt": group}, err)
	}
	return nil
}

//buildGroup build openoltpb2.Group from given group id and bucket list
func (f *OpenOltFlowMgr) buildGroup(ctx context.Context, groupID uint32, buckets []*ofp.OfpBucket) *openoltpb2.Group {
	group := openoltpb2.Group{
		GroupId: groupID}
	// create members of the group
	for _, ofBucket := range buckets {
		member := f.buildMember(ctx, ofBucket)
		if member != nil && !f.contains(group.Members, member) {
			group.Members = append(group.Members, member)
		}
	}
	return &group
}

//buildMember builds openoltpb2.GroupMember from an OpenFlow bucket
func (f *OpenOltFlowMgr) buildMember(ctx context.Context, ofBucket *ofp.OfpBucket) *openoltpb2.GroupMember {
	var outPort uint32
	outPortFound := false
	for _, ofAction := range ofBucket.Actions {
		if ofAction.Type == ofp.OfpActionType_OFPAT_OUTPUT {
			outPort = ofAction.GetOutput().Port
			outPortFound = true
		}
	}

	if !outPortFound {
		logger.Debugw(ctx, "bucket-skipped-since-no-out-port-found-in-it", log.Fields{"ofBucket": ofBucket})
		return nil
	}
	interfaceID := IntfIDFromUniPortNum(outPort)
	logger.Debugw(ctx, "got-associated-interface-id-of-the-port",
		log.Fields{
			"portNumber:":  outPort,
			"interfaceId:": interfaceID})
	if groupInfo, ok := f.interfaceToMcastQueueMap[interfaceID]; ok {
		member := openoltpb2.GroupMember{
			InterfaceId:   interfaceID,
			InterfaceType: openoltpb2.GroupMember_PON,
			GemPortId:     groupInfo.gemPortID,
			Priority:      groupInfo.servicePriority,
		}
		//add member to the group
		return &member
	}
	logger.Warnf(ctx, "bucket-skipped-since-interface-2-gem-mapping-cannot-be-found", log.Fields{"ofBucket": ofBucket})
	return nil
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
		f.deviceHandler.device.Type,
		onuDev.deviceType,
		onuDev.deviceID,
		onuDev.proxyDeviceID, "")
	if sendErr != nil {
		return olterrors.NewErrCommunication("send-techprofile-download-request",
			log.Fields{
				"from-adapter":  f.deviceHandler.device.Type,
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

	onu := rsrcMgr.OnuGemInfo{OnuID: onuID, SerialNumber: serialNum, IntfID: intfID}
	f.onuGemInfo[intfID] = append(f.onuGemInfo[intfID], onu)
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
			"onu-gem":     f.onuGemInfo[intfID]})
	onugem := f.onuGemInfo[intfID]
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
			f.onuGemInfo[intfID] = onugem
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
			"onu-gem":     f.onuGemInfo[intfID]})
}

// This function Lookup maps  by serialNumber or (intfId, gemPort)

//getOnuIDfromGemPortMap Returns OnuID,nil if found or set 0,error if no onuId is found for serialNumber or (intfId, gemPort)
func (f *OpenOltFlowMgr) getOnuIDfromGemPortMap(ctx context.Context, intfID uint32, gemPortID uint32) (uint32, error) {

	f.onuGemInfoLock.RLock()
	defer f.onuGemInfoLock.RUnlock()

	logger.Infow(ctx, "getting-onu-id-from-gem-port-and-pon-port",
		log.Fields{
			"device-id":   f.deviceHandler.device.Id,
			"onu-geminfo": f.onuGemInfo[intfID],
			"intf-id":     intfID,
			"gemport-id":  gemPortID})

	// get onuid from the onugem info cache
	onugem := f.onuGemInfo[intfID]

	for _, onu := range onugem {
		for _, gem := range onu.GemPorts {
			if gem == gemPortID {
				return onu.OnuID, nil
			}
		}
	}
	logger.Errorw(ctx, "onu-id-from-gem-port-not-found", log.Fields{
		"gem-port-id":      gemPortID,
		"interface-id":     intfID,
		"all-gems-on-port": onugem,
	})
	return uint32(0), olterrors.NewErrNotFound("onu-id", log.Fields{
		"interface-id": intfID,
		"gem-port-id":  gemPortID},
		nil)
}

//GetLogicalPortFromPacketIn function computes logical port UNI/NNI port from packet-in indication and returns the same
func (f *OpenOltFlowMgr) GetLogicalPortFromPacketIn(ctx context.Context, packetIn *openoltpb2.PacketIndication) (uint32, error) {
	var logicalPortNum uint32
	var onuID uint32
	var err error

	if packetIn.IntfType == "pon" {
		// packet indication does not have serial number , so sending as nil
		if onuID, err = f.getOnuIDfromGemPortMap(ctx, packetIn.IntfId, packetIn.GemportId); err != nil {
			// Called method is returning error with all data populated; just return the same
			return logicalPortNum, err
		}
		if packetIn.PortNo != 0 {
			logicalPortNum = packetIn.PortNo
		} else {
			uniID := uint32(0) //  FIXME - multi-uni support
			logicalPortNum = MkUniPortNum(ctx, packetIn.IntfId, onuID, uniID)
		}
		// Store the gem port through which the packet_in came. Use the same gem port for packet_out
		f.UpdateGemPortForPktIn(ctx, packetIn.IntfId, onuID, logicalPortNum, packetIn.GemportId, packetIn.Pkt)
	} else if packetIn.IntfType == "nni" {
		logicalPortNum = IntfIDToPortNo(packetIn.IntfId, voltha.Port_ETHERNET_NNI)
	}
	logger.Infow(ctx, "retrieved-logicalport-from-packet-in",
		log.Fields{
			"logical-port-num": logicalPortNum,
			"intf-type":        packetIn.IntfType,
			"packet":           hex.EncodeToString(packetIn.Pkt),
		})
	return logicalPortNum, nil
}

//GetPacketOutGemPortID returns gemPortId
func (f *OpenOltFlowMgr) GetPacketOutGemPortID(ctx context.Context, intfID uint32, onuID uint32, portNum uint32, packet []byte) (uint32, error) {
	var gemPortID uint32

	ctag, priority, err := getCTagFromPacket(ctx, packet)
	if err != nil {
		return 0, err
	}

	f.onuGemInfoLock.RLock()
	defer f.onuGemInfoLock.RUnlock()
	pktInkey := rsrcMgr.PacketInInfoKey{IntfID: intfID, OnuID: onuID, LogicalPort: portNum, VlanID: ctag, Priority: priority}
	var ok bool
	gemPortID, ok = f.packetInGemPort[pktInkey]
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
			f.packetInGemPort[pktInkey] = gemPortID
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

// nolint: gocyclo
func installFlowOnAllGemports(ctx context.Context,
	f1 func(ctx context.Context, intfId uint32, onuId uint32, uniId uint32,
		portNo uint32, classifier map[string]interface{}, action map[string]interface{},
		logicalFlow *ofp.OfpFlowStats, allocId uint32, gemPortId uint32, tpID uint32) error,
	f2 func(ctx context.Context, intfId uint32, onuId uint32, uniId uint32, portNo uint32,
		classifier map[string]interface{}, action map[string]interface{},
		logicalFlow *ofp.OfpFlowStats, allocId uint32, gemPortId uint32, vlanId uint32,
		tpID uint32) error,
	args map[string]uint32,
	classifier map[string]interface{}, action map[string]interface{},
	logicalFlow *ofp.OfpFlowStats,
	gemPorts []uint32,
	TpInst interface{},
	FlowType string,
	direction string,
	tpID uint32,
	vlanID ...uint32) {
	logger.Debugw(ctx, "installing-flow-on-all-gem-ports",
		log.Fields{
			"FlowType": FlowType,
			"gemPorts": gemPorts,
			"vlan":     vlanID})

	// The bit mapping for a gemport is expressed in tech-profile as a binary string. For example, 0b00000001
	// We need to trim prefix "0b", before further processing
	// Once the "0b" prefix is trimmed, we iterate each character in the string to identify which index
	// in the string is set to binary bit 1 (expressed as char '1' in the binary string).

	// If a particular character in the string is set to '1', identify the index of this character from
	// the LSB position which marks the PCP bit consumed by the given gem port.
	// This PCP bit now becomes a classifier in the flow.

	switch TpInst := TpInst.(type) {
	case *tp.TechProfile:
		attributes := TpInst.DownstreamGemPortAttributeList
		if direction == Upstream {
			attributes = TpInst.UpstreamGemPortAttributeList
		}

		for _, gemPortAttribute := range attributes {
			if direction == Downstream && strings.ToUpper(gemPortAttribute.IsMulticast) == "TRUE" {
				continue
			}
			gemPortID := gemPortAttribute.GemportID
			if allPbitsMarked(gemPortAttribute.PbitMap) {
				classifier[VlanPcp] = uint32(VlanPCPMask)
				if FlowType == DhcpFlow || FlowType == IgmpFlow || FlowType == HsiaFlow {
					if err := f1(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, tpID); err != nil {
						logger.Warn(ctx, err)
					}
				} else if FlowType == EapolFlow {
					if err := f2(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, vlanID[0], tpID); err != nil {
						logger.Warn(ctx, err)
					}
				}
			} else {
				for pos, pbitSet := range strings.TrimPrefix(gemPortAttribute.PbitMap, BinaryStringPrefix) {
					if pbitSet == BinaryBit1 {
						classifier[VlanPcp] = uint32(len(strings.TrimPrefix(gemPortAttribute.PbitMap, BinaryStringPrefix))) - 1 - uint32(pos)
						if FlowType == DhcpFlow || FlowType == IgmpFlow || FlowType == HsiaFlow {
							if err := f1(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, tpID); err != nil {
								logger.Warn(ctx, err)
							}
						} else if FlowType == EapolFlow {
							if err := f2(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, vlanID[0], tpID); err != nil {
								logger.Warn(ctx, err)
							}
						}
					}
				}
			}
		}
	case *tp.EponProfile:
		if direction == Upstream {
			attributes := TpInst.UpstreamQueueAttributeList
			for _, queueAttribute := range attributes {
				gemPortID := queueAttribute.GemportID
				if allPbitsMarked(queueAttribute.PbitMap) {
					classifier[VlanPcp] = uint32(VlanPCPMask)
					if FlowType == DhcpFlow || FlowType == IgmpFlow || FlowType == HsiaFlow {
						if err := f1(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, tpID); err != nil {
							logger.Warn(ctx, err)
						}
					} else if FlowType == EapolFlow {
						if err := f2(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, vlanID[0], tpID); err != nil {
							logger.Warn(ctx, err)
						}
					}
				} else {
					for pos, pbitSet := range strings.TrimPrefix(queueAttribute.PbitMap, BinaryStringPrefix) {
						if pbitSet == BinaryBit1 {
							classifier[VlanPcp] = uint32(len(strings.TrimPrefix(queueAttribute.PbitMap, BinaryStringPrefix))) - 1 - uint32(pos)
							if FlowType == DhcpFlow || FlowType == IgmpFlow || FlowType == HsiaFlow {
								if err := f1(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, tpID); err != nil {
									logger.Warn(ctx, err)
								}
							} else if FlowType == EapolFlow {
								if err := f2(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, vlanID[0], tpID); err != nil {
									logger.Warn(ctx, err)
								}
							}
						}
					}
				}
			}
		} else {
			attributes := TpInst.DownstreamQueueAttributeList
			for _, queueAttribute := range attributes {
				gemPortID := queueAttribute.GemportID
				if allPbitsMarked(queueAttribute.PbitMap) {
					classifier[VlanPcp] = uint32(VlanPCPMask)
					if FlowType == DhcpFlow || FlowType == IgmpFlow || FlowType == HsiaFlow {
						if err := f1(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, tpID); err != nil {
							logger.Warn(ctx, err)
						}
					} else if FlowType == EapolFlow {
						if err := f2(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, vlanID[0], tpID); err != nil {
							logger.Warn(ctx, err)
						}
					}
				} else {
					for pos, pbitSet := range strings.TrimPrefix(queueAttribute.PbitMap, BinaryStringPrefix) {
						if pbitSet == BinaryBit1 {
							classifier[VlanPcp] = uint32(len(strings.TrimPrefix(queueAttribute.PbitMap, BinaryStringPrefix))) - 1 - uint32(pos)
							if FlowType == DhcpFlow || FlowType == IgmpFlow || FlowType == HsiaFlow {
								if err := f1(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, tpID); err != nil {
									logger.Warn(ctx, err)
								}
							} else if FlowType == EapolFlow {
								if err := f2(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID, vlanID[0], tpID); err != nil {
									logger.Warn(ctx, err)
								}
							}
						}
					}
				}
			}
		}
	default:
		logger.Errorw(ctx, "unknown-tech", log.Fields{"tpInst": TpInst})
	}
}

func allPbitsMarked(pbitMap string) bool {
	for pos, pBit := range pbitMap {
		if pos >= 2 && pBit != BinaryBit1 {
			return false
		}
	}
	return true
}

func (f *OpenOltFlowMgr) addDHCPTrapFlowOnNNI(ctx context.Context, logicalFlow *ofp.OfpFlowStats, classifier map[string]interface{}, portNo uint32) error {
	logger.Debug(ctx, "adding-trap-dhcp-of-nni-flow")
	action := make(map[string]interface{})
	classifier[PacketTagType] = DoubleTag
	action[TrapToHost] = true
	/* We manage flowId resource pool on per PON port basis.
	   Since this situation is tricky, as a hack, we pass the NNI port
	   index (network_intf_id) as PON port Index for the flowId resource
	   pool. Also, there is no ONU Id available for trapping DHCP packets
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

	flowStoreCookie := getFlowStoreCookie(ctx, classifier, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), flowStoreCookie); present {
		logger.Info(ctx, "flow-exists-not-re-adding")
		return nil
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), uint32(gemPortID), flowStoreCookie, "", 0, 0)
	if err != nil {
		return olterrors.NewErrNotFound("dhcp-trap-nni-flow-id",
			log.Fields{
				"interface-id": networkInterfaceID,
				"onu-id":       onuID,
				"uni-id":       uniID,
				"gem-port-id":  gemPortID,
				"cookie":       flowStoreCookie},
			err)
	}
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
		FlowId:        flowID,
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
		return olterrors.NewErrFlowOp("add", flowID, log.Fields{"flow": downstreamflow}, err)
	}
	logger.Info(ctx, "dhcp-trap-on-nni-flow-added–to-device-successfully")
	flowsToKVStore := f.getUpdatedFlowInfo(ctx, &downstreamflow, flowStoreCookie, "", flowID, logicalFlow.Id)
	if err := f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID),
		int32(onuID),
		int32(uniID),
		flowID, flowsToKVStore); err != nil {
		return olterrors.NewErrPersistence("update", "flow", flowID, log.Fields{"flow": downstreamflow}, err)
	}
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
	flowStoreCookie := getFlowStoreCookie(ctx, classifier, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), flowStoreCookie); present {
		logger.Info(ctx, "igmp-flow-exists-not-re-adding")
		return nil
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), uint32(gemPortID), flowStoreCookie, "", 0, 0)
	if err != nil {
		return olterrors.NewErrNotFound("igmp-flow-id",
			log.Fields{
				"interface-id": networkInterfaceID,
				"onu-id":       onuID,
				"uni-id":       uniID,
				"gem-port-id":  gemPortID,
				"cookie":       flowStoreCookie},
			err)
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
		FlowId:        flowID,
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
		return olterrors.NewErrFlowOp("add", flowID, log.Fields{"flow": downstreamflow}, err)
	}
	logger.Info(ctx, "igmp-trap-on-nni-flow-added-to-device-successfully")
	flowsToKVStore := f.getUpdatedFlowInfo(ctx, &downstreamflow, flowStoreCookie, "", flowID, logicalFlow.Id)
	if err := f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID),
		int32(onuID),
		int32(uniID),
		flowID, flowsToKVStore); err != nil {
		return olterrors.NewErrPersistence("update", "flow", flowID, log.Fields{"flow": downstreamflow}, err)
	}
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
	var gemPort uint32
	intfID := args[IntfID]
	onuID := args[OnuID]
	uniID := args[UniID]
	portNo := args[PortNo]
	allocID := args[AllocID]
	if ipProto, ok := classifierInfo[IPProto]; ok {
		if ipProto.(uint32) == IPProtoDhcp {
			logger.Infow(ctx, "adding-dhcp-flow", log.Fields{
				"tp-id":    tpID,
				"alloc-id": allocID,
				"intf-id":  intfID,
				"onu-id":   onuID,
				"uni-id":   uniID,
			})
			if pcp, ok := classifierInfo[VlanPcp]; ok {
				gemPort = f.techprofile[intfID].GetGemportIDForPbit(ctx, TpInst,
					tp_pb.Direction_UPSTREAM,
					pcp.(uint32))
				//Adding DHCP upstream flow

				if err := f.addDHCPTrapFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID, gemPort, tpID); err != nil {
					logger.Warn(ctx, err)
				}
			} else {
				//Adding DHCP upstream flow to all gemports
				installFlowOnAllGemports(ctx, f.addDHCPTrapFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, TpInst, DhcpFlow, Upstream, tpID)
			}

		} else if ipProto.(uint32) == IgmpProto {
			logger.Infow(ctx, "adding-us-igmp-flow",
				log.Fields{
					"intf-id":          intfID,
					"onu-id":           onuID,
					"uni-id":           uniID,
					"classifier-info:": classifierInfo})
			if pcp, ok := classifierInfo[VlanPcp]; ok {
				gemPort = f.techprofile[intfID].GetGemportIDForPbit(ctx, TpInst,
					tp_pb.Direction_UPSTREAM,
					pcp.(uint32))
				if err := f.addIGMPTrapFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID, gemPort, tpID); err != nil {
					logger.Warn(ctx, err)
				}
			} else {
				//Adding IGMP upstream flow to all gem ports
				installFlowOnAllGemports(ctx, f.addIGMPTrapFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, TpInst, IgmpFlow, Upstream, tpID)
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
			})
			var vlanID uint32
			if val, ok := classifierInfo[VlanVid]; ok {
				vlanID = (val.(uint32)) & VlanvIDMask
			} else {
				vlanID = DefaultMgmtVlan
			}
			if pcp, ok := classifierInfo[VlanPcp]; ok {
				gemPort = f.techprofile[intfID].GetGemportIDForPbit(ctx, TpInst,
					tp_pb.Direction_UPSTREAM,
					pcp.(uint32))

				if err := f.addEAPOLFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID, gemPort, vlanID, tpID); err != nil {
					logger.Warn(ctx, err)
				}
			} else {
				installFlowOnAllGemports(ctx, nil, f.addEAPOLFlow, args, classifierInfo, actionInfo, flow, gemPorts, TpInst, EapolFlow, Upstream, tpID, vlanID)
			}
		}
	} else if _, ok := actionInfo[PushVlan]; ok {
		logger.Infow(ctx, "adding-upstream-data-rule", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"uni-id":  uniID,
		})
		if pcp, ok := classifierInfo[VlanPcp]; ok {
			gemPort = f.techprofile[intfID].GetGemportIDForPbit(ctx, TpInst,
				tp_pb.Direction_UPSTREAM,
				pcp.(uint32))
			//Adding HSIA upstream flow
			if err := f.addUpstreamDataFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID, gemPort, tpID); err != nil {
				logger.Warn(ctx, err)
			}
		} else {
			//Adding HSIA upstream flow to all gemports
			installFlowOnAllGemports(ctx, f.addUpstreamDataFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, TpInst, HsiaFlow, Upstream, tpID)
		}
	} else if _, ok := actionInfo[PopVlan]; ok {
		logger.Infow(ctx, "adding-downstream-data-rule", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"uni-id":  uniID,
		})
		if pcp, ok := classifierInfo[VlanPcp]; ok {
			gemPort = f.techprofile[intfID].GetGemportIDForPbit(ctx, TpInst,
				tp_pb.Direction_DOWNSTREAM,
				pcp.(uint32))
			//Adding HSIA downstream flow
			if err := f.addDownstreamDataFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID, gemPort, tpID); err != nil {
				logger.Warn(ctx, err)
			}
		} else {
			//Adding HSIA downstream flow to all gemports
			installFlowOnAllGemports(ctx, f.addDownstreamDataFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, TpInst, HsiaFlow, Downstream, tpID)
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

func (f *OpenOltFlowMgr) isGemPortUsedByAnotherFlow(ctx context.Context, gemPK gemPortKey) (bool, error) {
	if f.perGemPortLock.TryLock(gemPK) {
		flowIDList := f.flowsUsedByGemPort[gemPK]
		f.perGemPortLock.Unlock(gemPK)
		return len(flowIDList) > 1, nil
	}
	logger.Error(ctx, "failed-to-acquire-per-gem-port-lock",
		log.Fields{
			"device-id": f.deviceHandler.device.Id,
			"key":       gemPK,
		})
	return false, olterrors.NewErrAdapter("failed-to-acquire-per-gem-port-lock", log.Fields{
		"device-id": f.deviceHandler.device.Id,
		"key":       gemPK,
	}, nil)
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
		if err := f.resourceMgr.RemoveTechProfileIDForOnu(ctx, ponIntf, uint32(onuID), uint32(uniID), tpID); err != nil {
			logger.Warn(ctx, err)
		}
		if err := f.DeleteTechProfileInstance(ctx, ponIntf, uint32(onuID), uint32(uniID), "", tpID); err != nil {
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
			log.Debugw("action-set-vlan-pcp", log.Fields{"actionInfo[VLAN_PCP]": actionInfo[VlanPcp].(uint32)})
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

func appendUnique(slice []uint32, item uint32) []uint32 {
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

	f.onuGemInfoLock.Lock()
	defer f.onuGemInfoLock.Unlock()

	lookupGemPort, ok := f.packetInGemPort[pktInkey]
	if ok {
		if lookupGemPort == gemPort {
			logger.Infow(ctx, "pktin-key/value-found-in-cache--no-need-to-update-kv--assume-both-in-sync",
				log.Fields{
					"pktinkey": pktInkey,
					"gem":      gemPort})
			return
		}
	}
	f.packetInGemPort[pktInkey] = gemPort

	f.resourceMgr.UpdateGemPortForPktIn(ctx, pktInkey, gemPort)
	logger.Infow(ctx, "pktin-key-not-found-in-local-cache-value-is-different--updating-cache-and-kv-store",
		log.Fields{
			"pktinkey": pktInkey,
			"gem":      gemPort})
}

//getCTagFromPacket retrieves and returns c-tag and priority value from a packet.
func getCTagFromPacket(ctx context.Context, packet []byte) (uint16, uint8, error) {
	if packet == nil || len(packet) < 18 {
		log.Error("unable-get-c-tag-from-the-packet--invalid-packet-length ")
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

// AddUniPortToOnuInfo adds uni port to the onugem info both in cache and kvstore.
func (f *OpenOltFlowMgr) AddUniPortToOnuInfo(ctx context.Context, intfID uint32, onuID uint32, portNum uint32) {

	f.onuGemInfoLock.Lock()
	defer f.onuGemInfoLock.Unlock()

	onugem := f.onuGemInfo[intfID]
	for idx, onu := range onugem {
		if onu.OnuID == onuID {
			for _, uni := range onu.UniPorts {
				if uni == portNum {
					logger.Infow(ctx, "uni-already-in-cache--no-need-to-update-cache-and-kv-store", log.Fields{"uni": portNum})
					return
				}
			}
			onugem[idx].UniPorts = append(onugem[idx].UniPorts, portNum)
			f.onuGemInfo[intfID] = onugem
		}
	}
	f.resourceMgr.AddUniPortToOnuInfo(ctx, intfID, onuID, portNum)

}

func (f *OpenOltFlowMgr) loadFlowIDlistForGem(ctx context.Context, intf uint32) {
	flowIDsList, err := f.resourceMgr.GetFlowIDsGemMapForInterface(ctx, intf)
	if err != nil {
		logger.Error(ctx, "failed-to-get-flowid-list-per-gem", log.Fields{"intf": intf})
		return
	}
	for gem, FlowIDs := range flowIDsList {
		gemPK := gemPortKey{intf, uint32(gem)}
		if f.perGemPortLock.TryLock(gemPK) {
			f.flowsUsedByGemPort[gemPK] = FlowIDs
			f.perGemPortLock.Unlock(gemPK)
		} else {
			logger.Error(ctx, "failed-to-acquire-per-gem-port-lock",
				log.Fields{
					"intf-id":   intf,
					"device-id": f.deviceHandler.device.Id,
					"key":       gemPK,
				})
		}
	}
}

//loadInterfaceToMulticastQueueMap reads multicast queues per interface from the KV store
//and put them into interfaceToMcastQueueMap.
func (f *OpenOltFlowMgr) loadInterfaceToMulticastQueueMap(ctx context.Context) {
	storedMulticastQueueMap, err := f.resourceMgr.GetMcastQueuePerInterfaceMap(ctx)
	if err != nil {
		logger.Error(ctx, "failed-to-get-pon-interface-to-multicast-queue-map")
		return
	}
	for intf, queueInfo := range storedMulticastQueueMap {
		q := queueInfoBrief{
			gemPortID:       queueInfo[0],
			servicePriority: queueInfo[1],
		}
		f.interfaceToMcastQueueMap[intf] = &q
	}
}

//GetFlowGroupFromKVStore fetches and returns flow group from the KV store. Returns (nil, false, error) if any problem occurs during
//fetching the data. Returns (group, true, nil) if the group is fetched and returned successfully.
//Returns (nil, false, nil) if the group does not exists in the KV store.
func (f *OpenOltFlowMgr) GetFlowGroupFromKVStore(ctx context.Context, groupID uint32, cached bool) (*ofp.OfpGroupEntry, bool, error) {
	exists, groupInfo, err := f.resourceMgr.GetFlowGroupFromKVStore(ctx, groupID, cached)
	if err != nil {
		return nil, false, olterrors.NewErrNotFound("flow-group", log.Fields{"group-id": groupID}, err)
	}
	if exists {
		return newGroup(groupInfo.GroupID, groupInfo.OutPorts), exists, nil
	}
	return nil, exists, nil
}

func newGroup(groupID uint32, outPorts []uint32) *ofp.OfpGroupEntry {
	groupDesc := ofp.OfpGroupDesc{
		Type:    ofp.OfpGroupType_OFPGT_ALL,
		GroupId: groupID,
	}
	groupEntry := ofp.OfpGroupEntry{
		Desc: &groupDesc,
	}
	for i := 0; i < len(outPorts); i++ {
		var acts []*ofp.OfpAction
		acts = append(acts, flows.Output(outPorts[i]))
		bucket := ofp.OfpBucket{
			Actions: acts,
		}
		groupDesc.Buckets = append(groupDesc.Buckets, &bucket)
	}
	return &groupEntry
}
