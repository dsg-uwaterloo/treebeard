package shardnode

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"

	pb "github.com/dsg-uwaterloo/treebeard/api/shardnode"
	"github.com/dsg-uwaterloo/treebeard/pkg/commonerrs"
	"github.com/dsg-uwaterloo/treebeard/pkg/config"
	"github.com/dsg-uwaterloo/treebeard/pkg/rpc"
	"github.com/dsg-uwaterloo/treebeard/pkg/storage"
	"github.com/hashicorp/raft"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type shardNodeServer struct {
	pb.UnimplementedShardNodeServer
	shardNodeServerID  int
	replicaID          int
	raftNode           *raft.Raft
	shardNodeFSM       *shardNodeFSM
	oramNodeClients    RPCClientMap
	storageORAMNodeMap map[int]int // map of storageID to responsible oramNodeID
	storageTreeHeight  int
	batchManager       *batchManager
}

func newShardNodeServer(shardNodeServerID int, replicaID int, raftNode *raft.Raft, fsm *shardNodeFSM, oramNodeRPCClients RPCClientMap, storageORAMNodeMap map[int]int, storageTreeHeight int, batchManager *batchManager) *shardNodeServer {
	return &shardNodeServer{
		shardNodeServerID:  shardNodeServerID,
		replicaID:          replicaID,
		raftNode:           raftNode,
		shardNodeFSM:       fsm,
		oramNodeClients:    oramNodeRPCClients,
		batchManager:       batchManager,
		storageORAMNodeMap: storageORAMNodeMap,
		storageTreeHeight:  storageTreeHeight,
	}
}

type OperationType int

const (
	Read = iota
	Write
)

// For the initial request of a block, it returns the path and storageID from the position map.
// For other requests, it returns a random path and storageID and a random block.
func (s *shardNodeServer) getWhatToSendBasedOnRequest(ctx context.Context, block string, requestID string, isFirst bool) (blockToRequest string, path int, storageID int) {
	log.Debug().Msgf("Getting path and storageID based on request for block %s and requestID %s", block, requestID)
	if !isFirst {
		path, storageID = storage.GetRandomPathAndStorageID(s.storageTreeHeight, len(s.storageORAMNodeMap))
		return block + strconv.Itoa(rand.Int()), path, storageID
	} else {
		s.shardNodeFSM.positionMapMu.RLock()
		defer s.shardNodeFSM.positionMapMu.RUnlock()
		if _, exists := s.shardNodeFSM.positionMap[block]; !exists {
			path, storageID = storage.GetRandomPathAndStorageID(s.storageTreeHeight, len(s.storageORAMNodeMap))
			return block, path, storageID
		} else {
			return block, s.shardNodeFSM.positionMap[block].path, s.shardNodeFSM.positionMap[block].storageID
		}
	}
}

// It creates a channel for receiving the response from the raft FSM for the current requestID.
// The response channel should be buffered so that we don't block the raft FSM even if the client is not reading from the channel right now.
func (s *shardNodeServer) createResponseChannelForBatch(readRequests []*pb.ReadRequest, writeRequests []*pb.WriteRequest) map[string]chan string {
	channelMap := make(map[string]chan string)
	for _, req := range readRequests {
		channelMap[req.RequestId] = make(chan string, 1)
		s.shardNodeFSM.responseChannel.Store(req.RequestId, channelMap[req.RequestId])
	}
	for _, req := range writeRequests {
		channelMap[req.RequestId] = make(chan string, 1)
		s.shardNodeFSM.responseChannel.Store(req.RequestId, channelMap[req.RequestId])
	}
	return channelMap
}

// It periodically sends batches.
func (s *shardNodeServer) sendBatchesForever() {
	for {
		<-time.After(s.batchManager.batchTimeout)
		s.sendCurrentBatches()
	}
}

// For queues that have equal or more than batchSize requests, it sends a batch of size=BatchSize and waits for the responses.
// The logic here assumes that there are no duplicate blocks in the requests (which is fine since we only send a real request for the first one).
// It will not work otherwise because it will delete the response channel for a block after getting the first response.
func (s *shardNodeServer) sendCurrentBatches() {
	storageQueues := make(map[int][]blockRequest)
	responseChannels := make(map[string]chan string)
	// TODO: I have another idea instead of the high priority lock.
	// I can have a seperate go routine that has a for loop that manages the lock
	// It has two channels one for low priority and one for high priority
	// It will always start by trying to read from the high priority channel,
	// if it is empty, it will read from the low priority channel.
	s.batchManager.mu.HighPriorityLock()
	for storageID, requests := range s.batchManager.storageQueues {
		storageQueues[storageID] = append(storageQueues[storageID], requests...)
		delete(s.batchManager.storageQueues, storageID)
	}
	for block, responseChannel := range s.batchManager.responseChannel {
		responseChannels[block] = responseChannel
		delete(s.batchManager.responseChannel, block)
	}
	s.batchManager.mu.HighPriorityUnlock()

	batchRequestResponseChan := make(chan batchResponse)
	waitingBatchCount := 0
	for storageID, requests := range storageQueues {
		if len(requests) == 0 {
			continue
		}
		waitingBatchCount++
		oramNodeReplicaMap := s.oramNodeClients[s.storageORAMNodeMap[storageID]]
		go s.batchManager.asyncBatchRequests(context.Background(), storageID, requests, oramNodeReplicaMap, batchRequestResponseChan)
	}

	for i := 0; i < waitingBatchCount; i++ {
		response := <-batchRequestResponseChan
		if response.err != nil {
			log.Error().Msgf("Could not get value from the oramnode; %s", response.err)
			continue
		}
		log.Debug().Msgf("Got batch response from oram node replica: %v", response)
		go func(response batchResponse) {
			for _, readPathReply := range response.Responses {
				log.Debug().Msgf("Got reply from oram node replica: %v", readPathReply)
				if _, exists := responseChannels[readPathReply.Block]; exists {
					// timeout := time.After(1 * time.Second)
					select {
					// case <-timeout:
					// 	log.Error().Msgf("Timeout while waiting for batch response channel for block %s", readPathReply.Block)
					// 	continue
					case responseChannels[readPathReply.Block] <- readPathReply.Value:
					default:
					}
				} else {
					s.shardNodeFSM.stashMu.Lock()
					s.shardNodeFSM.stash[readPathReply.Block] = stashState{readPathReply.Value, 0, false}
					s.shardNodeFSM.stashMu.Unlock()
				}
			}
		}(response)
	}
}

func (s *shardNodeServer) getRequestReplicationBlocks(readRequests []*pb.ReadRequest, writeRequests []*pb.WriteRequest) (requestReplicationBlocks []ReplicateRequestAndPathAndStoragePayload) {
	for _, readRequest := range readRequests {
		newPath, newStorageID := storage.GetRandomPathAndStorageID(s.storageTreeHeight, len(s.storageORAMNodeMap))
		requestReplicationBlocks = append(requestReplicationBlocks, ReplicateRequestAndPathAndStoragePayload{
			RequestedBlock: readRequest.Block,
			RequestID:      readRequest.RequestId,
			Path:           newPath,
			StorageID:      newStorageID,
		})
	}
	for _, writeRequest := range writeRequests {
		newPath, newStorageID := storage.GetRandomPathAndStorageID(s.storageTreeHeight, len(s.storageORAMNodeMap))
		requestReplicationBlocks = append(requestReplicationBlocks, ReplicateRequestAndPathAndStoragePayload{
			RequestedBlock: writeRequest.Block,
			RequestID:      writeRequest.RequestId,
			Path:           newPath,
			StorageID:      newStorageID,
		})
	}
	return requestReplicationBlocks
}

type finalResponse struct {
	requestId string
	value     string
	opType    OperationType
	err       error
}

func (s *shardNodeServer) query(ctx context.Context, block string, requestID string, isFirst bool, newVal string, opType OperationType, raftResponseChannel chan string, finalResponseChannel chan finalResponse) {
	tracer := otel.Tracer("")

	blockToRequest, path, storageID := s.getWhatToSendBasedOnRequest(ctx, block, requestID, isFirst)
	var replyValue string
	_, waitOnReplySpan := tracer.Start(ctx, "wait on reply")
	log.Debug().Msgf("Adding request to storage queue and waiting for block %s", blockToRequest)
	oramReplyChan := s.batchManager.addRequestToStorageQueueAndWait(blockRequest{ctx: ctx, block: blockToRequest, path: path}, storageID)
	replyValue = <-oramReplyChan
	log.Debug().Msgf("Got reply from oram node channel for block %s; value: %s", blockToRequest, replyValue)
	waitOnReplySpan.End()

	if isFirst {
		log.Debug().Msgf("Adding response to response channel for block %s", blockToRequest)
		responseReplicationCommand, err := newResponseReplicationCommand(replyValue, requestID, block, newVal, opType, s.replicaID)
		if err != nil {
			finalResponseChannel <- finalResponse{requestId: requestID, value: "", opType: opType, err: fmt.Errorf("could not create response replication command; %s", err)}
			return
		}
		_, responseReplicationSpan := tracer.Start(ctx, "apply response replication")
		responseApplyFuture := s.raftNode.Apply(responseReplicationCommand, 0)
		err = responseApplyFuture.Error()
		responseReplicationSpan.End()
		if err != nil {
			finalResponseChannel <- finalResponse{requestId: requestID, value: "", opType: opType, err: fmt.Errorf("could not apply log to the FSM; %s", err)}
			return
		}
		response := responseApplyFuture.Response().(string)
		log.Debug().Msgf("Got is first response from response channel for block %s; value: %s", block, response)
		finalResponseChannel <- finalResponse{requestId: requestID, value: response, opType: opType, err: nil}
		return
	}
	responseValue := <-raftResponseChannel
	log.Debug().Msgf("Got response from response channel for block %s; value: %s", block, responseValue)
	finalResponseChannel <- finalResponse{requestId: requestID, value: responseValue, opType: opType, err: nil}
}

func (s *shardNodeServer) queryBatch(ctx context.Context, request *pb.RequestBatch) (reply *pb.ReplyBatch, err error) {
	if s.raftNode.State() != raft.Leader {
		return nil, fmt.Errorf(commonerrs.NotTheLeaderError)
	}
	tracer := otel.Tracer("")
	ctx, querySpan := tracer.Start(ctx, "shardnode query")

	responseChannel := s.createResponseChannelForBatch(request.ReadRequests, request.WriteRequests)
	requestReplicationBlocks := s.getRequestReplicationBlocks(request.ReadRequests, request.WriteRequests)
	requestReplicationCommand, err := newRequestReplicationCommand(requestReplicationBlocks, s.replicaID)
	if err != nil {
		return nil, fmt.Errorf("could not create request replication command; %s", err)
	}
	_, requestReplicationSpan := tracer.Start(ctx, "apply request replication")
	requestApplyFuture := s.raftNode.Apply(requestReplicationCommand, 0)
	err = requestApplyFuture.Error()
	requestReplicationSpan.End()
	if err != nil {
		return nil, fmt.Errorf("could not apply log to the FSM; %s", err)
	}
	isFirstMap := requestApplyFuture.Response().(map[string]bool)

	finalResponseChan := make(chan finalResponse)
	for _, readRequest := range request.ReadRequests {
		go s.query(ctx, readRequest.Block, readRequest.RequestId, isFirstMap[readRequest.RequestId], "", Read, responseChannel[readRequest.RequestId], finalResponseChan)
	}
	for _, writeRequest := range request.WriteRequests {
		go s.query(ctx, writeRequest.Block, writeRequest.RequestId, isFirstMap[writeRequest.RequestId], writeRequest.Value, Write, responseChannel[writeRequest.RequestId], finalResponseChan)
	}

	var readReplies []*pb.ReadReply
	var writeReplies []*pb.WriteReply
	for i := 0; i < len(request.ReadRequests)+len(request.WriteRequests); i++ {
		response := <-finalResponseChan
		if response.err != nil {
			return nil, fmt.Errorf("could not get response from the oramnode; %s", response.err)
		}
		if response.opType == Read {
			readReplies = append(readReplies, &pb.ReadReply{RequestId: response.requestId, Value: response.value})
		} else {
			writeReplies = append(writeReplies, &pb.WriteReply{RequestId: response.requestId, Success: true})
		}
	}
	querySpan.End()
	return &pb.ReplyBatch{ReadReplies: readReplies, WriteReplies: writeReplies}, nil
}

func (s *shardNodeServer) BatchQuery(ctx context.Context, request *pb.RequestBatch) (*pb.ReplyBatch, error) {
	log.Debug().Msgf("Received request batch request %v", request)
	reply, err := s.queryBatch(ctx, request)
	if err != nil {
		return nil, err
	}
	log.Debug().Msgf("Returning request batch reply %v", reply)
	return reply, nil
}

// It gets maxBlocks from the stash to send to the requesting oram node.
// The blocks should be for the same path and storageID.
func (s *shardNodeServer) getBlocksForSend(maxBlocks int, storageID int) (blocksToReturn []*pb.Block, blocks []string) {
	log.Debug().Msgf("Aquiring lock for shard node FSM in getBlocksForSend")
	s.shardNodeFSM.stashMu.Lock()
	s.shardNodeFSM.positionMapMu.RLock()
	log.Debug().Msgf("Aquired lock for shard node FSM in getBlocksForSend")
	defer func() {
		log.Debug().Msgf("Releasing lock for shard node FSM in getBlocksForSend")
		s.shardNodeFSM.stashMu.Unlock()
		s.shardNodeFSM.positionMapMu.RUnlock()
		log.Debug().Msgf("Released lock for shard node FSM in getBlocksForSend")
	}()

	counter := 0
	for block, stashState := range s.shardNodeFSM.stash {
		if counter == int(maxBlocks) {
			break
		}
		blocksToReturn = append(blocksToReturn, &pb.Block{Block: block, Value: stashState.value, Path: int32(s.shardNodeFSM.positionMap[block].path)})
		blocks = append(blocks, block)
		counter++

	}
	log.Debug().Msgf("Sending blocks %v for storageID %d", blocks, storageID)
	return blocksToReturn, blocks
}

// It sends blocks to the oram node for eviction.
func (s *shardNodeServer) SendBlocks(ctx context.Context, request *pb.SendBlocksRequest) (*pb.SendBlocksReply, error) {

	blocksToReturn, blocks := s.getBlocksForSend(int(request.MaxBlocks), int(request.StorageId))

	sentBlocksReplicationCommand, err := newSentBlocksReplicationCommand(blocks)
	if err != nil {
		return nil, fmt.Errorf("could not create sent blocks replication command; %s", err)
	}
	err = s.raftNode.Apply(sentBlocksReplicationCommand, 0).Error()
	if err != nil {
		return nil, fmt.Errorf("could not apply log to the FSM; %s", err)
	}

	return &pb.SendBlocksReply{Blocks: blocksToReturn}, nil
}

// It gets the acks and nacks from the oram node.
// Ackes and Nackes get replicated to be handled in the raft layer.
func (s *shardNodeServer) AckSentBlocks(ctx context.Context, reply *pb.AckSentBlocksRequest) (*pb.AckSentBlocksReply, error) {
	var ackedBlocks []string
	var nackedBlocks []string
	for _, ack := range reply.Acks {
		block := ack.Block
		is_ack := ack.IsAck
		if is_ack {
			ackedBlocks = append(ackedBlocks, block)
		} else {
			nackedBlocks = append(nackedBlocks, block)
		}
	}
	log.Debug().Msgf("Received acks %v and nacks %v", ackedBlocks, nackedBlocks)

	acksNacksReplicationCommand, err := newAcksNacksReplicationCommand(ackedBlocks, nackedBlocks)
	if err != nil {
		return nil, fmt.Errorf("could not create acks/nacks replication command")
	}
	err = s.raftNode.Apply(acksNacksReplicationCommand, 0).Error()
	if err != nil {
		return nil, fmt.Errorf("could not apply log to the FSM; %s", err)
	}
	return &pb.AckSentBlocksReply{Success: true}, nil
}

func (s *shardNodeServer) JoinRaftVoter(ctx context.Context, joinRaftVoterRequest *pb.JoinRaftVoterRequest) (*pb.JoinRaftVoterReply, error) {
	requestingNodeId := joinRaftVoterRequest.NodeId
	requestingNodeAddr := joinRaftVoterRequest.NodeAddr

	log.Printf("received join request from node %d at %s", requestingNodeId, requestingNodeAddr)

	err := s.raftNode.AddVoter(
		raft.ServerID(strconv.Itoa(int(requestingNodeId))),
		raft.ServerAddress(requestingNodeAddr),
		0, 0).Error()

	if err != nil {
		return &pb.JoinRaftVoterReply{Success: false}, fmt.Errorf("voter could not be added to the leader; %s", err)
	}
	return &pb.JoinRaftVoterReply{Success: true}, nil
}

func StartServer(shardNodeServerID int, bindIp string, advertiseIp string, rpcPort int, replicaID int, raftPort int, joinAddr string, oramNodeRPCClients map[int]ReplicaRPCClientMap, parameters config.Parameters, storages []config.RedisEndpoint, configsPath string) {
	isFirst := joinAddr == ""
	shardNodeFSM := newShardNodeFSM(replicaID)
	r, err := startRaftServer(isFirst, bindIp, advertiseIp, replicaID, raftPort, shardNodeFSM)
	if err != nil {
		log.Fatal().Msgf("The raft node creation did not succeed; %s", err)
	}

	if !isFirst {
		conn, err := grpc.Dial(joinAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatal().Msgf("The raft node could not connect to the leader as a new voter; %s", err)
		}
		client := pb.NewShardNodeClient(conn)
		joinRaftVoterReply, err := client.JoinRaftVoter(
			context.Background(),
			&pb.JoinRaftVoterRequest{
				NodeId:   int32(replicaID),
				NodeAddr: fmt.Sprintf("%s:%d", advertiseIp, raftPort),
			},
		)
		if err != nil || !joinRaftVoterReply.Success {
			log.Error().Msgf("The raft node could not connect to the leader as a new voter; %s", err)
		}
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bindIp, rpcPort))
	if err != nil {
		log.Fatal().Msgf("failed to listen: %v", err)
	}
	storageORAMNodeMap := make(map[int]int)
	for _, storage := range storages {
		storageORAMNodeMap[storage.ID] = storage.ORAMNodeID
	}
	shardnodeServer := newShardNodeServer(shardNodeServerID, replicaID, r, shardNodeFSM, oramNodeRPCClients, storageORAMNodeMap, parameters.TreeHeight, newBatchManager(time.Duration(parameters.BatchTimout)*time.Millisecond))
	go shardnodeServer.sendBatchesForever()

	go func() {
		for {
			time.Sleep(1 * time.Second)
			shardnodeServer.shardNodeFSM.printStashSize()
		}
	}()

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(rpc.ContextPropagationUnaryServerInterceptor()))
	pb.RegisterShardNodeServer(grpcServer, shardnodeServer)
	grpcServer.Serve(lis)
}
