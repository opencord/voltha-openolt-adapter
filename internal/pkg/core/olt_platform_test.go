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

//Package core provides the utility for olt devices, flows and statistics
package core

import (
	"context"
	"math"
	"reflect"
	"testing"

	fu "github.com/opencord/voltha-lib-go/v7/pkg/flows"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	ofp "github.com/opencord/voltha-protos/v5/go/openflow_13"
	"github.com/opencord/voltha-protos/v5/go/voltha"
)

func TestMkUniPortNum(t *testing.T) {
	type args struct {
		intfID uint32
		onuID  uint32
		uniID  uint32
	}
	tests := []struct {
		name string
		args args
		want uint32
	}{
		// TODO: Add test cases.
		{"MkUniPortNum-1", args{1, 1, 1}, ((1 * 4096) + (1 * 16) + 1)},
		{"MkUniPortNum-2", args{4, 5, 6}, ((4 * 4096) + (5 * 16) + 6)},
		// Negative test cases to cover the log.warn
		{"MkUniPortNum-3", args{4, 130, 6}, ((4 * 4096) + (130 * 16) + 6)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MkUniPortNum(context.Background(), tt.args.intfID, tt.args.onuID, tt.args.uniID); got != tt.want {
				t.Errorf("MkUniPortNum() = %v, want %v", got, tt.want)
			} else {
				t.Logf("Expected %v , Actual %v \n", tt.want, got)
			}
		})
	}
}

func TestOnuIDFromPortNum(t *testing.T) {
	type args struct {
		portNum uint32
	}
	tests := []struct {
		name string
		args args
		want uint32
	}{
		// TODO: Add test cases.
		{"OnuIDFromPortNum-1", args{portNum: 8096}, ((8096 / 16) & 255)},
		{"OnuIDFromPortNum-2", args{portNum: 9095}, ((9095 / 16) & 255)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OnuIDFromPortNum(tt.args.portNum); got != tt.want {
				t.Errorf("OnuIDFromPortNum() = %v, want %v", got, tt.want)
			} else {
				t.Logf("Expected %v , Actual %v \n", tt.want, got)
			}
		})
	}
}

func TestIntfIDFromUniPortNum(t *testing.T) {
	type args struct {
		portNum uint32
	}
	tests := []struct {
		name string
		args args
		want uint32
	}{
		// TODO: Add test cases.
		{"IntfIDFromUniPortNum-1", args{portNum: 8096}, ((8096 / 4096) & 15)},
		// Negative Testcase
		{"IntfIDFromUniPortNum-2", args{portNum: 1024}, ((1024 / 4096) & 15)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IntfIDFromUniPortNum(tt.args.portNum); got != tt.want {
				t.Errorf("IntfIDFromUniPortNum() = %v, want %v", got, tt.want)
			} else {
				t.Logf("Expected %v , Actual %v \n", tt.want, got)
			}
		})
	}
}

func TestUniIDFromPortNum(t *testing.T) {
	type args struct {
		portNum uint32
	}
	tests := []struct {
		name string
		args args
		want uint32
	}{

		// TODO: Add test cases.
		{"UniIDFromPortNum-1", args{portNum: 8096}, (8096 & 15)},
		{"UniIDFromPortNum-2", args{portNum: 1024}, (1024 & 15)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UniIDFromPortNum(tt.args.portNum); got != tt.want {
				t.Errorf("UniIDFromPortNum() = %v, want %v", got, tt.want)
			} else {
				t.Logf("Expected %v , Actual %v \n", tt.want, got)
			}
		})
	}
}

func TestIntfIDToPortNo(t *testing.T) {
	type args struct {
		intfID   uint32
		intfType voltha.Port_PortType
	}
	tests := []struct {
		name string
		args args
		want uint32
	}{
		// TODO: Add test cases.
		{"IntfIDToPortNo-1", args{intfID: 120, intfType: voltha.Port_ETHERNET_NNI}, (uint32(math.Pow(2, 20)) + 120)},
		{"IntfIDToPortNo-2", args{intfID: 1024, intfType: voltha.Port_ETHERNET_UNI}, 0},
		{"IntfIDToPortNo-3", args{intfID: 456, intfType: voltha.Port_PON_OLT}, (uint32(2*math.Pow(2, 28)) + 456)},
		{"IntfIDToPortNo-4", args{intfID: 28, intfType: voltha.Port_PON_ONU}, 0},
		{"IntfIDToPortNo-5", args{intfID: 45, intfType: voltha.Port_UNKNOWN}, 0},
		{"IntfIDToPortNo-6", args{intfID: 45, intfType: voltha.Port_VENET_OLT}, 0},
		{"IntfIDToPortNo-7", args{intfID: 45, intfType: voltha.Port_VENET_ONU}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IntfIDToPortNo(tt.args.intfID, tt.args.intfType); got != tt.want {
				t.Errorf("IntfIDToPortNo() = %v, want %v", got, tt.want)
			} else {
				t.Logf("Expected %v , Actual %v \n", tt.want, got)
			}
		})
	}
}

func TestIntfIDFromNniPortNum(t *testing.T) {
	type args struct {
		portNum uint32
	}

	tests := []struct {
		name    string
		args    args
		want    uint32
		wantErr error
	}{
		// TODO: Add test cases.
		{"IntfIDFromNniPortNum-01", args{portNum: 8081}, 0, olterrors.ErrInvalidPortRange},
		{"IntfIDFromNniPortNum-02", args{portNum: 9090}, 0, olterrors.ErrInvalidPortRange},
		{"IntfIDFromNniPortNum-03", args{portNum: 0}, 0, olterrors.ErrInvalidPortRange},
		{"IntfIDFromNniPortNum-04", args{portNum: 65535}, 0, olterrors.ErrInvalidPortRange},
		{"IntfIDFromNniPortNum-05", args{portNum: 1048575}, 0, olterrors.ErrInvalidPortRange},
		{"IntfIDFromNniPortNum-06", args{portNum: 1048576}, 0, nil},
		{"IntfIDFromNniPortNum-07", args{portNum: 1048577}, 1, nil},
		{"IntfIDFromNniPortNum-08", args{portNum: 1048578}, 2, nil},
		{"IntfIDFromNniPortNum-09", args{portNum: 1048579}, 3, nil},
		{"IntfIDFromNniPortNum-10", args{portNum: 2097150}, 65534, nil},
		{"IntfIDFromNniPortNum-11", args{portNum: 2097151}, 65535, nil},
		{"IntfIDFromNniPortNum-12", args{portNum: 3000000}, 0, olterrors.ErrInvalidPortRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IntfIDFromNniPortNum(context.Background(), tt.args.portNum)
			if got != tt.want || err != tt.wantErr {
				t.Errorf("IntfIDFromNniPortNum(): FOR[%v] WANT[%v and %v] GOT[%v and %v]",
					tt.args.portNum, tt.want, tt.wantErr, got, err)
			}
		})
	}
}

func TestIntfIDToPortTypeName(t *testing.T) {
	type args struct {
		intfID uint32
	}
	input := uint32(2*math.Pow(2, 28)) | 3
	tests := []struct {
		name string
		args args
		want voltha.Port_PortType
	}{
		// TODO: Add test cases.
		{"IntfIDToPortTypeName-1", args{intfID: 1048576}, voltha.Port_ETHERNET_NNI},
		{"IntfIDToPortTypeName-2", args{intfID: 1000}, voltha.Port_ETHERNET_UNI},
		{"IntfIDToPortTypeName-2", args{intfID: input}, voltha.Port_PON_OLT},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IntfIDToPortTypeName(tt.args.intfID); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IntfIDToPortTypeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractAccessFromFlow(t *testing.T) {
	type args struct {
		inPort  uint32
		outPort uint32
	}
	tests := []struct {
		name   string
		args   args
		port   uint32
		IntfID uint32
		onuID  uint32
		uniID  uint32
	}{
		// TODO: Add test cases.
		{"ExtractAccessFromFlow-1", args{inPort: 100, outPort: 1048576}, 100, 0, 6, 4},
		{"ExtractAccessFromFlow-2", args{inPort: 1048576, outPort: 10}, 10, 0, 0, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, got3 := ExtractAccessFromFlow(tt.args.inPort, tt.args.outPort)
			if got != tt.port {
				t.Errorf("ExtractAccessFromFlow() got = %v, want %v", got, tt.port)
			}
			if got1 != tt.IntfID {
				t.Errorf("ExtractAccessFromFlow() got1 = %v, want %v", got1, tt.IntfID)
			}
			if got2 != tt.onuID {
				t.Errorf("ExtractAccessFromFlow() got2 = %v, want %v", got2, tt.onuID)
			}
			if got3 != tt.uniID {
				t.Errorf("ExtractAccessFromFlow() got3 = %v, want %v", got3, tt.uniID)
			}
		})
	}
	//t.Error()
}

func TestIsUpstream(t *testing.T) {
	type args struct {
		outPort uint32
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{"TestIsUpstream-1", args{outPort: 65533}, true},
		{"TestIsUpstream-2", args{outPort: 1048576}, true},
		{"TestIsUpstream-3", args{outPort: 1048577}, true},
		{"TestIsUpstream-4", args{outPort: 1048578}, true},
		{"TestIsUpstream-6", args{outPort: 1000}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUpstream(tt.args.outPort); got != tt.want {
				t.Errorf("IsUpstream() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsControllerBoundFlow(t *testing.T) {
	type args struct {
		outPort uint32
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{"IsControllerBoundFlow-1", args{outPort: 65533}, true},
		{"IsControllerBoundFlow-2", args{outPort: 65536}, false},
		{"IsControllerBoundFlow-3", args{outPort: 65537}, false},
		{"IsControllerBoundFlow-4", args{outPort: 65538}, false},
		{"IsControllerBoundFlow-5", args{outPort: 65539}, false},
		{"IsControllerBoundFlow-6", args{outPort: 1000}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsControllerBoundFlow(tt.args.outPort); got != tt.want {
				t.Errorf("IsControllerBoundFlow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlowExtractInfo(t *testing.T) {
	fa := &fu.FlowArgs{
		MatchFields: []*ofp.OfpOxmOfbField{
			fu.InPort(2),
			fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA | 2)),
			fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT)),
			fu.EthType(2048),
		},

		Actions: []*ofp.OfpAction{
			fu.SetField(fu.Metadata_ofp(uint64(ofp.OfpInstructionType_OFPIT_WRITE_METADATA))),
			fu.SetField(fu.VlanVid(uint32(ofp.OfpVlanId_OFPVID_PRESENT) | 101)),
			fu.Output(1),
		},
	}
	ofpstats, _ := fu.MkFlowStat(fa)
	type args struct {
		flow          *ofp.OfpFlowStats
		flowDirection string
	}
	tests := []struct {
		name    string
		args    args
		want    uint32
		want1   uint32
		want2   uint32
		want3   uint32
		want4   uint32
		want5   uint32
		wantErr bool
	}{
		// TODO: Add test cases.
		{"FlowExtractInfo-1", args{flow: ofpstats, flowDirection: "upstream"}, 2, 0, 0, 2, 0, 0, false},

		// Negative Testcases
		{"FlowExtractInfo-2", args{flow: ofpstats, flowDirection: "downstream"}, 1, 0, 0, 1, 2, 2048, false},
		{"FlowExtractInfo-3", args{flow: nil, flowDirection: "downstream"}, 0, 0, 0, 0, 0, 0, true},
		{"FlowExtractInfo-4", args{flow: &ofp.OfpFlowStats{}, flowDirection: "downstream"}, 0, 0, 0, 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, got3, got4, got5, err := FlowExtractInfo(context.Background(), tt.args.flow, tt.args.flowDirection)
			if (err != nil) != tt.wantErr {
				t.Errorf("FlowExtractInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FlowExtractInfo() got = %v, want %v", got, tt.want)
				return
			}
			if got1 != tt.want1 {
				t.Errorf("FlowExtractInfo() got1 = %v, want %v", got1, tt.want1)
				return
			}
			if got2 != tt.want2 {
				t.Errorf("FlowExtractInfo() got2 = %v, want %v", got2, tt.want2)
				return
			}
			if got3 != tt.want3 {
				t.Errorf("FlowExtractInfo() got3 = %v, want %v", got3, tt.want3)
				return
			}
			if got4 != tt.want4 {
				t.Errorf("FlowExtractInfo() got4 = %v, want %v", got4, tt.want4)
				return
			}
			if got5 != tt.want5 {
				t.Errorf("FlowExtractInfo() got5 = %v, want %v", got5, tt.want5)
				return
			}
		})
	}
}
