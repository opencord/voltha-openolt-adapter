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

package resourcemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/opencord/voltha-go/common/log"
	ponrmgr "github.com/opencord/voltha-go/common/ponresourcemanager"
	"github.com/opencord/voltha-go/db/kvstore"
	"github.com/opencord/voltha-go/db/model"
	"github.com/opencord/voltha-protos/go/openolt"
)

const KVSTORE_TIMEOUT = 5
const BASE_PATH_KV_STORE = "service/voltha/openolt/{%s}" // service/voltha/openolt/<device_id>

type OpenOltResourceMgr struct {
	DeviceID    string         //OLT device id
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

func SetKVClient(Backend string, Host string, Port int, DeviceID string) *model.Backend {
	addr := Host + ":" + strconv.Itoa(Port)
	// TODO : Make sure direct call to NewBackend is working fine with backend , currently there is some
	// issue between kv store and backend , core is not calling NewBackend directly
	kvClient, err := newKVClient(Backend, addr, KVSTORE_TIMEOUT)
	if err != nil {
		log.Fatalw("Failed to init KV client\n", log.Fields{"err": err})
		return nil
	}
	kvbackend := &model.Backend{
		Client:     kvClient,
		StoreType:  Backend,
		Host:       Host,
		Port:       Port,
		Timeout:    KVSTORE_TIMEOUT,
		PathPrefix: fmt.Sprintf(BASE_PATH_KV_STORE, DeviceID)}

	return kvbackend
}

func NewResourceMgr(DeviceID string, HostPort string, DeviceType string, DevInfo *openolt.DeviceInfo) *OpenOltResourceMgr {

	/* init a New resource maanger instance which in turn instantiates pon resource manager
	   instances according to technology. Initializes the default resource ranges for all
	   the reources.
	*/
	var ResourceMgr OpenOltResourceMgr
	log.Debugf("Init new resource manager , host_port: %s, deviceid: %s", HostPort, DeviceID)
	ResourceMgr.HostAndPort = HostPort
	ResourceMgr.DeviceType = DeviceType
	ResourceMgr.DevInfo = DevInfo
	IpPort := strings.Split(HostPort, ":")
	ResourceMgr.Host = IpPort[0]
	ResourceMgr.Port, _ = strconv.Atoi(IpPort[1])

	Backend := "etcd" /* TODO  remove this once we get backend store from registry*/
	ResourceMgr.KVStore = SetKVClient(Backend, ResourceMgr.Host,
		ResourceMgr.Port, DeviceID)
	if ResourceMgr.KVStore == nil {
		log.Error("Failed to setup KV store")
	}
	Ranges := make(map[string]*openolt.DeviceInfo_DeviceResourceRanges)
	RsrcMgrsByTech := make(map[string]*ponrmgr.PONResourceManager)
	ResourceMgr.ResourceMgrs = make(map[uint32]*ponrmgr.PONResourceManager)

	// TODO self.args = registry('main').get_args()

	/*
	   If a legacy driver returns protobuf without any ranges,s synthesize one from
	   the legacy global per-device informaiton. This, in theory, is temporary until
	   the legacy drivers are upgrade to support pool ranges.
	*/
	if DevInfo.Ranges == nil {
		var ranges openolt.DeviceInfo_DeviceResourceRanges
		ranges.Technology = DevInfo.GetTechnology()

		NumPONPorts := DevInfo.GetPonPorts()
		var index uint32
		for index = 0; index < NumPONPorts; index++ {
			ranges.IntfIds = append(ranges.IntfIds, index)
		}

		var Pool *openolt.DeviceInfo_DeviceResourceRanges_Pool
		Pool.Type = openolt.DeviceInfo_DeviceResourceRanges_Pool_ONU_ID
		Pool.Start = DevInfo.OnuIdStart
		Pool.End = DevInfo.OnuIdEnd
		Pool.Sharing = openolt.DeviceInfo_DeviceResourceRanges_Pool_DEDICATED_PER_INTF
		ranges.Pools = append(ranges.Pools, Pool)

		Pool.Type = openolt.DeviceInfo_DeviceResourceRanges_Pool_ALLOC_ID
		Pool.Start = DevInfo.AllocIdStart
		Pool.End = DevInfo.AllocIdEnd
		Pool.Sharing = openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH
		ranges.Pools = append(ranges.Pools, Pool)

		Pool.Type = openolt.DeviceInfo_DeviceResourceRanges_Pool_GEMPORT_ID
		Pool.Start = DevInfo.GemportIdStart
		Pool.End = DevInfo.GemportIdEnd
		Pool.Sharing = openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH
		ranges.Pools = append(ranges.Pools, Pool)

		Pool.Type = openolt.DeviceInfo_DeviceResourceRanges_Pool_FLOW_ID
		Pool.Start = DevInfo.FlowIdStart
		Pool.End = DevInfo.FlowIdEnd
		Pool.Sharing = openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH
		ranges.Pools = append(ranges.Pools, Pool)
	}

	//Create a separate Resource Manager instance for each range. This assumes that
	// each technology is represented by only a single range
	var GlobalPONRsrcMgr *ponrmgr.PONResourceManager
	var err error
	for _, TechRange := range DevInfo.Ranges {
		technology := TechRange.Technology
		log.Debugf("Device info technology %s", technology)
		Ranges[technology] = TechRange
		RsrcMgrsByTech[technology], err = ponrmgr.NewPONResourceManager(technology, DeviceType, DeviceID,
			Backend, ResourceMgr.Host, ResourceMgr.Port)
		if err != nil {
			log.Errorf("Failed to create pon resource manager instacnce for technology %s", technology)
			return nil
		}
		//resource_mgrs_by_tech[technology] = resource_mgr
		if GlobalPONRsrcMgr == nil {
			GlobalPONRsrcMgr = RsrcMgrsByTech[technology]
		}
		for IntfId := range TechRange.IntfIds {
			ResourceMgr.ResourceMgrs[uint32(IntfId)] = RsrcMgrsByTech[technology]
		}
		//self.initialize_device_resource_range_and_pool(resource_mgr, global_resource_mgr, arange)
		InitializeDeviceResourceRangeAndPool(RsrcMgrsByTech[technology], GlobalPONRsrcMgr,
			TechRange, DevInfo)
	}
	// After we have initialized resource ranges, initialize the
	// resource pools accordingly.
	for _, PONRMgr := range RsrcMgrsByTech {
		PONRMgr.InitDeviceResourcePool()
	}
	return &ResourceMgr
}

func InitializeDeviceResourceRangeAndPool(PONRMgr *ponrmgr.PONResourceManager, GlobalPONRMgr *ponrmgr.PONResourceManager,
	TechRange *openolt.DeviceInfo_DeviceResourceRanges, DevInfo *openolt.DeviceInfo) {

	// init the resource range pool according to the sharing type

	log.Debugf("Resource range pool init for technology %s", PONRMgr.Technology)
	//first load from KV profiles
	status := PONRMgr.InitResourceRangesFromKVStore()
	if status == false {
		log.Debugf("Failed to load resource ranges from KV store for tech %s", PONRMgr.Technology)
	}

	/*
	   Then apply device specific information. If KV doesn't exist
	   or is broader than the device, the device's informationw ill
	   dictate the range limits
	*/
	log.Debugf("Using device info to init pon resource ranges for tech", PONRMgr.Technology)

	ONUIDStart := DevInfo.OnuIdStart
	ONUIDEnd := DevInfo.OnuIdEnd
	ONUIDShared := openolt.DeviceInfo_DeviceResourceRanges_Pool_DEDICATED_PER_INTF
	ONUIDSharedPoolID := uint32(0)
	AllocIDStart := DevInfo.AllocIdStart
	AllocIDEnd := DevInfo.AllocIdEnd
	AllocIDShared := openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH // TODO EdgeCore/BAL limitation
	AllocIDSharedPoolID := uint32(0)
	GEMPortIDStart := DevInfo.GemportIdStart
	GEMPortIDEnd := DevInfo.GemportIdEnd
	GEMPortIDShared := openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH // TODO EdgeCore/BAL limitation
	GEMPortIDSharedPoolID := uint32(0)
	FlowIDStart := DevInfo.FlowIdStart
	FlowIDEnd := DevInfo.FlowIdEnd
	FlowIDShared := openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH // TODO EdgeCore/BAL limitation
	FlowIDSharedPoolID := uint32(0)

	var GlobalPoolID uint32
	var FirstIntfPoolID uint32
	var SharedPoolID uint32

	for _, FirstIntfPoolID = range TechRange.IntfIds {
		break
	}

	for _, RangePool := range TechRange.Pools {
		if RangePool.Sharing == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
			SharedPoolID = GlobalPoolID
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

	log.Debugw("Device info init", log.Fields{"technology": TechRange.Technology,
		"onu_id_start": ONUIDStart, "onu_id_end": ONUIDEnd, "onu_id_shared_pool_id": ONUIDSharedPoolID,
		"alloc_id_start": AllocIDStart, "alloc_id_end": AllocIDEnd,
		"alloc_id_shared_pool_id": AllocIDSharedPoolID,
		"gemport_id_start":        GEMPortIDStart, "gemport_id_end": GEMPortIDEnd,
		"gemport_id_shared_pool_id": GEMPortIDSharedPoolID,
		"flow_id_start":             FlowIDStart,
		"flow_id_end_idx":           FlowIDEnd,
		"flow_id_shared_pool_id":    FlowIDSharedPoolID,
		"intf_ids":                  TechRange.IntfIds,
		"uni_id_start":              0,
		"uni_id_end_idx":/*MaxUNIIDperONU()*/ 1})

	PONRMgr.InitDefaultPONResourceRanges(ONUIDStart, ONUIDEnd, ONUIDSharedPoolID,
		AllocIDStart, AllocIDEnd, AllocIDSharedPoolID,
		GEMPortIDStart, GEMPortIDEnd, GEMPortIDSharedPoolID,
		FlowIDStart, FlowIDEnd, FlowIDSharedPoolID, 0, 1,
		DevInfo.PonPorts, TechRange.IntfIds)

	// For global sharing, make sure to refresh both local and global resource manager instances' range

	if ONUIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		GlobalPONRMgr.UpdateRanges(ponrmgr.ONU_ID_START_IDX, ONUIDStart, ponrmgr.ONU_ID_END_IDX, ONUIDEnd,
			"", 0, nil)
		PONRMgr.UpdateRanges(ponrmgr.ONU_ID_START_IDX, ONUIDStart, ponrmgr.ONU_ID_END_IDX, ONUIDEnd,
			"", 0, GlobalPONRMgr)
	}
	if AllocIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		GlobalPONRMgr.UpdateRanges(ponrmgr.ALLOC_ID_START_IDX, AllocIDStart, ponrmgr.ALLOC_ID_END_IDX, AllocIDEnd,
			"", 0, nil)

		PONRMgr.UpdateRanges(ponrmgr.ALLOC_ID_START_IDX, AllocIDStart, ponrmgr.ALLOC_ID_END_IDX, AllocIDEnd,
			"", 0, GlobalPONRMgr)
	}
	if GEMPortIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		GlobalPONRMgr.UpdateRanges(ponrmgr.GEMPORT_ID_START_IDX, GEMPortIDStart, ponrmgr.GEMPORT_ID_END_IDX, GEMPortIDEnd,
			"", 0, nil)
		PONRMgr.UpdateRanges(ponrmgr.GEMPORT_ID_START_IDX, GEMPortIDStart, ponrmgr.GEMPORT_ID_END_IDX, GEMPortIDEnd,
			"", 0, GlobalPONRMgr)
	}
	if FlowIDShared == openolt.DeviceInfo_DeviceResourceRanges_Pool_SHARED_BY_ALL_INTF_ALL_TECH {
		GlobalPONRMgr.UpdateRanges(ponrmgr.FLOW_ID_START_IDX, FlowIDStart, ponrmgr.FLOW_ID_END_IDX, FlowIDEnd,
			"", 0, nil)
		PONRMgr.UpdateRanges(ponrmgr.FLOW_ID_START_IDX, FlowIDStart, ponrmgr.FLOW_ID_END_IDX, FlowIDEnd,
			"", 0, GlobalPONRMgr)
	}

	// Make sure loaded range fits the platform bit encoding ranges
	PONRMgr.UpdateRanges(ponrmgr.UNI_ID_START_IDX, 0, ponrmgr.UNI_ID_END_IDX /* TODO =OpenOltPlatform.MAX_UNIS_PER_ONU-1*/, 1, "", 0, nil)
}

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

func (RsrcMgr *OpenOltResourceMgr) GetONUID(PONIntfID uint32) (uint32, error) {

	// Get ONU id for a provided pon interface ID.

	ONUID, err := RsrcMgr.ResourceMgrs[PONIntfID].GetResourceID(PONIntfID,
		ponrmgr.ONU_ID, 1)
	if err != nil {
		log.Errorf("Failed to get resource for interface %d for type %s",
			PONIntfID, ponrmgr.ONU_ID)
		return ONUID[0], err
	}
	if ONUID != nil {
		RsrcMgr.ResourceMgrs[PONIntfID].InitResourceMap(fmt.Sprintf("%d,%d", PONIntfID, ONUID))
	}

	return ONUID[0], err
}

func (RsrcMgr *OpenOltResourceMgr) GetFlowID(PONIntfID uint32, ONUID uint32, UNIID uint32,
	FlowStoreCookie interface{},
	FlowCategory interface{}) (uint32, error) {

	// Get flow ID for a given pon interface id, onu id and uni id.

	var err error
	FlowPath := fmt.Sprintf("%d,%d,%d", PONIntfID, ONUID, UNIID)
	FlowIDs := RsrcMgr.ResourceMgrs[PONIntfID].GetCurrentFlowIDsForOnu(FlowPath)
	if FlowIDs != nil {
		/* TODO once the flow id info structure is known
		   for Flow := range FlowIDs {
		       FlowInfo := RsrcMgr.ResourceMgrs[PONIntfID].GetFlowIDInfo(FlowPath, Flow)
		       for Info := range FlowInfo {
		           if FlowCategory != nil &&
		               Info[FlowCategory] == FlowCategory {
		               return 0, Flow
		           }
		           if FlowStoreCookie != nil &&
		               Info[FlowStoreCookie] == FlowStoreCookie {
		               return 0, Flow
		           }
		       }
		   }
		*/
	}
	FlowIDs, err = RsrcMgr.ResourceMgrs[PONIntfID].GetResourceID(PONIntfID,
		ponrmgr.FLOW_ID, 1)
	if err != nil {
		log.Errorf("Failed to get resource for interface %d for type %s",
			PONIntfID, ponrmgr.FLOW_ID)
		return FlowIDs[0], err
	}
	if FlowIDs != nil {
		RsrcMgr.ResourceMgrs[PONIntfID].UpdateFlowIDForOnu(FlowPath, FlowIDs[0], true)
	}

	return FlowIDs[0], err
}

func (RsrcMgr *OpenOltResourceMgr) GetAllocID(IntfID uint32, ONUID uint32) uint32 {

	// Get alloc id for a given pon interface id and onu id.
	var err error
	IntfONUID := fmt.Sprintf("%d,%d", IntfID, ONUID)
	AllocID := RsrcMgr.ResourceMgrs[IntfID].GetCurrentAllocIDForOnu(IntfONUID)
	if AllocID != nil {
		// Since we support only one alloc_id for the ONU at the moment,
		// return the first alloc_id in the list, if available, for that
		// ONU.
		return AllocID[0]
	}
	AllocID, err = RsrcMgr.ResourceMgrs[IntfID].GetResourceID(IntfID,
		ponrmgr.ALLOC_ID, 1)

	if AllocID == nil || err != nil {
		log.Error("Failed to allocate alloc id")
		return 0
	}
	// update the resource map on KV store with the list of alloc_id
	// allocated for the pon_intf_onu_id tuple
	err = RsrcMgr.ResourceMgrs[IntfID].UpdateAllocIdsForOnu(IntfONUID, AllocID)
	if err != nil {
		log.Error("Failed to update Alloc ID")
		return 0
	}
	return AllocID[0]
}

func (RsrcMgr *OpenOltResourceMgr) UpdateAllocIdsForOnu(PONPort uint32, ONUID uint32,
	UNIID uint32, AllocID []uint32) error {

	/* update alloc ids in kv store for a given pon interface id,
	   onu id and uni id.
	*/
	IntfONUID := fmt.Sprintf("%d,%d,%d", PONPort, ONUID, UNIID)
	return RsrcMgr.ResourceMgrs[PONPort].UpdateAllocIdsForOnu(IntfONUID,
		AllocID)
}
func (RsrcMgr *OpenOltResourceMgr) GetCurrentGEMPortIDsForOnu(IntfID uint32, ONUID uint32,
	UNIID uint32) []uint32 {

	/* Get gem ports for given pon interface , onu id and uni id. */

	IntfONUID := fmt.Sprintf("%d,%d,%d", IntfID, ONUID, UNIID)
	GEMPortID := RsrcMgr.ResourceMgrs[IntfID].GetCurrentGEMPortIDsForOnu(IntfONUID)
	if GEMPortID != nil {
		// Since we support only one alloc_id for the ONU at the moment,
		// return the first alloc_id in the list, if available, for that
		// ONU.
		return GEMPortID
	}
	return nil
}

func (RsrcMgr *OpenOltResourceMgr) GetCurrentAllocIDForOnu(IntfID uint32, ONUID uint32) uint32 {

	/* Get alloc ids for given pon interface and onu id. */

	IntfONUID := fmt.Sprintf("%d,%d", IntfID, ONUID)
	AllocID := RsrcMgr.ResourceMgrs[IntfID].GetCurrentAllocIDForOnu(IntfONUID)
	if AllocID != nil {
		// Since we support only one alloc_id for the ONU at the moment,
		// return the first alloc_id in the list, if available, for that
		// ONU.
		return AllocID[0]
	}
	return 0
}

func (RsrcMgr *OpenOltResourceMgr) UpdateGEMportsPonportToOnuMapOnKVStore(GEMPorts []uint32, PonPort uint32,
	ONUID uint32, UNIID uint32) error {

	/* Update onu and uni id associated with the gem port to the kv store. */
	var IntfGEMPortPath string
	Data := fmt.Sprintf("%d %d", ONUID, UNIID)
	for _, GEM := range GEMPorts {
		IntfGEMPortPath = fmt.Sprintf("%d,%d", PonPort, GEM)
		Val, err := json.Marshal(Data)
		if err != nil {
			log.Error("failed to Marshal")
			return err
		}
		// This information is used when packet_indication is received and
		// we need to derive the ONU Id for which the packet arrived based
		// on the pon_intf and gemport available in the packet_indication
		if err = RsrcMgr.KVStore.Put(IntfGEMPortPath, Val); err != nil {
			log.Errorf("Failed to update resource %s", IntfGEMPortPath)
			return err
		}
	}
	return nil
}

func (RsrcMgr *OpenOltResourceMgr) GetONUUNIfromPONPortGEMPort(PONPort uint32, GEMPort uint32) []uint32 {

	/* get the onu and uni id for a given gem port. */
	IntfGEMPortPath := fmt.Sprintf("%d,%d", PONPort, GEMPort)
	var GEMPortIDs []uint32
	var Data string
	Value, err := RsrcMgr.KVStore.Get(IntfGEMPortPath)
	if err == nil {
		if Value != nil {
			Val, _ := kvstore.ToByte(Value.Value)
			if err = json.Unmarshal(Val, &Data); err != nil {
				log.Error("Failed to unmarshal")
				return nil
			}
			IDs := strings.Split(Data, " ")
			for _, port := range IDs {
				Intport, _ := strconv.Atoi(port)
				GEMPortIDs = append(GEMPortIDs, uint32(Intport))
			}
		}
	}
	return GEMPortIDs
}

func (RsrcMgr *OpenOltResourceMgr) GetGEMPortID(PONPort uint32, ONUID uint32,
	UNIID uint32, NumOfPorts uint32) ([]uint32, error) {

	/* Get gem port id for a particular pon port, onu id
	   and uni id.
	*/

	var err error
	IntfONUID := fmt.Sprintf("%d,%d,%d", PONPort, ONUID, UNIID)

	GEMPortList := RsrcMgr.ResourceMgrs[PONPort].GetCurrentGEMPortIDsForOnu(IntfONUID)
	if GEMPortList != nil {
		return GEMPortList, nil
	}

	GEMPortList, err = RsrcMgr.ResourceMgrs[PONPort].GetResourceID(PONPort,
		ponrmgr.GEMPORT_ID, NumOfPorts)
	if err != nil && GEMPortList == nil {
		log.Errorf("Failed to get gem port id for %s", IntfONUID)
		return nil, err
	}

	// update the resource map on KV store with the list of gemport_id
	// allocated for the pon_intf_onu_id tuple
	err = RsrcMgr.ResourceMgrs[PONPort].UpdateGEMPortIDsForOnu(IntfONUID,
		GEMPortList)
	if err != nil {
		log.Errorf("Failed to update GEM ports to kv store for %s", IntfONUID)
		return nil, err
	}
	RsrcMgr.UpdateGEMportsPonportToOnuMapOnKVStore(GEMPortList, PONPort,
		ONUID, UNIID)
	return GEMPortList, err
}

func (RsrcMgr *OpenOltResourceMgr) UpdateGEMPortIDsForOnu(PONPort uint32, ONUID uint32,
	UNIID uint32, GEMPortList []uint32) error {

	/* Update gemport ids on to kv store for a given pon port,
	   onu id and uni id.
	*/
	IntfONUID := fmt.Sprintf("%d,%d,%d", PONPort, ONUID, UNIID)
	return RsrcMgr.ResourceMgrs[PONPort].UpdateGEMPortIDsForOnu(IntfONUID,
		GEMPortList)

}
func (RsrcMgr *OpenOltResourceMgr) FreeONUID(IntfID uint32, ONUID []uint32) {

	/* Free onu id for a particular interface.*/
	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID, ponrmgr.ONU_ID, ONUID)

	var IntfONUID string
	for _, onu := range ONUID {
		IntfONUID = fmt.Sprintf("%d,%d", IntfID, onu)
		RsrcMgr.ResourceMgrs[IntfID].RemoveResourceMap(IntfONUID)
	}
	return
}

func (RsrcMgr *OpenOltResourceMgr) FreeFlowID(IntfID uint32, ONUID uint32,
	UNIID uint32, FlowID []uint32) {

	/* Free flow id for a given interface, onu id and uni id.*/

	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID, ponrmgr.FLOW_ID, FlowID)

	var IntfONUID string
	var err error
	for _, flow := range FlowID {
		IntfONUID = fmt.Sprintf("%d,%d,%d", IntfID, ONUID, UNIID)
		err = RsrcMgr.ResourceMgrs[IntfID].UpdateFlowIDForOnu(IntfONUID, flow, false)
		if err != nil {
			log.Error("Failed to Update flow id infor for %s", IntfONUID)
		}
		RsrcMgr.ResourceMgrs[IntfID].RemoveFlowIDInfo(IntfONUID, flow)
	}
	return
}

func (RsrcMgr *OpenOltResourceMgr) FreePONResourcesForONU(IntfID uint32, ONUID uint32) {

	/* Free pon resources for a given pon interface and onu id. */

	var ONUIDs []uint32
	ONUIDs = append(ONUIDs, ONUID)
	IntfONUID := fmt.Sprintf("%d,%d", IntfID, ONUID)

	AllocIDs := RsrcMgr.ResourceMgrs[IntfID].GetCurrentAllocIDForOnu(IntfONUID)

	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID,
		ponrmgr.ALLOC_ID,
		AllocIDs)

	GEMPortIDs := RsrcMgr.ResourceMgrs[IntfID].GetCurrentGEMPortIDsForOnu(IntfONUID)
	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID,
		ponrmgr.GEMPORT_ID,
		GEMPortIDs)

	FlowIDs := RsrcMgr.ResourceMgrs[IntfID].GetCurrentFlowIDsForOnu(IntfONUID)
	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID,
		ponrmgr.FLOW_ID,
		FlowIDs)
	RsrcMgr.ResourceMgrs[IntfID].FreeResourceID(IntfID,
		ponrmgr.ONU_ID,
		ONUIDs)

	// Clear resource map associated with (pon_intf_id, gemport_id) tuple.
	RsrcMgr.ResourceMgrs[IntfID].RemoveResourceMap(IntfONUID)

	// Clear the ONU Id associated with the (pon_intf_id, gemport_id) tuple.
	for _, GEM := range GEMPortIDs {
		RsrcMgr.KVStore.Delete(fmt.Sprintf("%d,%d", IntfID, GEM))
	}
}

/* TODO once the flow id info structure is known
def is_flow_cookie_on_kv_store(self, intf_id, onu_id, uni_id, flow_store_cookie):
  '''
  Note: For flows which trap from the NNI and not really associated with any particular
  ONU (like LLDP), the onu_id and uni_id is set as -1. The intf_id is the NNI intf_id.
  '''
  intf_onu_id = (intf_id, onu_id, uni_id)
  try:
      flow_ids = self.resource_mgrs[intf_id]. \
          get_current_flow_ids_for_onu(intf_onu_id)
      if flow_ids is not None:
          for flow_id in flow_ids:
              flows = self.get_flow_id_info(intf_id, onu_id, uni_id, flow_id)
              assert (isinstance(flows, list))
              for flow in flows:
                  if flow['flow_store_cookie'] == flow_store_cookie:
                      return True
  except Exception as e:
      self.log.error("error-retrieving-flow-info", e=e)

  return False
*/
