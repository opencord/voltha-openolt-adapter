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
	"github.com/opencord/voltha-go/common/log"
	tp "github.com/opencord/voltha-go/common/techprofile"
	"github.com/opencord/voltha-go/rw_core/utils"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	ic "github.com/opencord/voltha-protos/go/inter_container"
	ofp "github.com/opencord/voltha-protos/go/openflow_13"
	openoltpb2 "github.com/opencord/voltha-protos/go/openolt"
	"github.com/opencord/voltha-protos/go/voltha"
	"math/big"
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

	//DefaultMgmtVlan default vlan value
	DefaultMgmtVlan = 4091

	// Openolt Flow

	//UPSTREAM constant
	UPSTREAM = "upstream"
	//DOWNSTREAM constant
	DOWNSTREAM = "downstream"
	//PacketTagType constant
	PacketTagType = "pkt_tag_type"
	//UNTAGGED constant
	UNTAGGED = "untagged"
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
	//METADATA constant
	METADATA = "metadata"
	//TunnelID constant
	TunnelID = "tunnel_id"
	//OUTPUT constant
	OUTPUT = "output"
	// Actions

	//PopVlan constant
	PopVlan = "pop_vlan"
	//PushVlan constant
	PushVlan = "push_vlan"
	//TrapToHost constant
	TrapToHost = "trap_to_host"
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
	if direction == UPSTREAM {
		log.Debug("upstream flow, shifting id")
		return 0x1<<15 | uint64(flowID), nil
	} else if direction == DOWNSTREAM {
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

func (f *OpenOltFlowMgr) divideAndAddFlow(intfID uint32, onuID uint32, uniID uint32, portNo uint32, classifierInfo map[string]interface{}, actionInfo map[string]interface{}, flow *ofp.OfpFlowStats) {
	var allocID []uint32
	var gemPorts []uint32

	log.Infow("Dividing flow", log.Fields{"intfId": intfID, "onuId": onuID, "uniId": uniID, "portNo": portNo, "classifier": classifierInfo, "action": actionInfo})

	log.Infow("sorting flow", log.Fields{"intfId": intfID, "onuId": onuID, "uniId": uniID, "portNo": portNo,
		"classifierInfo": classifierInfo, "actionInfo": actionInfo})

	uni := getUniPortPath(intfID, onuID, uniID)
	log.Debugw("Uni port name", log.Fields{"uni": uni})
	allocID, gemPorts = f.createTcontGemports(intfID, onuID, uniID, uni, portNo, flow.GetTableId())
	if allocID == nil || gemPorts == nil {
		log.Error("alloc-id-gem-ports-unavailable")
		return
	}

	/* Flows can't be added specific to gemport unless p-bits are received.
	 * Hence adding flows for all gemports
	 */
	for _, gemPort := range gemPorts {
		if ipProto, ok := classifierInfo[IPProto]; ok {
			if ipProto.(uint32) == IPProtoDhcp {
				log.Info("Adding DHCP flow")
				f.addDHCPTrapFlow(intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID[0], gemPort)
			} else if ipProto == IPProtoIgmp {
				log.Info("igmp flow add ignored, not implemented yet")
			} else {
				log.Errorw("Invalid-Classifier-to-handle", log.Fields{"classifier": classifierInfo, "action": actionInfo})
				//return errors.New("Invalid-Classifier-to-handle")
			}
		} else if ethType, ok := classifierInfo[EthType]; ok {
			if ethType.(uint32) == EapEthType {
				log.Info("Adding EAPOL flow")
				f.addEAPOLFlow(intfID, onuID, uniID, portNo, flow, allocID[0], gemPort, DefaultMgmtVlan)
				if vlan := getSubscriberVlan(utils.GetInPort(flow)); vlan != 0 {
					f.addEAPOLFlow(intfID, onuID, uniID, portNo, flow, allocID[0], gemPort, vlan)
				}
				// Send Techprofile download event to child device in go routine as it takes time
				go f.sendTPDownloadMsgToChild(intfID, onuID, uniID, uni)
			}
			if ethType == LldpEthType {
				log.Info("Adding LLDP flow")
				addLLDPFlow(flow, portNo)
			}
		} else if _, ok := actionInfo[PushVlan]; ok {
			log.Info("Adding upstream data rule")
			f.addUpstreamDataFlow(intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID[0], gemPort)
		} else if _, ok := actionInfo[PopVlan]; ok {
			log.Info("Adding Downstream data rule")
			f.addDownstreamDataFlow(intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow, allocID[0], gemPort)
		} else {
			log.Errorw("Invalid-flow-type-to-handle", log.Fields{"classifier": classifierInfo, "action": actionInfo, "flow": flow})
		}
	}
}

// This function allocates tconts and GEM ports for an ONU, currently one TCONT is supported per ONU
func (f *OpenOltFlowMgr) createTcontGemports(intfID uint32, onuID uint32, uniID uint32, uni string, uniPort uint32, tableID uint32) ([]uint32, []uint32) {
	var allocID []uint32
	var gemPortIDs []uint32
	//If we already have allocated earlier for this onu, render them
	if tcontID := f.resourceMgr.GetCurrentAllocIDForOnu(intfID, onuID, uniID); tcontID != 0 {
		allocID = append(allocID, tcontID)
	}
	gemPortIDs = f.resourceMgr.GetCurrentGEMPortIDsForOnu(intfID, onuID, uniID)
	if len(allocID) != 0 && len(gemPortIDs) != 0 {
		log.Debug("Rendered Tcont and GEM ports from resource manager", log.Fields{"intfId": intfID, "onuId": onuID, "uniPort": uniID,
			"allocID": allocID, "gemPortIDs": gemPortIDs})
		return allocID, gemPortIDs
	}
	log.Debug("Creating New TConts and Gem ports", log.Fields{"pon": intfID, "onu": onuID, "uni": uniID})

	//FIXME: If table id is <= 63 using 64 as table id
	if tableID < tp.DEFAULT_TECH_PROFILE_TABLE_ID {
		tableID = tp.DEFAULT_TECH_PROFILE_TABLE_ID
	}
	tpPath := f.getTPpath(intfID, uni)
	// Check tech profile instance already exists for derived port name
	techProfileInstance, err := f.techprofile[intfID].GetTPInstanceFromKVStore(tableID, tpPath)
	if err != nil { // This should not happen, something wrong in KV backend transaction
		log.Errorw("Error in fetching tech profile instance from KV store", log.Fields{"tableID": tableID, "path": tpPath})
		return nil, nil
	}
	if techProfileInstance == nil {
		log.Info("Creating tech profile instance", log.Fields{"path": tpPath})
		techProfileInstance = f.techprofile[intfID].CreateTechProfInstance(tableID, uni, intfID)
		if techProfileInstance == nil {
			log.Error("Tech-profile-instance-creation-failed")
			return nil, nil
		}
	} else {
		log.Debugw("Tech-profile-instance-already-exist-for-given port-name", log.Fields{"uni": uni})
	}
	// Get upstream and downstream scheduler protos
	usScheduler := f.techprofile[intfID].GetUsScheduler(techProfileInstance)
	dsScheduler := f.techprofile[intfID].GetDsScheduler(techProfileInstance)
	// Get TCONTS protos
	tconts := f.techprofile[intfID].GetTconts(techProfileInstance, usScheduler, dsScheduler)
	if len(tconts) == 0 {
		log.Error("TCONTS not found ")
		return nil, nil
	}
	log.Debugw("Sending Create tcont to device",
		log.Fields{"onu": onuID, "uni": uniID, "portNo": "", "tconts": tconts})
	if _, err := f.deviceHandler.Client.CreateTconts(context.Background(),
		&openoltpb2.Tconts{IntfId: intfID,
			OnuId:  onuID,
			UniId:  uniID,
			PortNo: uniPort,
			Tconts: tconts}); err != nil {
		log.Errorw("Error while creating TCONT in device", log.Fields{"error": err})
		return nil, nil
	}
	allocID = append(allocID, techProfileInstance.UsScheduler.AllocID)
	for _, gem := range techProfileInstance.UpstreamGemPortAttributeList {
		gemPortIDs = append(gemPortIDs, gem.GemportID)
	}
	log.Debugw("Allocated Tcont and GEM ports", log.Fields{"allocID": allocID, "gemports": gemPortIDs})
	// Send Tconts and GEM ports to KV store
	f.storeTcontsGEMPortsIntoKVStore(intfID, onuID, uniID, allocID, gemPortIDs)
	return allocID, gemPortIDs
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
	for _, techRange := range f.resourceMgr.DevInfo.Ranges {
		for intfID := range techRange.IntfIds {
			f.techprofile = append(f.techprofile, f.resourceMgr.ResourceMgrs[uint32(intfID)].TechProfileMgr)
		}
	}
	//Make sure we have as many tech_profiles as there are pon ports on the device
	if len(f.techprofile) != int(f.resourceMgr.DevInfo.GetPonPorts()) {
		log.Errorw("Error while populating techprofile",
			log.Fields{"numofTech": len(f.techprofile), "numPonPorts": f.resourceMgr.DevInfo.GetPonPorts()})
		return errors.New("error while populating techprofile mgrs")
	}
	log.Infow("Populated techprofile per ponport successfully",
		log.Fields{"numofTech": len(f.techprofile), "numPonPorts": f.resourceMgr.DevInfo.GetPonPorts()})
	return nil
}

func (f *OpenOltFlowMgr) addUpstreamDataFlow(intfID uint32, onuID uint32, uniID uint32,
	portNo uint32, uplinkClassifier map[string]interface{},
	uplinkAction map[string]interface{}, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemportID uint32) {
	uplinkClassifier[PacketTagType] = SingleTag
	log.Debugw("Adding upstream data flow", log.Fields{"uplinkClassifier": uplinkClassifier, "uplinkAction": uplinkAction})
	f.addHSIAFlow(intfID, onuID, uniID, portNo, uplinkClassifier, uplinkAction,
		UPSTREAM, logicalFlow, allocID, gemportID)
	/* TODO: Install Secondary EAP on the subscriber vlan */
}

func (f *OpenOltFlowMgr) addDownstreamDataFlow(intfID uint32, onuID uint32, uniID uint32,
	portNo uint32, downlinkClassifier map[string]interface{},
	downlinkAction map[string]interface{}, logicalFlow *ofp.OfpFlowStats,
	allocID uint32, gemportID uint32) {
	downlinkClassifier[PacketTagType] = DoubleTag
	log.Debugw("Adding downstream data flow", log.Fields{"downlinkClassifier": downlinkClassifier,
		"downlinkAction": downlinkAction})
	// Ignore private VLAN flow given by decomposer, cannot do anything with this flow
	if uint32(downlinkClassifier[METADATA].(uint64)) == MkUniPortNum(intfID, onuID, uniID) &&
		downlinkClassifier[VlanVid] == (uint32(ofp.OfpVlanId_OFPVID_PRESENT)|4000) {
		log.Infow("EAPOL DL flow , Already added ,ignoring it", log.Fields{"downlinkClassifier": downlinkClassifier,
			"downlinkAction": downlinkAction})
		return
	}
	/* Already this info available classifier? */
	downlinkAction[PopVlan] = true
	downlinkAction[VlanVid] = downlinkClassifier[VlanVid]
	f.addHSIAFlow(intfID, onuID, uniID, portNo, downlinkClassifier, downlinkAction,
		DOWNSTREAM, logicalFlow, allocID, gemportID)
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
	flowCategory := "HSIA"
	flowStoreCookie := getFlowStoreCookie(classifier, gemPortID)
	flowID, err := f.resourceMgr.GetFlowID(intfID, onuID, uniID, flowStoreCookie, flowCategory)
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
		flowsToKVStore := f.getUpdatedFlowInfo(&flow, flowStoreCookie, "HSIA", flowID)
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

	flowID, err := f.resourceMgr.GetFlowID(intfID, onuID, uniID, flowStoreCookie, "")

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
		FlowType:      UPSTREAM,
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
	uplinkFlowID, err := f.resourceMgr.GetFlowID(intfID, onuID, uniID, flowStoreCookie, "")
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
		FlowType:      UPSTREAM,
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

	if vlanID == DefaultMgmtVlan {
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
		downlinkClassifier[VlanVid] = uint32(specialVlanDlFlow)
		// Fill action
		downlinkAction[PushVlan] = true
		downlinkAction[VlanVid] = vlanID
		flowStoreCookie := getFlowStoreCookie(downlinkClassifier, gemPortID)
		downlinkFlowID, err := f.resourceMgr.GetFlowID(intfID, onuID, uniID, flowStoreCookie, "")
		if err != nil {
			log.Errorw("flowId unavailable for DL EAPOL",
				log.Fields{"intfId": intfID, "onuId": onuID, "flowStoreCookie": flowStoreCookie})
			return
		}
		log.Debugw("Creating DL EAPOL flow",
			log.Fields{"dl_classifier": downlinkClassifier, "dl_action": downlinkAction, "downlinkFlowId": downlinkFlowID})
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
			FlowType:      DOWNSTREAM,
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
	} else {
		log.Infow("EAPOL flow with non-default mgmt vlan is not supported", log.Fields{"vlanId": vlanID})
		return
	}
	log.Debugw("Added EAPOL flows to device successfully", log.Fields{"flow": logicalFlow})
}

func makeOpenOltClassifierField(classifierInfo map[string]interface{}) *openoltpb2.Classifier {
	var classifier openoltpb2.Classifier
	if etherType, ok := classifierInfo[EthType]; ok {
		classifier.EthType = etherType.(uint32)
	}
	if ipProto, ok := classifierInfo[IPProto]; ok {
		classifier.IpProto = ipProto.(uint32)
	}
	if vlanID, ok := classifierInfo[VlanVid]; ok {
		classifier.OVid = (vlanID.(uint32)) & 0xFFF
	}
	if metadata, ok := classifierInfo[METADATA]; ok { // TODO: Revisit
		classifier.IVid = uint32(metadata.(uint64))
	}
	if vlanPcp, ok := classifierInfo[VlanPcp]; ok {
		classifier.OPbits = vlanPcp.(uint32)
	}
	if udpSrc, ok := classifierInfo[UDPSrc]; ok {
		classifier.SrcPort = udpSrc.(uint32)
	}
	if udpDst, ok := classifierInfo[UDPDst]; ok {
		classifier.DstPort = udpDst.(uint32)
	}
	if ipv4Dst, ok := classifierInfo[Ipv4Dst]; ok {
		classifier.DstIp = ipv4Dst.(uint32)
	}
	if ipv4Src, ok := classifierInfo[Ipv4Src]; ok {
		classifier.SrcIp = ipv4Src.(uint32)
	}
	if pktTagType, ok := classifierInfo[PacketTagType]; ok {
		if pktTagType.(string) == SingleTag {
			classifier.PktTagType = SingleTag
		} else if pktTagType.(string) == DoubleTag {
			classifier.PktTagType = DoubleTag
		} else if pktTagType.(string) == UNTAGGED {
			classifier.PktTagType = UNTAGGED
		} else {
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

func (f *OpenOltFlowMgr) getTPpath(intfID uint32, uni string) string {
	/*
	   FIXME
	   Should get Table id form the flow, as of now hardcoded to DEFAULT_TECH_PROFILE_TABLE_ID (64)
	   'tp_path' contains the suffix part of the tech_profile_instance path. The prefix to the 'tp_path' should be set to
	   TechProfile.KV_STORE_TECH_PROFILE_PATH_PREFIX by the ONU adapter.
	*/
	return f.techprofile[intfID].GetTechProfileInstanceKVPath(tp.DEFAULT_TECH_PROFILE_TABLE_ID, uni)
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
		f.resourceMgr.FreeFlowID(intfID, uint32(deviceFlow.OnuId), uint32(deviceFlow.UniId), deviceFlow.FlowId)
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

	if direction == UPSTREAM{
		log.Debug("Upstream flow shifting flowid")
		return ((0x1 << 15) | flowId)
	}else if direction == DOWNSTREAM{
		log.Debug("Downstream flow not shifting flowid")
		return flowId
	}else{
		log.Errorw("Unrecognized direction",log.Fields{"direction": direction})
		return flowId
	}
}

*/

func addLLDPFlow(flow *ofp.OfpFlowStats, portNo uint32) {
	log.Info("unimplemented flow : %v, portNo : %v ", flow, portNo)
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

func getSubscriberVlan(inPort uint32) uint32 {
	/* For EAPOL case we will use default VLAN , so will implement later if required */
	log.Info("unimplemented inport %v", inPort)
	return 0
}

func (f *OpenOltFlowMgr) clearFlowsAndSchedulerForLogicalPort(childDevice *voltha.Device, logicalPort *voltha.LogicalPort) {
	log.Info("unimplemented device %v, logicalport %v", childDevice, logicalPort)
}

func (f *OpenOltFlowMgr) decodeStoredID(id uint64) (uint64, string) {
	if id>>15 == 0x1 {
		return id & 0x7fff, UPSTREAM
	}
	return id, DOWNSTREAM
}

func (f *OpenOltFlowMgr) clearFlowFromResourceManager(flow *ofp.OfpFlowStats, flowID uint32, flowDirection string) {
	log.Debugw("clearFlowFromResourceManager", log.Fields{"flowID": flowID, "flowDirection": flowDirection, "flow": *flow})
	ponIntf, onuID, uniID, err := FlowExtractInfo(flow, flowDirection)
	if err != nil {
		log.Error(err)
		return
	}
	log.Debugw("Extracted access info from flow to be deleted",
		log.Fields{"ponIntf": ponIntf, "onuID": onuID, "uniID": uniID, "flowID": flowID})

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
		return
	}
	log.Debugw("Releasing flow Id to resource manager", log.Fields{"ponIntf": ponIntf, "onuID": onuID, "uniID": uniID, "flowID": flowID})
	f.resourceMgr.FreeFlowID(ponIntf, onuID, uniID, flowID)
	flowIds := f.resourceMgr.GetCurrentFlowIDsForOnu(ponIntf, onuID, uniID)
	if len(flowIds) == 0 {
		/* TODO: Remove Upstream and Downstream Schedulers */
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
func (f *OpenOltFlowMgr) AddFlow(flow *ofp.OfpFlowStats) {
	classifierInfo := make(map[string]interface{})
	actionInfo := make(map[string]interface{})
	log.Debug("Adding Flow", log.Fields{"flow": flow})
	for _, field := range utils.GetOfbFields(flow) {
		f.updateClassifierInfo(field, classifierInfo)
	}
	for _, action := range utils.GetActions(flow) {
		f.updateFlowActionInfo(action, actionInfo, classifierInfo)
	}
	/* Controller bound trap flows */
	if isControllerFlow := IsControllerBoundFlow(actionInfo[OUTPUT].(uint32)); isControllerFlow {
		log.Debug("Controller bound trap flows, getting inport from tunnelid")
		/* Get UNI port/ IN Port from tunnel ID field for upstream controller bound flows  */
		if portType := IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := utils.GetChildPortFromTunnelId(flow); uniPort != 0 {
				classifierInfo[InPort] = uniPort
				log.Debugw("upstream pon-to-controller-flow,inport-in-tunnelid", log.Fields{"newInPort": classifierInfo[InPort].(uint32), "outPort": actionInfo[OUTPUT].(uint32)})
			} else {
				log.Error("upstream pon-to-controller-flow, NO-inport-in-tunnelid")
				return
			}
		}
	} else {
		log.Debug("Non-Controller flows, getting uniport from tunnelid")
		// Downstream flow from NNI to PON port , Use tunnel ID as new OUT port / UNI port
		if portType := IntfIDToPortTypeName(actionInfo[OUTPUT].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := utils.GetChildPortFromTunnelId(flow); uniPort != 0 {
				actionInfo[OUTPUT] = uniPort
				log.Debugw("downstream-nni-to-pon-port-flow, outport-in-tunnelid", log.Fields{"newOutPort": actionInfo[OUTPUT].(uint32), "outPort": actionInfo[OUTPUT].(uint32)})
			} else {
				log.Debug("downstream-nni-to-pon-port-flow, no-outport-in-tunnelid", log.Fields{"InPort": classifierInfo[InPort].(uint32), "outPort": actionInfo[OUTPUT].(uint32)})
				return
			}
			// Upstream flow from PON to NNI port , Use tunnel ID as new IN port / UNI port
		} else if portType := IntfIDToPortTypeName(classifierInfo[InPort].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := utils.GetChildPortFromTunnelId(flow); uniPort != 0 {
				classifierInfo[InPort] = uniPort
				log.Debugw("upstream-pon-to-nni-port-flow, inport-in-tunnelid", log.Fields{"newInPort": actionInfo[OUTPUT].(uint32),
					"outport": actionInfo[OUTPUT].(uint32)})
			} else {
				log.Debug("upstream-pon-to-nni-port-flow, no-inport-in-tunnelid", log.Fields{"InPort": classifierInfo[InPort].(uint32),
					"outPort": actionInfo[OUTPUT].(uint32)})
				return
			}
		}
	}
	log.Infow("Flow ports", log.Fields{"classifierInfo_inport": classifierInfo[InPort], "action_output": actionInfo[OUTPUT]})
	portNo, intfID, onuID, uniID := ExtractAccessFromFlow(classifierInfo[InPort].(uint32), actionInfo[OUTPUT].(uint32))
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
	f.divideAndAddFlow(intfID, onuID, uniID, portNo, classifierInfo, actionInfo, flow)
}

func (f *OpenOltFlowMgr) updateClassifierInfo(field *ofp.OfpOxmOfbField, classifierInfo map[string]interface{}) {
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
		classifierInfo[METADATA] = field.GetTableMetadata()
		log.Debug("field-type-metadata", log.Fields{"classifierInfo[METADATA]": classifierInfo[METADATA].(uint64)})
	} else if field.Type == utils.TUNNEL_ID {
		classifierInfo[TunnelID] = field.GetTunnelId()
		log.Debug("field-type-tunnelId", log.Fields{"classifierInfo[TUNNEL_ID]": classifierInfo[TunnelID].(uint64)})
	} else {
		log.Errorw("Un supported field type", log.Fields{"type": field.Type})
		return
	}
}

func (f *OpenOltFlowMgr) updateFlowActionInfo(action *ofp.OfpAction, actionInfo map[string]interface{}, classifierInfo map[string]interface{}) {
	if action.Type == utils.OUTPUT {
		if out := action.GetOutput(); out != nil {
			actionInfo[OUTPUT] = out.GetPort()
			log.Debugw("action-type-output", log.Fields{"out_port": actionInfo[OUTPUT].(uint32)})
		} else {
			log.Error("Invalid output port in action")
			return
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
					return
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
		return
	}
}

//sendTPDownloadMsgToChild send payload
func (f *OpenOltFlowMgr) sendTPDownloadMsgToChild(intfID uint32, onuID uint32, uniID uint32, uni string) error {

	onuDevice, err := f.getOnuChildDevice(intfID, onuID)
	if err != nil {
		log.Errorw("Error while fetching Child device from core", log.Fields{"onuId": onuID})
		return err
	}
	log.Debugw("Got child device from OLT device handler", log.Fields{"device": *onuDevice})

	tpPath := f.getTPpath(intfID, uni)
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
	flowID, err := f.resourceMgr.GetFlowID(uint32(networkInterfaceID), uint32(onuID), uint32(uniID), flowStoreCookie, "")
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
		FlowType:      DOWNSTREAM,
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
