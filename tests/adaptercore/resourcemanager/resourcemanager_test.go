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

package resourcemanager

import (
	rmgr "github.com/opencord/voltha-openolt-adapter/adaptercore/resourcemanager"
	"github.com/opencord/voltha-protos/go/openolt"
	"testing"
)

type TestResourceMrg struct {
	ResourceMgr     *rmgr.OpenOltResourceMgr
	DeviceId        string
	KVStoreHostPort string
	KVStoreType     string
	DeviceType      string
	DevInfo         *openolt.DeviceInfo
}

var rm TestResourceMrg

func GetDeviceInfo() openolt.DeviceInfo {
	var tdi openolt.DeviceInfo

	tdi.Vendor = "TestVendor"
	tdi.Model = "TestModel"
	tdi.HardwareVersion = "TestVersion"
	tdi.FirmwareVersion = "TestFirmWareVersion"
	tdi.DeviceId = "TestID"
	tdi.DeviceSerialNumber = "TestSerialNumber"
	tdi.PonPorts = 16

	return tdi
}
func setup() {
	DevInfo := GetDeviceInfo()
	rm.KVStoreHostPort = "localhost:8500"
	rm.KVStoreType = "Consul"
	rm.DeviceType = "OpenOlt"
	rm.ResourceMgr = rmgr.NewResourceMgr("TestID", rm.KVStoreHostPort,
		rm.KVStoreType, rm.DeviceType, &DevInfo)
}

func shutdown() {
	rm.ResourceMgr = nil
}

/* TODO: Implemention of Test Cases for the adaptercore

   Empty test cases has been added to make sure the
   CI build does not fail. These test cases will be
   added at a later stage
*/
func TestGetONUID(t *testing.T) {}

func TestGetFlowIDInfo(t *testing.T) {}

func TestGetCurrentFlowIDsForOnu(t *testing.T) {}

func TestUpdateFlowIDInfo(t *testing.T) {}

func TestGetFlowID(t *testing.T) {}

func TestGetAllocID(t *testing.T) {}

func TestUpdateAllocIdsForOnu(t *testing.T) {}

func TestGetCurrentGEMPortIDsForOnu(t *testing.T) {}

func TestGetCurrentAllocIDForOnu(t *testing.T) {}

func TestUpdateGEMportsPonportToOnuMapOnKVStore(t *testing.T) {}

func TestGetONUUNIfromPONPortGEMPort(t *testing.T) {}

func TestGetGEMPortID(t *testing.T) {}

func TestUpdateGEMPortIDsForOnu(t *testing.T) {}

func TestFreeONUID(t *testing.T) {}

func TestFreeFlowID(t *testing.T) {}

func TestFreePONResourcesForONU(t *testing.T) {}

func TestIsFlowCookieOnKVStore(t *testing.T) {}
