package oramnode

// type mockShardNodeClient struct {
// 	sendBlocksReply    func() (*shardnodepb.SendBlocksReply, error)
// 	ackSentBlocksReply func() (*shardnodepb.AckSentBlocksReply, error)
// }

// func (m *mockShardNodeClient) BatchQuery(ctx context.Context, in *shardnodepb.RequestBatch, opts ...grpc.CallOption) (*shardnodepb.ReplyBatch, error) {
// 	return nil, nil
// }

// func (m *mockShardNodeClient) Read(ctx context.Context, in *shardnodepb.ReadRequest, opts ...grpc.CallOption) (*shardnodepb.ReadReply, error) {
// 	return nil, nil
// }
// func (m *mockShardNodeClient) Write(ctx context.Context, in *shardnodepb.WriteRequest, opts ...grpc.CallOption) (*shardnodepb.WriteReply, error) {
// 	return nil, nil
// }
// func (m *mockShardNodeClient) SendBlocks(ctx context.Context, in *shardnodepb.SendBlocksRequest, opts ...grpc.CallOption) (*shardnodepb.SendBlocksReply, error) {
// 	return m.sendBlocksReply()
// }
// func (m *mockShardNodeClient) AckSentBlocks(ctx context.Context, in *shardnodepb.AckSentBlocksRequest, opts ...grpc.CallOption) (*shardnodepb.AckSentBlocksReply, error) {
// 	return m.ackSentBlocksReply()
// }
// func (m *mockShardNodeClient) JoinRaftVoter(ctx context.Context, in *shardnodepb.JoinRaftVoterRequest, opts ...grpc.CallOption) (*shardnodepb.JoinRaftVoterReply, error) {
// 	return nil, nil
// }

// func getMockShardNodeClients() map[int]ReplicaRPCClientMap {
// 	return map[int]ReplicaRPCClientMap{
// 		0: map[int]ShardNodeRPCClient{
// 			0: {
// 				ClientAPI: &mockShardNodeClient{
// 					sendBlocksReply: func() (*shardnodepb.SendBlocksReply, error) {
// 						return &shardnodepb.SendBlocksReply{
// 							Blocks: []*shardnodepb.Block{
// 								{Block: "a", Value: "valA"},
// 								{Block: "b", Value: "valB"},
// 								{Block: "c", Value: "valC"},
// 								{Block: "d", Value: "valD"},
// 							},
// 						}, nil
// 					},
// 					ackSentBlocksReply: func() (*shardnodepb.AckSentBlocksReply, error) {
// 						return &shardnodepb.AckSentBlocksReply{Success: true}, nil
// 					},
// 				},
// 			},
// 			1: {
// 				ClientAPI: &mockShardNodeClient{
// 					sendBlocksReply: func() (*shardnodepb.SendBlocksReply, error) {
// 						return nil, fmt.Errorf("not the leader")
// 					},
// 					ackSentBlocksReply: func() (*shardnodepb.AckSentBlocksReply, error) {
// 						return nil, fmt.Errorf("not the leader")
// 					},
// 				},
// 			},
// 		},
// 	}
// }

// func getFailedMockShardNodeClients() map[int]ReplicaRPCClientMap {
// 	return map[int]ReplicaRPCClientMap{
// 		0: map[int]ShardNodeRPCClient{
// 			0: {
// 				ClientAPI: &mockShardNodeClient{
// 					sendBlocksReply: func() (*shardnodepb.SendBlocksReply, error) {
// 						return nil, fmt.Errorf("not the leader")
// 					},
// 					ackSentBlocksReply: func() (*shardnodepb.AckSentBlocksReply, error) {
// 						return nil, fmt.Errorf("not the leader")
// 					},
// 				},
// 			},
// 			1: {
// 				ClientAPI: &mockShardNodeClient{
// 					sendBlocksReply: func() (*shardnodepb.SendBlocksReply, error) {
// 						return nil, fmt.Errorf("not the leader")
// 					},
// 					ackSentBlocksReply: func() (*shardnodepb.AckSentBlocksReply, error) {
// 						return nil, fmt.Errorf("not the leader")
// 					},
// 				},
// 			},
// 		},
// 	}
// }

// func startLeaderRaftNodeServer(t *testing.T, storageHandler storage) *oramNodeServer {
// 	fsm := newOramNodeFSM()
// 	raftPort, err := freeport.GetFreePort()
// 	if err != nil {
// 		t.Errorf("unable to get free port")
// 	}
// 	r, err := startRaftServer(true, "localhost", "localhost", 0, raftPort, fsm)
// 	if err != nil {
// 		t.Errorf("unable to start raft server")
// 	}
// 	<-r.LeaderCh() // wait to become the leader
// 	o := newOramNodeServer(0, 0, r, fsm, getMockShardNodeClients(), storageHandler, config.Parameters{MaxBlocksToSend: 5, EvictionRate: 4})
// 	return o
// }

// func (o *oramNodeServer) withFailedShardNodeClients() *oramNodeServer {
// 	o.shardNodeRPCClients = getFailedMockShardNodeClients()
// 	return o
// }

// func TestReadAllBucketsReturnsAllTreeBlocks(t *testing.T) {
// 	expectedBlocks := map[int]map[string]string{
// 		1: {
// 			"1": "val1",
// 			"2": "val2",
// 			"3": "val3",
// 		},
// 		2: {
// 			"4": "val4",
// 			"5": "val5",
// 		},
// 		3: {
// 			"6": "val6",
// 			"7": "val7",
// 		},
// 	}
// 	mockStorageHandler := strg.NewMockStorageHandler(3, 4).WithCustomBatchReadBucketFunc(
// 		func(bucketIDs []int, storageID int) (blocks map[int]map[string]string, err error) {
// 			return expectedBlocks, nil
// 		},
// 	)

// 	o := startLeaderRaftNodeServer(t, mockStorageHandler)
// 	o.parameters.RedisPipelineSize = 2
// 	blocks, err := o.readAllBuckets([]int{1, 2, 3}, 1)
// 	if err != nil {
// 		t.Errorf("Expected successful execution of readBucketAllLevels")
// 	}

// 	for bucketID, bucketBlocks := range expectedBlocks {
// 		for block, val := range bucketBlocks {
// 			if blocks[bucketID][block] != val {
// 				t.Errorf("Expected block %s to have value %s", block, val)
// 			}
// 		}
// 	}
// }

// func TestReadBlocksFromShardNodeReturnsAllShardNodeBlocks(t *testing.T) {
// 	o := startLeaderRaftNodeServer(t, strg.NewMockStorageHandler(3, 4))
// 	blocks, err := o.readBlocksFromShardNode([]int{4, 7, 11}, 1, o.shardNodeRPCClients[0])
// 	if err != nil {
// 		t.Errorf("expected Successful execution of readBlocksFromShardNode")
// 	}
// 	expectedBlocks := []string{"a", "b", "c", "d"}
// 	for _, block := range expectedBlocks {
// 		if _, exists := blocks[block]; !exists {
// 			t.Errorf("expected value %s to be in the received blocks", block)
// 		}
// 	}
// }

// func TestWriteBackBlocksToAllBucketsPushesReceivedBlocksToTree(t *testing.T) {
// 	m := strg.NewMockStorageHandler(3, 4).WithCustomBatchWriteBucketFunc(
// 		func(storageID int, readBucketBlocksList map[int]map[string]string, shardNodeBlocks map[string]strg.BlockInfo) (writtenBlocks map[string]string, err error) {
// 			writtenBlocks = make(map[string]string)
// 			// We sucessfuly write the blocks that we read from the buckets back to the tree
// 			for _, bucketBlocks := range readBucketBlocksList {
// 				for block, val := range bucketBlocks {
// 					writtenBlocks[block] = val
// 				}
// 			}
// 			// We only write back a single block from the shard node
// 			// (It simulates a scenario where we couldn't push all the blocks from the shard node to the tree in a single steps)
// 			for block, val := range shardNodeBlocks {
// 				writtenBlocks[block] = val.Value
// 				break
// 			}
// 			return writtenBlocks, nil
// 		},
// 	)
// 	o := startLeaderRaftNodeServer(t, m)
// 	o.parameters.RedisPipelineSize = 2
// 	receivedBlocksIsWritten, err := o.writeBackBlocksToAllBuckets([]int{1, 2, 3, 4, 5, 6},
// 		1,
// 		map[int]map[string]string{
// 			1: {
// 				"1": "val1",
// 				"2": "val2",
// 				"3": "val3",
// 			},
// 			2: {
// 				"4": "val4",
// 				"5": "val5",
// 			},
// 			3: {
// 				"6": "val6",
// 				"7": "val7",
// 			},
// 			4: {},
// 			5: {
// 				"8": "val8",
// 			},
// 			6: {},
// 		},
// 		map[string]strg.BlockInfo{
// 			"a": {Value: "valA", Path: 0},
// 			"b": {Value: "valB", Path: 0},
// 			"c": {Value: "valC", Path: 0},
// 		},
// 	)
// 	if err != nil {
// 		t.Errorf("expected successful execution of writeBackBlocksToAllLevels")
// 	}
// 	for _, block := range []string{"a", "b", "c"} {
// 		if receivedBlocksIsWritten[block] == false {
// 			t.Errorf("expected IsWritten to be true for block %s", block)
// 		}
// 	}
// }

// func TestWriteBackBlocksToAllBucketsReturnsFalseForNotPushedReceivedBlocks(t *testing.T) {
// 	m := strg.NewMockStorageHandler(3, 4).WithCustomBatchWriteBucketFunc(
// 		func(storageID int, readBucketBlocksList map[int]map[string]string, shardNodeBlocks map[string]strg.BlockInfo) (writtenBlocks map[string]string, err error) {
// 			writtenBlocks = make(map[string]string)
// 			// We sucessfuly write the blocks that we read from the buckets back to the tree
// 			for _, bucketBlocks := range readBucketBlocksList {
// 				for block, val := range bucketBlocks {
// 					writtenBlocks[block] = val
// 				}
// 			}
// 			// We only write back a single block from the shard node
// 			// (It simulates a scenario where we couldn't push all the blocks from the shard node to the tree in a single steps)
// 			for block, val := range shardNodeBlocks {
// 				writtenBlocks[block] = val.Value
// 				break
// 			}
// 			return writtenBlocks, nil
// 		},
// 	)
// 	o := startLeaderRaftNodeServer(t, m)
// 	o.parameters.RedisPipelineSize = 2
// 	receivedBlocksIsWritten, err := o.writeBackBlocksToAllBuckets([]int{1, 2, 3, 4, 5, 6},
// 		1,
// 		map[int]map[string]string{
// 			1: {
// 				"1": "val1",
// 				"2": "val2",
// 				"3": "val3",
// 			},
// 			2: {
// 				"4": "val4",
// 				"5": "val5",
// 			},
// 			3: {
// 				"6": "val6",
// 				"7": "val7",
// 			},
// 			4: {},
// 			5: {
// 				"8": "val8",
// 			},
// 			6: {},
// 		},
// 		map[string]strg.BlockInfo{
// 			"a": {Value: "valA", Path: 0},
// 			"b": {Value: "valB", Path: 0},
// 			"c": {Value: "valC", Path: 0},
// 			"d": {Value: "valD", Path: 0},
// 			// One will not be written to the tree since we only write back a single block from the shard node every time (3 blocks in total)
// 		},
// 	)
// 	if err != nil {
// 		t.Errorf("expected successful execution of writeBackBlocksToAllLevels")
// 	}
// 	notWrittenCount := 0
// 	for _, block := range []string{"a", "b", "c", "d"} {
// 		if receivedBlocksIsWritten[block] == false {
// 			notWrittenCount++
// 		}
// 	}
// 	if notWrittenCount != 1 {
// 		t.Errorf("expected IsWritten to be false for 1 block")
// 	}
// }

// func TestEvictCleansUpBeginEvictionAfterSuccessfulExecution(t *testing.T) {
// 	o := startLeaderRaftNodeServer(t, strg.NewMockStorageHandler(3, 4))
// 	o.parameters.RedisPipelineSize = 2
// 	o.evict(0)
// 	o.oramNodeFSM.unfinishedEvictionMu.Lock()
// 	defer o.oramNodeFSM.unfinishedEvictionMu.Unlock()

// 	if o.oramNodeFSM.unfinishedEviction != nil {
// 		t.Errorf("evict should remove unfinished eviction after successful eviction")
// 	}
// }

// func TestEvictKeepsBeginEvictionInFailureScenario(t *testing.T) {
// 	o := startLeaderRaftNodeServer(t, strg.NewMockStorageHandler(3, 4)).withFailedShardNodeClients()
// 	o.parameters.RedisPipelineSize = 2
// 	o.evict(0)
// 	fmt.Println("Failed and came back")
// 	o.oramNodeFSM.unfinishedEvictionMu.Lock()
// 	defer o.oramNodeFSM.unfinishedEvictionMu.Unlock()
// 	if o.oramNodeFSM.unfinishedEviction == nil {
// 		t.Errorf("evict should add an unfinished eviction to FSM in failure scenarios")
// 	}
// }

// func TestEvictResetsReadPathCounter(t *testing.T) {
// 	o := startLeaderRaftNodeServer(t, strg.NewMockStorageHandler(3, 4))
// 	o.parameters.RedisPipelineSize = 2
// 	o.evict(0)
// 	if o.readPathCounter.Load() != 0 {
// 		t.Errorf("Evict should reset readPathCounter after successful execution")
// 	}
// }

// func TestGetDistinctPathsInBatchReturnsDistinctPathsInRequests(t *testing.T) {
// 	o := newOramNodeServer(0, 0, &raft.Raft{}, &oramNodeFSM{}, make(map[int]ReplicaRPCClientMap), &strg.StorageHandler{}, config.Parameters{})
// 	paths := o.getDistinctPathsInBatch(
// 		[]*oramnode.BlockRequest{
// 			{Block: "a", Path: 1},
// 			{Block: "b", Path: 1},
// 			{Block: "c", Path: 2},
// 			{Block: "d", Path: 3},
// 			{Block: "e", Path: 2},
// 			{Block: "f", Path: 1},
// 		},
// 	)
// 	sort.Ints(paths)

// 	expectedPaths := []int{1, 2, 3}
// 	if len(paths) != 3 {
// 		t.Errorf("expected 3 unique paths")
// 	}
// 	for i, path := range paths {
// 		if path != expectedPaths[i] {
// 			t.Errorf("expected path %d but got %d", expectedPaths[i], path)
// 		}
// 	}
// }

// func TestReadPathIncrementsReadPathCounter(t *testing.T) {
// 	m := strg.NewMockStorageHandler(3, 4).WithCusomBatchGetBlockOffsetFunc(
// 		func(bucketIDs []int, storageID int, blocks []string) (offsets map[int]strg.BlockOffsetStatus, err error) {
// 			offsets = make(map[int]strg.BlockOffsetStatus)
// 			for _, bucketID := range bucketIDs {
// 				offsets[bucketID] = strg.BlockOffsetStatus{
// 					Offset:     0,
// 					IsReal:     true,
// 					BlockFound: "a",
// 				}
// 			}
// 			return offsets, nil
// 		},
// 	)
// 	o := startLeaderRaftNodeServer(t, m)
// 	o.parameters.RedisPipelineSize = 2
// 	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("requestid", "request1"))
// 	o.ReadPath(ctx, &oramnode.ReadPathRequest{StorageId: 2, Requests: []*oramnode.BlockRequest{{Block: "a", Path: 1}}})
// 	if o.readPathCounter.Load() != 1 {
// 		t.Errorf("ReadPath should increment readPathCounter")
// 	}
// }
