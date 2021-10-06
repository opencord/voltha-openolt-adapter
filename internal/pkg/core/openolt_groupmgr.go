/*
 * Copyright 2020-present Open Networking Foundation
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

//Package core provides the utility for olt devices, flows, groups and statistics
package core

import (
	"context"
	"sync"

	"github.com/opencord/voltha-lib-go/v7/pkg/flows"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	plt "github.com/opencord/voltha-lib-go/v7/pkg/platform"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"
	ofp "github.com/opencord/voltha-protos/v5/go/openflow_13"
	openoltpb2 "github.com/opencord/voltha-protos/v5/go/openolt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//QueueInfoBrief has information about gemPortID and service priority associated with Mcast group
type QueueInfoBrief struct {
	gemPortID       uint32
	servicePriority uint32
}

//OpenOltGroupMgr creates the Structure of OpenOltGroupMgr obj
type OpenOltGroupMgr struct {
	deviceHandler                *DeviceHandler
	resourceMgr                  *rsrcMgr.OpenOltResourceMgr
	interfaceToMcastQueueMap     map[uint32]*QueueInfoBrief /*pon interface -> multicast queue map. Required to assign GEM to a bucket during group population*/
	interfaceToMcastQueueMapLock sync.RWMutex
}

//////////////////////////////////////////////
//            EXPORTED FUNCTIONS            //
//////////////////////////////////////////////

//NewGroupManager creates OpenOltGroupMgr object and initializes the parameters
func NewGroupManager(ctx context.Context, dh *DeviceHandler, rMgr *rsrcMgr.OpenOltResourceMgr) *OpenOltGroupMgr {
	logger.Infow(ctx, "initializing-group-manager", log.Fields{"device-id": dh.device.Id})
	var grpMgr OpenOltGroupMgr
	grpMgr.deviceHandler = dh
	grpMgr.resourceMgr = rMgr
	grpMgr.interfaceToMcastQueueMap = make(map[uint32]*QueueInfoBrief)
	grpMgr.interfaceToMcastQueueMapLock = sync.RWMutex{}
	logger.Info(ctx, "initialization-of-group-manager-success")
	return &grpMgr
}

// AddGroup add or update the group
func (g *OpenOltGroupMgr) AddGroup(ctx context.Context, group *ofp.OfpGroupEntry) error {
	logger.Infow(ctx, "add-group", log.Fields{"group": group})
	if group == nil {
		return olterrors.NewErrInvalidValue(log.Fields{"group": group}, nil)
	}
	groupToOlt := openoltpb2.Group{
		GroupId: group.Desc.GroupId,
		Command: openoltpb2.Group_SET_MEMBERS,
		Action:  g.buildGroupAction(),
	}
	logger.Debugw(ctx, "sending-group-to-device", log.Fields{"groupToOlt": groupToOlt})
	_, err := g.deviceHandler.Client.PerformGroupOperation(ctx, &groupToOlt)
	if err != nil {
		return olterrors.NewErrAdapter("add-group-operation-failed", log.Fields{"groupToOlt": groupToOlt}, err)
	}
	// group members not created yet. So let's store the group
	if err := g.resourceMgr.AddFlowGroupToKVStore(ctx, group, true); err != nil {
		return olterrors.NewErrPersistence("add", "flow-group", uint64(group.Desc.GroupId), log.Fields{"group": group}, err)
	}
	logger.Infow(ctx, "add-group-operation-performed-on-the-device-successfully ", log.Fields{"groupToOlt": groupToOlt})
	return nil
}

// DeleteGroup deletes a group from the device
func (g *OpenOltGroupMgr) DeleteGroup(ctx context.Context, group *ofp.OfpGroupEntry) error {
	logger.Debugw(ctx, "delete-group", log.Fields{"group": group})
	if group == nil {
		logger.Error(ctx, "unable-to-delete-group--invalid-argument--group-is-nil")
		return olterrors.NewErrInvalidValue(log.Fields{"group": group}, nil)
	}
	groupToOlt := openoltpb2.Group{
		GroupId: group.Desc.GroupId,
	}
	logger.Debugw(ctx, "deleting-group-from-device", log.Fields{"groupToOlt": groupToOlt})
	_, err := g.deviceHandler.Client.DeleteGroup(ctx, &groupToOlt)
	if err != nil {
		logger.Errorw(ctx, "delete-group-failed-on-dev", log.Fields{"groupToOlt": groupToOlt, "err": err})
		return olterrors.NewErrAdapter("delete-group-operation-failed", log.Fields{"groupToOlt": groupToOlt}, err)
	}
	//remove group from the store
	if err := g.resourceMgr.RemoveFlowGroupFromKVStore(ctx, group.Desc.GroupId, false); err != nil {
		return olterrors.NewErrPersistence("delete", "flow-group", uint64(group.Desc.GroupId), log.Fields{"group": group}, err)
	}
	logger.Debugw(ctx, "delete-group-operation-performed-on-the-device-successfully ", log.Fields{"groupToOlt": groupToOlt})
	return nil
}

// ModifyGroup updates the group
func (g *OpenOltGroupMgr) ModifyGroup(ctx context.Context, group *ofp.OfpGroupEntry) error {
	logger.Infow(ctx, "modify-group", log.Fields{"group": group})
	if group == nil || group.Desc == nil {
		return olterrors.NewErrInvalidValue(log.Fields{"group": group}, nil)
	}
	newGroup := g.buildGroup(ctx, group.Desc.GroupId, group.Desc.Buckets)
	//get existing members of the group
	val, groupExists, err := g.getFlowGroupFromKVStore(ctx, group.Desc.GroupId, false)
	if err != nil {
		return olterrors.NewErrNotFound("flow-group-in-kv-store", log.Fields{"groupId": group.Desc.GroupId}, err)
	}
	var current *openoltpb2.Group // represents the group on the device
	if groupExists {
		// group already exists
		current = g.buildGroup(ctx, group.Desc.GroupId, val.Desc.GetBuckets())
		logger.Debugw(ctx, "modify-group--group exists",
			log.Fields{
				"group on the device": val,
				"new":                 group})
	} else {
		current = g.buildGroup(ctx, group.Desc.GroupId, nil)
	}
	logger.Debugw(ctx, "modify-group--comparing-current-and-new",
		log.Fields{
			"group on the device": current,
			"new":                 newGroup})
	// get members to be added
	membersToBeAdded := g.findDiff(current, newGroup)
	// get members to be removed
	membersToBeRemoved := g.findDiff(newGroup, current)
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
		errAdd = g.callGroupAddRemove(ctx, &groupToOlt)
	}
	if len(membersToBeRemoved) > 0 {
		groupToOlt.Command = openoltpb2.Group_REMOVE_MEMBERS
		groupToOlt.Members = membersToBeRemoved
		//execute removeMembers
		errRemoved = g.callGroupAddRemove(ctx, &groupToOlt)
	}
	//save the modified group
	if errAdd == nil && errRemoved == nil {
		if err := g.resourceMgr.AddFlowGroupToKVStore(ctx, group, false); err != nil {
			return olterrors.NewErrPersistence("add", "flow-group", uint64(group.Desc.GroupId), log.Fields{"group": group}, err)
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

//LoadInterfaceToMulticastQueueMap reads multicast queues per interface from the KV store
//and put them into interfaceToMcastQueueMap.
func (g *OpenOltGroupMgr) LoadInterfaceToMulticastQueueMap(ctx context.Context) {
	storedMulticastQueueMap, err := g.resourceMgr.GetMcastQueuePerInterfaceMap(ctx)
	if err != nil {
		logger.Error(ctx, "failed-to-get-pon-interface-to-multicast-queue-map")
		return
	}
	for intf, queueInfo := range storedMulticastQueueMap {
		q := QueueInfoBrief{
			gemPortID:       queueInfo[0],
			servicePriority: queueInfo[1],
		}
		g.interfaceToMcastQueueMap[intf] = &q
	}
}

//GetInterfaceToMcastQueueMap gets the mcast queue mapped to to the PON interface
func (g *OpenOltGroupMgr) GetInterfaceToMcastQueueMap(intfID uint32) (*QueueInfoBrief, bool) {
	g.interfaceToMcastQueueMapLock.RLock()
	defer g.interfaceToMcastQueueMapLock.RUnlock()
	val, present := g.interfaceToMcastQueueMap[intfID]
	return val, present
}

//UpdateInterfaceToMcastQueueMap updates the mcast queue information mapped to a given PON interface
func (g *OpenOltGroupMgr) UpdateInterfaceToMcastQueueMap(intfID uint32, val *QueueInfoBrief) {
	g.interfaceToMcastQueueMapLock.Lock()
	defer g.interfaceToMcastQueueMapLock.Unlock()
	g.interfaceToMcastQueueMap[intfID] = val
}

////////////////////////////////////////////////
//      INTERNAL or UNEXPORTED FUNCTIONS      //
////////////////////////////////////////////////
//getFlowGroupFromKVStore fetches and returns flow group from the KV store. Returns (nil, false, error) if any problem occurs during
//fetching the data. Returns (group, true, nil) if the group is fetched and returned successfully.
//Returns (nil, false, nil) if the group does not exists in the KV store.
func (g *OpenOltGroupMgr) getFlowGroupFromKVStore(ctx context.Context, groupID uint32, cached bool) (*ofp.OfpGroupEntry, bool, error) {
	exists, groupInfo, err := g.resourceMgr.GetFlowGroupFromKVStore(ctx, groupID, cached)
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

//buildGroupAction creates and returns a group action
func (g *OpenOltGroupMgr) buildGroupAction() *openoltpb2.Action {
	var actionCmd openoltpb2.ActionCmd
	var action openoltpb2.Action
	action.Cmd = &actionCmd
	//pop outer vlan
	action.Cmd.RemoveOuterTag = true
	return &action
}

//callGroupAddRemove performs add/remove buckets operation for the indicated group
func (g *OpenOltGroupMgr) callGroupAddRemove(ctx context.Context, group *openoltpb2.Group) error {
	if err := g.performGroupOperation(ctx, group); err != nil {
		st, _ := status.FromError(err)
		//ignore already exists error code
		if st.Code() != codes.AlreadyExists {
			return olterrors.NewErrGroupOp("groupAddRemove", group.GroupId, log.Fields{"status": st}, err)
		}
	}
	return nil
}

//findDiff compares group members and finds members which only exists in groups2
func (g *OpenOltGroupMgr) findDiff(group1 *openoltpb2.Group, group2 *openoltpb2.Group) []*openoltpb2.GroupMember {
	var members []*openoltpb2.GroupMember
	for _, bucket := range group2.Members {
		if !g.contains(group1.Members, bucket) {
			// bucket does not exist and must be added
			members = append(members, bucket)
		}
	}
	return members
}

//contains returns true if the members list contains the given member; false otherwise
func (g *OpenOltGroupMgr) contains(members []*openoltpb2.GroupMember, member *openoltpb2.GroupMember) bool {
	for _, groupMember := range members {
		if groupMember.InterfaceId == member.InterfaceId {
			return true
		}
	}
	return false
}

//performGroupOperation call performGroupOperation operation of openolt proto
func (g *OpenOltGroupMgr) performGroupOperation(ctx context.Context, group *openoltpb2.Group) error {
	logger.Debugw(ctx, "sending-group-to-device",
		log.Fields{
			"groupToOlt": group,
			"command":    group.Command})
	_, err := g.deviceHandler.Client.PerformGroupOperation(log.WithSpanFromContext(context.Background(), ctx), group)
	if err != nil {
		return olterrors.NewErrAdapter("group-operation-failed", log.Fields{"groupToOlt": group}, err)
	}
	return nil
}

//buildGroup build openoltpb2.Group from given group id and bucket list
func (g *OpenOltGroupMgr) buildGroup(ctx context.Context, groupID uint32, buckets []*ofp.OfpBucket) *openoltpb2.Group {
	group := openoltpb2.Group{
		GroupId: groupID}
	// create members of the group
	for _, ofBucket := range buckets {
		member := g.buildMember(ctx, ofBucket)
		if member != nil && !g.contains(group.Members, member) {
			group.Members = append(group.Members, member)
		}
	}
	return &group
}

//buildMember builds openoltpb2.GroupMember from an OpenFlow bucket
func (g *OpenOltGroupMgr) buildMember(ctx context.Context, ofBucket *ofp.OfpBucket) *openoltpb2.GroupMember {
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
	interfaceID := plt.IntfIDFromUniPortNum(outPort)
	logger.Debugw(ctx, "got-associated-interface-id-of-the-port",
		log.Fields{
			"portNumber:":  outPort,
			"interfaceId:": interfaceID})
	g.interfaceToMcastQueueMapLock.RLock()
	defer g.interfaceToMcastQueueMapLock.RUnlock()
	if groupInfo, ok := g.interfaceToMcastQueueMap[interfaceID]; ok {
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
