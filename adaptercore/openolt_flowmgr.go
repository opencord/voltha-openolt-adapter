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
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/opencord/voltha-go/common/log"
	tp "github.com/opencord/voltha-go/common/techprofile"
	"github.com/opencord/voltha-go/rw_core/utils"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	"github.com/opencord/voltha-protos/go/common"
	ic "github.com/opencord/voltha-protos/go/inter_container"
	ofp "github.com/opencord/voltha-protos/go/openflow_13"
	openoltpb2 "github.com/opencord/voltha-protos/go/openolt"
	tp_pb "github.com/opencord/voltha-protos/go/tech_profile"
	"github.com/opencord/voltha-protos/go/voltha"

	//deepcopy "github.com/getlantern/deepcopy"
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

	//IPProtoDhcp flow category
	IPProtoDhcp = 17

	//IPProtoIgmp flow category
	IPProtoIgmp = 2

	//EapEthType eapethtype value
	EapEthType = 0x888e
	//LldpEthType lldp ethtype value
	LldpEthType = 0x88cc

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
	//MaxPonPorts constant
	MaxPonPorts = 16
)

type onuInfo struct {
	intfID       uint32
	onuID        uint32
	serialNumber string
}

type onuIDKey struct {
	intfID uint32
	onuID  uint32
}

type gemPortKey struct {
	intfID  uint32
	gemPort uint32
}

type packetInInfoKey struct {
	intfID      uint32
	onuID       uint32
	logicalPort uint32
}

//OpenOltFlowMgr creates the Structure of OpenOltFlowMgr obj
type OpenOltFlowMgr struct {
	techprofile       []*tp.TechProfileMgr
	deviceHandler     *DeviceHandler
	resourceMgr       *rsrcMgr.OpenOltResourceMgr
	onuIds            map[onuIDKey]onuInfo       //OnuId -> OnuInfo
	onuSerialNumbers  map[string]onuInfo         //onu serial_number (string) -> OnuInfo
	onuGemPortIds     map[gemPortKey]onuInfo     //GemPortId -> OnuInfo
	packetInGemPort   map[packetInInfoKey]uint32 //packet in gem port
	storedDeviceFlows []ofp.OfpFlowStats         /* Required during deletion to obtain device flows from logical flows */
}

//NewFlowManager creates OpenOltFlowMgr object and initializes the parameters
func NewFlowManager(dh *DeviceHandler, rsrcMgr *rsrcMgr.OpenOltResourceMgr) *OpenOltFlowMgr {
	log.Info("Initializing flow manager")
	var flowMgr OpenOltFlowMgr
	flowMgr.deviceHandler = dh
	flowMgr.resourceMgr = rsrcMgr
	flowMgr.techprofile = make([]*tp.TechProfileMgr, MaxPonPorts)
	if err := flowMgr.populateTechProfilePerPonPort(); err != nil {
		log.Error("Error while populating tech profile mgr\n")
		return nil
	}
	flowMgr.onuIds = make(map[onuIDKey]onuInfo)
	flowMgr.onuSerialNumbers = make(map[string]onuInfo)
	flowMgr.onuGemPortIds = make(map[gemPortKey]onuInfo)
	flowMgr.packetInGemPort = make(map[packetInInfoKey]uint32)
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
	} else {
		log.Debug("Unrecognized direction")
		return 0, fmt.Errorf("unrecognized direction %s", direction)
	}
}

func (f *OpenOltFlowMgr) registerFlow(flowFromCore *ofp.OfpFlowStats, deviceFlow *openoltpb2.Flow) {
	log.Debug("Registering Flow for Device ", log.Fields{"flow": flowFromCore},
		log.Fields{"device": f.deviceHandler.deviceID})

	var storedFlow ofp.OfpFlowStats
	storedFlow.Id, _ = f.generateStoredFlowID(deviceFlow.FlowId, deviceFlow.FlowType)
	log.Debug(fmt.Sprintf("Generated stored device flow. id = %d, flowId = %d, direction = %s", storedFlow.Id,
		deviceFlow.FlowId, deviceFlow.FlowType))
	storedFlow.Cookie = flowFromCore.Id
	f.storedDeviceFlows = append(f.storedDeviceFlows, storedFlow)
	log.Debugw("updated Stored flow info", log.Fields{"storedDeviceFlows": f.storedDeviceFlows})
}

func (f *OpenOltFlowMgr) divideAndAddFlow(intfID uint32, onuID uint32, uniID uint32, portNo uint32,
	classifierInfo map[string]interface{}, actionInfo map[string]interface{}, flow *ofp.OfpFlowStats, TpID uint32,
	UsMeterID uint32, DsMeterID uint32, flowMetadata *voltha.FlowMetadata) {
	var allocID []uint32
	var gemPorts []uint32
	var gemPort uint32
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

	uni := getUniPortPath(intfID, onuID, uniID)
	log.Debugw("Uni port name", log.Fields{"uni": uni})
	allocID, gemPorts, TpInst = f.createTcontGemports(intfID, onuID, uniID, uni, portNo, TpID, UsMeterID, DsMeterID, flowMetadata)
	if allocID == nil || gemPorts == nil || TpInst == nil {
		log.Error("alloc-id-gem-ports-tp-unavailable")
		return
	}

	/* Flows can be added specific to gemport if p-bits are received.
	 * If no pbit mentioned then adding flows for all gemports
	 */

	args := make(map[string]uint32)
	args["intfId"] = intfID
	args["onuId"] = onuID
	args["uniId"] = uniID
	args["portNo"] = portNo
	args["allocId"] = allocID[0]

	f.checkAndAddFlow(args, classifierInfo, actionInfo, flow, gemPort, intfID, onuID, uniID, portNo, TpInst, allocID, gemPorts, TpID, uni)
}

// CreateSchedulerQueues creates traffic schedulers on the device with the given scheduler configuration and traffic shaping info
func (f *OpenOltFlowMgr) CreateSchedulerQueues(Dir tp_pb.Direction, IntfID uint32, OnuID uint32, UniID uint32, UniPort uint32, TpInst *tp.TechProfile, MeterID uint32, flowMetadata *voltha.FlowMetadata) error {

	log.Debugw("CreateSchedulerQueues", log.Fields{"Dir": Dir, "IntfID": IntfID, "OnuID": OnuID,
		"UniID": UniID, "MeterID": MeterID, "TpInst": *TpInst, "flowMetadata": flowMetadata})

	Direction, err := verifyMeterIDAndGetDirection(MeterID, Dir)
	if err != nil {
		return err
	}

	/* Lets make a simple assumption that if the meter-id is present on the KV store,
	 * then the scheduler and queues configuration is applied on the OLT device
	 * in the given direction.
	 */

	var SchedCfg *tp_pb.SchedulerConfig
	KvStoreMeter, err := f.resourceMgr.GetMeterIDForOnu(Direction, IntfID, OnuID, UniID)
	if err != nil {
		log.Error("Failed to get meter for intf %d, onuid %d, uniid %d", IntfID, OnuID, UniID)
		return err
	}
	if KvStoreMeter != nil {
		if KvStoreMeter.MeterId == MeterID {
			log.Debug("Scheduler already created for upstream")
			return nil
		}
		log.Errorw("Dynamic meter update not supported", log.Fields{"KvStoreMeterId": KvStoreMeter.MeterId, "MeterID-in-flow": MeterID})
		return errors.New("invalid-meter-id-in-flow")
	}
	log.Debugw("Meter-does-not-exist-Creating-new", log.Fields{"MeterID": MeterID, "Direction": Direction})
	if Dir == tp_pb.Direction_UPSTREAM {
		SchedCfg = f.techprofile[IntfID].GetUsScheduler(TpInst)
	} else if Dir == tp_pb.Direction_DOWNSTREAM {
		SchedCfg = f.techprofile[IntfID].GetDsScheduler(TpInst)
	}
	var meterConfig *ofp.OfpMeterConfig
	if flowMetadata != nil {
		for _, meter := range flowMetadata.Meters {
			if MeterID == meter.MeterId {
				meterConfig = meter
				log.Debugw("Found-meter-config-from-flowmetadata", log.Fields{"meterConfig": meterConfig})
				break
			}
		}
	} else {
		log.Error("Flow-metadata-is-not-present-in-flow")
	}
	if meterConfig == nil {
		log.Errorw("Could-not-get-meterbands-from-flowMetadata", log.Fields{"flowMetadata": flowMetadata, "MeterID": MeterID})
		return errors.New("failed-to-get-meter-from-flowMetadata")
	} else if len(meterConfig.Bands) < MaxMeterBand {
		log.Errorw("Invalid-number-of-bands-in-meter", log.Fields{"Bands": meterConfig.Bands, "MeterID": MeterID})
		return errors.New("invalid-number-of-bands-in-meter")
	}
	cir := meterConfig.Bands[0].Rate
	cbs := meterConfig.Bands[0].BurstSize
	eir := meterConfig.Bands[1].Rate
	ebs := meterConfig.Bands[1].BurstSize
	pir := cir + eir
	pbs := cbs + ebs
	TrafficShaping := &tp_pb.TrafficShapingInfo{Cir: cir, Cbs: cbs, Pir: pir, Pbs: pbs}

	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile[IntfID].GetTrafficScheduler(TpInst, SchedCfg, TrafficShaping)}

	log.Debugw("Sending Traffic scheduler create to device", log.Fields{"Direction": Direction, "TrafficScheds": TrafficSched})
	if _, err := f.deviceHandler.Client.CreateTrafficSchedulers(context.Background(), &tp_pb.TrafficSchedulers{
		IntfId: IntfID, OnuId: OnuID,
		UniId: UniID, PortNo: UniPort,
		TrafficScheds: TrafficSched}); err != nil {
		log.Errorw("Failed to create traffic schedulers", log.Fields{"error": err})
		return err
	}
	// On receiving the CreateTrafficQueues request, the driver should create corresponding
	// downstream queues.
	trafficQueues := f.techprofile[IntfID].GetTrafficQueues(TpInst, Dir)
	log.Debugw("Sending Traffic Queues create to device", log.Fields{"Direction": Direction, "TrafficQueues": trafficQueues})
	if _, err := f.deviceHandler.Client.CreateTrafficQueues(context.Background(),
		&tp_pb.TrafficQueues{IntfId: IntfID, OnuId: OnuID,
			UniId: UniID, PortNo: UniPort,
			TrafficQueues: trafficQueues}); err != nil {
		log.Errorw("Failed to create traffic queues in device", log.Fields{"error": err})
		return err
	}

	/* After we successfully applied the scheduler configuration on the OLT device,
	 * store the meter id on the KV store, for further reference.
	 */
	if err := f.resourceMgr.UpdateMeterIDForOnu(Direction, IntfID, OnuID, UniID, meterConfig); err != nil {
		log.Error("Failed to update meter id for onu %d, meterid %d", OnuID, MeterID)
		return err
	}
	log.Debugw("updated-meter-info into KV store successfully", log.Fields{"Direction": Direction,
		"Meter": meterConfig})
	return nil
}

// RemoveSchedulerQueues removes the traffic schedulers from the device based on the given scheduler configuration and traffic shaping info
func (f *OpenOltFlowMgr) RemoveSchedulerQueues(Dir tp_pb.Direction, IntfID uint32, OnuID uint32, UniID uint32, UniPort uint32, TpInst *tp.TechProfile) error {

	var Direction string
	var SchedCfg *tp_pb.SchedulerConfig
	var err error
	log.Debugw("Removing schedulers and Queues in OLT", log.Fields{"Direction": Dir, "IntfID": IntfID, "OnuID": OnuID, "UniID": UniID, "UniPort": UniPort})
	if Dir == tp_pb.Direction_UPSTREAM {
		SchedCfg = f.techprofile[IntfID].GetUsScheduler(TpInst)
		Direction = "upstream"
	} else if Dir == tp_pb.Direction_DOWNSTREAM {
		SchedCfg = f.techprofile[IntfID].GetDsScheduler(TpInst)
		Direction = "downstream"
	}

	KVStoreMeter, err := f.resourceMgr.GetMeterIDForOnu(Direction, IntfID, OnuID, UniID)
	if err != nil {
		log.Errorf("Failed to get Meter for Onu %d", OnuID)
		return err
	}
	if KVStoreMeter == nil {
		log.Debugw("No-meter-has-been-installed-yet", log.Fields{"direction": Direction, "IntfID": IntfID, "OnuID": OnuID, "UniID": UniID})
		return nil
	}
	cir := KVStoreMeter.Bands[0].Rate
	cbs := KVStoreMeter.Bands[0].BurstSize
	eir := KVStoreMeter.Bands[1].Rate
	ebs := KVStoreMeter.Bands[1].BurstSize
	pir := cir + eir
	pbs := cbs + ebs

	TrafficShaping := &tp_pb.TrafficShapingInfo{Cir: cir, Cbs: cbs, Pir: pir, Pbs: pbs}

	TrafficSched := []*tp_pb.TrafficScheduler{f.techprofile[IntfID].GetTrafficScheduler(TpInst, SchedCfg, TrafficShaping)}
	TrafficQueues := f.techprofile[IntfID].GetTrafficQueues(TpInst, Dir)

	if _, err = f.deviceHandler.Client.RemoveTrafficQueues(context.Background(),
		&tp_pb.TrafficQueues{IntfId: IntfID, OnuId: OnuID,
			UniId: UniID, PortNo: UniPort,
			TrafficQueues: TrafficQueues}); err != nil {
		log.Errorw("Failed to remove traffic queues", log.Fields{"error": err})
		return err
	}
	log.Debug("Removed traffic queues successfully")
	if _, err = f.deviceHandler.Client.RemoveTrafficSchedulers(context.Background(), &tp_pb.TrafficSchedulers{
		IntfId: IntfID, OnuId: OnuID,
		UniId: UniID, PortNo: UniPort,
		TrafficScheds: TrafficSched}); err != nil {
		log.Errorw("failed to remove traffic schedulers", log.Fields{"error": err})
		return err
	}

	log.Debug("Removed traffic schedulers successfully")

	/* After we successfully remove the scheduler configuration on the OLT device,
	 * delete the meter id on the KV store.
	 */
	err = f.resourceMgr.RemoveMeterIDForOnu(Direction, IntfID, OnuID, UniID)
	if err != nil {
		log.Errorf("Failed to remove meter for onu %d, meter id %d", OnuID, KVStoreMeter.MeterId)
		return err
	}
	log.Debugw("Removed-meter-from-KV-store successfully", log.Fields{"MeterId": KVStoreMeter.MeterId, "dir": Direction})
	return err
}

// This function allocates tconts and GEM ports for an ONU, currently one TCONT is supported per ONU
func (f *OpenOltFlowMgr) createTcontGemports(intfID uint32, onuID uint32, uniID uint32, uni string, uniPort uint32, TpID uint32, UsMeterID uint32, DsMeterID uint32, flowMetadata *voltha.FlowMetadata) ([]uint32, []uint32, *tp.TechProfile) {
	var allocID []uint32
	var gemPortIDs []uint32
	//If we already have allocated earlier for this onu, render them
	if tcontID := f.resourceMgr.GetCurrentAllocIDForOnu(intfID, onuID, uniID); tcontID != 0 {
		allocID = append(allocID, tcontID)
	}
	gemPortIDs = f.resourceMgr.GetCurrentGEMPortIDsForOnu(intfID, onuID, uniID)

	tpPath := f.getTPpath(intfID, uni, TpID)
	// Check tech profile instance already exists for derived port name
	techProfileInstance, err := f.techprofile[intfID].GetTPInstanceFromKVStore(TpID, tpPath)
	if err != nil { // This should not happen, something wrong in KV backend transaction
		log.Errorw("Error in fetching tech profile instance from KV store", log.Fields{"tpID": TpID, "path": tpPath})
		return nil, nil, nil
	}

	log.Debug("Creating New TConts and Gem ports", log.Fields{"pon": intfID, "onu": onuID, "uni": uniID})

	if techProfileInstance == nil {
		log.Info("Creating tech profile instance", log.Fields{"path": tpPath})
		techProfileInstance = f.techprofile[intfID].CreateTechProfInstance(TpID, uni, intfID)
		if techProfileInstance == nil {
			log.Error("Tech-profile-instance-creation-failed")
			return nil, nil, nil
		}
		f.resourceMgr.UpdateTechProfileIDForOnu(intfID, onuID, uniID, TpID)
	} else {
		log.Debugw("Tech-profile-instance-already-exist-for-given port-name", log.Fields{"uni": uni})
	}
	if UsMeterID != 0 {
		if err := f.CreateSchedulerQueues(tp_pb.Direction_UPSTREAM, intfID, onuID, uniID, uniPort, techProfileInstance, UsMeterID, flowMetadata); err != nil {
			log.Errorw("CreateSchedulerQueues Failed-upstream", log.Fields{"error": err, "meterID": UsMeterID})
			return nil, nil, nil
		}
	}
	if DsMeterID != 0 {
		if err := f.CreateSchedulerQueues(tp_pb.Direction_DOWNSTREAM, intfID, onuID, uniID, uniPort, techProfileInstance, DsMeterID, flowMetadata); err != nil {
			log.Errorw("CreateSchedulerQueues Failed-downstream", log.Fields{"error": err, "meterID": DsMeterID})
			return nil, nil, nil
		}
	}
	if len(allocID) == 0 { // Created TCONT first time
		allocID = append(allocID, techProfileInstance.UsScheduler.AllocID)
	}
	if len(gemPortIDs) == 0 { // Create GEM ports first time
		for _, gem := range techProfileInstance.UpstreamGemPortAttributeList {
			gemPortIDs = append(gemPortIDs, gem.GemportID)
		}
	}
	log.Debugw("Allocated Tcont and GEM ports", log.Fields{"allocID": allocID, "gemports": gemPortIDs})
	// Send Tconts and GEM ports to KV store
	f.storeTcontsGEMPortsIntoKVStore(intfID, onuID, uniID, allocID, gemPortIDs)
	return allocID, gemPortIDs, techProfileInstance
}

func (f *OpenOltFlowMgr) storeTcontsGEMPortsIntoKVStore(intfID uint32, onuID uint32, uniID uint32, allocID []uint32, gemPortIDs []uint32) {

	log.Debugw("Storing allocated Tconts and GEM ports into KV store",
		log.Fields{"intfId": intfID, "onuId": onuID, "uniId": uniID, "allocID": allocID, "gemPortIDs": gemPortIDs})
	/* Update the allocated alloc_id and gem_port_id for the ONU/UNI to KV store  */
	if err := f.resourceMgr.UpdateAllocIdsForOnu(intfID, onuID, uniID, allocID); err != nil {
		log.Error("Errow while uploading allocID to KV store")
	}
	if err := f.resourceMgr.UpdateGEMPortIDsForOnu(intfID, onuID, uniID, gemPortIDs); err != nil {
		log.Error("Errow while uploading GEMports to KV store")
	}
	if err := f.resourceMgr.UpdateGEMportsPonportToOnuMapOnKVStore(gemPortIDs, intfID, onuID, uniID); err != nil {
		log.Error("Errow while uploading gemtopon map to KV store")
	}
	log.Debug("Stored tconts and GEM into KV store successfully")
	for _, gemPort := range gemPortIDs {
		f.addGemPortToOnuInfoMap(intfID, onuID, gemPort)
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

func (f *OpenOltFlowMgr) addUpstreamDataFlow(intfID uint32, onuID uint32, uniID uint32,
	portNo uint32, uplinkClassifier map[string]interface{},
	uplinkAction map[string]interface{}, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemportID uint32) {
	uplinkClassifier[PacketTagType] = SingleTag
	log.Debugw("Adding upstream data flow", log.Fields{"uplinkClassifier": uplinkClassifier, "uplinkAction": uplinkAction})
	f.addHSIAFlow(intfID, onuID, uniID, portNo, uplinkClassifier, uplinkAction,
		Upstream, logicalFlow, allocID, gemportID)
	/* TODO: Install Secondary EAP on the subscriber vlan */
}

func (f *OpenOltFlowMgr) addDownstreamDataFlow(intfID uint32, onuID uint32, uniID uint32,
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
	downlinkAction[VlanVid] = downlinkClassifier[VlanVid]
	f.addHSIAFlow(intfID, onuID, uniID, portNo, downlinkClassifier, downlinkAction,
		Downstream, logicalFlow, allocID, gemportID)
}

func (f *OpenOltFlowMgr) addHSIAFlow(intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifier map[string]interface{},
	action map[string]interface{}, direction string, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemPortID uint32) {
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
	var vlanPit uint32
	if _, ok := classifier[VlanPcp]; ok {
		vlanPit = classifier[VlanPcp].(uint32)
		log.Debugw("Found pbit in the flow", log.Fields{"vlan_pit": vlanPit})
	}
	flowStoreCookie := getFlowStoreCookie(classifier, gemPortID)
	flowID, err := f.resourceMgr.GetFlowID(intfID, onuID, uniID, gemPortID, flowStoreCookie, HsiaFlow, vlanPit)
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
	networkIntfID := f.deviceHandler.nniIntfID
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
	if ok := f.addFlowToDevice(logicalFlow, &flow); ok {
		log.Debug("HSIA flow added to device successfully", log.Fields{"direction": direction})
		flowsToKVStore := f.getUpdatedFlowInfo(&flow, flowStoreCookie, HsiaFlow, flowID)
		if err := f.updateFlowInfoToKVStore(flow.AccessIntfId,
			flow.OnuId,
			flow.UniId,
			flow.FlowId /*flowCategory,*/, flowsToKVStore); err != nil {
			log.Errorw("Error uploading HSIA  flow into KV store", log.Fields{"flow": flow, "direction": direction, "error": err})
			return
		}
	}
}
func (f *OpenOltFlowMgr) addDHCPTrapFlow(intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifier map[string]interface{}, action map[string]interface{}, logicalFlow *ofp.OfpFlowStats, allocID uint32, gemPortID uint32) {

	var dhcpFlow openoltpb2.Flow
	var actionProto *openoltpb2.Action
	var classifierProto *openoltpb2.Classifier

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

	flowID, err := f.resourceMgr.GetFlowID(intfID, onuID, uniID, gemPortID, flowStoreCookie, "", 0 /*classifier[VLAN_PCP].(uint32)*/)

	if err != nil {
		log.Errorw("flowId unavailable for UL EAPOL", log.Fields{"intfId": intfID, "onuId": onuID, "flowStoreCookie": flowStoreCookie})
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
	networkIntfID := f.deviceHandler.nniIntfID

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

	if ok := f.addFlowToDevice(logicalFlow, &dhcpFlow); ok {
		log.Debug("DHCP UL flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(&dhcpFlow, flowStoreCookie, "DHCP", flowID)
		if err := f.updateFlowInfoToKVStore(dhcpFlow.AccessIntfId,
			dhcpFlow.OnuId,
			dhcpFlow.UniId,
			dhcpFlow.FlowId, flowsToKVStore); err != nil {
			log.Errorw("Error uploading DHCP UL flow into KV store", log.Fields{"flow": dhcpFlow, "error": err})
			return
		}
	}

	return
}

// Add EAPOL flow to  device with mac, vlanId as classifier for upstream and downstream
func (f *OpenOltFlowMgr) addEAPOLFlow(intfID uint32, onuID uint32, uniID uint32, portNo uint32, logicalFlow *ofp.OfpFlowStats, allocID uint32, gemPortID uint32, vlanID uint32) {
	log.Debugw("Adding EAPOL to device", log.Fields{"intfId": intfID, "onuId": onuID, "portNo": portNo, "allocId": allocID, "gemPortId": gemPortID, "vlanId": vlanID, "flow": logicalFlow})

	uplinkClassifier := make(map[string]interface{})
	uplinkAction := make(map[string]interface{})
	downlinkClassifier := make(map[string]interface{})
	downlinkAction := make(map[string]interface{})
	var upstreamFlow openoltpb2.Flow
	var downstreamFlow openoltpb2.Flow

	// Fill Classfier
	uplinkClassifier[EthType] = uint32(EapEthType)
	uplinkClassifier[PacketTagType] = SingleTag
	uplinkClassifier[VlanVid] = vlanID
	// Fill action
	uplinkAction[TrapToHost] = true
	flowStoreCookie := getFlowStoreCookie(uplinkClassifier, gemPortID)
	//Add Uplink EAPOL Flow
	uplinkFlowID, err := f.resourceMgr.GetFlowID(intfID, onuID, uniID, gemPortID, flowStoreCookie, "", 0)
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
	networkIntfID := f.deviceHandler.nniIntfID
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
	if ok := f.addFlowToDevice(logicalFlow, &upstreamFlow); ok {
		log.Debug("EAPOL UL flow added to device successfully")
		flowCategory := "EAPOL"
		flowsToKVStore := f.getUpdatedFlowInfo(&upstreamFlow, flowStoreCookie, flowCategory, uplinkFlowID)
		if err := f.updateFlowInfoToKVStore(upstreamFlow.AccessIntfId,
			upstreamFlow.OnuId,
			upstreamFlow.UniId,
			upstreamFlow.FlowId,
			/* lowCategory, */
			flowsToKVStore); err != nil {
			log.Errorw("Error uploading EAPOL UL flow into KV store", log.Fields{"flow": upstreamFlow, "error": err})
			return
		}
	}
	// Dummy Downstream flow due to BAL 2.6 limitation
	{
		/* Add Downstream EAPOL Flow, Only for first EAP flow (BAL
		# requirement)
		# On one of the platforms (Broadcom BAL), when same DL classifier
		# vlan was used across multiple ONUs, eapol flow re-adds after
		# flow delete (cases of onu reboot/disable) fails.
		# In order to generate unique vlan, a combination of intf_id
		# onu_id and uniId is used.
		# uniId defaults to 0, so add 1 to it.
		*/
		log.Debugw("Creating DL EAPOL flow with default vlan", log.Fields{"vlan": vlanID})
		specialVlanDlFlow := 4090 - intfID*onuID*(uniID+1)
		// Assert that we do not generate invalid vlans under no condition
		if specialVlanDlFlow <= 2 {
			log.Fatalw("invalid-vlan-generated", log.Fields{"vlan": specialVlanDlFlow})
			return
		}
		log.Debugw("specialVlanEAPOLDlFlow:", log.Fields{"dl_vlan": specialVlanDlFlow})
		// Fill Classfier
		downlinkClassifier[PacketTagType] = SingleTag
		downlinkClassifier[EthType] = uint32(EapEthType)
		downlinkClassifier[VlanVid] = uint32(specialVlanDlFlow)
		// Fill action
		downlinkAction[PushVlan] = true
		downlinkAction[VlanVid] = vlanID
		flowStoreCookie := getFlowStoreCookie(downlinkClassifier, gemPortID)
		downlinkFlowID, err := f.resourceMgr.GetFlowID(intfID, onuID, uniID, gemPortID, flowStoreCookie, "", 0)
		if err != nil {
			log.Errorw("flowId unavailable for DL EAPOL",
				log.Fields{"intfId": intfID, "onuId": onuID, "flowStoreCookie": flowStoreCookie})
			return
		}
		log.Debugw("Creating DL EAPOL flow",
			log.Fields{"dl_classifier": downlinkClassifier, "dl_action": downlinkAction, "downlinkFlowID": downlinkFlowID})
		if classifierProto = makeOpenOltClassifierField(downlinkClassifier); classifierProto == nil {
			log.Error("Error in making classifier protobuf for downlink flow")
			return
		}
		if actionProto = makeOpenOltActionField(downlinkAction); actionProto == nil {
			log.Error("Error in making action protobuf for dl flow")
			return
		}
		// Downstream flow in grpc protobuf
		downstreamFlow = openoltpb2.Flow{AccessIntfId: int32(intfID),
			OnuId:         int32(onuID),
			UniId:         int32(uniID),
			FlowId:        downlinkFlowID,
			FlowType:      Downstream,
			AllocId:       int32(allocID),
			NetworkIntfId: int32(networkIntfID),
			GemportId:     int32(gemPortID),
			Classifier:    classifierProto,
			Action:        actionProto,
			Priority:      int32(logicalFlow.Priority),
			Cookie:        logicalFlow.Cookie,
			PortNo:        portNo}
		if ok := f.addFlowToDevice(logicalFlow, &downstreamFlow); ok {
			log.Debug("EAPOL DL flow added to device successfully")
			flowCategory := ""
			flowsToKVStore := f.getUpdatedFlowInfo(&downstreamFlow, flowStoreCookie, flowCategory, downlinkFlowID)
			if err := f.updateFlowInfoToKVStore(downstreamFlow.AccessIntfId,
				downstreamFlow.OnuId,
				downstreamFlow.UniId,
				downstreamFlow.FlowId,
				/* flowCategory, */
				flowsToKVStore); err != nil {
				log.Errorw("Error uploading EAPOL DL flow into KV store", log.Fields{"flow": upstreamFlow, "error": err})
				return
			}
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

// DeleteTechProfileInstance removes the tech profile instance from persistent storage
func (f *OpenOltFlowMgr) DeleteTechProfileInstance(intfID uint32, onuID uint32, uniID uint32, sn string) error {
	tpID := f.resourceMgr.GetTechProfileIDForOnu(intfID, onuID, uniID)
	uniPortName := fmt.Sprintf("pon-{%d}/onu-{%d}/uni-{%d}", intfID, onuID, uniID)
	if err := f.techprofile[intfID].DeleteTechProfileInstance(tpID, uniPortName); err != nil {
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
	return hash.Uint64()
}

func (f *OpenOltFlowMgr) getUpdatedFlowInfo(flow *openoltpb2.Flow, flowStoreCookie uint64, flowCategory string, deviceFlowID uint32) *[]rsrcMgr.FlowInfo {
	var flows = []rsrcMgr.FlowInfo{{Flow: flow, FlowCategory: flowCategory, FlowStoreCookie: flowStoreCookie}}
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
	existingFlows := f.resourceMgr.GetFlowIDInfo(intfID, uint32(flow.OnuId), uint32(flow.UniId), flow.FlowId)
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

func (f *OpenOltFlowMgr) updateFlowInfoToKVStore(intfID int32, onuID int32, uniID int32, flowID uint32, flows *[]rsrcMgr.FlowInfo) error {
	log.Debugw("Storing flow(s) into KV store", log.Fields{"flows": *flows})
	if err := f.resourceMgr.UpdateFlowIDInfo(intfID, onuID, uniID, flowID, flows); err != nil {
		log.Debug("Error while Storing flow into KV store")
		return err
	}
	log.Info("Stored flow(s) into KV store successfully!")
	return nil
}

func (f *OpenOltFlowMgr) addFlowToDevice(logicalFlow *ofp.OfpFlowStats, deviceFlow *openoltpb2.Flow) bool {

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
		log.Debug("Flow already exixts", log.Fields{"err": err, "deviceFlow": deviceFlow})
		return false
	}

	if err != nil {
		log.Errorw("Failed to Add flow to device", log.Fields{"err": err, "deviceFlow": deviceFlow})
		f.resourceMgr.FreeFlowID(intfID, deviceFlow.OnuId, deviceFlow.UniId, deviceFlow.FlowId)
		return false
	}
	log.Debugw("Flow added to device successfully ", log.Fields{"flow": *deviceFlow})
	f.registerFlow(logicalFlow, deviceFlow)
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

func (f *OpenOltFlowMgr) addLLDPFlow(flow *ofp.OfpFlowStats, portNo uint32) {

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
	if present := f.resourceMgr.IsFlowCookieOnKVStore(uint32(networkInterfaceID), uint32(onuID), uint32(uniID), flowStoreCookie); present {
		log.Debug("Flow-exists--not-re-adding")
		return
	}
	flowID, err := f.resourceMgr.GetFlowID(uint32(networkInterfaceID), uint32(onuID), uint32(uniID), uint32(gemPortID), flowStoreCookie, "", 0)

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
	if ok := f.addFlowToDevice(flow, &downstreamflow); ok {
		log.Debug("LLDP trap on NNI flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(&downstreamflow, flowStoreCookie, "", flowID)
		if err := f.updateFlowInfoToKVStore(int32(networkInterfaceID),
			int32(onuID),
			int32(uniID),
			flowID, flowsToKVStore); err != nil {
			log.Errorw("Error uploading LLDP flow into KV store", log.Fields{"flow": downstreamflow, "error": err})
		}
	}
	return
}

func getUniPortPath(intfID uint32, onuID uint32, uniID uint32) string {
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

func (f *OpenOltFlowMgr) clearFlowFromResourceManager(flow *ofp.OfpFlowStats, flowID uint32, flowDirection string) {
	log.Debugw("clearFlowFromResourceManager", log.Fields{"flowID": flowID, "flowDirection": flowDirection, "flow": *flow})
	portNum, ponIntf, onuID, uniID, inPort, ethType, err := FlowExtractInfo(flow, flowDirection)
	if err != nil {
		log.Error(err)
		return
	}
	log.Debugw("Extracted access info from flow to be deleted",
		log.Fields{"ponIntf": ponIntf, "onuID": onuID, "uniID": uniID, "flowID": flowID})

	if ethType == LldpEthType {
		var networkInterfaceID uint32
		var onuID = -1
		var uniID = -1

		networkInterfaceID = IntfIDFromNniPortNum(inPort)
		f.resourceMgr.FreeFlowID(networkInterfaceID, int32(onuID), int32(uniID), flowID)
		return
	}

	flowsInfo := f.resourceMgr.GetFlowIDInfo(ponIntf, onuID, uniID, flowID)
	if flowsInfo == nil {
		log.Debugw("No FlowInfo found found in KV store",
			log.Fields{"ponIntf": ponIntf, "onuID": onuID, "uniID": uniID, "flowID": flowID})
		return
	}
	var updatedFlows []rsrcMgr.FlowInfo

	for _, flow := range *flowsInfo {
		updatedFlows = append(updatedFlows, flow)
	}

	for i, storedFlow := range updatedFlows {
		if flowDirection == storedFlow.Flow.FlowType {
			//Remove the Flow from FlowInfo
			log.Debugw("Removing flow to be deleted", log.Fields{"flow": storedFlow})
			updatedFlows = append(updatedFlows[:i], updatedFlows[i+1:]...)
			break
		}
	}

	if len(updatedFlows) >= 0 {
		// There are still flows referencing the same flow_id.
		// So the flow should not be freed yet.
		// For ex: Case of HSIA where same flow is shared
		// between DS and US.
		f.updateFlowInfoToKVStore(int32(ponIntf), int32(onuID), int32(uniID), flowID, &updatedFlows)
		if len(updatedFlows) == 0 {
			log.Debugw("Releasing flow Id to resource manager", log.Fields{"ponIntf": ponIntf, "onuId": onuID, "uniId": uniID, "flowId": flowID})
			f.resourceMgr.FreeFlowID(ponIntf, int32(onuID), int32(uniID), flowID)
		}
	}
	flowIds := f.resourceMgr.GetCurrentFlowIDsForOnu(ponIntf, onuID, uniID)
	if len(flowIds) == 0 {
		log.Debugf("Flow count for subscriber %d is zero", onuID)
		kvstoreTpID := f.resourceMgr.GetTechProfileIDForOnu(ponIntf, onuID, uniID)
		if kvstoreTpID == 0 {
			log.Warnw("Could-not-find-techprofile-tableid-for-uni", log.Fields{"ponIntf": ponIntf, "onuId": onuID, "uniId": uniID})
			return
		}
		uni := getUniPortPath(ponIntf, onuID, uniID)
		tpPath := f.getTPpath(ponIntf, uni, kvstoreTpID)
		log.Debugw("Getting-techprofile-instance-for-subscriber", log.Fields{"TP-PATH": tpPath})
		techprofileInst, err := f.techprofile[ponIntf].GetTPInstanceFromKVStore(kvstoreTpID, tpPath)
		if err != nil { // This should not happen, something wrong in KV backend transaction
			log.Errorw("Error in fetching tech profile instance from KV store", log.Fields{"tpID": 20, "path": tpPath})
			return
		}
		if techprofileInst == nil {
			log.Errorw("Tech-profile-instance-does-not-exist-in-KV Store", log.Fields{"tpPath": tpPath})
			return
		}

		f.RemoveSchedulerQueues(tp_pb.Direction_UPSTREAM, ponIntf, onuID, uniID, portNum, techprofileInst)
		f.RemoveSchedulerQueues(tp_pb.Direction_DOWNSTREAM, ponIntf, onuID, uniID, portNum, techprofileInst)
	} else {
		log.Debugf("Flow ids for subscriber", log.Fields{"onu": onuID, "flows": flowIds})
	}
}

//RemoveFlow removes the flow from the device
func (f *OpenOltFlowMgr) RemoveFlow(flow *ofp.OfpFlowStats) {
	log.Debugw("Removing Flow", log.Fields{"flow": flow})
	var deviceFlowsToRemove []ofp.OfpFlowStats
	var deletedFlowsIdx []int
	for _, curFlow := range f.storedDeviceFlows {
		if curFlow.Cookie == flow.Id {
			log.Debugw("Found found matching flow-cookie", log.Fields{"curFlow": curFlow})
			deviceFlowsToRemove = append(deviceFlowsToRemove, curFlow)
		}
	}
	log.Debugw("Flows to be deleted", log.Fields{"deviceFlowsToRemove": deviceFlowsToRemove})
	for index, curFlow := range deviceFlowsToRemove {
		id, direction := f.decodeStoredID(curFlow.GetId())
		removeFlowMessage := openoltpb2.Flow{FlowId: uint32(id), FlowType: direction}
		if ok := f.removeFlowFromDevice(&removeFlowMessage); ok {
			log.Debug("Flow removed from device successfully")
			deletedFlowsIdx = append(deletedFlowsIdx, index)
			f.clearFlowFromResourceManager(flow, uint32(id), direction) //TODO: Take care of the limitations
		}

	}
	// Can be done in separate go routine as it takes time ?
	for _, flowToRemove := range deletedFlowsIdx {
		for index, storedFlow := range f.storedDeviceFlows {
			if deviceFlowsToRemove[flowToRemove].Cookie == storedFlow.Cookie {
				log.Debugw("Removing flow from local Store", log.Fields{"flow": storedFlow})
				f.storedDeviceFlows = append(f.storedDeviceFlows[:index], f.storedDeviceFlows[index+1:]...)
				break
			}
		}
	}
	log.Debugw("Flows removed from the data store",
		log.Fields{"number_of_flows_removed": len(deviceFlowsToRemove), "updated_stored_flows": f.storedDeviceFlows})
	return
}

// AddFlow add flow to device
func (f *OpenOltFlowMgr) AddFlow(flow *ofp.OfpFlowStats, flowMetadata *voltha.FlowMetadata) {
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
			f.addLLDPFlow(flow, portNo)
			return
		}
	}
	if ipProto, ok := classifierInfo[IPProto]; ok {
		if ipProto.(uint32) == IPProtoDhcp {
			if udpSrc, ok := classifierInfo[UDPSrc]; ok {
				if udpSrc.(uint32) == uint32(67) {
					log.Debug("trap-dhcp-from-nni-flow")
					f.addDHCPTrapFlowOnNNI(flow, classifierInfo, portNo)
					return
				}
			}
		}
	}
	/* Metadata 8 bytes:
	    Most Significant 2 Bytes = Inner VLAN
	    Next 2 Bytes = Tech Profile ID(TPID)
	    Least Significant 4 Bytes = Port ID
	   Flow Metadata carries Tech-Profile (TP) ID and is mandatory in all
	   subscriber related flows.
	*/
	metadata := utils.GetMetadataFromWriteMetadataAction(flow)
	if metadata == 0 {
		log.Error("Metadata is not present in flow which is mandatory")
		return
	}
	TpID := utils.GetTechProfileIDFromWriteMetaData(metadata)
	kvstoreTpID := f.resourceMgr.GetTechProfileIDForOnu(intfID, onuID, uniID)
	if kvstoreTpID == 0 {
		log.Debugf("tpid-not-present-in-kvstore, using tp id %d from flow metadata", TpID)
	} else if kvstoreTpID != uint32(TpID) {
		log.Error(" Tech-profile-updates-not-supported", log.Fields{"Tpid-in-flow": TpID, "kvstore-TpId": kvstoreTpID})
		return
	}
	log.Debugw("TPID for this subcriber", log.Fields{"TpId": TpID, "pon": intfID, "onuID": onuID, "uniID": uniID})
	if IsUpstream(actionInfo[Output].(uint32)) {
		UsMeterID = utils.GetMeterIdFromFlow(flow)
		log.Debugw("Upstream-flow-meter-id", log.Fields{"UsMeterID": UsMeterID})
	} else {
		DsMeterID = utils.GetMeterIdFromFlow(flow)
		log.Debugw("Downstream-flow-meter-id", log.Fields{"DsMeterID": DsMeterID})

	}
	f.divideAndAddFlow(intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, uint32(TpID), UsMeterID, DsMeterID, flowMetadata)
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

//UpdateOnuInfo function adds onu info to cache
func (f *OpenOltFlowMgr) UpdateOnuInfo(intfID uint32, onuID uint32, serialNum string) {
	onu := onuInfo{intfID: intfID, onuID: onuID, serialNumber: serialNum}
	onuIDkey := onuIDKey{intfID: intfID, onuID: onuID}
	f.onuIds[onuIDkey] = onu
	log.Debugw("Updated onuinfo", log.Fields{"intfID": intfID, "onuID": onuID, "serialNum": serialNum})
}

//addGemPortToOnuInfoMap function stores adds GEMport to ONU map
func (f *OpenOltFlowMgr) addGemPortToOnuInfoMap(intfID uint32, onuID uint32, gemPort uint32) {
	onuIDkey := onuIDKey{intfID: intfID, onuID: onuID}
	if val, ok := f.onuIds[onuIDkey]; ok {
		onuInfo := val
		gemportKey := gemPortKey{intfID: intfID, gemPort: gemPort}
		f.onuGemPortIds[gemportKey] = onuInfo
		log.Debugw("Cached Gemport to Onuinfo map", log.Fields{"GemPort": gemPort, "intfId": onuInfo.intfID, "onuId": onuInfo.onuID})
		return
	}
	log.Errorw("OnuInfo not found", log.Fields{"intfId": intfID, "onuId": onuID, "gemPort": gemPort})
}

// This function Lookup maps  by serialNumber or (intfId, gemPort)

//getOnuIDfromGemPortMap Returns OnuID,nil if found or set 0,error if no onuId is found for serialNumber or (intfId, gemPort)
func (f *OpenOltFlowMgr) getOnuIDfromGemPortMap(serialNumber string, intfID uint32, gemPortID uint32) (uint32, error) {
	log.Debugw("Getting ONU ID from GEM port and PON port", log.Fields{"serialNumber": serialNumber, "intfId": intfID, "gemPortId": gemPortID})
	if serialNumber != "" {
		if onuInfo, ok := f.onuSerialNumbers[serialNumber]; ok {
			return onuInfo.onuID, nil
		}
	} else {
		gemportKey := gemPortKey{intfID: intfID, gemPort: gemPortID}
		if onuInfo, ok := f.onuGemPortIds[gemportKey]; ok {
			log.Debugw("Retrieved onu info from access", log.Fields{"intfId": intfID, "gemPortId": gemPortID, "onuId": onuInfo.onuID})
			return onuInfo.onuID, nil
		}
	}
	log.Errorw("onuid is not found", log.Fields{"serialNumber": serialNumber, "intfId": intfID, "gemPort": gemPortID})
	return uint32(0), errors.New("key error, onuid is not found") // ONU ID 0 is not a valid one
}

//GetLogicalPortFromPacketIn function computes logical port UNI/NNI port from packet-in indication and returns the same
func (f *OpenOltFlowMgr) GetLogicalPortFromPacketIn(packetIn *openoltpb2.PacketIndication) (uint32, error) {
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
		pktInkey := packetInInfoKey{intfID: packetIn.IntfId, onuID: onuID, logicalPort: logicalPortNum}
		f.packetInGemPort[pktInkey] = packetIn.GemportId
	} else if packetIn.IntfType == "nni" {
		logicalPortNum = IntfIDToPortNo(packetIn.IntfId, voltha.Port_ETHERNET_NNI)
	}
	log.Debugw("Retrieved logicalport from  packet-in", log.Fields{"logicalPortNum": logicalPortNum, "IntfType": packetIn.IntfType})
	return logicalPortNum, nil
}

//GetPacketOutGemPortID returns gemPortId
func (f *OpenOltFlowMgr) GetPacketOutGemPortID(intfID uint32, onuID uint32, portNum uint32) (uint32, error) {
	var gemPortID uint32
	var err error
	key := packetInInfoKey{intfID: intfID, onuID: onuID, logicalPort: portNum}
	if val, ok := f.packetInGemPort[key]; ok {
		gemPortID = val
	} else {
		log.Errorw("Key-Error while fetching packet-out GEM port", log.Fields{"key": key})
		err = errors.New("key-error while fetching packet-out GEM port")
	}
	return gemPortID, err
}

func installFlowOnAllGemports(
	f1 func(intfId uint32, onuId uint32, uniId uint32,
		portNo uint32, classifier map[string]interface{}, action map[string]interface{},
		logicalFlow *ofp.OfpFlowStats, allocId uint32, gemPortId uint32),
	f2 func(intfId uint32, onuId uint32, uniId uint32, portNo uint32,
		logicalFlow *ofp.OfpFlowStats, allocId uint32, gemPortId uint32, vlanId uint32),
	args map[string]uint32,
	classifier map[string]interface{}, action map[string]interface{},
	logicalFlow *ofp.OfpFlowStats,
	gemPorts []uint32,
	FlowType string,
	vlanID ...uint32) {
	log.Debugw("Installing flow on all GEM ports", log.Fields{"FlowType": FlowType, "gemPorts": gemPorts, "vlan": vlanID})
	for _, gemPortID := range gemPorts {
		if FlowType == HsiaFlow || FlowType == DhcpFlow {
			f1(args["intfId"], args["onuId"], args["uniId"], args["portNo"], classifier, action, logicalFlow, args["allocId"], gemPortID)
		} else if FlowType == EapolFlow {
			f2(args["intfId"], args["onuId"], args["uniId"], args["portNo"], logicalFlow, args["allocId"], gemPortID, vlanID[0])
		} else {
			log.Errorw("Unrecognized Flow Type", log.Fields{"FlowType": FlowType})
			return
		}
	}
}

func (f *OpenOltFlowMgr) addDHCPTrapFlowOnNNI(logicalFlow *ofp.OfpFlowStats, classifier map[string]interface{}, portNo uint32) {
	log.Debug("Adding trap-dhcp-of-nni-flow")
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
	networkInterfaceID := f.deviceHandler.nniIntfID
	flowStoreCookie := getFlowStoreCookie(classifier, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(uint32(networkInterfaceID), uint32(onuID), uint32(uniID), flowStoreCookie); present {
		log.Debug("Flow-exists--not-re-adding")
		return
	}
	flowID, err := f.resourceMgr.GetFlowID(uint32(networkInterfaceID), uint32(onuID), uint32(uniID), uint32(gemPortID), flowStoreCookie, "", 0)
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
	if ok := f.addFlowToDevice(logicalFlow, &downstreamflow); ok {
		log.Debug("DHCP trap on NNI flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(&downstreamflow, flowStoreCookie, "", flowID)
		if err := f.updateFlowInfoToKVStore(int32(networkInterfaceID),
			int32(onuID),
			int32(uniID),
			flowID, flowsToKVStore); err != nil {
			log.Errorw("Error uploading DHCP DL  flow into KV store", log.Fields{"flow": downstreamflow, "error": err})
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

func (f *OpenOltFlowMgr) checkAndAddFlow(args map[string]uint32, classifierInfo map[string]interface{},
	actionInfo map[string]interface{}, flow *ofp.OfpFlowStats, gemPort, intfID, onuID, uniID, portNo uint32,
	TpInst *tp.TechProfile, allocID []uint32, gemPorts []uint32, TpID uint32, uni string) {
	if ipProto, ok := classifierInfo[IPProto]; ok {
		if ipProto.(uint32) == IPProtoDhcp {
			log.Info("Adding DHCP flow")
			if pcp, ok := classifierInfo[VlanPcp]; ok {
				gemPort = f.techprofile[intfID].GetGemportIDForPbit(TpInst,
					tp_pb.Direction_UPSTREAM,
					pcp.(uint32))
				//Adding DHCP upstream flow
				f.addDHCPTrapFlow(intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID[0], gemPort)
			} else {
				//Adding DHCP upstream flow to all gemports
				installFlowOnAllGemports(f.addDHCPTrapFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, DhcpFlow)
			}

		} else if ipProto == IgmpProto {
			log.Info("igmp flow add ignored, not implemented yet")
			return
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

				f.addEAPOLFlow(intfID, onuID, uniID, portNo, flow, allocID[0], gemPort, vlanID)
			} else {
				installFlowOnAllGemports(nil, f.addEAPOLFlow, args, classifierInfo, actionInfo, flow, gemPorts, EapolFlow, vlanID)
			}
		}
	} else if _, ok := actionInfo[PushVlan]; ok {
		log.Info("Adding upstream data rule")
		if pcp, ok := classifierInfo[VlanPcp]; ok {
			gemPort = f.techprofile[intfID].GetGemportIDForPbit(TpInst,
				tp_pb.Direction_UPSTREAM,
				pcp.(uint32))
			//Adding HSIA upstream flow
			f.addUpstreamDataFlow(intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID[0], gemPort)
		} else {
			//Adding HSIA upstream flow to all gemports
			installFlowOnAllGemports(f.addUpstreamDataFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, HsiaFlow)
		}
	} else if _, ok := actionInfo[PopVlan]; ok {
		log.Info("Adding Downstream data rule")
		if pcp, ok := classifierInfo[VlanPcp]; ok {
			gemPort = f.techprofile[intfID].GetGemportIDForPbit(TpInst,
				tp_pb.Direction_DOWNSTREAM,
				pcp.(uint32))
			//Adding HSIA downstream flow
			f.addDownstreamDataFlow(intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID[0], gemPort)
		} else {
			//Adding HSIA downstream flow to all gemports
			installFlowOnAllGemports(f.addDownstreamDataFlow, nil, args, classifierInfo, actionInfo, flow, gemPorts, HsiaFlow)
		}
	} else {
		log.Errorw("Invalid-flow-type-to-handle", log.Fields{"classifier": classifierInfo, "action": actionInfo, "flow": flow})
		return
	}
	// Send Techprofile download event to child device in go routine as it takes time
	go f.sendTPDownloadMsgToChild(intfID, onuID, uniID, uni, TpID)
}

func formulateClassifierInfoFromFlow(classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) {
	for _, field := range utils.GetOfbFields(flow) {
		if field.Type == utils.ETH_TYPE {
			classifierInfo[EthType] = field.GetEthType()
			log.Debug("field-type-eth-type", log.Fields{"classifierInfo[ETH_TYPE]": classifierInfo[EthType].(uint32)})
		} else if field.Type == utils.IP_PROTO {
			classifierInfo[IPProto] = field.GetIpProto()
			log.Debug("field-type-ip-proto", log.Fields{"classifierInfo[IP_PROTO]": classifierInfo[IPProto].(uint32)})
		} else if field.Type == utils.IN_PORT {
			classifierInfo[InPort] = field.GetPort()
			log.Debug("field-type-in-port", log.Fields{"classifierInfo[IN_PORT]": classifierInfo[InPort].(uint32)})
		} else if field.Type == utils.VLAN_VID {
			classifierInfo[VlanVid] = field.GetVlanVid()
			log.Debug("field-type-vlan-vid", log.Fields{"classifierInfo[VLAN_VID]": classifierInfo[VlanVid].(uint32)})
		} else if field.Type == utils.VLAN_PCP {
			classifierInfo[VlanPcp] = field.GetVlanPcp()
			log.Debug("field-type-vlan-pcp", log.Fields{"classifierInfo[VLAN_PCP]": classifierInfo[VlanPcp].(uint32)})
		} else if field.Type == utils.UDP_DST {
			classifierInfo[UDPDst] = field.GetUdpDst()
			log.Debug("field-type-udp-dst", log.Fields{"classifierInfo[UDP_DST]": classifierInfo[UDPDst].(uint32)})
		} else if field.Type == utils.UDP_SRC {
			classifierInfo[UDPSrc] = field.GetUdpSrc()
			log.Debug("field-type-udp-src", log.Fields{"classifierInfo[UDP_SRC]": classifierInfo[UDPSrc].(uint32)})
		} else if field.Type == utils.IPV4_DST {
			classifierInfo[Ipv4Dst] = field.GetIpv4Dst()
			log.Debug("field-type-ipv4-dst", log.Fields{"classifierInfo[IPV4_DST]": classifierInfo[Ipv4Dst].(uint32)})
		} else if field.Type == utils.IPV4_SRC {
			classifierInfo[Ipv4Src] = field.GetIpv4Src()
			log.Debug("field-type-ipv4-src", log.Fields{"classifierInfo[IPV4_SRC]": classifierInfo[Ipv4Src].(uint32)})
		} else if field.Type == utils.METADATA {
			classifierInfo[Metadata] = field.GetTableMetadata()
			log.Debug("field-type-metadata", log.Fields{"classifierInfo[Metadata]": classifierInfo[Metadata].(uint64)})
		} else if field.Type == utils.TUNNEL_ID {
			classifierInfo[TunnelID] = field.GetTunnelId()
			log.Debug("field-type-tunnelId", log.Fields{"classifierInfo[TUNNEL_ID]": classifierInfo[TunnelID].(uint64)})
		} else {
			log.Errorw("Un supported field type", log.Fields{"type": field.Type})
			return
		}
	}
}

func formulateActionInfoFromFlow(actionInfo, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) error {
	for _, action := range utils.GetActions(flow) {
		if action.Type == utils.OUTPUT {
			if out := action.GetOutput(); out != nil {
				actionInfo[Output] = out.GetPort()
				log.Debugw("action-type-output", log.Fields{"out_port": actionInfo[Output].(uint32)})
			} else {
				log.Error("Invalid output port in action")
				return errors.New("invalid output port in action")
			}
		} else if action.Type == utils.POP_VLAN {
			actionInfo[PopVlan] = true
			log.Debugw("action-type-pop-vlan", log.Fields{"in_port": classifierInfo[InPort].(uint32)})
		} else if action.Type == utils.PUSH_VLAN {
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
		} else if action.Type == utils.SET_FIELD {
			if out := action.GetSetField(); out != nil {
				if field := out.GetField(); field != nil {
					if ofClass := field.GetOxmClass(); ofClass != ofp.OfpOxmClass_OFPXMC_OPENFLOW_BASIC {
						log.Errorw("Invalid openflow class", log.Fields{"class": ofClass})
						return errors.New("invalid openflow class")
					}
					/*log.Debugw("action-type-set-field",log.Fields{"field": field, "in_port": classifierInfo[IN_PORT].(uint32)})*/
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
			}
		} else {
			log.Errorw("Un supported action type", log.Fields{"type": action.Type})
			return errors.New("un supported action type")
		}
	}
	return nil
}

func formulateControllerBoundTrapFlowInfo(actionInfo, classifierInfo map[string]interface{}, flow *ofp.OfpFlowStats) error {
	if isControllerFlow := IsControllerBoundFlow(actionInfo[Output].(uint32)); isControllerFlow {
		log.Debug("Controller bound trap flows, getting inport from tunnelid")
		/* Get UNI port/ IN Port from tunnel ID field for upstream controller bound flows  */
		if portType := IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := utils.GetChildPortFromTunnelId(flow); uniPort != 0 {
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
			if uniPort := utils.GetChildPortFromTunnelId(flow); uniPort != 0 {
				actionInfo[Output] = uniPort
				log.Debugw("downstream-nni-to-pon-port-flow, outport-in-tunnelid", log.Fields{"newOutPort": actionInfo[Output].(uint32), "outPort": actionInfo[Output].(uint32)})
			} else {
				log.Debug("downstream-nni-to-pon-port-flow, no-outport-in-tunnelid", log.Fields{"InPort": classifierInfo[InPort].(uint32), "outPort": actionInfo[Output].(uint32)})
				return errors.New("downstream-nni-to-pon-port-flow, no-outport-in-tunnelid")
			}
			// Upstream flow from PON to NNI port , Use tunnel ID as new IN port / UNI port
		} else if portType := IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := utils.GetChildPortFromTunnelId(flow); uniPort != 0 {
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
