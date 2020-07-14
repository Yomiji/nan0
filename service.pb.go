// Code generated by protoc-gen-go. DO NOT EDIT.
// source: service.proto

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

type Service struct {
	Expired              bool     `protobuf:"varint,1,opt,name=expired,proto3" json:"expired,omitempty"`
	Port                 int32    `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
	MdnsPort             int32    `protobuf:"varint,3,opt,name=mdnsPort,proto3" json:"mdnsPort,omitempty"`
	StartTime            int64    `protobuf:"varint,4,opt,name=startTime,proto3" json:"startTime,omitempty"`
	HostName             string   `protobuf:"bytes,5,opt,name=hostName,proto3" json:"hostName,omitempty"`
	Uri                  string   `protobuf:"bytes,6,opt,name=uri,proto3" json:"uri,omitempty"`
	ServiceName          string   `protobuf:"bytes,7,opt,name=serviceName,proto3" json:"serviceName,omitempty"`
	ServiceType          string   `protobuf:"bytes,8,opt,name=serviceType,proto3" json:"serviceType,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Service) Reset()         { *m = Service{} }
func (m *Service) String() string { return proto.CompactTextString(m) }
func (*Service) ProtoMessage()    {}
func (*Service) Descriptor() ([]byte, []int) {
	return fileDescriptor_a0b84a42fa06f626, []int{0}
}

func (m *Service) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Service.Unmarshal(m, b)
}
func (m *Service) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Service.Marshal(b, m, deterministic)
}
func (m *Service) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Service.Merge(m, src)
}
func (m *Service) XXX_Size() int {
	return xxx_messageInfo_Service.Size(m)
}
func (m *Service) XXX_DiscardUnknown() {
	xxx_messageInfo_Service.DiscardUnknown(m)
}

var xxx_messageInfo_Service proto.InternalMessageInfo

func (m *Service) GetExpired() bool {
	if m != nil {
		return m.Expired
	}
	return false
}

func (m *Service) GetPort() int32 {
	if m != nil {
		return m.Port
	}
	return 0
}

func (m *Service) GetMdnsPort() int32 {
	if m != nil {
		return m.MdnsPort
	}
	return 0
}

func (m *Service) GetStartTime() int64 {
	if m != nil {
		return m.StartTime
	}
	return 0
}

func (m *Service) GetHostName() string {
	if m != nil {
		return m.HostName
	}
	return ""
}

func (m *Service) GetUri() string {
	if m != nil {
		return m.Uri
	}
	return ""
}

func (m *Service) GetServiceName() string {
	if m != nil {
		return m.ServiceName
	}
	return ""
}

func (m *Service) GetServiceType() string {
	if m != nil {
		return m.ServiceType
	}
	return ""
}

func init() {
	proto.RegisterType((*Service)(nil), "nan0.Service")
}

func init() { proto.RegisterFile("service.proto", fileDescriptor_a0b84a42fa06f626) }

var fileDescriptor_a0b84a42fa06f626 = []byte{
	// 214 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x54, 0xd0, 0x41, 0x4a, 0xc5, 0x30,
	0x10, 0x06, 0x60, 0x62, 0xdb, 0xd7, 0xbe, 0x11, 0x41, 0x66, 0x15, 0xc5, 0x45, 0x70, 0x95, 0x55,
	0x11, 0xbc, 0xc1, 0x3b, 0x80, 0x48, 0x7c, 0x2b, 0x77, 0xb5, 0x1d, 0x30, 0x4a, 0x9a, 0x90, 0x44,
	0xd1, 0xf3, 0x7a, 0x11, 0xc9, 0x54, 0x6b, 0xdd, 0xcd, 0xff, 0x65, 0x06, 0x26, 0x03, 0x67, 0x89,
	0xe2, 0xbb, 0x1d, 0xa9, 0x0f, 0xd1, 0x67, 0x8f, 0xf5, 0x3c, 0xcc, 0x37, 0xd7, 0x5f, 0x02, 0xda,
	0x87, 0xc5, 0x51, 0x42, 0x4b, 0x1f, 0xc1, 0x46, 0x9a, 0xa4, 0x50, 0x42, 0x77, 0xe6, 0x37, 0x22,
	0x42, 0x1d, 0x7c, 0xcc, 0xf2, 0x44, 0x09, 0xdd, 0x18, 0xae, 0xf1, 0x12, 0x3a, 0x37, 0xcd, 0xe9,
	0xbe, 0x78, 0xc5, 0xbe, 0x66, 0xbc, 0x82, 0x7d, 0xca, 0x43, 0xcc, 0x47, 0xeb, 0x48, 0xd6, 0x4a,
	0xe8, 0xca, 0xfc, 0x41, 0x99, 0x7c, 0xf6, 0x29, 0xdf, 0x0d, 0x8e, 0x64, 0xa3, 0x84, 0xde, 0x9b,
	0x35, 0xe3, 0x39, 0x54, 0x6f, 0xd1, 0xca, 0x1d, 0x73, 0x29, 0x51, 0xc1, 0xe9, 0xcf, 0xe2, 0x3c,
	0xd0, 0xf2, 0xcb, 0x96, 0x36, 0x1d, 0xc7, 0xcf, 0x40, 0xb2, 0xfb, 0xd7, 0x51, 0xe8, 0xa0, 0xe1,
	0x62, 0xf4, 0xae, 0x7f, 0xf5, 0x13, 0xa5, 0xb2, 0x7c, 0xff, 0x52, 0x3e, 0xbf, 0x1c, 0xe2, 0xd0,
	0x70, 0x78, 0xe4, 0x7b, 0x3c, 0xed, 0xd8, 0x6e, 0xbf, 0x03, 0x00, 0x00, 0xff, 0xff, 0xf6, 0xc5,
	0x49, 0x3d, 0x2d, 0x01, 0x00, 0x00,
}
