package rpc

import (
	"context"
	"fmt"
	"time"

	grpc "google.golang.org/grpc"
)

type result struct {
	reply interface{}
	err   error
}

type CallFunc func(ctx context.Context, client interface{}, request interface{}, opts ...grpc.CallOption) (interface{}, error)

// TODO: add timeout to all operations
// TODO: move previous tests for calling all replicas to this package
func CallAllReplicas(ctx context.Context, clients []interface{}, replicaFuncs []CallFunc, request interface{}) (reply interface{}, err error) {
	responseChannel := make(chan result)
	for i, clientFunc := range replicaFuncs {
		go func(f CallFunc, client interface{}) {
			reply, err := f(ctx, client, request)
			responseChannel <- result{reply: reply, err: err}
		}(clientFunc, clients[i])
	}
	timeout := time.After(2 * time.Second)
	for {
		select {
		case result := <-responseChannel:
			if result.err == nil {
				return result.reply, nil
			}
		case <-timeout:
			return nil, fmt.Errorf("could not read blocks from the shardnode")
		}
	}
}
