// Code generated by protoc-gen-go. DO NOT EDIT.
// source: voltha_protos/common.proto

package common

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type TestModeKeys int32

const (
	TestModeKeys_api_test TestModeKeys = 0
)

var TestModeKeys_name = map[int32]string{
	0: "api_test",
}

var TestModeKeys_value = map[string]int32{
	"api_test": 0,
}

func (x TestModeKeys) String() string {
	return proto.EnumName(TestModeKeys_name, int32(x))
}

func (TestModeKeys) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{0}
}

// Administrative State
type AdminState_Types int32

const (
	// The administrative state of the device is unknown
	AdminState_UNKNOWN AdminState_Types = 0
	// The device is pre-provisioned into Voltha, but not contacted by it
	AdminState_PREPROVISIONED AdminState_Types = 1
	// The device is enabled for activation and operation
	AdminState_ENABLED AdminState_Types = 2
	// The device is disabled and shall not perform its intended forwarding
	// functions other than being available for re-activation.
	AdminState_DISABLED AdminState_Types = 3
	// The device is in the state of image download
	AdminState_DOWNLOADING_IMAGE AdminState_Types = 4
)

var AdminState_Types_name = map[int32]string{
	0: "UNKNOWN",
	1: "PREPROVISIONED",
	2: "ENABLED",
	3: "DISABLED",
	4: "DOWNLOADING_IMAGE",
}

var AdminState_Types_value = map[string]int32{
	"UNKNOWN":           0,
	"PREPROVISIONED":    1,
	"ENABLED":           2,
	"DISABLED":          3,
	"DOWNLOADING_IMAGE": 4,
}

func (x AdminState_Types) String() string {
	return proto.EnumName(AdminState_Types_name, int32(x))
}

func (AdminState_Types) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{2, 0}
}

// Operational Status
type OperStatus_Types int32

const (
	// The status of the device is unknown at this point
	OperStatus_UNKNOWN OperStatus_Types = 0
	// The device has been discovered, but not yet activated
	OperStatus_DISCOVERED OperStatus_Types = 1
	// The device is being activated (booted, rebooted, upgraded, etc.)
	OperStatus_ACTIVATING OperStatus_Types = 2
	// Service impacting tests are being conducted
	OperStatus_TESTING OperStatus_Types = 3
	// The device is up and active
	OperStatus_ACTIVE OperStatus_Types = 4
	// The device has failed and cannot fulfill its intended role
	OperStatus_FAILED OperStatus_Types = 5
)

var OperStatus_Types_name = map[int32]string{
	0: "UNKNOWN",
	1: "DISCOVERED",
	2: "ACTIVATING",
	3: "TESTING",
	4: "ACTIVE",
	5: "FAILED",
}

var OperStatus_Types_value = map[string]int32{
	"UNKNOWN":    0,
	"DISCOVERED": 1,
	"ACTIVATING": 2,
	"TESTING":    3,
	"ACTIVE":     4,
	"FAILED":     5,
}

func (x OperStatus_Types) String() string {
	return proto.EnumName(OperStatus_Types_name, int32(x))
}

func (OperStatus_Types) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{3, 0}
}

// Connectivity Status
type ConnectStatus_Types int32

const (
	// The device connectivity status is unknown
	ConnectStatus_UNKNOWN ConnectStatus_Types = 0
	// The device cannot be reached by Voltha
	ConnectStatus_UNREACHABLE ConnectStatus_Types = 1
	// There is live communication between device and Voltha
	ConnectStatus_REACHABLE ConnectStatus_Types = 2
)

var ConnectStatus_Types_name = map[int32]string{
	0: "UNKNOWN",
	1: "UNREACHABLE",
	2: "REACHABLE",
}

var ConnectStatus_Types_value = map[string]int32{
	"UNKNOWN":     0,
	"UNREACHABLE": 1,
	"REACHABLE":   2,
}

func (x ConnectStatus_Types) String() string {
	return proto.EnumName(ConnectStatus_Types_name, int32(x))
}

func (ConnectStatus_Types) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{4, 0}
}

type OperationResp_OperationReturnCode int32

const (
	OperationResp_OPERATION_SUCCESS     OperationResp_OperationReturnCode = 0
	OperationResp_OPERATION_FAILURE     OperationResp_OperationReturnCode = 1
	OperationResp_OPERATION_UNSUPPORTED OperationResp_OperationReturnCode = 2
	OperationResp_OPERATION_IN_PROGRESS OperationResp_OperationReturnCode = 3
)

var OperationResp_OperationReturnCode_name = map[int32]string{
	0: "OPERATION_SUCCESS",
	1: "OPERATION_FAILURE",
	2: "OPERATION_UNSUPPORTED",
	3: "OPERATION_IN_PROGRESS",
}

var OperationResp_OperationReturnCode_value = map[string]int32{
	"OPERATION_SUCCESS":     0,
	"OPERATION_FAILURE":     1,
	"OPERATION_UNSUPPORTED": 2,
	"OPERATION_IN_PROGRESS": 3,
}

func (x OperationResp_OperationReturnCode) String() string {
	return proto.EnumName(OperationResp_OperationReturnCode_name, int32(x))
}

func (OperationResp_OperationReturnCode) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{5, 0}
}

type ValueType_Type int32

const (
	ValueType_EMPTY    ValueType_Type = 0
	ValueType_DISTANCE ValueType_Type = 1
)

var ValueType_Type_name = map[int32]string{
	0: "EMPTY",
	1: "DISTANCE",
}

var ValueType_Type_value = map[string]int32{
	"EMPTY":    0,
	"DISTANCE": 1,
}

func (x ValueType_Type) String() string {
	return proto.EnumName(ValueType_Type_name, int32(x))
}

func (ValueType_Type) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{6, 0}
}

// Convey a resource identifier
type ID struct {
	Id                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ID) Reset()         { *m = ID{} }
func (m *ID) String() string { return proto.CompactTextString(m) }
func (*ID) ProtoMessage()    {}
func (*ID) Descriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{0}
}

func (m *ID) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ID.Unmarshal(m, b)
}
func (m *ID) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ID.Marshal(b, m, deterministic)
}
func (m *ID) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ID.Merge(m, src)
}
func (m *ID) XXX_Size() int {
	return xxx_messageInfo_ID.Size(m)
}
func (m *ID) XXX_DiscardUnknown() {
	xxx_messageInfo_ID.DiscardUnknown(m)
}

var xxx_messageInfo_ID proto.InternalMessageInfo

func (m *ID) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

// Represents a list of IDs
type IDs struct {
	Items                []*ID    `protobuf:"bytes,1,rep,name=items,proto3" json:"items,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *IDs) Reset()         { *m = IDs{} }
func (m *IDs) String() string { return proto.CompactTextString(m) }
func (*IDs) ProtoMessage()    {}
func (*IDs) Descriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{1}
}

func (m *IDs) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_IDs.Unmarshal(m, b)
}
func (m *IDs) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_IDs.Marshal(b, m, deterministic)
}
func (m *IDs) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IDs.Merge(m, src)
}
func (m *IDs) XXX_Size() int {
	return xxx_messageInfo_IDs.Size(m)
}
func (m *IDs) XXX_DiscardUnknown() {
	xxx_messageInfo_IDs.DiscardUnknown(m)
}

var xxx_messageInfo_IDs proto.InternalMessageInfo

func (m *IDs) GetItems() []*ID {
	if m != nil {
		return m.Items
	}
	return nil
}

type AdminState struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AdminState) Reset()         { *m = AdminState{} }
func (m *AdminState) String() string { return proto.CompactTextString(m) }
func (*AdminState) ProtoMessage()    {}
func (*AdminState) Descriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{2}
}

func (m *AdminState) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AdminState.Unmarshal(m, b)
}
func (m *AdminState) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AdminState.Marshal(b, m, deterministic)
}
func (m *AdminState) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AdminState.Merge(m, src)
}
func (m *AdminState) XXX_Size() int {
	return xxx_messageInfo_AdminState.Size(m)
}
func (m *AdminState) XXX_DiscardUnknown() {
	xxx_messageInfo_AdminState.DiscardUnknown(m)
}

var xxx_messageInfo_AdminState proto.InternalMessageInfo

type OperStatus struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *OperStatus) Reset()         { *m = OperStatus{} }
func (m *OperStatus) String() string { return proto.CompactTextString(m) }
func (*OperStatus) ProtoMessage()    {}
func (*OperStatus) Descriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{3}
}

func (m *OperStatus) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_OperStatus.Unmarshal(m, b)
}
func (m *OperStatus) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_OperStatus.Marshal(b, m, deterministic)
}
func (m *OperStatus) XXX_Merge(src proto.Message) {
	xxx_messageInfo_OperStatus.Merge(m, src)
}
func (m *OperStatus) XXX_Size() int {
	return xxx_messageInfo_OperStatus.Size(m)
}
func (m *OperStatus) XXX_DiscardUnknown() {
	xxx_messageInfo_OperStatus.DiscardUnknown(m)
}

var xxx_messageInfo_OperStatus proto.InternalMessageInfo

type ConnectStatus struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ConnectStatus) Reset()         { *m = ConnectStatus{} }
func (m *ConnectStatus) String() string { return proto.CompactTextString(m) }
func (*ConnectStatus) ProtoMessage()    {}
func (*ConnectStatus) Descriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{4}
}

func (m *ConnectStatus) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ConnectStatus.Unmarshal(m, b)
}
func (m *ConnectStatus) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ConnectStatus.Marshal(b, m, deterministic)
}
func (m *ConnectStatus) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConnectStatus.Merge(m, src)
}
func (m *ConnectStatus) XXX_Size() int {
	return xxx_messageInfo_ConnectStatus.Size(m)
}
func (m *ConnectStatus) XXX_DiscardUnknown() {
	xxx_messageInfo_ConnectStatus.DiscardUnknown(m)
}

var xxx_messageInfo_ConnectStatus proto.InternalMessageInfo

type OperationResp struct {
	// Return code
	Code OperationResp_OperationReturnCode `protobuf:"varint,1,opt,name=code,proto3,enum=common.OperationResp_OperationReturnCode" json:"code,omitempty"`
	// Additional Info
	AdditionalInfo       string   `protobuf:"bytes,2,opt,name=additional_info,json=additionalInfo,proto3" json:"additional_info,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *OperationResp) Reset()         { *m = OperationResp{} }
func (m *OperationResp) String() string { return proto.CompactTextString(m) }
func (*OperationResp) ProtoMessage()    {}
func (*OperationResp) Descriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{5}
}

func (m *OperationResp) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_OperationResp.Unmarshal(m, b)
}
func (m *OperationResp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_OperationResp.Marshal(b, m, deterministic)
}
func (m *OperationResp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_OperationResp.Merge(m, src)
}
func (m *OperationResp) XXX_Size() int {
	return xxx_messageInfo_OperationResp.Size(m)
}
func (m *OperationResp) XXX_DiscardUnknown() {
	xxx_messageInfo_OperationResp.DiscardUnknown(m)
}

var xxx_messageInfo_OperationResp proto.InternalMessageInfo

func (m *OperationResp) GetCode() OperationResp_OperationReturnCode {
	if m != nil {
		return m.Code
	}
	return OperationResp_OPERATION_SUCCESS
}

func (m *OperationResp) GetAdditionalInfo() string {
	if m != nil {
		return m.AdditionalInfo
	}
	return ""
}

type ValueType struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ValueType) Reset()         { *m = ValueType{} }
func (m *ValueType) String() string { return proto.CompactTextString(m) }
func (*ValueType) ProtoMessage()    {}
func (*ValueType) Descriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{6}
}

func (m *ValueType) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ValueType.Unmarshal(m, b)
}
func (m *ValueType) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ValueType.Marshal(b, m, deterministic)
}
func (m *ValueType) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ValueType.Merge(m, src)
}
func (m *ValueType) XXX_Size() int {
	return xxx_messageInfo_ValueType.Size(m)
}
func (m *ValueType) XXX_DiscardUnknown() {
	xxx_messageInfo_ValueType.DiscardUnknown(m)
}

var xxx_messageInfo_ValueType proto.InternalMessageInfo

type ValueSpecifier struct {
	Id                   string         `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Value                ValueType_Type `protobuf:"varint,2,opt,name=value,proto3,enum=common.ValueType_Type" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *ValueSpecifier) Reset()         { *m = ValueSpecifier{} }
func (m *ValueSpecifier) String() string { return proto.CompactTextString(m) }
func (*ValueSpecifier) ProtoMessage()    {}
func (*ValueSpecifier) Descriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{7}
}

func (m *ValueSpecifier) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ValueSpecifier.Unmarshal(m, b)
}
func (m *ValueSpecifier) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ValueSpecifier.Marshal(b, m, deterministic)
}
func (m *ValueSpecifier) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ValueSpecifier.Merge(m, src)
}
func (m *ValueSpecifier) XXX_Size() int {
	return xxx_messageInfo_ValueSpecifier.Size(m)
}
func (m *ValueSpecifier) XXX_DiscardUnknown() {
	xxx_messageInfo_ValueSpecifier.DiscardUnknown(m)
}

var xxx_messageInfo_ValueSpecifier proto.InternalMessageInfo

func (m *ValueSpecifier) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *ValueSpecifier) GetValue() ValueType_Type {
	if m != nil {
		return m.Value
	}
	return ValueType_EMPTY
}

type ReturnValues struct {
	Set                  uint32   `protobuf:"varint,1,opt,name=Set,proto3" json:"Set,omitempty"`
	Unsupported          uint32   `protobuf:"varint,2,opt,name=Unsupported,proto3" json:"Unsupported,omitempty"`
	Error                uint32   `protobuf:"varint,3,opt,name=Error,proto3" json:"Error,omitempty"`
	Distance             uint32   `protobuf:"varint,4,opt,name=Distance,proto3" json:"Distance,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReturnValues) Reset()         { *m = ReturnValues{} }
func (m *ReturnValues) String() string { return proto.CompactTextString(m) }
func (*ReturnValues) ProtoMessage()    {}
func (*ReturnValues) Descriptor() ([]byte, []int) {
	return fileDescriptor_c2e3fd231961e826, []int{8}
}

func (m *ReturnValues) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReturnValues.Unmarshal(m, b)
}
func (m *ReturnValues) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReturnValues.Marshal(b, m, deterministic)
}
func (m *ReturnValues) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReturnValues.Merge(m, src)
}
func (m *ReturnValues) XXX_Size() int {
	return xxx_messageInfo_ReturnValues.Size(m)
}
func (m *ReturnValues) XXX_DiscardUnknown() {
	xxx_messageInfo_ReturnValues.DiscardUnknown(m)
}

var xxx_messageInfo_ReturnValues proto.InternalMessageInfo

func (m *ReturnValues) GetSet() uint32 {
	if m != nil {
		return m.Set
	}
	return 0
}

func (m *ReturnValues) GetUnsupported() uint32 {
	if m != nil {
		return m.Unsupported
	}
	return 0
}

func (m *ReturnValues) GetError() uint32 {
	if m != nil {
		return m.Error
	}
	return 0
}

func (m *ReturnValues) GetDistance() uint32 {
	if m != nil {
		return m.Distance
	}
	return 0
}

func init() {
	proto.RegisterEnum("common.TestModeKeys", TestModeKeys_name, TestModeKeys_value)
	proto.RegisterEnum("common.AdminState_Types", AdminState_Types_name, AdminState_Types_value)
	proto.RegisterEnum("common.OperStatus_Types", OperStatus_Types_name, OperStatus_Types_value)
	proto.RegisterEnum("common.ConnectStatus_Types", ConnectStatus_Types_name, ConnectStatus_Types_value)
	proto.RegisterEnum("common.OperationResp_OperationReturnCode", OperationResp_OperationReturnCode_name, OperationResp_OperationReturnCode_value)
	proto.RegisterEnum("common.ValueType_Type", ValueType_Type_name, ValueType_Type_value)
	proto.RegisterType((*ID)(nil), "common.ID")
	proto.RegisterType((*IDs)(nil), "common.IDs")
	proto.RegisterType((*AdminState)(nil), "common.AdminState")
	proto.RegisterType((*OperStatus)(nil), "common.OperStatus")
	proto.RegisterType((*ConnectStatus)(nil), "common.ConnectStatus")
	proto.RegisterType((*OperationResp)(nil), "common.OperationResp")
	proto.RegisterType((*ValueType)(nil), "common.ValueType")
	proto.RegisterType((*ValueSpecifier)(nil), "common.ValueSpecifier")
	proto.RegisterType((*ReturnValues)(nil), "common.ReturnValues")
}

func init() { proto.RegisterFile("voltha_protos/common.proto", fileDescriptor_c2e3fd231961e826) }

var fileDescriptor_c2e3fd231961e826 = []byte{
	// 606 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x53, 0x5d, 0x4f, 0xdb, 0x30,
	0x14, 0x6d, 0x9b, 0x96, 0xd1, 0x5b, 0x1a, 0x32, 0x03, 0x53, 0x87, 0x26, 0xad, 0xca, 0x0b, 0x6c,
	0x62, 0xad, 0xc4, 0x78, 0xdd, 0x43, 0x48, 0xbc, 0xce, 0x02, 0x9c, 0xc8, 0x49, 0x8a, 0xe0, 0xa5,
	0x0a, 0x8d, 0x29, 0x91, 0x68, 0x1c, 0x25, 0x2e, 0x12, 0x7f, 0x7b, 0xbf, 0x60, 0xb2, 0x53, 0xbe,
	0x26, 0x5e, 0x12, 0x9f, 0x7b, 0x4e, 0xee, 0xf1, 0x3d, 0x8e, 0x61, 0xff, 0x41, 0xdc, 0xcb, 0xbb,
	0x64, 0x56, 0x94, 0x42, 0x8a, 0x6a, 0x3c, 0x17, 0xcb, 0xa5, 0xc8, 0x47, 0x1a, 0xa1, 0x8d, 0x1a,
	0xd9, 0xbb, 0xd0, 0x22, 0x1e, 0x32, 0xa1, 0x95, 0xa5, 0x83, 0xe6, 0xb0, 0x79, 0xd8, 0x65, 0xad,
	0x2c, 0xb5, 0x0f, 0xc0, 0x20, 0x5e, 0x85, 0x86, 0xd0, 0xc9, 0x24, 0x5f, 0x56, 0x83, 0xe6, 0xd0,
	0x38, 0xec, 0x1d, 0xc3, 0x68, 0xdd, 0x82, 0x78, 0xac, 0x26, 0xec, 0x3b, 0x00, 0x27, 0x5d, 0x66,
	0x79, 0x28, 0x13, 0xc9, 0xed, 0x6b, 0xe8, 0x44, 0x8f, 0x05, 0xaf, 0x50, 0x0f, 0x3e, 0xc4, 0xf4,
	0x8c, 0xfa, 0x97, 0xd4, 0x6a, 0x20, 0x04, 0x66, 0xc0, 0x70, 0xc0, 0xfc, 0x29, 0x09, 0x89, 0x4f,
	0xb1, 0x67, 0x35, 0x95, 0x00, 0x53, 0xe7, 0xf4, 0x1c, 0x7b, 0x56, 0x0b, 0x6d, 0xc1, 0xa6, 0x47,
	0xc2, 0x1a, 0x19, 0x68, 0x0f, 0x3e, 0x7a, 0xfe, 0x25, 0x3d, 0xf7, 0x1d, 0x8f, 0xd0, 0xc9, 0x8c,
	0x5c, 0x38, 0x13, 0x6c, 0xb5, 0xed, 0x05, 0x80, 0x5f, 0xf0, 0x52, 0x19, 0xad, 0x2a, 0xfb, 0xea,
	0x5d, 0x27, 0x13, 0xc0, 0x23, 0xa1, 0xeb, 0x4f, 0x31, 0xd3, 0x2e, 0x26, 0x80, 0xe3, 0x46, 0x64,
	0xea, 0x44, 0x84, 0x4e, 0xac, 0x96, 0x12, 0x47, 0x38, 0xd4, 0xc0, 0x40, 0x00, 0x1b, 0x9a, 0xc4,
	0x56, 0x5b, 0xad, 0x7f, 0x3b, 0x44, 0xf9, 0x77, 0x6c, 0x0c, 0x7d, 0x57, 0xe4, 0x39, 0x9f, 0xcb,
	0xb5, 0xd7, 0xc9, 0xbb, 0x5e, 0xdb, 0xd0, 0x8b, 0x29, 0xc3, 0x8e, 0xfb, 0x47, 0x6d, 0xdc, 0x6a,
	0xa2, 0x3e, 0x74, 0x5f, 0x60, 0xcb, 0xfe, 0xdb, 0x84, 0xbe, 0xda, 0x70, 0x22, 0x33, 0x91, 0x33,
	0x5e, 0x15, 0xe8, 0x17, 0xb4, 0xe7, 0x22, 0xe5, 0x3a, 0x66, 0xf3, 0xf8, 0xdb, 0x53, 0x98, 0x6f,
	0x44, 0xaf, 0x91, 0x5c, 0x95, 0xb9, 0x2b, 0x52, 0xce, 0xf4, 0x67, 0xe8, 0x00, 0xb6, 0x93, 0x34,
	0xcd, 0x14, 0x97, 0xdc, 0xcf, 0xb2, 0xfc, 0x56, 0x0c, 0x5a, 0xfa, 0xc0, 0xcc, 0x97, 0x32, 0xc9,
	0x6f, 0x85, 0xfd, 0x08, 0x3b, 0xef, 0x74, 0x51, 0xb9, 0xfa, 0x01, 0x66, 0x4e, 0x44, 0x7c, 0x3a,
	0x0b, 0x63, 0xd7, 0xc5, 0x61, 0x68, 0x35, 0xde, 0x96, 0x55, 0x08, 0x31, 0x53, 0xd3, 0x7c, 0x86,
	0xbd, 0x97, 0x72, 0x4c, 0xc3, 0x38, 0x08, 0x7c, 0x16, 0xe9, 0xe3, 0x7a, 0x43, 0x11, 0x3a, 0x0b,
	0x98, 0x3f, 0x61, 0xaa, 0x99, 0x61, 0x1f, 0x41, 0x77, 0x9a, 0xdc, 0xaf, 0xb8, 0xca, 0xcb, 0xfe,
	0x0a, 0x6d, 0xf5, 0x46, 0x5d, 0xe8, 0xe0, 0x8b, 0x20, 0xba, 0xb2, 0x1a, 0xeb, 0x93, 0x8e, 0x1c,
	0xea, 0x62, 0xab, 0x69, 0x53, 0x30, 0xb5, 0x3a, 0x2c, 0xf8, 0x3c, 0xbb, 0xcd, 0x78, 0xf9, 0xff,
	0x7f, 0x88, 0x8e, 0xa0, 0xf3, 0xa0, 0x14, 0x7a, 0x52, 0xf3, 0xf8, 0xd3, 0x53, 0x66, 0xcf, 0x26,
	0x23, 0xf5, 0x60, 0xb5, 0xc8, 0x96, 0xb0, 0x55, 0xcf, 0xab, 0xe9, 0x0a, 0x59, 0x60, 0x84, 0x5c,
	0xea, 0x76, 0x7d, 0xa6, 0x96, 0x68, 0x08, 0xbd, 0x38, 0xaf, 0x56, 0x45, 0x21, 0x4a, 0xc9, 0x53,
	0xdd, 0xb5, 0xcf, 0x5e, 0x97, 0xd0, 0x2e, 0x74, 0x70, 0x59, 0x8a, 0x72, 0x60, 0x68, 0xae, 0x06,
	0x68, 0x1f, 0x36, 0xbd, 0xac, 0x92, 0x49, 0x3e, 0xe7, 0x83, 0xb6, 0x26, 0x9e, 0xf1, 0xf7, 0x2f,
	0xb0, 0x15, 0xf1, 0x4a, 0x5e, 0x88, 0x94, 0x9f, 0xf1, 0xc7, 0x4a, 0xcd, 0x98, 0x14, 0xd9, 0x4c,
	0xf2, 0x4a, 0x5a, 0x8d, 0x53, 0x0c, 0x3b, 0xa2, 0x5c, 0x8c, 0x44, 0xc1, 0xf3, 0xb9, 0x28, 0xd3,
	0x51, 0x7d, 0x25, 0xaf, 0x47, 0x8b, 0x4c, 0xde, 0xad, 0x6e, 0xd4, 0x3c, 0xe3, 0x27, 0x6e, 0x5c,
	0x73, 0x3f, 0xd6, 0xd7, 0xf5, 0xe1, 0x64, 0xbc, 0x10, 0xeb, 0x4b, 0x7b, 0xb3, 0xa1, 0x8b, 0x3f,
	0xff, 0x05, 0x00, 0x00, 0xff, 0xff, 0x27, 0x0a, 0x9c, 0xc8, 0xd3, 0x03, 0x00, 0x00,
}
