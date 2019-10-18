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

//Package adaptercore provides the utility for olt devices, flows and statistics
package adaptercore

import (
	"errors"
	"fmt"

	"github.com/opencord/voltha-lib-go/pkg/log"
	"github.com/opencord/voltha-protos/go/openolt"
	"github.com/opencord/voltha-protos/go/voltha"
)

// PonPort representation
type PonPort struct {
	/*
	   This is a highly reduced version taken from the adtran pon_port.
	   TODO: Extend for use in the openolt adapter set.
	*/
	/*    MAX_ONUS_SUPPORTED = 256
	      DEFAULT_ENABLED = False
	      MAX_DEPLOYMENT_RANGE = 25000  # Meters (OLT-PB maximum)

	      _MCAST_ONU_ID = 253
	      _MCAST_ALLOC_BASE = 0x500

	      _SUPPORTED_ACTIVATION_METHODS = ['autodiscovery']  # , 'autoactivate']
	      _SUPPORTED_AUTHENTICATION_METHODS = ['serial-number']
	*/
	PONID    uint32
	DeviceID string
	IntfID   uint32
	PortNum  uint32
	PortID   uint32
	Label    string
	ONUs     map[uint32]interface{}
	ONUsByID map[uint32]interface{}

	RxBytes        uint64
	RxPackets      uint64
	RxMcastPackets uint64
	RxBcastPackets uint64
	RxErrorPackets uint64
	TxBytes        uint64
	TxPackets      uint64
	TxUcastPackets uint64
	TxMcastPackets uint64
	TxBcastPackets uint64
	TxErrorPackets uint64
}

// NewPONPort returns a new instance of PonPort initialized with given PONID, DeviceID, IntfID and PortNum
func NewPONPort(PONID uint32, DeviceID string, IntfID uint32, PortNum uint32) *PonPort {

	var PON PonPort

	PON.PONID = PONID
	PON.DeviceID = DeviceID
	PON.IntfID = IntfID
	PON.PortNum = PortNum
	PON.PortID = 0
	PON.Label = fmt.Sprintf("%s,%d", "pon-", PONID)

	PON.ONUs = make(map[uint32]interface{})
	PON.ONUsByID = make(map[uint32]interface{})

	/*
	   Statistics  taken from nni_port
	   self.intf_id = 0  #handled by getter
	   self.port_no = 0  #handled by getter
	   self.port_id = 0  #handled by getter

	   Note:  In the current implementation of the kpis coming from the BAL the stats are the
	   samne model for NNI and PON.

	   TODO:   Integrate additional kpis for the PON and other southbound port objecgts.

	*/

	PON.RxBytes = 0
	PON.RxPackets = 0
	PON.RxMcastPackets = 0
	PON.RxBcastPackets = 0
	PON.RxErrorPackets = 0
	PON.TxBytes = 0
	PON.TxPackets = 0
	PON.TxUcastPackets = 0
	PON.TxMcastPackets = 0
	PON.TxBcastPackets = 0
	PON.TxErrorPackets = 0

	/*    def __str__(self):
	      return "PonPort-{}: Admin: {}, Oper: {}, OLT: {}".format(self._label,
	                                                               self._admin_state,
	                                                               self._oper_status,
	                                                               self.olt)
	*/
	return &PON
}

// NniPort representation
type NniPort struct {
	/*
	   Northbound network port, often Ethernet-based

	   This is a highly reduced version taken from the adtran nni_port code set
	   TODO:   add functions to allow for port specific values and operations
	*/
	PortNum     uint32
	Name        string
	LogicalPort uint32
	IntfID      uint32

	RxBytes        uint64
	RxPackets      uint64
	RxMcastPackets uint64
	RxBcastPackets uint64
	RxErrorPackets uint64
	TxBytes        uint64
	TxPackets      uint64
	TxUcastPackets uint64
	TxMcastPackets uint64
	TxBcastPackets uint64
	TxErrorPackets uint64
}

// NewNniPort returns a new instance of NniPort initialized with the given PortNum and IntfID
func NewNniPort(PortNum uint32, IntfID uint32) *NniPort {

	var NNI NniPort

	NNI.PortNum = PortNum
	NNI.Name = fmt.Sprintf("%s,%d", "nni-", PortNum)
	NNI.IntfID = IntfID

	NNI.RxBytes = 0
	NNI.RxPackets = 0
	NNI.RxMcastPackets = 0
	NNI.RxBcastPackets = 0
	NNI.RxErrorPackets = 0
	NNI.TxBytes = 0
	NNI.TxPackets = 0
	NNI.TxUcastPackets = 0
	NNI.TxMcastPackets = 0
	NNI.TxBcastPackets = 0
	NNI.TxErrorPackets = 0

	return &NNI
}

// OpenOltStatisticsMgr structure
type OpenOltStatisticsMgr struct {
	Device         *DeviceHandler
	NorthBoundPort map[uint32]NniPort
	SouthBoundPort map[uint32]PonPort
	// TODO  PMMetrics Metrics
}

// NewOpenOltStatsMgr returns a new instance of the OpenOltStatisticsMgr
func NewOpenOltStatsMgr(Dev *DeviceHandler) *OpenOltStatisticsMgr {

	var StatMgr OpenOltStatisticsMgr

	StatMgr.Device = Dev
	// TODO call metric PMMetric =
	// Northbound and Southbound ports
	// added to initialize the pm_metrics
	var Ports interface{}
	Ports, _ = InitPorts("nni", Dev.deviceID)
	StatMgr.NorthBoundPort, _ = Ports.(map[uint32]NniPort)
	Ports, _ = InitPorts("pon", Dev.deviceID)
	StatMgr.SouthBoundPort, _ = Ports.(map[uint32]PonPort)

	return &StatMgr
}

// InitPorts collects the port objects:  nni and pon that are updated with the current data from the OLT
func InitPorts(Intftype string, DeviceID string) (interface{}, error) {
	/*
	     This method collects the port objects:  nni and pon that are updated with the
	     current data from the OLT

	     Both the northbound (nni) and southbound ports are indexed by the interface id (intf_id)
	     and NOT the port number. When the port object is instantiated it will contain the intf_id and
	     port_no values

	   :param type:
	   :return:
	*/
	var i uint32
	if Intftype == "nni" {
		NniPorts := make(map[uint32]NniPort)
		for i = 0; i <= 1; i++ {
			Port := BuildPortObject(i, "nni", DeviceID).(*NniPort)
			NniPorts[Port.IntfID] = *Port
		}
		return NniPorts, nil
	} else if Intftype == "pon" {
		PONPorts := make(map[uint32]PonPort)
		for i = 0; i <= 16; i++ {
			PONPort := BuildPortObject(i, "pon", DeviceID).(*PonPort)
			PONPorts[PONPort.IntfID] = *PONPort
		}
		return PONPorts, nil
	} else {
		log.Errorf("Invalid type of interface %s", Intftype)
		return nil, errors.New("invalid type of interface ")
	}
}

// BuildPortObject allows for updating north and southbound ports, newly discovered ports, and devices
func BuildPortObject(PortNum uint32, IntfType string, DeviceID string) interface{} {
	/*
	   Separate method to allow for updating north and southbound ports
	   newly discovered ports and devices

	   :param port_num:
	   :param type:
	   :return:
	*/

	//This builds a port object which is added to the
	//appropriate northbound or southbound values
	if IntfType == "nni" {
		IntfID := IntfIDToPortNo(PortNum, voltha.Port_ETHERNET_UNI)
		return NewNniPort(PortNum, IntfID)
	} else if IntfType == "pon" {
		// PON ports require a different configuration
		//  intf_id and pon_id are currently equal.
		IntfID := IntfIDToPortNo(PortNum, voltha.Port_ETHERNET_NNI)
		PONID := IntfID
		return NewPONPort(PONID, DeviceID, IntfID, PortNum)
	} else {
		log.Errorf("Invalid type of interface %s", IntfType)
		return nil
	}
}

// PortStatisticsIndication handles the port statistics indication
func (StatMgr *OpenOltStatisticsMgr) PortStatisticsIndication(PortStats *openolt.PortStatistics) {
	log.Debugf("port-stats-collected %v", PortStats)
	StatMgr.PortsStatisticsKpis(PortStats)
	// TODO send stats to core topic to the voltha kafka or a different kafka ?
}

// FlowStatisticsIndication to be implemented
func FlowStatisticsIndication(self, FlowStats *openolt.FlowStatistics) {
	log.Debugf("flow-stats-collected %v", FlowStats)
	//TODO send to kafka ?
}

// PortsStatisticsKpis map the port stats values into a dictionary, creates the kpiEvent and then publish to Kafka
func (StatMgr *OpenOltStatisticsMgr) PortsStatisticsKpis(PortStats *openolt.PortStatistics) {

	/*map the port stats values into a dictionary
	  Create a kpoEvent and publish to Kafka

	  :param port_stats:
	  :return:
	*/
	//var err error
	IntfID := PortStats.IntfId

	if (IntfIDToPortNo(0, voltha.Port_ETHERNET_NNI) < IntfID) &&
		(IntfID < IntfIDToPortNo(4, voltha.Port_ETHERNET_NNI)) {
		/*
		   for this release we are only interested in the first NNI for
		   Northbound.
		   we are not using the other 3
		*/
		return
	}
	PMData := make(map[string]uint64)
	PMData["rx_bytes"] = PortStats.RxBytes
	PMData["rx_packets"] = PortStats.RxPackets
	PMData["rx_ucast_packets"] = PortStats.RxUcastPackets
	PMData["rx_mcast_packets"] = PortStats.RxMcastPackets
	PMData["rx_bcast_packets"] = PortStats.RxBcastPackets
	PMData["rx_error_packets"] = PortStats.RxErrorPackets
	PMData["tx_bytes"] = PortStats.TxBytes
	PMData["tx_packets"] = PortStats.TxPackets
	PMData["tx_ucast_packets"] = PortStats.TxUcastPackets
	PMData["tx_mcast_packets"] = PortStats.TxMcastPackets
	PMData["tx_bcast_packets"] = PortStats.TxBcastPackets
	PMData["tx_error_packets"] = PortStats.TxErrorPackets
	PMData["rx_crc_errors"] = PortStats.RxCrcErrors
	PMData["bip_errors"] = PortStats.BipErrors

	PMData["intf_id"] = uint64(PortStats.IntfId)

	/*
	   Based upon the intf_id map to an nni port or a pon port
	   the intf_id is the key to the north or south bound collections

	   Based upon the intf_id the port object (nni_port or pon_port) will
	   have its data attr. updated by the current dataset collected.

	   For prefixing the rule is currently to use the port number and not the intf_id
	*/
	//FIXME : Just use first NNI for now
	/* TODO should the data be marshaled before sending it ?
	   if IntfID == IntfIdToPortNo(0, voltha.Port_ETHERNET_NNI) {
	       //NNI port (just the first one)
	       err = UpdatePortObjectKpiData(StatMgr.NorthBoundPorts[PortStats.IntfID], PMData)
	   } else {
	       //PON ports
	       err = UpdatePortObjectKpiData(SouthboundPorts[PortStats.IntfID], PMData)
	   }
	   if (err != nil) {
	       log.Error("Error publishing statistics data")
	   }
	*/

}
