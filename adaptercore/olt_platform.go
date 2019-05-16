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
	"errors"
	fd "github.com/opencord/voltha-go/rw_core/flow_decomposition"
	ofp "github.com/opencord/voltha-protos/go/openflow_13"
	voltha "github.com/opencord/voltha-protos/go/voltha"
)

/*=====================================================================

Flow id

    Identifies a flow within a single OLT
    Flow Id is unique per OLT
    Multiple GEM ports can map to same flow id

     13    11              4      0
    +--------+--------------+------+
    | pon id |    onu id    | Flow |
    |        |              | idx  |
    +--------+--------------+------+

    14 bits = 16384 flows (per OLT).

    pon id = 4 bits = 16 PON ports
    onu id = 7 bits = 128 ONUss per PON port
    Flow index = 3 bits = 4 bi-directional flows per ONU
                        = 8 uni-directional flows per ONU


Logical (OF) UNI port number

    OpenFlow port number corresponding to PON UNI

     15       11              4      0
    +--+--------+--------------+------+
    |0 | pon id |    onu id    |   0  |
    +--+--------+--------------+------+

    pon id = 4 bits = 16 PON ports
    onu id = 7 bits = 128 ONUs per PON port

Logical (OF) NNI port number

    OpenFlow port number corresponding to PON UNI

     16                             0
    +--+----------------------------+
    |1 |                    intf_id |
    +--+----------------------------+

    No overlap with UNI port number space


PON OLT (OF) port number

    OpenFlow port number corresponding to PON OLT ports

     31    28                                 0
    +--------+------------------------~~~------+
    |  0x2   |          pon intf id            |
    +--------+------------------------~~~------+
*/

var MAX_ONUS_PER_PON = 32
var MIN_UPSTREAM_PORT_ID = 0xfffd
var MAX_UPSTREAM_PORT_ID = 0xfffffffd

var controllerPorts []uint32 = []uint32{0xfffd, 0x7ffffffd, 0xfffffffd}

func MkUniPortNum(intfId uint32, onuId uint32, uniId uint32) uint32 {
	/* TODO: Add checks */
	return ((intfId << 11) | (onuId << 4) | uniId)
}

func MkFlowId(intfId uint32, onuId uint32, idx uint32) uint32 {
	return (((intfId << 9) | (onuId << 4)) | idx)
}

func OnuIdFromPortNum(portNum uint32) uint32 {
	return ((portNum >> 4) & 127)
}

func IntfIdFromUniPortNum(portNum uint32) uint32 {
	return ((portNum >> 11) & 15)
}

func UniIdFromPortNum(portNum uint32) uint32 {
	return ((portNum) & 0xF)
}

func IntfIdFromPonPortNo(portNo uint32) uint32 {
	return (portNo & 15)
}

func IntfIdToPortNo(intfId uint32, intfType voltha.Port_PortType) uint32 {
	if (intfType) == voltha.Port_ETHERNET_NNI {
		return ((1 << 16) | intfId)
	} else {
		if (intfType) == voltha.Port_PON_OLT {
			return ((2 << 28) | intfId)
		} else {
			return 0
		}
	}
}

func IntfIdFromNniPortNum(portNum uint32) uint32 {
	return (portNum & 0xFFFF)
}

func IntfIdToPortTypeName(intfId uint32) voltha.Port_PortType {
	if ((2 << 28) ^ intfId) < 16 {
		return voltha.Port_PON_OLT
	} else {
		if (intfId & (1 << 16)) == (1 << 16) {
			return voltha.Port_ETHERNET_NNI
		} else {
			return voltha.Port_ETHERNET_UNI
		}
	}
}

func PortTypeNameByPortIndex(portIndex int32) string {
	return voltha.Port_PortType_name[portIndex]
}

func ExtractAccessFromFlow(inPort uint32, outPort uint32) (uint32, uint32, uint32, uint32) {
	if IsUpstream(outPort) {
		return inPort, IntfIdFromUniPortNum(inPort), OnuIdFromPortNum(inPort), UniIdFromPortNum(inPort)
	} else {
		return outPort, IntfIdFromUniPortNum(outPort), OnuIdFromPortNum(outPort), UniIdFromPortNum(outPort)
	}
}

func IsUpstream(outPort uint32) bool {
	for _, port := range controllerPorts {
		if port == outPort {
			return true
		}
	}
	if (outPort & (1 << 16)) == (1 << 16) {
		return true
	}
	return false
}

func IsControllerBoundFlow(outPort uint32) bool {
	for _, port := range controllerPorts {
		if port == outPort {
			return true
		}
	}
	return false
}

func OnuIdFromUniPortNum(portNum uint32) uint32 {
	return (portNum >> 4) & 0x7F
}

func FlowExtractInfo(flow *ofp.OfpFlowStats, flowDirection string) (uint32, uint32, uint32, error) {
	var uniPortNo uint32 = 0
	var ponIntf uint32 = 0
	var onuId uint32 = 0
	var uniId uint32 = 0

	if flowDirection == "upstream" {
		if uniPortNo = fd.GetChildPortFromTunnelId(flow); uniPortNo == 0 {
			for _, field := range fd.GetOfbFields(flow) {
				if field.GetType() == fd.IN_PORT {
					uniPortNo = field.GetPort()
					break
				}
			}
		}
	} else if flowDirection == "downstream" {
		if uniPortNo = fd.GetChildPortFromTunnelId(flow); uniPortNo == 0 {
			for _, action := range fd.GetActions(flow) {
				if action.Type == fd.OUTPUT {
					if out := action.GetOutput(); out != nil {
						uniPortNo = out.GetPort()
					}
					break
				}
			}
		}
	}

	if uniPortNo == 0 {
		return 0, 0, 0, errors.New("Failed to extract Pon Interface, ONU Id and Uni Id from flow")
	}

	ponIntf = IntfIdFromUniPortNum(uniPortNo)
	onuId = OnuIdFromUniPortNum(uniPortNo)
	uniId = UniIdFromPortNum(uniPortNo)

	return ponIntf, onuId, uniId, nil
}
