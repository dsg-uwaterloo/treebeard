// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.12
// source: shardnode.proto

package shardnode

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ShardNodeClient is the client API for ShardNode service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ShardNodeClient interface {
	Read(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (*ReadReply, error)
	Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*WriteReply, error)
}

type shardNodeClient struct {
	cc grpc.ClientConnInterface
}

func NewShardNodeClient(cc grpc.ClientConnInterface) ShardNodeClient {
	return &shardNodeClient{cc}
}

func (c *shardNodeClient) Read(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (*ReadReply, error) {
	out := new(ReadReply)
	err := c.cc.Invoke(ctx, "/shardnode.ShardNode/Read", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *shardNodeClient) Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*WriteReply, error) {
	out := new(WriteReply)
	err := c.cc.Invoke(ctx, "/shardnode.ShardNode/Write", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ShardNodeServer is the server API for ShardNode service.
// All implementations must embed UnimplementedShardNodeServer
// for forward compatibility
type ShardNodeServer interface {
	Read(context.Context, *ReadRequest) (*ReadReply, error)
	Write(context.Context, *WriteRequest) (*WriteReply, error)
	mustEmbedUnimplementedShardNodeServer()
}

// UnimplementedShardNodeServer must be embedded to have forward compatible implementations.
type UnimplementedShardNodeServer struct {
}

func (UnimplementedShardNodeServer) Read(context.Context, *ReadRequest) (*ReadReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Read not implemented")
}
func (UnimplementedShardNodeServer) Write(context.Context, *WriteRequest) (*WriteReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Write not implemented")
}
func (UnimplementedShardNodeServer) mustEmbedUnimplementedShardNodeServer() {}

// UnsafeShardNodeServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ShardNodeServer will
// result in compilation errors.
type UnsafeShardNodeServer interface {
	mustEmbedUnimplementedShardNodeServer()
}

func RegisterShardNodeServer(s grpc.ServiceRegistrar, srv ShardNodeServer) {
	s.RegisterService(&ShardNode_ServiceDesc, srv)
}

func _ShardNode_Read_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReadRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ShardNodeServer).Read(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/shardnode.ShardNode/Read",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ShardNodeServer).Read(ctx, req.(*ReadRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ShardNode_Write_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(WriteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ShardNodeServer).Write(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/shardnode.ShardNode/Write",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ShardNodeServer).Write(ctx, req.(*WriteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ShardNode_ServiceDesc is the grpc.ServiceDesc for ShardNode service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ShardNode_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "shardnode.ShardNode",
	HandlerType: (*ShardNodeServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Read",
			Handler:    _ShardNode_Read_Handler,
		},
		{
			MethodName: "Write",
			Handler:    _ShardNode_Write_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "shardnode.proto",
}
