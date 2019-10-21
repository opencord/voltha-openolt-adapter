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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/opencord/voltha-lib-go/v2/pkg/db/kvstore"
	"github.com/opencord/voltha-lib-go/v2/pkg/db/model"
	"github.com/opencord/voltha-lib-go/v2/pkg/log"
	ponrmgr "github.com/opencord/voltha-lib-go/v2/pkg/ponresourcemanager"
	ofp "github.com/opencord/voltha-protos/v2/go/openflow_13"
	"github.com/opencord/voltha-protos/v2/go/openolt"
)

const (
	// KvstoreTimeout specifies the time out for KV Store Connection
	KvstoreTimeout = 5
	// BasePathKvStore - service/voltha/openolt/<device_id>
	BasePathKvStore = "service/voltha/openolt/{%s}"
	// TpIDPathSuffix - <(pon_id, onu_id, uni_id)>/tp_id
	TpIDPathSuffix = "{%d,%d,%d}/tp_id"
	//MeterIDPathSuffix - <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
	MeterIDPathSuffix = "{%d,%d,%d}/{%d}/meter_id/{%s}"
	//Nni intfid suffix - nniintfids
	NnniIntfID = "nniintfids"
	//path on the kvstore to store packetin gemport,which will be used for packetin, pcketout
	//format: onu_packetin/<intfid>,<onuid>,<logicalport>
	OnuPacketINPath = "onu_packetin/{%d},{%d},{%d}"
)

// FlowInfo holds the flow information
type FlowInfo struct {
	Flow            *openolt.Flow
	FlowStoreCookie uint64
	FlowCategory    string
	LogicalFlowID   uint64
}

// OnuGemInfo holds onu information along with gem port list and uni port list
type OnuGemInfo struct {
	OnuID        uint32
	SerialNumber string
	IntfID       uint32
	GemPorts     []uint32
	UniPorts     []uint32
}

type PacketInInfoKey struct {
	IntfID      uint32
	OnuID       uint32
	LogicalPort uint32
}

// OpenOltResourceMgr holds resource related information as provided below for each field
type OpenOltResourceMgr struct {
	DeviceID    string         // OLT device id
	HostAndPort string         // Host and port of the kv store to connect to
	Args        string         // args
	KVStore     *model.Backend // backend kv store connection handle
	DeviceType  string
	Host        string              // Host ip of the kv store
	Port        int                 // port of the kv store
	DevInfo     *openolt.DeviceInfo // device information
	// array of pon resource managers per interface technology
	ResourceMgrs map[uint32]*ponrmgr.PONResourceManager
}

func newKVClient(storeType string, address string, timeout uint32) (kvstore.Client, error) {
	log.Infow("kv-store-type", log.Fields{"store": storeType})
	switch storeType {
	case "consul":
		return kvstore.NewConsulClient(address, int(timeout))
	case "etcd":
		return kvstore.NewEtcdClient(address, int(timeout))
	}
	return nil, errors.New("unsupported-kv-store")
}

// SetKVClient sets the KV client and return a kv backend
func SetKVClient(backend string, Host string, Port int, DeviceID string) *model.Backend {
	addr := Host + ":" + strconv.Itoa(Port)
	// TODO : Make sure direct call to NewBackend is working fine with backend , currently there is some
	// issue between kv store and backend , core is not calling NewBackend directly
	kvClient, err := newKVClient(backend, addr, KvstoreTimeout)
	if err != nil {
		log.Fatalw("Failed to init KV client\n", log.Fields{"err": err})
		return nil
	}
	kvbackend := &model.Backend{
		Client:     kvClient,
		StoreType:  backend,
		Host:       Host,
		Port:       Port,
		Timeout:    KvstoreTimeout,
		PathPrefix: fmt.Sprintf(BasePathKvStore, DeviceID)}

	return kvbackend
}

// NewResourceMgr init a New resource manager instance which in turn instantiates pon resource manager
// instances according to technology. Initializes the default resource ranges for all
// the resources.
func NewResourceMgr(deviceID string, KVStoreHostPort string, kvStoreType string, deviceType string, devInfo *openolt.DeviceInfo) *OpenOltResourceMgr {
	var ResourceMgr OpenOltResourceMgr
	log.Debugf("Init new resource manager , host_port: %s, deviceid: %s", KVStoreHostPort, deviceID)
	ResourceMgr.HostAndPort = KVStoreHostPort
	ResourceMgr.DeviceType = deviceType
	ResourceMgr.DevInfo = devInfo
	IPPort := strings.Split(KVStoreHostPort, ":")
	ResourceMgr.Host = IPPort[0]
	ResourceMgr.Port, _ = strconv.Atoi(IPPort[1])

	Backend := kvStoreType
	ResourceMgr.KVStore = SetKVClient(Backend, ResourceMgr.Host,
		ResourceMgr.Port, deviceID)
	if ResourceMgr.KVStore == nil {
		log.Error("Failed to setup KV store")
	}
	Ranges := make(map[string]*openolt.DeviceInfo_DeviceResourceRanges)
	RsrcMgrsByTech := make(map[string]*ponrmgr.PONResourceManager)
	ResourceMgr.ResourceMgrs = make(map[uint32]*ponrmgr.PONResourceManager)

	// TODO self.args = registry('main').get_args()

	/*
	   If a legacy driver returns protobuf without any ranges,s synthesize one from
	   the legacy global per-device information. This, in theory, is temporary until
	   the legacy drivers are upgrade to support pool ranges.
	*/
	if devInfo.Ranges == nil {
		var ranges openolt.DeviceInfo_DeviceResourceRanges
		ranges.Technology = devInfo.GetTechnology()

		NumPONPorts := devInfo.GetPonPorts()
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
		log.Debugf("Device info technology %s", technology)
		Ranges[technology] = TechRange
		RsrcMgrsByTech[technology], err = ponrmgr.NewPONResourceManager(technology, deviceType, deviceID,
			Backend, ResourceMgr.Host, ResourceMgr.Port)
		if err != nil {
			log.Errorf("Failed to create pon resource manager instance for technology %s", technology)
			return nil
		}
		// resource_mgrs_by_tech[technology] = resource_mgr
		if GlobalPONRsrcMgr == nil {
			GlobalPONRsrcMgr = RsrcMgrsByTech[technology]
		}
		for _, IntfID := range TechRange.IntfIds {
			ResourceMgr.ResourceMgrs[uint32(IntfID)] = RsrcMgrsByTech[technology]
		}
		// self.initialize_device_resource_range_and_pool(resource_mgr, global_resource_mgr, arange)
		InitializeDeviceResourceRangeAndPool(RsrcMgrsByTech[technology], GlobalPONRsrcMgr,
			TechRange, devInfo)
	}
	// After we have initialized resource ranges, initialize the
	// resource pools accordingly.
	for _, PONRMgr := range RsrcMgrsByTech {
		_ = PONRMgr.InitDeviceResourcePool()
	}
	log.Info("Initialization of  resource manager success!")
	return &ResourceMgr
}

// InitializeDeviceResourceRangeAndPool initializes the resource range pool according to the sharing type, then apply
// device specific information. If KV doesn't exist
// or is broader than the device, the device's information will
// dictate the range limits
func InitializeDeviceResourceRangeAndPool(ponRMgr *ponrmgr.PONResourceManager, globalPONRMgr *ponrmgr.PONResourceManager,
	techRange *openolt.DeviceInfo_DeviceResourceRanges, devInfo *openolt.DeviceInfo) {

	// init the resource range pool according to the sharing type

	log.Debugf("Resource range pool init for technology %s", ponRMgr.Technology)
	// first load from KV profiles
	status := ponRMgr.InitResourceRangesFromKVStore()
	if !status {
		log.Debugf("Failed to load resource ranges from KV store for tech %s", ponRMgr.Technology)
	}

	/*
	   Then apply device specific information. If KV doesn't exist
	   or is broader than the device, the device's information will
	   dictate the range limits
	*/
	log.Debugf("Using device info to init pon resource ranges for tech", ponRMgr.Technology)

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

	log.Debugw("Device info init", log.Fields{"technology": techRange.Technology,
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

	ponRMgr.InitDefaultPONResourceRanges(ONUIDStart, ONUIDEnd, ONUIDSharedPoolID,
		AllocIDStart, AllocIDEnd, AllocIDSharedPoolID,
		GEMPortIDStart, GEMPortIDEnd, GEMPortIDSharedPoolID,
		FlowIDStart, FlowIDEnd, FlowIDSharedPoolID, 0, 1,
		devInfo.PonPorts, techRange.IntfIds)

	// For global sharing, make sure to refresh both local and global resource manager instances' range

	if ONUIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		globalPONRMgr.UpdateRanges(ponrmgr.ONU_ID_START_IDX, ONUIDStart, ponrmgr.ONU_ID_END_IDX, ONUIDEnd,
			"", 0, nil)
		ponRMgr.UpdateRanges(ponrmgr.ONU_ID_START_IDX, ONUIDStart, ponrmgr.ONU_ID_END_IDX, ONUIDEnd,
			"", 0, globalPONRMgr)
	}
	if AllocIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		globalPONRMgr.UpdateRanges(ponrmgr.ALLOC_ID_START_IDX, AllocIDStart, ponrmgr.ALLOC_ID_END_IDX, AllocIDEnd,
			"", 0, nil)

		ponRMgr.UpdateRanges(ponrmgr.ALLOC_ID_START_IDX, AllocIDStart, ponrmgr.ALLOC_ID_END_IDX, AllocIDEnd,
			"", 0, globalPONRMgr)
	}
	if GEMPortIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		globalPONRMgr.UpdateRanges(ponrmgr.GEMPORT_ID_START_IDX, GEMPortIDStart, ponrmgr.GEMPORT_ID_END_IDX, GEMPortIDEnd,
			"", 0, nil)
		ponRMgr.UpdateRanges(ponrmgr.GEMPORT_ID_START_IDX, GEMPortIDStart, ponrmgr.GEMPORT_ID_END_IDX, GEMPortIDEnd,
			"", 0, globalPONRMgr)
	}
	if FlowIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		globalPONRMgr.UpdateRanges(ponrmgr.FLOW_ID_START_IDX, FlowIDStart, ponrmgr.FLOW_ID_END_IDX, FlowIDEnd,
			"", 0, nil)
		ponRMgr.UpdateRanges(ponrmgr.FLOW_ID_START_IDX, FlowIDStart, ponrmgr.FLOW_ID_END_IDX, FlowIDEnd,
			"", 0, globalPONRMgr)
	}

	// Make sure loaded range fits the platform bit encoding ranges
	ponRMgr.UpdateRanges(ponrmgr.UNI_ID_START_IDX, 0, ponrmgr.UNI_ID_END_IDX /* TODO =OpenOltPlatform.MAX_UNIS_PER_ONU-1*/, 1, "", 0, nil)
}

// Delete clears used resources for the particular olt device being deleted
func (RsrcMgr *OpenOltResourceMgr) Delete() error {
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
		if err := rsrcMgr.ClearDeviceResourcePool(); err != nil {
			log.Debug("Failed to clear device resource pool")
			return err
		}
	}
	log.Debug("Cleared device resource pool")
	return nil
}

// GetONUID returns the available OnuID for the given pon-port
func (RsrcMgr *OpenOltResourceMgr) GetONUID(ponIntfID uint32) (uint32, error) {
	// Check if Pon Interface ID is present in Resource-manager-map
	if _, ok := RsrcMgr.ResourceMgrs[ponIntfID]; !ok {
		err := errors.New("invalid-pon-interface-" + strconv.Itoa(int(ponIntfID)))
		return 0, err
	}
	// Get ONU id for a provided pon interface ID.
	ONUID, err := RsrcMgr.ResourceMgrs[ponIntfID].GetResourceID(ponIntfID,
		ponrmgr.ONU_ID, 1)
	if err != nil {
		log.Errorf("Failed to get resource for interface %d for type %s",
			ponIntfID, ponrmgr.ONU_ID)
		return 0, err
	}
	if ONUID != nil {
		RsrcMgr.ResourceMgrs[ponIntfID].InitResourceMap(fmt.Sprintf("%d,%d", ponIntfID, ONUID[0]))
		return ONUID[0], err
	}

	return 0, err // return OnuID 0 on error
}

// GetFlowIDInfo returns the slice of flow info of the given pon-port
// Note: For flows which trap from the NNI and not really associated with any particular
// ONU (like LLDP), the onu_id and uni_id is set as -1. The intf_id is the NNI intf_id.
func (RsrcMgr *OpenOltResourceMgr) GetFlowIDInfo(ponIntfID uint32, onuID int32, uniID int32, flowID uint32) *[]FlowInfo {
	var flows []FlowInfo

	FlowPath := fmt.Sprintf("%d,%d,%d", ponIntfID, onuID, uniID)
	if err := RsrcMgr.ResourceMgrs[ponIntfID].GetFlowIDInfo(FlowPath, flowID, &flows); err != nil {
		log.Errorw("Error while getting flows from KV store", log.Fields{"flowId": flowID})
		return nil
	}
	if len(flows) == 0 {
		log.Debugw("No flowInfo found in KV store", log.Fields{"flowPath": FlowPath})
		return nil
	}
	return &flows
}

// GetCurrentFlowIDsForOnu fetches flow ID from the resource manager
// Note: For flows which trap from the NNI and not really associated with any particular
// ONU (like LLDP), the onu_id and uni_id is set as -1. The intf_id is the NNI intf_id.
func (RsrcMgr *OpenOltResourceMgr) GetCurrentFlowIDsForOnu(PONIntfID uint32, ONUID int32, UNIID int32) []uint32 {

	FlowPath := fmt.Sprintf("%d,%d,%d", PONIntfID, ONUID, UNIID)
	return RsrcMgr.ResourceMgrs[PONIntfID].GetCurrentFlowIDsForOnu(FlowPath)
}

// UpdateFlowIDInfo updates flow info for the given pon interface, onu id, and uni id
// Note: For flows which trap from the NNI and not really associated with any particular
// ONU (like LLDP), the onu_id and uni_id is set as -1. The intf_id is the NNI intf_id.
func (RsrcMgr *OpenOltResourceMgr) UpdateFlowIDInfo(ponIntfID int32, onuID int32, uniID int32,
	flowID uint32, flowData *[]FlowInfo) error {
	FlowPath := fmt.Sprintf("%d,%d,%d", ponIntfID, onuID, uniID)
	return RsrcMgr.ResourceMgrs[uint32(ponIntfID)].UpdateFlowIDInfoForOnu(FlowPath, flowID, *flowData)
}

// GetFlowID return flow ID for a given pon interface id, onu id and uni id
func (RsrcMgr *OpenOltResourceMgr) GetFlowID(ponIntfID uint32, ONUID int32, uniID int32,
	gemportID uint32,
	flowStoreCookie uint64,
	flowCategory string, vlanPcp ...uint32) (uint32, error) {

	var err error
	FlowPath := fmt.Sprintf("%d,%d,%d", ponIntfID, ONUID, uniID)
	FlowIDs := RsrcMgr.ResourceMgrs[ponIntfID].GetCurrentFlowIDsForOnu(FlowPath)
	if FlowIDs != nil {
		log.Debugw("Found flowId(s) for this ONU", log.Fields{"pon": ponIntfID, "ONUID": ONUID, "uniID": uniID, "KVpath": FlowPath})
		for _, flowID := range FlowIDs {
			FlowInfo := RsrcMgr.GetFlowIDInfo(ponIntfID, int32(ONUID), int32(uniID), uint32(flowID))
			er := getFlowIDFromFlowInfo(FlowInfo, flowID, gemportID, flowStoreCookie, flowCategory, vlanPcp...)
			if er == nil {
				return flowID, er
			}
		}
	}
	log.Debug("No matching flows with flow cookie or flow category, allocating new flowid")
	FlowIDs, err = RsrcMgr.ResourceMgrs[ponIntfID].GetResourceID(ponIntfID,
		ponrmgr.FLOW_ID, 1)
	if err != nil {
		log.Errorf("Failed to get resource for interface %d for type %s",
			ponIntfID, ponrmgr.FLOW_ID)
		return uint32(0), err
	}
	if FlowIDs != nil {
		_ = RsrcMgr.ResourceMgrs[ponIntfID].UpdateFlowIDForOnu(FlowPath, FlowIDs[0], true)
		return FlowIDs[0], err
	}

	return 0, err
}

// GetAllocID return the first Alloc ID for a given pon interface id and onu id and then update the resource map on
// the KV store with the list of alloc_ids allocated for the pon_intf_onu_id tuple
// Currently of all the alloc_ids available, it returns the first alloc_id in the list for tha given ONU
func (RsrcMgr *OpenOltResourceMgr) GetAllocID(intfID uint32, onuID uint32, uniID uint32) uint32 {

	var err error
	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)
	AllocID := RsrcMgr.ResourceMgrs[intfID].GetCurrentAllocIDForOnu(IntfOnuIDUniID)
	if AllocID != nil {
		// Since we support only one alloc_id for the ONU at the moment,
		// return the first alloc_id in the list, if available, for that
		// ONU.
		log.Debugw("Retrieved alloc ID from pon resource mgr", log.Fields{"AllocID": AllocID})
		return AllocID[0]
	}
	AllocID, err = RsrcMgr.ResourceMgrs[intfID].GetResourceID(intfID,
		ponrmgr.ALLOC_ID, 1)

	if AllocID == nil || err != nil {
		log.Error("Failed to allocate alloc id")
		return 0
	}
	// update the resource map on KV store with the list of alloc_id
	// allocated for the pon_intf_onu_id tuple
	err = RsrcMgr.ResourceMgrs[intfID].UpdateAllocIdsForOnu(IntfOnuIDUniID, AllocID)
	if err != nil {
		log.Error("Failed to update Alloc ID")
		return 0
	}
	log.Debugw("Allocated new Tcont from pon resource mgr", log.Fields{"AllocID": AllocID})
	return AllocID[0]
}

// UpdateAllocIdsForOnu updates alloc ids in kv store for a given pon interface id, onu id and uni id
func (RsrcMgr *OpenOltResourceMgr) UpdateAllocIdsForOnu(ponPort uint32, onuID uint32, uniID uint32, allocID []uint32) error {

	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", ponPort, onuID, uniID)
	return RsrcMgr.ResourceMgrs[ponPort].UpdateAllocIdsForOnu(IntfOnuIDUniID,
		allocID)
}

// GetCurrentGEMPortIDsForOnu returns gem ports for given pon interface , onu id and uni id
func (RsrcMgr *OpenOltResourceMgr) GetCurrentGEMPortIDsForOnu(intfID uint32, onuID uint32,
	uniID uint32) []uint32 {

	/* Get gem ports for given pon interface , onu id and uni id. */

	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)
	return RsrcMgr.ResourceMgrs[intfID].GetCurrentGEMPortIDsForOnu(IntfOnuIDUniID)
}

// GetCurrentAllocIDsForOnu returns alloc ids for given pon interface and onu id
func (RsrcMgr *OpenOltResourceMgr) GetCurrentAllocIDsForOnu(intfID uint32, onuID uint32, uniID uint32) []uint32 {

	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)
	AllocID := RsrcMgr.ResourceMgrs[intfID].GetCurrentAllocIDForOnu(IntfOnuIDUniID)
	if AllocID != nil {
		return AllocID
	}
	return []uint32{}
}

// RemoveAllocIDForOnu removes the alloc id for given pon interface, onu id, uni id and alloc id
func (RsrcMgr *OpenOltResourceMgr) RemoveAllocIDForOnu(intfID uint32, onuID uint32, uniID uint32, allocID uint32) {
	allocIDs := RsrcMgr.GetCurrentAllocIDsForOnu(intfID, onuID, uniID)
	for i := 0; i < len(allocIDs); i++ {
		if allocIDs[i] == allocID {
			allocIDs = append(allocIDs[:i], allocIDs[i+1:]...)
			break
		}
	}
	err := RsrcMgr.UpdateAllocIdsForOnu(intfID, onuID, uniID, allocIDs)
	if err != nil {
		log.Errorf("Failed to Remove Alloc Id For Onu. IntfID %d onuID %d uniID %d allocID %d",
			intfID, onuID, uniID, allocID)
	}
}

// RemoveGemPortIDForOnu removes the gem port id for given pon interface, onu id, uni id and gem port id
func (RsrcMgr *OpenOltResourceMgr) RemoveGemPortIDForOnu(intfID uint32, onuID uint32, uniID uint32, gemPortID uint32) {
	gemPortIDs := RsrcMgr.GetCurrentGEMPortIDsForOnu(intfID, onuID, uniID)
	for i := 0; i < len(gemPortIDs); i++ {
		if gemPortIDs[i] == gemPortID {
			gemPortIDs = append(gemPortIDs[:i], gemPortIDs[i+1:]...)
			break
		}
	}
	err := RsrcMgr.UpdateGEMPortIDsForOnu(intfID, onuID, uniID, gemPortIDs)
	if err != nil {
		log.Errorf("Failed to Remove Gem Id For Onu. IntfID %d onuID %d uniID %d gemPortId %d",
			intfID, onuID, uniID, gemPortID)
	}
}

// UpdateGEMportsPonportToOnuMapOnKVStore updates onu and uni id associated with the gem port to the kv store
// This stored information is used when packet_indication is received and we need to derive the ONU Id for which
// the packet arrived based on the pon_intf and gemport available in the packet_indication
func (RsrcMgr *OpenOltResourceMgr) UpdateGEMportsPonportToOnuMapOnKVStore(gemPorts []uint32, PonPort uint32,
	onuID uint32, uniID uint32) error {

	/* Update onu and uni id associated with the gem port to the kv store. */
	var IntfGEMPortPath string
	Data := fmt.Sprintf("%d %d", onuID, uniID)
	for _, GEM := range gemPorts {
		IntfGEMPortPath = fmt.Sprintf("%d,%d", PonPort, GEM)
		Val, err := json.Marshal(Data)
		if err != nil {
			log.Error("failed to Marshal")
			return err
		}

		if err = RsrcMgr.KVStore.Put(IntfGEMPortPath, Val); err != nil {
			log.Errorf("Failed to update resource %s", IntfGEMPortPath)
			return err
		}
	}
	return nil
}

// RemoveGEMportPonportToOnuMapOnKVStore removes the relationship between the gem port and pon port
func (RsrcMgr *OpenOltResourceMgr) RemoveGEMportPonportToOnuMapOnKVStore(GemPort uint32, PonPort uint32) {
	IntfGEMPortPath := fmt.Sprintf("%d,%d", PonPort, GemPort)
	err := RsrcMgr.KVStore.Delete(IntfGEMPortPath)
	if err != nil {
		log.Errorf("Failed to Remove Gem port-Pon port to onu map on kv store. Gem %d PonPort %d", GemPort, PonPort)
	}
}

// GetGEMPortID gets gem port id for a particular pon port, onu id and uni id and then update the resource map on
// the KV store with the list of gemport_id allocated for the pon_intf_onu_id tuple
func (RsrcMgr *OpenOltResourceMgr) GetGEMPortID(ponPort uint32, onuID uint32,
	uniID uint32, NumOfPorts uint32) ([]uint32, error) {

	/* Get gem port id for a particular pon port, onu id
	   and uni id.
	*/

	var err error
	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", ponPort, onuID, uniID)

	GEMPortList := RsrcMgr.ResourceMgrs[ponPort].GetCurrentGEMPortIDsForOnu(IntfOnuIDUniID)
	if GEMPortList != nil {
		return GEMPortList, nil
	}

	GEMPortList, err = RsrcMgr.ResourceMgrs[ponPort].GetResourceID(ponPort,
		ponrmgr.GEMPORT_ID, NumOfPorts)
	if err != nil && GEMPortList == nil {
		log.Errorf("Failed to get gem port id for %s", IntfOnuIDUniID)
		return nil, err
	}

	// update the resource map on KV store with the list of gemport_id
	// allocated for the pon_intf_onu_id tuple
	err = RsrcMgr.ResourceMgrs[ponPort].UpdateGEMPortIDsForOnu(IntfOnuIDUniID,
		GEMPortList)
	if err != nil {
		log.Errorf("Failed to update GEM ports to kv store for %s", IntfOnuIDUniID)
		return nil, err
	}
	_ = RsrcMgr.UpdateGEMportsPonportToOnuMapOnKVStore(GEMPortList, ponPort,
		onuID, uniID)
	return GEMPortList, err
}

// UpdateGEMPortIDsForOnu updates gemport ids on to the kv store for a given pon port, onu id and uni id
func (RsrcMgr *OpenOltResourceMgr) UpdateGEMPortIDsForOnu(ponPort uint32, onuID uint32,
	uniID uint32, GEMPortList []uint32) error {
	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", ponPort, onuID, uniID)
	return RsrcMgr.ResourceMgrs[ponPort].UpdateGEMPortIDsForOnu(IntfOnuIDUniID,
		GEMPortList)

}

// FreeonuID releases(make free) onu id for a particular pon-port
func (RsrcMgr *OpenOltResourceMgr) FreeonuID(intfID uint32, onuID []uint32) {

	RsrcMgr.ResourceMgrs[intfID].FreeResourceID(intfID, ponrmgr.ONU_ID, onuID)

	/* Free onu id for a particular interface.*/
	var IntfonuID string
	for _, onu := range onuID {
		IntfonuID = fmt.Sprintf("%d,%d", intfID, onu)
		RsrcMgr.ResourceMgrs[intfID].RemoveResourceMap(IntfonuID)
	}
}

// FreeFlowID returns the free flow id for a given interface, onu id and uni id
func (RsrcMgr *OpenOltResourceMgr) FreeFlowID(IntfID uint32, onuID int32,
	uniID int32, FlowID uint32) {
	var IntfONUID string
	var err error
	FlowIds := make([]uint32, 0)

	FlowIds = append(FlowIds, FlowID)
	IntfONUID = fmt.Sprintf("%d,%d,%d", IntfID, onuID, uniID)
	err = RsrcMgr.ResourceMgrs[IntfID].UpdateFlowIDForOnu(IntfONUID, FlowID, false)
	if err != nil {
		log.Errorw("Failed to Update flow id  for", log.Fields{"intf": IntfONUID})
	}
	RsrcMgr.ResourceMgrs[IntfID].RemoveFlowIDInfo(IntfONUID, FlowID)
	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID, ponrmgr.FLOW_ID, FlowIds)
}

// FreeFlowIDs releases the flow Ids
func (RsrcMgr *OpenOltResourceMgr) FreeFlowIDs(IntfID uint32, onuID uint32,
	uniID uint32, FlowID []uint32) {

	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID, ponrmgr.FLOW_ID, FlowID)

	var IntfOnuIDUniID string
	var err error
	for _, flow := range FlowID {
		IntfOnuIDUniID = fmt.Sprintf("%d,%d,%d", IntfID, onuID, uniID)
		err = RsrcMgr.ResourceMgrs[IntfID].UpdateFlowIDForOnu(IntfOnuIDUniID, flow, false)
		if err != nil {
			log.Errorw("Failed to Update flow id for", log.Fields{"intf": IntfOnuIDUniID})
		}
		RsrcMgr.ResourceMgrs[IntfID].RemoveFlowIDInfo(IntfOnuIDUniID, flow)
	}
}

// FreeAllocID frees AllocID on the PON resource pool and also frees the allocID association
// for the given OLT device.
func (RsrcMgr *OpenOltResourceMgr) FreeAllocID(IntfID uint32, onuID uint32,
	uniID uint32, allocID uint32) {
	RsrcMgr.RemoveAllocIDForOnu(IntfID, onuID, uniID, allocID)
	allocIDs := make([]uint32, 0)
	allocIDs = append(allocIDs, allocID)
	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID, ponrmgr.ALLOC_ID, allocIDs)
}

// FreeGemPortID frees GemPortID on the PON resource pool and also frees the gemPortID association
// for the given OLT device.
func (RsrcMgr *OpenOltResourceMgr) FreeGemPortID(IntfID uint32, onuID uint32,
	uniID uint32, gemPortID uint32) {
	RsrcMgr.RemoveGemPortIDForOnu(IntfID, onuID, uniID, gemPortID)
	gemPortIDs := make([]uint32, 0)
	gemPortIDs = append(gemPortIDs, gemPortID)
	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID, ponrmgr.GEMPORT_ID, gemPortIDs)
}

// FreePONResourcesForONU make the pon resources free for a given pon interface and onu id, and the clears the
// resource map and the onuID associated with (pon_intf_id, gemport_id) tuple,
func (RsrcMgr *OpenOltResourceMgr) FreePONResourcesForONU(intfID uint32, onuID uint32, uniID uint32) {

	var onuIDs []uint32
	onuIDs = append(onuIDs, onuID)
	IntfOnuIDUniID := fmt.Sprintf("%d,%d,%d", intfID, onuID, uniID)

	AllocIDs := RsrcMgr.ResourceMgrs[intfID].GetCurrentAllocIDForOnu(IntfOnuIDUniID)

	RsrcMgr.ResourceMgrs[intfID].FreeResourceID(intfID,
		ponrmgr.ALLOC_ID,
		AllocIDs)

	GEMPortIDs := RsrcMgr.ResourceMgrs[intfID].GetCurrentGEMPortIDsForOnu(IntfOnuIDUniID)
	RsrcMgr.ResourceMgrs[intfID].FreeResourceID(intfID,
		ponrmgr.GEMPORT_ID,
		GEMPortIDs)

	FlowIDs := RsrcMgr.ResourceMgrs[intfID].GetCurrentFlowIDsForOnu(IntfOnuIDUniID)
	RsrcMgr.ResourceMgrs[intfID].FreeResourceID(intfID,
		ponrmgr.FLOW_ID,
		FlowIDs)
	if int32(onuID) >= 0 {
		RsrcMgr.ResourceMgrs[intfID].FreeResourceID(intfID,
			ponrmgr.ONU_ID,
			onuIDs)
	}
	// Clear resource map associated with (pon_intf_id, gemport_id) tuple.
	RsrcMgr.ResourceMgrs[intfID].RemoveResourceMap(IntfOnuIDUniID)

	// Clear the ONU Id associated with the (pon_intf_id, gemport_id) tuple.
	for _, GEM := range GEMPortIDs {
		_ = RsrcMgr.KVStore.Delete(fmt.Sprintf("%d,%d", intfID, GEM))
	}
}

// IsFlowCookieOnKVStore checks if the given flow cookie is present on the kv store
// Returns true if the flow cookie is found, otherwise it returns false
func (RsrcMgr *OpenOltResourceMgr) IsFlowCookieOnKVStore(ponIntfID uint32, onuID int32, uniID int32,
	flowStoreCookie uint64) bool {

	FlowPath := fmt.Sprintf("%d,%d,%d", ponIntfID, onuID, uniID)
	FlowIDs := RsrcMgr.ResourceMgrs[ponIntfID].GetCurrentFlowIDsForOnu(FlowPath)
	if FlowIDs != nil {
		log.Debugw("Found flowId(s) for this ONU", log.Fields{"pon": ponIntfID, "onuID": onuID, "uniID": uniID, "KVpath": FlowPath})
		for _, flowID := range FlowIDs {
			FlowInfo := RsrcMgr.GetFlowIDInfo(ponIntfID, int32(onuID), int32(uniID), uint32(flowID))
			if FlowInfo != nil {
				log.Debugw("Found flows", log.Fields{"flows": *FlowInfo, "flowId": flowID})
				for _, Info := range *FlowInfo {
					if Info.FlowStoreCookie == flowStoreCookie {
						log.Debug("Found flow matching with flowStore cookie", log.Fields{"flowId": flowID, "flowStoreCookie": flowStoreCookie})
						return true
					}
				}
			}
		}
	}
	return false
}

// GetTechProfileIDForOnu fetches Tech-Profile-ID from the KV-Store for the given onu based on the path
// This path is formed as the following: {IntfID, OnuID, UniID}/tp_id
func (RsrcMgr *OpenOltResourceMgr) GetTechProfileIDForOnu(IntfID uint32, OnuID uint32, UniID uint32) []uint32 {
	Path := fmt.Sprintf(TpIDPathSuffix, IntfID, OnuID, UniID)
	var Data []uint32
	Value, err := RsrcMgr.KVStore.Get(Path)
	if err == nil {
		if Value != nil {
			Val, err := kvstore.ToByte(Value.Value)
			if err != nil {
				log.Errorw("Failed to convert into byte array", log.Fields{"error": err})
				return Data
			}
			if err = json.Unmarshal(Val, &Data); err != nil {
				log.Error("Failed to unmarshal", log.Fields{"error": err})
				return Data
			}
		}
	} else {
		log.Errorf("Failed to get TP id from kvstore for path %s", Path)
	}
	log.Debugf("Getting TP id %d from path %s", Data, Path)
	return Data

}

// RemoveTechProfileIDsForOnu deletes all tech profile ids from the KV-Store for the given onu based on the path
// This path is formed as the following: {IntfID, OnuID, UniID}/tp_id
func (RsrcMgr *OpenOltResourceMgr) RemoveTechProfileIDsForOnu(IntfID uint32, OnuID uint32, UniID uint32) error {
	IntfOnuUniID := fmt.Sprintf(TpIDPathSuffix, IntfID, OnuID, UniID)
	if err := RsrcMgr.KVStore.Delete(IntfOnuUniID); err != nil {
		log.Errorw("Failed to delete techprofile id resource in KV store", log.Fields{"path": IntfOnuUniID})
		return err
	}
	return nil
}

// RemoveTechProfileIDForOnu deletes a specific tech profile id from the KV-Store for the given onu based on the path
// This path is formed as the following: {IntfID, OnuID, UniID}/tp_id
func (RsrcMgr *OpenOltResourceMgr) RemoveTechProfileIDForOnu(IntfID uint32, OnuID uint32, UniID uint32, TpID uint32) error {
	tpIDList := RsrcMgr.GetTechProfileIDForOnu(IntfID, OnuID, UniID)
	for i, tpIDInList := range tpIDList {
		if tpIDInList == TpID {
			tpIDList = append(tpIDList[:i], tpIDList[i+1:]...)
		}
	}
	IntfOnuUniID := fmt.Sprintf(TpIDPathSuffix, IntfID, OnuID, UniID)
	Value, err := json.Marshal(tpIDList)
	if err != nil {
		log.Error("failed to Marshal")
		return err
	}
	if err = RsrcMgr.KVStore.Put(IntfOnuUniID, Value); err != nil {
		log.Errorf("Failed to update resource %s", IntfOnuUniID)
		return err
	}
	return err
}

// UpdateTechProfileIDForOnu updates (put) already present tech-profile-id for the given onu based on the path
// This path is formed as the following: {IntfID, OnuID, UniID}/tp_id
func (RsrcMgr *OpenOltResourceMgr) UpdateTechProfileIDForOnu(IntfID uint32, OnuID uint32,
	UniID uint32, TpID uint32) error {
	var Value []byte
	var err error

	IntfOnuUniID := fmt.Sprintf(TpIDPathSuffix, IntfID, OnuID, UniID)

	tpIDList := RsrcMgr.GetTechProfileIDForOnu(IntfID, OnuID, UniID)
	for _, value := range tpIDList {
		if value == TpID {
			log.Debugf("TpID %d is already in tpIdList for the path %s", TpID, IntfOnuUniID)
			return err
		}
	}
	log.Debugf("updating tp id %d on path %s", TpID, IntfOnuUniID)
	tpIDList = append(tpIDList, TpID)
	Value, err = json.Marshal(tpIDList)
	if err != nil {
		log.Error("failed to Marshal")
		return err
	}
	if err = RsrcMgr.KVStore.Put(IntfOnuUniID, Value); err != nil {
		log.Errorf("Failed to update resource %s", IntfOnuUniID)
		return err
	}
	return err
}

// UpdateMeterIDForOnu updates the meter id in the KV-Store for the given onu based on the path
// This path is formed as the following: <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
func (RsrcMgr *OpenOltResourceMgr) UpdateMeterIDForOnu(Direction string, IntfID uint32, OnuID uint32,
	UniID uint32, TpID uint32, MeterConfig *ofp.OfpMeterConfig) error {
	var Value []byte
	var err error

	IntfOnuUniID := fmt.Sprintf(MeterIDPathSuffix, IntfID, OnuID, UniID, TpID, Direction)
	Value, err = json.Marshal(*MeterConfig)
	if err != nil {
		log.Error("failed to Marshal meter config")
		return err
	}
	if err = RsrcMgr.KVStore.Put(IntfOnuUniID, Value); err != nil {
		log.Errorf("Failed to store meter into KV store %s", IntfOnuUniID)
		return err
	}
	return err
}

// GetMeterIDForOnu fetches the meter id from the kv store for the given onu based on the path
// This path is formed as the following: <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
func (RsrcMgr *OpenOltResourceMgr) GetMeterIDForOnu(Direction string, IntfID uint32, OnuID uint32,
	UniID uint32, TpID uint32) (*ofp.OfpMeterConfig, error) {
	Path := fmt.Sprintf(MeterIDPathSuffix, IntfID, OnuID, UniID, TpID, Direction)
	var meterConfig ofp.OfpMeterConfig
	Value, err := RsrcMgr.KVStore.Get(Path)
	if err == nil {
		if Value != nil {
			log.Debug("Found meter in KV store", log.Fields{"Direction": Direction})
			Val, er := kvstore.ToByte(Value.Value)
			if er != nil {
				log.Errorw("Failed to convert into byte array", log.Fields{"error": er})
				return nil, er
			}
			if er = json.Unmarshal(Val, &meterConfig); er != nil {
				log.Error("Failed to unmarshal meterconfig", log.Fields{"error": er})
				return nil, er
			}
		} else {
			log.Debug("meter-does-not-exists-in-KVStore")
			return nil, err
		}
	} else {
		log.Errorf("Failed to get Meter config from kvstore for path %s", Path)

	}
	return &meterConfig, err
}

// RemoveMeterIDForOnu deletes the meter id from the kV-Store for the given onu based on the path
// This path is formed as the following: <(pon_id, onu_id, uni_id)>/<tp_id>/meter_id/<direction>
func (RsrcMgr *OpenOltResourceMgr) RemoveMeterIDForOnu(Direction string, IntfID uint32, OnuID uint32,
	UniID uint32, TpID uint32) error {
	Path := fmt.Sprintf(MeterIDPathSuffix, IntfID, OnuID, UniID, TpID, Direction)
	if err := RsrcMgr.KVStore.Delete(Path); err != nil {
		log.Errorf("Failed to delete meter id %s from kvstore ", Path)
		return err
	}
	return nil
}

func getFlowIDFromFlowInfo(FlowInfo *[]FlowInfo, flowID, gemportID uint32, flowStoreCookie uint64, flowCategory string, vlanPcp ...uint32) error {
	if FlowInfo != nil {
		for _, Info := range *FlowInfo {
			if int32(gemportID) == Info.Flow.GemportId && flowCategory != "" && Info.FlowCategory == flowCategory {
				log.Debug("Found flow matching with flow category", log.Fields{"flowId": flowID, "FlowCategory": flowCategory})
				if Info.FlowCategory == "HSIA_FLOW" && Info.Flow.Classifier.OPbits == vlanPcp[0] {
					log.Debug("Found matching vlan pcp ", log.Fields{"flowId": flowID, "Vlanpcp": vlanPcp[0]})
					return nil
				}
			}
			if int32(gemportID) == Info.Flow.GemportId && flowStoreCookie != 0 && Info.FlowStoreCookie == flowStoreCookie {
				if flowCategory != "" && Info.FlowCategory == flowCategory {
					log.Debug("Found flow matching with flow category", log.Fields{"flowId": flowID, "FlowCategory": flowCategory})
					return nil
				}
			}
		}
	}
	log.Debugw("the flow can be related to a different service", log.Fields{"flow_info": FlowInfo})
	return errors.New("invalid flow-info")
}

//Adds gemport to onugem info kvstore
func (RMgr *OpenOltResourceMgr) AddGemToOnuGemInfo(intfID uint32, onuID uint32, gemPort uint32) error {
	var onuGemData []OnuGemInfo
	var err error

	if err = RMgr.ResourceMgrs[intfID].GetOnuGemInfo(intfID, &onuGemData); err != nil {
		log.Errorf("failed to get onuifo for intfid %d", intfID)
		return err
	}
	if len(onuGemData) == 0 {
		log.Errorw("failed to ger Onuid info ", log.Fields{"intfid": intfID, "onuid": onuID})
		return err
	}

	for idx, onugem := range onuGemData {
		if onugem.OnuID == onuID {
			for _, gem := range onuGemData[idx].GemPorts {
				if gem == gemPort {
					log.Debugw("Gem already present in onugem info, skpping addition", log.Fields{"gem": gem})
					return nil
				}
			}
			log.Debugw("Added gem to onugem info", log.Fields{"gem": gemPort})
			onuGemData[idx].GemPorts = append(onuGemData[idx].GemPorts, gemPort)
			break
		}
	}
	err = RMgr.ResourceMgrs[intfID].AddOnuGemInfo(intfID, onuGemData)
	if err != nil {
		log.Error("Failed to add onugem to kv store")
		return err
	}
	return err
}

// Get onu gem info from the kvstore per interface
func (RMgr *OpenOltResourceMgr) GetOnuGemInfo(IntfId uint32) ([]OnuGemInfo, error) {
	var onuGemData []OnuGemInfo

	if err := RMgr.ResourceMgrs[IntfId].GetOnuGemInfo(IntfId, &onuGemData); err != nil {
		log.Errorf("failed to get onuifo for intfid %d", IntfId)
		return nil, err
	}

	return onuGemData, nil
}

//Adds onu info on to the kvstore per interface
func (RMgr *OpenOltResourceMgr) AddOnuInfo(IntfId uint32, onuGem OnuGemInfo) error {
	var onuGemData []OnuGemInfo
	var err error

	if err = RMgr.ResourceMgrs[IntfId].GetOnuGemInfo(IntfId, &onuGemData); err != nil {
		log.Errorf("failed to get onuifo for intfid %d", IntfId)
		return err
	}
	onuGemData = append(onuGemData, onuGem)
	err = RMgr.ResourceMgrs[IntfId].AddOnuGemInfo(IntfId, onuGemData)
	if err != nil {
		log.Error("Failed to add onugem to kv store")
		return err
	}

	log.Debugw("added onu to onugeminfo", log.Fields{"intf": IntfId, "onugem": onuGem})
	return err
}

//Updates Onuinfo on the kvstore per interface
func (RMgr *OpenOltResourceMgr) UpdateOnuInfo(IntfId uint32, onuGem []OnuGemInfo) error {
	var onuGemData []OnuGemInfo
	var err error

	err = RMgr.ResourceMgrs[IntfId].AddOnuGemInfo(IntfId, onuGemData)
	if err != nil {
		log.Error("Failed to add onugem to kv store")
		return err
	}

	log.Debugw("updated onugeminfo", log.Fields{"intf": IntfId, "onugem": onuGem})
	return err
}

//Adds uni port to the onuinfo kvstore. check if the uni is already present if not update the kv store.
func (Rmgr *OpenOltResourceMgr) AddUniPortToOnuInfo(intfID uint32, onuID uint32, portNo uint32) {
	var onuGemData []OnuGemInfo
	var err error

	if err = Rmgr.ResourceMgrs[intfID].GetOnuGemInfo(intfID, &onuGemData); err != nil {
		log.Errorf("failed to get onuifo for intfid %d", intfID)
		return
	}
	for idx, onu := range onuGemData {
		if onu.OnuID == onuID {
			for _, uni := range onu.UniPorts {
				if uni == portNo {
					log.Debugw("uni already present in onugem info", log.Fields{"uni": portNo})
					return
				}
			}
			onuGemData[idx].UniPorts = append(onuGemData[idx].UniPorts, portNo)
			break
		}
	}
	err = Rmgr.ResourceMgrs[intfID].AddOnuGemInfo(intfID, onuGemData)
	if err != nil {
		log.Errorw("Failed to add uin port in onugem to kv store", log.Fields{"uni": portNo})
		return
	}
	return
}

// updates gemport for pkt in path to kvstore, path being intfid, onuid, portno
func (Rmgr *OpenOltResourceMgr) UpdateGemPortForPktIn(pktIn PacketInInfoKey, gemPort uint32) {

	path := fmt.Sprintf(OnuPacketINPath, pktIn.IntfID, pktIn.OnuID, pktIn.LogicalPort)
	Value, err := json.Marshal(gemPort)
	if err != nil {
		log.Error("Failed to marshal data")
		return
	}
	if err = Rmgr.KVStore.Put(path, Value); err != nil {
		log.Errorw("Failed to put to kvstore", log.Fields{"path": path, "value": gemPort})
		return
	}
	log.Debugw("added gem packet in successfully", log.Fields{"path": path, "gem": gemPort})

	return
}

// get the gem port from onu pkt in path, path being intfid, onuid, portno
func (Rmgr *OpenOltResourceMgr) GetGemPortFromOnuPktIn(intfID uint32, onuID uint32, logicalPort uint32) (uint32, error) {

	var Val []byte
	var gemPort uint32

	path := fmt.Sprintf(OnuPacketINPath, intfID, onuID, logicalPort)

	value, err := Rmgr.KVStore.Get(path)
	if err != nil {
		log.Errorw("Failed to get from kv store", log.Fields{"path": path})
		return uint32(0), err
	} else if value == nil {
		log.Debugw("No pkt in gem found", log.Fields{"path": path})
		return uint32(0), nil
	}

	if Val, err = kvstore.ToByte(value.Value); err != nil {
		log.Error("Failed to convert to byte array")
		return uint32(0), err
	}
	if err = json.Unmarshal(Val, &gemPort); err != nil {
		log.Error("Failed to unmarshall")
		return uint32(0), err
	}
	log.Debugw("found packein gemport from path", log.Fields{"path": path, "gem": gemPort})

	return gemPort, nil
}

// delete the gemport from the pkt in path
func (Rmgr *OpenOltResourceMgr) DelGemPortPktIn(intfID uint32, onuID uint32, logicalPort uint32) error {

	path := fmt.Sprintf(OnuPacketINPath, intfID, onuID, logicalPort)
	if err := Rmgr.KVStore.Delete(path); err != nil {
		log.Errorf("Falied to remove resource %s", path)
		return err
	}
	return nil
}

// Deletes the onugem info from kvstore per interface
func (Rmgr *OpenOltResourceMgr) DelOnuGemInfoForIntf(intfId uint32) error {
	if err := Rmgr.ResourceMgrs[intfId].DelOnuGemInfoForIntf(intfId); err != nil {
		log.Errorw("failed to delete onu gem info for", log.Fields{"intfid": intfId})
		return err
	}
	return nil
}

//Get NNi intfids from kvstore. path being per device
func (Rmgr *OpenOltResourceMgr) GetNNIFromKVStore() ([]uint32, error) {

	var nni []uint32
	var Val []byte

	path := fmt.Sprintf(NnniIntfID)
	value, err := Rmgr.KVStore.Get(path)
	if err != nil {
		log.Error("failed to get data from kv store")
		return nil, err
	}
	if value != nil {
		if Val, err = kvstore.ToByte(value.Value); err != nil {
			log.Error("Failed to convert to byte array")
			return nil, err
		}
		if err = json.Unmarshal(Val, &nni); err != nil {
			log.Error("Failed to unmarshall")
			return nil, err
		}
	}
	return nni, err
}

// Add Nni interfaces to kvstore, path being per device.
func (Rmgr *OpenOltResourceMgr) AddNNIToKVStore(nniIntf uint32) error {
	var Value []byte

	nni, err := Rmgr.GetNNIFromKVStore()
	if err != nil {
		log.Error("failed to fetch nni interfaces from kv store")
		return err
	}

	path := fmt.Sprintf(NnniIntfID)
	nni = append(nni, nniIntf)
	Value, err = json.Marshal(nni)
	if err != nil {
		log.Error("Failed to marshal data")
	}
	if err = Rmgr.KVStore.Put(path, Value); err != nil {
		log.Errorw("Failed to put to kvstore", log.Fields{"path": path, "value": Value})
		return err
	}
	log.Debugw("added nni to kv successfully", log.Fields{"path": path, "nni": nniIntf})
	return nil
}

// Delete nni interface list from kv store.
func (Rmgr *OpenOltResourceMgr) DelNNiFromKVStore() error {

	path := fmt.Sprintf(NnniIntfID)

	if err := Rmgr.KVStore.Delete(path); err != nil {
		log.Errorw("Failed to delete nni interfaces from kv store", log.Fields{"path": path})
		return err
	}
	return nil
}
