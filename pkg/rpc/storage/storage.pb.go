// Code generated by protoc-gen-go. DO NOT EDIT.
// source: storage.proto

package storage

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type GetFileRequest struct {
	Key                  string   `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetFileRequest) Reset()         { *m = GetFileRequest{} }
func (m *GetFileRequest) String() string { return proto.CompactTextString(m) }
func (*GetFileRequest) ProtoMessage()    {}
func (*GetFileRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_c2f0ab088043e9e6, []int{0}
}
func (m *GetFileRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetFileRequest.Unmarshal(m, b)
}
func (m *GetFileRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetFileRequest.Marshal(b, m, deterministic)
}
func (dst *GetFileRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetFileRequest.Merge(dst, src)
}
func (m *GetFileRequest) XXX_Size() int {
	return xxx_messageInfo_GetFileRequest.Size(m)
}
func (m *GetFileRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GetFileRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GetFileRequest proto.InternalMessageInfo

func (m *GetFileRequest) GetKey() string {
	if m != nil {
		return m.Key
	}
	return ""
}

type GetFileResponse struct {
	FileChunk            []byte   `protobuf:"bytes,1,opt,name=file_chunk,json=fileChunk,proto3" json:"file_chunk,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetFileResponse) Reset()         { *m = GetFileResponse{} }
func (m *GetFileResponse) String() string { return proto.CompactTextString(m) }
func (*GetFileResponse) ProtoMessage()    {}
func (*GetFileResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_c2f0ab088043e9e6, []int{1}
}
func (m *GetFileResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetFileResponse.Unmarshal(m, b)
}
func (m *GetFileResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetFileResponse.Marshal(b, m, deterministic)
}
func (dst *GetFileResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetFileResponse.Merge(dst, src)
}
func (m *GetFileResponse) XXX_Size() int {
	return xxx_messageInfo_GetFileResponse.Size(m)
}
func (m *GetFileResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GetFileResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GetFileResponse proto.InternalMessageInfo

func (m *GetFileResponse) GetFileChunk() []byte {
	if m != nil {
		return m.FileChunk
	}
	return nil
}

type PutFileRequest struct {
	Key                  string   `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	FileChunk            []byte   `protobuf:"bytes,2,opt,name=file_chunk,json=fileChunk,proto3" json:"file_chunk,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PutFileRequest) Reset()         { *m = PutFileRequest{} }
func (m *PutFileRequest) String() string { return proto.CompactTextString(m) }
func (*PutFileRequest) ProtoMessage()    {}
func (*PutFileRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_c2f0ab088043e9e6, []int{2}
}
func (m *PutFileRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PutFileRequest.Unmarshal(m, b)
}
func (m *PutFileRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PutFileRequest.Marshal(b, m, deterministic)
}
func (dst *PutFileRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PutFileRequest.Merge(dst, src)
}
func (m *PutFileRequest) XXX_Size() int {
	return xxx_messageInfo_PutFileRequest.Size(m)
}
func (m *PutFileRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PutFileRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PutFileRequest proto.InternalMessageInfo

func (m *PutFileRequest) GetKey() string {
	if m != nil {
		return m.Key
	}
	return ""
}

func (m *PutFileRequest) GetFileChunk() []byte {
	if m != nil {
		return m.FileChunk
	}
	return nil
}

type PutFileResponse struct {
	Successful           bool     `protobuf:"varint,1,opt,name=successful,proto3" json:"successful,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PutFileResponse) Reset()         { *m = PutFileResponse{} }
func (m *PutFileResponse) String() string { return proto.CompactTextString(m) }
func (*PutFileResponse) ProtoMessage()    {}
func (*PutFileResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_c2f0ab088043e9e6, []int{3}
}
func (m *PutFileResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PutFileResponse.Unmarshal(m, b)
}
func (m *PutFileResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PutFileResponse.Marshal(b, m, deterministic)
}
func (dst *PutFileResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PutFileResponse.Merge(dst, src)
}
func (m *PutFileResponse) XXX_Size() int {
	return xxx_messageInfo_PutFileResponse.Size(m)
}
func (m *PutFileResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_PutFileResponse.DiscardUnknown(m)
}

var xxx_messageInfo_PutFileResponse proto.InternalMessageInfo

func (m *PutFileResponse) GetSuccessful() bool {
	if m != nil {
		return m.Successful
	}
	return false
}

type DeleteFileRequest struct {
	Key                  string   `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DeleteFileRequest) Reset()         { *m = DeleteFileRequest{} }
func (m *DeleteFileRequest) String() string { return proto.CompactTextString(m) }
func (*DeleteFileRequest) ProtoMessage()    {}
func (*DeleteFileRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_c2f0ab088043e9e6, []int{4}
}
func (m *DeleteFileRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DeleteFileRequest.Unmarshal(m, b)
}
func (m *DeleteFileRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DeleteFileRequest.Marshal(b, m, deterministic)
}
func (dst *DeleteFileRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DeleteFileRequest.Merge(dst, src)
}
func (m *DeleteFileRequest) XXX_Size() int {
	return xxx_messageInfo_DeleteFileRequest.Size(m)
}
func (m *DeleteFileRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_DeleteFileRequest.DiscardUnknown(m)
}

var xxx_messageInfo_DeleteFileRequest proto.InternalMessageInfo

func (m *DeleteFileRequest) GetKey() string {
	if m != nil {
		return m.Key
	}
	return ""
}

type DeleteFileResponse struct {
	Successful           string   `protobuf:"bytes,1,opt,name=successful,proto3" json:"successful,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DeleteFileResponse) Reset()         { *m = DeleteFileResponse{} }
func (m *DeleteFileResponse) String() string { return proto.CompactTextString(m) }
func (*DeleteFileResponse) ProtoMessage()    {}
func (*DeleteFileResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_c2f0ab088043e9e6, []int{5}
}
func (m *DeleteFileResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DeleteFileResponse.Unmarshal(m, b)
}
func (m *DeleteFileResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DeleteFileResponse.Marshal(b, m, deterministic)
}
func (dst *DeleteFileResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DeleteFileResponse.Merge(dst, src)
}
func (m *DeleteFileResponse) XXX_Size() int {
	return xxx_messageInfo_DeleteFileResponse.Size(m)
}
func (m *DeleteFileResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_DeleteFileResponse.DiscardUnknown(m)
}

var xxx_messageInfo_DeleteFileResponse proto.InternalMessageInfo

func (m *DeleteFileResponse) GetSuccessful() string {
	if m != nil {
		return m.Successful
	}
	return ""
}

func init() {
	proto.RegisterType((*GetFileRequest)(nil), "storage.GetFileRequest")
	proto.RegisterType((*GetFileResponse)(nil), "storage.GetFileResponse")
	proto.RegisterType((*PutFileRequest)(nil), "storage.PutFileRequest")
	proto.RegisterType((*PutFileResponse)(nil), "storage.PutFileResponse")
	proto.RegisterType((*DeleteFileRequest)(nil), "storage.DeleteFileRequest")
	proto.RegisterType((*DeleteFileResponse)(nil), "storage.DeleteFileResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// StorageServiceClient is the client API for StorageService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type StorageServiceClient interface {
	GetFile(ctx context.Context, in *GetFileRequest, opts ...grpc.CallOption) (StorageService_GetFileClient, error)
	PutFile(ctx context.Context, opts ...grpc.CallOption) (StorageService_PutFileClient, error)
	DeleteFile(ctx context.Context, in *DeleteFileRequest, opts ...grpc.CallOption) (*DeleteFileResponse, error)
}

type storageServiceClient struct {
	cc *grpc.ClientConn
}

func NewStorageServiceClient(cc *grpc.ClientConn) StorageServiceClient {
	return &storageServiceClient{cc}
}

func (c *storageServiceClient) GetFile(ctx context.Context, in *GetFileRequest, opts ...grpc.CallOption) (StorageService_GetFileClient, error) {
	stream, err := c.cc.NewStream(ctx, &_StorageService_serviceDesc.Streams[0], "/storage.StorageService/GetFile", opts...)
	if err != nil {
		return nil, err
	}
	x := &storageServiceGetFileClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type StorageService_GetFileClient interface {
	Recv() (*GetFileResponse, error)
	grpc.ClientStream
}

type storageServiceGetFileClient struct {
	grpc.ClientStream
}

func (x *storageServiceGetFileClient) Recv() (*GetFileResponse, error) {
	m := new(GetFileResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *storageServiceClient) PutFile(ctx context.Context, opts ...grpc.CallOption) (StorageService_PutFileClient, error) {
	stream, err := c.cc.NewStream(ctx, &_StorageService_serviceDesc.Streams[1], "/storage.StorageService/PutFile", opts...)
	if err != nil {
		return nil, err
	}
	x := &storageServicePutFileClient{stream}
	return x, nil
}

type StorageService_PutFileClient interface {
	Send(*PutFileRequest) error
	CloseAndRecv() (*PutFileResponse, error)
	grpc.ClientStream
}

type storageServicePutFileClient struct {
	grpc.ClientStream
}

func (x *storageServicePutFileClient) Send(m *PutFileRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *storageServicePutFileClient) CloseAndRecv() (*PutFileResponse, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(PutFileResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *storageServiceClient) DeleteFile(ctx context.Context, in *DeleteFileRequest, opts ...grpc.CallOption) (*DeleteFileResponse, error) {
	out := new(DeleteFileResponse)
	err := c.cc.Invoke(ctx, "/storage.StorageService/DeleteFile", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StorageServiceServer is the server API for StorageService service.
type StorageServiceServer interface {
	GetFile(*GetFileRequest, StorageService_GetFileServer) error
	PutFile(StorageService_PutFileServer) error
	DeleteFile(context.Context, *DeleteFileRequest) (*DeleteFileResponse, error)
}

func RegisterStorageServiceServer(s *grpc.Server, srv StorageServiceServer) {
	s.RegisterService(&_StorageService_serviceDesc, srv)
}

func _StorageService_GetFile_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetFileRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StorageServiceServer).GetFile(m, &storageServiceGetFileServer{stream})
}

type StorageService_GetFileServer interface {
	Send(*GetFileResponse) error
	grpc.ServerStream
}

type storageServiceGetFileServer struct {
	grpc.ServerStream
}

func (x *storageServiceGetFileServer) Send(m *GetFileResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _StorageService_PutFile_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(StorageServiceServer).PutFile(&storageServicePutFileServer{stream})
}

type StorageService_PutFileServer interface {
	SendAndClose(*PutFileResponse) error
	Recv() (*PutFileRequest, error)
	grpc.ServerStream
}

type storageServicePutFileServer struct {
	grpc.ServerStream
}

func (x *storageServicePutFileServer) SendAndClose(m *PutFileResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *storageServicePutFileServer) Recv() (*PutFileRequest, error) {
	m := new(PutFileRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _StorageService_DeleteFile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteFileRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StorageServiceServer).DeleteFile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/storage.StorageService/DeleteFile",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StorageServiceServer).DeleteFile(ctx, req.(*DeleteFileRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _StorageService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "storage.StorageService",
	HandlerType: (*StorageServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "DeleteFile",
			Handler:    _StorageService_DeleteFile_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetFile",
			Handler:       _StorageService_GetFile_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "PutFile",
			Handler:       _StorageService_PutFile_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "storage.proto",
}

func init() { proto.RegisterFile("storage.proto", fileDescriptor_storage_c2f0ab088043e9e6) }

var fileDescriptor_storage_c2f0ab088043e9e6 = []byte{
	// 254 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x2d, 0x2e, 0xc9, 0x2f,
	0x4a, 0x4c, 0x4f, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x87, 0x72, 0x95, 0x94, 0xb8,
	0xf8, 0xdc, 0x53, 0x4b, 0xdc, 0x32, 0x73, 0x52, 0x83, 0x52, 0x0b, 0x4b, 0x53, 0x8b, 0x4b, 0x84,
	0x04, 0xb8, 0x98, 0xb3, 0x53, 0x2b, 0x25, 0x18, 0x15, 0x18, 0x35, 0x38, 0x83, 0x40, 0x4c, 0x25,
	0x03, 0x2e, 0x7e, 0xb8, 0x9a, 0xe2, 0x82, 0xfc, 0xbc, 0xe2, 0x54, 0x21, 0x59, 0x2e, 0xae, 0xb4,
	0xcc, 0x9c, 0xd4, 0xf8, 0xe4, 0x8c, 0xd2, 0xbc, 0x6c, 0xb0, 0x5a, 0x9e, 0x20, 0x4e, 0x90, 0x88,
	0x33, 0x48, 0x40, 0xc9, 0x91, 0x8b, 0x2f, 0xa0, 0x14, 0xbf, 0xa9, 0x68, 0x46, 0x30, 0xa1, 0x1b,
	0x61, 0xc8, 0xc5, 0x0f, 0x37, 0x02, 0x6a, 0xa9, 0x1c, 0x17, 0x57, 0x71, 0x69, 0x72, 0x72, 0x6a,
	0x71, 0x71, 0x5a, 0x69, 0x0e, 0xd8, 0x28, 0x8e, 0x20, 0x24, 0x11, 0x25, 0x55, 0x2e, 0x41, 0x97,
	0xd4, 0x9c, 0xd4, 0x92, 0x54, 0xfc, 0xde, 0x31, 0xe1, 0x12, 0x42, 0x56, 0x86, 0xd3, 0x70, 0x4e,
	0x64, 0xc3, 0x8d, 0xae, 0x33, 0x72, 0xf1, 0x05, 0x43, 0x02, 0x2d, 0x38, 0xb5, 0xa8, 0x2c, 0x33,
	0x39, 0x55, 0xc8, 0x8e, 0x8b, 0x1d, 0x1a, 0x2e, 0x42, 0xe2, 0x7a, 0xb0, 0xf0, 0x45, 0x0d, 0x4d,
	0x29, 0x09, 0x4c, 0x09, 0x88, 0x85, 0x06, 0x8c, 0x20, 0xfd, 0x50, 0x2f, 0x22, 0xe9, 0x47, 0x0d,
	0x37, 0x24, 0xfd, 0x68, 0xa1, 0xa1, 0xc1, 0x28, 0xe4, 0xca, 0xc5, 0x85, 0xf0, 0x88, 0x90, 0x14,
	0x5c, 0x25, 0x46, 0x20, 0x48, 0x49, 0x63, 0x95, 0x83, 0x18, 0x94, 0xc4, 0x06, 0x4e, 0x12, 0xc6,
	0x80, 0x00, 0x00, 0x00, 0xff, 0xff, 0xaf, 0xd9, 0x14, 0x42, 0x23, 0x02, 0x00, 0x00,
}