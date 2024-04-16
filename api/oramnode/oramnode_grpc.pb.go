// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.3
// source: oramnode.proto

package oramnode

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

const (
	OramNode_ReadPath_FullMethodName      = "/oramnode.OramNode/ReadPath"
	OramNode_JoinRaftVoter_FullMethodName = "/oramnode.OramNode/JoinRaftVoter"
)

// OramNodeClient is the client API for OramNode service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OramNodeClient interface {
	ReadPath(ctx context.Context, in *ReadPathRequest, opts ...grpc.CallOption) (*ReadPathReply, error)
	JoinRaftVoter(ctx context.Context, in *JoinRaftVoterRequest, opts ...grpc.CallOption) (*JoinRaftVoterReply, error)
}

type oramNodeClient struct {
	cc grpc.ClientConnInterface
}

func NewOramNodeClient(cc grpc.ClientConnInterface) OramNodeClient {
	return &oramNodeClient{cc}
}

func (c *oramNodeClient) ReadPath(ctx context.Context, in *ReadPathRequest, opts ...grpc.CallOption) (*ReadPathReply, error) {
	out := new(ReadPathReply)
	err := c.cc.Invoke(ctx, OramNode_ReadPath_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *oramNodeClient) JoinRaftVoter(ctx context.Context, in *JoinRaftVoterRequest, opts ...grpc.CallOption) (*JoinRaftVoterReply, error) {
	out := new(JoinRaftVoterReply)
	err := c.cc.Invoke(ctx, OramNode_JoinRaftVoter_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OramNodeServer is the server API for OramNode service.
// All implementations must embed UnimplementedOramNodeServer
// for forward compatibility
type OramNodeServer interface {
	ReadPath(context.Context, *ReadPathRequest) (*ReadPathReply, error)
	JoinRaftVoter(context.Context, *JoinRaftVoterRequest) (*JoinRaftVoterReply, error)
	mustEmbedUnimplementedOramNodeServer()
}

// UnimplementedOramNodeServer must be embedded to have forward compatible implementations.
type UnimplementedOramNodeServer struct {
}

func (UnimplementedOramNodeServer) ReadPath(context.Context, *ReadPathRequest) (*ReadPathReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadPath not implemented")
}
func (UnimplementedOramNodeServer) JoinRaftVoter(context.Context, *JoinRaftVoterRequest) (*JoinRaftVoterReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method JoinRaftVoter not implemented")
}
func (UnimplementedOramNodeServer) mustEmbedUnimplementedOramNodeServer() {}

// UnsafeOramNodeServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OramNodeServer will
// result in compilation errors.
type UnsafeOramNodeServer interface {
	mustEmbedUnimplementedOramNodeServer()
}

func RegisterOramNodeServer(s grpc.ServiceRegistrar, srv OramNodeServer) {
	s.RegisterService(&OramNode_ServiceDesc, srv)
}

func _OramNode_ReadPath_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReadPathRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OramNodeServer).ReadPath(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: OramNode_ReadPath_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OramNodeServer).ReadPath(ctx, req.(*ReadPathRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _OramNode_JoinRaftVoter_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JoinRaftVoterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OramNodeServer).JoinRaftVoter(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: OramNode_JoinRaftVoter_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OramNodeServer).JoinRaftVoter(ctx, req.(*JoinRaftVoterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// OramNode_ServiceDesc is the grpc.ServiceDesc for OramNode service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var OramNode_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "oramnode.OramNode",
	HandlerType: (*OramNodeServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ReadPath",
			Handler:    _OramNode_ReadPath_Handler,
		},
		{
			MethodName: "JoinRaftVoter",
			Handler:    _OramNode_JoinRaftVoter_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "oramnode.proto",
}
