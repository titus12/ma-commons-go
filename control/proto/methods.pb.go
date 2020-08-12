// Code generated by protoc-gen-go. DO NOT EDIT.
// source: methods.proto

package control

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

type Request struct {
	NodeKey              string   `protobuf:"bytes,1,opt,name=nodeKey,proto3" json:"nodeKey,omitempty"`
	Addr                 string   `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Request) Reset()         { *m = Request{} }
func (m *Request) String() string { return proto.CompactTextString(m) }
func (*Request) ProtoMessage()    {}
func (*Request) Descriptor() ([]byte, []int) {
	return fileDescriptor_2b67352c8413ff6d, []int{0}
}

func (m *Request) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Request.Unmarshal(m, b)
}
func (m *Request) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Request.Marshal(b, m, deterministic)
}
func (m *Request) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Request.Merge(m, src)
}
func (m *Request) XXX_Size() int {
	return xxx_messageInfo_Request.Size(m)
}
func (m *Request) XXX_DiscardUnknown() {
	xxx_messageInfo_Request.DiscardUnknown(m)
}

var xxx_messageInfo_Request proto.InternalMessageInfo

func (m *Request) GetNodeKey() string {
	if m != nil {
		return m.NodeKey
	}
	return ""
}

func (m *Request) GetAddr() string {
	if m != nil {
		return m.Addr
	}
	return ""
}

type Response struct {
	NodeKey              string   `protobuf:"bytes,1,opt,name=nodeKey,proto3" json:"nodeKey,omitempty"`
	ErrInfo              string   `protobuf:"bytes,2,opt,name=errInfo,proto3" json:"errInfo,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Response) Reset()         { *m = Response{} }
func (m *Response) String() string { return proto.CompactTextString(m) }
func (*Response) ProtoMessage()    {}
func (*Response) Descriptor() ([]byte, []int) {
	return fileDescriptor_2b67352c8413ff6d, []int{1}
}

func (m *Response) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Response.Unmarshal(m, b)
}
func (m *Response) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Response.Marshal(b, m, deterministic)
}
func (m *Response) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Response.Merge(m, src)
}
func (m *Response) XXX_Size() int {
	return xxx_messageInfo_Response.Size(m)
}
func (m *Response) XXX_DiscardUnknown() {
	xxx_messageInfo_Response.DiscardUnknown(m)
}

var xxx_messageInfo_Response proto.InternalMessageInfo

func (m *Response) GetNodeKey() string {
	if m != nil {
		return m.NodeKey
	}
	return ""
}

func (m *Response) GetErrInfo() string {
	if m != nil {
		return m.ErrInfo
	}
	return ""
}

func init() {
	proto.RegisterType((*Request)(nil), "control.Request")
	proto.RegisterType((*Response)(nil), "control.Response")
}

func init() { proto.RegisterFile("methods.proto", fileDescriptor_2b67352c8413ff6d) }

var fileDescriptor_2b67352c8413ff6d = []byte{
	// 169 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0xcd, 0x4d, 0x2d, 0xc9,
	0xc8, 0x4f, 0x29, 0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x4f, 0xce, 0xcf, 0x2b, 0x29,
	0xca, 0xcf, 0x51, 0x32, 0xe7, 0x62, 0x0f, 0x4a, 0x2d, 0x2c, 0x4d, 0x2d, 0x2e, 0x11, 0x92, 0xe0,
	0x62, 0xcf, 0xcb, 0x4f, 0x49, 0xf5, 0x4e, 0xad, 0x94, 0x60, 0x54, 0x60, 0xd4, 0xe0, 0x0c, 0x82,
	0x71, 0x85, 0x84, 0xb8, 0x58, 0x12, 0x53, 0x52, 0x8a, 0x24, 0x98, 0xc0, 0xc2, 0x60, 0xb6, 0x92,
	0x1d, 0x17, 0x47, 0x50, 0x6a, 0x71, 0x41, 0x7e, 0x5e, 0x71, 0x2a, 0x1e, 0x9d, 0x12, 0x5c, 0xec,
	0xa9, 0x45, 0x45, 0x9e, 0x79, 0x69, 0xf9, 0x50, 0xcd, 0x30, 0xae, 0x91, 0x33, 0x17, 0x9f, 0x33,
	0xc4, 0x0d, 0xc1, 0xa9, 0x45, 0x65, 0x99, 0xc9, 0xa9, 0x42, 0x86, 0x5c, 0x1c, 0xc1, 0x25, 0xf9,
	0x05, 0x7e, 0xf9, 0x29, 0xa9, 0x42, 0x02, 0x7a, 0x50, 0x07, 0xea, 0x41, 0x5d, 0x27, 0x25, 0x88,
	0x24, 0x02, 0xb1, 0x56, 0x89, 0x21, 0x89, 0x0d, 0xec, 0x1b, 0x63, 0x40, 0x00, 0x00, 0x00, 0xff,
	0xff, 0x50, 0xfc, 0x54, 0xe6, 0xde, 0x00, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// ControlServiceClient is the client API for ControlService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type ControlServiceClient interface {
	// 关闭节点
	StopNode(ctx context.Context, in *Request, opts ...grpc.CallOption) (*Response, error)
}

type controlServiceClient struct {
	cc *grpc.ClientConn
}

func NewControlServiceClient(cc *grpc.ClientConn) ControlServiceClient {
	return &controlServiceClient{cc}
}

func (c *controlServiceClient) StopNode(ctx context.Context, in *Request, opts ...grpc.CallOption) (*Response, error) {
	out := new(Response)
	err := c.cc.Invoke(ctx, "/control.ControlService/StopNode", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ControlServiceServer is the server API for ControlService service.
type ControlServiceServer interface {
	// 关闭节点
	StopNode(context.Context, *Request) (*Response, error)
}

// UnimplementedControlServiceServer can be embedded to have forward compatible implementations.
type UnimplementedControlServiceServer struct {
}

func (*UnimplementedControlServiceServer) StopNode(ctx context.Context, req *Request) (*Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopNode not implemented")
}

func RegisterControlServiceServer(s *grpc.Server, srv ControlServiceServer) {
	s.RegisterService(&_ControlService_serviceDesc, srv)
}

func _ControlService_StopNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServiceServer).StopNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/control.ControlService/StopNode",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServiceServer).StopNode(ctx, req.(*Request))
	}
	return interceptor(ctx, in, info, handler)
}

var _ControlService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "control.ControlService",
	HandlerType: (*ControlServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "StopNode",
			Handler:    _ControlService_StopNode_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "methods.proto",
}
