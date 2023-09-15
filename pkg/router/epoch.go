package router

import (
	"context"
	"math"
	"sync"
	"time"

	shardnodepb "github.com/dsg-uwaterloo/oblishard/api/shardnode"
	"github.com/dsg-uwaterloo/oblishard/pkg/rpc"
	utils "github.com/dsg-uwaterloo/oblishard/pkg/utils"
	"google.golang.org/grpc"
)

type epochManager struct {
	shardNodeRPCClients map[int]ReplicaRPCClientMap
	requests            map[int][]*request            // map of epoch round to requests
	reponseChans        map[int]map[*request]chan any // map of epoch round to map of request to response channel
	currentEpoch        int
	epochDuration       time.Duration
	hasher              utils.Hasher
	mu                  sync.Mutex
}

func newEpochManager(shardNodeRPCClients map[int]ReplicaRPCClientMap, epochDuration time.Duration) *epochManager {
	return &epochManager{
		shardNodeRPCClients: shardNodeRPCClients,
		requests:            make(map[int][]*request),
		reponseChans:        make(map[int]map[*request]chan any),
		currentEpoch:        0,
		epochDuration:       epochDuration,
		hasher:              utils.Hasher{KnownHashes: make(map[string]uint32)},
	}
}

const (
	Read int = iota
	Write
)

type request struct {
	ctx           context.Context
	operationType int
	block         string
	value         string
}

func (e *epochManager) addRequestToCurrentEpoch(r *request) chan any {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.requests[e.currentEpoch] = append(e.requests[e.currentEpoch], r)
	if _, exists := e.reponseChans[e.currentEpoch]; !exists {
		e.reponseChans[e.currentEpoch] = make(map[*request]chan any)
	}
	e.reponseChans[e.currentEpoch][r] = make(chan any)
	return e.reponseChans[e.currentEpoch][r]
}

func (e *epochManager) whereToForward(block string) (shardNodeID int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.hasher.Hash(block)
	return int(math.Mod(float64(h), float64(len(e.shardNodeRPCClients))))
}

type readResponse struct {
	value string
	err   error
}

type writeResponse struct {
	success bool
	err     error
}

type requestResponse struct {
	req      *request
	response any
}

func (e *epochManager) sendReadRequest(req *request, responseChannel chan requestResponse) {
	whereToForward := e.whereToForward(req.block)
	shardNodeRPCClient := e.shardNodeRPCClients[whereToForward]

	var replicaFuncs []rpc.CallFunc
	var clients []any
	for _, c := range shardNodeRPCClient {
		replicaFuncs = append(replicaFuncs,
			func(ctx context.Context, client any, request any, opts ...grpc.CallOption) (any, error) {
				return client.(ShardNodeRPCClient).ClientAPI.Read(ctx, request.(*shardnodepb.ReadRequest), opts...)
			},
		)
		clients = append(clients, c)
	}
	reply, err := rpc.CallAllReplicas(req.ctx, clients, replicaFuncs, &shardnodepb.ReadRequest{Block: req.block})
	if err != nil {
		responseChannel <- requestResponse{req: req, response: readResponse{err: err}}
	} else {
		shardNodeReply := reply.(*shardnodepb.ReadReply)
		responseChannel <- requestResponse{req: req, response: readResponse{value: shardNodeReply.Value, err: err}}
	}
}

func (e *epochManager) sendWriteRequest(req *request, responseChannel chan requestResponse) {
	whereToForward := e.whereToForward(req.block)
	shardNodeRPCClient := e.shardNodeRPCClients[whereToForward]

	var replicaFuncs []rpc.CallFunc
	var clients []any
	for _, c := range shardNodeRPCClient {
		replicaFuncs = append(replicaFuncs,
			func(ctx context.Context, client any, request any, opts ...grpc.CallOption) (any, error) {
				return client.(ShardNodeRPCClient).ClientAPI.Write(ctx, request.(*shardnodepb.WriteRequest), opts...)
			},
		)
		clients = append(clients, c)
	}

	reply, err := rpc.CallAllReplicas(req.ctx, clients, replicaFuncs, &shardnodepb.WriteRequest{Block: req.block, Value: req.value})
	if err != nil {
		responseChannel <- requestResponse{req: req, response: writeResponse{err: err}}
	} else {
		shardNodeReply := reply.(*shardnodepb.WriteReply)
		responseChannel <- requestResponse{req: req, response: writeResponse{success: shardNodeReply.Success, err: err}}
	}
}

// This function waits for all the responses then answers all of the requests.
// It can time out since a request may have failed.
func (e *epochManager) sendEpochRequestsAndAnswerThem(epochNumber int) {
	e.mu.Lock()
	responseChannel := make(chan requestResponse)
	requestsCount := len(e.requests[epochNumber])
	for _, r := range e.requests[epochNumber] {
		if r.operationType == Read {
			go e.sendReadRequest(r, responseChannel)
		} else if r.operationType == Write {
			go e.sendWriteRequest(r, responseChannel)
		}
	}
	e.mu.Unlock()
	timeout := time.After(5 * time.Second) // TODO: make this a parameter
	responsesReceived := make(map[*request]any)

	for {
		if len(responsesReceived) == requestsCount {
			break
		}
		select {
		case <-timeout:
			return
		case requestResponse := <-responseChannel:
			responsesReceived[requestResponse.req] = requestResponse.response
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for req, response := range responsesReceived {
		e.reponseChans[epochNumber][req] <- response
	}
}

// This function runs the epochManger forever.
func (e *epochManager) run() {
	for {
		epochTimeOut := time.After(e.epochDuration)
		<-epochTimeOut
		e.mu.Lock()
		go e.sendEpochRequestsAndAnswerThem(e.currentEpoch)
		e.currentEpoch++
		e.mu.Unlock()
	}
}