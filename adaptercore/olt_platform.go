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

//Package adaptercore provides the utility for olt devices, flows and statistics
package adaptercore

import (
	"errors"
	"github.com/opencord/voltha-go/rw_core/utils"
	"github.com/opencord/voltha-lib-go/pkg/common/log"
	ofp "github.com/opencord/voltha-protos/go/openflow_13"
	"github.com/opencord/voltha-protos/go/voltha"
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

//MaxOnusPerPon value
var MaxOnusPerPon = 128

//MinUpstreamPortID value
var MinUpstreamPortID = 0xfffd

//MaxUpstreamPortID value
var MaxUpstreamPortID = 0xfffffffd

var controllerPorts = []uint32{0xfffd, 0x7ffffffd, 0xfffffffd}

//MkUniPortNum returns new UNIportNum based on intfID, inuID and uniID
func MkUniPortNum(intfID, onuID, uniID uint32) uint32 {
	var limit = int(onuID)
	if limit > MaxOnusPerPon {
		log.Warn("Warning: exceeded the MAX ONUS per PON")
	}
	return (intfID << 11) | (onuID << 4) | uniID
}

//OnuIDFromPortNum returns ONUID derived from portNumber
func OnuIDFromPortNum(portNum uint32) uint32 {
	return (portNum >> 4) & 127
}

//IntfIDFromUniPortNum returns IntfID derived from portNum
func IntfIDFromUniPortNum(portNum uint32) uint32 {
	return (portNum >> 11) & 15
}

//UniIDFromPortNum return UniID derived from portNum
func UniIDFromPortNum(portNum uint32) uint32 {
	return (portNum) & 0xF
}

//IntfIDToPortNo returns portId derived from intftype, intfId and portType
func IntfIDToPortNo(intfID uint32, intfType voltha.Port_PortType) uint32 {
	if (intfType) == voltha.Port_ETHERNET_NNI {
		return (1 << 16) | intfID
	}
	if (intfType) == voltha.Port_PON_OLT {
		return (2 << 28) | intfID
	}
	return 0
}

//IntfIDFromNniPortNum returns Intf ID derived from portNum
func IntfIDFromNniPortNum(portNum uint32) uint32 {
	return portNum & 0xFFFF
}

//IntfIDToPortTypeName returns port type derived from the intfId
func IntfIDToPortTypeName(intfID uint32) voltha.Port_PortType {
	if ((2 << 28) ^ intfID) < 16 {
		return voltha.Port_PON_OLT
	}
	if (intfID & (1 << 16)) == (1 << 16) {
		return voltha.Port_ETHERNET_NNI
	}
	return voltha.Port_ETHERNET_UNI
}

//ExtractAccessFromFlow returns AccessDevice information
func ExtractAccessFromFlow(inPort, outPort uint32) (uint32, uint32, uint32, uint32) {
	if IsUpstream(outPort) {
		return inPort, IntfIDFromUniPortNum(inPort), OnuIDFromPortNum(inPort), UniIDFromPortNum(inPort)
	}
	return outPort, IntfIDFromUniPortNum(outPort), OnuIDFromPortNum(outPort), UniIDFromPortNum(outPort)
}

//IsUpstream returns true for Upstream and false for downstream
func IsUpstream(outPort uint32) bool {
	for _, port := range controllerPorts {
		if port == outPort {
			return true
		}
	}
	return (outPort & (1 << 16)) == (1 << 16)
}

//IsControllerBoundFlow returns true/false
func IsControllerBoundFlow(outPort uint32) bool {
	for _, port := range controllerPorts {
		if port == outPort {
			return true
		}
	}
	return false
}

//OnuIDFromUniPortNum returns onuId from give portNum information.
func OnuIDFromUniPortNum(portNum uint32) uint32 {
	return (portNum >> 4) & 0x7F
}

//FlowExtractInfo fetches uniport from the flow, based on which it gets and returns ponInf, onuID, uniID, inPort and ethType
func FlowExtractInfo(flow *ofp.OfpFlowStats, flowDirection string) (uint32, uint32, uint32, uint32, uint32, uint32, error) {
	var uniPortNo uint32
	var ponIntf uint32
	var onuID uint32
	var uniID uint32
	var inPort uint32
	var ethType uint32

	if flowDirection == "upstream" {
		if uniPortNo = utils.GetChildPortFromTunnelId(flow); uniPortNo == 0 {
			for _, field := range utils.GetOfbFields(flow) {
				if field.GetType() == utils.IN_PORT {
					uniPortNo = field.GetPort()
					break
				}
			}
		}
	} else if flowDirection == "downstream" {
		if uniPortNo = utils.GetChildPortFromTunnelId(flow); uniPortNo == 0 {
			for _, field := range utils.GetOfbFields(flow) {
				if field.GetType() == utils.METADATA {
					for _, action := range utils.GetActions(flow) {
						if action.Type == utils.OUTPUT {
							if out := action.GetOutput(); out != nil {
								uniPortNo = out.GetPort()
							}
							break
						}
					}
				} else if field.GetType() == utils.IN_PORT {
					inPort = field.GetPort()
				} else if field.GetType() == utils.ETH_TYPE {
					ethType = field.GetEthType()
				}
			}
		}
	}

	if uniPortNo == 0 {
		return 0, 0, 0, 0, 0, 0, errors.New("failed to extract Pon Interface, ONU Id and Uni Id from flow")
	}

	ponIntf = IntfIDFromUniPortNum(uniPortNo)
	onuID = OnuIDFromUniPortNum(uniPortNo)
	uniID = UniIDFromPortNum(uniPortNo)

	return uniPortNo, ponIntf, onuID, uniID, inPort, ethType, nil
}
