// Code generated by protoc-gen-go. DO NOT EDIT.
// source: proto/nanoservice_discovery.proto

package service_discovery

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Nanoservice struct {
	Port                 int32    `protobuf:"varint,1,opt,name=port,proto3" json:"port,omitempty"`
	TimeToLiveInMS       int64    `protobuf:"varint,2,opt,name=timeToLiveInMS,proto3" json:"timeToLiveInMS,omitempty"`
	StartTime            int64    `protobuf:"varint,3,opt,name=startTime,proto3" json:"startTime,omitempty"`
	HostName             string   `protobuf:"bytes,4,opt,name=hostName,proto3" json:"hostName,omitempty"`
	ServiceName          string   `protobuf:"bytes,5,opt,name=serviceName,proto3" json:"serviceName,omitempty"`
	ServiceType          string   `protobuf:"bytes,6,opt,name=serviceType,proto3" json:"serviceType,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Nanoservice) Reset()         { *m = Nanoservice{} }
func (m *Nanoservice) String() string { return proto.CompactTextString(m) }
func (*Nanoservice) ProtoMessage()    {}
func (*Nanoservice) Descriptor() ([]byte, []int) {
	return fileDescriptor_nanoservice_discovery_670e47c5b677dd2d, []int{0}
}
func (m *Nanoservice) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Nanoservice.Unmarshal(m, b)
}
func (m *Nanoservice) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Nanoservice.Marshal(b, m, deterministic)
}
func (dst *Nanoservice) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Nanoservice.Merge(dst, src)
}
func (m *Nanoservice) XXX_Size() int {
	return xxx_messageInfo_Nanoservice.Size(m)
}
func (m *Nanoservice) XXX_DiscardUnknown() {
	xxx_messageInfo_Nanoservice.DiscardUnknown(m)
}

var xxx_messageInfo_Nanoservice proto.InternalMessageInfo

func (m *Nanoservice) GetPort() int32 {
	if m != nil {
		return m.Port
	}
	return 0
}

func (m *Nanoservice) GetTimeToLiveInMS() int64 {
	if m != nil {
		return m.TimeToLiveInMS
	}
	return 0
}

func (m *Nanoservice) GetStartTime() int64 {
	if m != nil {
		return m.StartTime
	}
	return 0
}

func (m *Nanoservice) GetHostName() string {
	if m != nil {
		return m.HostName
	}
	return ""
}

func (m *Nanoservice) GetServiceName() string {
	if m != nil {
		return m.ServiceName
	}
	return ""
}

func (m *Nanoservice) GetServiceType() string {
	if m != nil {
		return m.ServiceType
	}
	return ""
}

type NanoserviceList struct {
	ServiceType          string         `protobuf:"bytes,2,opt,name=serviceType,proto3" json:"serviceType,omitempty"`
	ServicesAvailable    []*Nanoservice `protobuf:"bytes,3,rep,name=servicesAvailable,proto3" json:"servicesAvailable,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *NanoserviceList) Reset()         { *m = NanoserviceList{} }
func (m *NanoserviceList) String() string { return proto.CompactTextString(m) }
func (*NanoserviceList) ProtoMessage()    {}
func (*NanoserviceList) Descriptor() ([]byte, []int) {
	return fileDescriptor_nanoservice_discovery_670e47c5b677dd2d, []int{1}
}
func (m *NanoserviceList) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NanoserviceList.Unmarshal(m, b)
}
func (m *NanoserviceList) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NanoserviceList.Marshal(b, m, deterministic)
}
func (dst *NanoserviceList) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NanoserviceList.Merge(dst, src)
}
func (m *NanoserviceList) XXX_Size() int {
	return xxx_messageInfo_NanoserviceList.Size(m)
}
func (m *NanoserviceList) XXX_DiscardUnknown() {
	xxx_messageInfo_NanoserviceList.DiscardUnknown(m)
}

var xxx_messageInfo_NanoserviceList proto.InternalMessageInfo

func (m *NanoserviceList) GetServiceType() string {
	if m != nil {
		return m.ServiceType
	}
	return ""
}

func (m *NanoserviceList) GetServicesAvailable() []*Nanoservice {
	if m != nil {
		return m.ServicesAvailable
	}
	return nil
}

func init() {
	proto.RegisterType((*Nanoservice)(nil), "service_discovery.Nanoservice")
	proto.RegisterType((*NanoserviceList)(nil), "service_discovery.NanoserviceList")
}

func init() {
	proto.RegisterFile("proto/nanoservice_discovery.proto", fileDescriptor_nanoservice_discovery_670e47c5b677dd2d)
}

var fileDescriptor_nanoservice_discovery_670e47c5b677dd2d = []byte{
	// 235 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x52, 0x2c, 0x28, 0xca, 0x2f,
	0xc9, 0xd7, 0xcf, 0x4b, 0xcc, 0xcb, 0x2f, 0x4e, 0x2d, 0x2a, 0xcb, 0x4c, 0x4e, 0x8d, 0x4f, 0xc9,
	0x2c, 0x4e, 0xce, 0x2f, 0x4b, 0x2d, 0xaa, 0xd4, 0x03, 0xcb, 0x09, 0x09, 0x62, 0x48, 0x28, 0x1d,
	0x67, 0xe4, 0xe2, 0xf6, 0x43, 0x68, 0x11, 0x12, 0xe2, 0x62, 0x29, 0xc8, 0x2f, 0x2a, 0x91, 0x60,
	0x54, 0x60, 0xd4, 0x60, 0x0d, 0x02, 0xb3, 0x85, 0xd4, 0xb8, 0xf8, 0x4a, 0x32, 0x73, 0x53, 0x43,
	0xf2, 0x7d, 0x32, 0xcb, 0x52, 0x3d, 0xf3, 0x7c, 0x83, 0x25, 0x98, 0x14, 0x18, 0x35, 0x98, 0x83,
	0xd0, 0x44, 0x85, 0x64, 0xb8, 0x38, 0x8b, 0x4b, 0x12, 0x8b, 0x4a, 0x42, 0x32, 0x73, 0x53, 0x25,
	0x98, 0xc1, 0x4a, 0x10, 0x02, 0x42, 0x52, 0x5c, 0x1c, 0x19, 0xf9, 0xc5, 0x25, 0x7e, 0x89, 0xb9,
	0xa9, 0x12, 0x2c, 0x0a, 0x8c, 0x1a, 0x9c, 0x41, 0x70, 0xbe, 0x90, 0x02, 0x17, 0x37, 0xd4, 0x01,
	0x60, 0x69, 0x56, 0xb0, 0x34, 0xb2, 0x10, 0x92, 0x8a, 0x90, 0xca, 0x82, 0x54, 0x09, 0x36, 0x14,
	0x15, 0x20, 0x21, 0xa5, 0x46, 0x46, 0x2e, 0x7e, 0x24, 0x9f, 0xf8, 0x64, 0x16, 0x97, 0xa0, 0xeb,
	0x62, 0xc2, 0xd0, 0x25, 0xe4, 0xc3, 0x05, 0x0b, 0x94, 0x62, 0xc7, 0xb2, 0xc4, 0xcc, 0x9c, 0xc4,
	0xa4, 0x1c, 0x90, 0xdb, 0x99, 0x35, 0xb8, 0x8d, 0xe4, 0xf4, 0x30, 0xc3, 0x11, 0xc9, 0x82, 0x20,
	0x4c, 0x8d, 0x4e, 0xc2, 0x51, 0x98, 0x41, 0x9c, 0xc4, 0x06, 0x0e, 0x7c, 0x63, 0x40, 0x00, 0x00,
	0x00, 0xff, 0xff, 0xab, 0x90, 0x5b, 0x61, 0xa1, 0x01, 0x00, 0x00,
}