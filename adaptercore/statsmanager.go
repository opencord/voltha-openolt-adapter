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
	"sync"
	"time"

	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	"github.com/opencord/voltha-protos/v3/go/openolt"
	"github.com/opencord/voltha-protos/v3/go/voltha"
)

var mutex = &sync.Mutex{}

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
	RxUcastPackets uint64
	RxMcastPackets uint64
	RxBcastPackets uint64
	RxErrorPackets uint64
	TxBytes        uint64
	TxPackets      uint64
	TxUcastPackets uint64
	TxMcastPackets uint64
	TxBcastPackets uint64
	TxErrorPackets uint64
	RxCrcErrors    uint64
	BipErrors      uint64
}

// NewPONPort returns a new instance of PonPort initialized with given PONID, DeviceID, IntfID and PortNum
func NewPONPort(PONID uint32, DeviceID string, IntfID uint32, PortNum uint32) *PonPort {

	var PON PonPort

	PON.PONID = PONID
	PON.DeviceID = DeviceID
	PON.IntfID = IntfID
	PON.PortNum = PortNum
	PON.PortID = 0
	PON.Label = fmt.Sprintf("%s%d", "pon-", PONID)

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
	PON.RxUcastPackets = 0
	PON.RxMcastPackets = 0
	PON.RxBcastPackets = 0
	PON.RxErrorPackets = 0
	PON.TxBytes = 0
	PON.TxPackets = 0
	PON.TxUcastPackets = 0
	PON.TxMcastPackets = 0
	PON.TxBcastPackets = 0
	PON.TxErrorPackets = 0
	PON.RxCrcErrors = 0
	PON.BipErrors = 0

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
	RxUcastPackets uint64
	RxMcastPackets uint64
	RxBcastPackets uint64
	RxErrorPackets uint64
	TxBytes        uint64
	TxPackets      uint64
	TxUcastPackets uint64
	TxMcastPackets uint64
	TxBcastPackets uint64
	TxErrorPackets uint64
	RxCrcErrors    uint64
	BipErrors      uint64
}

// NewNniPort returns a new instance of NniPort initialized with the given PortNum and IntfID
func NewNniPort(PortNum uint32, IntfID uint32) *NniPort {

	var NNI NniPort

	NNI.PortNum = PortNum
	NNI.Name = fmt.Sprintf("%s%d", "nni-", PortNum)
	NNI.IntfID = IntfID

	NNI.RxBytes = 0
	NNI.RxPackets = 0
	NNI.RxUcastPackets = 0
	NNI.RxMcastPackets = 0
	NNI.RxBcastPackets = 0
	NNI.RxErrorPackets = 0
	NNI.TxBytes = 0
	NNI.TxPackets = 0
	NNI.TxUcastPackets = 0
	NNI.TxMcastPackets = 0
	NNI.TxBcastPackets = 0
	NNI.TxErrorPackets = 0
	NNI.RxCrcErrors = 0
	NNI.BipErrors = 0

	return &NNI
}

// OpenOltStatisticsMgr structure
type OpenOltStatisticsMgr struct {
	Device         *DeviceHandler
	NorthBoundPort map[uint32]*NniPort
	SouthBoundPort map[uint32]*PonPort
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
	Ports, _ = InitPorts("nni", Dev.deviceID, 1)
	StatMgr.NorthBoundPort, _ = Ports.(map[uint32]*NniPort)
	NumPonPorts := Dev.resourceMgr.DevInfo.GetPonPorts()
	Ports, _ = InitPorts("pon", Dev.deviceID, NumPonPorts)
	StatMgr.SouthBoundPort, _ = Ports.(map[uint32]*PonPort)
	return &StatMgr
}

// InitPorts collects the port objects:  nni and pon that are updated with the current data from the OLT
func InitPorts(Intftype string, DeviceID string, numOfPorts uint32) (interface{}, error) {
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
		NniPorts := make(map[uint32]*NniPort)
		for i = 0; i < numOfPorts; i++ {
			Port := BuildPortObject(i, "nni", DeviceID).(*NniPort)
			NniPorts[Port.IntfID] = Port
		}
		return NniPorts, nil
	} else if Intftype == "pon" {
		PONPorts := make(map[uint32]*PonPort)
		for i = 0; i < numOfPorts; i++ {
			PONPort := BuildPortObject(i, "pon", DeviceID).(*PonPort)
			PONPorts[PortNoToIntfID(PONPort.IntfID, voltha.Port_PON_OLT)] = PONPort
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
		IntfID := IntfIDToPortNo(PortNum, voltha.Port_ETHERNET_NNI)
		nniID := PortNoToIntfID(IntfID, voltha.Port_ETHERNET_NNI)
		log.Debugf("NniID %v", nniID)
		return NewNniPort(PortNum, nniID)
	} else if IntfType == "pon" {
		// PON ports require a different configuration
		//  intf_id and pon_id are currently equal.
		IntfID := IntfIDToPortNo(PortNum, voltha.Port_PON_OLT)
		PONID := PortNoToIntfID(IntfID, voltha.Port_PON_OLT)
		log.Debugf("PonID %v", PONID)
		return NewPONPort(PONID, DeviceID, IntfID, PortNum)
	} else {
		log.Errorf("Invalid type of interface %s", IntfType)
		return nil
	}
}

// collectNNIMetrics will collect the nni port metrics
func (StatMgr *OpenOltStatisticsMgr) collectNNIMetrics(nniID uint32) map[string]float32 {

	nnival := make(map[string]float32)
	mutex.Lock()
	cm := StatMgr.Device.portStats.NorthBoundPort[nniID]
	mutex.Unlock()
	metricName := StatMgr.Device.metrics.GetSubscriberMetrics()

	if metricName != nil && len(metricName) > 0 {
		for mName := range metricName {
			switch mName {
			case "rx_bytes":
				nnival["RxBytes"] = float32(cm.RxBytes)
			case "rx_packets":
				nnival["RxPackets"] = float32(cm.RxPackets)
			case "rx_ucast_packets":
				nnival["RxUcastPackets"] = float32(cm.RxUcastPackets)
			case "rx_mcast_packets":
				nnival["RxMcastPackets"] = float32(cm.RxMcastPackets)
			case "rx_bcast_packets":
				nnival["RxBcastPackets"] = float32(cm.RxBcastPackets)
			case "tx_bytes":
				nnival["TxBytes"] = float32(cm.TxBytes)
			case "tx_packets":
				nnival["TxPackets"] = float32(cm.TxPackets)
			case "tx_mcast_packets":
				nnival["TxMcastPackets"] = float32(cm.TxMcastPackets)
			case "tx_bcast_packets":
				nnival["TxBcastPackets"] = float32(cm.TxBcastPackets)
			}
		}
	}
	return nnival
}

// collectPONMetrics will collect the pon port metrics
func (StatMgr *OpenOltStatisticsMgr) collectPONMetrics(pID uint32) map[string]float32 {

	ponval := make(map[string]float32)
	mutex.Lock()
	cm := StatMgr.Device.portStats.SouthBoundPort[pID]
	mutex.Unlock()
	metricName := StatMgr.Device.metrics.GetSubscriberMetrics()

	if metricName != nil && len(metricName) > 0 {
		for mName := range metricName {
			switch mName {
			case "rx_bytes":
				ponval["RxBytes"] = float32(cm.RxBytes)
			case "rx_packets":
				ponval["RxPackets"] = float32(cm.RxPackets)
			// these are not supported in OpenOlt Agent now
			// will return zero until supported
			case "rx_ucast_packets":
				ponval["RxUcastPackets"] = float32(cm.RxUcastPackets)
			case "rx_mcast_packets":
				ponval["RxMcastPackets"] = float32(cm.RxMcastPackets)
			case "rx_bcast_packets":
				ponval["RxBcastPackets"] = float32(cm.RxBcastPackets)
			// End will return zero until supported
			case "tx_bytes":
				ponval["TxBytes"] = float32(cm.TxBytes)
			case "tx_packets":
				ponval["TxPackets"] = float32(cm.TxPackets)
			// these are not supported in OpenOlt Agent now
			// will return zero until supported
			case "tx_ucast_packets":
				ponval["TxUcastPackets"] = float32(cm.TxUcastPackets)
			case "tx_mcast_packets":
				ponval["TxMcastPackets"] = float32(cm.TxMcastPackets)
			case "tx_bcast_packets":
				ponval["TxBcastPackets"] = float32(cm.TxBcastPackets)
			}
		}
	}
	return ponval
}

// publishMatrics will publish the pon port metrics
func (StatMgr OpenOltStatisticsMgr) publishMetrics(portType string, val map[string]float32, portnum uint32, context map[string]string, devID string) {
	log.Debugf("Post-%v %v", portType, val)

	var metricInfo voltha.MetricInformation
	var ke voltha.KpiEvent2
	var volthaEventSubCatgry voltha.EventSubCategory_Types

	if portType == "NNIStats" {
		volthaEventSubCatgry = voltha.EventSubCategory_NNI
	} else {
		volthaEventSubCatgry = voltha.EventSubCategory_PON
	}

	raisedTs := time.Now().UnixNano()
	mmd := voltha.MetricMetaData{
		Title:    portType,
		Ts:       float64(raisedTs),
		Context:  context,
		DeviceId: devID,
	}

	metricInfo.Metadata = &mmd
	metricInfo.Metrics = val

	ke.SliceData = []*voltha.MetricInformation{&metricInfo}
	ke.Type = voltha.KpiEventType_slice
	ke.Ts = float64(time.Now().UnixNano())

	if err := StatMgr.Device.EventProxy.SendKpiEvent("STATS_EVENT", &ke, voltha.EventCategory_EQUIPMENT, volthaEventSubCatgry, raisedTs); err != nil {
		log.Errorw("Failed to send Pon stats", log.Fields{"err": err})
	}

}

// PortStatisticsIndication handles the port statistics indication
func (StatMgr *OpenOltStatisticsMgr) PortStatisticsIndication(PortStats *openolt.PortStatistics, NumPonPorts uint32) {
	log.Debugf("port-stats-collected %v", PortStats)
	StatMgr.PortsStatisticsKpis(PortStats, NumPonPorts)
	log.Infow("Received port stats indication", log.Fields{"PortStats": PortStats})
	// TODO send stats to core topic to the voltha kafka or a different kafka ?
}

// FlowStatisticsIndication to be implemented
func FlowStatisticsIndication(self, FlowStats *openolt.FlowStatistics) {
	log.Debugf("flow-stats-collected %v", FlowStats)
	//TODO send to kafka ?
}

// PortsStatisticsKpis map the port stats values into a dictionary, creates the kpiEvent and then publish to Kafka
func (StatMgr *OpenOltStatisticsMgr) PortsStatisticsKpis(PortStats *openolt.PortStatistics, NumPonPorts uint32) {

	/*map the port stats values into a dictionary
	  Create a kpoEvent and publish to Kafka

	  :param port_stats:
	  :return:
	*/
	//var err error
	IntfID := PortStats.IntfId

	if (IntfIDToPortNo(1, voltha.Port_ETHERNET_NNI) < IntfID) &&
		(IntfID < IntfIDToPortNo(4, voltha.Port_ETHERNET_NNI)) {
		/*
		   for this release we are only interested in the first NNI for
		   Northbound.
		   we are not using the other 3
		*/
		return
	} else if IntfIDToPortNo(0, voltha.Port_ETHERNET_NNI) == IntfID {

		var portNNIStat NniPort
		portNNIStat.IntfID = IntfID
		portNNIStat.PortNum = uint32(0)
		portNNIStat.RxBytes = PortStats.RxBytes
		portNNIStat.RxPackets = PortStats.RxPackets
		portNNIStat.RxUcastPackets = PortStats.RxUcastPackets
		portNNIStat.RxMcastPackets = PortStats.RxMcastPackets
		portNNIStat.RxBcastPackets = PortStats.RxBcastPackets
		portNNIStat.TxBytes = PortStats.TxBytes
		portNNIStat.TxPackets = PortStats.TxPackets
		portNNIStat.TxUcastPackets = PortStats.TxUcastPackets
		portNNIStat.TxMcastPackets = PortStats.TxMcastPackets
		portNNIStat.TxBcastPackets = PortStats.TxBcastPackets
		mutex.Lock()
		StatMgr.NorthBoundPort[0] = &portNNIStat
		mutex.Unlock()
		log.Debugf("Received-NNI-Stats: %v", StatMgr.NorthBoundPort)
	}
	for i := uint32(0); i < NumPonPorts; i++ {

		if IntfIDToPortNo(i, voltha.Port_PON_OLT) == IntfID {
			var portPonStat PonPort
			portPonStat.IntfID = IntfID
			portPonStat.PortNum = i
			portPonStat.PONID = i
			portPonStat.RxBytes = PortStats.RxBytes
			portPonStat.RxPackets = PortStats.RxPackets
			portPonStat.RxUcastPackets = PortStats.RxUcastPackets
			portPonStat.RxMcastPackets = PortStats.RxMcastPackets
			portPonStat.RxBcastPackets = PortStats.RxBcastPackets
			portPonStat.TxBytes = PortStats.TxBytes
			portPonStat.TxPackets = PortStats.TxPackets
			portPonStat.TxUcastPackets = PortStats.TxUcastPackets
			portPonStat.TxMcastPackets = PortStats.TxMcastPackets
			portPonStat.TxBcastPackets = PortStats.TxBcastPackets
			mutex.Lock()
			StatMgr.SouthBoundPort[i] = &portPonStat
			mutex.Unlock()
			log.Debugf("Received-PON-Stats-for-Port %v : %v", i, StatMgr.SouthBoundPort[i])
		}
	}

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
