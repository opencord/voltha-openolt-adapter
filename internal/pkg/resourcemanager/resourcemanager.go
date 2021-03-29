/*
 * Copyright 2019-present Open Networking Foundation

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

//Package resourcemanager provides the utility for managing resources
package resourcemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"

	"github.com/opencord/voltha-lib-go/v4/pkg/db"
	"github.com/opencord/voltha-lib-go/v4/pkg/db/kvstore"
	"github.com/opencord/voltha-lib-go/v4/pkg/log"
	ponrmgr "github.com/opencord/voltha-lib-go/v4/pkg/ponresourcemanager"
	ofp "github.com/opencord/voltha-protos/v4/go/openflow_13"
	"github.com/opencord/voltha-protos/v4/go/openolt"
)

const (
	// KvstoreTimeout specifies the time out for KV Store Connection
	KvstoreTimeout = 5 * time.Second
	// BasePathKvStore - <pathPrefix>/openolt/<device_id>
	BasePathKvStore = "%s/openolt/{%s}"
	// TpIDPathSuffix - <(pon_id, onu_id, uni_id)>/tp_id
	TpIDPathSuffix = "{%d,%d,%d}/tp_id"
	//MeterIDPathSuffix - <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
	MeterIDPathSuffix = "{%d,%d,%d}/{%d}/meter_id/{%s}"
	//NnniIntfID - nniintfids
	NnniIntfID = "nniintfids"
	// OnuPacketINPathPrefix - path prefix where ONU packet-in vlanID/PCP is stored
	//format: onu_packetin/{<intfid>,<onuid>,<logicalport>}
	OnuPacketINPathPrefix = "onu_packetin/{%d,%d,%d}"
	// OnuPacketINPath path on the kvstore to store packetin gemport,which will be used for packetin, packetout
	//format: onu_packetin/{<intfid>,<onuid>,<logicalport>}/{<vlanId>,<priority>}
	OnuPacketINPath = OnuPacketINPathPrefix + "/{%d,%d}"
	//FlowIDsForGem flowids_per_gem/<intfid>
	FlowIDsForGem = "flowids_per_gem/{%d}"
	//McastQueuesForIntf multicast queues for pon interfaces
	McastQueuesForIntf = "mcast_qs_for_int"
	//FlowGroup flow_groups/<flow_group_id>
	// A group is stored under this path on the KV store after it has been installed to the device.
	// It should also be deleted after it has been removed from the device accordingly.
	FlowGroup = "flow_groups/{%d}"
	//FlowGroupCached flow_groups_cached/<flow_group_id>
	// When a group add request received, we create the group without setting any members to it since we cannot
	// set any members to a group until it is associated with a multicast flow. It is a BAL limitation.
	// When the related multicast flow has been created we perform set members operation for the group.
	// That is why we need to keep the members of a group until the multicast flow creation request comes.
	// We preserve the groups under "FlowGroupsCached" directory in the KV store temporarily. Having set members,
	// we remove the group from the cached group store.
	FlowGroupCached = "flow_groups_cached/{%d}"

	//FlowIDPath - Path on the KV store for storing list of Flow IDs for a given subscriber
	//Format: BasePathKvStore/<(pon_intf_id, onu_id, uni_id)>/flow_ids
	FlowIDPath = "{%s}/flow_ids"
	//FlowIDInfoPath - Used to store more metadata associated with the flow_id
	//Format: BasePathKvStore/<(pon_intf_id, onu_id, uni_id)>/flow_id_info/<flow_id>
	FlowIDInfoPath = "{%s}/flow_id_info/{%d}"
)

// FlowInfo holds the flow information
type FlowInfo struct {
	Flow           *openolt.Flow
	IsSymmtricFlow bool
}

// OnuGemInfo holds onu information along with gem port list and uni port list
type OnuGemInfo struct {
	OnuID        uint32
	SerialNumber string
	IntfID       uint32
	GemPorts     []uint32
	UniPorts     []uint32
}

// PacketInInfoKey is the key for packet in gemport
type PacketInInfoKey struct {
	IntfID      uint32
	OnuID       uint32
	LogicalPort uint32
	VlanID      uint16
	Priority    uint8
}

// GroupInfo holds group information
type GroupInfo struct {
	GroupID  uint32
	OutPorts []uint32
}

// MeterInfo store meter information at path <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
type MeterInfo struct {
	RefCnt      uint8 // number of flow references for this meter. When RefCnt is 0, the MeterInfo should be deleted.
	MeterConfig ofp.OfpMeterConfig
}

// OpenOltResourceMgr holds resource related information as provided below for each field
type OpenOltResourceMgr struct {
	DeviceID   string      // OLT device id
	Address    string      // Host and port of the kv store to connect to
	Args       string      // args
	KVStore    *db.Backend // backend kv store connection handle
	DeviceType string
	DevInfo    *openolt.DeviceInfo // device information
	// array of pon resource managers per interface technology
	ResourceMgrs map[uint32]*ponrmgr.PONResourceManager

	// This protects concurrent gemport_id allocate/delete calls on a per PON port basis
	GemPortIDMgmtLock []sync.RWMutex
	// This protects concurrent alloc_id allocate/delete calls on a per PON port basis
	AllocIDMgmtLock []sync.RWMutex
	// This protects concurrent onu_id allocate/delete calls on a per PON port basis
	OnuIDMgmtLock []sync.RWMutex
}

func newKVClient(ctx context.Context, storeType string, address string, timeout time.Duration) (kvstore.Client, error) {
	logger.Infow(ctx, "kv-store-type", log.Fields{"store": storeType})
	switch storeType {
	case "etcd":
		return kvstore.NewEtcdClient(ctx, address, timeout, log.FatalLevel)
	}
	return nil, errors.New("unsupported-kv-store")
}

// SetKVClient sets the KV client and return a kv backend
func SetKVClient(ctx context.Context, backend string, addr string, DeviceID string, basePathKvStore string) *db.Backend {
	// TODO : Make sure direct call to NewBackend is working fine with backend , currently there is some
	// issue between kv store and backend , core is not calling NewBackend directly
	kvClient, err := newKVClient(ctx, backend, addr, KvstoreTimeout)
	if err != nil {
		logger.Fatalw(ctx, "Failed to init KV client\n", log.Fields{"err": err})
		return nil
	}

	kvbackend := &db.Backend{
		Client:     kvClient,
		StoreType:  backend,
		Address:    addr,
		Timeout:    KvstoreTimeout,
		PathPrefix: fmt.Sprintf(BasePathKvStore, basePathKvStore, DeviceID)}

	return kvbackend
}

// NewResourceMgr init a New resource manager instance which in turn instantiates pon resource manager
// instances according to technology. Initializes the default resource ranges for all
// the resources.
func NewResourceMgr(ctx context.Context, deviceID string, KVStoreAddress string, kvStoreType string, deviceType string, devInfo *openolt.DeviceInfo, basePathKvStore string) *OpenOltResourceMgr {
	var ResourceMgr OpenOltResourceMgr
	logger.Debugf(ctx, "Init new resource manager , address: %s, device-id: %s", KVStoreAddress, deviceID)
	ResourceMgr.DeviceID = deviceID
	ResourceMgr.Address = KVStoreAddress
	ResourceMgr.DeviceType = deviceType
	ResourceMgr.DevInfo = devInfo
	NumPONPorts := devInfo.GetPonPorts()

	Backend := kvStoreType
	ResourceMgr.KVStore = SetKVClient(ctx, Backend, ResourceMgr.Address, deviceID, basePathKvStore)
	if ResourceMgr.KVStore == nil {
		logger.Error(ctx, "Failed to setup KV store")
	}
	Ranges := make(map[string]*openolt.DeviceInfo_DeviceResourceRanges)
	RsrcMgrsByTech := make(map[string]*ponrmgr.PONResourceManager)
	ResourceMgr.ResourceMgrs = make(map[uint32]*ponrmgr.PONResourceManager)

	ResourceMgr.AllocIDMgmtLock = make([]sync.RWMutex, NumPONPorts)
	ResourceMgr.GemPortIDMgmtLock = make([]sync.RWMutex, NumPONPorts)
	ResourceMgr.OnuIDMgmtLock = make([]sync.RWMutex, NumPONPorts)

	// TODO self.args = registry('main').get_args()

	/*
	   If a legacy driver returns protobuf without any ranges,s synthesize one from
	   the legacy global per-device information. This, in theory, is temporary until
	   the legacy drivers are upgrade to support pool ranges.
	*/
	if devInfo.Ranges == nil {
		var ranges openolt.DeviceInfo_DeviceResourceRanges
		ranges.Technology = devInfo.GetTechnology()

		var index uint32
		for index = 0; index < NumPONPorts; index++ {
			ranges.IntfIds = append(ranges.IntfIds, index)
		}

		var Pool openolt.DeviceInfo_DeviceResourceRanges_Pool
		Pool.Type = openolt.DeviceInfo_DeviceResourceRanges_Pool_ONU_ID
		Pool.Start = devInfo.OnuIdStart
		Pool.End = devInfo.OnuIdEnd
		Pool.Sharing = openolt.DeviceInfo_DeviceResourceRanges_Pool_DEDICATED_PER_INTF
		onuPool := Pool
		ranges.Pools = append(ranges.Pools, &onuPool)

		Pool.Type = openolt.DeviceInfo_DeviceResourceRanges_Pool_ALLOC_ID
		Pool.Start = devInfo.AllocIdStart
		Pool.End = devInfo.AllocIdEnd
		Pool.Sharing = openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH
		allocPool := Pool
		ranges.Pools = append(ranges.Pools, &allocPool)

		Pool.Type = openolt.DeviceInfo_DeviceResourceRanges_Pool_GEMPORT_ID
		Pool.Start = devInfo.GemportIdStart
		Pool.End = devInfo.GemportIdEnd
		Pool.Sharing = openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH
		gemPool := Pool
		ranges.Pools = append(ranges.Pools, &gemPool)

		Pool.Type = openolt.DeviceInfo_DeviceResourceRanges_Pool_FLOW_ID
		Pool.Start = devInfo.FlowIdStart
		Pool.End = devInfo.FlowIdEnd
		Pool.Sharing = openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH
		ranges.Pools = append(ranges.Pools, &Pool)
		// Add to device info
		devInfo.Ranges = append(devInfo.Ranges, &ranges)
	}

	// Create a separate Resource Manager instance for each range. This assumes that
	// each technology is represented by only a single range
	var GlobalPONRsrcMgr *ponrmgr.PONResourceManager
	var err error
	for _, TechRange := range devInfo.Ranges {
		technology := TechRange.Technology
		logger.Debugf(ctx, "Device info technology %s", technology)
		Ranges[technology] = TechRange

		RsrcMgrsByTech[technology], err = ponrmgr.NewPONResourceManager(ctx, technology, deviceType, deviceID,
			Backend, ResourceMgr.Address, basePathKvStore)
		if err != nil {
			logger.Errorf(ctx, "Failed to create pon resource manager instance for technology %s", technology)
			return nil
		}
		// resource_mgrs_by_tech[technology] = resource_mgr
		if GlobalPONRsrcMgr == nil {
			GlobalPONRsrcMgr = RsrcMgrsByTech[technology]
		}
		for _, IntfID := range TechRange.IntfIds {
			ResourceMgr.ResourceMgrs[IntfID] = RsrcMgrsByTech[technology]
		}
		// self.initialize_device_resource_range_and_pool(resource_mgr, global_resource_mgr, arange)
		InitializeDeviceResourceRangeAndPool(ctx, RsrcMgrsByTech[technology], GlobalPONRsrcMgr,
			TechRange, devInfo)
	}
	// After we have initialized resource ranges, initialize the
	// resource pools accordingly.
	for _, PONRMgr := range RsrcMgrsByTech {
		_ = PONRMgr.InitDeviceResourcePool(ctx)
	}
	logger.Info(ctx, "Initialization of  resource manager success!")
	return &ResourceMgr
}

// InitializeDeviceResourceRangeAndPool initializes the resource range pool according to the sharing type, then apply
// device specific information. If KV doesn't exist
// or is broader than the device, the device's information will
// dictate the range limits
func InitializeDeviceResourceRangeAndPool(ctx context.Context, ponRMgr *ponrmgr.PONResourceManager, globalPONRMgr *ponrmgr.PONResourceManager,
	techRange *openolt.DeviceInfo_DeviceResourceRanges, devInfo *openolt.DeviceInfo) {

	// init the resource range pool according to the sharing type

	logger.Debugf(ctx, "Resource range pool init for technology %s", ponRMgr.Technology)
	// first load from KV profiles
	status := ponRMgr.InitResourceRangesFromKVStore(ctx)
	if !status {
		logger.Debugf(ctx, "Failed to load resource ranges from KV store for tech %s", ponRMgr.Technology)
	}

	/*
	   Then apply device specific information. If KV doesn't exist
	   or is broader than the device, the device's information will
	   dictate the range limits
	*/
	logger.Debugw(ctx, "Using device info to init pon resource ranges", log.Fields{"Tech": ponRMgr.Technology})

	ONUIDStart := devInfo.OnuIdStart
	ONUIDEnd := devInfo.OnuIdEnd
	ONUIDShared := openolt.DeviceInfo_DeviceResourceRanges_Pool_DEDICATED_PER_INTF
	ONUIDSharedPoolID := uint32(0)
	AllocIDStart := devInfo.AllocIdStart
	AllocIDEnd := devInfo.AllocIdEnd
	AllocIDShared := openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH // TODO EdgeCore/BAL limitation
	AllocIDSharedPoolID := uint32(0)
	GEMPortIDStart := devInfo.GemportIdStart
	GEMPortIDEnd := devInfo.GemportIdEnd
	GEMPortIDShared := openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH // TODO EdgeCore/BAL limitation
	GEMPortIDSharedPoolID := uint32(0)
	FlowIDStart := devInfo.FlowIdStart
	FlowIDEnd := devInfo.FlowIdEnd
	FlowIDShared := openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH // TODO EdgeCore/BAL limitation
	FlowIDSharedPoolID := uint32(0)

	var FirstIntfPoolID uint32
	var SharedPoolID uint32

	/*
	 * As a zero check is made against SharedPoolID to check whether the resources are shared across all intfs
	 * if resources are shared across interfaces then SharedPoolID is given a positive number.
	 */
	for _, FirstIntfPoolID = range techRange.IntfIds {
		// skip the intf id 0
		if FirstIntfPoolID == 0 {
			continue
		}
		break
	}

	for _, RangePool := range techRange.Pools {
		if RangePool.Sharing == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
			SharedPoolID = FirstIntfPoolID
		} else if RangePool.Sharing == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_SAME_TECH {
			SharedPoolID = FirstIntfPoolID
		} else {
			SharedPoolID = 0
		}
		if RangePool.Type == openolt.DeviceInfo_DeviceResourceRanges_Pool_ONU_ID {
			ONUIDStart = RangePool.Start
			ONUIDEnd = RangePool.End
			ONUIDShared = RangePool.Sharing
			ONUIDSharedPoolID = SharedPoolID
		} else if RangePool.Type == openolt.DeviceInfo_DeviceResourceRanges_Pool_ALLOC_ID {
			AllocIDStart = RangePool.Start
			AllocIDEnd = RangePool.End
			AllocIDShared = RangePool.Sharing
			AllocIDSharedPoolID = SharedPoolID
		} else if RangePool.Type == openolt.DeviceInfo_DeviceResourceRanges_Pool_GEMPORT_ID {
			GEMPortIDStart = RangePool.Start
			GEMPortIDEnd = RangePool.End
			GEMPortIDShared = RangePool.Sharing
			GEMPortIDSharedPoolID = SharedPoolID
		} else if RangePool.Type == openolt.DeviceInfo_DeviceResourceRanges_Pool_FLOW_ID {
			FlowIDStart = RangePool.Start
			FlowIDEnd = RangePool.End
			FlowIDShared = RangePool.Sharing
			FlowIDSharedPoolID = SharedPoolID
		}
	}

	logger.Debugw(ctx, "Device info init", log.Fields{"technology": techRange.Technology,
		"onu_id_start": ONUIDStart, "onu_id_end": ONUIDEnd, "onu_id_shared_pool_id": ONUIDSharedPoolID,
		"alloc_id_start": AllocIDStart, "alloc_id_end": AllocIDEnd,
		"alloc_id_shared_pool_id": AllocIDSharedPoolID,
		"gemport_id_start":        GEMPortIDStart, "gemport_id_end": GEMPortIDEnd,
		"gemport_id_shared_pool_id": GEMPortIDSharedPoolID,
		"flow_id_start":             FlowIDStart,
		"flow_id_end_idx":           FlowIDEnd,
		"flow_id_shared_pool_id":    FlowIDSharedPoolID,
		"intf_ids":                  techRange.IntfIds,
		"uni_id_start":              0,
		"uni_id_end_idx":            1, /*MaxUNIIDperONU()*/
	})

	ponRMgr.InitDefaultPONResourceRanges(ctx, ONUIDStart, ONUIDEnd, ONUIDSharedPoolID,
		AllocIDStart, AllocIDEnd, AllocIDSharedPoolID,
		GEMPortIDStart, GEMPortIDEnd, GEMPortIDSharedPoolID,
		FlowIDStart, FlowIDEnd, FlowIDSharedPoolID, 0, 1,
		devInfo.PonPorts, techRange.IntfIds)

	// For global sharing, make sure to refresh both local and global resource manager instances' range

	if ONUIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		globalPONRMgr.UpdateRanges(ctx, ponrmgr.ONU_ID_START_IDX, ONUIDStart, ponrmgr.ONU_ID_END_IDX, ONUIDEnd,
			"", 0, nil)
		ponRMgr.UpdateRanges(ctx, ponrmgr.ONU_ID_START_IDX, ONUIDStart, ponrmgr.ONU_ID_END_IDX, ONUIDEnd,
			"", 0, globalPONRMgr)
	}
	if AllocIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		globalPONRMgr.UpdateRanges(ctx, ponrmgr.ALLOC_ID_START_IDX, AllocIDStart, ponrmgr.ALLOC_ID_END_IDX, AllocIDEnd,
			"", 0, nil)

		ponRMgr.UpdateRanges(ctx, ponrmgr.ALLOC_ID_START_IDX, AllocIDStart, ponrmgr.ALLOC_ID_END_IDX, AllocIDEnd,
			"", 0, globalPONRMgr)
	}
	if GEMPortIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		globalPONRMgr.UpdateRanges(ctx, ponrmgr.GEMPORT_ID_START_IDX, GEMPortIDStart, ponrmgr.GEMPORT_ID_END_IDX, GEMPortIDEnd,
			"", 0, nil)
		ponRMgr.UpdateRanges(ctx, ponrmgr.GEMPORT_ID_START_IDX, GEMPortIDStart, ponrmgr.GEMPORT_ID_END_IDX, GEMPortIDEnd,
			"", 0, globalPONRMgr)
	}
	if FlowIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		globalPONRMgr.UpdateRanges(ctx, ponrmgr.FLOW_ID_START_IDX, FlowIDStart, ponrmgr.FLOW_ID_END_IDX, FlowIDEnd,
			"", 0, nil)
		ponRMgr.UpdateRanges(ctx, ponrmgr.FLOW_ID_START_IDX, FlowIDStart, ponrmgr.FLOW_ID_END_IDX, FlowIDEnd,
			"", 0, globalPONRMgr)
	}

	// Make sure loaded range fits the platform bit encoding ranges
	ponRMgr.UpdateRanges(ctx, ponrmgr.UNI_ID_START_IDX, 0, ponrmgr.UNI_ID_END_IDX /* TODO =OpenOltPlatform.MAX_UNIS_PER_ONU-1*/, 1, "", 0, nil)
}

// Delete clears used resources for the particular olt device being deleted
func (RsrcMgr *OpenOltResourceMgr) Delete(ctx context.Context) error {
	/* TODO
	   def __del__(self):
	           self.log.info("clearing-device-resource-pool")
	           for key, resource_mgr in self.resource_mgrs.iteritems():
	               resource_mgr.clear_device_resource_pool()

	       def assert_pon_id_limit(self, pon_intf_id):
	           assert pon_intf_id in self.resource_mgrs

	       def assert_onu_id_limit(self, pon_intf_id, onu_id):
	           self.assert_pon_id_limit(pon_intf_id)
	           self.resource_mgrs[pon_intf_id].assert_resource_limits(onu_id, PONResourceManager.ONU_ID)

	       @property
	       def max_uni_id_per_onu(self):
	           return 0 #OpenOltPlatform.MAX_UNIS_PER_ONU-1, zero-based indexing Uncomment or override to make default multi-uni

	       def assert_uni_id_limit(self, pon_intf_id, onu_id, uni_id):
	           self.assert_onu_id_limit(pon_intf_id, onu_id)
	           self.resource_mgrs[pon_intf_id].assert_resource_limits(uni_id, PONResourceManager.UNI_ID)
	*/
	for _, rsrcMgr := range RsrcMgr.ResourceMgrs {
		if err := rsrcMgr.ClearDeviceResourcePool(ctx); err != nil {
			logger.Debug(ctx, "Failed to clear device resource pool")
			return err
		}
	}
	logger.Debug(ctx, "Cleared device resource pool")
	return nil
}

// GetONUID returns the available OnuID for the given pon-port
func (RsrcMgr *OpenOltResourceMgr) GetONUID(ctx context.Context, ponIntfID uint32) (uint32, error) {
	// Check if Pon Interface ID is present in Resource-manager-map
	RsrcMgr.OnuIDMgmtLock[ponIntfID].Lock()
	defer RsrcMgr.OnuIDMgmtLock[ponIntfID].Unlock()

	if _, ok := RsrcMgr.ResourceMgrs[ponIntfID]; !ok {
		err := errors.New("invalid-pon-interface-" + strconv.Itoa(int(ponIntfID)))
		return 0, err
	}
	// Get ONU id for a provided pon interface ID.
	onuID, err := RsrcMgr.ResourceMgrs[ponIntfID].TechProfileMgr.GetResourceID(ctx, ponIntfID,
		ponrmgr.ONU_ID, 1)
	if err != nil {
		logger.Errorf(ctx, "Failed to get resource for interface %d for type %s",
			ponIntfID, ponrmgr.ONU_ID)
		return 0, err
	}
	if onuID != nil {
		RsrcMgr.ResourceMgrs[ponIntfID].InitResourceMap(ctx, fmt.Sprintf("%d,%d", ponIntfID, onuID[0]))
		return onuID[0], err
	}

	return 0, err // return OnuID 0 on error
}

// GetFlowIDInfo returns the slice of flow info of the given pon-port
// Note: For flows which trap from the NNI and not really associated with any particular
// ONU (like LLDP), the onu_id and uni_id is set as -1. The intf_id is the NNI intf_id.
func (RsrcMgr *OpenOltResourceMgr) GetFlowIDInfo(ctx context.Context, ponIntfID uint32, onuID int32, uniID int32, flowID uint64) *FlowInfo {
	var flowInfo FlowInfo

	subs := fmt.Sprintf("%d,%d,%d", ponIntfID, onuID, uniID)
	Path := fmt.Sprintf(FlowIDInfoPath, subs, flowID)
	value, err := RsrcMgr.KVStore.Get(ctx, Path)
	if err == nil {
		if value != nil {
			Val, err := toByte(value.Value)
			if err != nil {
				logger.Errorw(ctx, "Failed to convert flowinfo into byte array", log.Fields{"error": err, "subs": subs})
				return nil
			}
			if err = json.Unmarshal(Val, &flowInfo); err != nil {
				logger.Errorw(ctx, "Failed to unmarshal", log.Fields{"error": err, "subs": subs})
				return nil
			}
		}
	}
	if flowInfo.Flow == nil {
		logger.Debugw(ctx, "No flowInfo found in KV store", log.Fields{"subs": subs})
		return nil
	}
	return &flowInfo
}

// GetCurrentFlowIDsForOnu fetches flow ID from the resource manager
// Note: For flows which trap from the NNI and not really associated with any particular
// ONU (like LLDP), the onu_id and uni_id is set as -1. The intf_id is the NNI intf_id.
func (RsrcMgr *OpenOltResourceMgr) GetCurrentFlowIDsForOnu(ctx context.Context, ponIntfID uint32, onuID int32, uniID int32) ([]uint64, error) {

	subs := fmt.Sprintf("%d,%d,%d", ponIntfID, onuID, uniID)
	path := fmt.Sprintf(FlowIDPath, subs)

	var data []uint64
	value, err := RsrcMgr.KVStore.Get(ctx, path)
	if err == nil {
		if value != nil {
			Val, _ := toByte(value.Value)
			if err = json.Unmarshal(Val, &data); err != nil {
				logger.Error(ctx, "Failed to unmarshal")
				return nil, err
			}
		}
	}
	return data, nil
}

// UpdateFlowIDInfo updates flow info for the given pon interface, onu id, and uni id
// Note: For flows which trap from the NNI and not really associated with any particular
// ONU (like LLDP), the onu_id and uni_id is set as -1. The intf_id is the NNI intf_id.
func (RsrcMgr *OpenOltResourceMgr) UpdateFlowIDInfo(ctx context.Context, ponIntfID uint32, onuID int32, uniID int32,
	flowID uint64, flowData FlowInfo) error {

	subs := fmt.Sprintf("%d,%d,%d", ponIntfID, onuID, uniID)
	path := fmt.Sprintf(FlowIDInfoPath, subs, flowID)

	var value []byte
	var err error
	value, err = json.Marshal(flowData)
	if err != nil {
		logger.Errorf(ctx, "failed to Marshal, resource path %s", path)
		return err
	}

	if err = RsrcMgr.KVStore.Put(ctx, path, value); err != nil {
		logger.Errorf(ctx, "Failed to update resource %s", path)
	}

	// Update the flowID list for the ONU
	if err = RsrcMgr.UpdateFlowIDForOnu(ctx, ponIntfID, onuID, uniID, flowID, true); err != nil {
		// If the operation fails, try to remove FlowInfo from the KV store
		_ = RsrcMgr.KVStore.Delete(ctx, path)
		return err
	}
	return err
}

// UpdateFlowIDForOnu updates the flow_id list of the ONU (add or remove flow_id from the list)
func (RsrcMgr *OpenOltResourceMgr) UpdateFlowIDForOnu(ctx context.Context, ponIntfID uint32, onuID int32, uniID int32, flowID uint64, add bool) error {
	/*
	   Update the flow_id list of the ONU (add or remove flow_id from the list)
	   :param pon_intf_onu_id: reference of PON interface id and onu id
	   :param flow_id: flow ID
	   :param add: Boolean flag to indicate whether the flow_id should be
	               added or removed from the list. Defaults to adding the flow.
	*/
	var Value []byte
	var err error
	var retVal bool
	var idx uint64
	subs := fmt.Sprintf("%d,%d,%d", ponIntfID, onuID, uniID)
	path := fmt.Sprintf(FlowIDPath, subs)
	flowIDs, err := RsrcMgr.GetCurrentFlowIDsForOnu(ctx, ponIntfID, onuID, uniID)
	if err != nil {
		// Error logged in the called function
		return err
	}

	if add {
		if retVal, _ = checkForFlowIDInList(flowIDs, flowID); retVal {
			return nil
		}
		flowIDs = append(flowIDs, flowID)
	} else {
		if retVal, idx = checkForFlowIDInList(flowIDs, flowID); !retVal {
			return nil
		}
		// delete the index and shift
		flowIDs = append(flowIDs[:idx], flowIDs[idx+1:]...)
	}
	Value, err = json.Marshal(flowIDs)
	if err != nil {
		logger.Error(ctx, "Failed to Marshal")
		return err
	}

	if err = RsrcMgr.KVStore.Put(ctx, path, Value); err != nil {
		logger.Errorf(ctx, "Failed to update resource %s", path)
		return err
	}
	return err
}

// RemoveFlowIDInfo remove flow info for the given pon interface, onu id, and uni id
// Note: For flows which trap from the NNI and not really associated with any particular
// ONU (like LLDP), the onu_id and uni_id is set as -1. The intf_id is the NNI intf_id.
func (RsrcMgr *OpenOltResourceMgr) RemoveFlowIDInfo(ctx context.Context, ponIntfID uint32, onuID int32, uniID int32,
	flowID uint64) error {

	subs := fmt.Sprintf("%d,%d,%d", ponIntfID, onuID, uniID)
	path := fmt.Sprintf(FlowIDInfoPath, subs, flowID)

	var err error
	if err = RsrcMgr.KVStore.Delete(ctx, path); err != nil {
		logger.Errorf(ctx, "Failed to delete resource %s", path)
		return err
	}

	// Update the flowID list for the ONU
	err = RsrcMgr.UpdateFlowIDForOnu(ctx, ponIntfID, onuID, uniID, flowID, false)

	return err
}

// RemoveAllFlowsForIntfOnuUniKey removes flow info for the given interface, onu id, and uni id
func (RsrcMgr *OpenOltResourceMgr) RemoveAllFlowsForIntfOnuUniKey(ctx context.Context, intf uint32, onuID int32, uniID int32) error {
	flowIDs, err := RsrcMgr.GetCurrentFlowIDsForOnu(ctx, intf, onuID, uniID)
	if err != nil {
		// error logged in the called function
		return err
	}
	for _, flID := range flowIDs {
		if err := RsrcMgr.RemoveFlowIDInfo(ctx, intf, onuID, uniID, flID); err != nil {
			logger.Errorw(ctx, "failed-to-delete-flow-id-info", log.Fields{"intf": intf, "onuID": onuID, "uniID": uniID, "flowID": flID})
		}
	}
	subs := fmt.Sprintf("%d,%d,%d", intf, onuID, uniID)
	path := fmt.Sprintf(FlowIDPath, subs)
	if err := RsrcMgr.KVStore.Delete(ctx, path); err != nil {
		logger.Errorf(ctx, "Failed to delete resource %s", path)
		return err
	}
	return nil
}

// GetAllocID return the first Alloc ID for a given pon interface id and onu id and then update the resource map on
// the KV store with the list of alloc_ids allocated for the pon_intf_onu_id tuple
// Currently of all the alloc_ids available, it returns the first alloc_id in the list for tha given ONU
func (RsrcMgr *OpenOltResourceMgr) GetAllocID(ctx context.Context, intfID uint32, onuID uint32, uniID uint32) uint32 {

	var err error
	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)

	RsrcMgr.AllocIDMgmtLock[intfID].Lock()
	defer RsrcMgr.AllocIDMgmtLock[intfID].Unlock()

	AllocID := RsrcMgr.ResourceMgrs[intfID].GetCurrentAllocIDForOnu(ctx, IntfOnuIDUniID)
	if AllocID != nil {
		// Since we support only one alloc_id for the ONU at the moment,
		// return the first alloc_id in the list, if available, for that
		// ONU.
		logger.Debugw(ctx, "Retrieved alloc ID from pon resource mgr", log.Fields{"AllocID": AllocID})
		return AllocID[0]
	}
	AllocID, err = RsrcMgr.ResourceMgrs[intfID].GetResourceID(ctx, intfID,
		ponrmgr.ALLOC_ID, 1)

	if AllocID == nil || err != nil {
		logger.Error(ctx, "Failed to allocate alloc id")
		return 0
	}
	// update the resource map on KV store with the list of alloc_id
	// allocated for the pon_intf_onu_id tuple
	err = RsrcMgr.ResourceMgrs[intfID].UpdateAllocIdsForOnu(ctx, IntfOnuIDUniID, AllocID)
	if err != nil {
		logger.Error(ctx, "Failed to update Alloc ID")
		return 0
	}
	logger.Debugw(ctx, "Allocated new Tcont from pon resource mgr", log.Fields{"AllocID": AllocID})
	return AllocID[0]
}

// UpdateAllocIdsForOnu updates alloc ids in kv store for a given pon interface id, onu id and uni id
func (RsrcMgr *OpenOltResourceMgr) UpdateAllocIdsForOnu(ctx context.Context, ponPort uint32, onuID uint32, uniID uint32, allocID []uint32) error {

	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", ponPort, onuID, uniID)
	return RsrcMgr.ResourceMgrs[ponPort].UpdateAllocIdsForOnu(ctx, IntfOnuIDUniID,
		allocID)
}

// GetCurrentGEMPortIDsForOnu returns gem ports for given pon interface , onu id and uni id
func (RsrcMgr *OpenOltResourceMgr) GetCurrentGEMPortIDsForOnu(ctx context.Context, intfID uint32, onuID uint32,
	uniID uint32) []uint32 {

	/* Get gem ports for given pon interface , onu id and uni id. */

	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)
	return RsrcMgr.ResourceMgrs[intfID].GetCurrentGEMPortIDsForOnu(ctx, IntfOnuIDUniID)
}

// GetCurrentAllocIDsForOnu returns alloc ids for given pon interface and onu id
func (RsrcMgr *OpenOltResourceMgr) GetCurrentAllocIDsForOnu(ctx context.Context, intfID uint32, onuID uint32, uniID uint32) []uint32 {

	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)
	AllocID := RsrcMgr.ResourceMgrs[intfID].GetCurrentAllocIDForOnu(ctx, IntfOnuIDUniID)
	if AllocID != nil {
		return AllocID
	}
	return []uint32{}
}

// RemoveAllocIDForOnu removes the alloc id for given pon interface, onu id, uni id and alloc id
func (RsrcMgr *OpenOltResourceMgr) RemoveAllocIDForOnu(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, allocID uint32) {
	allocIDs := RsrcMgr.GetCurrentAllocIDsForOnu(ctx, intfID, onuID, uniID)
	for i := 0; i < len(allocIDs); i++ {
		if allocIDs[i] == allocID {
			allocIDs = append(allocIDs[:i], allocIDs[i+1:]...)
			break
		}
	}
	err := RsrcMgr.UpdateAllocIdsForOnu(ctx, intfID, onuID, uniID, allocIDs)
	if err != nil {
		logger.Errorf(ctx, "Failed to Remove Alloc Id For Onu. IntfID %d onuID %d uniID %d allocID %d",
			intfID, onuID, uniID, allocID)
	}
}

// RemoveGemPortIDForOnu removes the gem port id for given pon interface, onu id, uni id and gem port id
func (RsrcMgr *OpenOltResourceMgr) RemoveGemPortIDForOnu(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, gemPortID uint32) {
	gemPortIDs := RsrcMgr.GetCurrentGEMPortIDsForOnu(ctx, intfID, onuID, uniID)
	for i := 0; i < len(gemPortIDs); i++ {
		if gemPortIDs[i] == gemPortID {
			gemPortIDs = append(gemPortIDs[:i], gemPortIDs[i+1:]...)
			break
		}
	}
	err := RsrcMgr.UpdateGEMPortIDsForOnu(ctx, intfID, onuID, uniID, gemPortIDs)
	if err != nil {
		logger.Errorf(ctx, "Failed to Remove Gem Id For Onu. IntfID %d onuID %d uniID %d gemPortId %d",
			intfID, onuID, uniID, gemPortID)
	}
}

//GetUniPortByPonPortGemPortFromKVStore retrieves onu and uni ID associated with the pon and gem ports.
func (RsrcMgr *OpenOltResourceMgr) GetUniPortByPonPortGemPortFromKVStore(ctx context.Context, PonPort uint32, GemPort uint32) (uint32, uint32, error) {
	IntfGEMPortPath := fmt.Sprintf("%d,%d", PonPort, GemPort)
	logger.Debugf(ctx, "Getting ONU and UNI IDs from the path %s", IntfGEMPortPath)
	var Data []uint32
	Value, err := RsrcMgr.KVStore.Get(ctx, IntfGEMPortPath)
	if err == nil {
		if Value != nil {
			Val, _ := ponrmgr.ToByte(Value.Value)
			if err = json.Unmarshal(Val, &Data); err != nil {
				logger.Errorw(ctx, "Failed to unmarshal", log.Fields{"error": err})
				return 0, 0, errors.New("failed to unmarshal the data retrieved")
			}
		}
	} else {
		logger.Errorf(ctx, "Failed to get data from kvstore for %s", IntfGEMPortPath, err)
		return 0, 0, errors.New("could not get data")
	}
	if len(Data) < 2 {
		return 0, 0, errors.New("invalid data format")
	}
	return Data[0], Data[1], nil
}

// UpdateGEMportsPonportToOnuMapOnKVStore updates onu and uni id associated with the gem port to the kv store
// This stored information is used when packet_indication is received and we need to derive the ONU Id for which
// the packet arrived based on the pon_intf and gemport available in the packet_indication
func (RsrcMgr *OpenOltResourceMgr) UpdateGEMportsPonportToOnuMapOnKVStore(ctx context.Context, gemPorts []uint32, PonPort uint32,
	onuID uint32, uniID uint32) error {

	/* Update onu and uni id associated with the gem port to the kv store. */
	var IntfGEMPortPath string
	Data := []uint32{onuID, uniID}
	for _, GEM := range gemPorts {
		IntfGEMPortPath = fmt.Sprintf("%d,%d", PonPort, GEM)
		Val, err := json.Marshal(Data)
		if err != nil {
			logger.Error(ctx, "failed to Marshal")
			return err
		}

		if err = RsrcMgr.KVStore.Put(ctx, IntfGEMPortPath, Val); err != nil {
			logger.Errorf(ctx, "Failed to update resource %s", IntfGEMPortPath)
			return err
		}
	}
	return nil
}

// RemoveGEMportPonportToOnuMapOnKVStore removes the relationship between the gem port and pon port
func (RsrcMgr *OpenOltResourceMgr) RemoveGEMportPonportToOnuMapOnKVStore(ctx context.Context, GemPort uint32, PonPort uint32) {
	IntfGEMPortPath := fmt.Sprintf("%d,%d", PonPort, GemPort)
	err := RsrcMgr.KVStore.Delete(ctx, IntfGEMPortPath)
	if err != nil {
		logger.Errorf(ctx, "Failed to Remove Gem port-Pon port to onu map on kv store. Gem %d PonPort %d", GemPort, PonPort)
	}
}

// GetGEMPortID gets gem port id for a particular pon port, onu id and uni id and then update the resource map on
// the KV store with the list of gemport_id allocated for the pon_intf_onu_id tuple
func (RsrcMgr *OpenOltResourceMgr) GetGEMPortID(ctx context.Context, ponPort uint32, onuID uint32,
	uniID uint32, NumOfPorts uint32) ([]uint32, error) {

	/* Get gem port id for a particular pon port, onu id
	   and uni id.
	*/

	var err error
	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", ponPort, onuID, uniID)

	RsrcMgr.GemPortIDMgmtLock[ponPort].Lock()
	defer RsrcMgr.GemPortIDMgmtLock[ponPort].Unlock()

	GEMPortList := RsrcMgr.ResourceMgrs[ponPort].GetCurrentGEMPortIDsForOnu(ctx, IntfOnuIDUniID)
	if GEMPortList != nil {
		return GEMPortList, nil
	}

	GEMPortList, err = RsrcMgr.ResourceMgrs[ponPort].GetResourceID(ctx, ponPort,
		ponrmgr.GEMPORT_ID, NumOfPorts)
	if err != nil && GEMPortList == nil {
		logger.Errorf(ctx, "Failed to get gem port id for %s", IntfOnuIDUniID)
		return nil, err
	}

	// update the resource map on KV store with the list of gemport_id
	// allocated for the pon_intf_onu_id tuple
	err = RsrcMgr.ResourceMgrs[ponPort].UpdateGEMPortIDsForOnu(ctx, IntfOnuIDUniID,
		GEMPortList)
	if err != nil {
		logger.Errorf(ctx, "Failed to update GEM ports to kv store for %s", IntfOnuIDUniID)
		return nil, err
	}
	_ = RsrcMgr.UpdateGEMportsPonportToOnuMapOnKVStore(ctx, GEMPortList, ponPort,
		onuID, uniID)
	return GEMPortList, err
}

// UpdateGEMPortIDsForOnu updates gemport ids on to the kv store for a given pon port, onu id and uni id
func (RsrcMgr *OpenOltResourceMgr) UpdateGEMPortIDsForOnu(ctx context.Context, ponPort uint32, onuID uint32,
	uniID uint32, GEMPortList []uint32) error {
	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", ponPort, onuID, uniID)
	return RsrcMgr.ResourceMgrs[ponPort].UpdateGEMPortIDsForOnu(ctx, IntfOnuIDUniID,
		GEMPortList)

}

// FreeonuID releases(make free) onu id for a particular pon-port
func (RsrcMgr *OpenOltResourceMgr) FreeonuID(ctx context.Context, intfID uint32, onuID []uint32) {

	RsrcMgr.OnuIDMgmtLock[intfID].Lock()
	defer RsrcMgr.OnuIDMgmtLock[intfID].Unlock()

	if err := RsrcMgr.ResourceMgrs[intfID].TechProfileMgr.FreeResourceID(ctx, intfID, ponrmgr.ONU_ID, onuID); err != nil {
		logger.Errorw(ctx, "error-while-freeing-onu-id", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	}

	/* Free onu id for a particular interface.*/
	var IntfonuID string
	for _, onu := range onuID {
		IntfonuID = fmt.Sprintf("%d,%d", intfID, onu)
		RsrcMgr.ResourceMgrs[intfID].RemoveResourceMap(ctx, IntfonuID)
	}
}

// FreeAllocID frees AllocID on the PON resource pool and also frees the allocID association
// for the given OLT device.
func (RsrcMgr *OpenOltResourceMgr) FreeAllocID(ctx context.Context, IntfID uint32, onuID uint32,
	uniID uint32, allocID uint32) {
	RsrcMgr.AllocIDMgmtLock[IntfID].Lock()
	defer RsrcMgr.AllocIDMgmtLock[IntfID].Unlock()

	RsrcMgr.RemoveAllocIDForOnu(ctx, IntfID, onuID, uniID, allocID)
	allocIDs := make([]uint32, 0)
	allocIDs = append(allocIDs, allocID)
	if err := RsrcMgr.ResourceMgrs[IntfID].TechProfileMgr.FreeResourceID(ctx, IntfID, ponrmgr.ALLOC_ID, allocIDs); err != nil {
		logger.Errorw(ctx, "error-while-freeing-alloc-id", log.Fields{
			"intf-id": IntfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	}
}

// FreeGemPortID frees GemPortID on the PON resource pool and also frees the gemPortID association
// for the given OLT device.
func (RsrcMgr *OpenOltResourceMgr) FreeGemPortID(ctx context.Context, IntfID uint32, onuID uint32,
	uniID uint32, gemPortID uint32) {
	RsrcMgr.GemPortIDMgmtLock[IntfID].Lock()
	defer RsrcMgr.GemPortIDMgmtLock[IntfID].Unlock()

	RsrcMgr.RemoveGemPortIDForOnu(ctx, IntfID, onuID, uniID, gemPortID)
	gemPortIDs := make([]uint32, 0)
	gemPortIDs = append(gemPortIDs, gemPortID)
	if err := RsrcMgr.ResourceMgrs[IntfID].TechProfileMgr.FreeResourceID(ctx, IntfID, ponrmgr.GEMPORT_ID, gemPortIDs); err != nil {
		logger.Errorw(ctx, "error-while-freeing-gem-port-id", log.Fields{
			"intf-id": IntfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	}
}

// FreePONResourcesForONU make the pon resources free for a given pon interface and onu id, and the clears the
// resource map and the onuID associated with (pon_intf_id, gemport_id) tuple,
func (RsrcMgr *OpenOltResourceMgr) FreePONResourcesForONU(ctx context.Context, intfID uint32, onuID uint32, uniID uint32) {

	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)

	RsrcMgr.AllocIDMgmtLock[intfID].Lock()
	AllocIDs := RsrcMgr.ResourceMgrs[intfID].GetCurrentAllocIDForOnu(ctx, IntfOnuIDUniID)

	if err := RsrcMgr.ResourceMgrs[intfID].TechProfileMgr.FreeResourceID(ctx, intfID,
		ponrmgr.ALLOC_ID,
		AllocIDs); err != nil {
		logger.Errorw(ctx, "error-while-freeing-all-alloc-ids-for-onu", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	}
	RsrcMgr.AllocIDMgmtLock[intfID].Unlock()

	RsrcMgr.GemPortIDMgmtLock[intfID].Lock()
	GEMPortIDs := RsrcMgr.ResourceMgrs[intfID].GetCurrentGEMPortIDsForOnu(ctx, IntfOnuIDUniID)
	if err := RsrcMgr.ResourceMgrs[intfID].TechProfileMgr.FreeResourceID(ctx, intfID,
		ponrmgr.GEMPORT_ID,
		GEMPortIDs); err != nil {
		logger.Errorw(ctx, "error-while-freeing-all-gem-port-ids-for-onu", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	}
	RsrcMgr.GemPortIDMgmtLock[intfID].Unlock()

	// Clear resource map associated with (pon_intf_id, gemport_id) tuple.
	RsrcMgr.ResourceMgrs[intfID].RemoveResourceMap(ctx, IntfOnuIDUniID)
	// Clear the ONU Id associated with the (pon_intf_id, gemport_id) tuple.
	for _, GEM := range GEMPortIDs {
		_ = RsrcMgr.KVStore.Delete(ctx, fmt.Sprintf("%d,%d", intfID, GEM))
	}
}

// IsFlowOnKvStore checks if the given flowID is present on the kv store
// Returns true if the flowID is found, otherwise it returns false
func (RsrcMgr *OpenOltResourceMgr) IsFlowOnKvStore(ctx context.Context, ponIntfID uint32, onuID int32, uniID int32,
	flowID uint64) bool {

	FlowIDs, err := RsrcMgr.GetCurrentFlowIDsForOnu(ctx, ponIntfID, onuID, uniID)
	if err != nil {
		// error logged in the called function
		return false
	}
	if FlowIDs != nil {
		logger.Debugw(ctx, "Found flowId(s) for this ONU", log.Fields{"pon": ponIntfID, "onuID": onuID, "uniID": uniID})
		for _, id := range FlowIDs {
			if flowID == id {
				return true
			}
		}
	}
	return false
}

// GetTechProfileIDForOnu fetches Tech-Profile-ID from the KV-Store for the given onu based on the path
// This path is formed as the following: {IntfID, OnuID, UniID}/tp_id
func (RsrcMgr *OpenOltResourceMgr) GetTechProfileIDForOnu(ctx context.Context, IntfID uint32, OnuID uint32, UniID uint32) []uint32 {
	Path := fmt.Sprintf(TpIDPathSuffix, IntfID, OnuID, UniID)
	var Data []uint32
	Value, err := RsrcMgr.KVStore.Get(ctx, Path)
	if err == nil {
		if Value != nil {
			Val, err := kvstore.ToByte(Value.Value)
			if err != nil {
				logger.Errorw(ctx, "Failed to convert into byte array", log.Fields{"error": err})
				return Data
			}
			if err = json.Unmarshal(Val, &Data); err != nil {
				logger.Error(ctx, "Failed to unmarshal", log.Fields{"error": err})
				return Data
			}
		}
	} else {
		logger.Errorf(ctx, "Failed to get TP id from kvstore for path %s", Path)
	}
	logger.Debugf(ctx, "Getting TP id %d from path %s", Data, Path)
	return Data

}

// RemoveTechProfileIDsForOnu deletes all tech profile ids from the KV-Store for the given onu based on the path
// This path is formed as the following: {IntfID, OnuID, UniID}/tp_id
func (RsrcMgr *OpenOltResourceMgr) RemoveTechProfileIDsForOnu(ctx context.Context, IntfID uint32, OnuID uint32, UniID uint32) error {
	IntfOnuUniID := fmt.Sprintf(TpIDPathSuffix, IntfID, OnuID, UniID)
	if err := RsrcMgr.KVStore.Delete(ctx, IntfOnuUniID); err != nil {
		logger.Errorw(ctx, "Failed to delete techprofile id resource in KV store", log.Fields{"path": IntfOnuUniID})
		return err
	}
	return nil
}

// RemoveTechProfileIDForOnu deletes a specific tech profile id from the KV-Store for the given onu based on the path
// This path is formed as the following: {IntfID, OnuID, UniID}/tp_id
func (RsrcMgr *OpenOltResourceMgr) RemoveTechProfileIDForOnu(ctx context.Context, IntfID uint32, OnuID uint32, UniID uint32, TpID uint32) error {
	tpIDList := RsrcMgr.GetTechProfileIDForOnu(ctx, IntfID, OnuID, UniID)
	for i, tpIDInList := range tpIDList {
		if tpIDInList == TpID {
			tpIDList = append(tpIDList[:i], tpIDList[i+1:]...)
		}
	}
	IntfOnuUniID := fmt.Sprintf(TpIDPathSuffix, IntfID, OnuID, UniID)
	Value, err := json.Marshal(tpIDList)
	if err != nil {
		logger.Error(ctx, "failed to Marshal")
		return err
	}
	if err = RsrcMgr.KVStore.Put(ctx, IntfOnuUniID, Value); err != nil {
		logger.Errorf(ctx, "Failed to update resource %s", IntfOnuUniID)
		return err
	}
	return err
}

// UpdateTechProfileIDForOnu updates (put) already present tech-profile-id for the given onu based on the path
// This path is formed as the following: {IntfID, OnuID, UniID}/tp_id
func (RsrcMgr *OpenOltResourceMgr) UpdateTechProfileIDForOnu(ctx context.Context, IntfID uint32, OnuID uint32,
	UniID uint32, TpID uint32) error {
	var Value []byte
	var err error

	IntfOnuUniID := fmt.Sprintf(TpIDPathSuffix, IntfID, OnuID, UniID)

	tpIDList := RsrcMgr.GetTechProfileIDForOnu(ctx, IntfID, OnuID, UniID)
	for _, value := range tpIDList {
		if value == TpID {
			logger.Debugf(ctx, "TpID %d is already in tpIdList for the path %s", TpID, IntfOnuUniID)
			return err
		}
	}
	logger.Debugf(ctx, "updating tp id %d on path %s", TpID, IntfOnuUniID)
	tpIDList = append(tpIDList, TpID)
	Value, err = json.Marshal(tpIDList)
	if err != nil {
		logger.Error(ctx, "failed to Marshal")
		return err
	}
	if err = RsrcMgr.KVStore.Put(ctx, IntfOnuUniID, Value); err != nil {
		logger.Errorf(ctx, "Failed to update resource %s", IntfOnuUniID)
		return err
	}
	return err
}

// StoreMeterInfoForOnu updates the meter id in the KV-Store for the given onu based on the path
// This path is formed as the following: <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
func (RsrcMgr *OpenOltResourceMgr) StoreMeterInfoForOnu(ctx context.Context, Direction string, IntfID uint32, OnuID uint32,
	UniID uint32, TpID uint32, meterInfo *MeterInfo) error {
	var Value []byte
	var err error
	IntfOnuUniID := fmt.Sprintf(MeterIDPathSuffix, IntfID, OnuID, UniID, TpID, Direction)
	Value, err = json.Marshal(*meterInfo)
	if err != nil {
		logger.Error(ctx, "failed to Marshal meter config")
		return err
	}
	if err = RsrcMgr.KVStore.Put(ctx, IntfOnuUniID, Value); err != nil {
		logger.Errorf(ctx, "Failed to store meter into KV store %s", IntfOnuUniID)
		return err
	}
	logger.Debugw(ctx, "meter info updated successfully", log.Fields{"path": IntfOnuUniID, "meter-info": meterInfo})
	return err
}

// GetMeterInfoForOnu fetches the meter id from the kv store for the given onu based on the path
// This path is formed as the following: <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
func (RsrcMgr *OpenOltResourceMgr) GetMeterInfoForOnu(ctx context.Context, Direction string, IntfID uint32, OnuID uint32,
	UniID uint32, TpID uint32) (*MeterInfo, error) {
	Path := fmt.Sprintf(MeterIDPathSuffix, IntfID, OnuID, UniID, TpID, Direction)
	var meterInfo MeterInfo
	Value, err := RsrcMgr.KVStore.Get(ctx, Path)
	if err == nil {
		if Value != nil {
			logger.Debug(ctx, "Found meter info in KV store", log.Fields{"Direction": Direction})
			Val, er := kvstore.ToByte(Value.Value)
			if er != nil {
				logger.Errorw(ctx, "Failed to convert into byte array", log.Fields{"error": er})
				return nil, er
			}
			if er = json.Unmarshal(Val, &meterInfo); er != nil {
				logger.Error(ctx, "Failed to unmarshal meter info", log.Fields{"error": er})
				return nil, er
			}
		} else {
			logger.Debug(ctx, "meter-does-not-exists-in-KVStore")
			return nil, err
		}
	} else {
		logger.Errorf(ctx, "Failed to get Meter config from kvstore for path %s", Path)

	}
	return &meterInfo, err
}

// HandleMeterInfoRefCntUpdate increments or decrements the reference counter for a given meter.
// When reference count becomes 0, it clears the meter information from the kv store
func (RsrcMgr *OpenOltResourceMgr) HandleMeterInfoRefCntUpdate(ctx context.Context, Direction string,
	IntfID uint32, OnuID uint32, UniID uint32, TpID uint32, increment bool) error {
	meterInfo, err := RsrcMgr.GetMeterInfoForOnu(ctx, Direction, IntfID, OnuID, UniID, TpID)
	if err != nil {
		return err
	} else if meterInfo == nil {
		// If increasing reference count we expect the meter information to be present on KV store
		// But if decrementing the reference count, the meter is possibly already cleared from KV store. Just log warn but do not return error.
		if increment {
			logger.Errorf(ctx, "error-fetching-meter-info-for-intf-%d-onu-%d-uni-%d-tp-id-%d-direction-%s", IntfID, OnuID, UniID, TpID, Direction)
			return fmt.Errorf("error-fetching-meter-info-for-intf-%d-onu-%d-uni-%d-tp-id-%d-direction-%s", IntfID, OnuID, UniID, TpID, Direction)
		}
		logger.Warnw(ctx, "meter is already cleared",
			log.Fields{"intfID": IntfID, "onuID": OnuID, "uniID": UniID, "direction": Direction, "increment": increment})
		return nil
	}

	if increment {
		meterInfo.RefCnt++
	} else {
		meterInfo.RefCnt--
		// If RefCnt become 0 clear the meter information from the DB.
		if meterInfo.RefCnt == 0 {
			if err := RsrcMgr.RemoveMeterInfoForOnu(ctx, Direction, IntfID, OnuID, UniID, TpID); err != nil {
				return err
			}
			return nil
		}
	}
	if err := RsrcMgr.StoreMeterInfoForOnu(ctx, Direction, IntfID, OnuID, UniID, TpID, meterInfo); err != nil {
		return err
	}
	return nil
}

// RemoveMeterInfoForOnu deletes the meter id from the kV-Store for the given onu based on the path
// This path is formed as the following: <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
func (RsrcMgr *OpenOltResourceMgr) RemoveMeterInfoForOnu(ctx context.Context, Direction string, IntfID uint32, OnuID uint32,
	UniID uint32, TpID uint32) error {
	Path := fmt.Sprintf(MeterIDPathSuffix, IntfID, OnuID, UniID, TpID, Direction)
	if err := RsrcMgr.KVStore.Delete(ctx, Path); err != nil {
		logger.Errorf(ctx, "Failed to delete meter id %s from kvstore ", Path)
		return err
	}
	return nil
}

//AddGemToOnuGemInfo adds gemport to onugem info kvstore
func (RsrcMgr *OpenOltResourceMgr) AddGemToOnuGemInfo(ctx context.Context, intfID uint32, onuID uint32, gemPort uint32) error {
	var onuGemData []OnuGemInfo
	var err error

	if err = RsrcMgr.ResourceMgrs[intfID].GetOnuGemInfo(ctx, intfID, &onuGemData); err != nil {
		logger.Errorf(ctx, "failed to get onuifo for intfid %d", intfID)
		return err
	}
	if len(onuGemData) == 0 {
		logger.Errorw(ctx, "failed to ger Onuid info ", log.Fields{"intfid": intfID, "onuid": onuID})
		return err
	}

	for idx, onugem := range onuGemData {
		if onugem.OnuID == onuID {
			for _, gem := range onuGemData[idx].GemPorts {
				if gem == gemPort {
					logger.Debugw(ctx, "Gem already present in onugem info, skpping addition", log.Fields{"gem": gem})
					return nil
				}
			}
			logger.Debugw(ctx, "Added gem to onugem info", log.Fields{"gem": gemPort})
			onuGemData[idx].GemPorts = append(onuGemData[idx].GemPorts, gemPort)
			break
		}
	}
	err = RsrcMgr.ResourceMgrs[intfID].AddOnuGemInfo(ctx, intfID, onuGemData)
	if err != nil {
		logger.Error(ctx, "Failed to add onugem to kv store")
		return err
	}
	return err
}

//GetOnuGemInfo gets onu gem info from the kvstore per interface
func (RsrcMgr *OpenOltResourceMgr) GetOnuGemInfo(ctx context.Context, IntfID uint32) ([]OnuGemInfo, error) {
	var onuGemData []OnuGemInfo

	if err := RsrcMgr.ResourceMgrs[IntfID].GetOnuGemInfo(ctx, IntfID, &onuGemData); err != nil {
		logger.Errorf(ctx, "failed to get onuifo for intfid %d", IntfID)
		return nil, err
	}

	return onuGemData, nil
}

// AddOnuGemInfo adds onu info on to the kvstore per interface
func (RsrcMgr *OpenOltResourceMgr) AddOnuGemInfo(ctx context.Context, IntfID uint32, onuGem OnuGemInfo) error {
	var onuGemData []OnuGemInfo
	var err error

	if err = RsrcMgr.ResourceMgrs[IntfID].GetOnuGemInfo(ctx, IntfID, &onuGemData); err != nil {
		logger.Errorf(ctx, "failed to get onuifo for intfid %d", IntfID)
		return olterrors.NewErrPersistence("get", "OnuGemInfo", uint64(IntfID),
			log.Fields{"onuGem": onuGem, "intfID": IntfID}, err)
	}
	onuGemData = append(onuGemData, onuGem)
	err = RsrcMgr.ResourceMgrs[IntfID].AddOnuGemInfo(ctx, IntfID, onuGemData)
	if err != nil {
		logger.Error(ctx, "Failed to add onugem to kv store")
		return olterrors.NewErrPersistence("set", "OnuGemInfo", uint64(IntfID),
			log.Fields{"onuGemData": onuGemData, "intfID": IntfID}, err)
	}

	logger.Debugw(ctx, "added onu to onugeminfo", log.Fields{"intf": IntfID, "onugem": onuGem})
	return nil
}

// AddUniPortToOnuInfo adds uni port to the onuinfo kvstore. check if the uni is already present if not update the kv store.
func (RsrcMgr *OpenOltResourceMgr) AddUniPortToOnuInfo(ctx context.Context, intfID uint32, onuID uint32, portNo uint32) {
	var onuGemData []OnuGemInfo
	var err error

	if err = RsrcMgr.ResourceMgrs[intfID].GetOnuGemInfo(ctx, intfID, &onuGemData); err != nil {
		logger.Errorf(ctx, "failed to get onuifo for intfid %d", intfID)
		return
	}
	for idx, onu := range onuGemData {
		if onu.OnuID == onuID {
			for _, uni := range onu.UniPorts {
				if uni == portNo {
					logger.Debugw(ctx, "uni already present in onugem info", log.Fields{"uni": portNo})
					return
				}
			}
			onuGemData[idx].UniPorts = append(onuGemData[idx].UniPorts, portNo)
			break
		}
	}
	err = RsrcMgr.ResourceMgrs[intfID].AddOnuGemInfo(ctx, intfID, onuGemData)
	if err != nil {
		logger.Errorw(ctx, "Failed to add uin port in onugem to kv store", log.Fields{"uni": portNo})
		return
	}
}

//UpdateGemPortForPktIn updates gemport for pkt in path to kvstore, path being intfid, onuid, portno, vlan id, priority bit
func (RsrcMgr *OpenOltResourceMgr) UpdateGemPortForPktIn(ctx context.Context, pktIn PacketInInfoKey, gemPort uint32) {

	path := fmt.Sprintf(OnuPacketINPath, pktIn.IntfID, pktIn.OnuID, pktIn.LogicalPort, pktIn.VlanID, pktIn.Priority)
	Value, err := json.Marshal(gemPort)
	if err != nil {
		logger.Error(ctx, "Failed to marshal data")
		return
	}
	if err = RsrcMgr.KVStore.Put(ctx, path, Value); err != nil {
		logger.Errorw(ctx, "Failed to put to kvstore", log.Fields{"path": path, "value": gemPort})
		return
	}
	logger.Debugw(ctx, "added gem packet in successfully", log.Fields{"path": path, "gem": gemPort})
}

// GetGemPortFromOnuPktIn gets the gem port from onu pkt in path, path being intfid, onuid, portno, vlan id, priority bit
func (RsrcMgr *OpenOltResourceMgr) GetGemPortFromOnuPktIn(ctx context.Context, packetInInfoKey PacketInInfoKey) (uint32, error) {

	var Val []byte
	var gemPort uint32

	path := fmt.Sprintf(OnuPacketINPath, packetInInfoKey.IntfID, packetInInfoKey.OnuID, packetInInfoKey.LogicalPort,
		packetInInfoKey.VlanID, packetInInfoKey.Priority)

	value, err := RsrcMgr.KVStore.Get(ctx, path)
	if err != nil {
		logger.Errorw(ctx, "Failed to get from kv store", log.Fields{"path": path})
		return uint32(0), err
	} else if value == nil {
		logger.Debugw(ctx, "No pkt in gem found", log.Fields{"path": path})
		return uint32(0), nil
	}

	if Val, err = kvstore.ToByte(value.Value); err != nil {
		logger.Error(ctx, "Failed to convert to byte array")
		return uint32(0), err
	}
	if err = json.Unmarshal(Val, &gemPort); err != nil {
		logger.Error(ctx, "Failed to unmarshall")
		return uint32(0), err
	}
	logger.Debugw(ctx, "found packein gemport from path", log.Fields{"path": path, "gem": gemPort})

	return gemPort, nil
}

//DeletePacketInGemPortForOnu deletes the packet-in gemport for ONU
func (RsrcMgr *OpenOltResourceMgr) DeletePacketInGemPortForOnu(ctx context.Context, intfID uint32, onuID uint32, logicalPort uint32) error {

	path := fmt.Sprintf(OnuPacketINPathPrefix, intfID, onuID, logicalPort)
	value, err := RsrcMgr.KVStore.List(ctx, path)
	if err != nil {
		logger.Errorf(ctx, "failed-to-read-value-from-path-%s", path)
		return errors.New("failed-to-read-value-from-path-" + path)
	}

	//remove them one by one
	for key := range value {
		// Formulate the right key path suffix ti be delete
		stringToBeReplaced := fmt.Sprintf(BasePathKvStore, RsrcMgr.KVStore.PathPrefix, RsrcMgr.DeviceID) + "/"
		replacedWith := ""
		key = strings.Replace(key, stringToBeReplaced, replacedWith, 1)

		logger.Debugf(ctx, "removing-key-%s", key)
		if err := RsrcMgr.KVStore.Delete(ctx, key); err != nil {
			logger.Errorf(ctx, "failed-to-remove-resource-%s", key)
			return err
		}
	}

	return nil
}

// DelOnuGemInfoForIntf deletes the onugem info from kvstore per interface
func (RsrcMgr *OpenOltResourceMgr) DelOnuGemInfoForIntf(ctx context.Context, intfID uint32) error {
	if err := RsrcMgr.ResourceMgrs[intfID].DelOnuGemInfoForIntf(ctx, intfID); err != nil {
		logger.Errorw(ctx, "failed to delete onu gem info for", log.Fields{"intfid": intfID})
		return err
	}
	return nil
}

//GetNNIFromKVStore gets NNi intfids from kvstore. path being per device
func (RsrcMgr *OpenOltResourceMgr) GetNNIFromKVStore(ctx context.Context) ([]uint32, error) {

	var nni []uint32
	var Val []byte

	path := NnniIntfID
	value, err := RsrcMgr.KVStore.Get(ctx, path)
	if err != nil {
		logger.Error(ctx, "failed to get data from kv store")
		return nil, err
	}
	if value != nil {
		if Val, err = kvstore.ToByte(value.Value); err != nil {
			logger.Error(ctx, "Failed to convert to byte array")
			return nil, err
		}
		if err = json.Unmarshal(Val, &nni); err != nil {
			logger.Error(ctx, "Failed to unmarshall")
			return nil, err
		}
	}
	return nni, err
}

// AddNNIToKVStore adds Nni interfaces to kvstore, path being per device.
func (RsrcMgr *OpenOltResourceMgr) AddNNIToKVStore(ctx context.Context, nniIntf uint32) error {
	var Value []byte

	nni, err := RsrcMgr.GetNNIFromKVStore(ctx)
	if err != nil {
		logger.Error(ctx, "failed to fetch nni interfaces from kv store")
		return err
	}

	path := NnniIntfID
	nni = append(nni, nniIntf)
	Value, err = json.Marshal(nni)
	if err != nil {
		logger.Error(ctx, "Failed to marshal data")
	}
	if err = RsrcMgr.KVStore.Put(ctx, path, Value); err != nil {
		logger.Errorw(ctx, "Failed to put to kvstore", log.Fields{"path": path, "value": Value})
		return err
	}
	logger.Debugw(ctx, "added nni to kv successfully", log.Fields{"path": path, "nni": nniIntf})
	return nil
}

// DelNNiFromKVStore deletes nni interface list from kv store.
func (RsrcMgr *OpenOltResourceMgr) DelNNiFromKVStore(ctx context.Context) error {

	path := NnniIntfID

	if err := RsrcMgr.KVStore.Delete(ctx, path); err != nil {
		logger.Errorw(ctx, "Failed to delete nni interfaces from kv store", log.Fields{"path": path})
		return err
	}
	return nil
}

//UpdateFlowIDsForGem updates flow id per gemport
func (RsrcMgr *OpenOltResourceMgr) UpdateFlowIDsForGem(ctx context.Context, intf uint32, gem uint32, flowIDs []uint64) error {
	var val []byte
	path := fmt.Sprintf(FlowIDsForGem, intf)

	flowsForGem, err := RsrcMgr.GetFlowIDsGemMapForInterface(ctx, intf)
	if err != nil {
		logger.Error(ctx, "Failed to ger flowids for interface", log.Fields{"error": err, "intf": intf})
		return err
	}
	if flowsForGem == nil {
		flowsForGem = make(map[uint32][]uint64)
	}
	flowsForGem[gem] = flowIDs
	val, err = json.Marshal(flowsForGem)
	if err != nil {
		logger.Error(ctx, "Failed to marshal data", log.Fields{"error": err})
		return err
	}

	if err = RsrcMgr.KVStore.Put(ctx, path, val); err != nil {
		logger.Errorw(ctx, "Failed to put to kvstore", log.Fields{"error": err, "path": path, "value": val})
		return err
	}
	logger.Debugw(ctx, "added flowid list for gem to kv successfully", log.Fields{"path": path, "flowidlist": flowsForGem[gem]})
	return nil
}

//DeleteFlowIDsForGem deletes the flowID list entry per gem from kvstore.
func (RsrcMgr *OpenOltResourceMgr) DeleteFlowIDsForGem(ctx context.Context, intf uint32, gem uint32) {
	path := fmt.Sprintf(FlowIDsForGem, intf)
	var val []byte

	flowsForGem, err := RsrcMgr.GetFlowIDsGemMapForInterface(ctx, intf)
	if err != nil {
		logger.Error(ctx, "Failed to ger flowids for interface", log.Fields{"error": err, "intf": intf})
		return
	}
	if flowsForGem == nil {
		logger.Error(ctx, "No flowids found ", log.Fields{"intf": intf, "gemport": gem})
		return
	}
	// once we get the flows per gem map from kv , just delete the gem entry from the map
	delete(flowsForGem, gem)
	// once gem entry is deleted update the kv store.
	val, err = json.Marshal(flowsForGem)
	if err != nil {
		logger.Error(ctx, "Failed to marshal data", log.Fields{"error": err})
		return
	}

	if err = RsrcMgr.KVStore.Put(ctx, path, val); err != nil {
		logger.Errorw(ctx, "Failed to put to kvstore", log.Fields{"error": err, "path": path, "value": val})
	}
}

//GetFlowIDsGemMapForInterface gets flowids per gemport and interface
func (RsrcMgr *OpenOltResourceMgr) GetFlowIDsGemMapForInterface(ctx context.Context, intf uint32) (map[uint32][]uint64, error) {
	path := fmt.Sprintf(FlowIDsForGem, intf)
	var flowsForGem map[uint32][]uint64
	var val []byte
	value, err := RsrcMgr.KVStore.Get(ctx, path)
	if err != nil {
		logger.Error(ctx, "failed to get data from kv store")
		return nil, err
	}
	if value != nil && value.Value != nil {
		if val, err = kvstore.ToByte(value.Value); err != nil {
			logger.Error(ctx, "Failed to convert to byte array ", log.Fields{"error": err})
			return nil, err
		}
		if err = json.Unmarshal(val, &flowsForGem); err != nil {
			logger.Error(ctx, "Failed to unmarshall", log.Fields{"error": err})
			return nil, err
		}
	}
	return flowsForGem, nil
}

//DeleteIntfIDGempMapPath deletes the intf id path used to store flow ids per gem to kvstore.
func (RsrcMgr *OpenOltResourceMgr) DeleteIntfIDGempMapPath(ctx context.Context, intf uint32) {
	path := fmt.Sprintf(FlowIDsForGem, intf)

	if err := RsrcMgr.KVStore.Delete(ctx, path); err != nil {
		logger.Errorw(ctx, "Failed to delete nni interfaces from kv store", log.Fields{"path": path})
	}
}

// RemoveResourceMap Clear resource map associated with (intfid, onuid, uniid) tuple.
func (RsrcMgr *OpenOltResourceMgr) RemoveResourceMap(ctx context.Context, intfID uint32, onuID int32, uniID int32) {
	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)
	RsrcMgr.ResourceMgrs[intfID].RemoveResourceMap(ctx, IntfOnuIDUniID)
}

//GetMcastQueuePerInterfaceMap gets multicast queue info per pon interface
func (RsrcMgr *OpenOltResourceMgr) GetMcastQueuePerInterfaceMap(ctx context.Context) (map[uint32][]uint32, error) {
	path := McastQueuesForIntf
	var mcastQueueToIntfMap map[uint32][]uint32
	var val []byte

	kvPair, err := RsrcMgr.KVStore.Get(ctx, path)
	if err != nil {
		logger.Error(ctx, "failed to get data from kv store")
		return nil, err
	}
	if kvPair != nil && kvPair.Value != nil {
		if val, err = kvstore.ToByte(kvPair.Value); err != nil {
			logger.Error(ctx, "Failed to convert to byte array ", log.Fields{"error": err})
			return nil, err
		}
		if err = json.Unmarshal(val, &mcastQueueToIntfMap); err != nil {
			logger.Error(ctx, "Failed to unmarshall ", log.Fields{"error": err})
			return nil, err
		}
	}
	return mcastQueueToIntfMap, nil
}

//AddMcastQueueForIntf adds multicast queue for pon interface
func (RsrcMgr *OpenOltResourceMgr) AddMcastQueueForIntf(ctx context.Context, intf uint32, gem uint32, servicePriority uint32) error {
	var val []byte
	path := McastQueuesForIntf

	mcastQueues, err := RsrcMgr.GetMcastQueuePerInterfaceMap(ctx)
	if err != nil {
		logger.Errorw(ctx, "Failed to get multicast queue info for interface", log.Fields{"error": err, "intf": intf})
		return err
	}
	if mcastQueues == nil {
		mcastQueues = make(map[uint32][]uint32)
	}
	mcastQueues[intf] = []uint32{gem, servicePriority}
	if val, err = json.Marshal(mcastQueues); err != nil {
		logger.Errorw(ctx, "Failed to marshal data", log.Fields{"error": err})
		return err
	}
	if err = RsrcMgr.KVStore.Put(ctx, path, val); err != nil {
		logger.Errorw(ctx, "Failed to put to kvstore", log.Fields{"error": err, "path": path, "value": val})
		return err
	}
	logger.Debugw(ctx, "added multicast queue info to KV store successfully", log.Fields{"path": path, "mcastQueueInfo": mcastQueues[intf], "interfaceId": intf})
	return nil
}

//AddFlowGroupToKVStore adds flow group into KV store
func (RsrcMgr *OpenOltResourceMgr) AddFlowGroupToKVStore(ctx context.Context, groupEntry *ofp.OfpGroupEntry, cached bool) error {
	var Value []byte
	var err error
	var path string
	if cached {
		path = fmt.Sprintf(FlowGroupCached, groupEntry.Desc.GroupId)
	} else {
		path = fmt.Sprintf(FlowGroup, groupEntry.Desc.GroupId)
	}
	//build group info object
	var outPorts []uint32
	for _, ofBucket := range groupEntry.Desc.Buckets {
		for _, ofAction := range ofBucket.Actions {
			if ofAction.Type == ofp.OfpActionType_OFPAT_OUTPUT {
				outPorts = append(outPorts, ofAction.GetOutput().Port)
			}
		}
	}
	groupInfo := GroupInfo{
		GroupID:  groupEntry.Desc.GroupId,
		OutPorts: outPorts,
	}

	Value, err = json.Marshal(groupInfo)

	if err != nil {
		logger.Error(ctx, "failed to Marshal flow group object")
		return err
	}

	if err = RsrcMgr.KVStore.Put(ctx, path, Value); err != nil {
		logger.Errorf(ctx, "Failed to update resource %s", path)
		return err
	}
	return nil
}

//RemoveFlowGroupFromKVStore removes flow group from KV store
func (RsrcMgr *OpenOltResourceMgr) RemoveFlowGroupFromKVStore(ctx context.Context, groupID uint32, cached bool) error {
	var path string
	if cached {
		path = fmt.Sprintf(FlowGroupCached, groupID)
	} else {
		path = fmt.Sprintf(FlowGroup, groupID)
	}
	if err := RsrcMgr.KVStore.Delete(ctx, path); err != nil {
		logger.Errorf(ctx, "Failed to remove resource %s due to %s", path, err)
		return err
	}
	return nil
}

//GetFlowGroupFromKVStore fetches flow group from the KV store. Returns (false, {} error) if any problem occurs during
//fetching the data. Returns (true, groupInfo, nil) if the group is fetched successfully.
// Returns (false, {}, nil) if the group does not exists in the KV store.
func (RsrcMgr *OpenOltResourceMgr) GetFlowGroupFromKVStore(ctx context.Context, groupID uint32, cached bool) (bool, GroupInfo, error) {
	var groupInfo GroupInfo
	var path string
	if cached {
		path = fmt.Sprintf(FlowGroupCached, groupID)
	} else {
		path = fmt.Sprintf(FlowGroup, groupID)
	}
	kvPair, err := RsrcMgr.KVStore.Get(ctx, path)
	if err != nil {
		return false, groupInfo, err
	}
	if kvPair != nil && kvPair.Value != nil {
		Val, err := kvstore.ToByte(kvPair.Value)
		if err != nil {
			logger.Errorw(ctx, "Failed to convert flow group into byte array", log.Fields{"error": err})
			return false, groupInfo, err
		}
		if err = json.Unmarshal(Val, &groupInfo); err != nil {
			logger.Errorw(ctx, "Failed to unmarshal", log.Fields{"error": err})
			return false, groupInfo, err
		}
		return true, groupInfo, nil
	}
	return false, groupInfo, nil
}

// toByte converts an interface value to a []byte.  The interface should either be of
// a string type or []byte.  Otherwise, an error is returned.
func toByte(value interface{}) ([]byte, error) {
	switch t := value.(type) {
	case []byte:
		return value.([]byte), nil
	case string:
		return []byte(value.(string)), nil
	default:
		return nil, fmt.Errorf("unexpected-type-%T", t)
	}
}

func checkForFlowIDInList(FlowIDList []uint64, FlowID uint64) (bool, uint64) {
	/*
	   Check for a flow id in a given list of flow IDs.
	   :param FLowIDList: List of Flow IDs
	   :param FlowID: Flowd to check in the list
	   : return true and the index if present false otherwise.
	*/

	for idx := range FlowIDList {
		if FlowID == FlowIDList[idx] {
			return true, uint64(idx)
		}
	}
	return false, 0
}
