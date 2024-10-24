package oramnode

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	pb "github.com/dsg-uwaterloo/treebeard/api/oramnode"
	"github.com/dsg-uwaterloo/treebeard/pkg/commonerrs"
	"github.com/dsg-uwaterloo/treebeard/pkg/config"
	"github.com/dsg-uwaterloo/treebeard/pkg/rpc"
	strg "github.com/dsg-uwaterloo/treebeard/pkg/storage"
	"github.com/hashicorp/raft"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type storage interface {
	// GetMaxAccessCount() int
	LockStorage(storageID int)
	UnlockStorage(storageID int)
	BatchReadBucket(bucketIDs []int, storageID int) (blocks map[int]map[string]string, err error)
	BatchWriteBucket(storageID int, readBucketBlocksList map[int]map[string]string, shardNodeBlocks map[string]strg.BlockInfo) (writtenBlocks map[string]string, err error)
	BatchReadBlock(offsets map[int]int, storageID int) (values map[int]string, err error)
	GetBucketsInPaths(paths []int) (bucketIDs []int, err error)
	GetRandomStorageID() int
	GetMultipleReverseLexicographicPaths(evictionCount int, count int) (paths []int)
}

type oramNodeServer struct {
	pb.UnimplementedOramNodeServer
	oramNodeServerID    int
	replicaID           int
	raftNode            *raft.Raft
	oramNodeFSM         *oramNodeFSM
	shardNodeRPCClients ShardNodeRPCClients
	readPathCounter     atomic.Int32
	storageHandler      storage
	parameters          config.Parameters
}

func newOramNodeServer(oramNodeServerID int, replicaID int, raftNode *raft.Raft, oramNodeFSM *oramNodeFSM, shardNodeRPCClients map[int]ReplicaRPCClientMap, storageHandler storage, parameters config.Parameters) *oramNodeServer {
	return &oramNodeServer{
		oramNodeServerID:    oramNodeServerID,
		replicaID:           replicaID,
		raftNode:            raftNode,
		oramNodeFSM:         oramNodeFSM,
		shardNodeRPCClients: shardNodeRPCClients,
		readPathCounter:     atomic.Int32{},
		storageHandler:      storageHandler,
		parameters:          parameters,
	}
}

// It runs the failed eviction and read path as the new leader.
func (o *oramNodeServer) performFailedOperations() error {
	<-o.raftNode.LeaderCh()
	o.oramNodeFSM.unfinishedEvictionMu.Lock()
	o.oramNodeFSM.unfinishedReadPathMu.Lock()
	needsEviction := o.oramNodeFSM.unfinishedEviction
	o.oramNodeFSM.unfinishedEvictionMu.Unlock()
	o.oramNodeFSM.unfinishedReadPathMu.Unlock()
	if needsEviction != nil {
		o.oramNodeFSM.unfinishedEvictionMu.Lock()
		storageID := o.oramNodeFSM.unfinishedEviction.storageID
		o.oramNodeFSM.unfinishedEvictionMu.Unlock()
		log.Debug().Msgf("Performing failed eviction for storageID %d", storageID)
		o.evict(storageID)
	}
	return nil
}

type getAccessCountResponse struct {
	counts map[int]int
	err    error
}

type readBucketResponse struct {
	bucketValues map[int]map[string]string
	err          error
}

func (o *oramNodeServer) asyncReadBucket(bucketIDs []int, storageID int, responseChan chan readBucketResponse) {
	bucketValues, err := o.storageHandler.BatchReadBucket(bucketIDs, storageID)
	responseChan <- readBucketResponse{bucketValues: bucketValues, err: err}
}

func (o *oramNodeServer) readAllBuckets(buckets []int, storageID int) (blocksFromReadBucket map[int]map[string]string, err error) {
	log.Debug().Msgf("Reading all buckets with buckets %v and storageID %d", buckets, storageID)
	blocksFromReadBucket = make(map[int]map[string]string) // map of bucket to map of block to value
	for _, bucket := range buckets {
		blocksFromReadBucket[bucket] = make(map[string]string)
	}
	readBucketResponseChan := make(chan readBucketResponse)
	batches := distributeBucketIDs(buckets, o.parameters.RedisPipelineSize)
	for _, bucketIDs := range batches {
		go o.asyncReadBucket(bucketIDs, storageID, readBucketResponseChan)
	}

	for i := 0; i < len(batches); i++ {
		response := <-readBucketResponseChan
		log.Debug().Msgf("Got blocks %v", response.bucketValues)
		if response.err != nil {
			return nil, fmt.Errorf("unable to read bucket; %s", err)
		}
		for bucket, blockValues := range response.bucketValues {
			for block, value := range blockValues {
				blocksFromReadBucket[bucket][block] = value
			}
		}
	}
	return blocksFromReadBucket, nil
}

func (o *oramNodeServer) readBlocksFromShardNode(paths []int, storageID int, randomShardNode ReplicaRPCClientMap) (receivedBlocks map[string]strg.BlockInfo, err error) {
	log.Debug().Msgf("Reading blocks from shard node with paths %v and storageID %d", paths, storageID)
	receivedBlocks = make(map[string]strg.BlockInfo) // map of received block to value and path

	shardNodeBlocks, err := randomShardNode.getBlocksFromShardNode(storageID, o.parameters.MaxBlocksToSend)
	if err != nil {
		return nil, fmt.Errorf("unable to get blocks from shard node; %s", err)
	}
	for _, block := range shardNodeBlocks {
		receivedBlocks[block.Block] = strg.BlockInfo{Value: block.Value, Path: int(block.Path)}
	}
	return receivedBlocks, nil
}

func (o *oramNodeServer) writeBackBlocksToAllBuckets(buckets []int, storageID int, blocksFromReadBucket map[int]map[string]string, receivedBlocks map[string]strg.BlockInfo) (receivedBlocksIsWritten map[string]bool, err error) {
	log.Debug().Msgf("blocks from read bucket: %v", blocksFromReadBucket)
	log.Debug().Msgf("Writing back blocks to all buckets with buckets %v and storageID %d", buckets, storageID)
	receivedBlocksIsWritten = make(map[string]bool)
	for block := range receivedBlocks {
		receivedBlocksIsWritten[block] = false
	}
	// TODO: is there any way to parallelize this?
	batches := distributeBucketIDs(buckets, o.parameters.RedisPipelineSize)
	bucketIDToBatchIndex := make(map[int]int)
	for i, bucketIDs := range batches {
		for _, bucketID := range bucketIDs {
			bucketIDToBatchIndex[bucketID] = i
		}
	}
	batchedBlocksFromReadBucket := make([]map[int]map[string]string, len(batches))
	for bucket, blockValues := range blocksFromReadBucket {
		if batchedBlocksFromReadBucket[bucketIDToBatchIndex[bucket]] == nil {
			batchedBlocksFromReadBucket[bucketIDToBatchIndex[bucket]] = make(map[int]map[string]string)
		}
		batchedBlocksFromReadBucket[bucketIDToBatchIndex[bucket]][bucket] = blockValues
	}

	for _, blocksFromReadBucket := range batchedBlocksFromReadBucket {
		writtenBlocks, err := o.storageHandler.BatchWriteBucket(storageID, blocksFromReadBucket, receivedBlocks)
		if err != nil {
			return nil, fmt.Errorf("unable to atomic write bucket; %s", err)
		}
		for block := range writtenBlocks {
			if _, exists := receivedBlocks[block]; exists {
				delete(receivedBlocks, block)
				receivedBlocksIsWritten[block] = true
			}
		}
	}
	return receivedBlocksIsWritten, nil
}

func (o *oramNodeServer) evict(storageID int) error {
	o.storageHandler.LockStorage(storageID)
	defer o.storageHandler.UnlockStorage(storageID)
	currentEvictionCount := o.oramNodeFSM.evictionCountMap[storageID]
	paths := o.storageHandler.GetMultipleReverseLexicographicPaths(currentEvictionCount, o.parameters.EvictPathCount)
	log.Debug().Msgf("Evicting with paths %v and storageID %d", paths, storageID)
	beginEvictionCommand, err := newReplicateBeginEvictionCommand(currentEvictionCount, storageID)
	if err != nil {
		return fmt.Errorf("unable to marshal begin eviction command; %s", err)
	}
	err = o.raftNode.Apply(beginEvictionCommand, 0).Error()
	if err != nil {
		return fmt.Errorf("could not apply log to the FSM; %s", err)
	}

	buckets, err := o.storageHandler.GetBucketsInPaths(paths)
	if err != nil {
		return fmt.Errorf("unable to get buckets for paths; %v", err)
	}
	blocksFromReadBucket, err := o.readAllBuckets(buckets, storageID)
	if err != nil {
		return fmt.Errorf("unable to perform ReadBucket on all levels")
	}

	randomShardNode := o.shardNodeRPCClients.getRandomShardNodeClient()
	receivedBlocks, err := o.readBlocksFromShardNode(paths, storageID, randomShardNode)
	log.Debug().Msgf("Received blocks from shardnode %v", receivedBlocks)
	if err != nil {
		return err
	}

	receivedBlocksIsWritten, err := o.writeBackBlocksToAllBuckets(buckets, storageID, blocksFromReadBucket, receivedBlocks)
	log.Debug().Msgf("Received blocks is written %v", receivedBlocksIsWritten)
	if err != nil {
		return fmt.Errorf("unable to perform WriteBucket on all levels; %s", err)
	}

	randomShardNode.sendBackAcksNacks(receivedBlocksIsWritten)

	endEvictionCommand, err := newReplicateEndEvictionCommand(currentEvictionCount+o.parameters.EvictPathCount, storageID)
	if err != nil {
		return fmt.Errorf("unable to marshal end eviction command; %s", err)
	}
	err = o.raftNode.Apply(endEvictionCommand, 0).Error()
	if err != nil {
		return fmt.Errorf("could not apply log to the FSM; %s", err)
	}

	o.readPathCounter.Store(0)

	return nil
}

func (o *oramNodeServer) getDistinctPathsInBatch(requests []*pb.BlockRequest) []int {
	paths := make(map[int]bool)
	for _, request := range requests {
		paths[int(request.Path)] = true
	}
	var pathList []int
	for path := range paths {
		pathList = append(pathList, path)
	}
	return pathList
}

type readBlockResponse struct {
	values map[int]string
	err    error
}

func (o *oramNodeServer) asyncReadBlock(offsets map[int]int, storageID int, responseChan chan readBlockResponse) {
	values, err := o.storageHandler.BatchReadBlock(offsets, storageID)
	responseChan <- readBlockResponse{values: values, err: err}
}

func (o *oramNodeServer) ReadPath(ctx context.Context, request *pb.ReadPathRequest) (*pb.ReadPathReply, error) {
	if o.raftNode.State() != raft.Leader {
		return nil, fmt.Errorf(commonerrs.NotTheLeaderError)
	}
	log.Debug().Msgf("Received read path request %v", request)
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "oramnode read path request")
	o.storageHandler.LockStorage(int(request.StorageId))
	defer o.storageHandler.UnlockStorage(int(request.StorageId))

	var blocks []string
	for _, request := range request.Requests {
		blocks = append(blocks, request.Block)
	}

	realBlockBucketMapping := make(map[int]string) // map of bucket id to block

	paths := o.getDistinctPathsInBatch(request.Requests)
	beginReadPathCommand, err := newReplicateBeginReadPathCommand(paths, int(request.StorageId))
	if err != nil {
		return nil, fmt.Errorf("unable to create begin read path replication command; %v", err)
	}
	_, beginReadPathReplicationSpan := tracer.Start(ctx, "replicate begin read path")
	err = o.raftNode.Apply(beginReadPathCommand, 0).Error()
	if err != nil {
		return nil, fmt.Errorf("could not apply log to the FSM; %s", err)
	}
	beginReadPathReplicationSpan.End()

	buckets, err := o.storageHandler.GetBucketsInPaths(paths)
	log.Debug().Msgf("Got buckets %v", buckets)
	if err != nil {
		return nil, fmt.Errorf("could not get bucket ids in the paths; %v", err)
	}
	// _, getBlockOffsetsSpan := tracer.Start(ctx, "get block offsets")
	// offsetListResponseChan := make(chan blockOffsetResponse)
	batches := distributeBucketIDs(buckets, o.parameters.RedisPipelineSize)
	// for _, bucketIDs := range batches {
	// go o.asyncGetBlockOffset(bucketIDs, int(request.StorageId), blocks, offsetListResponseChan)
	// }
	// getBlockOffsetsSpan.End()
	var offsetList []map[int]int // list of map of bucket id to offset
	for i := 0; i < len(batches); i++ {
		offsetList = append(offsetList, make(map[int]int))
		for _, bucketID := range batches[i] {
			offsetList[i][bucketID] = 0
		}
	}
	log.Debug().Msgf("Got offsets %v", offsetList)

	returnValues := make(map[string]string) // map of block to value
	for _, block := range blocks {
		returnValues[block] = ""
	}
	_, readBlocksSpan := tracer.Start(ctx, "read blocks")
	readBlockResponseChan := make(chan readBlockResponse)

	for _, offsets := range offsetList {
		go o.asyncReadBlock(offsets, int(request.StorageId), readBlockResponseChan)
	}
	for i := 0; i < len(offsetList); i++ {
		response := <-readBlockResponseChan
		if response.err != nil {
			log.Error().Msgf("Could not read block %v; %s", response.values, response.err)
			return nil, err
		}
		for bucketID, value := range response.values {
			if _, exists := realBlockBucketMapping[bucketID]; exists {
				returnValues[realBlockBucketMapping[bucketID]] = value
			}
		}
	}
	readBlocksSpan.End()
	log.Debug().Msgf("Going to return values %v", returnValues)

	o.readPathCounter.Add(1)

	endReadPathCommand, err := newReplicateEndReadPathCommand()
	if err != nil {
		return nil, fmt.Errorf("unable to create end read path replication command; %s", err)
	}
	_, endReadPathReplicationSpan := tracer.Start(ctx, "replicate end read path")
	err = o.raftNode.Apply(endReadPathCommand, 0).Error()
	if err != nil {
		return nil, fmt.Errorf("could not apply log to the FSM; %s", err)
	}
	endReadPathReplicationSpan.End()

	var response []*pb.BlockResponse
	for block, value := range returnValues {
		response = append(response, &pb.BlockResponse{Block: block, Value: value})
	}
	span.End()
	log.Debug().Msgf("Returning read path response %v", response)
	return &pb.ReadPathReply{Responses: response}, nil
}

func (o *oramNodeServer) JoinRaftVoter(ctx context.Context, joinRaftVoterRequest *pb.JoinRaftVoterRequest) (*pb.JoinRaftVoterReply, error) {
	requestingNodeId := joinRaftVoterRequest.NodeId
	requestingNodeAddr := joinRaftVoterRequest.NodeAddr

	log.Printf("received join request from node %d at %s", requestingNodeId, requestingNodeAddr)

	err := o.raftNode.AddVoter(
		raft.ServerID(strconv.Itoa(int(requestingNodeId))),
		raft.ServerAddress(requestingNodeAddr),
		0, 0).Error()

	if err != nil {
		return &pb.JoinRaftVoterReply{Success: false}, fmt.Errorf("voter could not be added to the leader; %s", err)
	}
	return &pb.JoinRaftVoterReply{Success: true}, nil
}

func StartServer(oramNodeServerID int, bindIP string, advIP string, rpcPort int, replicaID int, raftPort int, joinAddr string, shardNodeRPCClients map[int]ReplicaRPCClientMap, redisEndpoints []config.RedisEndpoint, parameters config.Parameters) {
	isFirst := joinAddr == ""
	oramNodeFSM := newOramNodeFSM()
	r, err := startRaftServer(isFirst, bindIP, advIP, replicaID, raftPort, oramNodeFSM)
	if err != nil {
		log.Fatal().Msgf("The raft node creation did not succeed; %s", err)
	}

	if !isFirst {
		conn, err := grpc.Dial(joinAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatal().Msgf("The raft node could not connect to the leader as a new voter; %s", err)
		}
		client := pb.NewOramNodeClient(conn)
		joinRaftVoterReply, err := client.JoinRaftVoter(
			context.Background(),
			&pb.JoinRaftVoterRequest{
				NodeId:   int32(replicaID),
				NodeAddr: fmt.Sprintf("%s:%d", advIP, raftPort),
			},
		)
		if err != nil || !joinRaftVoterReply.Success {
			log.Error().Msgf("The raft node could not connect to the leader as a new voter; %s", err)
		}
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bindIP, rpcPort))
	if err != nil {
		log.Fatal().Msgf("failed to listen: %v", err)
	}
	var storages []config.RedisEndpoint
	for _, redisEndpoint := range redisEndpoints {
		if redisEndpoint.ORAMNodeID == oramNodeServerID {
			storages = append(storages, redisEndpoint)
		}
	}
	storageHandler := strg.NewStorageHandler(parameters.TreeHeight, parameters.Z, parameters.Shift, storages)
	if isFirst {
		err = storageHandler.InitDatabase()
		if err != nil {
			log.Fatal().Msgf("failed to initialize the database: %v", err)
		}
	}
	oramNodeServer := newOramNodeServer(oramNodeServerID, replicaID, r, oramNodeFSM, shardNodeRPCClients, storageHandler, parameters)
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if oramNodeServer.readPathCounter.Load() >= int32(oramNodeServer.parameters.EvictionRate) {
				storageID := oramNodeServer.storageHandler.GetRandomStorageID()
				oramNodeServer.evict(storageID)
			}
		}
	}()
	go func() {
		for {
			oramNodeServer.performFailedOperations()
		}
	}()
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(rpc.ContextPropagationUnaryServerInterceptor()))
	pb.RegisterOramNodeServer(grpcServer, oramNodeServer)
	grpcServer.Serve(lis)
}
