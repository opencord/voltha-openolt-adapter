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
	"strings"
	"sync"
	"time"

	tp "github.com/opencord/voltha-lib-go/v7/pkg/techprofile"

	"github.com/opencord/voltha-lib-go/v7/pkg/db"
	"github.com/opencord/voltha-lib-go/v7/pkg/db/kvstore"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	ponrmgr "github.com/opencord/voltha-lib-go/v7/pkg/ponresourcemanager"
	ofp "github.com/opencord/voltha-protos/v5/go/openflow_13"
	"github.com/opencord/voltha-protos/v5/go/openolt"
)

const (
	// KvstoreTimeout specifies the time out for KV Store Connection
	KvstoreTimeout = 5 * time.Second
	// BasePathKvStore - <pathPrefix>/openolt/<device_id>
	BasePathKvStore = "%s/openolt/{%s}"
	// tpIDPathSuffix - <(pon_id, onu_id, uni_id)>/tp_id
	tpIDPathSuffix = "{%d,%d,%d}/tp_id"
	//MeterIDPathSuffix - <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
	MeterIDPathSuffix = "{%d,%d,%d}/{%d}/meter_id/{%s}"

	// OnuPacketInPathPrefix - path prefix where ONU packet-in vlanID/PCP is stored
	//format: onu_packetin/{<intfid>,<onuid>,<logicalport>}
	OnuPacketInPathPrefix = "onu_packetin/{%d,%d,%d}"
	// OnuPacketInPath path on the kvstore to store packetin gemport,which will be used for packetin, packetout
	//format: onu_packetin/{<intfid>,<onuid>,<logicalport>}/{<vlanId>,<priority>}
	OnuPacketInPath = OnuPacketInPathPrefix + "/{%d,%d}"

	//FlowIDsForGemPathPrefix format: flowids_for_gem/<intfid>
	FlowIDsForGemPathPrefix = "flowids_per_gem/{%d}"
	//FlowIDsForGem flowids_for_gem/<intfid>/<gemport-id>
	FlowIDsForGem = FlowIDsForGemPathPrefix + "/{%d}"

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

	//OnuGemInfoPathPathPrefix format: onu_gem_info/<intfid>
	OnuGemInfoPathPathPrefix = "onu_gem_info/{%d}"
	//OnuGemInfoPath is path on the kvstore to store onugem info map
	//format: onu_gem_info/<intfid>/<onu_id>
	OnuGemInfoPath = OnuGemInfoPathPathPrefix + "/{%d}"
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
	RefCnt  uint8 // number of flow references for this meter. When RefCnt is 0, the MeterInfo should be deleted.
	MeterID uint32
}

// OpenOltResourceMgr holds resource related information as provided below for each field
type OpenOltResourceMgr struct {
	PonIntfID  uint32
	DeviceID   string      // OLT device id
	Address    string      // Host and port of the kv store to connect to
	Args       string      // args
	KVStore    *db.Backend // backend kv store connection handle
	DeviceType string
	DevInfo    *openolt.DeviceInfo // device information
	// array of pon resource managers per interface technology
	PonRsrMgr *ponrmgr.PONResourceManager

	// Local maps used for write-through-cache - start
	flowIDsForOnu     map[string][]uint64
	flowIDsForOnuLock sync.RWMutex

	allocIDsForOnu     map[string][]uint32
	allocIDsForOnuLock sync.RWMutex

	gemPortIDsForOnu     map[string][]uint32
	gemPortIDsForOnuLock sync.RWMutex

	techProfileIDsForOnu     map[string][]uint32
	techProfileIDsForOnuLock sync.RWMutex

	meterInfoForOnu     map[string]*MeterInfo
	meterInfoForOnuLock sync.RWMutex

	onuGemInfo     map[string]*OnuGemInfo
	onuGemInfoLock sync.RWMutex

	gemPortForPacketInInfo     map[string]uint32
	gemPortForPacketInInfoLock sync.RWMutex

	flowIDsForGem     map[uint32][]uint64
	flowIDsForGemLock sync.RWMutex

	mcastQueueForIntf                  map[uint32][]uint32
	mcastQueueForIntfLock              sync.RWMutex
	mcastQueueForIntfLoadedFromKvStore bool

	groupInfo     map[string]*GroupInfo
	groupInfoLock sync.RWMutex
	// Local maps used for write-through-cache - end

	TechprofileRef tp.TechProfileIf
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
	// return db.NewBackend(ctx, backend, addr, KvstoreTimeout, fmt.Sprintf(BasePathKvStore, basePathKvStore, DeviceID))

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
func NewResourceMgr(ctx context.Context, PonIntfID uint32, deviceID string, KVStoreAddress string, kvStoreType string, deviceType string, devInfo *openolt.DeviceInfo, basePathKvStore string) *OpenOltResourceMgr {
	var ResourceMgr OpenOltResourceMgr
	logger.Debugf(ctx, "Init new resource manager , ponIf: %v, address: %s, device-id: %s", PonIntfID, KVStoreAddress, deviceID)
	ResourceMgr.PonIntfID = PonIntfID
	ResourceMgr.DeviceID = deviceID
	ResourceMgr.Address = KVStoreAddress
	ResourceMgr.DeviceType = deviceType
	ResourceMgr.DevInfo = devInfo

	Backend := kvStoreType
	ResourceMgr.KVStore = SetKVClient(ctx, Backend, ResourceMgr.Address, deviceID, basePathKvStore)
	if ResourceMgr.KVStore == nil {
		logger.Error(ctx, "Failed to setup KV store")
	}

	// TODO self.args = registry('main').get_args()

	// Create a separate Resource Manager instance for each range. This assumes that
	// each technology is represented by only a single range
	for _, TechRange := range devInfo.Ranges {
		for _, intfID := range TechRange.IntfIds {
			if intfID == PonIntfID {
				technology := TechRange.Technology
				logger.Debugf(ctx, "Device info technology %s, intf-id %v", technology, PonIntfID)

				rsrMgr, err := ponrmgr.NewPONResourceManager(ctx, technology, deviceType, deviceID,
					Backend, ResourceMgr.Address, basePathKvStore)
				if err != nil {
					logger.Errorf(ctx, "Failed to create pon resource manager instance for technology %s", technology)
					return nil
				}
				ResourceMgr.PonRsrMgr = rsrMgr
				// self.initialize_device_resource_range_and_pool(resource_mgr, global_resource_mgr, arange)
				InitializeDeviceResourceRangeAndPool(ctx, rsrMgr, TechRange, devInfo)
				if err := ResourceMgr.PonRsrMgr.InitDeviceResourcePoolForIntf(ctx, intfID); err != nil {
					logger.Fatal(ctx, "failed-to-initialize-device-resource-pool-intf-id-%v-device-id", ResourceMgr.PonIntfID, ResourceMgr.DeviceID)
					return nil
				}
			}
		}
	}

	ResourceMgr.InitLocalCache()

	logger.Info(ctx, "Initialization of  resource manager success!")
	return &ResourceMgr
}

//InitLocalCache initializes local maps used for write-through-cache
func (rsrcMgr *OpenOltResourceMgr) InitLocalCache() {
	rsrcMgr.flowIDsForOnu = make(map[string][]uint64)
	rsrcMgr.allocIDsForOnu = make(map[string][]uint32)
	rsrcMgr.gemPortIDsForOnu = make(map[string][]uint32)
	rsrcMgr.techProfileIDsForOnu = make(map[string][]uint32)
	rsrcMgr.meterInfoForOnu = make(map[string]*MeterInfo)
	rsrcMgr.onuGemInfo = make(map[string]*OnuGemInfo)
	rsrcMgr.gemPortForPacketInInfo = make(map[string]uint32)
	rsrcMgr.flowIDsForGem = make(map[uint32][]uint64)
	rsrcMgr.mcastQueueForIntf = make(map[uint32][]uint32)
	rsrcMgr.groupInfo = make(map[string]*GroupInfo)
}

// InitializeDeviceResourceRangeAndPool initializes the resource range pool according to the sharing type, then apply
// device specific information. If KV doesn't exist
// or is broader than the device, the device's information will
// dictate the range limits
func InitializeDeviceResourceRangeAndPool(ctx context.Context, ponRMgr *ponrmgr.PONResourceManager,
	techRange *openolt.DeviceInfo_DeviceResourceRanges, devInfo *openolt.DeviceInfo) {
	// var ONUIDShared, AllocIDShared, GEMPortIDShared openolt.DeviceInfo_DeviceResourceRanges_Pool_SharingType
	var ONUIDStart, ONUIDEnd, AllocIDStart, AllocIDEnd, GEMPortIDStart, GEMPortIDEnd uint32
	var ONUIDShared, AllocIDShared, GEMPortIDShared, FlowIDShared uint32

	// The below variables are just dummy and needed to pass as arguments to InitDefaultPONResourceRanges function.
	// The openolt adapter does not need flowIDs to be managed as it is managed on the OLT device
	// The UNI IDs are dynamically generated by openonu adapter for every discovered UNI.
	var flowIDDummyStart, flowIDDummyEnd uint32 = 1, 2
	var uniIDDummyStart, uniIDDummyEnd uint32 = 0, 1

	// init the resource range pool according to the sharing type
	logger.Debugw(ctx, "Device info init", log.Fields{"technology": techRange.Technology,
		"onu_id_start": ONUIDStart, "onu_id_end": ONUIDEnd,
		"alloc_id_start": AllocIDStart, "alloc_id_end": AllocIDEnd,
		"gemport_id_start": GEMPortIDStart, "gemport_id_end": GEMPortIDEnd,
		"intf_ids": techRange.IntfIds,
	})
	for _, RangePool := range techRange.Pools {
		// FIXME: Remove hardcoding
		if RangePool.Type == openolt.DeviceInfo_DeviceResourceRanges_Pool_ONU_ID {
			ONUIDStart = RangePool.Start
			ONUIDEnd = RangePool.End
			ONUIDShared = uint32(RangePool.Sharing)
		} else if RangePool.Type == openolt.DeviceInfo_DeviceResourceRanges_Pool_ALLOC_ID {
			AllocIDStart = RangePool.Start
			AllocIDEnd = RangePool.End
			AllocIDShared = uint32(RangePool.Sharing)
		} else if RangePool.Type == openolt.DeviceInfo_DeviceResourceRanges_Pool_GEMPORT_ID {
			GEMPortIDStart = RangePool.Start
			GEMPortIDEnd = RangePool.End
			GEMPortIDShared = uint32(RangePool.Sharing)
		}
	}

	ponRMgr.InitDefaultPONResourceRanges(ctx, ONUIDStart, ONUIDEnd, ONUIDShared,
		AllocIDStart, AllocIDEnd, AllocIDShared,
		GEMPortIDStart, GEMPortIDEnd, GEMPortIDShared,
		flowIDDummyStart, flowIDDummyEnd, FlowIDShared, uniIDDummyStart, uniIDDummyEnd,
		devInfo.PonPorts, techRange.IntfIds)

}

// Delete clears used resources for the particular olt device being deleted
func (rsrcMgr *OpenOltResourceMgr) Delete(ctx context.Context, intfID uint32) error {
	if err := rsrcMgr.PonRsrMgr.ClearDeviceResourcePoolForIntf(ctx, intfID); err != nil {
		logger.Debug(ctx, "Failed to clear device resource pool")
		return err
	}
	logger.Debug(ctx, "Cleared device resource pool")
	return nil
}

// GetONUID returns the available onuID for the given pon-port
func (rsrcMgr *OpenOltResourceMgr) GetONUID(ctx context.Context, PonIntfID uint32) (uint32, error) {
	// Get ONU id for a provided pon interface ID.
	onuID, err := rsrcMgr.TechprofileRef.GetResourceID(ctx, PonIntfID,
		ponrmgr.ONU_ID, 1)
	if err != nil {
		logger.Errorf(ctx, "Failed to get resource for interface %d for type %s",
			PonIntfID, ponrmgr.ONU_ID)
		return 0, err
	}
	if len(onuID) > 0 {
		return onuID[0], err
	}

	return 0, fmt.Errorf("no-onu-id-allocated")
}

// GetCurrentFlowIDsForOnu fetches flow ID from the resource manager
// Note: For flows which trap from the NNI and not really associated with any particular
// ONU (like LLDP), the onu_id and uni_id is set as -1. The intf_id is the NNI intf_id.
func (rsrcMgr *OpenOltResourceMgr) GetCurrentFlowIDsForOnu(ctx context.Context, PonIntfID uint32, onuID int32, uniID int32) ([]uint64, error) {

	subs := fmt.Sprintf("%d,%d,%d", PonIntfID, onuID, uniID)
	path := fmt.Sprintf(FlowIDPath, subs)

	// fetch from cache
	rsrcMgr.flowIDsForOnuLock.RLock()
	flowIDsForOnu, ok := rsrcMgr.flowIDsForOnu[path]
	rsrcMgr.flowIDsForOnuLock.RUnlock()

	if ok {
		return flowIDsForOnu, nil
	}

	var data []uint64
	value, err := rsrcMgr.KVStore.Get(ctx, path)
	if err == nil {
		if value != nil {
			Val, _ := toByte(value.Value)
			if err = json.Unmarshal(Val, &data); err != nil {
				logger.Error(ctx, "Failed to unmarshal")
				return nil, err
			}
		}
	}
	// update cache
	rsrcMgr.flowIDsForOnuLock.Lock()
	rsrcMgr.flowIDsForOnu[path] = data
	rsrcMgr.flowIDsForOnuLock.Unlock()

	return data, nil
}

// UpdateAllocIdsForOnu updates alloc ids in kv store for a given pon interface id, onu id and uni id
func (rsrcMgr *OpenOltResourceMgr) UpdateAllocIdsForOnu(ctx context.Context, ponPort uint32, onuID uint32, uniID uint32, allocIDs []uint32) error {

	intfOnuIDuniID := fmt.Sprintf("%d,%d,%d", ponPort, onuID, uniID)

	// Note: in case the write to DB fails there could be inconsistent data between cache and db.
	// Although this is highly unlikely with DB retries in place, this is something we have to deal with in the next release
	if err := rsrcMgr.PonRsrMgr.UpdateAllocIdsForOnu(ctx, intfOnuIDuniID, allocIDs); err != nil {
		logger.Errorw(ctx, "Failed to update alloc ids for onu", log.Fields{"err": err})
		return err
	}

	// update cache
	rsrcMgr.allocIDsForOnuLock.Lock()
	rsrcMgr.allocIDsForOnu[intfOnuIDuniID] = allocIDs
	rsrcMgr.allocIDsForOnuLock.Unlock()
	return nil
}

// GetCurrentGEMPortIDsForOnu returns gem ports for given pon interface , onu id and uni id
func (rsrcMgr *OpenOltResourceMgr) GetCurrentGEMPortIDsForOnu(ctx context.Context, intfID uint32, onuID uint32,
	uniID uint32) []uint32 {

	intfOnuIDuniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)

	// fetch from cache
	rsrcMgr.gemPortIDsForOnuLock.RLock()
	gemIDs, ok := rsrcMgr.gemPortIDsForOnu[intfOnuIDuniID]
	rsrcMgr.gemPortIDsForOnuLock.RUnlock()
	if ok {
		return gemIDs
	}
	/* Get gem ports for given pon interface , onu id and uni id. */
	gemIDs = rsrcMgr.PonRsrMgr.GetCurrentGEMPortIDsForOnu(ctx, intfOnuIDuniID)

	// update cache
	rsrcMgr.gemPortIDsForOnuLock.Lock()
	rsrcMgr.gemPortIDsForOnu[intfOnuIDuniID] = gemIDs
	rsrcMgr.gemPortIDsForOnuLock.Unlock()

	return gemIDs
}

// GetCurrentAllocIDsForOnu returns alloc ids for given pon interface and onu id
func (rsrcMgr *OpenOltResourceMgr) GetCurrentAllocIDsForOnu(ctx context.Context, intfID uint32, onuID uint32, uniID uint32) []uint32 {

	intfOnuIDuniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)
	// fetch from cache
	rsrcMgr.allocIDsForOnuLock.RLock()
	allocIDs, ok := rsrcMgr.allocIDsForOnu[intfOnuIDuniID]
	rsrcMgr.allocIDsForOnuLock.RUnlock()
	if ok {
		return allocIDs
	}
	allocIDs = rsrcMgr.PonRsrMgr.GetCurrentAllocIDForOnu(ctx, intfOnuIDuniID)

	// update cache
	rsrcMgr.allocIDsForOnuLock.Lock()
	rsrcMgr.allocIDsForOnu[intfOnuIDuniID] = allocIDs
	rsrcMgr.allocIDsForOnuLock.Unlock()

	return allocIDs
}

// RemoveAllocIDForOnu removes the alloc id for given pon interface, onu id, uni id and alloc id
func (rsrcMgr *OpenOltResourceMgr) RemoveAllocIDForOnu(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, allocID uint32) {
	allocIDs := rsrcMgr.GetCurrentAllocIDsForOnu(ctx, intfID, onuID, uniID)
	for i := 0; i < len(allocIDs); i++ {
		if allocIDs[i] == allocID {
			allocIDs = append(allocIDs[:i], allocIDs[i+1:]...)
			break
		}
	}
	err := rsrcMgr.UpdateAllocIdsForOnu(ctx, intfID, onuID, uniID, allocIDs)
	if err != nil {
		logger.Errorf(ctx, "Failed to Remove Alloc Id For Onu. intfID %d onuID %d uniID %d allocID %d",
			intfID, onuID, uniID, allocID)
	}
}

// RemoveGemPortIDForOnu removes the gem port id for given pon interface, onu id, uni id and gem port id
func (rsrcMgr *OpenOltResourceMgr) RemoveGemPortIDForOnu(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, gemPortID uint32) {
	gemPortIDs := rsrcMgr.GetCurrentGEMPortIDsForOnu(ctx, intfID, onuID, uniID)
	for i := 0; i < len(gemPortIDs); i++ {
		if gemPortIDs[i] == gemPortID {
			gemPortIDs = append(gemPortIDs[:i], gemPortIDs[i+1:]...)
			break
		}
	}
	err := rsrcMgr.UpdateGEMPortIDsForOnu(ctx, intfID, onuID, uniID, gemPortIDs)
	if err != nil {
		logger.Errorf(ctx, "Failed to Remove Gem Id For Onu. intfID %d onuID %d uniID %d gemPortId %d",
			intfID, onuID, uniID, gemPortID)
	}
}

// UpdateGEMPortIDsForOnu updates gemport ids on to the kv store for a given pon port, onu id and uni id
func (rsrcMgr *OpenOltResourceMgr) UpdateGEMPortIDsForOnu(ctx context.Context, ponPort uint32, onuID uint32,
	uniID uint32, gemIDs []uint32) error {
	intfOnuIDuniID := fmt.Sprintf("%d,%d,%d", ponPort, onuID, uniID)

	if err := rsrcMgr.PonRsrMgr.UpdateGEMPortIDsForOnu(ctx, intfOnuIDuniID, gemIDs); err != nil {
		logger.Errorw(ctx, "Failed to update gem port ids for onu", log.Fields{"err": err})
		return err
	}

	// update cache
	rsrcMgr.gemPortIDsForOnuLock.Lock()
	rsrcMgr.gemPortIDsForOnu[intfOnuIDuniID] = gemIDs
	rsrcMgr.gemPortIDsForOnuLock.Unlock()
	return nil
}

// FreeonuID releases(make free) onu id for a particular pon-port
func (rsrcMgr *OpenOltResourceMgr) FreeonuID(ctx context.Context, intfID uint32, onuID []uint32) {
	if len(onuID) == 0 {
		logger.Info(ctx, "onu id slice is nil, nothing to free")
		return
	}
	if err := rsrcMgr.TechprofileRef.FreeResourceID(ctx, intfID, ponrmgr.ONU_ID, onuID); err != nil {
		logger.Errorw(ctx, "error-while-freeing-onu-id", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	} else {
		logger.Infow(ctx, "freed onu id", log.Fields{"intfID": intfID, "onuID": onuID})
	}
}

// FreeAllocID frees AllocID on the PON resource pool and also frees the allocID association
// for the given OLT device.
// The caller should ensure that this is a blocking call and this operation is serialized for
// the ONU so as not cause resource corruption since there are no mutexes used here.
func (rsrcMgr *OpenOltResourceMgr) FreeAllocID(ctx context.Context, intfID uint32, onuID uint32,
	uniID uint32, allocID uint32) {

	rsrcMgr.RemoveAllocIDForOnu(ctx, intfID, onuID, uniID, allocID)
	allocIDs := make([]uint32, 0)
	allocIDs = append(allocIDs, allocID)
	if err := rsrcMgr.TechprofileRef.FreeResourceID(ctx, intfID, ponrmgr.ALLOC_ID, allocIDs); err != nil {
		logger.Errorw(ctx, "error-while-freeing-alloc-id", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	}
}

// FreeGemPortID frees GemPortID on the PON resource pool and also frees the gemPortID association
// for the given OLT device.
// The caller should ensure that this is a blocking call and this operation is serialized for
// the ONU so as not cause resource corruption since there are no mutexes used here.
func (rsrcMgr *OpenOltResourceMgr) FreeGemPortID(ctx context.Context, intfID uint32, onuID uint32,
	uniID uint32, gemPortID uint32) {
	rsrcMgr.RemoveGemPortIDForOnu(ctx, intfID, onuID, uniID, gemPortID)

	gemPortIDs := make([]uint32, 0)
	gemPortIDs = append(gemPortIDs, gemPortID)
	if err := rsrcMgr.TechprofileRef.FreeResourceID(ctx, intfID, ponrmgr.GEMPORT_ID, gemPortIDs); err != nil {
		logger.Errorw(ctx, "error-while-freeing-gem-port-id", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	}
}

// FreePONResourcesForONU make the pon resources free for a given pon interface and onu id
func (rsrcMgr *OpenOltResourceMgr) FreePONResourcesForONU(ctx context.Context, intfID uint32, onuID uint32, uniID uint32) {

	intfOnuIDuniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)

	AllocIDs := rsrcMgr.PonRsrMgr.GetCurrentAllocIDForOnu(ctx, intfOnuIDuniID)

	if err := rsrcMgr.TechprofileRef.FreeResourceID(ctx, intfID,
		ponrmgr.ALLOC_ID,
		AllocIDs); err != nil {
		logger.Errorw(ctx, "error-while-freeing-all-alloc-ids-for-onu", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	}

	//update cache
	rsrcMgr.allocIDsForOnuLock.Lock()
	delete(rsrcMgr.allocIDsForOnu, intfOnuIDuniID)
	rsrcMgr.allocIDsForOnuLock.Unlock()

	GEMPortIDs := rsrcMgr.PonRsrMgr.GetCurrentGEMPortIDsForOnu(ctx, intfOnuIDuniID)

	if err := rsrcMgr.TechprofileRef.FreeResourceID(ctx, intfID,
		ponrmgr.GEMPORT_ID,
		GEMPortIDs); err != nil {
		logger.Errorw(ctx, "error-while-freeing-all-gem-port-ids-for-onu", log.Fields{
			"intf-id": intfID,
			"onu-id":  onuID,
			"err":     err.Error(),
		})
	}

	// update cache
	rsrcMgr.gemPortIDsForOnuLock.Lock()
	delete(rsrcMgr.gemPortIDsForOnu, intfOnuIDuniID)
	rsrcMgr.gemPortIDsForOnuLock.Unlock()

	// Clear resource map associated with (pon_intf_id, gemport_id) tuple.
	rsrcMgr.PonRsrMgr.RemoveResourceMap(ctx, intfOnuIDuniID)
}

// IsFlowOnKvStore checks if the given flowID is present on the kv store
// Returns true if the flowID is found, otherwise it returns false
func (rsrcMgr *OpenOltResourceMgr) IsFlowOnKvStore(ctx context.Context, intfID uint32, onuID int32, uniID int32,
	flowID uint64) bool {

	FlowIDs, err := rsrcMgr.GetCurrentFlowIDsForOnu(ctx, intfID, onuID, uniID)
	if err != nil {
		// error logged in the called function
		return false
	}
	if FlowIDs != nil {
		logger.Debugw(ctx, "Found flowId(s) for this ONU", log.Fields{"pon": intfID, "onuID": onuID, "uniID": uniID})
		for _, id := range FlowIDs {
			if flowID == id {
				return true
			}
		}
	}
	return false
}

// GetTechProfileIDForOnu fetches Tech-Profile-ID from the KV-Store for the given onu based on the path
// This path is formed as the following: {intfID, onuID, uniID}/tp_id
func (rsrcMgr *OpenOltResourceMgr) GetTechProfileIDForOnu(ctx context.Context, intfID uint32, onuID uint32, uniID uint32) []uint32 {
	Path := fmt.Sprintf(tpIDPathSuffix, intfID, onuID, uniID)
	// fetch from cache
	rsrcMgr.techProfileIDsForOnuLock.RLock()
	tpIDs, ok := rsrcMgr.techProfileIDsForOnu[Path]
	rsrcMgr.techProfileIDsForOnuLock.RUnlock()
	if ok {
		return tpIDs
	}
	Value, err := rsrcMgr.KVStore.Get(ctx, Path)
	if err == nil {
		if Value != nil {
			Val, err := kvstore.ToByte(Value.Value)
			if err != nil {
				logger.Errorw(ctx, "Failed to convert into byte array", log.Fields{"err": err})
				return tpIDs
			}
			if err = json.Unmarshal(Val, &tpIDs); err != nil {
				logger.Error(ctx, "Failed to unmarshal", log.Fields{"err": err})
				return tpIDs
			}
		}
	} else {
		logger.Errorf(ctx, "Failed to get TP id from kvstore for path %s", Path)
	}
	logger.Debugf(ctx, "Getting TP id %d from path %s", tpIDs, Path)

	// update cache
	rsrcMgr.techProfileIDsForOnuLock.Lock()
	rsrcMgr.techProfileIDsForOnu[Path] = tpIDs
	rsrcMgr.techProfileIDsForOnuLock.Unlock()

	return tpIDs

}

// RemoveTechProfileIDsForOnu deletes all tech profile ids from the KV-Store for the given onu based on the path
// This path is formed as the following: {intfID, onuID, uniID}/tp_id
func (rsrcMgr *OpenOltResourceMgr) RemoveTechProfileIDsForOnu(ctx context.Context, intfID uint32, onuID uint32, uniID uint32) error {
	intfOnuUniID := fmt.Sprintf(tpIDPathSuffix, intfID, onuID, uniID)

	if err := rsrcMgr.KVStore.Delete(ctx, intfOnuUniID); err != nil {
		logger.Errorw(ctx, "Failed to delete techprofile id resource in KV store", log.Fields{"path": intfOnuUniID})
		return err
	}

	// update cache
	rsrcMgr.techProfileIDsForOnuLock.Lock()
	delete(rsrcMgr.techProfileIDsForOnu, intfOnuUniID)
	rsrcMgr.techProfileIDsForOnuLock.Unlock()
	return nil
}

// RemoveTechProfileIDForOnu deletes a specific tech profile id from the KV-Store for the given onu based on the path
// This path is formed as the following: {intfID, onuID, uniID}/tp_id
func (rsrcMgr *OpenOltResourceMgr) RemoveTechProfileIDForOnu(ctx context.Context, intfID uint32, onuID uint32, uniID uint32, tpID uint32) error {
	tpIDList := rsrcMgr.GetTechProfileIDForOnu(ctx, intfID, onuID, uniID)
	for i, tpIDInList := range tpIDList {
		if tpIDInList == tpID {
			tpIDList = append(tpIDList[:i], tpIDList[i+1:]...)
		}
	}
	intfOnuUniID := fmt.Sprintf(tpIDPathSuffix, intfID, onuID, uniID)

	Value, err := json.Marshal(tpIDList)
	if err != nil {
		logger.Error(ctx, "failed to Marshal")
		return err
	}
	if err = rsrcMgr.KVStore.Put(ctx, intfOnuUniID, Value); err != nil {
		logger.Errorf(ctx, "Failed to update resource %s", intfOnuUniID)
		return err
	}

	// update cache
	rsrcMgr.techProfileIDsForOnuLock.Lock()
	rsrcMgr.techProfileIDsForOnu[intfOnuUniID] = tpIDList
	rsrcMgr.techProfileIDsForOnuLock.Unlock()
	return err
}

// UpdateTechProfileIDForOnu updates (put) already present tech-profile-id for the given onu based on the path
// This path is formed as the following: {intfID, onuID, uniID}/tp_id
func (rsrcMgr *OpenOltResourceMgr) UpdateTechProfileIDForOnu(ctx context.Context, intfID uint32, onuID uint32,
	uniID uint32, tpID uint32) error {
	var Value []byte
	var err error

	intfOnuUniID := fmt.Sprintf(tpIDPathSuffix, intfID, onuID, uniID)

	tpIDList := rsrcMgr.GetTechProfileIDForOnu(ctx, intfID, onuID, uniID)
	for _, value := range tpIDList {
		if value == tpID {
			logger.Debugf(ctx, "tpID %d is already in tpIdList for the path %s", tpID, intfOnuUniID)
			return err
		}
	}
	logger.Debugf(ctx, "updating tp id %d on path %s", tpID, intfOnuUniID)
	tpIDList = append(tpIDList, tpID)

	Value, err = json.Marshal(tpIDList)
	if err != nil {
		logger.Error(ctx, "failed to Marshal")
		return err
	}
	if err = rsrcMgr.KVStore.Put(ctx, intfOnuUniID, Value); err != nil {
		logger.Errorf(ctx, "Failed to update resource %s", intfOnuUniID)
		return err
	}

	// update cache
	rsrcMgr.techProfileIDsForOnuLock.Lock()
	rsrcMgr.techProfileIDsForOnu[intfOnuUniID] = tpIDList
	rsrcMgr.techProfileIDsForOnuLock.Unlock()
	return err
}

// StoreMeterInfoForOnu updates the meter id in the KV-Store for the given onu based on the path
// This path is formed as the following: <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
func (rsrcMgr *OpenOltResourceMgr) StoreMeterInfoForOnu(ctx context.Context, Direction string, intfID uint32, onuID uint32,
	uniID uint32, tpID uint32, meterInfo *MeterInfo) error {
	var Value []byte
	var err error
	intfOnuUniID := fmt.Sprintf(MeterIDPathSuffix, intfID, onuID, uniID, tpID, Direction)

	Value, err = json.Marshal(*meterInfo)
	if err != nil {
		logger.Error(ctx, "failed to Marshal meter config")
		return err
	}
	if err = rsrcMgr.KVStore.Put(ctx, intfOnuUniID, Value); err != nil {
		logger.Errorf(ctx, "Failed to store meter into KV store %s", intfOnuUniID)
		return err
	}

	// update cache
	rsrcMgr.meterInfoForOnuLock.Lock()
	rsrcMgr.meterInfoForOnu[intfOnuUniID] = meterInfo
	rsrcMgr.meterInfoForOnuLock.Unlock()
	logger.Debugw(ctx, "meter info updated successfully", log.Fields{"path": intfOnuUniID, "meter-info": meterInfo})
	return err
}

// GetMeterInfoForOnu fetches the meter id from the kv store for the given onu based on the path
// This path is formed as the following: <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
func (rsrcMgr *OpenOltResourceMgr) GetMeterInfoForOnu(ctx context.Context, Direction string, intfID uint32, onuID uint32,
	uniID uint32, tpID uint32) (*MeterInfo, error) {
	Path := fmt.Sprintf(MeterIDPathSuffix, intfID, onuID, uniID, tpID, Direction)

	// get from cache
	rsrcMgr.meterInfoForOnuLock.RLock()
	val, ok := rsrcMgr.meterInfoForOnu[Path]
	rsrcMgr.meterInfoForOnuLock.RUnlock()
	if ok {
		return val, nil
	}

	var meterInfo MeterInfo
	Value, err := rsrcMgr.KVStore.Get(ctx, Path)
	if err == nil {
		if Value != nil {
			logger.Debug(ctx, "Found meter info in KV store", log.Fields{"Direction": Direction})
			Val, er := kvstore.ToByte(Value.Value)
			if er != nil {
				logger.Errorw(ctx, "Failed to convert into byte array", log.Fields{"err": er})
				return nil, er
			}
			if er = json.Unmarshal(Val, &meterInfo); er != nil {
				logger.Error(ctx, "Failed to unmarshal meter info", log.Fields{"err": er})
				return nil, er
			}
		} else {
			logger.Debug(ctx, "meter-does-not-exists-in-KVStore")
			return nil, err
		}
	} else {
		logger.Errorf(ctx, "Failed to get Meter config from kvstore for path %s", Path)

	}
	// update cache
	rsrcMgr.meterInfoForOnuLock.Lock()
	rsrcMgr.meterInfoForOnu[Path] = &meterInfo
	rsrcMgr.meterInfoForOnuLock.Unlock()

	return &meterInfo, err
}

// HandleMeterInfoRefCntUpdate increments or decrements the reference counter for a given meter.
// When reference count becomes 0, it clears the meter information from the kv store
func (rsrcMgr *OpenOltResourceMgr) HandleMeterInfoRefCntUpdate(ctx context.Context, Direction string,
	intfID uint32, onuID uint32, uniID uint32, tpID uint32, increment bool) error {
	meterInfo, err := rsrcMgr.GetMeterInfoForOnu(ctx, Direction, intfID, onuID, uniID, tpID)
	if err != nil {
		return err
	} else if meterInfo == nil {
		// If we are increasing the reference count, we expect the meter information to be present on KV store.
		// But if decrementing the reference count, the meter is possibly already cleared from KV store. Just log warn but do not return error.
		if increment {
			logger.Errorf(ctx, "error-fetching-meter-info-for-intf-%d-onu-%d-uni-%d-tp-id-%d-direction-%s", intfID, onuID, uniID, tpID, Direction)
			return fmt.Errorf("error-fetching-meter-info-for-intf-%d-onu-%d-uni-%d-tp-id-%d-direction-%s", intfID, onuID, uniID, tpID, Direction)
		}
		logger.Warnw(ctx, "meter is already cleared",
			log.Fields{"intfID": intfID, "onuID": onuID, "uniID": uniID, "direction": Direction, "increment": increment})
		return nil
	}

	if increment {
		meterInfo.RefCnt++
	} else {
		meterInfo.RefCnt--
	}
	if err := rsrcMgr.StoreMeterInfoForOnu(ctx, Direction, intfID, onuID, uniID, tpID, meterInfo); err != nil {
		return err
	}
	return nil
}

// RemoveMeterInfoForOnu deletes the meter id from the kV-Store for the given onu based on the path
// This path is formed as the following: <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
func (rsrcMgr *OpenOltResourceMgr) RemoveMeterInfoForOnu(ctx context.Context, Direction string, intfID uint32, onuID uint32,
	uniID uint32, tpID uint32) error {
	Path := fmt.Sprintf(MeterIDPathSuffix, intfID, onuID, uniID, tpID, Direction)

	if err := rsrcMgr.KVStore.Delete(ctx, Path); err != nil {
		logger.Errorf(ctx, "Failed to delete meter id %s from kvstore ", Path)
		return err
	}

	// update cache
	rsrcMgr.meterInfoForOnuLock.Lock()
	delete(rsrcMgr.meterInfoForOnu, Path)
	rsrcMgr.meterInfoForOnuLock.Unlock()
	return nil
}

//AddGemToOnuGemInfo adds gemport to onugem info kvstore and also local cache
func (rsrcMgr *OpenOltResourceMgr) AddGemToOnuGemInfo(ctx context.Context, intfID uint32, onuID uint32, gemPort uint32) error {
	onugem, err := rsrcMgr.GetOnuGemInfo(ctx, intfID, onuID)
	if err != nil || onugem == nil || onugem.SerialNumber == "" {
		logger.Errorf(ctx, "failed to get onuifo for intfid %d", intfID)
		return err
	}
	if onugem.OnuID == onuID {
		for _, gem := range onugem.GemPorts {
			if gem == gemPort {
				logger.Debugw(ctx, "Gem already present in onugem info, skpping addition", log.Fields{"gem": gem})
				return nil
			}
		}
		logger.Debugw(ctx, "Added gem to onugem info", log.Fields{"gem": gemPort})
		onugem.GemPorts = append(onugem.GemPorts, gemPort)
	} else {
		logger.Errorw(ctx, "onu id in OnuGemInfo does not match", log.Fields{"onuID": onuID, "ponIf": intfID, "onuGemInfoOnuID": onugem.OnuID})
		return fmt.Errorf("onu-id-in-OnuGemInfo-does-not-match-%v", onuID)
	}

	err = rsrcMgr.AddOnuGemInfo(ctx, intfID, onuID, *onugem)
	if err != nil {
		logger.Error(ctx, "Failed to add onugem to kv store")
		return err
	}
	return err
}

//RemoveGemFromOnuGemInfo removes gemport from onugem info on kvstore and also local cache
func (rsrcMgr *OpenOltResourceMgr) RemoveGemFromOnuGemInfo(ctx context.Context, intfID uint32, onuID uint32, gemPort uint32) error {
	onugem, err := rsrcMgr.GetOnuGemInfo(ctx, intfID, onuID)
	if err != nil || onugem == nil || onugem.SerialNumber == "" {
		logger.Errorf(ctx, "failed to get onuifo for intfid %d", intfID)
		return err
	}
	updated := false
	if onugem.OnuID == onuID {
		for i, gem := range onugem.GemPorts {
			if gem == gemPort {
				logger.Debugw(ctx, "Gem found, removing from onu gem info", log.Fields{"gem": gem})
				onugem.GemPorts = append(onugem.GemPorts[:i], onugem.GemPorts[i+1:]...)
				updated = true
				break
			}
		}
	} else {
		logger.Errorw(ctx, "onu id in OnuGemInfo does not match", log.Fields{"onuID": onuID, "ponIf": intfID, "onuGemInfoOnuID": onugem.OnuID})
		return fmt.Errorf("onu-id-in-OnuGemInfo-does-not-match-%v", onuID)
	}
	if updated {
		err = rsrcMgr.AddOnuGemInfo(ctx, intfID, onuID, *onugem)
		if err != nil {
			logger.Error(ctx, "Failed to add onugem to kv store")
			return err
		}
	} else {
		logger.Debugw(ctx, "Gem port not found in onu gem info", log.Fields{"gem": gemPort})
	}
	return nil
}

//GetOnuGemInfo gets onu gem info from the kvstore per interface
func (rsrcMgr *OpenOltResourceMgr) GetOnuGemInfo(ctx context.Context, intfID uint32, onuID uint32) (*OnuGemInfo, error) {
	var err error
	var Val []byte
	var onugem OnuGemInfo

	path := fmt.Sprintf(OnuGemInfoPath, intfID, onuID)

	rsrcMgr.onuGemInfoLock.RLock()
	val, ok := rsrcMgr.onuGemInfo[path]
	rsrcMgr.onuGemInfoLock.RUnlock()
	if ok {
		return val, nil
	}
	value, err := rsrcMgr.KVStore.Get(ctx, path)
	if err != nil {
		logger.Errorw(ctx, "Failed to get from kv store", log.Fields{"path": path})
		return nil, err
	} else if value == nil {
		logger.Debug(ctx, "No onuinfo for path", log.Fields{"path": path})
		return nil, nil // returning nil as this could happen if there are no onus for the interface yet
	}
	if Val, err = kvstore.ToByte(value.Value); err != nil {
		logger.Error(ctx, "Failed to convert to byte array")
		return nil, err
	}

	if err = json.Unmarshal(Val, &onugem); err != nil {
		logger.Error(ctx, "Failed to unmarshall")
		return nil, err
	}
	logger.Debugw(ctx, "found onugem info from path", log.Fields{"path": path, "onuGemInfo": onugem})
	rsrcMgr.onuGemInfoLock.Lock()
	rsrcMgr.onuGemInfo[path] = &onugem
	rsrcMgr.onuGemInfoLock.Unlock()

	return &onugem, nil
}

// AddOnuGemInfo adds onu info on to the kvstore per interface
func (rsrcMgr *OpenOltResourceMgr) AddOnuGemInfo(ctx context.Context, intfID uint32, onuID uint32, onuGem OnuGemInfo) error {

	var Value []byte
	var err error
	Path := fmt.Sprintf(OnuGemInfoPath, intfID, onuID)

	Value, err = json.Marshal(onuGem)
	if err != nil {
		logger.Error(ctx, "failed to Marshal")
		return err
	}

	if err = rsrcMgr.KVStore.Put(ctx, Path, Value); err != nil {
		logger.Errorf(ctx, "Failed to update resource %s", Path)
		return err
	}
	logger.Debugw(ctx, "added onu gem info to store", log.Fields{"onuGemInfo": onuGem})

	//update cache
	rsrcMgr.onuGemInfoLock.Lock()
	rsrcMgr.onuGemInfo[Path] = &onuGem
	rsrcMgr.onuGemInfoLock.Unlock()
	return err
}

// DelOnuGemInfo deletes the onugem info from kvstore per ONU
func (rsrcMgr *OpenOltResourceMgr) DelOnuGemInfo(ctx context.Context, intfID uint32, onuID uint32) error {
	path := fmt.Sprintf(OnuGemInfoPath, intfID, onuID)

	if err := rsrcMgr.KVStore.Delete(ctx, path); err != nil {
		logger.Errorf(ctx, "failed to remove resource %s", path)
		return err
	}

	//update cache
	rsrcMgr.onuGemInfoLock.Lock()
	logger.Debugw(ctx, "removing onu gem info", log.Fields{"onuGemInfo": rsrcMgr.onuGemInfo[path]})
	delete(rsrcMgr.onuGemInfo, path)
	rsrcMgr.onuGemInfoLock.Unlock()
	return nil
}

//DeleteAllOnuGemInfoForIntf deletes all the all onu gem info on the given pon interface
func (rsrcMgr *OpenOltResourceMgr) DeleteAllOnuGemInfoForIntf(ctx context.Context, intfID uint32) error {

	path := fmt.Sprintf(OnuGemInfoPathPathPrefix, intfID)

	logger.Debugw(ctx, "delete-all-onu-gem-info-for-pon-intf", log.Fields{"intfID": intfID})
	if err := rsrcMgr.KVStore.DeleteWithPrefix(ctx, path); err != nil {
		logger.Errorf(ctx, "failed-to-remove-resource-%s", path)
		return err
	}

	// Reset cache. Normally not necessary as the entire device is getting deleted when this API is invoked.
	rsrcMgr.onuGemInfoLock.Lock()
	rsrcMgr.onuGemInfo = make(map[string]*OnuGemInfo)
	rsrcMgr.onuGemInfoLock.Unlock()
	return nil
}

// AddUniPortToOnuInfo adds uni port to the onuinfo kvstore. check if the uni is already present if not update the kv store.
func (rsrcMgr *OpenOltResourceMgr) AddUniPortToOnuInfo(ctx context.Context, intfID uint32, onuID uint32, portNo uint32) {

	onugem, err := rsrcMgr.GetOnuGemInfo(ctx, intfID, onuID)
	if err != nil || onugem == nil || onugem.SerialNumber == "" {
		logger.Warnf(ctx, "failed to get onuifo for intfid %d", intfID)
		return
	}

	if onugem.OnuID == onuID {
		for _, uni := range onugem.UniPorts {
			if uni == portNo {
				logger.Debugw(ctx, "uni already present in onugem info", log.Fields{"uni": portNo})
				return
			}
		}
		onugem.UniPorts = append(onugem.UniPorts, portNo)
	} else {
		logger.Warnw(ctx, "onu id mismatch in onu gem info", log.Fields{"intfID": intfID, "onuID": onuID})
		return
	}
	err = rsrcMgr.AddOnuGemInfo(ctx, intfID, onuID, *onugem)
	if err != nil {
		logger.Errorw(ctx, "Failed to add uni port in onugem to kv store", log.Fields{"uni": portNo})
		return
	}
}

//UpdateGemPortForPktIn updates gemport for pkt in path to kvstore, path being intfid, onuid, portno, vlan id, priority bit
func (rsrcMgr *OpenOltResourceMgr) UpdateGemPortForPktIn(ctx context.Context, pktIn PacketInInfoKey, gemPort uint32) {

	path := fmt.Sprintf(OnuPacketInPath, pktIn.IntfID, pktIn.OnuID, pktIn.LogicalPort, pktIn.VlanID, pktIn.Priority)

	Value, err := json.Marshal(gemPort)
	if err != nil {
		logger.Error(ctx, "Failed to marshal data")
		return
	}
	if err = rsrcMgr.KVStore.Put(ctx, path, Value); err != nil {
		logger.Errorw(ctx, "Failed to put to kvstore", log.Fields{"path": path, "value": gemPort})
		return
	}

	// update cache
	rsrcMgr.gemPortForPacketInInfoLock.Lock()
	rsrcMgr.gemPortForPacketInInfo[path] = gemPort
	rsrcMgr.gemPortForPacketInInfoLock.Unlock()
	logger.Debugw(ctx, "added gem packet in successfully", log.Fields{"path": path, "gem": gemPort})
}

// GetGemPortFromOnuPktIn gets the gem port from onu pkt in path, path being intfid, onuid, portno, vlan id, priority bit
func (rsrcMgr *OpenOltResourceMgr) GetGemPortFromOnuPktIn(ctx context.Context, packetInInfoKey PacketInInfoKey) (uint32, error) {

	var Val []byte

	path := fmt.Sprintf(OnuPacketInPath, packetInInfoKey.IntfID, packetInInfoKey.OnuID, packetInInfoKey.LogicalPort,
		packetInInfoKey.VlanID, packetInInfoKey.Priority)
	// get from cache
	rsrcMgr.gemPortForPacketInInfoLock.RLock()
	gemPort, ok := rsrcMgr.gemPortForPacketInInfo[path]
	rsrcMgr.gemPortForPacketInInfoLock.RUnlock()
	if ok {
		logger.Debugw(ctx, "found packein gemport from path", log.Fields{"path": path, "gem": gemPort})
		return gemPort, nil
	}

	value, err := rsrcMgr.KVStore.Get(ctx, path)
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
	// update cache
	rsrcMgr.gemPortForPacketInInfoLock.Lock()
	rsrcMgr.gemPortForPacketInInfo[path] = gemPort
	rsrcMgr.gemPortForPacketInInfoLock.Unlock()

	return gemPort, nil
}

//DeletePacketInGemPortForOnu deletes the packet-in gemport for ONU
func (rsrcMgr *OpenOltResourceMgr) DeletePacketInGemPortForOnu(ctx context.Context, intfID uint32, onuID uint32, logicalPort uint32) error {
	path := fmt.Sprintf(OnuPacketInPathPrefix, intfID, onuID, logicalPort)
	value, err := rsrcMgr.KVStore.List(ctx, path)
	if err != nil {
		logger.Errorf(ctx, "failed-to-read-value-from-path-%s", path)
		return errors.New("failed-to-read-value-from-path-" + path)
	}

	logger.Debugw(ctx, "delete-packetin-gem-port", log.Fields{"realPath": path})
	if err := rsrcMgr.KVStore.DeleteWithPrefix(ctx, path); err != nil {
		logger.Errorf(ctx, "failed-to-remove-resource-%s", path)
		return err
	}

	//remove them one by one
	for key := range value {
		// Remove the PathPrefix from the path on KV key.
		// gemPortForPacketInInfo cache uses OnuPacketInPath as the key
		stringToBeReplaced := rsrcMgr.KVStore.PathPrefix + "/"
		replacedWith := ""
		key = strings.Replace(key, stringToBeReplaced, replacedWith, 1)
		// update cache
		rsrcMgr.gemPortForPacketInInfoLock.Lock()
		delete(rsrcMgr.gemPortForPacketInInfo, key)
		rsrcMgr.gemPortForPacketInInfoLock.Unlock()

		logger.Debugw(ctx, "removed-key-from-packetin-gem-port-cache", log.Fields{"key": key, "cache-len": len(rsrcMgr.gemPortForPacketInInfo)})
	}

	return nil
}

//GetFlowIDsForGem gets the list of FlowIDs for the given gemport
func (rsrcMgr *OpenOltResourceMgr) GetFlowIDsForGem(ctx context.Context, intf uint32, gem uint32) ([]uint64, error) {
	path := fmt.Sprintf(FlowIDsForGem, intf, gem)

	// get from cache
	rsrcMgr.flowIDsForGemLock.RLock()
	flowIDs, ok := rsrcMgr.flowIDsForGem[gem]
	rsrcMgr.flowIDsForGemLock.RUnlock()
	if ok {
		return flowIDs, nil
	}

	value, err := rsrcMgr.KVStore.Get(ctx, path)
	if err != nil {
		logger.Errorw(ctx, "Failed to get from kv store", log.Fields{"path": path})
		return nil, err
	} else if value == nil {
		logger.Debug(ctx, "no flow-ids found", log.Fields{"path": path})
		return nil, nil
	}
	Val, err := kvstore.ToByte(value.Value)
	if err != nil {
		logger.Error(ctx, "Failed to convert to byte array")
		return nil, err
	}

	if err = json.Unmarshal(Val, &flowIDs); err != nil {
		logger.Error(ctx, "Failed to unmarshall")
		return nil, err
	}

	// update cache
	rsrcMgr.flowIDsForGemLock.Lock()
	rsrcMgr.flowIDsForGem[gem] = flowIDs
	rsrcMgr.flowIDsForGemLock.Unlock()

	return flowIDs, nil
}

//UpdateFlowIDsForGem updates flow id per gemport
func (rsrcMgr *OpenOltResourceMgr) UpdateFlowIDsForGem(ctx context.Context, intf uint32, gem uint32, flowIDs []uint64) error {
	var val []byte
	path := fmt.Sprintf(FlowIDsForGem, intf, gem)

	if flowIDs == nil {
		if err := rsrcMgr.KVStore.Delete(ctx, path); err != nil {
			logger.Errorw(ctx, "Failed to delete from kvstore", log.Fields{"err": err, "path": path})
		}
		return nil
	}
	val, err := json.Marshal(flowIDs)
	if err != nil {
		logger.Error(ctx, "Failed to marshal data", log.Fields{"err": err})
		return err
	}

	if err = rsrcMgr.KVStore.Put(ctx, path, val); err != nil {
		logger.Errorw(ctx, "Failed to put to kvstore", log.Fields{"err": err, "path": path, "value": val})
		return err
	}
	logger.Debugw(ctx, "added flowid list for gem to kv successfully", log.Fields{"path": path, "flowidlist": flowIDs})

	// update cache
	rsrcMgr.flowIDsForGemLock.Lock()
	rsrcMgr.flowIDsForGem[gem] = flowIDs
	rsrcMgr.flowIDsForGemLock.Unlock()
	return nil
}

//DeleteFlowIDsForGem deletes the flowID list entry per gem from kvstore.
func (rsrcMgr *OpenOltResourceMgr) DeleteFlowIDsForGem(ctx context.Context, intf uint32, gem uint32) error {
	path := fmt.Sprintf(FlowIDsForGem, intf, gem)
	if err := rsrcMgr.KVStore.Delete(ctx, path); err != nil {
		logger.Errorw(ctx, "Failed to delete from kvstore", log.Fields{"err": err, "path": path})
		return err
	}
	// update cache
	rsrcMgr.flowIDsForGemLock.Lock()
	delete(rsrcMgr.flowIDsForGem, gem)
	rsrcMgr.flowIDsForGemLock.Unlock()
	return nil
}

//DeleteAllFlowIDsForGemForIntf deletes all the flow ids associated for all the gems on the given pon interface
func (rsrcMgr *OpenOltResourceMgr) DeleteAllFlowIDsForGemForIntf(ctx context.Context, intfID uint32) error {

	path := fmt.Sprintf(FlowIDsForGemPathPrefix, intfID)

	logger.Debugw(ctx, "delete-flow-ids-for-gem-for-pon-intf", log.Fields{"intfID": intfID})
	if err := rsrcMgr.KVStore.DeleteWithPrefix(ctx, path); err != nil {
		logger.Errorf(ctx, "failed-to-remove-resource-%s", path)
		return err
	}

	// Reset cache. Normally not necessary as the entire device is getting deleted when this API is invoked.
	rsrcMgr.flowIDsForGemLock.Lock()
	rsrcMgr.flowIDsForGem = make(map[uint32][]uint64)
	rsrcMgr.flowIDsForGemLock.Unlock()
	return nil
}

//GetMcastQueuePerInterfaceMap gets multicast queue info per pon interface
func (rsrcMgr *OpenOltResourceMgr) GetMcastQueuePerInterfaceMap(ctx context.Context) (map[uint32][]uint32, error) {
	path := McastQueuesForIntf
	var val []byte

	rsrcMgr.mcastQueueForIntfLock.RLock()
	if rsrcMgr.mcastQueueForIntfLoadedFromKvStore {
		rsrcMgr.mcastQueueForIntfLock.RUnlock()
		return rsrcMgr.mcastQueueForIntf, nil
	}
	rsrcMgr.mcastQueueForIntfLock.RUnlock()

	kvPair, err := rsrcMgr.KVStore.Get(ctx, path)
	if err != nil {
		logger.Error(ctx, "failed to get data from kv store")
		return nil, err
	}
	if kvPair != nil && kvPair.Value != nil {
		if val, err = kvstore.ToByte(kvPair.Value); err != nil {
			logger.Error(ctx, "Failed to convert to byte array ", log.Fields{"err": err})
			return nil, err
		}
		rsrcMgr.mcastQueueForIntfLock.Lock()
		defer rsrcMgr.mcastQueueForIntfLock.Unlock()
		if err = json.Unmarshal(val, &rsrcMgr.mcastQueueForIntf); err != nil {
			logger.Error(ctx, "Failed to unmarshall ", log.Fields{"err": err})
			return nil, err
		}
		rsrcMgr.mcastQueueForIntfLoadedFromKvStore = true
	}
	return rsrcMgr.mcastQueueForIntf, nil
}

//AddMcastQueueForIntf adds multicast queue for pon interface
func (rsrcMgr *OpenOltResourceMgr) AddMcastQueueForIntf(ctx context.Context, intf uint32, gem uint32, servicePriority uint32) error {
	var val []byte
	path := McastQueuesForIntf

	// Load local cache from kv store the first time
	rsrcMgr.mcastQueueForIntfLock.RLock()
	if !rsrcMgr.mcastQueueForIntfLoadedFromKvStore {
		rsrcMgr.mcastQueueForIntfLock.RUnlock()
		_, err := rsrcMgr.GetMcastQueuePerInterfaceMap(ctx)
		if err != nil {
			logger.Errorw(ctx, "Failed to get multicast queue info for interface", log.Fields{"err": err, "intf": intf})
			return err
		}
	} else {
		rsrcMgr.mcastQueueForIntfLock.RUnlock()
	}

	// Update KV store
	rsrcMgr.mcastQueueForIntfLock.Lock()
	rsrcMgr.mcastQueueForIntf[intf] = []uint32{gem, servicePriority}
	val, err := json.Marshal(rsrcMgr.mcastQueueForIntf)
	if err != nil {
		rsrcMgr.mcastQueueForIntfLock.Unlock()
		logger.Errorw(ctx, "Failed to marshal data", log.Fields{"err": err})
		return err
	}
	rsrcMgr.mcastQueueForIntfLock.Unlock()

	if err = rsrcMgr.KVStore.Put(ctx, path, val); err != nil {
		logger.Errorw(ctx, "Failed to put to kvstore", log.Fields{"err": err, "path": path, "value": val})
		return err
	}
	logger.Debugw(ctx, "added multicast queue info to KV store successfully", log.Fields{"path": path, "interfaceId": intf, "gem": gem, "svcPrior": servicePriority})
	return nil
}

//AddFlowGroupToKVStore adds flow group into KV store
func (rsrcMgr *OpenOltResourceMgr) AddFlowGroupToKVStore(ctx context.Context, groupEntry *ofp.OfpGroupEntry, cached bool) error {
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

	if err = rsrcMgr.KVStore.Put(ctx, path, Value); err != nil {
		logger.Errorf(ctx, "Failed to update resource %s", path)
		return err
	}

	// update cache
	rsrcMgr.groupInfoLock.Lock()
	rsrcMgr.groupInfo[path] = &groupInfo
	rsrcMgr.groupInfoLock.Unlock()
	return nil
}

//RemoveFlowGroupFromKVStore removes flow group from KV store
func (rsrcMgr *OpenOltResourceMgr) RemoveFlowGroupFromKVStore(ctx context.Context, groupID uint32, cached bool) error {
	var path string
	if cached {
		path = fmt.Sprintf(FlowGroupCached, groupID)
	} else {
		path = fmt.Sprintf(FlowGroup, groupID)
	}

	if err := rsrcMgr.KVStore.Delete(ctx, path); err != nil {
		logger.Errorf(ctx, "Failed to remove resource %s due to %s", path, err)
		return err
	}

	// update cache
	rsrcMgr.groupInfoLock.Lock()
	delete(rsrcMgr.groupInfo, path)
	rsrcMgr.groupInfoLock.Unlock()
	return nil
}

//GetFlowGroupFromKVStore fetches flow group from the KV store. Returns (false, {} error) if any problem occurs during
//fetching the data. Returns (true, groupInfo, nil) if the group is fetched successfully.
// Returns (false, {}, nil) if the group does not exists in the KV store.
func (rsrcMgr *OpenOltResourceMgr) GetFlowGroupFromKVStore(ctx context.Context, groupID uint32, cached bool) (bool, GroupInfo, error) {
	var groupInfo GroupInfo
	var path string
	if cached {
		path = fmt.Sprintf(FlowGroupCached, groupID)
	} else {
		path = fmt.Sprintf(FlowGroup, groupID)
	}

	// read from cache
	rsrcMgr.groupInfoLock.RLock()
	gi, ok := rsrcMgr.groupInfo[path]
	rsrcMgr.groupInfoLock.RUnlock()
	if ok {
		return true, *gi, nil
	}

	kvPair, err := rsrcMgr.KVStore.Get(ctx, path)
	if err != nil {
		return false, groupInfo, err
	}
	if kvPair != nil && kvPair.Value != nil {
		Val, err := kvstore.ToByte(kvPair.Value)
		if err != nil {
			logger.Errorw(ctx, "Failed to convert flow group into byte array", log.Fields{"err": err})
			return false, groupInfo, err
		}
		if err = json.Unmarshal(Val, &groupInfo); err != nil {
			logger.Errorw(ctx, "Failed to unmarshal", log.Fields{"err": err})
			return false, groupInfo, err
		}
		// update cache
		rsrcMgr.groupInfoLock.Lock()
		rsrcMgr.groupInfo[path] = &groupInfo
		rsrcMgr.groupInfoLock.Unlock()

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
