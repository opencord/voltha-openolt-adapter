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

//Package core provides the utility for olt devices, flows and statistics
package core

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"

	plt "github.com/opencord/voltha-lib-go/v7/pkg/platform"
	rsrcMgr "github.com/opencord/voltha-openolt-adapter/internal/pkg/resourcemanager"

	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	"github.com/opencord/voltha-protos/v5/go/extension"
	"github.com/opencord/voltha-protos/v5/go/openolt"
	"github.com/opencord/voltha-protos/v5/go/voltha"
)

const (
	//NNIStats statType constant
	NNIStats = "NNIStats"
	//PONStats statType constant
	PONStats = "PONStats"
	//ONUStats statType constant
	ONUStats = "ONUStats"
	//GEMStats statType constant
	GEMStats = "GEMStats"

	//RxBytes constant
	RxBytes = "RxBytes"
	//RxPackets constant
	RxPackets = "RxPackets"
	//TxBytes constant
	TxBytes = "TxBytes"
	//TxPackets constant
	TxPackets = "TxPackets"
	//FecCodewords constant
	FecCodewords = "FecCodewords"
	//BipUnits constant
	BipUnits = "BipUnits"
	//BipErrors constant
	BipErrors = "BipErrors"
	//RxPloamsNonIdle constant
	RxPloamsNonIdle = "RxPloamsNonIdle"
	//RxPloamsError constant
	RxPloamsError = "RxPloamsError"
	//RxOmci constant
	RxOmci = "RxOmci"
	//RxOmciPacketsCrcError constant
	RxOmciPacketsCrcError = "RxOmciPacketsCrcError"
	//PositiveDrift constant
	PositiveDrift = "PositiveDrift"
	//NegativeDrift constant
	NegativeDrift = "NegativeDrift"
	//DelimiterMissDetection constant
	DelimiterMissDetection = "DelimiterMissDetection"
	//FecCorrectedSymbols constant
	FecCorrectedSymbols = "FecCorrectedSymbols"
	//FecCodewordsCorrected constant
	FecCodewordsCorrected = "FecCodewordsCorrected"
	//fecCodewordsUncorrectable constant
	fecCodewordsUncorrectable = "fec_codewords_uncorrectable"
	//FecCorrectedUnits constant
	FecCorrectedUnits = "FecCorrectedUnits"
	//XGEMKeyErrors constant
	XGEMKeyErrors = "XGEMKeyErrors"
	//XGEMLoss constant
	XGEMLoss = "XGEMLOSS"
	//BerReported constant
	BerReported = "BerReported"
	//LcdgErrors constant
	LcdgErrors = "LcdgErrors"
	//RdiErrors constant
	RdiErrors = "RdiErrors"
	//Timestamp constant
	Timestamp = "Timestamp"
)

var mutex = &sync.Mutex{}

var onuStats = make(chan *openolt.OnuStatistics, 100)
var gemStats = make(chan *openolt.GemPortStatistics, 100)

//statRegInfo is used to register for notifications
//on receiving port stats and flow stats indication
type statRegInfo struct {
	chn      chan bool
	portNo   uint32
	portType extension.GetOltPortCounters_PortType
}

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

//StatType defines portStatsType and flowStatsType types
type StatType int

const (
	portStatsType StatType = iota
	flowStatsType
)

// OpenOltStatisticsMgr structure
type OpenOltStatisticsMgr struct {
	Device         *DeviceHandler
	NorthBoundPort map[uint32]*NniPort
	SouthBoundPort map[uint32]*PonPort
	// TODO  PMMetrics Metrics
	//statIndListners is the list of requests to be notified when port and flow stats indication is received
	statIndListnerMu sync.Mutex
	statIndListners  map[StatType]*list.List
}

// NewOpenOltStatsMgr returns a new instance of the OpenOltStatisticsMgr
func NewOpenOltStatsMgr(ctx context.Context, Dev *DeviceHandler) *OpenOltStatisticsMgr {

	var StatMgr OpenOltStatisticsMgr

	StatMgr.Device = Dev
	// TODO call metric PMMetric =
	// Northbound and Southbound ports
	// added to initialize the pm_metrics
	var Ports interface{}
	Ports, _ = InitPorts(ctx, "nni", Dev.device.Id, 1)
	StatMgr.NorthBoundPort, _ = Ports.(map[uint32]*NniPort)
	NumPonPorts := Dev.resourceMgr[0].DevInfo.GetPonPorts()
	Ports, _ = InitPorts(ctx, "pon", Dev.device.Id, NumPonPorts)
	StatMgr.SouthBoundPort, _ = Ports.(map[uint32]*PonPort)
	if StatMgr.Device.openOLT.enableONUStats {
		go StatMgr.publishOnuStats()
	}
	if StatMgr.Device.openOLT.enableGemStats {
		go StatMgr.publishGemStats()
	}
	StatMgr.statIndListners = make(map[StatType]*list.List)
	StatMgr.statIndListners[portStatsType] = list.New()
	StatMgr.statIndListners[flowStatsType] = list.New()
	return &StatMgr
}

// InitPorts collects the port objects:  nni and pon that are updated with the current data from the OLT
func InitPorts(ctx context.Context, Intftype string, DeviceID string, numOfPorts uint32) (interface{}, error) {
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
			Port := BuildPortObject(ctx, i, "nni", DeviceID).(*NniPort)
			NniPorts[Port.IntfID] = Port
		}
		return NniPorts, nil
	} else if Intftype == "pon" {
		PONPorts := make(map[uint32]*PonPort)
		for i = 0; i < numOfPorts; i++ {
			PONPort := BuildPortObject(ctx, i, "pon", DeviceID).(*PonPort)
			PONPorts[plt.PortNoToIntfID(PONPort.IntfID, voltha.Port_PON_OLT)] = PONPort
		}
		return PONPorts, nil
	} else {
		logger.Errorw(ctx, "invalid-type-of-interface", log.Fields{"interface-type": Intftype})
		return nil, olterrors.NewErrInvalidValue(log.Fields{"interface-type": Intftype}, nil)
	}
}

// BuildPortObject allows for updating north and southbound ports, newly discovered ports, and devices
func BuildPortObject(ctx context.Context, PortNum uint32, IntfType string, DeviceID string) interface{} {
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
		IntfID := plt.IntfIDToPortNo(PortNum, voltha.Port_ETHERNET_NNI)
		nniID := plt.PortNoToIntfID(IntfID, voltha.Port_ETHERNET_NNI)
		logger.Debugw(ctx, "interface-type-nni",
			log.Fields{
				"nni-id":    nniID,
				"intf-type": IntfType})
		return NewNniPort(PortNum, nniID)
	} else if IntfType == "pon" {
		// PON ports require a different configuration
		//  intf_id and pon_id are currently equal.
		IntfID := plt.IntfIDToPortNo(PortNum, voltha.Port_PON_OLT)
		PONID := plt.PortNoToIntfID(IntfID, voltha.Port_PON_OLT)
		logger.Debugw(ctx, "interface-type-pon",
			log.Fields{
				"pon-id":    PONID,
				"intf-type": IntfType})
		return NewPONPort(PONID, DeviceID, IntfID, PortNum)
	} else {
		logger.Errorw(ctx, "invalid-type-of-interface", log.Fields{"intf-type": IntfType})
		return nil
	}
}

// collectNNIMetrics will collect the nni port metrics
func (StatMgr *OpenOltStatisticsMgr) collectNNIMetrics(nniID uint32) map[string]float32 {

	nnival := make(map[string]float32)
	mutex.Lock()
	cm := StatMgr.Device.portStats.NorthBoundPort[nniID]
	mutex.Unlock()
	metricNames := StatMgr.Device.metrics.GetSubscriberMetrics()

	var metrics []string
	for metric := range metricNames {
		if metricNames[metric].Enabled {
			metrics = append(metrics, metric)
		}
	}

	for _, mName := range metrics {
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
	return nnival
}

// collectPONMetrics will collect the pon port metrics
func (StatMgr *OpenOltStatisticsMgr) collectPONMetrics(pID uint32) map[string]float32 {

	ponval := make(map[string]float32)
	mutex.Lock()
	cm := StatMgr.Device.portStats.SouthBoundPort[pID]
	mutex.Unlock()
	metricNames := StatMgr.Device.metrics.GetSubscriberMetrics()

	var metrics []string
	for metric := range metricNames {
		if metricNames[metric].Enabled {
			metrics = append(metrics, metric)
		}
	}

	for _, mName := range metrics {
		switch mName {
		case "rx_bytes":
			ponval["RxBytes"] = float32(cm.RxBytes)
		case "rx_packets":
			ponval["RxPackets"] = float32(cm.RxPackets)
		case "rx_ucast_packets":
			ponval["RxUcastPackets"] = float32(cm.RxUcastPackets)
		case "rx_mcast_packets":
			ponval["RxMcastPackets"] = float32(cm.RxMcastPackets)
		case "rx_bcast_packets":
			ponval["RxBcastPackets"] = float32(cm.RxBcastPackets)
		case "tx_bytes":
			ponval["TxBytes"] = float32(cm.TxBytes)
		case "tx_packets":
			ponval["TxPackets"] = float32(cm.TxPackets)
		case "tx_mcast_packets":
			ponval["TxMcastPackets"] = float32(cm.TxMcastPackets)
		case "tx_bcast_packets":
			ponval["TxBcastPackets"] = float32(cm.TxBcastPackets)
		}
	}

	return ponval
}

// converGemStats will convert gem stats response to kpi context
func (StatMgr *OpenOltStatisticsMgr) convertGemStats(gemStats *openolt.GemPortStatistics) map[string]float32 {
	gemStatsVal := make(map[string]float32)
	gemStatsVal[IntfID] = float32(gemStats.IntfId)
	gemStatsVal[GemID] = float32(gemStats.GemportId)
	gemStatsVal[RxPackets] = float32(gemStats.RxPackets)
	gemStatsVal[RxBytes] = float32(gemStats.RxBytes)
	gemStatsVal[TxPackets] = float32(gemStats.TxPackets)
	gemStatsVal[TxBytes] = float32(gemStats.TxBytes)
	return gemStatsVal
}

// convertONUStats will convert onu stats response to kpi context
func (StatMgr *OpenOltStatisticsMgr) convertONUStats(onuStats *openolt.OnuStatistics) map[string]float32 {
	onuStatsVal := make(map[string]float32)
	onuStatsVal[IntfID] = float32(onuStats.IntfId)
	onuStatsVal[OnuID] = float32(onuStats.OnuId)
	onuStatsVal[PositiveDrift] = float32(onuStats.PositiveDrift)
	onuStatsVal[NegativeDrift] = float32(onuStats.NegativeDrift)
	onuStatsVal[DelimiterMissDetection] = float32(onuStats.DelimiterMissDetection)
	onuStatsVal[BipErrors] = float32(onuStats.BipErrors)
	onuStatsVal[BipUnits] = float32(onuStats.BipUnits)
	onuStatsVal[FecCorrectedSymbols] = float32(onuStats.FecCorrectedSymbols)
	onuStatsVal[FecCodewordsCorrected] = float32(onuStats.FecCodewordsCorrected)
	onuStatsVal[fecCodewordsUncorrectable] = float32(onuStats.FecCodewordsUncorrectable)
	onuStatsVal[FecCodewords] = float32(onuStats.FecCodewords)
	onuStatsVal[FecCorrectedUnits] = float32(onuStats.FecCorrectedUnits)
	onuStatsVal[XGEMKeyErrors] = float32(onuStats.XgemKeyErrors)
	onuStatsVal[XGEMLoss] = float32(onuStats.XgemLoss)
	onuStatsVal[RxPloamsError] = float32(onuStats.RxPloamsError)
	onuStatsVal[RxPloamsNonIdle] = float32(onuStats.RxPloamsNonIdle)
	onuStatsVal[RxOmci] = float32(onuStats.RxOmci)
	onuStatsVal[RxOmciPacketsCrcError] = float32(onuStats.RxOmciPacketsCrcError)
	onuStatsVal[RxBytes] = float32(onuStats.RxBytes)
	onuStatsVal[RxPackets] = float32(onuStats.RxPackets)
	onuStatsVal[TxBytes] = float32(onuStats.TxBytes)
	onuStatsVal[TxPackets] = float32(onuStats.TxPackets)
	onuStatsVal[BerReported] = float32(onuStats.BerReported)
	onuStatsVal[LcdgErrors] = float32(onuStats.LcdgErrors)
	onuStatsVal[RdiErrors] = float32(onuStats.RdiErrors)
	onuStatsVal[Timestamp] = float32(onuStats.Timestamp)
	return onuStatsVal
}

// collectOnuStats will collect the onu metrics
func (StatMgr *OpenOltStatisticsMgr) collectOnuStats(ctx context.Context, onuGemInfo rsrcMgr.OnuGemInfo) {
	onu := &openolt.Onu{IntfId: onuGemInfo.IntfID, OnuId: onuGemInfo.OnuID}
	logger.Debugw(ctx, "pulling-onu-stats", log.Fields{"IntfID": onuGemInfo.IntfID, "OnuID": onuGemInfo.OnuID})
	if stats, err := StatMgr.Device.Client.GetOnuStatistics(context.Background(), onu); err == nil {
		onuStats <- stats
	} else {
		logger.Errorw(ctx, "error-while-getting-onu-stats-for-onu", log.Fields{"IntfID": onuGemInfo.IntfID, "OnuID": onuGemInfo.OnuID, "err": err})
	}
}

// collectOnDemandOnuStats will collect the onui-pon metrics
func (StatMgr *OpenOltStatisticsMgr) collectOnDemandOnuStats(ctx context.Context, intfID uint32, onuID uint32) map[string]float32 {
	onu := &openolt.Onu{IntfId: intfID, OnuId: onuID}
	var stats *openolt.OnuStatistics
	var err error
	logger.Debugw(ctx, "pulling-onu-stats-on-demand", log.Fields{"IntfID": intfID, "OnuID": onuID})
	if stats, err = StatMgr.Device.Client.GetOnuStatistics(context.Background(), onu); err == nil {
		statValue := StatMgr.convertONUStats(stats)
		return statValue

	}
	logger.Errorw(ctx, "error-while-getting-onu-stats-for-onu", log.Fields{"IntfID": intfID, "OnuID": onuID, "err": err})
	return nil
}

// collectOnuAndGemStats will collect both onu and gem metrics
func (StatMgr *OpenOltStatisticsMgr) collectOnuAndGemStats(ctx context.Context, onuGemInfo []rsrcMgr.OnuGemInfo) {
	if !StatMgr.Device.openOLT.enableONUStats && !StatMgr.Device.openOLT.enableGemStats {
		return
	}

	for _, onuInfo := range onuGemInfo {
		if StatMgr.Device.openOLT.enableONUStats {
			go StatMgr.collectOnuStats(ctx, onuInfo)
		}
		if StatMgr.Device.openOLT.enableGemStats {
			go StatMgr.collectGemStats(ctx, onuInfo)
		}
	}
}

// collectGemStats will collect gem metrics
func (StatMgr *OpenOltStatisticsMgr) collectGemStats(ctx context.Context, onuGemInfo rsrcMgr.OnuGemInfo) {
	for _, gem := range onuGemInfo.GemPorts {
		logger.Debugw(ctx, "pulling-gem-stats", log.Fields{"IntfID": onuGemInfo.IntfID, "OnuID": onuGemInfo.OnuID, "GemID": gem})
		onuPacket := &openolt.OnuPacket{IntfId: onuGemInfo.IntfID, OnuId: onuGemInfo.OnuID, GemportId: gem}
		if stats, err := StatMgr.Device.Client.GetGemPortStatistics(context.Background(), onuPacket); err == nil {
			gemStats <- stats
		} else {
			logger.Errorw(ctx, "error-while-getting-gem-stats-for-onu",
				log.Fields{"IntfID": onuGemInfo.IntfID, "OnuID": onuGemInfo.OnuID, "GemID": gem, "err": err})
		}
	}
}

// publishGemStats will publish the gem metrics
func (StatMgr *OpenOltStatisticsMgr) publishGemStats() {
	for {
		statValue := StatMgr.convertGemStats(<-gemStats)
		StatMgr.publishMetrics(context.Background(), GEMStats, statValue, &voltha.Port{Label: "GEM"}, StatMgr.Device.device.Id, StatMgr.Device.device.Type)
	}
}

// publishOnuStats will publish the onu metrics
func (StatMgr *OpenOltStatisticsMgr) publishOnuStats() {
	for {
		statValue := StatMgr.convertONUStats(<-onuStats)
		StatMgr.publishMetrics(context.Background(), ONUStats, statValue, &voltha.Port{Label: "ONU"}, StatMgr.Device.device.Id, StatMgr.Device.device.Type)
	}
}

// publishMetrics will publish the pon port metrics
func (StatMgr *OpenOltStatisticsMgr) publishMetrics(ctx context.Context, statType string, val map[string]float32,
	port *voltha.Port, devID string, devType string) {
	logger.Debugw(ctx, "publish-metrics",
		log.Fields{
			"port":    port.Label,
			"metrics": val})

	var metricInfo voltha.MetricInformation
	var ke voltha.KpiEvent2
	var volthaEventSubCatgry voltha.EventSubCategory_Types
	metricsContext := make(map[string]string)
	metricsContext["oltid"] = devID
	metricsContext["devicetype"] = devType
	metricsContext["portlabel"] = port.Label

	if statType == NNIStats {
		volthaEventSubCatgry = voltha.EventSubCategory_NNI
	} else if statType == PONStats {
		volthaEventSubCatgry = voltha.EventSubCategory_PON
	} else if statType == GEMStats || statType == ONUStats {
		volthaEventSubCatgry = voltha.EventSubCategory_ONT
	}

	raisedTs := time.Now().Unix()
	mmd := voltha.MetricMetaData{
		Title:    statType,
		Ts:       float64(raisedTs),
		Context:  metricsContext,
		DeviceId: devID,
	}

	metricInfo.Metadata = &mmd
	metricInfo.Metrics = val

	ke.SliceData = []*voltha.MetricInformation{&metricInfo}
	ke.Type = voltha.KpiEventType_slice
	ke.Ts = float64(raisedTs)

	if err := StatMgr.Device.EventProxy.SendKpiEvent(ctx, "STATS_PUBLISH_EVENT", &ke, voltha.EventCategory_EQUIPMENT, volthaEventSubCatgry, raisedTs); err != nil {
		logger.Errorw(ctx, "failed-to-send-stats", log.Fields{"err": err})
	}
}

// PortStatisticsIndication handles the port statistics indication
func (StatMgr *OpenOltStatisticsMgr) PortStatisticsIndication(ctx context.Context, PortStats *openolt.PortStatistics, NumPonPorts uint32) {
	StatMgr.PortsStatisticsKpis(ctx, PortStats, NumPonPorts)
	logger.Debugw(ctx, "received-port-stats-indication", log.Fields{"port-stats": PortStats})
	//Indicate that PortStatisticsIndication is handled
	//PortStats.IntfId is actually the port number
	StatMgr.processStatIndication(ctx, portStatsType, PortStats.IntfId)
	// TODO send stats to core topic to the voltha kafka or a different kafka ?
}

// FlowStatisticsIndication to be implemented
func FlowStatisticsIndication(ctx context.Context, self, FlowStats *openolt.FlowStatistics) {
	logger.Debugw(ctx, "flow-stats-collected", log.Fields{"flow-stats": FlowStats})
	//TODO send to kafka ?
}

// PortsStatisticsKpis map the port stats values into a dictionary, creates the kpiEvent and then publish to Kafka
func (StatMgr *OpenOltStatisticsMgr) PortsStatisticsKpis(ctx context.Context, PortStats *openolt.PortStatistics, NumPonPorts uint32) {

	/*map the port stats values into a dictionary
	  Create a kpoEvent and publish to Kafka

	  :param port_stats:
	  :return:
	*/
	//var err error
	IntfID := PortStats.IntfId

	if (plt.IntfIDToPortNo(1, voltha.Port_ETHERNET_NNI) < IntfID) &&
		(IntfID < plt.IntfIDToPortNo(4, voltha.Port_ETHERNET_NNI)) {
		/*
		   for this release we are only interested in the first NNI for
		   Northbound.
		   we are not using the other 3
		*/
		return
	} else if plt.IntfIDToPortNo(0, voltha.Port_ETHERNET_NNI) == IntfID {

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
		logger.Debugw(ctx, "received-nni-stats", log.Fields{"nni-stats": StatMgr.NorthBoundPort})
	}
	for i := uint32(0); i < NumPonPorts; i++ {

		if plt.IntfIDToPortNo(i, voltha.Port_PON_OLT) == IntfID {
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
			logger.Debugw(ctx, "received-pon-stats-for-port", log.Fields{"port-pon-stats": portPonStat})
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
	       logger.Error(ctx, "Error publishing statistics data")
	   }
	*/

}

func (StatMgr *OpenOltStatisticsMgr) updateGetOltPortCountersResponse(ctx context.Context, singleValResp *extension.SingleGetValueResponse, stats map[string]float32) {

	metrics := singleValResp.GetResponse().GetPortCoutners()
	metrics.TxBytes = uint64(stats["TxBytes"])
	metrics.RxBytes = uint64(stats["RxBytes"])
	metrics.TxPackets = uint64(stats["TxPackets"])
	metrics.RxPackets = uint64(stats["RxPackets"])
	metrics.TxErrorPackets = uint64(stats["TxErrorPackets"])
	metrics.RxErrorPackets = uint64(stats["RxErrorPackets"])
	metrics.TxBcastPackets = uint64(stats["TxBcastPackets"])
	metrics.RxBcastPackets = uint64(stats["RxBcastPackets"])
	metrics.TxUcastPackets = uint64(stats["TxUcastPackets"])
	metrics.RxUcastPackets = uint64(stats["RxUcastPackets"])
	metrics.TxMcastPackets = uint64(stats["TxMcastPackets"])
	metrics.RxMcastPackets = uint64(stats["RxMcastPackets"])

	singleValResp.Response.Status = extension.GetValueResponse_OK
	logger.Debugw(ctx, "updateGetOltPortCountersResponse", log.Fields{"resp": singleValResp})
}

//RegisterForStatIndication registers ch as a channel on which indication is sent when statistics of type t is received
func (StatMgr *OpenOltStatisticsMgr) RegisterForStatIndication(ctx context.Context, t StatType, ch chan bool, portNo uint32, portType extension.GetOltPortCounters_PortType) {
	statInd := statRegInfo{
		chn:      ch,
		portNo:   portNo,
		portType: portType,
	}

	logger.Debugf(ctx, "RegisterForStatIndication stat type %v portno %v porttype %v chan %v", t, portNo, portType, ch)
	StatMgr.statIndListnerMu.Lock()
	StatMgr.statIndListners[t].PushBack(statInd)
	StatMgr.statIndListnerMu.Unlock()

}

//DeRegisterFromStatIndication removes the previously registered channel ch for type t of statistics
func (StatMgr *OpenOltStatisticsMgr) DeRegisterFromStatIndication(ctx context.Context, t StatType, ch chan bool) {
	StatMgr.statIndListnerMu.Lock()
	defer StatMgr.statIndListnerMu.Unlock()

	for e := StatMgr.statIndListners[t].Front(); e != nil; e = e.Next() {
		statInd := e.Value.(statRegInfo)
		if statInd.chn == ch {
			StatMgr.statIndListners[t].Remove(e)
			return
		}
	}
}

func (StatMgr *OpenOltStatisticsMgr) processStatIndication(ctx context.Context, t StatType, portNo uint32) {
	var deRegList []*list.Element
	var statInd statRegInfo

	StatMgr.statIndListnerMu.Lock()
	defer StatMgr.statIndListnerMu.Unlock()

	if StatMgr.statIndListners[t] == nil || StatMgr.statIndListners[t].Len() == 0 {
		logger.Debugf(ctx, "processStatIndication %v list is empty ", t)
		return
	}

	for e := StatMgr.statIndListners[t].Front(); e != nil; e = e.Next() {
		statInd = e.Value.(statRegInfo)
		if statInd.portNo != portNo {
			fmt.Printf("Skipping %v\n", e.Value)
			continue
		}
		// message sent
		statInd.chn <- true
		deRegList = append(deRegList, e)

	}
	for _, e := range deRegList {
		StatMgr.statIndListners[t].Remove(e)
	}

}

func (StatMgr *OpenOltStatisticsMgr) updateGetOnuPonCountersResponse(ctx context.Context, singleValResp *extension.SingleGetValueResponse, stats map[string]float32) {

	metrics := singleValResp.GetResponse().GetOnuPonCounters()
	metrics.IsIntfId = &extension.GetOnuCountersResponse_IntfId{
		IntfId: uint32(stats[IntfID]),
	}
	metrics.IsOnuId = &extension.GetOnuCountersResponse_OnuId{
		OnuId: uint32(stats[OnuID]),
	}
	metrics.IsPositiveDrift = &extension.GetOnuCountersResponse_PositiveDrift{
		PositiveDrift: uint64(stats[PositiveDrift]),
	}
	metrics.IsNegativeDrift = &extension.GetOnuCountersResponse_NegativeDrift{
		NegativeDrift: uint64(stats[NegativeDrift]),
	}
	metrics.IsDelimiterMissDetection = &extension.GetOnuCountersResponse_DelimiterMissDetection{
		DelimiterMissDetection: uint64(stats[DelimiterMissDetection]),
	}
	metrics.IsBipErrors = &extension.GetOnuCountersResponse_BipErrors{
		BipErrors: uint64(stats[BipErrors]),
	}
	metrics.IsBipUnits = &extension.GetOnuCountersResponse_BipUnits{
		BipUnits: uint64(stats[BipUnits]),
	}
	metrics.IsFecCorrectedSymbols = &extension.GetOnuCountersResponse_FecCorrectedSymbols{
		FecCorrectedSymbols: uint64(stats[FecCorrectedSymbols]),
	}
	metrics.IsFecCodewordsCorrected = &extension.GetOnuCountersResponse_FecCodewordsCorrected{
		FecCodewordsCorrected: uint64(stats[FecCodewordsCorrected]),
	}
	metrics.IsFecCodewordsUncorrectable = &extension.GetOnuCountersResponse_FecCodewordsUncorrectable{
		FecCodewordsUncorrectable: uint64(stats[fecCodewordsUncorrectable]),
	}
	metrics.IsFecCodewords = &extension.GetOnuCountersResponse_FecCodewords{
		FecCodewords: uint64(stats[FecCodewords]),
	}
	metrics.IsFecCorrectedUnits = &extension.GetOnuCountersResponse_FecCorrectedUnits{
		FecCorrectedUnits: uint64(stats[FecCorrectedUnits]),
	}
	metrics.IsXgemKeyErrors = &extension.GetOnuCountersResponse_XgemKeyErrors{
		XgemKeyErrors: uint64(stats[XGEMKeyErrors]),
	}
	metrics.IsXgemLoss = &extension.GetOnuCountersResponse_XgemLoss{
		XgemLoss: uint64(stats[XGEMLoss]),
	}
	metrics.IsRxPloamsError = &extension.GetOnuCountersResponse_RxPloamsError{
		RxPloamsError: uint64(stats[RxPloamsError]),
	}
	metrics.IsRxPloamsNonIdle = &extension.GetOnuCountersResponse_RxPloamsNonIdle{
		RxPloamsNonIdle: uint64(stats[RxPloamsNonIdle]),
	}
	metrics.IsRxOmci = &extension.GetOnuCountersResponse_RxOmci{
		RxOmci: uint64(stats[RxOmci]),
	}
	metrics.IsRxOmciPacketsCrcError = &extension.GetOnuCountersResponse_RxOmciPacketsCrcError{
		RxOmciPacketsCrcError: uint64(stats[RxOmciPacketsCrcError]),
	}
	metrics.IsRxBytes = &extension.GetOnuCountersResponse_RxBytes{
		RxBytes: uint64(stats[RxBytes]),
	}
	metrics.IsRxPackets = &extension.GetOnuCountersResponse_RxPackets{
		RxPackets: uint64(stats[RxPackets]),
	}
	metrics.IsTxBytes = &extension.GetOnuCountersResponse_TxBytes{
		TxBytes: uint64(stats[TxBytes]),
	}
	metrics.IsTxPackets = &extension.GetOnuCountersResponse_TxPackets{
		TxPackets: uint64(stats[TxPackets]),
	}
	metrics.IsBerReported = &extension.GetOnuCountersResponse_BerReported{
		BerReported: uint64(stats[BerReported]),
	}
	metrics.IsLcdgErrors = &extension.GetOnuCountersResponse_LcdgErrors{
		LcdgErrors: uint64(stats[LcdgErrors]),
	}
	metrics.IsRdiErrors = &extension.GetOnuCountersResponse_RdiErrors{
		RdiErrors: uint64(stats[RdiErrors]),
	}
	metrics.IsTimestamp = &extension.GetOnuCountersResponse_Timestamp{
		Timestamp: uint32(stats[Timestamp]),
	}

	singleValResp.Response.Status = extension.GetValueResponse_OK
	logger.Debugw(ctx, "updateGetOnuPonCountersResponse", log.Fields{"resp": singleValResp})
}
