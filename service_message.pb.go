// Code generated by protoc-gen-go. DO NOT EDIT.
// source: service_message.proto

package service_discovery

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import any "github.com/golang/protobuf/ptypes/any"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type ServiceMessage struct {
	Message              *any.Any `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ServiceMessage) Reset()         { *m = ServiceMessage{} }
func (m *ServiceMessage) String() string { return proto.CompactTextString(m) }
func (*ServiceMessage) ProtoMessage()    {}
func (*ServiceMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_service_message_1d912c92ea4b5ca1, []int{0}
}
func (m *ServiceMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ServiceMessage.Unmarshal(m, b)
}
func (m *ServiceMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ServiceMessage.Marshal(b, m, deterministic)
}
func (dst *ServiceMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ServiceMessage.Merge(dst, src)
}
func (m *ServiceMessage) XXX_Size() int {
	return xxx_messageInfo_ServiceMessage.Size(m)
}
func (m *ServiceMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_ServiceMessage.DiscardUnknown(m)
}

var xxx_messageInfo_ServiceMessage proto.InternalMessageInfo

func (m *ServiceMessage) GetMessage() *any.Any {
	if m != nil {
		return m.Message
	}
	return nil
}

func init() {
	proto.RegisterType((*ServiceMessage)(nil), "service_discovery.ServiceMessage")
}

func init() {
	proto.RegisterFile("service_message.proto", fileDescriptor_service_message_1d912c92ea4b5ca1)
}

var fileDescriptor_service_message_1d912c92ea4b5ca1 = []byte{
	// 127 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x2d, 0x4e, 0x2d, 0x2a,
	0xcb, 0x4c, 0x4e, 0x8d, 0xcf, 0x4d, 0x2d, 0x2e, 0x4e, 0x4c, 0x4f, 0xd5, 0x2b, 0x28, 0xca, 0x2f,
	0xc9, 0x17, 0x12, 0x84, 0x09, 0xa7, 0x64, 0x16, 0x27, 0xe7, 0x97, 0xa5, 0x16, 0x55, 0x4a, 0x49,
	0xa6, 0xe7, 0xe7, 0xa7, 0xe7, 0xa4, 0xea, 0x83, 0x15, 0x24, 0x95, 0xa6, 0xe9, 0x27, 0xe6, 0x55,
	0x42, 0x54, 0x2b, 0x39, 0x70, 0xf1, 0x05, 0x43, 0xd4, 0xfb, 0x42, 0x4c, 0x11, 0xd2, 0xe3, 0x62,
	0x87, 0x1a, 0x28, 0xc1, 0xa8, 0xc0, 0xa8, 0xc1, 0x6d, 0x24, 0xa2, 0x07, 0xd1, 0xae, 0x07, 0xd3,
	0xae, 0xe7, 0x98, 0x57, 0x19, 0x04, 0x53, 0xe4, 0x24, 0x1c, 0x85, 0x69, 0x63, 0x12, 0x1b, 0x58,
	0xad, 0x31, 0x20, 0x00, 0x00, 0xff, 0xff, 0xb2, 0x5a, 0x2c, 0xd2, 0xa4, 0x00, 0x00, 0x00,
}
