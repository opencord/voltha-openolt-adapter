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
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/opencord/voltha-lib-go/v3/pkg/flows"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	tp "github.com/opencord/voltha-lib-go/v3/pkg/techprofile"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	"github.com/opencord/voltha-protos/v3/go/common"
	ic "github.com/opencord/voltha-protos/v3/go/inter_container"
	ofp "github.com/opencord/voltha-protos/v3/go/openflow_13"
	openoltpb2 "github.com/opencord/voltha-protos/v3/go/openolt"
	tp_pb "github.com/opencord/voltha-protos/v3/go/tech_profile"
	"github.com/opencord/voltha-protos/v3/go/voltha"

	//deepcopy "github.com/getlantern/deepcopy"
	"github.com/EagleChen/mapmutex"
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

	//FIXME - see also BRDCM_DEFAULT_VLAN in broadcom_onu.py

	//ReservedVlan Transparent Vlan
	ReservedVlan = 4095

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
	tpInst       *tp.TechProfile
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
	flowsUsedByGemPort map[gemPortKey][]uint32            //gem port id to flow ids
	packetInGemPort    map[rsrcMgr.PacketInInfoKey]uint32 //packet in gem port local cache
	onuGemInfo         map[uint32][]rsrcMgr.OnuGemInfo    //onu, gem and uni info local cache
	lockCache          sync.RWMutex
	pendingFlowDelete  sync.Map
	// The mapmutex.Mutex can be fine tuned to use mapmutex.NewCustomizedMapMutex
	perUserFlowHandleLock    *mapmutex.Mutex
	interfaceToMcastQueueMap map[uint32]*queueInfoBrief /*pon interface -> multicast queue map. Required to assign GEM to a bucket during group population*/
}

//NewFlowManager creates OpenOltFlowMgr object and initializes the parameters
func NewFlowManager(ctx context.Context, dh *DeviceHandler, rMgr *rsrcMgr.OpenOltResourceMgr) *OpenOltFlowMgr {
	log.Info("Initializing flow manager")
	var flowMgr OpenOltFlowMgr
	var err error
	var idx uint32

	flowMgr.deviceHandler = dh
	flowMgr.resourceMgr = rMgr
	flowMgr.techprofile = make(map[uint32]tp.TechProfileIf)
	if err = flowMgr.populateTechProfilePerPonPort(); err != nil {
		log.Error("Error while populating tech profile mgr\n")
		return nil
	}
	flowMgr.onuIdsLock = sync.RWMutex{}
	flowMgr.flowsUsedByGemPort = make(map[gemPortKey][]uint32)
	flowMgr.packetInGemPort = make(map[rsrcMgr.PacketInInfoKey]uint32)
	flowMgr.onuGemInfo = make(map[uint32][]rsrcMgr.OnuGemInfo)
	ponPorts := rMgr.DevInfo.GetPonPorts()
	//Load the onugem info cache from kv store on flowmanager start
	for idx = 0; idx < ponPorts; idx++ {
		if flowMgr.onuGemInfo[idx], err = rMgr.GetOnuGemInfo(ctx, idx); err != nil {
			log.Error("Failed to load onu gem info cache")
		}
		//Load flowID list per gem map per interface from the kvstore.
		flowMgr.loadFlowIDlistForGem(ctx, idx)
	}
	flowMgr.lockCache = sync.RWMutex{}
	flowMgr.pendingFlowDelete = sync.Map{}
	flowMgr.perUserFlowHandleLock = mapmutex.NewMapMutex()
	flowMgr.interfaceToMcastQueueMap = make(map[uint32]*queueInfoBrief)
	//load interface to multicast queue map from kv store
	flowMgr.loadInterfaceToMulticastQueueMap(ctx)
	log.Info("Initialization of  flow manager success!!")
	return &flowMgr
}

func (f *OpenOltFlowMgr) generateStoredFlowID(flowID uint32, direction string) (uint64, error) {
	if direction == Upstream {
		log.Debug("upstream flow, shifting id")
		return 0x1<<15 | uint64(flowID), nil
	} else if direction == Downstream {
		log.Debug("downstream flow, not shifting id")
		return uint64(flowID), nil
	} else if direction == Multicast {
		log.Debug("multicast flow, shifting id")
		return 0x2<<15 | uint64(flowID), nil
	} else {
		log.Debug("Unrecognized direction")
		return 0, fmt.Errorf("unrecognized direction %s", direction)
	}
}

func (f *OpenOltFlowMgr) registerFlow(ctx context.Context, flowFromCore *ofp.OfpFlowStats, deviceFlow *openoltpb2.Flow) {
	log.Debug("Registering Flow for Device ", log.Fields{"flow": flowFromCore},
		log.Fields{"device": f.deviceHandler.deviceID})
	gemPK := gemPortKey{uint32(deviceFlow.AccessIntfId), uint32(deviceFlow.GemportId)}
	flowIDList, ok := f.flowsUsedByGemPort[gemPK]
	if !ok {
		flowIDList = []uint32{deviceFlow.FlowId}
	}
	flowIDList = appendUnique(flowIDList, deviceFlow.FlowId)
	f.flowsUsedByGemPort[gemPK] = flowIDList
	// update the flowids for a gem to the KVstore
	f.resourceMgr.UpdateFlowIDsForGem(ctx, uint32(deviceFlow.AccessIntfId), uint32(deviceFlow.GemportId), flowIDList)
}

func (f *OpenOltFlowMgr) divideAndAddFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32,
	classifierInfo map[string]interface{}, actionInfo map[string]interface{}, flow *ofp.OfpFlowStats, TpID uint32,
	UsMeterID uint32, DsMeterID uint32, flowMetadata *voltha.FlowMetadata) {
	var allocID uint32
	var gemPorts []uint32
	var TpInst *tp.TechProfile

	log.Infow("Dividing flow", log.Fields{"intfId": intfID, "onuId": onuID, "uniId": uniID, "portNo": portNo,
		"classifier": classifierInfo, "action": actionInfo, "UsMeterID": UsMeterID, "DsMeterID": DsMeterID, "TpID": TpID})
	// only create tcont/gemports if there is actually an onu id.  otherwise BAL throws an error.  Usually this
	// is because the flow is an NNI flow and there would be no onu resources associated with it
	// TODO: properly deal with NNI flows
	if onuID <= 0 {
		log.Errorw("No onu id for flow", log.Fields{"portNo": portNo, "classifer": classifierInfo, "action": actionInfo})
		return
	}

	uni := getUniPortPath(intfID, int32(onuID), int32(uniID))
	log.Debugw("Uni port name", log.Fields{"uni": uni})

	tpLockMapKey := tpLockKey{intfID, onuID, uniID}
	if f.perUserFlowHandleLock.TryLock(tpLockMapKey) {
		allocID, gemPorts, TpInst = f.createTcontGemports(ctx, intfID, onuID, uniID, uni, portNo, TpID, UsMeterID, DsMeterID, flowMetadata)
		if allocID == 0 || gemPorts == nil || TpInst == nil {
			log.Error("alloc-id-gem-ports-tp-unavailable")
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
		log.Errorw("failed to acquire per user flow handle lock",
			log.Fields{"intfId": intfID, "onuId": onuID, "uniId": uniID})
		return
	}
}

// CreateSchedulerQueues creates traffic schedulers on the device with the given scheduler configuration and traffic shaping info
func (f *OpenOltFlowMgr) CreateSchedulerQueues(ctx context.Context, sq schedQueue) error {

	log.Debugw("CreateSchedulerQueues", log.Fields{"Dir": sq.direction, "IntfID": sq.intfID,
		"OnuID": sq.onuID, "UniID": sq.uniID, "TpID": sq.tpID, "MeterID": sq.meterID,
		"TpInst": sq.tpInst, "flowMetadata": sq.flowMetadata})

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
		log.Error("Failed to get meter for intf %d, onuid %d, uniid %d", sq.intfID, sq.onuID, sq.uniID)
		return err
	}
	if KvStoreMeter != nil {
		if KvStoreMeter.MeterId == sq.meterID {
			log.Debug("Scheduler already created for upstream")
			return nil
		}
		log.Errorw("Dynamic meter update not supported", log.Fields{"KvStoreMeterId": KvStoreMeter.MeterId, "MeterID-in-flow": sq.meterID})
		return errors.New("invalid-meter-id-in-flow")
	}

	log.Debugw("Meter-does-not-exist-Creating-new", log.Fields{"MeterID": sq.meterID, "Direction": Direction})

	if sq.direction == tp_pb.Direction_UPSTREAM {
		SchedCfg, err = f.techprofile[sq.intfID].GetUsScheduler(sq.tpInst)
	} else if sq.direction == tp_pb.Direction_DOWNSTREAM {
		SchedCfg, err = f.techprofile[sq.intfID].GetDsScheduler(sq.tpInst)
	}

	if err != nil {
		log.Errorw("Unable to get Scheduler config", log.Fields{"IntfID": sq.intfID, "Direction": sq.direction, "Error": err})
		return err
	}

	var meterConfig *ofp.OfpMeterConfig
	if sq.flowMetadata != nil {
		for _, meter := range sq.flowMetadata.Meters {
			if sq.meterID == meter.MeterId {
				meterConfig = meter
				log.Debugw("Found-meter-config-from-flowmetadata", log.Fields{"meterConfig": meterConfig})
				break
			}
		}
	} else {
		log.Error("Flow-metadata-is-not-present-in-flow")
	}
	if meterConfig == nil {
		log.Errorw("Could-not-get-meterbands-from-flowMetadata", log.Fields{"flowMetadata": sq.flowMetadata,
			"MeterID": sq.meterID})
		return errors.New("failed-to-get-meter-from-flowMetadata")
	} else if len(meterConfig.Bands) < MaxMeterBand {
		log.Errorw("Invalid-number-of-bands-in-meter", log.Fields{"Bands": meterConfig.Bands, "MeterID": sq.meterID})
		return errors.New("invalid-number-of-bands-in-meter")
	}
	cir := meterConfig.Bands[0].Rate
	cbs := meterConfig.Bands[0].BurstSize
	eir := meterConfig.Bands[1].Rate
	ebs := meterConfig.Bands[1].BurstSize
	pir := cir + eir
	pbs := cbs + ebs
	TrafficShaping := &tp_pb.TrafficShapingInfo{Cir: cir, Cbs: cbs, Pir: pir, Pbs: pbs}

	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile[sq.intfID].GetTrafficScheduler(sq.tpInst, SchedCfg, TrafficShaping)}

	if err := f.pushSchedulerQueuesToDevice(ctx, sq, TrafficShaping, TrafficSched); err != nil {
		log.Errorw("Failed to push traffic scheduler and queues to device", log.Fields{"intfID": sq.intfID, "direction": sq.direction})
		return err
	}

	/* After we successfully applied the scheduler configuration on the OLT device,
	 * store the meter id on the KV store, for further reference.
	 */
	if err := f.resourceMgr.UpdateMeterIDForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID, meterConfig); err != nil {
		log.Error("Failed to update meter id for onu %d, meterid %d", sq.onuID, sq.meterID)
		return err
	}
	log.Debugw("updated-meter-info into KV store successfully", log.Fields{"Direction": Direction,
		"Meter": meterConfig})
	return nil
}

func (f *OpenOltFlowMgr) pushSchedulerQueuesToDevice(ctx context.Context, sq schedQueue, TrafficShaping *tp_pb.TrafficShapingInfo, TrafficSched []*tp_pb.TrafficScheduler) error {

	trafficQueues, err := f.techprofile[sq.intfID].GetTrafficQueues(sq.tpInst, sq.direction)

	if err != nil {
		log.Errorw("Unable to construct traffic queue configuration", log.Fields{"intfID": sq.intfID, "direction": sq.direction})
		return err
	}

	log.Debugw("Sending Traffic scheduler create to device", log.Fields{"Direction": sq.direction, "TrafficScheds": TrafficSched})
	if _, err := f.deviceHandler.Client.CreateTrafficSchedulers(ctx, &tp_pb.TrafficSchedulers{
		IntfId: sq.intfID, OnuId: sq.onuID,
		UniId: sq.uniID, PortNo: sq.uniPort,
		TrafficScheds: TrafficSched}); err != nil {
		log.Errorw("Failed to create traffic schedulers", log.Fields{"error": err})
		return err
	}

	// On receiving the CreateTrafficQueues request, the driver should create corresponding
	// downstream queues.
	log.Debugw("Sending Traffic Queues create to device", log.Fields{"Direction": sq.direction, "TrafficQueues": trafficQueues})
	if _, err := f.deviceHandler.Client.CreateTrafficQueues(ctx,
		&tp_pb.TrafficQueues{IntfId: sq.intfID, OnuId: sq.onuID,
			UniId: sq.uniID, PortNo: sq.uniPort,
			TrafficQueues: trafficQueues}); err != nil {
		log.Errorw("Failed to create traffic queues in device", log.Fields{"error": err})
		return err
	}

	if sq.direction == tp_pb.Direction_DOWNSTREAM {
		multicastTrafficQueues := f.techprofile[sq.intfID].GetMulticastTrafficQueues(sq.tpInst)
		if len(multicastTrafficQueues) > 0 {
			if _, present := f.interfaceToMcastQueueMap[sq.intfID]; !present {
				//assumed that there is only one queue per PON for the multicast service
				//the default queue with multicastQueuePerPonPort.Priority per a pon interface is used for multicast service
				//just put it in interfaceToMcastQueueMap to use for building group members
				multicastQueuePerPonPort := multicastTrafficQueues[0]
				f.interfaceToMcastQueueMap[sq.intfID] = &queueInfoBrief{
					gemPortID:       multicastQueuePerPonPort.GemportId,
					servicePriority: multicastQueuePerPonPort.Priority,
				}
				//also store the queue info in kv store
				f.resourceMgr.AddMcastQueueForIntf(ctx, sq.intfID,
					multicastQueuePerPonPort.GemportId,
					multicastQueuePerPonPort.Priority)
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
	log.Debugw("Removing schedulers and Queues in OLT", log.Fields{"Direction": sq.direction, "IntfID": sq.intfID,
		"OnuID": sq.onuID, "UniID": sq.uniID, "UniPort": sq.uniPort})
	if sq.direction == tp_pb.Direction_UPSTREAM {
		SchedCfg, err = f.techprofile[sq.intfID].GetUsScheduler(sq.tpInst)
		Direction = "upstream"
	} else if sq.direction == tp_pb.Direction_DOWNSTREAM {
		SchedCfg, err = f.techprofile[sq.intfID].GetDsScheduler(sq.tpInst)
		Direction = "downstream"
	}

	if err != nil {
		log.Errorw("Unable to get Scheduler config", log.Fields{"IntID": sq.intfID, "Direction": sq.direction, "Error": err})
		return err
	}

	KVStoreMeter, err := f.resourceMgr.GetMeterIDForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID)
	if err != nil {
		log.Errorf("Failed to get Meter for Onu %d", sq.onuID)
		return err
	}
	if KVStoreMeter == nil {
		log.Debugw("No-meter-has-been-installed-yet", log.Fields{"direction": Direction, "IntfID": sq.intfID, "OnuID": sq.onuID, "UniID": sq.uniID})
		return nil
	}
	cir := KVStoreMeter.Bands[0].Rate
	cbs := KVStoreMeter.Bands[0].BurstSize
	eir := KVStoreMeter.Bands[1].Rate
	ebs := KVStoreMeter.Bands[1].BurstSize
	pir := cir + eir
	pbs := cbs + ebs

	TrafficShaping := &tp_pb.TrafficShapingInfo{Cir: cir, Cbs: cbs, Pir: pir, Pbs: pbs}

	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile[sq.intfID].GetTrafficScheduler(sq.tpInst, SchedCfg, TrafficShaping)}

	TrafficQueues, err := f.techprofile[sq.intfID].GetTrafficQueues(sq.tpInst, sq.direction)
	if err != nil {
		log.Errorw("Unable to construct traffic queue configuration", log.Fields{"intfID": sq.intfID, "direction": sq.direction})
		return err
	}

	if _, err = f.deviceHandler.Client.RemoveTrafficQueues(ctx,
		&tp_pb.TrafficQueues{IntfId: sq.intfID, OnuId: sq.onuID,
			UniId: sq.uniID, PortNo: sq.uniPort,
			TrafficQueues: TrafficQueues}); err != nil {
		log.Errorw("Failed to remove traffic queues", log.Fields{"error": err})
		return err
	}
	log.Debug("Removed traffic queues successfully")
	if _, err = f.deviceHandler.Client.RemoveTrafficSchedulers(ctx, &tp_pb.TrafficSchedulers{
		IntfId: sq.intfID, OnuId: sq.onuID,
		UniId: sq.uniID, PortNo: sq.uniPort,
		TrafficScheds: TrafficSched}); err != nil {
		log.Errorw("failed to remove traffic schedulers", log.Fields{"error": err})
		return err
	}

	log.Debug("Removed traffic schedulers successfully")

	/* After we successfully remove the scheduler configuration on the OLT device,
	 * delete the meter id on the KV store.
	 */
	err = f.resourceMgr.RemoveMeterIDForOnu(ctx, Direction, sq.intfID, sq.onuID, sq.uniID, sq.tpID)
	if err != nil {
		log.Errorf("Failed to remove meter for onu %d, meter id %d", sq.onuID, KVStoreMeter.MeterId)
		return err
	}
	log.Debugw("Removed-meter-from-KV-store successfully", log.Fields{"MeterId": KVStoreMeter.MeterId, "dir": Direction})
	return err
}

// This function allocates tconts and GEM ports for an ONU
func (f *OpenOltFlowMgr) createTcontGemports(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, uni string, uniPort uint32, TpID uint32, UsMeterID uint32, DsMeterID uint32, flowMetadata *voltha.FlowMetadata) (uint32, []uint32, *tp.TechProfile) {
	var allocIDs []uint32
	var allgemPortIDs []uint32
	var gemPortIDs []uint32
	tpInstanceExists := false
	var err error

	allocIDs = f.resourceMgr.GetCurrentAllocIDsForOnu(ctx, intfID, onuID, uniID)
	allgemPortIDs = f.resourceMgr.GetCurrentGEMPortIDsForOnu(ctx, intfID, onuID, uniID)

	tpPath := f.getTPpath(intfID, uni, TpID)

	log.Infow("creating-new-tcont-and-gem", log.Fields{"pon": intfID, "onu": onuID, "uni": uniID})

	// Check tech profile instance already exists for derived port name
	techProfileInstance, _ := f.techprofile[intfID].GetTPInstanceFromKVStore(ctx, TpID, tpPath)
	if techProfileInstance == nil {
		log.Infow("tp-instance-not-found--creating-new", log.Fields{"path": tpPath})
		techProfileInstance, err = f.techprofile[intfID].CreateTechProfInstance(ctx, TpID, uni, intfID)
		if err != nil {
			// This should not happen, something wrong in KV backend transaction
			log.Errorw("tp-instance-create-failed", log.Fields{"error": err, "tpID": TpID})
			return 0, nil, nil
		}
		f.resourceMgr.UpdateTechProfileIDForOnu(ctx, intfID, onuID, uniID, TpID)
	} else {
		log.Debugw("Tech-profile-instance-already-exist-for-given port-name", log.Fields{"uni": uni})
		tpInstanceExists = true
	}
	if UsMeterID != 0 {
		sq := schedQueue{direction: tp_pb.Direction_UPSTREAM, intfID: intfID, onuID: onuID, uniID: uniID, tpID: TpID,
			uniPort: uniPort, tpInst: techProfileInstance, meterID: UsMeterID, flowMetadata: flowMetadata}
		if err := f.CreateSchedulerQueues(ctx, sq); err != nil {
			log.Errorw("CreateSchedulerQueues Failed-upstream", log.Fields{"error": err, "meterID": UsMeterID})
			return 0, nil, nil
		}
	}
	if DsMeterID != 0 {
		sq := schedQueue{direction: tp_pb.Direction_DOWNSTREAM, intfID: intfID, onuID: onuID, uniID: uniID, tpID: TpID,
			uniPort: uniPort, tpInst: techProfileInstance, meterID: DsMeterID, flowMetadata: flowMetadata}
		if err := f.CreateSchedulerQueues(ctx, sq); err != nil {
			log.Errorw("CreateSchedulerQueues Failed-downstream", log.Fields{"error": err, "meterID": DsMeterID})
			return 0, nil, nil
		}
	}

	allocID := techProfileInstance.UsScheduler.AllocID
	for _, gem := range techProfileInstance.UpstreamGemPortAttributeList {
		gemPortIDs = append(gemPortIDs, gem.GemportID)
	}

	if tpInstanceExists {
		return allocID, gemPortIDs, techProfileInstance
	}

	allocIDs = appendUnique(allocIDs, allocID)
	for _, gemPortID := range gemPortIDs {
		allgemPortIDs = appendUnique(allgemPortIDs, gemPortID)
	}

	log.Debugw("Allocated Tcont and GEM ports", log.Fields{"allocIDs": allocIDs, "gemports": allgemPortIDs})
	// Send Tconts and GEM ports to KV store
	f.storeTcontsGEMPortsIntoKVStore(ctx, intfID, onuID, uniID, allocIDs, allgemPortIDs)
	return allocID, gemPortIDs, techProfileInstance
}

func (f *OpenOltFlowMgr) storeTcontsGEMPortsIntoKVStore(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, allocID []uint32, gemPortIDs []uint32) {

	log.Debugw("Storing allocated Tconts and GEM ports into KV store",
		log.Fields{"intfId": intfID, "onuId": onuID, "uniId": uniID, "allocID": allocID, "gemPortIDs": gemPortIDs})
	/* Update the allocated alloc_id and gem_port_id for the ONU/UNI to KV store  */
	if err := f.resourceMgr.UpdateAllocIdsForOnu(ctx, intfID, onuID, uniID, allocID); err != nil {
		log.Error("Errow while uploading allocID to KV store")
	}
	if err := f.resourceMgr.UpdateGEMPortIDsForOnu(ctx, intfID, onuID, uniID, gemPortIDs); err != nil {
		log.Error("Errow while uploading GEMports to KV store")
	}
	if err := f.resourceMgr.UpdateGEMportsPonportToOnuMapOnKVStore(ctx, gemPortIDs, intfID, onuID, uniID); err != nil {
		log.Error("Errow while uploading gemtopon map to KV store")
	}
	log.Debug("Stored tconts and GEM into KV store successfully")
	for _, gemPort := range gemPortIDs {
		f.addGemPortToOnuInfoMap(ctx, intfID, onuID, gemPort)
	}
}

func (f *OpenOltFlowMgr) populateTechProfilePerPonPort() error {
	var tpCount int
	for _, techRange := range f.resourceMgr.DevInfo.Ranges {
		for _, intfID := range techRange.IntfIds {
			f.techprofile[intfID] = f.resourceMgr.ResourceMgrs[uint32(intfID)].TechProfileMgr
			tpCount++
			log.Debugw("Init tech profile done", log.Fields{"intfID": intfID})
		}
	}
	//Make sure we have as many tech_profiles as there are pon ports on the device
	if tpCount != int(f.resourceMgr.DevInfo.GetPonPorts()) {
		log.Errorw("Error while populating techprofile",
			log.Fields{"numofTech": tpCount, "numPonPorts": f.resourceMgr.DevInfo.GetPonPorts()})
		return errors.New("error while populating techprofile mgrs")
	}
	log.Infow("Populated techprofile for ponports successfully",
		log.Fields{"numofTech": tpCount, "numPonPorts": f.resourceMgr.DevInfo.GetPonPorts()})
	return nil
}

func (f *OpenOltFlowMgr) addUpstreamDataFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32,
	portNo uint32, uplinkClassifier map[string]interface{},
	uplinkAction map[string]interface{}, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemportID uint32) {
	uplinkClassifier[PacketTagType] = SingleTag
	log.Debugw("Adding upstream data flow", log.Fields{"uplinkClassifier": uplinkClassifier, "uplinkAction": uplinkAction})
	f.addHSIAFlow(ctx, intfID, onuID, uniID, portNo, uplinkClassifier, uplinkAction,
		Upstream, logicalFlow, allocID, gemportID)
	/* TODO: Install Secondary EAP on the subscriber vlan */
}

func (f *OpenOltFlowMgr) addDownstreamDataFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32,
	portNo uint32, downlinkClassifier map[string]interface{},
	downlinkAction map[string]interface{}, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemportID uint32) {
	downlinkClassifier[PacketTagType] = DoubleTag
	log.Debugw("Adding downstream data flow", log.Fields{"downlinkClassifier": downlinkClassifier,
		"downlinkAction": downlinkAction})
	// Ignore Downlink trap flow given by core, cannot do anything with this flow */
	if vlan, exists := downlinkClassifier[VlanVid]; exists {
		if vlan.(uint32) == (uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 4000) { //private VLAN given by core
			if metadata, exists := downlinkClassifier[Metadata]; exists { // inport is filled in metadata by core
				if uint32(metadata.(uint64)) == MkUniPortNum(intfID, onuID, uniID) {
					log.Infow("Ignoring DL trap device flow from core", log.Fields{"flow": logicalFlow})
					return
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
		log.Error("dl-classifier-vid-type-conversion-failed")
		return
	}

	f.addHSIAFlow(ctx, intfID, onuID, uniID, portNo, downlinkClassifier, downlinkAction,
		Downstream, logicalFlow, allocID, gemportID)
}

func (f *OpenOltFlowMgr) addHSIAFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifier map[string]interface{},
	action map[string]interface{}, direction string, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemPortID uint32) {
	var networkIntfID uint32
	/* One of the OLT platform (Broadcom BAL) requires that symmetric
	   flows require the same flow_id to be used across UL and DL.
	   Since HSIA flow is the only symmetric flow currently, we need to
	   re-use the flow_id across both direction. The 'flow_category'
	   takes priority over flow_cookie to find any available HSIA_FLOW
	   id for the ONU.
	*/
	log.Debugw("Adding HSIA flow", log.Fields{"intfId": intfID, "onuId": onuID, "uniId": uniID, "classifier": classifier,
		"action": action, "direction": direction, "allocId": allocID, "gemPortId": gemPortID,
		"logicalFlow": *logicalFlow})
	var vlanPbit uint32
	if _, ok := classifier[VlanPcp]; ok {
		vlanPbit = classifier[VlanPcp].(uint32)
		log.Debugw("Found pbit in the flow", log.Fields{"VlanPbit": vlanPbit})
	}
	flowStoreCookie := getFlowStoreCookie(classifier, gemPortID)
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(intfID), int32(onuID), int32(uniID), flowStoreCookie); present {
		log.Debug("Flow-exists--not-re-adding")
		return
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, intfID, int32(onuID), int32(uniID), gemPortID, flowStoreCookie, HsiaFlow, vlanPbit)
	if err != nil {
		log.Errorw("Flow id unavailable for HSIA flow", log.Fields{"direction": direction})
		return
	}
	var classifierProto *openoltpb2.Classifier
	var actionProto *openoltpb2.Action
	if classifierProto = makeOpenOltClassifierField(classifier); classifierProto == nil {
		log.Error("Error in making classifier protobuf for hsia flow")
		return
	}
	log.Debugw("Created classifier proto", log.Fields{"classifier": *classifierProto})
	if actionProto = makeOpenOltActionField(action); actionProto == nil {
		log.Errorw("Error in making action protobuf for hsia flow", log.Fields{"direction": direction})
		return
	}
	log.Debugw("Created action proto", log.Fields{"action": *actionProto})
	networkIntfID, err = getNniIntfID(classifier, action)
	if err != nil {
		log.Error("Failed to get nniIntf ID")
		return
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
		PortNo:        portNo}
	if ok := f.addFlowToDevice(ctx, logicalFlow, &flow); ok {
		log.Debug("HSIA flow added to device successfully", log.Fields{"direction": direction})
		flowsToKVStore := f.getUpdatedFlowInfo(ctx, &flow, flowStoreCookie, HsiaFlow, flowID, logicalFlow.Id)
		if err := f.updateFlowInfoToKVStore(ctx, flow.AccessIntfId,
			flow.OnuId,
			flow.UniId,
			flow.FlowId /*flowCategory,*/, flowsToKVStore); err != nil {
			log.Errorw("Error uploading HSIA  flow into KV store", log.Fields{"flow": flow, "direction": direction, "error": err})
			return
		}
	}
}

func (f *OpenOltFlowMgr) addDHCPTrapFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifier map[string]interface{}, action map[string]interface{}, logicalFlow *ofp.OfpFlowStats, allocID uint32, gemPortID uint32) {

	var dhcpFlow openoltpb2.Flow
	var actionProto *openoltpb2.Action
	var classifierProto *openoltpb2.Classifier
	var flowID uint32
	networkIntfID, err := getNniIntfID(classifier, action)
	if err != nil {
		log.Error("Failed to get nniIntf ID")
		return
	}

	// Clear the action map
	for k := range action {
		delete(action, k)
	}

	action[TrapToHost] = true
	classifier[UDPSrc] = uint32(68)
	classifier[UDPDst] = uint32(67)
	classifier[PacketTagType] = SingleTag
	delete(classifier, VlanVid)

	flowStoreCookie := getFlowStoreCookie(classifier, gemPortID)
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(intfID), int32(onuID), int32(uniID), flowStoreCookie); present {
		log.Debug("Flow-exists--not-re-adding")
		return
	}

	flowID, err = f.resourceMgr.GetFlowID(ctx, intfID, int32(onuID), int32(uniID), gemPortID, flowStoreCookie, DhcpFlow, 0 /*classifier[VLAN_PCP].(uint32)*/)

	if err != nil {
		log.Errorw("flowId unavailable for UL DHCP", log.Fields{"intfId": intfID, "onuId": onuID, "flowStoreCookie": flowStoreCookie})
		return
	}

	log.Debugw("Creating UL DHCP flow", log.Fields{"ul_classifier": classifier, "ul_action": action, "uplinkFlowId": flowID})

	if classifierProto = makeOpenOltClassifierField(classifier); classifierProto == nil {
		log.Error("Error in making classifier protobuf for ul flow")
		return
	}
	log.Debugw("Created classifier proto", log.Fields{"classifier": *classifierProto})
	if actionProto = makeOpenOltActionField(action); actionProto == nil {
		log.Error("Error in making action protobuf for ul flow")
		return
	}

	dhcpFlow = openoltpb2.Flow{AccessIntfId: int32(intfID),
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
		PortNo:        portNo}

	if ok := f.addFlowToDevice(ctx, logicalFlow, &dhcpFlow); ok {
		log.Debug("DHCP UL flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(ctx, &dhcpFlow, flowStoreCookie, "DHCP", flowID, logicalFlow.Id)
		if err := f.updateFlowInfoToKVStore(ctx, dhcpFlow.AccessIntfId,
			dhcpFlow.OnuId,
			dhcpFlow.UniId,
			dhcpFlow.FlowId, flowsToKVStore); err != nil {
			log.Errorw("Error uploading DHCP UL flow into KV store", log.Fields{"flow": dhcpFlow, "error": err})
			return
		}
	}

	return
}

//addIGMPTrapFlow creates IGMP trap-to-host flow
func (f *OpenOltFlowMgr) addIGMPTrapFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifier map[string]interface{},
	action map[string]interface{}, logicalFlow *ofp.OfpFlowStats, allocID uint32, gemPortID uint32) {
	f.addUpstreamTrapFlow(ctx, intfID, onuID, uniID, portNo, classifier, action, logicalFlow, allocID, gemPortID, IgmpFlow)
}

//addUpstreamTrapFlow creates a trap-to-host flow
func (f *OpenOltFlowMgr) addUpstreamTrapFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifier map[string]interface{},
	action map[string]interface{}, logicalFlow *ofp.OfpFlowStats, allocID uint32, gemPortID uint32, flowType string) {

	var flow openoltpb2.Flow
	var actionProto *openoltpb2.Action
	var classifierProto *openoltpb2.Classifier

	networkIntfID, err := getNniIntfID(classifier, action)
	if err != nil {
		log.Error("Failed to get nniIntf ID")
		return
	}

	// Clear the action map
	for k := range action {
		delete(action, k)
	}

	action[TrapToHost] = true
	classifier[PacketTagType] = SingleTag
	delete(classifier, VlanVid)

	flowStoreCookie := getFlowStoreCookie(classifier, gemPortID)
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkIntfID), int32(onuID), int32(uniID), flowStoreCookie); present {
		log.Debug("Flow-exists--not-re-adding")
		return
	}

	flowID, err := f.resourceMgr.GetFlowID(ctx, intfID, int32(onuID), int32(uniID), gemPortID, flowStoreCookie, flowType, 0, 0 /*classifier[VLAN_PCP].(uint32)*/)

	if err != nil {
		log.Errorw("flowId unavailable for upstream trap flow", log.Fields{"intfId": intfID, "onuId": onuID, "flowStoreCookie": flowStoreCookie, "flowType": flowType})
		return
	}

	log.Debugw("Creating upstream trap flow", log.Fields{"ul_classifier": classifier, "ul_action": action, "uplinkFlowId": flowID, "flowType": flowType})

	if classifierProto = makeOpenOltClassifierField(classifier); classifierProto == nil {
		log.Error("Error in making classifier protobuf for ul flow")
		return
	}
	log.Debugw("Created classifier proto", log.Fields{"classifier": *classifierProto})
	if actionProto = makeOpenOltActionField(action); actionProto == nil {
		log.Error("Error in making action protobuf for ul flow")
		return
	}

	flow = openoltpb2.Flow{AccessIntfId: int32(intfID),
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
		PortNo:        portNo}

	if ok := f.addFlowToDevice(ctx, logicalFlow, &flow); ok {
		log.Debugf("%s UL flow added to device successfully", flowType)

		flowsToKVStore := f.getUpdatedFlowInfo(ctx, &flow, flowStoreCookie, flowType, flowID, logicalFlow.Id)
		if err := f.updateFlowInfoToKVStore(ctx, flow.AccessIntfId,
			flow.OnuId,
			flow.UniId,
			flow.FlowId, flowsToKVStore); err != nil {
			log.Errorw("Error uploading UL flow into KV store", log.Fields{"flow": flow, "error": err})
			return
		}
	}

	return
}

// Add EAPOL flow to  device with mac, vlanId as classifier for upstream and downstream
func (f *OpenOltFlowMgr) addEAPOLFlow(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, portNo uint32, logicalFlow *ofp.OfpFlowStats, allocID uint32, gemPortID uint32, vlanID uint32, classifier map[string]interface{}, action map[string]interface{}) {
	log.Debugw("Adding EAPOL to device", log.Fields{"intfId": intfID, "onuId": onuID, "portNo": portNo, "allocId": allocID, "gemPortId": gemPortID, "vlanId": vlanID, "flow": logicalFlow})

	uplinkClassifier := make(map[string]interface{})
	uplinkAction := make(map[string]interface{})

	var upstreamFlow openoltpb2.Flow
	var networkIntfID uint32

	// Fill Classfier
	uplinkClassifier[EthType] = uint32(EapEthType)
	uplinkClassifier[PacketTagType] = SingleTag
	uplinkClassifier[VlanVid] = vlanID
	// Fill action
	uplinkAction[TrapToHost] = true
	flowStoreCookie := getFlowStoreCookie(uplinkClassifier, gemPortID)
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(intfID), int32(onuID), int32(uniID), flowStoreCookie); present {
		log.Debug("Flow-exists--not-re-adding")
		return
	}
	//Add Uplink EAPOL Flow
	uplinkFlowID, err := f.resourceMgr.GetFlowID(ctx, intfID, int32(onuID), int32(uniID), gemPortID, flowStoreCookie, "", 0)
	if err != nil {
		log.Errorw("flowId unavailable for UL EAPOL", log.Fields{"intfId": intfID, "onuId": onuID, "flowStoreCookie": flowStoreCookie})
		return
	}
	var classifierProto *openoltpb2.Classifier
	var actionProto *openoltpb2.Action
	log.Debugw("Creating UL EAPOL flow", log.Fields{"ul_classifier": uplinkClassifier, "ul_action": uplinkAction, "uplinkFlowId": uplinkFlowID})

	if classifierProto = makeOpenOltClassifierField(uplinkClassifier); classifierProto == nil {
		log.Error("Error in making classifier protobuf for ul flow")
		return
	}
	log.Debugw("Created classifier proto", log.Fields{"classifier": *classifierProto})
	if actionProto = makeOpenOltActionField(uplinkAction); actionProto == nil {
		log.Error("Error in making action protobuf for ul flow")
		return
	}
	log.Debugw("Created action proto", log.Fields{"action": *actionProto})
	networkIntfID, err = getNniIntfID(classifier, action)
	if err != nil {
		log.Error("Failed to get nniIntf ID")
		return
	}

	upstreamFlow = openoltpb2.Flow{AccessIntfId: int32(intfID),
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
		PortNo:        portNo}
	if ok := f.addFlowToDevice(ctx, logicalFlow, &upstreamFlow); ok {
		log.Debug("EAPOL UL flow added to device successfully")
		flowCategory := "EAPOL"
		flowsToKVStore := f.getUpdatedFlowInfo(ctx, &upstreamFlow, flowStoreCookie, flowCategory, uplinkFlowID, logicalFlow.Id)
		if err := f.updateFlowInfoToKVStore(ctx, upstreamFlow.AccessIntfId,
			upstreamFlow.OnuId,
			upstreamFlow.UniId,
			upstreamFlow.FlowId,
			/* lowCategory, */
			flowsToKVStore); err != nil {
			log.Errorw("Error uploading EAPOL UL flow into KV store", log.Fields{"flow": upstreamFlow, "error": err})
			return
		}
	}

	log.Debugw("Added EAPOL flows to device successfully", log.Fields{"flow": logicalFlow})
}

func makeOpenOltClassifierField(classifierInfo map[string]interface{}) *openoltpb2.Classifier {
	var classifier openoltpb2.Classifier

	classifier.EthType, _ = classifierInfo[EthType].(uint32)
	classifier.IpProto, _ = classifierInfo[IPProto].(uint32)
	if vlanID, ok := classifierInfo[VlanVid].(uint32); ok {
		vid := vlanID & VlanvIDMask
		if vid != ReservedVlan {
			classifier.OVid = vid
		}
	}
	if metadata, ok := classifierInfo[Metadata].(uint64); ok {
		vid := uint32(metadata)
		if vid != ReservedVlan {
			classifier.IVid = vid
		}
	}
	if vlanPcp, ok := classifierInfo[VlanPcp].(uint32); ok {
		if vlanPcp == 0 {
			classifier.OPbits = VlanPCPMask
		} else {
			classifier.OPbits = vlanPcp & VlanPCPMask
		}
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
			log.Error("Invalid tag type in classifier") // should not hit
			return nil
		}
	}
	return &classifier
}

func makeOpenOltActionField(actionInfo map[string]interface{}) *openoltpb2.Action {
	var actionCmd openoltpb2.ActionCmd
	var action openoltpb2.Action
	action.Cmd = &actionCmd
	if _, ok := actionInfo[PopVlan]; ok {
		action.OVid = actionInfo[VlanVid].(uint32)
		action.Cmd.RemoveOuterTag = true
	} else if _, ok := actionInfo[PushVlan]; ok {
		action.OVid = actionInfo[VlanVid].(uint32)
		action.Cmd.AddOuterTag = true
	} else if _, ok := actionInfo[TrapToHost]; ok {
		action.Cmd.TrapToHost = actionInfo[TrapToHost].(bool)
	} else {
		log.Errorw("Invalid-action-field", log.Fields{"action": actionInfo})
		return nil
	}
	return &action
}

func (f *OpenOltFlowMgr) getTPpath(intfID uint32, uni string, TpID uint32) string {
	return f.techprofile[intfID].GetTechProfileInstanceKVPath(TpID, uni)
}

// DeleteTechProfileInstances removes the tech profile instances from persistent storage
func (f *OpenOltFlowMgr) DeleteTechProfileInstances(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, sn string) error {
	tpIDList := f.resourceMgr.GetTechProfileIDForOnu(ctx, intfID, onuID, uniID)
	uniPortName := fmt.Sprintf("pon-{%d}/onu-{%d}/uni-{%d}", intfID, onuID, uniID)
	for _, tpID := range tpIDList {
		if err := f.DeleteTechProfileInstance(ctx, intfID, onuID, uniID, uniPortName, tpID); err != nil {
			log.Debugw("Failed-to-delete-tp-instance-from-kv-store", log.Fields{"tp-id": tpID, "uni-port-name": uniPortName})
			// return err
			// We should continue to delete tech-profile instances for other TP IDs
		}
	}
	return nil
}

// DeleteTechProfileInstance removes the tech profile instance from persistent storage
func (f *OpenOltFlowMgr) DeleteTechProfileInstance(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, uniPortName string, tpID uint32) error {
	if uniPortName == "" {
		uniPortName = fmt.Sprintf("pon-{%d}/onu-{%d}/uni-{%d}", intfID, onuID, uniID)
	}
	if err := f.techprofile[intfID].DeleteTechProfileInstance(ctx, tpID, uniPortName); err != nil {
		log.Debugw("Failed-to-delete-tp-instance-from-kv-store", log.Fields{"tp-id": tpID, "uni-port-name": uniPortName})
		return err
	}
	return nil
}

func getFlowStoreCookie(classifier map[string]interface{}, gemPortID uint32) uint64 {
	if len(classifier) == 0 { // should never happen
		log.Error("Invalid classfier object")
		return 0
	}
	log.Debugw("generating flow store cookie", log.Fields{"classifier": classifier, "gemPortID": gemPortID})
	var jsonData []byte
	var flowString string
	var err error
	// TODO: Do we need to marshall ??
	if jsonData, err = json.Marshal(classifier); err != nil {
		log.Error("Failed to encode classifier")
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
	log.Debugw("hash generated", log.Fields{"hash": generatedHash})
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
		log.Debugw("Flow exists for given flowID, appending it to current flow", log.Fields{"flowID": flow.FlowId})
		//for _, f := range *existingFlows {
		//	flows = append(flows, f)
		//}
		flows = append(flows, *existingFlows...)
	}
	log.Debugw("Updated flows for given flowID and onuid", log.Fields{"updatedflow": flows, "flowid": flow.FlowId, "onu": flow.OnuId})
	return &flows
}

//func (f *OpenOltFlowMgr) getUpdatedFlowInfo(flow *openolt_pb2.Flow, flowStoreCookie uint64, flowCategory string) *[]rsrcMgr.FlowInfo {
//	var flows []rsrcMgr.FlowInfo = []rsrcMgr.FlowInfo{rsrcMgr.FlowInfo{Flow: flow, FlowCategory: flowCategory, FlowStoreCookie: flowStoreCookie}}
//	var intfId uint32
//	/* For flows which trap out of the NNI, the AccessIntfId is invalid
//	   (set to -1). In such cases, we need to refer to the NetworkIntfId .
//	*/
//	if flow.AccessIntfId != -1 {
//		intfId = uint32(flow.AccessIntfId)
//	} else {
//		intfId = uint32(flow.NetworkIntfId)
//	}
//	// Get existing flows matching flowid for given subscriber from KV store
//	existingFlows := f.resourceMgr.GetFlowIDInfo(intfId, uint32(flow.OnuId), uint32(flow.UniId), flow.FlowId)
//	if existingFlows != nil {
//		log.Debugw("Flow exists for given flowID, appending it to current flow", log.Fields{"flowID": flow.FlowId})
//		for _, f := range *existingFlows {
//			flows = append(flows, f)
//		}
//	}
//	log.Debugw("Updated flows for given flowID and onuid", log.Fields{"updatedflow": flows, "flowid": flow.FlowId, "onu": flow.OnuId})
//	return &flows
//}

func (f *OpenOltFlowMgr) updateFlowInfoToKVStore(ctx context.Context, intfID int32, onuID int32, uniID int32, flowID uint32, flows *[]rsrcMgr.FlowInfo) error {
	log.Debugw("Storing flow(s) into KV store", log.Fields{"flows": *flows})
	if err := f.resourceMgr.UpdateFlowIDInfo(ctx, intfID, onuID, uniID, flowID, flows); err != nil {
		log.Debug("Error while Storing flow into KV store")
		return err
	}
	log.Info("Stored flow(s) into KV store successfully!")
	return nil
}

func (f *OpenOltFlowMgr) addFlowToDevice(ctx context.Context, logicalFlow *ofp.OfpFlowStats, deviceFlow *openoltpb2.Flow) bool {

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

	log.Debugw("Sending flow to device via grpc", log.Fields{"flow": *deviceFlow})
	_, err := f.deviceHandler.Client.FlowAdd(context.Background(), deviceFlow)

	st, _ := status.FromError(err)
	if st.Code() == codes.AlreadyExists {
		log.Debug("Flow already exists", log.Fields{"err": err, "deviceFlow": deviceFlow})
		return true
	}

	if err != nil {
		log.Errorw("Failed to Add flow to device", log.Fields{"err": err, "deviceFlow": deviceFlow})
		f.resourceMgr.FreeFlowID(ctx, intfID, deviceFlow.OnuId, deviceFlow.UniId, deviceFlow.FlowId)
		return false
	}
	if deviceFlow.GemportId != -1 {
		// No need to register the flow if it is a trap on nni flow.
		f.registerFlow(ctx, logicalFlow, deviceFlow)
	}
	log.Debugw("Flow added to device successfully ", log.Fields{"flow": *deviceFlow})
	return true
}

func (f *OpenOltFlowMgr) removeFlowFromDevice(deviceFlow *openoltpb2.Flow) bool {
	log.Debugw("Sending flow to device via grpc", log.Fields{"flow": *deviceFlow})
	_, err := f.deviceHandler.Client.FlowRemove(context.Background(), deviceFlow)
	if err != nil {
		if f.deviceHandler.device.ConnectStatus == common.ConnectStatus_UNREACHABLE {
			log.Warnw("Can not remove flow from device since it's unreachable", log.Fields{"err": err, "deviceFlow": deviceFlow})
			//Assume the flow is removed
			return true
		}
		log.Errorw("Failed to Remove flow from device", log.Fields{"err": err, "deviceFlow": deviceFlow})
		return false

	}
	log.Debugw("Flow removed from device successfully ", log.Fields{"flow": *deviceFlow})
	return true
}

/*func register_flow(deviceFlow *openolt_pb2.Flow, logicalFlow *ofp.OfpFlowStats){
 //update core flows_proxy : flows_proxy.update('/', flows)
}

func generateStoredId(flowId uint32, direction string)uint32{

	if direction == Upstream{
		log.Debug("Upstream flow shifting flowid")
		return ((0x1 << 15) | flowId)
	}else if direction == Downstream{
		log.Debug("Downstream flow not shifting flowid")
		return flowId
	}else{
		log.Errorw("Unrecognized direction",log.Fields{"direction": direction})
		return flowId
	}
}

*/

func (f *OpenOltFlowMgr) addLLDPFlow(ctx context.Context, flow *ofp.OfpFlowStats, portNo uint32) {

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

	var networkInterfaceID = IntfIDFromNniPortNum(portNo)
	var flowStoreCookie = getFlowStoreCookie(classifierInfo, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), flowStoreCookie); present {
		log.Debug("Flow-exists--not-re-adding")
		return
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), uint32(gemPortID), flowStoreCookie, "", 0)

	if err != nil {
		log.Errorw("Flow id unavailable for LLDP traponNNI flow", log.Fields{"error": err})
		return
	}
	var classifierProto *openoltpb2.Classifier
	var actionProto *openoltpb2.Action
	if classifierProto = makeOpenOltClassifierField(classifierInfo); classifierProto == nil {
		log.Error("Error in making classifier protobuf for  LLDP trap on nni flow")
		return
	}
	log.Debugw("Created classifier proto", log.Fields{"classifier": *classifierProto})
	if actionProto = makeOpenOltActionField(actionInfo); actionProto == nil {
		log.Error("Error in making action protobuf for LLDP trap on nni flow")
		return
	}
	log.Debugw("Created action proto", log.Fields{"action": *actionProto})

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
	if ok := f.addFlowToDevice(ctx, flow, &downstreamflow); ok {
		log.Debug("LLDP trap on NNI flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(ctx, &downstreamflow, flowStoreCookie, "", flowID, flow.Id)
		if err := f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID),
			int32(onuID),
			int32(uniID),
			flowID, flowsToKVStore); err != nil {
			log.Errorw("Error uploading LLDP flow into KV store", log.Fields{"flow": downstreamflow, "error": err})
		}
	}
	return
}

func getUniPortPath(intfID uint32, onuID int32, uniID int32) string {
	return fmt.Sprintf("pon-{%d}/onu-{%d}/uni-{%d}", intfID, onuID, uniID)
}

//getOnuChildDevice to fetch onu
func (f *OpenOltFlowMgr) getOnuChildDevice(intfID uint32, onuID uint32) (*voltha.Device, error) {
	log.Debugw("GetChildDevice", log.Fields{"pon port": intfID, "onuId": onuID})
	parentPortNo := IntfIDToPortNo(intfID, voltha.Port_PON_OLT)
	onuDevice := f.deviceHandler.GetChildDevice(parentPortNo, onuID)
	if onuDevice == nil {
		log.Errorw("onu not found", log.Fields{"intfId": parentPortNo, "onuId": onuID})
		return nil, errors.New("onu not found")
	}
	log.Debugw("Successfully received child device from core", log.Fields{"child_device": *onuDevice})
	return onuDevice, nil
}

func findNextFlow(flow *ofp.OfpFlowStats) *ofp.OfpFlowStats {
	log.Info("unimplemented flow : %v", flow)
	return nil
}

func (f *OpenOltFlowMgr) clearFlowsAndSchedulerForLogicalPort(childDevice *voltha.Device, logicalPort *voltha.LogicalPort) {
	log.Info("unimplemented device %v, logicalport %v", childDevice, logicalPort)
}

func (f *OpenOltFlowMgr) decodeStoredID(id uint64) (uint64, string) {
	if id>>15 == 0x1 {
		return id & 0x7fff, Upstream
	}
	return id, Downstream
}

func (f *OpenOltFlowMgr) sendDeleteGemPortToChild(intfID uint32, onuID uint32, uniID uint32, gemPortID uint32, tpPath string) error {
	onuDevice, err := f.getOnuChildDevice(intfID, onuID)
	if err != nil {
		log.Errorw("error fetching child device from core", log.Fields{"onuId": onuID})
		return err
	}

	delGemPortMsg := &ic.InterAdapterDeleteGemPortMessage{UniId: uniID, TpPath: tpPath, GemPortId: gemPortID}
	log.Debugw("sending gem port delete to openonu adapter", log.Fields{"msg": *delGemPortMsg})
	if sendErr := f.deviceHandler.AdapterProxy.SendInterAdapterMessage(context.Background(),
		delGemPortMsg,
		ic.InterAdapterMessageType_DELETE_GEM_PORT_REQUEST,
		f.deviceHandler.deviceType,
		onuDevice.Type,
		onuDevice.Id,
		onuDevice.ProxyAddress.DeviceId, ""); sendErr != nil {
		log.Errorw("failure sending del gem port to onu adapter", log.Fields{"fromAdapter": f.deviceHandler.deviceType,
			"toAdapter": onuDevice.Type, "onuId": onuDevice.Id,
			"proxyDeviceId": onuDevice.ProxyAddress.DeviceId})
		return sendErr
	}
	log.Debugw("success sending del gem port to onu adapter", log.Fields{"msg": delGemPortMsg})
	return nil
}

func (f *OpenOltFlowMgr) sendDeleteTcontToChild(intfID uint32, onuID uint32, uniID uint32, allocID uint32, tpPath string) error {
	onuDevice, err := f.getOnuChildDevice(intfID, onuID)
	if err != nil {
		log.Errorw("error fetching child device from core", log.Fields{"onuId": onuID})
		return err
	}

	delTcontMsg := &ic.InterAdapterDeleteTcontMessage{UniId: uniID, TpPath: tpPath, AllocId: allocID}
	log.Debugw("sending tcont delete to openonu adapter", log.Fields{"msg": *delTcontMsg})
	if sendErr := f.deviceHandler.AdapterProxy.SendInterAdapterMessage(context.Background(),
		delTcontMsg,
		ic.InterAdapterMessageType_DELETE_TCONT_REQUEST,
		f.deviceHandler.deviceType,
		onuDevice.Type,
		onuDevice.Id,
		onuDevice.ProxyAddress.DeviceId, ""); sendErr != nil {
		log.Errorw("failure sending del tcont to onu adapter", log.Fields{"fromAdapter": f.deviceHandler.deviceType,
			"toAdapter": onuDevice.Type, "onuId": onuDevice.Id,
			"proxyDeviceId": onuDevice.ProxyAddress.DeviceId})
		return sendErr
	}
	log.Debugw("success sending del tcont to onu adapter", log.Fields{"msg": delTcontMsg})
	return nil
}

func (f *OpenOltFlowMgr) deletePendingFlows(Intf uint32, onuID int32, uniID int32) {
	pnFlDelKey := pendingFlowDeleteKey{Intf, uint32(onuID), uint32(uniID)}
	if val, ok := f.pendingFlowDelete.Load(pnFlDelKey); ok {
		if val.(int) > 0 {
			pnFlDels := val.(int) - 1
			if pnFlDels > 0 {
				log.Debugw("flow delete succeeded, more pending",
					log.Fields{"intf": Intf, "onuID": onuID, "uniID": uniID, "currPendingFlowCnt": pnFlDels})
				f.pendingFlowDelete.Store(pnFlDelKey, pnFlDels)
			} else {
				log.Debugw("all pending flow deletes handled, removing entry from map",
					log.Fields{"intf": Intf, "onuID": onuID, "uniID": uniID})
				f.pendingFlowDelete.Delete(pnFlDelKey)
			}
		}
	} else {
		log.Debugw("no pending delete flows found",
			log.Fields{"intf": Intf, "onuID": onuID, "uniID": uniID})

	}

}

// Once the gemport is released for a given onu, it also has to be cleared from local cache
// which was used for deriving the gemport->logicalPortNo during packet-in.
// Otherwise stale info continues to exist after gemport is freed and wrong logicalPortNo
// is conveyed to ONOS during packet-in OF message.
func (f *OpenOltFlowMgr) deleteGemPortFromLocalCache(intfID uint32, onuID uint32, gemPortID uint32) {
	f.lockCache.Lock()
	defer f.lockCache.Unlock()
	onugem := f.onuGemInfo[intfID]
	for _, onu := range onugem {
		if onu.OnuID == onuID {
			for i, gem := range onu.GemPorts {
				// If the gemport is found, delete it from local cache.
				if gem == gemPortID {
					onu.GemPorts = append(onu.GemPorts[:i], onu.GemPorts[i+1:]...)
					log.Debugw("removed gemport from local cache",
						log.Fields{"intfID": intfID, "onuID": onuID, "gemPortID": gemPortID})
					break
				}
			}
			break
		}
	}
}

//clearResources clears pon resources in kv store and the device
func (f *OpenOltFlowMgr) clearResources(ctx context.Context, flow *ofp.OfpFlowStats, Intf uint32, onuID int32, uniID int32,
	gemPortID int32, flowID uint32, flowDirection string,
	portNum uint32, updatedFlows []rsrcMgr.FlowInfo) error {

	tpID, err := getTpIDFromFlow(flow)
	if err != nil {
		log.Error("metadata-is-not-present-invalid-flow-to-process", log.Fields{"pon": Intf, "onuID": onuID, "uniID": uniID})
		return err
	}

	if len(updatedFlows) >= 0 {
		// There are still flows referencing the same flow_id.
		// So the flow should not be freed yet.
		// For ex: Case of HSIA where same flow is shared
		// between DS and US.
		f.updateFlowInfoToKVStore(ctx, int32(Intf), int32(onuID), int32(uniID), flowID, &updatedFlows)
		if len(updatedFlows) == 0 {
			// Do this for subscriber flows only (not trap from NNI flows)
			if onuID != -1 && uniID != -1 {
				pnFlDelKey := pendingFlowDeleteKey{Intf, uint32(onuID), uint32(uniID)}
				if val, ok := f.pendingFlowDelete.Load(pnFlDelKey); !ok {
					log.Debugw("creating entry for pending flow delete",
						log.Fields{"intf": Intf, "onuID": onuID, "uniID": uniID})
					f.pendingFlowDelete.Store(pnFlDelKey, 1)
				} else {
					pnFlDels := val.(int) + 1
					log.Debugw("updating flow delete entry",
						log.Fields{"intf": Intf, "onuID": onuID, "uniID": uniID, "currPendingFlowCnt": pnFlDels})
					f.pendingFlowDelete.Store(pnFlDelKey, pnFlDels)
				}

				defer f.deletePendingFlows(Intf, onuID, uniID)
			}

			log.Debugw("Releasing flow Id to resource manager", log.Fields{"Intf": Intf, "onuId": onuID, "uniId": uniID, "flowId": flowID})
			f.resourceMgr.FreeFlowID(ctx, Intf, int32(onuID), int32(uniID), flowID)

			uni := getUniPortPath(Intf, onuID, uniID)
			tpPath := f.getTPpath(Intf, uni, tpID)
			log.Debugw("Getting-techprofile-instance-for-subscriber", log.Fields{"TP-PATH": tpPath})
			techprofileInst, err := f.techprofile[Intf].GetTPInstanceFromKVStore(ctx, tpID, tpPath)
			if err != nil { // This should not happen, something wrong in KV backend transaction
				log.Errorw("Error in fetching tech profile instance from KV store", log.Fields{"tpID": 20, "path": tpPath})
				return err
			}
			if techprofileInst == nil {
				log.Errorw("Tech-profile-instance-does-not-exist-in-KV Store", log.Fields{"tpPath": tpPath})
				return err
			}

			gemPK := gemPortKey{Intf, uint32(gemPortID)}
			if f.isGemPortUsedByAnotherFlow(gemPK) {
				flowIDs := f.flowsUsedByGemPort[gemPK]
				for i, flowIDinMap := range flowIDs {
					if flowIDinMap == flowID {
						flowIDs = append(flowIDs[:i], flowIDs[i+1:]...)
						// everytime flowsUsedByGemPort cache is updated the same should be updated
						// in kv store by calling UpdateFlowIDsForGem
						f.flowsUsedByGemPort[gemPK] = flowIDs
						f.resourceMgr.UpdateFlowIDsForGem(ctx, Intf, uint32(gemPortID), flowIDs)
						break
					}
				}
				log.Debugw("Gem port id is still used by other flows", log.Fields{"gemPortID": gemPortID, "usedByFlows": flowIDs})
				return nil
			}
			log.Debugf("Gem port id %d is not used by another flow - releasing the gem port", gemPortID)
			f.resourceMgr.RemoveGemPortIDForOnu(ctx, Intf, uint32(onuID), uint32(uniID), uint32(gemPortID))
			// TODO: The TrafficQueue corresponding to this gem-port also should be removed immediately.
			// But it is anyway eventually  removed later when the TechProfile is freed, so not a big issue for now.
			f.resourceMgr.RemoveGEMportPonportToOnuMapOnKVStore(ctx, uint32(gemPortID), Intf)
			f.deleteGemPortFromLocalCache(Intf, uint32(onuID), uint32(gemPortID))
			f.onuIdsLock.Lock()
			//everytime an entry is deleted from flowsUsedByGemPort cache, the same should be updated in kv as well
			// by calling DeleteFlowIDsForGem
			delete(f.flowsUsedByGemPort, gemPK)
			f.resourceMgr.DeleteFlowIDsForGem(ctx, Intf, uint32(gemPortID))
			f.resourceMgr.FreeGemPortID(ctx, Intf, uint32(onuID), uint32(uniID), uint32(gemPortID))
			f.onuIdsLock.Unlock()
			// Delete the gem port on the ONU.
			if err := f.sendDeleteGemPortToChild(Intf, uint32(onuID), uint32(uniID), uint32(gemPortID), tpPath); err != nil {
				log.Errorw("error processing delete gem-port towards onu",
					log.Fields{"err": err, "pon": Intf, "onuID": onuID, "uniID": uniID, "gemPortId": gemPortID})
			}

			ok, _ := f.isTechProfileUsedByAnotherGem(ctx, Intf, uint32(onuID), uint32(uniID), tpID, techprofileInst, uint32(gemPortID))
			if !ok {
				f.resourceMgr.RemoveTechProfileIDForOnu(ctx, Intf, uint32(onuID), uint32(uniID), tpID)
				f.RemoveSchedulerQueues(ctx, schedQueue{direction: tp_pb.Direction_UPSTREAM, intfID: Intf, onuID: uint32(onuID), uniID: uint32(uniID), tpID: tpID, uniPort: portNum, tpInst: techprofileInst})
				f.RemoveSchedulerQueues(ctx, schedQueue{direction: tp_pb.Direction_DOWNSTREAM, intfID: Intf, onuID: uint32(onuID), uniID: uint32(uniID), tpID: tpID, uniPort: portNum, tpInst: techprofileInst})
				f.DeleteTechProfileInstance(ctx, Intf, uint32(onuID), uint32(uniID), "", tpID)
				f.resourceMgr.FreeAllocID(ctx, Intf, uint32(onuID), uint32(uniID), techprofileInst.UsScheduler.AllocID)
				// Delete the TCONT on the ONU.
				if err := f.sendDeleteTcontToChild(Intf, uint32(onuID), uint32(uniID), uint32(techprofileInst.UsScheduler.AllocID), tpPath); err != nil {
					log.Errorw("error processing delete tcont towards onu",
						log.Fields{"pon": Intf, "onuID": onuID, "uniID": uniID, "allocId": techprofileInst.UsScheduler.AllocID})
				}
			}
		}
	}
	return nil
}

func (f *OpenOltFlowMgr) clearFlowFromResourceManager(ctx context.Context, flow *ofp.OfpFlowStats, flowDirection string) {

	log.Debugw("clearFlowFromResourceManager", log.Fields{"flowDirection": flowDirection, "flow": *flow})

	if flowDirection == Multicast {
		f.clearMulticastFlowFromResourceManager(ctx, flow)
		return
	}

	var updatedFlows []rsrcMgr.FlowInfo
	var flowID uint32
	var onuID, uniID int32
	classifierInfo := make(map[string]interface{})

	portNum, Intf, onu, uni, inPort, ethType, err := FlowExtractInfo(flow, flowDirection)
	if err != nil {
		log.Error(err)
		return
	}

	onuID = int32(onu)
	uniID = int32(uni)

	for _, field := range flows.GetOfbFields(flow) {
		if field.Type == flows.IP_PROTO {
			classifierInfo[IPProto] = field.GetIpProto()
			log.Debug("field-type-ip-proto", log.Fields{"classifierInfo[IP_PROTO]": classifierInfo[IPProto].(uint32)})
		}
	}
	log.Debugw("Extracted access info from flow to be deleted",
		log.Fields{"ponIntf": Intf, "onuID": onuID, "uniID": uniID})

	if ethType == LldpEthType || ((classifierInfo[IPProto] == IPProtoDhcp) && (flowDirection == "downstream")) {
		onuID = -1
		uniID = -1
		log.Debug("Trap on nni flow set oni, uni to -1")
		Intf = IntfIDFromNniPortNum(inPort)
	}
	flowIds := f.resourceMgr.GetCurrentFlowIDsForOnu(ctx, Intf, onuID, uniID)
	for _, flowID = range flowIds {
		flowInfo := f.resourceMgr.GetFlowIDInfo(ctx, Intf, onuID, uniID, flowID)
		if flowInfo == nil {
			log.Debugw("No FlowInfo found found in KV store",
				log.Fields{"Intf": Intf, "onuID": onuID, "uniID": uniID, "flowID": flowID})
			return
		}
		updatedFlows = nil
		for _, flow := range *flowInfo {
			updatedFlows = append(updatedFlows, flow)
		}

		for i, storedFlow := range updatedFlows {
			if flow.Id == storedFlow.LogicalFlowID {
				removeFlowMessage := openoltpb2.Flow{FlowId: storedFlow.Flow.FlowId, FlowType: storedFlow.Flow.FlowType}
				log.Debugw("Flow to be deleted", log.Fields{"flow": storedFlow})
				if ok := f.removeFlowFromDevice(&removeFlowMessage); ok {
					log.Debug("Flow removed from device successfully")
					//Remove the Flow from FlowInfo
					updatedFlows = append(updatedFlows[:i], updatedFlows[i+1:]...)
					err = f.clearResources(ctx, flow, Intf, onuID, uniID, storedFlow.Flow.GemportId,
						flowID, flowDirection, portNum, updatedFlows)
					if err != nil {
						log.Error("Failed to clear resources for flow", log.Fields{"flow": storedFlow})
						return
					}
				} else {
					log.Error("Failed to remove flow from device")
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
	formulateClassifierInfoFromFlow(classifierInfo, flow)
	inPort, err := f.getInPortOfMulticastFlow(ctx, classifierInfo)

	if err != nil {
		log.Warnw("No inPort found. Cannot release resources of the multicast flow.", log.Fields{"flowId:": flow.Id})
		return
	}

	networkInterfaceID := IntfIDFromNniPortNum(inPort)
	var onuID = int32(NoneOnuID)
	var uniID = int32(NoneUniID)
	var flowID uint32
	var updatedFlows []rsrcMgr.FlowInfo

	flowIds := f.resourceMgr.GetCurrentFlowIDsForOnu(ctx, networkInterfaceID, onuID, uniID)

	for _, flowID = range flowIds {
		flowInfo := f.resourceMgr.GetFlowIDInfo(ctx, networkInterfaceID, onuID, uniID, flowID)
		if flowInfo == nil {
			log.Debugw("No multicast FlowInfo found in the KV store",
				log.Fields{"Intf": networkInterfaceID, "onuID": onuID, "uniID": uniID, "flowID": flowID})
			continue
		}
		updatedFlows = nil
		for _, flow := range *flowInfo {
			updatedFlows = append(updatedFlows, flow)
		}
		for i, storedFlow := range updatedFlows {
			if flow.Id == storedFlow.LogicalFlowID {
				removeFlowMessage := openoltpb2.Flow{FlowId: storedFlow.Flow.FlowId, FlowType: storedFlow.Flow.FlowType}
				log.Debugw("Multicast flow to be deleted", log.Fields{"flow": storedFlow})
				//remove from device
				if ok := f.removeFlowFromDevice(&removeFlowMessage); !ok {
					log.Errorw("Failed to remove multicast flow from device", log.Fields{"flowId": flow.Id})
					return
				}
				log.Debugw("Multicast flow removed from device successfully", log.Fields{"flowId": flow.Id})
				//Remove the Flow from FlowInfo
				updatedFlows = append(updatedFlows[:i], updatedFlows[i+1:]...)
				if err := f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID), NoneOnuID, NoneUniID, flowID, &updatedFlows); err != nil {
					log.Error("Failed to delete multicast flow from the KV store", log.Fields{"flow": storedFlow, "err": err})
					return
				}
				//release flow id
				log.Debugw("Releasing multicast flow id", log.Fields{"flowId": flowID, "interfaceID": networkInterfaceID})
				f.resourceMgr.FreeFlowID(ctx, uint32(networkInterfaceID), NoneOnuID, NoneUniID, flowID)
			}
		}
	}
}

//RemoveFlow removes the flow from the device
func (f *OpenOltFlowMgr) RemoveFlow(ctx context.Context, flow *ofp.OfpFlowStats) {
	log.Debugw("Removing Flow", log.Fields{"flow": flow})
	var direction string
	actionInfo := make(map[string]interface{})

	for _, action := range flows.GetActions(flow) {
		if action.Type == flows.OUTPUT {
			if out := action.GetOutput(); out != nil {
				actionInfo[Output] = out.GetPort()
				log.Debugw("action-type-output", log.Fields{"out_port": actionInfo[Output].(uint32)})
			} else {
				log.Error("Invalid output port in action")
				return
			}
		}
	}

	if flows.HasGroup(flow) {
		direction = Multicast
	} else if IsUpstream(actionInfo[Output].(uint32)) {
		direction = Upstream
	} else {
		direction = Downstream
	}
	f.clearFlowFromResourceManager(ctx, flow, direction) //TODO: Take care of the limitations

	return
}

func (f *OpenOltFlowMgr) waitForFlowDeletesToCompleteForOnu(ctx context.Context, intfID uint32, onuID uint32,
	uniID uint32, ch chan bool) {
	pnFlDelKey := pendingFlowDeleteKey{intfID, onuID, uniID}
	for {
		select {
		case <-time.After(20 * time.Millisecond):
			if flowDelRefCnt, ok := f.pendingFlowDelete.Load(pnFlDelKey); !ok || flowDelRefCnt == 0 {
				log.Debug("pending flow deletes completed")
				ch <- true
				return
			}
		case <-ctx.Done():
			log.Error("flow delete wait handler routine canceled")
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
func (f *OpenOltFlowMgr) AddFlow(ctx context.Context, flow *ofp.OfpFlowStats, flowMetadata *voltha.FlowMetadata) {
	classifierInfo := make(map[string]interface{})
	actionInfo := make(map[string]interface{})
	var UsMeterID uint32
	var DsMeterID uint32

	log.Debug("Adding Flow", log.Fields{"flow": flow, "flowMetadata": flowMetadata})
	formulateClassifierInfoFromFlow(classifierInfo, flow)

	err := formulateActionInfoFromFlow(actionInfo, classifierInfo, flow)
	if err != nil {
		// Error logging is already done in the called function
		// So just return in case of error
		return
	}

	if flows.HasGroup(flow) {
		// handle multicast flow
		f.handleFlowWithGroup(ctx, actionInfo, classifierInfo, flow)
		return
	}

	/* Controller bound trap flows */
	err = formulateControllerBoundTrapFlowInfo(actionInfo, classifierInfo, flow)
	if err != nil {
		// error if any, already logged in the called function
		return
	}

	log.Infow("Flow ports", log.Fields{"classifierInfo_inport": classifierInfo[InPort], "action_output": actionInfo[Output]})
	portNo, intfID, onuID, uniID := ExtractAccessFromFlow(classifierInfo[InPort].(uint32), actionInfo[Output].(uint32))

	if ethType, ok := classifierInfo[EthType]; ok {
		if ethType.(uint32) == LldpEthType {
			log.Info("Adding LLDP flow")
			f.addLLDPFlow(ctx, flow, portNo)
			return
		}
	}
	if ipProto, ok := classifierInfo[IPProto]; ok {
		if ipProto.(uint32) == IPProtoDhcp {
			if udpSrc, ok := classifierInfo[UDPSrc]; ok {
				if udpSrc.(uint32) == uint32(67) || udpSrc.(uint32) == uint32(546) {
					log.Debug("trap-dhcp-from-nni-flow")
					f.addDHCPTrapFlowOnNNI(ctx, flow, classifierInfo, portNo)
					return
				}
			}
		}
	}
	if isIgmpTrapDownstreamFlow(classifierInfo) {
		log.Debug("trap-igmp-from-nni-flow")
		f.addIgmpTrapFlowOnNNI(ctx, flow, classifierInfo, portNo)
		return
	}

	f.deviceHandler.AddUniPortToOnu(intfID, onuID, portNo)
	f.resourceMgr.AddUniPortToOnuInfo(ctx, intfID, onuID, portNo)

	TpID, err := getTpIDFromFlow(flow)
	if err != nil {
		log.Error("metadata-is-not-present-invalid-flow-to-process", log.Fields{"pon": intfID, "onuID": onuID, "uniID": uniID})
		return
	}
	log.Debugw("TPID for this subcriber", log.Fields{"TpId": TpID, "pon": intfID, "onuID": onuID, "uniID": uniID})
	if IsUpstream(actionInfo[Output].(uint32)) {
		UsMeterID = flows.GetMeterIdFromFlow(flow)
		log.Debugw("Upstream-flow-meter-id", log.Fields{"UsMeterID": UsMeterID})
	} else {
		DsMeterID = flows.GetMeterIdFromFlow(flow)
		log.Debugw("Downstream-flow-meter-id", log.Fields{"DsMeterID": DsMeterID})

	}

	pnFlDelKey := pendingFlowDeleteKey{intfID, onuID, uniID}
	if _, ok := f.pendingFlowDelete.Load(pnFlDelKey); !ok {
		log.Debugw("no pending flows found, going ahead with flow install", log.Fields{"pon": intfID, "onuID": onuID, "uniID": uniID})
		f.divideAndAddFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, uint32(TpID), UsMeterID, DsMeterID, flowMetadata)
	} else {
		pendingFlowDelComplete := make(chan bool)
		go f.waitForFlowDeletesToCompleteForOnu(ctx, intfID, onuID, uniID, pendingFlowDelComplete)
		select {
		case <-pendingFlowDelComplete:
			log.Debugw("all pending flow deletes completed", log.Fields{"pon": intfID, "onuID": onuID, "uniID": uniID})
			f.divideAndAddFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, uint32(TpID), UsMeterID, DsMeterID, flowMetadata)

		case <-time.After(10 * time.Second):
			log.Errorw("pending flow deletes not completed after timeout", log.Fields{"pon": intfID, "onuID": onuID, "uniID": uniID})
		}
	}
}

// handleFlowWithGroup adds multicast flow to the device.
func (f *OpenOltFlowMgr) handleFlowWithGroup(ctx context.Context, actionInfo, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) {
	classifierInfo[PacketTagType] = DoubleTag
	log.Debugw("add-multicast-flow", log.Fields{"classifierInfo": classifierInfo, "actionInfo": actionInfo})

	inPort, err := f.getInPortOfMulticastFlow(ctx, classifierInfo)
	if err != nil {
		log.Warnw("No inPort found. Ignoring multicast flow.", log.Fields{"flowId:": flow.Id})
		return
	}
	//replace ipDst with ethDst
	if ipv4Dst, ok := classifierInfo[Ipv4Dst]; ok &&
		flows.IsMulticastIp(ipv4Dst.(uint32)) {
		// replace ipv4_dst classifier with eth_dst
		multicastMac := flows.ConvertToMulticastMacBytes(ipv4Dst.(uint32))
		delete(classifierInfo, Ipv4Dst)
		delete(classifierInfo, EthType)
		classifierInfo[EthDst] = multicastMac
		log.Debugw("multicast-ip-to-mac-conversion-success", log.Fields{"ip:": ipv4Dst.(uint32), "mac:": multicastMac})
	}

	var onuID = NoneOnuID
	var uniID = NoneUniID
	var gemPortID = NoneGemPortID

	networkInterfaceID := IntfIDFromNniPortNum(inPort)

	var flowStoreCookie = getFlowStoreCookie(classifierInfo, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), flowStoreCookie); present {
		log.Debugw("multicast-flow-exists--not-re-adding", log.Fields{"classifierInfo": classifierInfo})
		return
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), uint32(gemPortID), flowStoreCookie, "", 0, 0)
	if err != nil {
		log.Errorw("Flow id unavailable for multicast flow", log.Fields{"error": err})
		return
	}
	var classifierProto *openoltpb2.Classifier
	if classifierProto = makeOpenOltClassifierField(classifierInfo); classifierProto == nil {
		log.Error("Error in making classifier protobuf for multicast flow")
		return
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

	if ok := f.addFlowToDevice(ctx, flow, &multicastFlow); ok {
		log.Debug("multicast flow added to device successfully")
		//get cached group
		group, _, err := f.GetFlowGroupFromKVStore(ctx, groupID, true)
		if err == nil {
			//calling groupAdd to set group members after multicast flow creation
			if f.ModifyGroup(ctx, group) {
				//cached group can be removed now
				f.resourceMgr.RemoveFlowGroupFromKVStore(ctx, groupID, true)
			}
		}

		flowsToKVStore := f.getUpdatedFlowInfo(ctx, &multicastFlow, flowStoreCookie, MulticastFlow, flowID, flow.Id)
		if err := f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID),
			int32(onuID),
			int32(uniID),
			flowID, flowsToKVStore); err != nil {
			log.Errorw("Error uploading multicast flow into KV store", log.Fields{"flow": multicastFlow, "error": err})
		}
	}
	return
}

//getInPortOfMulticastFlow return inPort criterion if exists; returns NNI interface of the device otherwise
func (f *OpenOltFlowMgr) getInPortOfMulticastFlow(ctx context.Context, classifierInfo map[string]interface{}) (uint32, error) {
	if _, ok := classifierInfo[InPort]; ok {
		return classifierInfo[InPort].(uint32), nil
	}
	// find first NNI port of the device
	nniPorts, e := f.resourceMgr.GetNNIFromKVStore(ctx)
	if e == nil && len(nniPorts) > 0 {
		return nniPorts[0], nil
	}
	return 0, errors.New("cannot find NNI port of device")
}

// AddGroup add or update the group
func (f *OpenOltFlowMgr) AddGroup(ctx context.Context, group *ofp.OfpGroupEntry) {
	log.Infow("add-group", log.Fields{"group": group})
	if group == nil {
		log.Warn("skipping nil group")
		return
	}

	groupToOlt := openoltpb2.Group{
		GroupId: group.Desc.GroupId,
		Command: openoltpb2.Group_SET_MEMBERS,
		Action:  f.buildGroupAction(),
	}

	log.Debugw("Sending group to device", log.Fields{"groupToOlt": groupToOlt})
	_, err := f.deviceHandler.Client.PerformGroupOperation(ctx, &groupToOlt)
	if err != nil {
		log.Errorw("add-group operation failed", log.Fields{"err": err, "groupToOlt": groupToOlt})
		return
	}
	// group members not created yet. So let's store the group
	if err := f.resourceMgr.AddFlowGroupToKVStore(ctx, group, true); err != nil {
		log.Errorw("Group cannot be stored in KV store", log.Fields{"groupId": group.Desc.GroupId, "err": err})
	} else {
		log.Debugw("add-group operation performed on the device successfully ", log.Fields{"groupToOlt": groupToOlt})
	}
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
func (f *OpenOltFlowMgr) ModifyGroup(ctx context.Context, group *ofp.OfpGroupEntry) bool {
	log.Infow("modify-group", log.Fields{"group": group})
	if group == nil || group.Desc == nil {
		log.Warn("cannot modify group; group is nil")
		return false
	}

	new := f.buildGroup(group.Desc.GroupId, group.Desc.Buckets)
	//get existing members of the group
	val, groupExists, err := f.GetFlowGroupFromKVStore(ctx, group.Desc.GroupId, false)

	if err != nil {
		log.Errorw("Failed to retrieve the group from the store. Cannot modify group.",
			log.Fields{"groupId": group.Desc.GroupId, "err": err})
		return false
	}

	var current *openoltpb2.Group // represents the group on the device
	if groupExists {
		// group already exists
		current = f.buildGroup(group.Desc.GroupId, val.Desc.GetBuckets())
		log.Debugw("modify-group: group exists.", log.Fields{"group on the device": val, "new": group})
	} else {
		current = f.buildGroup(group.Desc.GroupId, nil)
	}

	log.Debugw("modify-group: comparing current and new.", log.Fields{"group on the device": current, "new": new})
	// get members to be added
	membersToBeAdded := f.findDiff(current, new)
	// get members to be removed
	membersToBeRemoved := f.findDiff(new, current)

	log.Infow("modify-group -> differences found", log.Fields{"membersToBeAdded": membersToBeAdded,
		"membersToBeRemoved": membersToBeRemoved, "groupId": group.Desc.GroupId})

	groupToOlt := openoltpb2.Group{
		GroupId: group.Desc.GroupId,
	}
	var added, removed = true, true
	if membersToBeAdded != nil && len(membersToBeAdded) > 0 {
		groupToOlt.Command = openoltpb2.Group_ADD_MEMBERS
		groupToOlt.Members = membersToBeAdded
		//execute addMembers
		added = f.callGroupAddRemove(&groupToOlt)
	}
	if membersToBeRemoved != nil && len(membersToBeRemoved) > 0 {
		groupToOlt.Command = openoltpb2.Group_REMOVE_MEMBERS
		groupToOlt.Members = membersToBeRemoved
		//execute removeMembers
		removed = f.callGroupAddRemove(&groupToOlt)
	}

	//save the modified group
	if added && removed {
		if err := f.resourceMgr.AddFlowGroupToKVStore(ctx, group, false); err != nil {
			log.Errorw("Failed to save the group into kv store", log.Fields{"groupId": group.Desc.GroupId})
		}
		log.Debugw("modify-group was success. Storing the group", log.Fields{"group": group, "existingGroup": current})
	} else {
		log.Warnw("One of the group add/remove operations has failed. Cannot save group modifications",
			log.Fields{"group": group})
	}
	return added && removed
}

//callGroupAddRemove performs add/remove buckets operation for the indicated group
func (f *OpenOltFlowMgr) callGroupAddRemove(group *openoltpb2.Group) bool {
	if err := f.performGroupOperation(group); err != nil {
		st, _ := status.FromError(err)
		//ignore already exists error code
		if st.Code() != codes.AlreadyExists {
			return false
		}
	}
	return true
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
func (f *OpenOltFlowMgr) performGroupOperation(group *openoltpb2.Group) error {
	log.Debugw("Sending group to device", log.Fields{"groupToOlt": group, "command": group.Command})
	_, err := f.deviceHandler.Client.PerformGroupOperation(context.Background(), group)
	if err != nil {
		log.Errorw("group operation failed", log.Fields{"err": err, "groupToOlt": group})
	}
	return err
}

//buildGroup build openoltpb2.Group from given group id and bucket list
func (f *OpenOltFlowMgr) buildGroup(groupID uint32, buckets []*ofp.OfpBucket) *openoltpb2.Group {
	group := openoltpb2.Group{
		GroupId: groupID}
	// create members of the group
	if buckets != nil {
		for _, ofBucket := range buckets {
			member := f.buildMember(ofBucket)
			if member != nil && !f.contains(group.Members, member) {
				group.Members = append(group.Members, member)
			}
		}
	}
	return &group
}

//buildMember builds openoltpb2.GroupMember from an OpenFlow bucket
func (f *OpenOltFlowMgr) buildMember(ofBucket *ofp.OfpBucket) *openoltpb2.GroupMember {
	var outPort uint32
	outPortFound := false
	for _, ofAction := range ofBucket.Actions {
		if ofAction.Type == ofp.OfpActionType_OFPAT_OUTPUT {
			outPort = ofAction.GetOutput().Port
			outPortFound = true
		}
	}

	if !outPortFound {
		log.Debugw("bucket skipped since no out port found in it",
			log.Fields{"ofBucket": ofBucket})
		return nil
	}
	interfaceID := IntfIDFromUniPortNum(outPort)
	log.Debugw("got associated interface id of the port", log.Fields{"portNumber:": outPort, "interfaceId:": interfaceID})
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
	log.Warnf("bucket skipped since interface-2-gem mapping cannot be found",
		log.Fields{"ofBucket": ofBucket})
	return nil
}

//sendTPDownloadMsgToChild send payload
func (f *OpenOltFlowMgr) sendTPDownloadMsgToChild(intfID uint32, onuID uint32, uniID uint32, uni string, TpID uint32) error {

	onuDevice, err := f.getOnuChildDevice(intfID, onuID)
	if err != nil {
		log.Errorw("Error while fetching Child device from core", log.Fields{"onuId": onuID})
		return err
	}
	log.Debugw("Got child device from OLT device handler", log.Fields{"device": *onuDevice})

	tpPath := f.getTPpath(intfID, uni, TpID)
	tpDownloadMsg := &ic.InterAdapterTechProfileDownloadMessage{UniId: uniID, Path: tpPath}
	log.Infow("Sending Load-tech-profile-request-to-brcm-onu-adapter", log.Fields{"msg": *tpDownloadMsg})
	sendErr := f.deviceHandler.AdapterProxy.SendInterAdapterMessage(context.Background(),
		tpDownloadMsg,
		ic.InterAdapterMessageType_TECH_PROFILE_DOWNLOAD_REQUEST,
		f.deviceHandler.deviceType,
		onuDevice.Type,
		onuDevice.Id,
		onuDevice.ProxyAddress.DeviceId, "")
	if sendErr != nil {
		log.Errorw("send techprofile-download request error", log.Fields{"fromAdapter": f.deviceHandler.deviceType,
			"toAdapter": onuDevice.Type, "onuId": onuDevice.Id,
			"proxyDeviceId": onuDevice.ProxyAddress.DeviceId})
		return sendErr
	}
	log.Debugw("success Sending Load-tech-profile-request-to-brcm-onu-adapter", log.Fields{"msg": tpDownloadMsg})
	return nil
}

//UpdateOnuInfo function adds onu info to cache and kvstore
func (f *OpenOltFlowMgr) UpdateOnuInfo(ctx context.Context, intfID uint32, onuID uint32, serialNum string) {

	f.lockCache.Lock()
	defer f.lockCache.Unlock()
	onu := rsrcMgr.OnuGemInfo{OnuID: onuID, SerialNumber: serialNum, IntfID: intfID}
	f.onuGemInfo[intfID] = append(f.onuGemInfo[intfID], onu)
	if err := f.resourceMgr.AddOnuInfo(ctx, intfID, onu); err != nil {
		log.Errorw("failed to add onu info", log.Fields{"onu": onu})
		return
	}
	log.Debugw("Updated onuinfo", log.Fields{"intfID": intfID, "onuID": onuID, "serialNum": serialNum})
}

//addGemPortToOnuInfoMap function adds GEMport to ONU map
func (f *OpenOltFlowMgr) addGemPortToOnuInfoMap(ctx context.Context, intfID uint32, onuID uint32, gemPort uint32) {
	f.lockCache.Lock()
	defer f.lockCache.Unlock()
	onugem := f.onuGemInfo[intfID]
	// update the gem to the local cache as well as to kv strore
	for idx, onu := range onugem {
		if onu.OnuID == onuID {
			// check if gem already exists , else update the cache and kvstore
			for _, gem := range onu.GemPorts {
				if gem == gemPort {
					log.Debugw("Gem already in cache, no need to update cache and kv store",
						log.Fields{"gem": gemPort})
					return
				}
			}
			onugem[idx].GemPorts = append(onugem[idx].GemPorts, gemPort)
			f.onuGemInfo[intfID] = onugem
		}
	}
	err := f.resourceMgr.AddGemToOnuGemInfo(ctx, intfID, onuID, gemPort)
	if err != nil {
		log.Errorw("Failed to add gem to onu", log.Fields{"intfId": intfID, "onuId": onuID, "gemPort": gemPort})
		return
	}
}

// This function Lookup maps  by serialNumber or (intfId, gemPort)

//getOnuIDfromGemPortMap Returns OnuID,nil if found or set 0,error if no onuId is found for serialNumber or (intfId, gemPort)
func (f *OpenOltFlowMgr) getOnuIDfromGemPortMap(serialNumber string, intfID uint32, gemPortID uint32) (uint32, error) {

	f.lockCache.Lock()
	defer f.lockCache.Unlock()

	log.Debugw("Getting ONU ID from GEM port and PON port", log.Fields{"serialNumber": serialNumber, "intfId": intfID, "gemPortId": gemPortID})
	// get onuid from the onugem info cache
	onugem := f.onuGemInfo[intfID]
	for _, onu := range onugem {
		for _, gem := range onu.GemPorts {
			if gem == gemPortID {
				return onu.OnuID, nil
			}
		}
	}
	log.Errorw("onuid is not found", log.Fields{"serialNumber": serialNumber, "intfId": intfID, "gemPort": gemPortID})
	return uint32(0), errors.New("key error, onuid is not found") // ONU ID 0 is not a valid one
}

//GetLogicalPortFromPacketIn function computes logical port UNI/NNI port from packet-in indication and returns the same
func (f *OpenOltFlowMgr) GetLogicalPortFromPacketIn(ctx context.Context, packetIn *openoltpb2.PacketIndication) (uint32, error) {
	var logicalPortNum uint32
	var onuID uint32
	var err error

	if packetIn.IntfType == "pon" {
		// packet indication does not have serial number , so sending as nil
		if onuID, err = f.getOnuIDfromGemPortMap("", packetIn.IntfId, packetIn.GemportId); err != nil {
			log.Errorw("Unable to get ONU ID from GEM/PON port", log.Fields{"pon port": packetIn.IntfId, "gemport": packetIn.GemportId})
			return logicalPortNum, err
		}
		if packetIn.PortNo != 0 {
			logicalPortNum = packetIn.PortNo
		} else {
			uniID := uint32(0) //  FIXME - multi-uni support
			logicalPortNum = MkUniPortNum(packetIn.IntfId, onuID, uniID)
		}
		// Store the gem port through which the packet_in came. Use the same gem port for packet_out
		f.UpdateGemPortForPktIn(ctx, packetIn.IntfId, onuID, logicalPortNum, packetIn.GemportId)
	} else if packetIn.IntfType == "nni" {
		logicalPortNum = IntfIDToPortNo(packetIn.IntfId, voltha.Port_ETHERNET_NNI)
	}
	log.Debugw("Retrieved logicalport from  packet-in", log.Fields{
		"logicalPortNum": logicalPortNum,
		"IntfType":       packetIn.IntfType,
		"packet":         hex.EncodeToString(packetIn.Pkt),
	})
	return logicalPortNum, nil
}

//GetPacketOutGemPortID returns gemPortId
func (f *OpenOltFlowMgr) GetPacketOutGemPortID(ctx context.Context, intfID uint32, onuID uint32, portNum uint32) (uint32, error) {
	var gemPortID uint32
	var err error

	f.lockCache.Lock()
	defer f.lockCache.Unlock()
	pktInkey := rsrcMgr.PacketInInfoKey{IntfID: intfID, OnuID: onuID, LogicalPort: portNum}

	gemPortID, ok := f.packetInGemPort[pktInkey]
	if ok {
		log.Debugw("Found gemport for pktin key", log.Fields{"pktinkey": pktInkey, "gem": gemPortID})
		return gemPortID, err
	}
	//If gem is not found in cache try to get it from kv store, if found in kv store, update the cache and return.
	gemPortID, err = f.resourceMgr.GetGemPortFromOnuPktIn(ctx, intfID, onuID, portNum)
	if err == nil {
		if gemPortID != 0 {
			f.packetInGemPort[pktInkey] = gemPortID
			log.Debugw("Found gem port from kv store and updating cache with gemport",
				log.Fields{"pktinkey": pktInkey, "gem": gemPortID})
			return gemPortID, nil
		}
	}
	log.Errorw("Failed to get gemport", log.Fields{"pktinkey": pktInkey, "gem": gemPortID})
	return uint32(0), err
}

func installFlowOnAllGemports(ctx context.Context,
	f1 func(ctx context.Context, intfId uint32, onuId uint32, uniId uint32,
		portNo uint32, classifier map[string]interface{}, action map[string]interface{},
		logicalFlow *ofp.OfpFlowStats, allocId uint32, gemPortId uint32),
	f2 func(ctx context.Context, intfId uint32, onuId uint32, uniId uint32, portNo uint32,
		logicalFlow *ofp.OfpFlowStats, allocId uint32, gemPortId uint32, vlanId uint32,
		classifier map[string]interface{}, action map[string]interface{}),
	args map[string]uint32,
	classifier map[string]interface{}, action map[string]interface{},
	logicalFlow *ofp.OfpFlowStats,
	gemPorts []uint32,
	FlowType string,
	vlanID ...uint32) {
	log.Debugw("Installing flow on all GEM ports", log.Fields{"FlowType": FlowType, "gemPorts": gemPorts, "vlan": vlanID})
	for _, gemPortID := range gemPorts {
		if FlowType == HsiaFlow || FlowType == DhcpFlow {
			f1(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID)
		} else if FlowType == EapolFlow {
			f2(ctx, args["intfId"], args["onuId"], args["uniId"], args["portNo"], logicalFlow, args["allocId"], gemPortID, vlanID[0], classifier, action)
		} else {
			log.Errorw("Unrecognized Flow Type", log.Fields{"FlowType": FlowType})
			return
		}
	}
}

func (f *OpenOltFlowMgr) addDHCPTrapFlowOnNNI(ctx context.Context, logicalFlow *ofp.OfpFlowStats, classifier map[string]interface{}, portNo uint32) {
	log.Debug("Adding trap-dhcp-of-nni-flow")
	action := make(map[string]interface{})
	classifier[PacketTagType] = DoubleTag
	action[TrapToHost] = true
	var err error
	var networkInterfaceID uint32
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
	networkInterfaceID, err = getNniIntfID(classifier, action)
	if err != nil {
		log.Error("Failed to get nniIntf ID")
		return
	}

	flowStoreCookie := getFlowStoreCookie(classifier, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), flowStoreCookie); present {
		log.Debug("Flow-exists--not-re-adding")
		return
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), uint32(gemPortID), flowStoreCookie, "", 0)
	if err != nil {
		log.Errorw("Flow id unavailable for DHCP traponNNI flow", log.Fields{"error": err})
		return
	}
	var classifierProto *openoltpb2.Classifier
	var actionProto *openoltpb2.Action
	if classifierProto = makeOpenOltClassifierField(classifier); classifierProto == nil {
		log.Error("Error in making classifier protobuf for  dhcp trap on nni flow")
		return
	}
	log.Debugw("Created classifier proto", log.Fields{"classifier": *classifierProto})
	if actionProto = makeOpenOltActionField(action); actionProto == nil {
		log.Error("Error in making action protobuf for dhcp trap on nni flow")
		return
	}
	log.Debugw("Created action proto", log.Fields{"action": *actionProto})
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
	if ok := f.addFlowToDevice(ctx, logicalFlow, &downstreamflow); ok {
		log.Debug("DHCP trap on NNI flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(ctx, &downstreamflow, flowStoreCookie, "", flowID, logicalFlow.Id)
		if err := f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID),
			int32(onuID),
			int32(uniID),
			flowID, flowsToKVStore); err != nil {
			log.Errorw("Error uploading DHCP DL  flow into KV store", log.Fields{"flow": downstreamflow, "error": err})
		}
	}
	return
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
func (f *OpenOltFlowMgr) addIgmpTrapFlowOnNNI(ctx context.Context, logicalFlow *ofp.OfpFlowStats, classifier map[string]interface{}, portNo uint32) {
	log.Debugw("Adding igmp-trap-of-nni-flow", log.Fields{"classifierInfo": classifier})
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
	networkInterfaceID, err := getNniIntfID(classifier, action)
	if err != nil {
		log.Error("Failed to get nniIntf ID")
		return
	}
	flowStoreCookie := getFlowStoreCookie(classifier, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), flowStoreCookie); present {
		log.Debug("igmp-flow-exists--not-re-adding")
		return
	}
	flowID, err := f.resourceMgr.GetFlowID(ctx, uint32(networkInterfaceID), int32(onuID), int32(uniID), uint32(gemPortID), flowStoreCookie, "", 0, 0)
	if err != nil {
		log.Errorw("IGMP flow id unavailable for trap-on-NNI flow", log.Fields{"error": err})
		return
	}
	var classifierProto *openoltpb2.Classifier
	var actionProto *openoltpb2.Action
	if classifierProto = makeOpenOltClassifierField(classifier); classifierProto == nil {
		log.Error("Error in making classifier protobuf for igmp trap on nni flow")
		return
	}
	log.Debugw("Created classifier proto for the IGMP flow", log.Fields{"classifier": *classifierProto})
	if actionProto = makeOpenOltActionField(action); actionProto == nil {
		log.Error("Error in making action protobuf for IGMP trap on nni flow")
		return
	}
	log.Debugw("Created action proto for the IGMP flow", log.Fields{"action": *actionProto})
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
	if ok := f.addFlowToDevice(ctx, logicalFlow, &downstreamflow); ok {
		log.Debug("IGMP Trap on NNI flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(ctx, &downstreamflow, flowStoreCookie, "", flowID, logicalFlow.Id)
		if err := f.updateFlowInfoToKVStore(ctx, int32(networkInterfaceID),
			int32(onuID),
			int32(uniID),
			flowID, flowsToKVStore); err != nil {
			log.Errorw("Error uploading igmp-trap-on-nni flow into KV store", log.Fields{"flow": downstreamflow, "error": err})
		}
	}
	return
}

func verifyMeterIDAndGetDirection(MeterID uint32, Dir tp_pb.Direction) (string, error) {
	if MeterID == 0 { // This should never happen
		log.Error("Invalid meter id")
		return "", errors.New("invalid meter id")
	}
	if Dir == tp_pb.Direction_UPSTREAM {
		return "upstream", nil
	} else if Dir == tp_pb.Direction_DOWNSTREAM {
		return "downstream", nil
	}
	return "", nil
}

func (f *OpenOltFlowMgr) checkAndAddFlow(ctx context.Context, args map[string]uint32, classifierInfo map[string]interface{},
	actionInfo map[string]interface{}, flow *ofp.OfpFlowStats, TpInst *tp.TechProfile, gemPorts []uint32,
	TpID uint32, uni string) {
	var gemPort uint32
	intfID := args[IntfID]
	onuID := args[OnuID]
	uniID := args[UniID]
	portNo := args[PortNo]
	allocID := TpInst.UsScheduler.AllocID
	if ipProto, ok := classifierInfo[IPProto]; ok {
		if ipProto.(uint32) == IPProtoDhcp {
			log.Info("Adding DHCP flow")
			if pcp, ok := classifierInfo[VlanPcp]; ok {
				gemPort = f.techprofile[intfID].GetGemportIDForPbit(TpInst,
					tp_pb.Direction_UPSTREAM,
					pcp.(uint32))
				//Adding DHCP upstream flow
				f.addDHCPTrapFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID, gemPort)
			} else {
				//Adding DHCP upstream flow to all gemports
				installFlowOnAllGemports(ctx, f.addDHCPTrapFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, DhcpFlow)
			}

		} else if ipProto == IgmpProto {
			log.Infow("Adding Us IGMP flow", log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID, "classifierInfo:": classifierInfo})
			if pcp, ok := classifierInfo[VlanPcp]; ok {
				gemPort = f.techprofile[intfID].GetGemportIDForPbit(TpInst,
					tp_pb.Direction_UPSTREAM,
					pcp.(uint32))
				f.addIGMPTrapFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID, gemPort)
			} else {
				//Adding IGMP upstream flow to all gem ports
				installFlowOnAllGemports(ctx, f.addIGMPTrapFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, IgmpFlow)
			}
		} else {
			log.Errorw("Invalid-Classifier-to-handle", log.Fields{"classifier": classifierInfo, "action": actionInfo})
			return
		}
	} else if ethType, ok := classifierInfo[EthType]; ok {
		if ethType.(uint32) == EapEthType {
			log.Info("Adding EAPOL flow")
			var vlanID uint32
			if val, ok := classifierInfo[VlanVid]; ok {
				vlanID = (val.(uint32)) & VlanvIDMask
			} else {
				vlanID = DefaultMgmtVlan
			}
			if pcp, ok := classifierInfo[VlanPcp]; ok {
				gemPort = f.techprofile[intfID].GetGemportIDForPbit(TpInst,
					tp_pb.Direction_UPSTREAM,
					pcp.(uint32))

				f.addEAPOLFlow(ctx, intfID, onuID, uniID, portNo, flow, allocID, gemPort, vlanID, classifierInfo, actionInfo)
			} else {
				installFlowOnAllGemports(ctx, nil, f.addEAPOLFlow, args, classifierInfo, actionInfo, flow, gemPorts, EapolFlow, vlanID)
			}
		}
	} else if _, ok := actionInfo[PushVlan]; ok {
		log.Info("Adding upstream data rule")
		if pcp, ok := classifierInfo[VlanPcp]; ok {
			gemPort = f.techprofile[intfID].GetGemportIDForPbit(TpInst,
				tp_pb.Direction_UPSTREAM,
				pcp.(uint32))
			//Adding HSIA upstream flow
			f.addUpstreamDataFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID, gemPort)
		} else {
			//Adding HSIA upstream flow to all gemports
			installFlowOnAllGemports(ctx, f.addUpstreamDataFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, HsiaFlow)
		}
	} else if _, ok := actionInfo[PopVlan]; ok {
		log.Info("Adding Downstream data rule")
		if pcp, ok := classifierInfo[VlanPcp]; ok {
			gemPort = f.techprofile[intfID].GetGemportIDForPbit(TpInst,
				tp_pb.Direction_DOWNSTREAM,
				pcp.(uint32))
			//Adding HSIA downstream flow
			f.addDownstreamDataFlow(ctx, intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID, gemPort)
		} else {
			//Adding HSIA downstream flow to all gemports
			installFlowOnAllGemports(ctx, f.addDownstreamDataFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, HsiaFlow)
		}
	} else {
		log.Errorw("Invalid-flow-type-to-handle", log.Fields{"classifier": classifierInfo, "action": actionInfo, "flow": flow})
		return
	}
	// Send Techprofile download event to child device in go routine as it takes time
	go f.sendTPDownloadMsgToChild(intfID, onuID, uniID, uni, TpID)
}

func (f *OpenOltFlowMgr) isGemPortUsedByAnotherFlow(gemPK gemPortKey) bool {
	flowIDList := f.flowsUsedByGemPort[gemPK]
	if len(flowIDList) > 1 {
		return true
	}
	return false
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
		f.resourceMgr.RemoveTechProfileIDForOnu(ctx, ponIntf, uint32(onuID), uint32(uniID), tpID)
		f.DeleteTechProfileInstance(ctx, ponIntf, uint32(onuID), uint32(uniID), "", tpID)

		// Although we cleaned up TP Instance for the given (PON ID, ONU ID, UNI ID), the TP might
		// still be used on other uni ports.
		// So, we need to check and make sure that no other gem port is referring to the given TP ID
		// on any other uni port.
		tpInstances := f.techprofile[ponIntf].FindAllTpInstances(ctx, tpID, ponIntf, onuID)
		log.Debugw("got single instance tp instances", log.Fields{"tpInstances": tpInstances})
		for i := 0; i < len(tpInstances); i++ {
			tpI := tpInstances[i]
			tpGemPorts := tpI.UpstreamGemPortAttributeList
			for _, tpGemPort := range tpGemPorts {
				if tpGemPort.GemportID != gemPortID {
					log.Debugw("single instance tp is in use by gem", log.Fields{"gemPort": tpGemPort.GemportID})
					return true, tpGemPort.GemportID
				}
			}
		}
	}
	log.Debug("tech profile is not in use by any gem")
	return false, 0
}

func formulateClassifierInfoFromFlow(classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) {
	for _, field := range flows.GetOfbFields(flow) {
		if field.Type == flows.ETH_TYPE {
			classifierInfo[EthType] = field.GetEthType()
			log.Debug("field-type-eth-type", log.Fields{"classifierInfo[ETH_TYPE]": classifierInfo[EthType].(uint32)})
		} else if field.Type == flows.ETH_DST {
			classifierInfo[EthDst] = field.GetEthDst()
			log.Debug("field-type-eth-type", log.Fields{"classifierInfo[ETH_DST]": classifierInfo[EthDst].([]uint8)})
		} else if field.Type == flows.IP_PROTO {
			classifierInfo[IPProto] = field.GetIpProto()
			log.Debug("field-type-ip-proto", log.Fields{"classifierInfo[IP_PROTO]": classifierInfo[IPProto].(uint32)})
		} else if field.Type == flows.IN_PORT {
			classifierInfo[InPort] = field.GetPort()
			log.Debug("field-type-in-port", log.Fields{"classifierInfo[IN_PORT]": classifierInfo[InPort].(uint32)})
		} else if field.Type == flows.VLAN_VID {
			classifierInfo[VlanVid] = field.GetVlanVid() & 0xfff
			log.Debug("field-type-vlan-vid", log.Fields{"classifierInfo[VLAN_VID]": classifierInfo[VlanVid].(uint32)})
		} else if field.Type == flows.VLAN_PCP {
			classifierInfo[VlanPcp] = field.GetVlanPcp()
			log.Debug("field-type-vlan-pcp", log.Fields{"classifierInfo[VLAN_PCP]": classifierInfo[VlanPcp].(uint32)})
		} else if field.Type == flows.UDP_DST {
			classifierInfo[UDPDst] = field.GetUdpDst()
			log.Debug("field-type-udp-dst", log.Fields{"classifierInfo[UDP_DST]": classifierInfo[UDPDst].(uint32)})
		} else if field.Type == flows.UDP_SRC {
			classifierInfo[UDPSrc] = field.GetUdpSrc()
			log.Debug("field-type-udp-src", log.Fields{"classifierInfo[UDP_SRC]": classifierInfo[UDPSrc].(uint32)})
		} else if field.Type == flows.IPV4_DST {
			classifierInfo[Ipv4Dst] = field.GetIpv4Dst()
			log.Debug("field-type-ipv4-dst", log.Fields{"classifierInfo[IPV4_DST]": classifierInfo[Ipv4Dst].(uint32)})
		} else if field.Type == flows.IPV4_SRC {
			classifierInfo[Ipv4Src] = field.GetIpv4Src()
			log.Debug("field-type-ipv4-src", log.Fields{"classifierInfo[IPV4_SRC]": classifierInfo[Ipv4Src].(uint32)})
		} else if field.Type == flows.METADATA {
			classifierInfo[Metadata] = field.GetTableMetadata()
			log.Debug("field-type-metadata", log.Fields{"classifierInfo[Metadata]": classifierInfo[Metadata].(uint64)})
		} else if field.Type == flows.TUNNEL_ID {
			classifierInfo[TunnelID] = field.GetTunnelId()
			log.Debug("field-type-tunnelId", log.Fields{"classifierInfo[TUNNEL_ID]": classifierInfo[TunnelID].(uint64)})
		} else {
			log.Errorw("Un supported field type", log.Fields{"type": field.Type})
			return
		}
	}
}

func formulateActionInfoFromFlow(actionInfo, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) error {
	for _, action := range flows.GetActions(flow) {
		if action.Type == flows.OUTPUT {
			if out := action.GetOutput(); out != nil {
				actionInfo[Output] = out.GetPort()
				log.Debugw("action-type-output", log.Fields{"out_port": actionInfo[Output].(uint32)})
			} else {
				log.Error("Invalid output port in action")
				return errors.New("invalid output port in action")
			}
		} else if action.Type == flows.POP_VLAN {
			actionInfo[PopVlan] = true
			log.Debugw("action-type-pop-vlan", log.Fields{"in_port": classifierInfo[InPort].(uint32)})
		} else if action.Type == flows.PUSH_VLAN {
			if out := action.GetPush(); out != nil {
				if tpid := out.GetEthertype(); tpid != 0x8100 {
					log.Errorw("Invalid ethertype in push action", log.Fields{"ethertype": actionInfo[PushVlan].(int32)})
				} else {
					actionInfo[PushVlan] = true
					actionInfo[TPID] = tpid
					log.Debugw("action-type-push-vlan",
						log.Fields{"push_tpid": actionInfo[TPID].(uint32), "in_port": classifierInfo[InPort].(uint32)})
				}
			}
		} else if action.Type == flows.SET_FIELD {
			if out := action.GetSetField(); out != nil {
				if field := out.GetField(); field != nil {
					if ofClass := field.GetOxmClass(); ofClass != ofp.OfpOxmClass_OFPXMC_OPENFLOW_BASIC {
						log.Errorw("Invalid openflow class", log.Fields{"class": ofClass})
						return errors.New("invalid openflow class")
					}
					/*log.Debugw("action-type-set-field",log.Fields{"field": field, "in_port": classifierInfo[IN_PORT].(uint32)})*/
					formulateSetFieldActionInfoFromFlow(field, actionInfo)
				}
			}
		} else if action.Type == flows.GROUP {
			formulateGroupActionInfoFromFlow(action, actionInfo)
		} else {
			log.Errorw("Un supported action type", log.Fields{"type": action.Type})
			return errors.New("un supported action type")
		}
	}
	return nil
}

func formulateSetFieldActionInfoFromFlow(field *ofp.OfpOxmField, actionInfo map[string]interface{}) {
	if ofbField := field.GetOfbField(); ofbField != nil {
		if fieldtype := ofbField.GetType(); fieldtype == ofp.OxmOfbFieldTypes_OFPXMT_OFB_VLAN_VID {
			if vlan := ofbField.GetVlanVid(); vlan != 0 {
				actionInfo[VlanVid] = vlan & 0xfff
				log.Debugw("action-set-vlan-vid", log.Fields{"actionInfo[VLAN_VID]": actionInfo[VlanVid].(uint32)})
			} else {
				log.Error("No Invalid vlan id in set vlan-vid action")
			}
		} else {
			log.Errorw("unsupported-action-set-field-type", log.Fields{"type": fieldtype})
		}
	}
}

func formulateGroupActionInfoFromFlow(action *ofp.OfpAction, actionInfo map[string]interface{}) {
	if action.GetGroup() == nil {
		log.Warn("No group entry found in the group action")
	} else {
		actionInfo[GroupID] = action.GetGroup().GroupId
		log.Debugw("action-group-id", log.Fields{"actionInfo[GroupID]": actionInfo[GroupID].(uint32)})
	}
}

func formulateControllerBoundTrapFlowInfo(actionInfo, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) error {
	if isControllerFlow := IsControllerBoundFlow(actionInfo[Output].(uint32)); isControllerFlow {
		log.Debug("Controller bound trap flows, getting inport from tunnelid")
		/* Get UNI port/ IN Port from tunnel ID field for upstream controller bound flows  */
		if portType := IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := flows.GetChildPortFromTunnelId(flow); uniPort != 0 {
				classifierInfo[InPort] = uniPort
				log.Debugw("upstream pon-to-controller-flow,inport-in-tunnelid", log.Fields{"newInPort": classifierInfo[InPort].(uint32), "outPort": actionInfo[Output].(uint32)})
			} else {
				log.Error("upstream pon-to-controller-flow, NO-inport-in-tunnelid")
				return errors.New("upstream pon-to-controller-flow, NO-inport-in-tunnelid")
			}
		}
	} else {
		log.Debug("Non-Controller flows, getting uniport from tunnelid")
		// Downstream flow from NNI to PON port , Use tunnel ID as new OUT port / UNI port
		if portType := IntfIDToPortTypeName(actionInfo[Output].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := flows.GetChildPortFromTunnelId(flow); uniPort != 0 {
				actionInfo[Output] = uniPort
				log.Debugw("downstream-nni-to-pon-port-flow, outport-in-tunnelid", log.Fields{"newOutPort": actionInfo[Output].(uint32), "outPort": actionInfo[Output].(uint32)})
			} else {
				log.Debug("downstream-nni-to-pon-port-flow, no-outport-in-tunnelid", log.Fields{"InPort": classifierInfo[InPort].(uint32), "outPort": actionInfo[Output].(uint32)})
				return errors.New("downstream-nni-to-pon-port-flow, no-outport-in-tunnelid")
			}
			// Upstream flow from PON to NNI port , Use tunnel ID as new IN port / UNI port
		} else if portType := IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := flows.GetChildPortFromTunnelId(flow); uniPort != 0 {
				classifierInfo[InPort] = uniPort
				log.Debugw("upstream-pon-to-nni-port-flow, inport-in-tunnelid", log.Fields{"newInPort": actionInfo[Output].(uint32),
					"outport": actionInfo[Output].(uint32)})
			} else {
				log.Debug("upstream-pon-to-nni-port-flow, no-inport-in-tunnelid", log.Fields{"InPort": classifierInfo[InPort].(uint32),
					"outPort": actionInfo[Output].(uint32)})
				return errors.New("upstream-pon-to-nni-port-flow, no-inport-in-tunnelid")
			}
		}
	}
	return nil
}

func getTpIDFromFlow(flow *ofp.OfpFlowStats) (uint32, error) {
	/*     Metadata 8 bytes:
		   Most Significant 2 Bytes = Inner VLAN
		   Next 2 Bytes = Tech Profile ID(TPID)
		   Least Significant 4 Bytes = Port ID
	       Flow Metadata carries Tech-Profile (TP) ID and is mandatory in all
	       subscriber related flows.
	*/
	metadata := flows.GetMetadataFromWriteMetadataAction(flow)
	if metadata == 0 {
		log.Error("metadata-is-not-present-in-flow-which-is-mandatory")
		return 0, errors.New("metadata-is-not-present-in-flow-which-is-mandatory")
	}
	TpID := flows.GetTechProfileIDFromWriteMetaData(metadata)
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
func getNniIntfID(classifier map[string]interface{}, action map[string]interface{}) (uint32, error) {

	portType := IntfIDToPortTypeName(classifier[InPort].(uint32))
	if portType == voltha.Port_PON_OLT {
		intfID := IntfIDFromNniPortNum(action[Output].(uint32))
		log.Debugw("output Nni IntfID is", log.Fields{"intfid": intfID})
		return intfID, nil
	} else if portType == voltha.Port_ETHERNET_NNI {
		intfID := IntfIDFromNniPortNum(classifier[InPort].(uint32))
		log.Debugw("input Nni IntfID is", log.Fields{"intfid": intfID})
		return intfID, nil
	}
	return uint32(0), nil
}

// UpdateGemPortForPktIn updates gemport for packet-in in to the cache and to the kv store as well.
func (f *OpenOltFlowMgr) UpdateGemPortForPktIn(ctx context.Context, intfID uint32, onuID uint32, logicalPort uint32, gemPort uint32) {
	pktInkey := rsrcMgr.PacketInInfoKey{IntfID: intfID, OnuID: onuID, LogicalPort: logicalPort}

	f.lockCache.Lock()
	defer f.lockCache.Unlock()
	lookupGemPort, ok := f.packetInGemPort[pktInkey]
	if ok {
		if lookupGemPort == gemPort {
			log.Debugw("pktin key/value found in cache , no need to update kv as we are assuming both will be in sync",
				log.Fields{"pktinkey": pktInkey, "gem": gemPort})
			return
		}
	}
	f.packetInGemPort[pktInkey] = gemPort

	f.resourceMgr.UpdateGemPortForPktIn(ctx, pktInkey, gemPort)
	log.Debugw("pktin key not found in local cache or value is different. updating cache and kv store", log.Fields{"pktinkey": pktInkey, "gem": gemPort})
	return
}

// AddUniPortToOnuInfo adds uni port to the onugem info both in cache and kvstore.
func (f *OpenOltFlowMgr) AddUniPortToOnuInfo(ctx context.Context, intfID uint32, onuID uint32, portNum uint32) {

	f.lockCache.Lock()
	defer f.lockCache.Unlock()
	onugem := f.onuGemInfo[intfID]
	for idx, onu := range onugem {
		if onu.OnuID == onuID {
			for _, uni := range onu.UniPorts {
				if uni == portNum {
					log.Debugw("uni already in cache, no need to update cache and kv store",
						log.Fields{"uni": portNum})
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
		log.Error("Failed to get flowid list per gem", log.Fields{"intf": intf})
		return
	}
	for gem, FlowIDs := range flowIDsList {
		gemPK := gemPortKey{intf, uint32(gem)}
		f.flowsUsedByGemPort[gemPK] = FlowIDs
	}
	return
}

//loadInterfaceToMulticastQueueMap reads multicast queues per interface from the KV store
//and put them into interfaceToMcastQueueMap.
func (f *OpenOltFlowMgr) loadInterfaceToMulticastQueueMap(ctx context.Context) {
	storedMulticastQueueMap, err := f.resourceMgr.GetMcastQueuePerInterfaceMap(ctx)
	if err != nil {
		log.Error("Failed to get pon interface to multicast queue map")
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
		log.Errorw("Failed to get the flow group from KV store", log.Fields{"groupId": groupID, "err": err})
		return nil, false, errors.New("failed to retrieve the flow group")
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
