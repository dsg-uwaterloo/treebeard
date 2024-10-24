package storage

// import (
// 	"context"
// 	"fmt"
// 	"testing"

// 	"github.com/dsg-uwaterloo/treebeard/pkg/config"
// 	"github.com/redis/go-redis/v9"
// )

// func TestParseMetadataBlockParsesKeyAndPos(t *testing.T) {
// 	metadata := "2dummy1"
// 	pos, key, _ := parseMetadataBlock(metadata)
// 	if key != "dummy1" {
// 		t.Errorf("expected user1, but found %s", key)
// 	}
// 	if pos != 2 {
// 		t.Errorf("expected 2, but found %d", pos)
// 	}
// }

// func TestParseMetadataBlockReturnsNegativePosOnInvalidMetadata(t *testing.T) {
// 	metadata := "__nul__"
// 	pos, _, _ := parseMetadataBlock(metadata)
// 	if pos != -1 {
// 		t.Errorf("expected -1, but found %d", pos)
// 	}
// }

// func TestParseMetadataBlocksReturnsAllValidRealAndDummyBlocks(t *testing.T) {
// 	metadataMap := make(map[int]*redis.MapStringStringCmd)
// 	metadataMap[1] = redis.NewMapStringStringCmd(context.Background())
// 	metadataMap[1].SetVal(map[string]string{
// 		"accessCount": "4",
// 		"1":           "2user1",
// 		"2":           "2dummy1",
// 		"3":           "3dummy2",
// 		"4":           "__null__",
// 	})
// 	expectedOffsetMap1 := map[string]int{
// 		"user1":  2,
// 		"dummy1": 2,
// 		"dummy2": 3,
// 	}
// 	metadataMap[2] = redis.NewMapStringStringCmd(context.Background())
// 	metadataMap[2].SetVal(map[string]string{
// 		"accessCount": "4",
// 		"1":           "2user5",
// 		"2":           "2dummy3",
// 		"3":           "__null__",
// 		"4":           "__null__",
// 	})
// 	expectedOffsetMap2 := map[string]int{
// 		"user5":  2,
// 		"dummy3": 2,
// 	}
// 	blockOffsets, err := parseMetadataBlocks(metadataMap)
// 	if err != nil {
// 		t.Errorf("error parsing metadata blocks")
// 	}
// 	if len(blockOffsets) != 2 {
// 		t.Errorf("expected 2 buckets, but found %d", len(blockOffsets))
// 	}
// 	if len(blockOffsets[1]) != len(expectedOffsetMap1) {
// 		t.Errorf("expected %d blocks, but found %d", len(expectedOffsetMap1), len(blockOffsets[1]))
// 	}
// 	if len(blockOffsets[2]) != len(expectedOffsetMap2) {
// 		t.Errorf("expected %d blocks, but found %d", len(expectedOffsetMap2), len(blockOffsets[2]))
// 	}
// 	for key, val := range expectedOffsetMap1 {
// 		if blockOffsets[1][key] != val {
// 			t.Errorf("expected %d, but found %d", val, blockOffsets[1][key])
// 		}
// 	}
// 	for key, val := range expectedOffsetMap2 {
// 		if blockOffsets[2][key] != val {
// 			t.Errorf("expected %d, but found %d", val, blockOffsets[2][key])
// 		}
// 	}
// }

// // Expects redis to be running on port 6379
// func TestBatchGetAllMetaDataReturnsAllBucketOffsets(t *testing.T) {
// 	storageHandler := NewStorageHandler(3, 1, 9, 1, []config.RedisEndpoint{{ID: 0, IP: "localhost", Port: 6379}})
// 	storageHandler.InitDatabase()
// 	pipe := storageHandler.storages[0].Pipeline()
// 	storageHandler.BatchPushDataAndMetadata(1, []string{"user1", "user2", "user3"}, []string{"2user5", "3user2"}, pipe)
// 	_, err := pipe.Exec(context.Background())
// 	if err != nil {
// 		t.Errorf("error pushing data and metadata")
// 	}
// 	metadatas, err := storageHandler.BatchGetAllMetaData([]int{1}, 0)
// 	if err != nil {
// 		t.Errorf("error getting metadata")
// 	}
// 	metadataBucket1 := metadatas[1]
// 	fmt.Println(metadataBucket1)
// 	expectedOffsetMap := map[string]int{
// 		"user5": 2,
// 		"user2": 3,
// 	}
// 	for key, val := range expectedOffsetMap {
// 		if metadataBucket1[key] != val {
// 			t.Errorf("expected %d, but found %d", val, metadataBucket1[key])
// 		}
// 	}
// }

// func TestBatchPushDataAndMetadataResetsAccessCount(t *testing.T) {
// 	storageHandler := NewStorageHandler(3, 1, 9, 1, []config.RedisEndpoint{{ID: 0, IP: "localhost", Port: 6379}})
// 	storageHandler.InitDatabase()
// 	pipe := storageHandler.storages[0].Pipeline()
// 	storageHandler.BatchPushDataAndMetadata(1, []string{"user1", "user2", "user3"}, []string{"2user5", "3user2"}, pipe)
// 	_, err := pipe.Exec(context.Background())
// 	if err != nil {
// 		t.Errorf("error pushing data and metadata")
// 	}
// 	counts, err := storageHandler.BatchGetAccessCount([]int{1}, 0)
// 	if err != nil {
// 		t.Errorf("error getting access count")
// 	}
// 	if counts[1] != 0 {
// 		t.Errorf("expected 0, but found %d", counts[1])
// 	}
// }
