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

package adaptercore

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/opencord/voltha-go/common/log"
	tp "github.com/opencord/voltha-go/common/techprofile"
	fd "github.com/opencord/voltha-go/rw_core/flow_decomposition"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	ic "github.com/opencord/voltha-protos/go/inter_container"
	ofp "github.com/opencord/voltha-protos/go/openflow_13"
	openolt_pb2 "github.com/opencord/voltha-protos/go/openolt"
	voltha "github.com/opencord/voltha-protos/go/voltha"
	"math/big"
)

const (
	// Flow categories
	HSIA_FLOW  = "HSIA_FLOW"
	EAPOL_FLOW = "EAPOL_FLOW"

	IP_PROTO_DHCP = 17

	IP_PROTO_IGMP = 2

	EAP_ETH_TYPE  = 0x888e
	LLDP_ETH_TYPE = 0x88cc

	IGMP_PROTO = 2

	//FIXME - see also BRDCM_DEFAULT_VLAN in broadcom_onu.py
	DEFAULT_MGMT_VLAN = 4091

	DEFAULT_NETWORK_INTERFACE_ID = 0

	// Openolt Flow
	UPSTREAM        = "upstream"
	DOWNSTREAM      = "downstream"
	PACKET_TAG_TYPE = "pkt_tag_type"
	UNTAGGED        = "untagged"
	SINGLE_TAG      = "single_tag"
	DOUBLE_TAG      = "double_tag"

	// classifierInfo
	ETH_TYPE  = "eth_type"
	TPID      = "tpid"
	IP_PROTO  = "ip_proto"
	IN_PORT   = "in_port"
	VLAN_VID  = "vlan_vid"
	VLAN_PCP  = "vlan_pcp"
	UDP_DST   = "udp_dst"
	UDP_SRC   = "udp_src"
	IPV4_DST  = "ipv4_dst"
	IPV4_SRC  = "ipv4_src"
	METADATA  = "metadata"
	TUNNEL_ID = "tunnel_id"
	OUTPUT    = "output"
	// Action
	POP_VLAN     = "pop_vlan"
	PUSH_VLAN    = "push_vlan"
	TRAP_TO_HOST = "trap_to_host"
)

type onuInfo struct {
	intfId       uint32
	onuId        uint32
	serialNumber string
}

type onuIdKey struct {
	intfId uint32
	onuId  uint32
}

type gemPortKey struct {
	intfId  uint32
	gemPort uint32
}

type packetInInfoKey struct {
	intfId      uint32
	onuId       uint32
	logicalPort uint32
}

type OpenOltFlowMgr struct {
	techprofile      []*tp.TechProfileMgr
	deviceHandler    *DeviceHandler
	resourceMgr      *rsrcMgr.OpenOltResourceMgr
	onuIds           map[onuIdKey]onuInfo       //OnuId -> OnuInfo
	onuSerialNumbers map[string]onuInfo         //onu serial_number (string) -> OnuInfo
	onuGemPortIds    map[gemPortKey]onuInfo     //GemPortId -> OnuInfo
	packetInGemPort  map[packetInInfoKey]uint32 //packet in gem port
}

func NewFlowManager(dh *DeviceHandler, rsrcMgr *rsrcMgr.OpenOltResourceMgr) *OpenOltFlowMgr {
	log.Info("Initializing flow manager")
	var flowMgr OpenOltFlowMgr
	flowMgr.deviceHandler = dh
	flowMgr.resourceMgr = rsrcMgr
	if err := flowMgr.populateTechProfilePerPonPort(); err != nil {
		log.Error("Error while populating tech profile mgr\n")
		return nil
	}
	flowMgr.onuIds = make(map[onuIdKey]onuInfo)
	flowMgr.onuSerialNumbers = make(map[string]onuInfo)
	flowMgr.onuGemPortIds = make(map[gemPortKey]onuInfo)
	flowMgr.packetInGemPort = make(map[packetInInfoKey]uint32)
	log.Info("Initialization of  flow manager success!!")
	return &flowMgr
}

func (f *OpenOltFlowMgr) divideAndAddFlow(intfId uint32, onuId uint32, uniId uint32, portNo uint32, classifierInfo map[string]interface{}, actionInfo map[string]interface{}, flow *ofp.OfpFlowStats) {
	var allocId []uint32
	var gemPorts []uint32

	log.Infow("Dividing flow", log.Fields{"intfId": intfId, "onuId": onuId, "uniId": uniId, "portNo": portNo, "classifier": classifierInfo, "action": actionInfo})

	log.Infow("sorting flow", log.Fields{"intfId": intfId, "onuId": onuId, "uniId": uniId, "portNo": portNo,
		"classifierInfo": classifierInfo, "actionInfo": actionInfo})

	uni := getUniPortPath(intfId, onuId, uniId)
	log.Debugw("Uni port name", log.Fields{"uni": uni})
	allocId, gemPorts = f.createTcontGemports(intfId, onuId, uniId, uni, portNo, flow.GetTableId())
	if allocId == nil || gemPorts == nil {
		log.Error("alloc-id-gem-ports-unavailable")
		return
	}

	/* Flows can't be added specific to gemport unless p-bits are received.
	 * Hence adding flows for all gemports
	 */
	for _, gemPort := range gemPorts {
		if ipProto, ok := classifierInfo[IP_PROTO]; ok {
			if ipProto.(uint32) == IP_PROTO_DHCP {
				log.Info("Adding DHCP flow")
				f.addDHCPTrapFlow(intfId, onuId, uniId, portNo, classifierInfo, actionInfo, flow, allocId[0], gemPort)
			} else if ipProto == IP_PROTO_IGMP {
				log.Info("igmp flow add ignored, not implemented yet")
			} else {
				log.Errorw("Invalid-Classifier-to-handle", log.Fields{"classifier": classifierInfo, "action": actionInfo})
				//return errors.New("Invalid-Classifier-to-handle")
			}
		} else if ethType, ok := classifierInfo[ETH_TYPE]; ok {
			if ethType.(uint32) == EAP_ETH_TYPE {
				log.Info("Adding EAPOL flow")
				f.addEAPOLFlow(intfId, onuId, uniId, portNo, flow, allocId[0], gemPort, DEFAULT_MGMT_VLAN)
				if vlan := getSubscriberVlan(fd.GetInPort(flow)); vlan != 0 {
					f.addEAPOLFlow(intfId, onuId, uniId, portNo, flow, allocId[0], gemPort, vlan)
				}
				// Send Techprofile download event to child device in go routine as it takes time
				go f.sendTPDownloadMsgToChild(intfId, onuId, uniId, uni)
			}
			if ethType == LLDP_ETH_TYPE {
				log.Info("Adding LLDP flow")
				addLLDPFlow(flow, portNo)
			}
		} else if _, ok := actionInfo[PUSH_VLAN]; ok {
			log.Info("Adding upstream data rule")
			f.addUpstreamDataFlow(intfId, onuId, uniId, portNo, classifierInfo, actionInfo, flow, allocId[0], gemPort)
		} else if _, ok := actionInfo[POP_VLAN]; ok {
			log.Info("Adding Downstream data rule")
			f.addDownstreamDataFlow(intfId, onuId, uniId, portNo, classifierInfo, actionInfo, flow, allocId[0], gemPort)
		} else {
			log.Errorw("Invalid-flow-type-to-handle", log.Fields{"classifier": classifierInfo, "action": actionInfo, "flow": flow})
		}
	}
}

// This function allocates tconts and GEM ports for an ONU, currently one TCONT is supported per ONU
func (f *OpenOltFlowMgr) createTcontGemports(intfId uint32, onuId uint32, uniId uint32, uni string, uniPort uint32, tableID uint32) ([]uint32, []uint32) {
	var allocID []uint32
	var gemPortIDs []uint32
	//If we already have allocated earlier for this onu, render them
	if tcontId := f.resourceMgr.GetCurrentAllocIDForOnu(intfId, onuId, uniId); tcontId != 0 {
		allocID = append(allocID, tcontId)
	}
	gemPortIDs = f.resourceMgr.GetCurrentGEMPortIDsForOnu(intfId, onuId, uniId)
	if len(allocID) != 0 && len(gemPortIDs) != 0 {
		log.Debug("Rendered Tcont and GEM ports from resource manager", log.Fields{"intfId": intfId, "onuId": onuId, "uniPort": uniId,
			"allocID": allocID, "gemPortIDs": gemPortIDs})
		return allocID, gemPortIDs
	}
	log.Debug("Creating New TConts and Gem ports", log.Fields{"pon": intfId, "onu": onuId, "uni": uniId})

	//FIXME: If table id is <= 63 using 64 as table id
	if tableID < tp.DEFAULT_TECH_PROFILE_TABLE_ID {
		tableID = tp.DEFAULT_TECH_PROFILE_TABLE_ID
	}
	tpPath := f.getTPpath(intfId, uni)
	// Check tech profile instance already exists for derived port name
	tech_profile_instance, err := f.techprofile[intfId].GetTPInstanceFromKVStore(tableID, tpPath)
	if err != nil { // This should not happen, something wrong in KV backend transaction
		log.Errorw("Error in fetching tech profile instance from KV store", log.Fields{"tableID": tableID, "path": tpPath})
		return nil, nil
	}
	if tech_profile_instance == nil {
		log.Info("Creating tech profile instance", log.Fields{"path": tpPath})
		tech_profile_instance = f.techprofile[intfId].CreateTechProfInstance(tableID, uni, intfId)
		if tech_profile_instance == nil {
			log.Error("Tech-profile-instance-creation-failed")
			return nil, nil
		}
	} else {
		log.Debugw("Tech-profile-instance-already-exist-for-given port-name", log.Fields{"uni": uni})
	}
	// Get upstream and downstream scheduler protos
	us_scheduler := f.techprofile[intfId].GetUsScheduler(tech_profile_instance)
	ds_scheduler := f.techprofile[intfId].GetDsScheduler(tech_profile_instance)
	// Get TCONTS protos
	tconts := f.techprofile[intfId].GetTconts(tech_profile_instance, us_scheduler, ds_scheduler)
	if len(tconts) == 0 {
		log.Error("TCONTS not found ")
		return nil, nil
	}
	log.Debugw("Sending Create tcont to device",
		log.Fields{"onu": onuId, "uni": uniId, "portNo": "", "tconts": tconts})
	if _, err := f.deviceHandler.Client.CreateTconts(context.Background(),
		&openolt_pb2.Tconts{IntfId: intfId,
			OnuId:  onuId,
			UniId:  uniId,
			PortNo: uniPort,
			Tconts: tconts}); err != nil {
		log.Errorw("Error while creating TCONT in device", log.Fields{"error": err})
		return nil, nil
	}
	allocID = append(allocID, tech_profile_instance.UsScheduler.AllocID)
	for _, gem := range tech_profile_instance.UpstreamGemPortAttributeList {
		gemPortIDs = append(gemPortIDs, gem.GemportID)
	}
	log.Debugw("Allocated Tcont and GEM ports", log.Fields{"allocID": allocID, "gemports": gemPortIDs})
	// Send Tconts and GEM ports to KV store
	f.storeTcontsGEMPortsIntoKVStore(intfId, onuId, uniId, allocID, gemPortIDs)
	return allocID, gemPortIDs
}

func (f *OpenOltFlowMgr) storeTcontsGEMPortsIntoKVStore(intfId uint32, onuId uint32, uniId uint32, allocID []uint32, gemPortIDs []uint32) {

	log.Debugw("Storing allocated Tconts and GEM ports into KV store",
		log.Fields{"intfId": intfId, "onuId": onuId, "uniId": uniId, "allocID": allocID, "gemPortIDs": gemPortIDs})
	/* Update the allocated alloc_id and gem_port_id for the ONU/UNI to KV store  */
	if err := f.resourceMgr.UpdateAllocIdsForOnu(intfId, onuId, uniId, allocID); err != nil {
		log.Error("Errow while uploading allocID to KV store")
	}
	if err := f.resourceMgr.UpdateGEMPortIDsForOnu(intfId, onuId, uniId, gemPortIDs); err != nil {
		log.Error("Errow while uploading GEMports to KV store")
	}
	if err := f.resourceMgr.UpdateGEMportsPonportToOnuMapOnKVStore(gemPortIDs, intfId, onuId, uniId); err != nil {
		log.Error("Errow while uploading gemtopon map to KV store")
	}
	log.Debug("Stored tconts and GEM into KV store successfully")
	for _, gemPort := range gemPortIDs {
		f.addGemPortToOnuInfoMap(intfId, onuId, gemPort)
	}
}

func (f *OpenOltFlowMgr) populateTechProfilePerPonPort() error {
	for _, techRange := range f.resourceMgr.DevInfo.Ranges {
		for intfId := range techRange.IntfIds {
			f.techprofile = append(f.techprofile, f.resourceMgr.ResourceMgrs[uint32(intfId)].TechProfileMgr)
		}
	}
	//Make sure we have as many tech_profiles as there are pon ports on the device
	if len(f.techprofile) != int(f.resourceMgr.DevInfo.GetPonPorts()) {
		log.Errorw("Error while populating techprofile",
			log.Fields{"numofTech": len(f.techprofile), "numPonPorts": f.resourceMgr.DevInfo.GetPonPorts()})
		return errors.New("Error while populating techprofile mgrs")
	}
	log.Infow("Populated techprofile per ponport successfully",
		log.Fields{"numofTech": len(f.techprofile), "numPonPorts": f.resourceMgr.DevInfo.GetPonPorts()})
	return nil
}

func (f *OpenOltFlowMgr) addUpstreamDataFlow(intfId uint32, onuId uint32, uniId uint32,
	portNo uint32, uplinkClassifier map[string]interface{},
	uplinkAction map[string]interface{}, logicalFlow *ofp.OfpFlowStats,
	allocId uint32, gemportId uint32) {
	uplinkClassifier[PACKET_TAG_TYPE] = SINGLE_TAG
	log.Debugw("Adding upstream data flow", log.Fields{"uplinkClassifier": uplinkClassifier, "uplinkAction": uplinkAction})
	f.addHSIAFlow(intfId, onuId, uniId, portNo, uplinkClassifier, uplinkAction,
		UPSTREAM, logicalFlow, allocId, gemportId)
	/* TODO: Install Secondary EAP on the subscriber vlan */
}

func (f *OpenOltFlowMgr) addDownstreamDataFlow(intfId uint32, onuId uint32, uniId uint32,
	portNo uint32, downlinkClassifier map[string]interface{},
	downlinkAction map[string]interface{}, logicalFlow *ofp.OfpFlowStats,
	allocId uint32, gemportId uint32) {
	downlinkClassifier[PACKET_TAG_TYPE] = DOUBLE_TAG
	log.Debugw("Adding downstream data flow", log.Fields{"downlinkClassifier": downlinkClassifier,
		"downlinkAction": downlinkAction})
	// Ignore private VLAN flow given by decomposer, cannot do anything with this flow
	if uint32(downlinkClassifier[METADATA].(uint64)) == MkUniPortNum(intfId, onuId, uniId) &&
		downlinkClassifier[VLAN_VID] == (uint32(ofp.OfpVlanId_OFPVID_PRESENT)|4000) {
		log.Infow("EAPOL DL flow , Already added ,ignoring it", log.Fields{"downlinkClassifier": downlinkClassifier,
			"downlinkAction": downlinkAction})
		return
	}
	/* Already this info available classifier? */
	downlinkAction[POP_VLAN] = true
	downlinkAction[VLAN_VID] = downlinkClassifier[VLAN_VID]
	f.addHSIAFlow(intfId, onuId, uniId, portNo, downlinkClassifier, downlinkAction,
		DOWNSTREAM, logicalFlow, allocId, gemportId)
}

func (f *OpenOltFlowMgr) addHSIAFlow(intfId uint32, onuId uint32, uniId uint32, portNo uint32, classifier map[string]interface{},
	action map[string]interface{}, direction string, logicalFlow *ofp.OfpFlowStats,
	allocId uint32, gemPortId uint32) {
	/* One of the OLT platform (Broadcom BAL) requires that symmetric
	   flows require the same flow_id to be used across UL and DL.
	   Since HSIA flow is the only symmetric flow currently, we need to
	   re-use the flow_id across both direction. The 'flow_category'
	   takes priority over flow_cookie to find any available HSIA_FLOW
	   id for the ONU.
	*/
	log.Debugw("Adding HSIA flow", log.Fields{"intfId": intfId, "onuId": onuId, "uniId": uniId, "classifier": classifier,
		"action": action, "direction": direction, "allocId": allocId, "gemPortId": gemPortId,
		"logicalFlow": *logicalFlow})
	flowStoreCookie := getFlowStoreCookie(classifier, gemPortId)
	flowId, err := f.resourceMgr.GetFlowID(intfId, onuId, uniId, flowStoreCookie, "HSIA")
	if err != nil {
		log.Errorw("Flow id unavailable for HSIA flow", log.Fields{"direction": direction})
		return
	}
	var classifierProto *openolt_pb2.Classifier
	var actionProto *openolt_pb2.Action
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
	flow := openolt_pb2.Flow{AccessIntfId: int32(intfId),
		OnuId:         int32(onuId),
		UniId:         int32(uniId),
		FlowId:        flowId,
		FlowType:      direction,
		AllocId:       int32(allocId),
		NetworkIntfId: DEFAULT_NETWORK_INTERFACE_ID, // one NNI port is supported now
		GemportId:     int32(gemPortId),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo}
	if ok := f.addFlowToDevice(&flow); ok {
		log.Debug("HSIA flow added to device successfully", log.Fields{"direction": direction})
		flowsToKVStore := f.getUpdatedFlowInfo(&flow, flowStoreCookie, "HSIA")
		if err := f.updateFlowInfoToKVStore(flow.AccessIntfId,
			flow.OnuId,
			flow.UniId,
			flow.FlowId, flowsToKVStore); err != nil {
			log.Errorw("Error uploading HSIA  flow into KV store", log.Fields{"flow": flow, "direction": direction, "error": err})
			return
		}
	}
}
func (f *OpenOltFlowMgr) addDHCPTrapFlow(intfId uint32, onuId uint32, uniId uint32, portNo uint32, classifier map[string]interface{}, action map[string]interface{}, logicalFlow *ofp.OfpFlowStats, allocId uint32, gemPortId uint32) {

	var dhcpFlow openolt_pb2.Flow
	var actionProto *openolt_pb2.Action
	var classifierProto *openolt_pb2.Classifier

	// Clear the action map
	for k := range action {
		delete(action, k)
	}

	action[TRAP_TO_HOST] = true
	classifier[UDP_SRC] = uint32(68)
	classifier[UDP_DST] = uint32(67)
	classifier[PACKET_TAG_TYPE] = SINGLE_TAG
	delete(classifier, VLAN_VID)

	flowStoreCookie := getFlowStoreCookie(classifier, gemPortId)

	flowID, err := f.resourceMgr.GetFlowID(intfId, onuId, uniId, flowStoreCookie, "")

	if err != nil {
		log.Errorw("flowId unavailable for UL EAPOL", log.Fields{"intfId": intfId, "onuId": onuId, "flowStoreCookie": flowStoreCookie})
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

	dhcpFlow = openolt_pb2.Flow{AccessIntfId: int32(intfId),
		OnuId:         int32(onuId),
		UniId:         int32(uniId),
		FlowId:        flowID,
		FlowType:      UPSTREAM,
		AllocId:       int32(allocId),
		NetworkIntfId: DEFAULT_NETWORK_INTERFACE_ID, // one NNI port is supported now
		GemportId:     int32(gemPortId),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo}

	if ok := f.addFlowToDevice(&dhcpFlow); ok {
		log.Debug("DHCP UL flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(&dhcpFlow, flowStoreCookie, "DHCP")
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

// Add EAPOL to  device
func (f *OpenOltFlowMgr) addEAPOLFlow(intfId uint32, onuId uint32, uniId uint32, portNo uint32, logicalFlow *ofp.OfpFlowStats, allocId uint32, gemPortId uint32, vlanId uint32) {
	log.Debugw("Adding EAPOL to device", log.Fields{"intfId": intfId, "onuId": onuId, "portNo": portNo, "allocId": allocId, "gemPortId": gemPortId, "vlanId": vlanId, "flow": logicalFlow})

	uplinkClassifier := make(map[string]interface{})
	uplinkAction := make(map[string]interface{})
	downlinkClassifier := make(map[string]interface{})
	downlinkAction := make(map[string]interface{})
	var upstreamFlow openolt_pb2.Flow
	var downstreamFlow openolt_pb2.Flow

	// Fill Classfier
	uplinkClassifier[ETH_TYPE] = uint32(EAP_ETH_TYPE)
	uplinkClassifier[PACKET_TAG_TYPE] = SINGLE_TAG
	uplinkClassifier[VLAN_VID] = vlanId
	// Fill action
	uplinkAction[TRAP_TO_HOST] = true
	flowStoreCookie := getFlowStoreCookie(uplinkClassifier, gemPortId)
	//Add Uplink EAPOL Flow
	uplinkFlowId, err := f.resourceMgr.GetFlowID(intfId, onuId, uniId, flowStoreCookie, "")
	if err != nil {
		log.Errorw("flowId unavailable for UL EAPOL", log.Fields{"intfId": intfId, "onuId": onuId, "flowStoreCookie": flowStoreCookie})
		return
	}
	var classifierProto *openolt_pb2.Classifier
	var actionProto *openolt_pb2.Action
	log.Debugw("Creating UL EAPOL flow", log.Fields{"ul_classifier": uplinkClassifier, "ul_action": uplinkAction, "uplinkFlowId": uplinkFlowId})

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
	upstreamFlow = openolt_pb2.Flow{AccessIntfId: int32(intfId),
		OnuId:         int32(onuId),
		UniId:         int32(uniId),
		FlowId:        uplinkFlowId,
		FlowType:      UPSTREAM,
		AllocId:       int32(allocId),
		NetworkIntfId: DEFAULT_NETWORK_INTERFACE_ID, // one NNI port is supported now
		GemportId:     int32(gemPortId),
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo}
	if ok := f.addFlowToDevice(&upstreamFlow); ok {
		log.Debug("EAPOL UL flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(&upstreamFlow, flowStoreCookie, "EAPOL")
		if err := f.updateFlowInfoToKVStore(upstreamFlow.AccessIntfId,
			upstreamFlow.OnuId,
			upstreamFlow.UniId,
			upstreamFlow.FlowId, flowsToKVStore); err != nil {
			log.Errorw("Error uploading EAPOL UL flow into KV store", log.Fields{"flow": upstreamFlow, "error": err})
			return
		}
	}

	if vlanId == DEFAULT_MGMT_VLAN {
		/* Add Downstream EAPOL Flow, Only for first EAP flow (BAL
		# requirement)
		# On one of the platforms (Broadcom BAL), when same DL classifier
		# vlan was used across multiple ONUs, eapol flow re-adds after
		# flow delete (cases of onu reboot/disable) fails.
		# In order to generate unique vlan, a combination of intf_id
		# onu_id and uniId is used.
		# uniId defaults to 0, so add 1 to it.
		*/
		log.Debugw("Creating DL EAPOL flow with default vlan", log.Fields{"vlan": vlanId})
		specialVlanDlFlow := 4090 - intfId*onuId*(uniId+1)
		// Assert that we do not generate invalid vlans under no condition
		if specialVlanDlFlow <= 2 {
			log.Fatalw("invalid-vlan-generated", log.Fields{"vlan": specialVlanDlFlow})
			return
		}
		log.Debugw("specialVlanEAPOLDlFlow:", log.Fields{"dl_vlan": specialVlanDlFlow})
		// Fill Classfier
		downlinkClassifier[PACKET_TAG_TYPE] = SINGLE_TAG
		downlinkClassifier[VLAN_VID] = uint32(specialVlanDlFlow)
		// Fill action
		downlinkAction[PUSH_VLAN] = true
		downlinkAction[VLAN_VID] = vlanId
		flowStoreCookie := getFlowStoreCookie(downlinkClassifier, gemPortId)
		downlinkFlowId, err := f.resourceMgr.GetFlowID(intfId, onuId, uniId, flowStoreCookie, "")
		if err != nil {
			log.Errorw("flowId unavailable for DL EAPOL",
				log.Fields{"intfId": intfId, "onuId": onuId, "flowStoreCookie": flowStoreCookie})
			return
		}
		log.Debugw("Creating DL EAPOL flow",
			log.Fields{"dl_classifier": downlinkClassifier, "dl_action": downlinkAction, "downlinkFlowId": downlinkFlowId})
		if classifierProto = makeOpenOltClassifierField(downlinkClassifier); classifierProto == nil {
			log.Error("Error in making classifier protobuf for downlink flow")
			return
		}
		if actionProto = makeOpenOltActionField(downlinkAction); actionProto == nil {
			log.Error("Error in making action protobuf for dl flow")
			return
		}
		// Downstream flow in grpc protobuf
		downstreamFlow = openolt_pb2.Flow{AccessIntfId: int32(intfId),
			OnuId:         int32(onuId),
			UniId:         int32(uniId),
			FlowId:        downlinkFlowId,
			FlowType:      DOWNSTREAM,
			AllocId:       int32(allocId),
			NetworkIntfId: DEFAULT_NETWORK_INTERFACE_ID,
			GemportId:     int32(gemPortId),
			Classifier:    classifierProto,
			Action:        actionProto,
			Priority:      int32(logicalFlow.Priority),
			Cookie:        logicalFlow.Cookie,
			PortNo:        portNo}
		if ok := f.addFlowToDevice(&downstreamFlow); ok {
			log.Debug("EAPOL DL flow added to device successfully")
			flowsToKVStore := f.getUpdatedFlowInfo(&downstreamFlow, flowStoreCookie, "")
			if err := f.updateFlowInfoToKVStore(downstreamFlow.AccessIntfId,
				downstreamFlow.OnuId,
				downstreamFlow.UniId,
				downstreamFlow.FlowId, flowsToKVStore); err != nil {
				log.Errorw("Error uploading EAPOL DL flow into KV store", log.Fields{"flow": upstreamFlow, "error": err})
				return
			}
		}
	} else {
		log.Infow("EAPOL flow with non-default mgmt vlan is not supported", log.Fields{"vlanId": vlanId})
		return
	}
	log.Debugw("Added EAPOL flows to device successfully", log.Fields{"flow": logicalFlow})
}

func makeOpenOltClassifierField(classifierInfo map[string]interface{}) *openolt_pb2.Classifier {
	var classifier openolt_pb2.Classifier
	if etherType, ok := classifierInfo[ETH_TYPE]; ok {
		classifier.EthType = etherType.(uint32)
	}
	if ipProto, ok := classifierInfo[IP_PROTO]; ok {
		classifier.IpProto = ipProto.(uint32)
	}
	if vlanId, ok := classifierInfo[VLAN_VID]; ok {
		classifier.OVid = (vlanId.(uint32)) & 0xFFF
	}
	if metadata, ok := classifierInfo[METADATA]; ok { // TODO: Revisit
		classifier.IVid = uint32(metadata.(uint64))
	}
	if vlanPcp, ok := classifierInfo[VLAN_PCP]; ok {
		classifier.OPbits = vlanPcp.(uint32)
	}
	if udpSrc, ok := classifierInfo[UDP_SRC]; ok {
		classifier.SrcPort = udpSrc.(uint32)
	}
	if udpDst, ok := classifierInfo[UDP_DST]; ok {
		classifier.DstPort = udpDst.(uint32)
	}
	if ipv4Dst, ok := classifierInfo[IPV4_DST]; ok {
		classifier.DstIp = ipv4Dst.(uint32)
	}
	if ipv4Src, ok := classifierInfo[IPV4_SRC]; ok {
		classifier.SrcIp = ipv4Src.(uint32)
	}
	if pktTagType, ok := classifierInfo[PACKET_TAG_TYPE]; ok {
		if pktTagType.(string) == SINGLE_TAG {
			classifier.PktTagType = SINGLE_TAG
		} else if pktTagType.(string) == DOUBLE_TAG {
			classifier.PktTagType = DOUBLE_TAG
		} else if pktTagType.(string) == UNTAGGED {
			classifier.PktTagType = UNTAGGED
		} else {
			log.Error("Invalid tag type in classifier") // should not hit
			return nil
		}
	}
	return &classifier
}

func makeOpenOltActionField(actionInfo map[string]interface{}) *openolt_pb2.Action {
	var actionCmd openolt_pb2.ActionCmd
	var action openolt_pb2.Action
	action.Cmd = &actionCmd
	if _, ok := actionInfo[POP_VLAN]; ok {
		action.OVid = actionInfo[VLAN_VID].(uint32)
		action.Cmd.RemoveOuterTag = true
	} else if _, ok := actionInfo[PUSH_VLAN]; ok {
		action.OVid = actionInfo[VLAN_VID].(uint32)
		action.Cmd.AddOuterTag = true
	} else if _, ok := actionInfo[TRAP_TO_HOST]; ok {
		action.Cmd.TrapToHost = actionInfo[TRAP_TO_HOST].(bool)
	} else {
		log.Errorw("Invalid-action-field", log.Fields{"action": actionInfo})
		return nil
	}
	return &action
}

func (f *OpenOltFlowMgr) getTPpath(intfId uint32, uni string) string {
	/*
	   FIXME
	   Should get Table id form the flow, as of now hardcoded to DEFAULT_TECH_PROFILE_TABLE_ID (64)
	   'tp_path' contains the suffix part of the tech_profile_instance path. The prefix to the 'tp_path' should be set to
	   TechProfile.KV_STORE_TECH_PROFILE_PATH_PREFIX by the ONU adapter.
	*/
	return f.techprofile[intfId].GetTechProfileInstanceKVPath(tp.DEFAULT_TECH_PROFILE_TABLE_ID, uni)
}

func getFlowStoreCookie(classifier map[string]interface{}, gemPortId uint32) uint64 {
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
	if gemPortId != 0 {
		flowString = fmt.Sprintf("%s%s", string(jsonData), string(gemPortId))
	}
	h := md5.New()
	h.Write([]byte(flowString))
	hash := big.NewInt(0)
	hash.SetBytes(h.Sum(nil))
	return hash.Uint64()
}

func (f *OpenOltFlowMgr) getUpdatedFlowInfo(flow *openolt_pb2.Flow, flowStoreCookie uint64, flowCategory string) *[]rsrcMgr.FlowInfo {
	var flows []rsrcMgr.FlowInfo = []rsrcMgr.FlowInfo{rsrcMgr.FlowInfo{Flow: flow, FlowCategory: flowCategory, FlowStoreCookie: flowStoreCookie}}
	var intfId uint32
	/* For flows which trap out of the NNI, the AccessIntfId is invalid
	   (set to -1). In such cases, we need to refer to the NetworkIntfId .
	*/
	if flow.AccessIntfId != -1 {
		intfId = uint32(flow.AccessIntfId)
	} else {
		intfId = uint32(flow.NetworkIntfId)
	}
	// Get existing flows matching flowid for given subscriber from KV store
	existingFlows := f.resourceMgr.GetFlowIDInfo(intfId, uint32(flow.OnuId), uint32(flow.UniId), flow.FlowId)
	if existingFlows != nil {
		log.Debugw("Flow exists for given flowID, appending it to current flow", log.Fields{"flowID": flow.FlowId})
		for _, f := range *existingFlows {
			flows = append(flows, f)
		}
	}
	log.Debugw("Updated flows for given flowID and onuid", log.Fields{"updatedflow": flows, "flowid": flow.FlowId, "onu": flow.OnuId})
	return &flows
}

func (f *OpenOltFlowMgr) updateFlowInfoToKVStore(intfId int32, onuId int32, uniId int32, flowId uint32, flows *[]rsrcMgr.FlowInfo) error {
	log.Debugw("Storing flow(s) into KV store", log.Fields{"flows": *flows})
	if err := f.resourceMgr.UpdateFlowIDInfo(intfId, onuId, uniId, flowId, flows); err != nil {
		log.Debug("Error while Storing flow into KV store")
		return err
	}
	log.Info("Stored flow(s) into KV store successfully!")
	return nil
}

func (f *OpenOltFlowMgr) addFlowToDevice(deviceFlow *openolt_pb2.Flow) bool {
	log.Debugw("Sending flow to device via grpc", log.Fields{"flow": *deviceFlow})
	_, err := f.deviceHandler.Client.FlowAdd(context.Background(), deviceFlow)
	if err != nil {
		log.Errorw("Failed to Add flow to device", log.Fields{"err": err, "deviceFlow": deviceFlow})
		return false
	}
	log.Debugw("Flow added to device successfuly ", log.Fields{"flow": *deviceFlow})
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
	log.Info("Unimplemented")
}
func getUniPortPath(intfId uint32, onuId uint32, uniId uint32) string {
	return fmt.Sprintf("pon-{%d}/onu-{%d}/uni-{%d}", intfId, onuId, uniId)
}

func (f *OpenOltFlowMgr) getOnuChildDevice(intfId uint32, onuId uint32) (*voltha.Device, error) {
	log.Debugw("GetChildDevice", log.Fields{"pon port": intfId, "onuId": onuId})
	//parentPortNo := IntfIdToPortNo(intfId, voltha.Port_PON_OLT)
	parentPortNo := intfId
	onuDevice := f.deviceHandler.GetChildDevice(parentPortNo, onuId)
	if onuDevice == nil {
		log.Errorw("onu not found", log.Fields{"intfId": parentPortNo, "onuId": onuId})
		return nil, errors.New("onu not found")
	}
	log.Debugw("Successfully received child device from core", log.Fields{"child_device": *onuDevice})
	return onuDevice, nil
}

func findNextFlow(flow *ofp.OfpFlowStats) *ofp.OfpFlowStats {
	log.Info("Unimplemented")
	return nil
}

func getSubscriberVlan(inPort uint32) uint32 {
	/* For EAPOL case we will use default VLAN , so will implement later if required */
	log.Info("Unimplemented")
	return 0
}

func (f *OpenOltFlowMgr) clear_flows_and_scheduler_for_logical_port(childDevice *voltha.Device, logicalPort *voltha.LogicalPort) {
	log.Info("Unimplemented")
}

func (f *OpenOltFlowMgr) AddFlow(flow *ofp.OfpFlowStats) {
	classifierInfo := make(map[string]interface{}, 0)
	actionInfo := make(map[string]interface{}, 0)
	log.Debug("Adding Flow", log.Fields{"flow": flow})
	for _, field := range fd.GetOfbFields(flow) {
		if field.Type == fd.ETH_TYPE {
			classifierInfo[ETH_TYPE] = field.GetEthType()
			log.Debug("field-type-eth-type", log.Fields{"classifierInfo[ETH_TYPE]": classifierInfo[ETH_TYPE].(uint32)})
		} else if field.Type == fd.IP_PROTO {
			classifierInfo[IP_PROTO] = field.GetIpProto()
			log.Debug("field-type-ip-proto", log.Fields{"classifierInfo[IP_PROTO]": classifierInfo[IP_PROTO].(uint32)})
		} else if field.Type == fd.IN_PORT {
			classifierInfo[IN_PORT] = field.GetPort()
			log.Debug("field-type-in-port", log.Fields{"classifierInfo[IN_PORT]": classifierInfo[IN_PORT].(uint32)})
		} else if field.Type == fd.VLAN_VID {
			classifierInfo[VLAN_VID] = field.GetVlanVid()
			log.Debug("field-type-vlan-vid", log.Fields{"classifierInfo[VLAN_VID]": classifierInfo[VLAN_VID].(uint32)})
		} else if field.Type == fd.VLAN_PCP {
			classifierInfo[VLAN_PCP] = field.GetVlanPcp()
			log.Debug("field-type-vlan-pcp", log.Fields{"classifierInfo[VLAN_PCP]": classifierInfo[VLAN_PCP].(uint32)})
		} else if field.Type == fd.UDP_DST {
			classifierInfo[UDP_DST] = field.GetUdpDst()
			log.Debug("field-type-udp-dst", log.Fields{"classifierInfo[UDP_DST]": classifierInfo[UDP_DST].(uint32)})
		} else if field.Type == fd.UDP_SRC {
			classifierInfo[UDP_SRC] = field.GetUdpSrc()
			log.Debug("field-type-udp-src", log.Fields{"classifierInfo[UDP_SRC]": classifierInfo[UDP_SRC].(uint32)})
		} else if field.Type == fd.IPV4_DST {
			classifierInfo[IPV4_DST] = field.GetIpv4Dst()
			log.Debug("field-type-ipv4-dst", log.Fields{"classifierInfo[IPV4_DST]": classifierInfo[IPV4_DST].(uint32)})
		} else if field.Type == fd.IPV4_SRC {
			classifierInfo[IPV4_SRC] = field.GetIpv4Src()
			log.Debug("field-type-ipv4-src", log.Fields{"classifierInfo[IPV4_SRC]": classifierInfo[IPV4_SRC].(uint32)})
		} else if field.Type == fd.METADATA {
			classifierInfo[METADATA] = field.GetTableMetadata()
			log.Debug("field-type-metadata", log.Fields{"classifierInfo[METADATA]": classifierInfo[METADATA].(uint64)})
		} else if field.Type == fd.TUNNEL_ID {
			classifierInfo[TUNNEL_ID] = field.GetTunnelId()
			log.Debug("field-type-tunnelId", log.Fields{"classifierInfo[TUNNEL_ID]": classifierInfo[TUNNEL_ID].(uint64)})
		} else {
			log.Errorw("Un supported field type", log.Fields{"type": field.Type})
			return
		}
	}
	for _, action := range fd.GetActions(flow) {
		if action.Type == fd.OUTPUT {
			if out := action.GetOutput(); out != nil {
				actionInfo[OUTPUT] = out.GetPort()
				log.Debugw("action-type-output", log.Fields{"out_port": actionInfo[OUTPUT].(uint32)})
			} else {
				log.Error("Invalid output port in action")
				return
			}
		} else if action.Type == fd.POP_VLAN {
			actionInfo[POP_VLAN] = true
			log.Debugw("action-type-pop-vlan", log.Fields{"in_port": classifierInfo[IN_PORT].(uint32)})
		} else if action.Type == fd.PUSH_VLAN {
			if out := action.GetPush(); out != nil {
				if tpid := out.GetEthertype(); tpid != 0x8100 {
					log.Errorw("Invalid ethertype in push action", log.Fields{"ethertype": actionInfo[PUSH_VLAN].(int32)})
				} else {
					actionInfo[PUSH_VLAN] = true
					actionInfo[TPID] = tpid
					log.Debugw("action-type-push-vlan",
						log.Fields{"push_tpid": actionInfo[TPID].(uint32), "in_port": classifierInfo[IN_PORT].(uint32)})
				}
			}
		} else if action.Type == fd.SET_FIELD {
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
								actionInfo[VLAN_VID] = vlan & 0xfff
								log.Debugw("action-set-vlan-vid", log.Fields{"actionInfo[VLAN_VID]": actionInfo[VLAN_VID].(uint32)})
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
	/* Controller bound trap flows */
	if isControllerFlow := IsControllerBoundFlow(actionInfo[OUTPUT].(uint32)); isControllerFlow {
		log.Debug("Controller bound trap flows, getting inport from tunnelid")
		/* Get UNI port/ IN Port from tunnel ID field for upstream controller bound flows  */
		if portType := IntfIdToPortTypeName(classifierInfo[IN_PORT].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := fd.GetChildPortFromTunnelId(flow); uniPort != 0 {
				classifierInfo[IN_PORT] = uniPort
				log.Debugw("upstream pon-to-controller-flow,inport-in-tunnelid", log.Fields{"newInPort": classifierInfo[IN_PORT].(uint32), "outPort": actionInfo[OUTPUT].(uint32)})
			} else {
				log.Error("upstream pon-to-controller-flow, NO-inport-in-tunnelid")
				return
			}
		}
	} else {
		log.Debug("Non-Controller flows, getting uniport from tunnelid")
		// Downstream flow from NNI to PON port , Use tunnel ID as new OUT port / UNI port
		if portType := IntfIdToPortTypeName(actionInfo[OUTPUT].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := fd.GetChildPortFromTunnelId(flow); uniPort != 0 {
				actionInfo[OUTPUT] = uniPort
				log.Debugw("downstream-nni-to-pon-port-flow, outport-in-tunnelid", log.Fields{"newOutPort": actionInfo[OUTPUT].(uint32), "outPort": actionInfo[OUTPUT].(uint32)})
			} else {
				log.Debug("downstream-nni-to-pon-port-flow, no-outport-in-tunnelid", log.Fields{"InPort": classifierInfo[IN_PORT].(uint32), "outPort": actionInfo[OUTPUT].(uint32)})
				return
			}
			// Upstream flow from PON to NNI port , Use tunnel ID as new IN port / UNI port
		} else if portType := IntfIdToPortTypeName(classifierInfo[IN_PORT].(uint32)); portType == voltha.Port_PON_OLT {
			if uniPort := fd.GetChildPortFromTunnelId(flow); uniPort != 0 {
				classifierInfo[IN_PORT] = uniPort
				log.Debugw("upstream-pon-to-nni-port-flow, inport-in-tunnelid", log.Fields{"newInPort": actionInfo[OUTPUT].(uint32),
					"outport": actionInfo[OUTPUT].(uint32)})
			} else {
				log.Debug("upstream-pon-to-nni-port-flow, no-inport-in-tunnelid", log.Fields{"InPort": classifierInfo[IN_PORT].(uint32),
					"outPort": actionInfo[OUTPUT].(uint32)})
				return
			}
		}
	}
	log.Infow("Flow ports", log.Fields{"classifierInfo_inport": classifierInfo[IN_PORT], "action_output": actionInfo[OUTPUT]})
	portNo, intfId, onuId, uniId := ExtractAccessFromFlow(classifierInfo[IN_PORT].(uint32), actionInfo[OUTPUT].(uint32))
	if ipProto, ok := classifierInfo[IP_PROTO]; ok {
		if ipProto.(uint32) == IP_PROTO_DHCP {
			if udpSrc, ok := classifierInfo[UDP_SRC]; ok {
				if udpSrc.(uint32) == uint32(67) {
					log.Debug("trap-dhcp-from-nni-flow")
					f.addDHCPTrapFlowOnNNI(flow, classifierInfo, portNo)
					return
				}
			}
		}
	}
	f.divideAndAddFlow(intfId, onuId, uniId, portNo, classifierInfo, actionInfo, flow)
}

func (f *OpenOltFlowMgr) sendTPDownloadMsgToChild(intfId uint32, onuId uint32, uniId uint32, uni string) error {

	onuDevice, err := f.getOnuChildDevice(intfId, onuId)
	if err != nil {
		log.Errorw("Error while fetching Child device from core", log.Fields{"onuId": onuId})
		return err
	}
	log.Debugw("Got child device from OLT device handler", log.Fields{"device": *onuDevice})

	tpPath := f.getTPpath(intfId, uni)
	tpDownloadMsg := &ic.InterAdapterTechProfileDownloadMessage{UniId: uniId, Path: tpPath}
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

// This function adds onu info to cache
func (f *OpenOltFlowMgr) UpdateOnuInfo(intfID uint32, onuID uint32, serialNum string) {
	onu := onuInfo{intfId: intfID, onuId: onuID, serialNumber: serialNum}
	onuIDkey := onuIdKey{intfId: intfID, onuId: onuID}
	f.onuIds[onuIDkey] = onu
	log.Debugw("Updated onuinfo", log.Fields{"intfID": intfID, "onuID": onuID, "serialNum": serialNum})
}

// This function stores adds GEMport to ONU map
func (f *OpenOltFlowMgr) addGemPortToOnuInfoMap(intfId uint32, onuId uint32, gemPort uint32) {
	onuIDkey := onuIdKey{intfId: intfId, onuId: onuId}
	if val, ok := f.onuIds[onuIDkey]; ok {
		onuInfo := val
		gemPortKey := gemPortKey{intfId: intfId, gemPort: gemPort}
		f.onuGemPortIds[gemPortKey] = onuInfo
		log.Debugw("Cached Gemport to Onuinfo map", log.Fields{"GemPort": gemPort, "intfId": onuInfo.intfId, "onuId": onuInfo.onuId})
		return
	}
	log.Errorw("OnuInfo not found", log.Fields{"intfId": intfId, "onuId": onuId, "gemPort": gemPort})
}

// This function Lookup maps  by serialNumber or (intfId, gemPort)
// Returns OnuID,nil if found or set 0,error if no onuId is found for serialNumber or (intfId, gemPort)
func (f *OpenOltFlowMgr) getOnuIdfromGemPortMap(serialNumber string, intfId uint32, gemPortId uint32) (uint32, error) {
	log.Debugw("Getting ONU ID from GEM port and PON port", log.Fields{"serialNumber": serialNumber, "intfId": intfId, "gemPortId": gemPortId})
	if serialNumber != "" {
		if onuInfo, ok := f.onuSerialNumbers[serialNumber]; ok {
			return onuInfo.onuId, nil
		}
	} else {
		gemPortKey := gemPortKey{intfId: intfId, gemPort: gemPortId}
		if onuInfo, ok := f.onuGemPortIds[gemPortKey]; ok {
			log.Debugw("Retrived onu info from access", log.Fields{"intfId": intfId, "gemPortId": gemPortId, "onuId": onuInfo.onuId})
			return onuInfo.onuId, nil
		}
	}
	log.Errorw("ONU ID  is not found", log.Fields{"serialNumber": serialNumber, "intfId": intfId, "gemPort": gemPortId})
	return uint32(0), errors.New("Key Error ,ONU ID  is not found") // ONU ID 0 is not a valid one
}

// This function computes logical port UNI/NNI port from packet-in indication and returns the same
func (f *OpenOltFlowMgr) GetLogicalPortFromPacketIn(packetIn *openolt_pb2.PacketIndication) (uint32, error) {
	var logicalPortNum uint32
	var onuId uint32
	var err error

	if packetIn.IntfType == "pon" {
		// packet indication does not have serial number , so sending as nil
		if onuId, err = f.getOnuIdfromGemPortMap("", packetIn.IntfId, packetIn.GemportId); err != nil {
			log.Errorw("Unable to get ONU ID from GEM/PON port", log.Fields{"pon port": packetIn.IntfId, "gemport": packetIn.GemportId})
			return logicalPortNum, err
		}
		if packetIn.PortNo != 0 {
			logicalPortNum = packetIn.PortNo
		} else {
			uniId := uint32(0) //  FIXME - multi-uni support
			logicalPortNum = MkUniPortNum(packetIn.IntfId, onuId, uniId)
		}
		// Store the gem port through which the packet_in came. Use the same gem port for packet_out
		pktInkey := packetInInfoKey{intfId: packetIn.IntfId, onuId: onuId, logicalPort: logicalPortNum}
		f.packetInGemPort[pktInkey] = packetIn.GemportId
	} else if packetIn.IntfType == "nni" {
		logicalPortNum = IntfIdToPortNo(packetIn.IntfId, voltha.Port_ETHERNET_NNI)
	}
	log.Debugw("Retrieved logicalport from  packet-in", log.Fields{"logicalPortNum": logicalPortNum, "IntfType": packetIn.IntfType})
	return logicalPortNum, nil
}

func (f *OpenOltFlowMgr) GetPacketOutGemPortId(intfId uint32, onuId uint32, portNum uint32) (uint32, error) {
	var gemPortId uint32
	var err error
	key := packetInInfoKey{intfId: intfId, onuId: onuId, logicalPort: portNum}
	if val, ok := f.packetInGemPort[key]; ok {
		gemPortId = val
	} else {
		log.Errorw("Key-Error while fetching packet-out GEM port", log.Fields{"key": key})
		err = errors.New("Key-Error while fetching packet-out GEM port")
	}
	return gemPortId, err
}

func (f *OpenOltFlowMgr) addDHCPTrapFlowOnNNI(logicalFlow *ofp.OfpFlowStats, classifier map[string]interface{}, portNo uint32) {
	log.Debug("Adding trap-dhcp-of-nni-flow")
	action := make(map[string]interface{})
	classifier[PACKET_TAG_TYPE] = DOUBLE_TAG
	action[TRAP_TO_HOST] = true
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
	onuId := -1
	uniId := -1
	gemPortId := -1
	allocId := -1
	networkInterfaceId := DEFAULT_NETWORK_INTERFACE_ID
	flowStoreCookie := getFlowStoreCookie(classifier, uint32(0))
	if present := f.resourceMgr.IsFlowCookieOnKVStore(uint32(networkInterfaceId), uint32(onuId), uint32(uniId), flowStoreCookie); present {
		log.Debug("Flow-exists--not-re-adding")
		return
	}
	flowId, err := f.resourceMgr.GetFlowID(uint32(networkInterfaceId), uint32(onuId), uint32(uniId), flowStoreCookie, "")
	if err != nil {
		log.Errorw("Flow id unavailable for DHCP traponNNI flow", log.Fields{"error": err})
		return
	}
	var classifierProto *openolt_pb2.Classifier
	var actionProto *openolt_pb2.Action
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
	downstreamflow := openolt_pb2.Flow{AccessIntfId: int32(-1), // AccessIntfId not required
		OnuId:         int32(onuId), // OnuId not required
		UniId:         int32(uniId), // UniId not used
		FlowId:        flowId,
		FlowType:      DOWNSTREAM,
		AllocId:       int32(allocId),               // AllocId not used
		NetworkIntfId: DEFAULT_NETWORK_INTERFACE_ID, // one NNI port is supported now
		GemportId:     int32(gemPortId),             // GemportId not used
		Classifier:    classifierProto,
		Action:        actionProto,
		Priority:      int32(logicalFlow.Priority),
		Cookie:        logicalFlow.Cookie,
		PortNo:        portNo}
	if ok := f.addFlowToDevice(&downstreamflow); ok {
		log.Debug("DHCP trap on NNI flow added to device successfully")
		flowsToKVStore := f.getUpdatedFlowInfo(&downstreamflow, flowStoreCookie, "")
		if err := f.updateFlowInfoToKVStore(int32(networkInterfaceId),
			int32(onuId),
			int32(uniId),
			flowId, flowsToKVStore); err != nil {
			log.Errorw("Error uploading DHCP DL  flow into KV store", log.Fields{"flow": downstreamflow, "error": err})
		}
	}
	return
}
