// Code generated by protoc-gen-go. DO NOT EDIT.
// source: mdns_definition.proto

package nan0

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

type MDefinition struct {
	ConnectionInfo        *ConnectionInfo `protobuf:"bytes,1,opt,name=connectionInfo,proto3" json:"connectionInfo,omitempty"`
	SupportedMessageTypes []string        `protobuf:"bytes,3,rep,name=supportedMessageTypes,proto3" json:"supportedMessageTypes,omitempty"`
	XXX_NoUnkeyedLiteral  struct{}        `json:"-"`
	XXX_unrecognized      []byte          `json:"-"`
	XXX_sizecache         int32           `json:"-"`
}

func (m *MDefinition) Reset()         { *m = MDefinition{} }
func (m *MDefinition) String() string { return proto.CompactTextString(m) }
func (*MDefinition) ProtoMessage()    {}
func (*MDefinition) Descriptor() ([]byte, []int) {
	return fileDescriptor_59d9f359e4b577c2, []int{0}
}

func (m *MDefinition) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MDefinition.Unmarshal(m, b)
}
func (m *MDefinition) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MDefinition.Marshal(b, m, deterministic)
}
func (m *MDefinition) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MDefinition.Merge(m, src)
}
func (m *MDefinition) XXX_Size() int {
	return xxx_messageInfo_MDefinition.Size(m)
}
func (m *MDefinition) XXX_DiscardUnknown() {
	xxx_messageInfo_MDefinition.DiscardUnknown(m)
}

var xxx_messageInfo_MDefinition proto.InternalMessageInfo

func (m *MDefinition) GetConnectionInfo() *ConnectionInfo {
	if m != nil {
		return m.ConnectionInfo
	}
	return nil
}

func (m *MDefinition) GetSupportedMessageTypes() []string {
	if m != nil {
		return m.SupportedMessageTypes
	}
	return nil
}

type ConnectionInfo struct {
	Websocket            bool     `protobuf:"varint,1,opt,name=websocket,proto3" json:"websocket,omitempty"`
	Port                 int32    `protobuf:"varint,3,opt,name=port,proto3" json:"port,omitempty"`
	HostName             string   `protobuf:"bytes,5,opt,name=hostName,proto3" json:"hostName,omitempty"`
	Uri                  string   `protobuf:"bytes,7,opt,name=uri,proto3" json:"uri,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ConnectionInfo) Reset()         { *m = ConnectionInfo{} }
func (m *ConnectionInfo) String() string { return proto.CompactTextString(m) }
func (*ConnectionInfo) ProtoMessage()    {}
func (*ConnectionInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_59d9f359e4b577c2, []int{1}
}

func (m *ConnectionInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ConnectionInfo.Unmarshal(m, b)
}
func (m *ConnectionInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ConnectionInfo.Marshal(b, m, deterministic)
}
func (m *ConnectionInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConnectionInfo.Merge(m, src)
}
func (m *ConnectionInfo) XXX_Size() int {
	return xxx_messageInfo_ConnectionInfo.Size(m)
}
func (m *ConnectionInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_ConnectionInfo.DiscardUnknown(m)
}

var xxx_messageInfo_ConnectionInfo proto.InternalMessageInfo

func (m *ConnectionInfo) GetWebsocket() bool {
	if m != nil {
		return m.Websocket
	}
	return false
}

func (m *ConnectionInfo) GetPort() int32 {
	if m != nil {
		return m.Port
	}
	return 0
}

func (m *ConnectionInfo) GetHostName() string {
	if m != nil {
		return m.HostName
	}
	return ""
}

func (m *ConnectionInfo) GetUri() string {
	if m != nil {
		return m.Uri
	}
	return ""
}

func init() {
	proto.RegisterType((*MDefinition)(nil), "nan0.MDefinition")
	proto.RegisterType((*ConnectionInfo)(nil), "nan0.ConnectionInfo")
}

func init() { proto.RegisterFile("mdns_definition.proto", fileDescriptor_59d9f359e4b577c2) }

var fileDescriptor_59d9f359e4b577c2 = []byte{
	// 233 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x50, 0xbf, 0x4b, 0x03, 0x31,
	0x14, 0x26, 0x5c, 0x4f, 0x7b, 0xaf, 0x50, 0x24, 0x58, 0x88, 0xe2, 0x70, 0x74, 0xca, 0x14, 0x44,
	0x1d, 0x9d, 0xaa, 0x8b, 0x43, 0x1d, 0x82, 0x93, 0x8b, 0x5c, 0xef, 0x5e, 0x35, 0x96, 0xcb, 0x0b,
	0x97, 0x14, 0x71, 0xf4, 0x3f, 0x97, 0xbc, 0xa1, 0x72, 0xd2, 0xed, 0xfb, 0x95, 0x8f, 0xbc, 0x0f,
	0x16, 0x7d, 0xe7, 0xe3, 0x5b, 0x87, 0x5b, 0xe7, 0x5d, 0x72, 0xe4, 0x4d, 0x18, 0x28, 0x91, 0x9c,
	0xf8, 0xc6, 0x5f, 0x2f, 0x7f, 0x04, 0xcc, 0xd6, 0x8f, 0x07, 0x4f, 0xde, 0xc3, 0xbc, 0x25, 0xef,
	0xb1, 0xcd, 0xec, 0xc9, 0x6f, 0x49, 0x89, 0x5a, 0xe8, 0xd9, 0xcd, 0xb9, 0xc9, 0x71, 0xf3, 0x30,
	0xf2, 0xec, 0xbf, 0xac, 0xbc, 0x83, 0x45, 0xdc, 0x87, 0x40, 0x43, 0xc2, 0x6e, 0x8d, 0x31, 0x36,
	0xef, 0xf8, 0xf2, 0x1d, 0x30, 0xaa, 0xa2, 0x2e, 0x74, 0x65, 0x8f, 0x9b, 0xcb, 0x00, 0xf3, 0x71,
	0xaf, 0xbc, 0x82, 0xea, 0x0b, 0x37, 0x91, 0xda, 0x1d, 0x26, 0xfe, 0xc0, 0xd4, 0xfe, 0x09, 0x52,
	0xc2, 0x24, 0xb7, 0xa8, 0xa2, 0x16, 0xba, 0xb4, 0x8c, 0xe5, 0x25, 0x4c, 0x3f, 0x28, 0xa6, 0xe7,
	0xa6, 0x47, 0x55, 0xd6, 0x42, 0x57, 0xf6, 0xc0, 0xe5, 0x19, 0x14, 0xfb, 0xc1, 0xa9, 0x53, 0x96,
	0x33, 0x5c, 0x69, 0xb8, 0x68, 0xa9, 0x37, 0x3b, 0xea, 0x30, 0xe6, 0xe7, 0xe6, 0x93, 0xaf, 0xe3,
	0x61, 0x56, 0x25, 0x93, 0x57, 0xde, 0x67, 0x73, 0xc2, 0xda, 0xed, 0x6f, 0x00, 0x00, 0x00, 0xff,
	0xff, 0xc4, 0xfb, 0xd3, 0x56, 0x45, 0x01, 0x00, 0x00,
}