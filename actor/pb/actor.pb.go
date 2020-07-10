// Code generated by protoc-gen-go. DO NOT EDIT.
// source: actor.proto

package pb

import (
	context "context"
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

type ActorDesc struct {
	Id                   int64    `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	System               string   `protobuf:"bytes,2,opt,name=system,proto3" json:"system,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ActorDesc) Reset()         { *m = ActorDesc{} }
func (m *ActorDesc) String() string { return proto.CompactTextString(m) }
func (*ActorDesc) ProtoMessage()    {}
func (*ActorDesc) Descriptor() ([]byte, []int) {
	return fileDescriptor_93a2698287ded216, []int{0}
}

func (m *ActorDesc) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ActorDesc.Unmarshal(m, b)
}
func (m *ActorDesc) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ActorDesc.Marshal(b, m, deterministic)
}
func (m *ActorDesc) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ActorDesc.Merge(m, src)
}
func (m *ActorDesc) XXX_Size() int {
	return xxx_messageInfo_ActorDesc.Size(m)
}
func (m *ActorDesc) XXX_DiscardUnknown() {
	xxx_messageInfo_ActorDesc.DiscardUnknown(m)
}

var xxx_messageInfo_ActorDesc proto.InternalMessageInfo

func (m *ActorDesc) GetId() int64 {
	if m != nil {
		return m.Id
	}
	return 0
}

func (m *ActorDesc) GetSystem() string {
	if m != nil {
		return m.System
	}
	return ""
}

// 重定向消息
type RedirectInfo struct {
	NodeKey              string   `protobuf:"bytes,1,opt,name=node_key,json=nodeKey,proto3" json:"node_key,omitempty"`
	NodeStatus           int32    `protobuf:"varint,2,opt,name=node_status,json=nodeStatus,proto3" json:"node_status,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RedirectInfo) Reset()         { *m = RedirectInfo{} }
func (m *RedirectInfo) String() string { return proto.CompactTextString(m) }
func (*RedirectInfo) ProtoMessage()    {}
func (*RedirectInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_93a2698287ded216, []int{1}
}

func (m *RedirectInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RedirectInfo.Unmarshal(m, b)
}
func (m *RedirectInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RedirectInfo.Marshal(b, m, deterministic)
}
func (m *RedirectInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RedirectInfo.Merge(m, src)
}
func (m *RedirectInfo) XXX_Size() int {
	return xxx_messageInfo_RedirectInfo.Size(m)
}
func (m *RedirectInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_RedirectInfo.DiscardUnknown(m)
}

var xxx_messageInfo_RedirectInfo proto.InternalMessageInfo

func (m *RedirectInfo) GetNodeKey() string {
	if m != nil {
		return m.NodeKey
	}
	return ""
}

func (m *RedirectInfo) GetNodeStatus() int32 {
	if m != nil {
		return m.NodeStatus
	}
	return 0
}

type WrapMsg struct {
	MsgType              string   `protobuf:"bytes,1,opt,name=msg_type,json=msgType,proto3" json:"msg_type,omitempty"`
	MsgData              []byte   `protobuf:"bytes,2,opt,name=msg_data,json=msgData,proto3" json:"msg_data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *WrapMsg) Reset()         { *m = WrapMsg{} }
func (m *WrapMsg) String() string { return proto.CompactTextString(m) }
func (*WrapMsg) ProtoMessage()    {}
func (*WrapMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_93a2698287ded216, []int{2}
}

func (m *WrapMsg) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_WrapMsg.Unmarshal(m, b)
}
func (m *WrapMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_WrapMsg.Marshal(b, m, deterministic)
}
func (m *WrapMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_WrapMsg.Merge(m, src)
}
func (m *WrapMsg) XXX_Size() int {
	return xxx_messageInfo_WrapMsg.Size(m)
}
func (m *WrapMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_WrapMsg.DiscardUnknown(m)
}

var xxx_messageInfo_WrapMsg proto.InternalMessageInfo

func (m *WrapMsg) GetMsgType() string {
	if m != nil {
		return m.MsgType
	}
	return ""
}

func (m *WrapMsg) GetMsgData() []byte {
	if m != nil {
		return m.MsgData
	}
	return nil
}

// 请求消息
type RequestMsg struct {
	Sender               *ActorDesc    `protobuf:"bytes,1,opt,name=sender,proto3" json:"sender,omitempty"`
	Target               *ActorDesc    `protobuf:"bytes,2,opt,name=target,proto3" json:"target,omitempty"`
	ReqId                int64         `protobuf:"varint,3,opt,name=req_id,json=reqId,proto3" json:"req_id,omitempty"`
	Redirect             *RedirectInfo `protobuf:"bytes,4,opt,name=redirect,proto3" json:"redirect,omitempty"`
	IsRespond            bool          `protobuf:"varint,5,opt,name=is_respond,json=isRespond,proto3" json:"is_respond,omitempty"`
	Data                 *WrapMsg      `protobuf:"bytes,6,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *RequestMsg) Reset()         { *m = RequestMsg{} }
func (m *RequestMsg) String() string { return proto.CompactTextString(m) }
func (*RequestMsg) ProtoMessage()    {}
func (*RequestMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_93a2698287ded216, []int{3}
}

func (m *RequestMsg) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RequestMsg.Unmarshal(m, b)
}
func (m *RequestMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RequestMsg.Marshal(b, m, deterministic)
}
func (m *RequestMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RequestMsg.Merge(m, src)
}
func (m *RequestMsg) XXX_Size() int {
	return xxx_messageInfo_RequestMsg.Size(m)
}
func (m *RequestMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_RequestMsg.DiscardUnknown(m)
}

var xxx_messageInfo_RequestMsg proto.InternalMessageInfo

func (m *RequestMsg) GetSender() *ActorDesc {
	if m != nil {
		return m.Sender
	}
	return nil
}

func (m *RequestMsg) GetTarget() *ActorDesc {
	if m != nil {
		return m.Target
	}
	return nil
}

func (m *RequestMsg) GetReqId() int64 {
	if m != nil {
		return m.ReqId
	}
	return 0
}

func (m *RequestMsg) GetRedirect() *RedirectInfo {
	if m != nil {
		return m.Redirect
	}
	return nil
}

func (m *RequestMsg) GetIsRespond() bool {
	if m != nil {
		return m.IsRespond
	}
	return false
}

func (m *RequestMsg) GetData() *WrapMsg {
	if m != nil {
		return m.Data
	}
	return nil
}

// 响应消息
type ResponseMsg struct {
	Resper               *ActorDesc `protobuf:"bytes,1,opt,name=resper,proto3" json:"resper,omitempty"`
	ReqId                int64      `protobuf:"varint,2,opt,name=req_id,json=reqId,proto3" json:"req_id,omitempty"`
	Data                 *WrapMsg   `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *ResponseMsg) Reset()         { *m = ResponseMsg{} }
func (m *ResponseMsg) String() string { return proto.CompactTextString(m) }
func (*ResponseMsg) ProtoMessage()    {}
func (*ResponseMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_93a2698287ded216, []int{4}
}

func (m *ResponseMsg) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ResponseMsg.Unmarshal(m, b)
}
func (m *ResponseMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ResponseMsg.Marshal(b, m, deterministic)
}
func (m *ResponseMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ResponseMsg.Merge(m, src)
}
func (m *ResponseMsg) XXX_Size() int {
	return xxx_messageInfo_ResponseMsg.Size(m)
}
func (m *ResponseMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_ResponseMsg.DiscardUnknown(m)
}

var xxx_messageInfo_ResponseMsg proto.InternalMessageInfo

func (m *ResponseMsg) GetResper() *ActorDesc {
	if m != nil {
		return m.Resper
	}
	return nil
}

func (m *ResponseMsg) GetReqId() int64 {
	if m != nil {
		return m.ReqId
	}
	return 0
}

func (m *ResponseMsg) GetData() *WrapMsg {
	if m != nil {
		return m.Data
	}
	return nil
}

func init() {
	proto.RegisterType((*ActorDesc)(nil), "pb.ActorDesc")
	proto.RegisterType((*RedirectInfo)(nil), "pb.RedirectInfo")
	proto.RegisterType((*WrapMsg)(nil), "pb.WrapMsg")
	proto.RegisterType((*RequestMsg)(nil), "pb.RequestMsg")
	proto.RegisterType((*ResponseMsg)(nil), "pb.ResponseMsg")
}

func init() { proto.RegisterFile("actor.proto", fileDescriptor_93a2698287ded216) }

var fileDescriptor_93a2698287ded216 = []byte{
	// 364 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x92, 0x5f, 0x6b, 0xea, 0x40,
	0x10, 0xc5, 0x49, 0xd4, 0x68, 0x26, 0xea, 0xbd, 0x2c, 0xdc, 0x4b, 0xee, 0x85, 0xa2, 0x04, 0x0a,
	0x52, 0x8a, 0x0f, 0xfa, 0xd8, 0x87, 0x52, 0xf0, 0xc5, 0x96, 0xbe, 0xac, 0x85, 0x3e, 0x86, 0x35,
	0x3b, 0x0d, 0xa1, 0xcd, 0x1f, 0x77, 0xd7, 0x42, 0x3e, 0x6d, 0xbf, 0x4a, 0xd9, 0x49, 0x22, 0x42,
	0xf1, 0x71, 0xe6, 0xcc, 0xfc, 0x76, 0xce, 0x61, 0x21, 0x10, 0x89, 0x29, 0xd5, 0xb2, 0x52, 0xa5,
	0x29, 0x99, 0x5b, 0xed, 0xa3, 0x35, 0xf8, 0x0f, 0xb6, 0xb5, 0x41, 0x9d, 0xb0, 0x29, 0xb8, 0x99,
	0x0c, 0x9d, 0xb9, 0xb3, 0xe8, 0x71, 0x37, 0x93, 0xec, 0x2f, 0x78, 0xba, 0xd6, 0x06, 0xf3, 0xd0,
	0x9d, 0x3b, 0x0b, 0x9f, 0xb7, 0x55, 0xf4, 0x08, 0x63, 0x8e, 0x32, 0x53, 0x98, 0x98, 0x6d, 0xf1,
	0x56, 0xb2, 0x7f, 0x30, 0x2a, 0x4a, 0x89, 0xf1, 0x3b, 0xd6, 0xb4, 0xed, 0xf3, 0xa1, 0xad, 0x9f,
	0xb0, 0x66, 0x33, 0x08, 0x48, 0xd2, 0x46, 0x98, 0xa3, 0x26, 0xce, 0x80, 0x83, 0x6d, 0xed, 0xa8,
	0x13, 0xdd, 0xc3, 0xf0, 0x55, 0x89, 0xea, 0x59, 0xa7, 0x16, 0x93, 0xeb, 0x34, 0x36, 0x75, 0x85,
	0x1d, 0x26, 0xd7, 0xe9, 0x4b, 0x5d, 0x61, 0x27, 0x49, 0x61, 0x04, 0x31, 0xc6, 0x24, 0x6d, 0x84,
	0x11, 0xd1, 0x97, 0x03, 0xc0, 0xf1, 0x70, 0x44, 0x6d, 0x2c, 0xe4, 0x1a, 0x3c, 0x8d, 0x85, 0x44,
	0x45, 0x88, 0x60, 0x35, 0x59, 0x56, 0xfb, 0xe5, 0xc9, 0x22, 0x6f, 0x45, 0x3b, 0x66, 0x84, 0x4a,
	0xd1, 0x10, 0xee, 0xe7, 0x58, 0x23, 0xb2, 0x3f, 0xe0, 0x29, 0x3c, 0xc4, 0x99, 0x0c, 0x7b, 0x94,
	0xca, 0x40, 0xe1, 0x61, 0x2b, 0xd9, 0x2d, 0x8c, 0x54, 0x1b, 0x40, 0xd8, 0xa7, 0xfd, 0xdf, 0x76,
	0xff, 0x3c, 0x14, 0x7e, 0x9a, 0x60, 0x57, 0x00, 0x99, 0x8e, 0x15, 0xea, 0xaa, 0x2c, 0x64, 0x38,
	0x98, 0x3b, 0x8b, 0x11, 0xf7, 0x33, 0xcd, 0x9b, 0x06, 0x9b, 0x41, 0x9f, 0x7c, 0x79, 0x04, 0x0a,
	0x2c, 0xa8, 0x4d, 0x84, 0x93, 0x10, 0x7d, 0x40, 0xd0, 0xcc, 0x6a, 0x6c, 0x1d, 0x5a, 0xd6, 0x45,
	0x87, 0x8d, 0x78, 0x76, 0xba, 0x7b, 0x7e, 0x7a, 0xf7, 0x5a, 0xef, 0xc2, 0x6b, 0xab, 0x3b, 0x98,
	0x70, 0xcc, 0x4b, 0x83, 0x3b, 0x54, 0x9f, 0x59, 0x82, 0xec, 0x06, 0x86, 0x6d, 0xbe, 0x6c, 0xda,
	0xb8, 0xec, 0xc2, 0xfe, 0xff, 0xab, 0xa9, 0x4f, 0xb7, 0xed, 0x3d, 0xfa, 0x59, 0xeb, 0xef, 0x00,
	0x00, 0x00, 0xff, 0xff, 0x1d, 0x3c, 0x4e, 0xa1, 0x68, 0x02, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// RemoteServiceClient is the client API for RemoteService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RemoteServiceClient interface {
	Request(ctx context.Context, in *RequestMsg, opts ...grpc.CallOption) (*ResponseMsg, error)
}

type remoteServiceClient struct {
	cc *grpc.ClientConn
}

func NewRemoteServiceClient(cc *grpc.ClientConn) RemoteServiceClient {
	return &remoteServiceClient{cc}
}

func (c *remoteServiceClient) Request(ctx context.Context, in *RequestMsg, opts ...grpc.CallOption) (*ResponseMsg, error) {
	out := new(ResponseMsg)
	err := c.cc.Invoke(ctx, "/pb.RemoteService/Request", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RemoteServiceServer is the server API for RemoteService service.
type RemoteServiceServer interface {
	Request(context.Context, *RequestMsg) (*ResponseMsg, error)
}

// UnimplementedRemoteServiceServer can be embedded to have forward compatible implementations.
type UnimplementedRemoteServiceServer struct {
}

func (*UnimplementedRemoteServiceServer) Request(ctx context.Context, req *RequestMsg) (*ResponseMsg, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Request not implemented")
}

func RegisterRemoteServiceServer(s *grpc.Server, srv RemoteServiceServer) {
	s.RegisterService(&_RemoteService_serviceDesc, srv)
}

func _RemoteService_Request_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RequestMsg)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RemoteServiceServer).Request(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.RemoteService/Request",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RemoteServiceServer).Request(ctx, req.(*RequestMsg))
	}
	return interceptor(ctx, in, info, handler)
}

var _RemoteService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "pb.RemoteService",
	HandlerType: (*RemoteServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Request",
			Handler:    _RemoteService_Request_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "actor.proto",
}