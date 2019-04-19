// Code generated by protoc-gen-go. DO NOT EDIT.
// source: voltha_protos/yang_options.proto

package common

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	descriptor "github.com/golang/protobuf/protoc-gen-go/descriptor"
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

type MessageParserOption int32

const (
	// Move any enclosing child enum/message definition to the same level
	// as the parent (this message) in the yang generated file
	MessageParserOption_MOVE_TO_PARENT_LEVEL MessageParserOption = 0
	// Create both a grouping and a container for this message.  The container
	// name will be the message name.  The grouping name will be the message
	// name prefixed with "grouping_"
	MessageParserOption_CREATE_BOTH_GROUPING_AND_CONTAINER MessageParserOption = 1
)

var MessageParserOption_name = map[int32]string{
	0: "MOVE_TO_PARENT_LEVEL",
	1: "CREATE_BOTH_GROUPING_AND_CONTAINER",
}

var MessageParserOption_value = map[string]int32{
	"MOVE_TO_PARENT_LEVEL":               0,
	"CREATE_BOTH_GROUPING_AND_CONTAINER": 1,
}

func (x MessageParserOption) String() string {
	return proto.EnumName(MessageParserOption_name, int32(x))
}

func (MessageParserOption) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_e6be2fba65eb89fb, []int{0}
}

type InlineNode struct {
	Id                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Type                 string   `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *InlineNode) Reset()         { *m = InlineNode{} }
func (m *InlineNode) String() string { return proto.CompactTextString(m) }
func (*InlineNode) ProtoMessage()    {}
func (*InlineNode) Descriptor() ([]byte, []int) {
	return fileDescriptor_e6be2fba65eb89fb, []int{0}
}

func (m *InlineNode) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_InlineNode.Unmarshal(m, b)
}
func (m *InlineNode) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_InlineNode.Marshal(b, m, deterministic)
}
func (m *InlineNode) XXX_Merge(src proto.Message) {
	xxx_messageInfo_InlineNode.Merge(m, src)
}
func (m *InlineNode) XXX_Size() int {
	return xxx_messageInfo_InlineNode.Size(m)
}
func (m *InlineNode) XXX_DiscardUnknown() {
	xxx_messageInfo_InlineNode.DiscardUnknown(m)
}

var xxx_messageInfo_InlineNode proto.InternalMessageInfo

func (m *InlineNode) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *InlineNode) GetType() string {
	if m != nil {
		return m.Type
	}
	return ""
}

type RpcReturnDef struct {
	// The gRPC methods return message types.  NETCONF expects an actual
	// attribute as defined in the YANG schema.  The xnl_tag will be used
	// as the top most tag when translating a gRPC response into an xml
	// response
	XmlTag string `protobuf:"bytes,1,opt,name=xml_tag,json=xmlTag,proto3" json:"xml_tag,omitempty"`
	// When the gRPC response is a list of items, we need to differentiate
	// between a YANG schema attribute whose name is "items" and when "items"
	// is used only to indicate a list of items is being returned.  The default
	// behavior assumes a list is returned when "items" is present in
	// the response.  This option will therefore be used when the attribute
	// name in the YANG schema is 'items'
	ListItemsName        string   `protobuf:"bytes,2,opt,name=list_items_name,json=listItemsName,proto3" json:"list_items_name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RpcReturnDef) Reset()         { *m = RpcReturnDef{} }
func (m *RpcReturnDef) String() string { return proto.CompactTextString(m) }
func (*RpcReturnDef) ProtoMessage()    {}
func (*RpcReturnDef) Descriptor() ([]byte, []int) {
	return fileDescriptor_e6be2fba65eb89fb, []int{1}
}

func (m *RpcReturnDef) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RpcReturnDef.Unmarshal(m, b)
}
func (m *RpcReturnDef) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RpcReturnDef.Marshal(b, m, deterministic)
}
func (m *RpcReturnDef) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RpcReturnDef.Merge(m, src)
}
func (m *RpcReturnDef) XXX_Size() int {
	return xxx_messageInfo_RpcReturnDef.Size(m)
}
func (m *RpcReturnDef) XXX_DiscardUnknown() {
	xxx_messageInfo_RpcReturnDef.DiscardUnknown(m)
}

var xxx_messageInfo_RpcReturnDef proto.InternalMessageInfo

func (m *RpcReturnDef) GetXmlTag() string {
	if m != nil {
		return m.XmlTag
	}
	return ""
}

func (m *RpcReturnDef) GetListItemsName() string {
	if m != nil {
		return m.ListItemsName
	}
	return ""
}

var E_YangChildRule = &proto.ExtensionDesc{
	ExtendedType:  (*descriptor.MessageOptions)(nil),
	ExtensionType: (*MessageParserOption)(nil),
	Field:         7761774,
	Name:          "common.yang_child_rule",
	Tag:           "varint,7761774,opt,name=yang_child_rule,enum=common.MessageParserOption",
	Filename:      "voltha_protos/yang_options.proto",
}

var E_YangMessageRule = &proto.ExtensionDesc{
	ExtendedType:  (*descriptor.MessageOptions)(nil),
	ExtensionType: (*MessageParserOption)(nil),
	Field:         7761775,
	Name:          "common.yang_message_rule",
	Tag:           "varint,7761775,opt,name=yang_message_rule,enum=common.MessageParserOption",
	Filename:      "voltha_protos/yang_options.proto",
}

var E_YangInlineNode = &proto.ExtensionDesc{
	ExtendedType:  (*descriptor.FieldOptions)(nil),
	ExtensionType: (*InlineNode)(nil),
	Field:         7761776,
	Name:          "common.yang_inline_node",
	Tag:           "bytes,7761776,opt,name=yang_inline_node",
	Filename:      "voltha_protos/yang_options.proto",
}

var E_YangXmlTag = &proto.ExtensionDesc{
	ExtendedType:  (*descriptor.MethodOptions)(nil),
	ExtensionType: (*RpcReturnDef)(nil),
	Field:         7761777,
	Name:          "common.yang_xml_tag",
	Tag:           "bytes,7761777,opt,name=yang_xml_tag",
	Filename:      "voltha_protos/yang_options.proto",
}

func init() {
	proto.RegisterEnum("common.MessageParserOption", MessageParserOption_name, MessageParserOption_value)
	proto.RegisterType((*InlineNode)(nil), "common.InlineNode")
	proto.RegisterType((*RpcReturnDef)(nil), "common.RpcReturnDef")
	proto.RegisterExtension(E_YangChildRule)
	proto.RegisterExtension(E_YangMessageRule)
	proto.RegisterExtension(E_YangInlineNode)
	proto.RegisterExtension(E_YangXmlTag)
}

func init() { proto.RegisterFile("voltha_protos/yang_options.proto", fileDescriptor_e6be2fba65eb89fb) }

var fileDescriptor_e6be2fba65eb89fb = []byte{
	// 452 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x92, 0x4d, 0x6f, 0xd3, 0x30,
	0x18, 0xc7, 0x69, 0x41, 0x45, 0x98, 0xad, 0x2b, 0x66, 0x12, 0x15, 0x08, 0xa8, 0x7a, 0x98, 0x26,
	0xd0, 0x12, 0x34, 0x6e, 0xbd, 0x75, 0x5d, 0x18, 0x95, 0xb6, 0xa4, 0xb2, 0xc2, 0x78, 0x39, 0x60,
	0xa5, 0xc9, 0x33, 0xc7, 0xc2, 0xb1, 0xa3, 0xd8, 0x41, 0xdb, 0x47, 0xe5, 0xc2, 0x47, 0xe0, 0xe5,
	0x1b, 0xa0, 0xd8, 0x09, 0x43, 0x62, 0x87, 0xde, 0xda, 0x7f, 0x9e, 0xfc, 0x7e, 0x79, 0x5e, 0xd0,
	0xe4, 0xab, 0x12, 0x26, 0x4f, 0x68, 0x59, 0x29, 0xa3, 0xb4, 0x7f, 0x95, 0x48, 0x46, 0x55, 0x69,
	0xb8, 0x92, 0xda, 0xb3, 0x19, 0x1e, 0xa4, 0xaa, 0x28, 0x94, 0x7c, 0x3c, 0x61, 0x4a, 0x31, 0x01,
	0xbe, 0x4d, 0xd7, 0xf5, 0x85, 0x9f, 0x81, 0x4e, 0x2b, 0x5e, 0x1a, 0x55, 0xb9, 0xca, 0xe9, 0x2b,
	0x84, 0x96, 0x52, 0x70, 0x09, 0xa1, 0xca, 0x00, 0x0f, 0x51, 0x9f, 0x67, 0xe3, 0xde, 0xa4, 0xb7,
	0x7f, 0x8f, 0xf4, 0x79, 0x86, 0x31, 0xba, 0x63, 0xae, 0x4a, 0x18, 0xf7, 0x6d, 0x62, 0x7f, 0x4f,
	0x23, 0xb4, 0x45, 0xca, 0x94, 0x80, 0xa9, 0x2b, 0x79, 0x0c, 0x17, 0xf8, 0x11, 0xba, 0x7b, 0x59,
	0x08, 0x6a, 0x12, 0xd6, 0xbe, 0x38, 0xb8, 0x2c, 0x44, 0x9c, 0x30, 0xbc, 0x87, 0x76, 0x04, 0xd7,
	0x86, 0x72, 0x03, 0x85, 0xa6, 0x32, 0x29, 0x3a, 0xce, 0x76, 0x13, 0x2f, 0x9b, 0x34, 0x4c, 0x0a,
	0x78, 0xf1, 0x1e, 0x3d, 0x3c, 0x03, 0xad, 0x13, 0x06, 0xab, 0xa4, 0xd2, 0x50, 0x45, 0xb6, 0x15,
	0x3c, 0x46, 0xbb, 0x67, 0xd1, 0x79, 0x40, 0xe3, 0x88, 0xae, 0xe6, 0x24, 0x08, 0x63, 0x7a, 0x1a,
	0x9c, 0x07, 0xa7, 0xa3, 0x5b, 0x78, 0x0f, 0x4d, 0x17, 0x24, 0x98, 0xc7, 0x01, 0x3d, 0x8a, 0xe2,
	0xb7, 0xf4, 0x84, 0x44, 0xef, 0x56, 0xcb, 0xf0, 0x84, 0xce, 0xc3, 0x63, 0xba, 0x88, 0xc2, 0x78,
	0xbe, 0x0c, 0x03, 0x32, 0xea, 0xcd, 0x18, 0xda, 0xb1, 0xb3, 0x49, 0x73, 0x2e, 0x32, 0x5a, 0xd5,
	0x02, 0xf0, 0x73, 0xcf, 0x4d, 0xc4, 0xeb, 0x26, 0xe2, 0xb5, 0x6a, 0x27, 0xd5, 0xe3, 0x1f, 0xdf,
	0xbf, 0xdd, 0x9e, 0xf4, 0xf6, 0x87, 0x87, 0x4f, 0x3c, 0x37, 0x43, 0xef, 0x86, 0x6f, 0x23, 0xdb,
	0x0d, 0x77, 0xd1, 0x60, 0x49, 0x2d, 0x60, 0xf6, 0x05, 0x3d, 0xb0, 0xa2, 0xc2, 0x95, 0x6e, 0xa8,
	0xfa, 0xb9, 0x91, 0xca, 0xb6, 0xd0, 0x3e, 0xb0, 0xb2, 0xcf, 0x68, 0x64, 0x65, 0xdc, 0xae, 0x8d,
	0xca, 0x66, 0x6f, 0x4f, 0xff, 0x73, 0xbd, 0xe1, 0x20, 0xb2, 0xce, 0xf4, 0xcb, 0x99, 0xee, 0x1f,
	0xe2, 0xce, 0x74, 0xbd, 0x73, 0x32, 0x6c, 0x68, 0xd7, 0xff, 0x67, 0x1f, 0xd1, 0x96, 0xe5, 0xb7,
	0x4b, 0xc5, 0xcf, 0x6e, 0xe8, 0xc3, 0xe4, 0xea, 0x2f, 0xfc, 0x77, 0x07, 0xdf, 0xed, 0xe0, 0xff,
	0x9e, 0x07, 0x41, 0x0d, 0xec, 0x83, 0xbd, 0x88, 0xa3, 0x83, 0x4f, 0x2f, 0x19, 0x37, 0x79, 0xbd,
	0x6e, 0x2a, 0x7d, 0x55, 0x82, 0x4c, 0x55, 0x95, 0xf9, 0xee, 0x9c, 0x0f, 0xda, 0x73, 0x66, 0xca,
	0x77, 0x9c, 0xf5, 0xc0, 0x26, 0xaf, 0xff, 0x04, 0x00, 0x00, 0xff, 0xff, 0x45, 0xe4, 0xb5, 0xec,
	0xf0, 0x02, 0x00, 0x00,
}
