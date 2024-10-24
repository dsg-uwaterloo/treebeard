package storage

// import (
// 	"context"
// 	"strconv"
// 	"strings"
// 	"testing"

// 	"github.com/dsg-uwaterloo/treebeard/pkg/config"
// 	"github.com/rs/zerolog/log"
// )

// func TestGetBucketsInPathsReturnsAllBucketIDsInPath(t *testing.T) {
// 	s := NewStorageHandler(3, 9, 1, 1, []config.RedisEndpoint{})
// 	buckets, err := s.GetBucketsInPaths([]int{1})
// 	expectedMap := map[int]bool{
// 		4: true,
// 		2: true,
// 		1: true,
// 	}
// 	if err != nil {
// 		t.Errorf("expected no erros in GetBucketsInPaths")
// 	}
// 	for _, bucket := range buckets {
// 		if _, exists := expectedMap[bucket]; !exists {
// 			t.Errorf("expected bucketID %d to exist", bucket)
// 		}
// 	}
// }

// func TestGetNextReverseLexicographicPath(t *testing.T) {
// 	numberOfEvictions := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
// 	expectedPaths := []int{1, 3, 2, 4, 1, 3, 2, 4, 1}
// 	for i, eviction := range numberOfEvictions {
// 		path := GetNextReverseLexicographicPath(eviction, 3)
// 		if path != expectedPaths[i] {
// 			t.Errorf("expected path %d, but got %d", expectedPaths[i], path)
// 		}
// 	}
// }

// func TestGetMultipleReverseLexicographicPaths(t *testing.T) {
// 	pathCount := 5
// 	currentEvictionCount := 1
// 	expectedPaths := []int{3, 2, 4, 1, 3}
// 	s := NewStorageHandler(3, 1, 9, 1, []config.RedisEndpoint{{ID: 0, IP: "localhost", Port: 6379}})
// 	paths := s.GetMultipleReverseLexicographicPaths(currentEvictionCount, pathCount)
// 	if len(paths) != pathCount {
// 		t.Errorf("expected %d paths, but got %d", pathCount, len(paths))
// 	}
// 	for i, path := range paths {
// 		if path != expectedPaths[i] {
// 			t.Errorf("expected path %d, but got %d", expectedPaths[i], path)
// 		}
// 	}
// }

// func TestBatchWriteBucket(t *testing.T) {
// 	log.Debug().Msgf("TestBatchWriteBucket")
// 	bucketIds := []int{0, 1, 2, 3, 4, 5}
// 	storageId := 0
// 	s := NewStorageHandler(3, 1, 9, 1, []config.RedisEndpoint{{ID: 0, IP: "localhost", Port: 6379}})
// 	s.InitDatabase()
// 	expectedWrittenBlocks := map[string]string{"usr0": "value0", "usr1": "value1", "usr2": "value2", "usr3": "value3", "usr4": "value4", "usr5": "value5"}
// 	toWriteBlocks := map[int]map[string]string{0: {"usr0": "value0"}, 1: {"usr1": "value1"}, 2: {"usr2": "value2"}, 3: {"usr3": "value3"}, 4: {"usr4": "value4"}, 5: {"usr5": "value5"}}
// 	writtenBlocks, _ := s.BatchWriteBucket(storageId, toWriteBlocks, map[string]BlockInfo{})
// 	for block := range writtenBlocks {
// 		if _, exist := expectedWrittenBlocks[block]; !exist {
// 			t.Errorf("%s was written", block)
// 		}
// 	}
// 	metadatas, err := s.BatchGetAllMetaData(bucketIds, storageId)
// 	if err != nil {
// 		t.Errorf("error getting metadata")
// 	}
// 	for _, bucketID := range bucketIds {
// 		metadata := metadatas[bucketID]
// 		for key, _ := range toWriteBlocks[bucketID] {
// 			if _, exist := metadata[key]; !exist {
// 				t.Errorf("%s was not written", key)
// 			}
// 		}
// 	}
// }

// func TestBatchReadBlock(t *testing.T) {
// 	log.Debug().Msgf("TestBatchReadBlock")
// 	bucketIds := []int{1, 2, 3, 4, 5}
// 	storageId := 0
// 	s := NewStorageHandler(3, 1, 9, 1, []config.RedisEndpoint{{ID: 0, IP: "localhost", Port: 6379}})
// 	s.InitDatabase()
// 	toWriteBlocks := map[int]map[string]string{1: {"usr1": "value1"}, 2: {"usr2": "value2"}, 3: {"usr3": "value3"}, 4: {"usr4": "value4"}, 5: {"usr5": "value5"}}
// 	s.BatchWriteBucket(storageId, toWriteBlocks, map[string]BlockInfo{})
// 	metadatas, err := s.BatchGetAllMetaData(bucketIds, storageId)
// 	if err != nil {
// 		t.Errorf("error getting metadata")
// 	}
// 	for _, bucketID := range bucketIds {
// 		metadata := metadatas[bucketID]
// 		for key, pos := range metadata {
// 			if strings.HasPrefix(key, "dummy") {
// 				continue
// 			}
// 			res := s.storages[0].HGet(context.Background(), strconv.Itoa(bucketID), strconv.Itoa(pos))
// 			decrypted, _ := Decrypt(res.Val(), s.key)
// 			if decrypted != toWriteBlocks[bucketID][key] {
// 				t.Errorf("expected %s, but got %s", toWriteBlocks[bucketID][key], decrypted)
// 			}
// 		}
// 	}
// }

// func TestBatchGetBlockOffset(t *testing.T) {
// 	bucketIDs := []int{1, 2, 3, 4, 5}
// 	s := NewStorageHandler(4, 1, 9, 1, []config.RedisEndpoint{{ID: 0, IP: "localhost", Port: 6379}})
// 	s.InitDatabase()
// 	toWriteBlocks := map[int]map[string]string{1: {"usr1": "value1"}, 2: {"usr2": "value2"}, 3: {"usr3": "value3"}, 4: {"usr4": "value4"}, 5: {"usr5": "value5"}}
// 	s.BatchWriteBucket(0, toWriteBlocks, map[string]BlockInfo{})
// 	metadatas, err := s.BatchGetAllMetaData(bucketIDs, 0)
// 	if err != nil {
// 		t.Errorf("error getting metadata")
// 	}
// 	blockoffsetStatuses, err := s.BatchGetBlockOffset(bucketIDs, 0, []string{"usr1", "usr3"})
// 	if err != nil {
// 		t.Errorf("error getting block offset")
// 	}
// 	usr1Offset := 0
// 	usr3Offset := 0
// 	for _, bucketID := range bucketIDs {
// 		metadata := metadatas[bucketID]
// 		for key, pos := range metadata {
// 			if strings.HasPrefix(key, "dummy") {
// 				continue
// 			}
// 			if key == "usr1" {
// 				usr1Offset = pos
// 			}
// 			if key == "usr3" {
// 				usr3Offset = pos
// 			}
// 		}
// 	}
// 	if blockoffsetStatuses[1].Offset != usr1Offset || blockoffsetStatuses[1].IsReal != true || blockoffsetStatuses[1].BlockFound != "usr1" {
// 		t.Errorf("expected offset %d, but got %d", usr1Offset, blockoffsetStatuses[1].Offset)
// 	}
// 	if blockoffsetStatuses[3].Offset != usr3Offset || blockoffsetStatuses[3].IsReal != true || blockoffsetStatuses[3].BlockFound != "usr3" {
// 		t.Errorf("expected offset %d, but got %d", usr3Offset, blockoffsetStatuses[3].Offset)
// 	}
// }

// func TestBatchReadBucketReturnsBlocksInAllBuckets(t *testing.T) {
// 	s := NewStorageHandler(4, 1, 9, 1, []config.RedisEndpoint{{ID: 0, IP: "localhost", Port: 6379}})
// 	s.InitDatabase()
// 	toWriteBlocks := map[int]map[string]string{1: {"usr1": "value1"}, 2: {"usr2": "value2"}, 3: {"usr3": "value3"}, 4: {"usr4": "value4"}, 5: {"usr5": "value5"}}
// 	s.BatchWriteBucket(0, toWriteBlocks, map[string]BlockInfo{})
// 	blocks, err := s.BatchReadBucket([]int{1, 2, 3, 4, 5}, 0)
// 	if err != nil {
// 		t.Errorf("error reading bucket")
// 	}
// 	expectedReadBuckets := toWriteBlocks
// 	log.Debug().Msgf("blocks: %v", expectedReadBuckets)
// 	for bucketID, blockToVal := range expectedReadBuckets {
// 		for block, val := range blockToVal {
// 			if blocks[bucketID][block] != val {
// 				t.Errorf("expected %s, but got %s", val, blocks[bucketID][block])
// 			}
// 		}
// 	}
// }

// func TestRandom(t *testing.T) {
// 	s := NewStorageHandler(4, 1, 9, 1, []config.RedisEndpoint{{ID: 0, IP: "localhost", Port: 6379}})
// 	s.InitDatabase()
// 	toWriteBlocks := map[int]map[string]string{1: {"usr1": "value1"}, 2: {"usr2": "value2"}, 3: {"usr3": "value3"}, 4: {"usr4": "value4"}, 5: {"usr5": "value5"}}
// 	s.BatchWriteBucket(0, toWriteBlocks, map[string]BlockInfo{})
// }
