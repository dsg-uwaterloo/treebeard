package storage

// TODO: It might need to handle multiple storage shards.

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"github.com/dsg-uwaterloo/treebeard/pkg/config"
	"github.com/dsg-uwaterloo/treebeard/pkg/utils"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// Path and bucket id start from one.

// StorageHandler is responsible for handling one or multiple storage shards.
type StorageHandler struct {
	treeHeight int
	Z          int // the maximum number of real blocks in each bucket
	S          int // the number of dummy blocks in each bucket
	shift      int
	storages   map[int]*redis.Client // map of storage id to redis client
	storageMus map[int]*sync.Mutex   // map of storage id to mutex
	key        []byte
}

type BlockInfo struct {
	Value string
	Path  int
}

func NewStorageHandler(treeHeight int, Z int, S int, shift int, redisEndpoints []config.RedisEndpoint) *StorageHandler { // map of storage id to storage info
	log.Debug().Msgf("Creating a new storage handler")
	storages := make(map[int]*redis.Client)
	for _, endpoint := range redisEndpoints {
		storages[endpoint.ID] = getClient(endpoint.IP, endpoint.Port)
	}
	storageMus := make(map[int]*sync.Mutex)
	for storageID := range storages {
		storageMus[storageID] = &sync.Mutex{}
	}
	storageLatestEviction := make(map[int]int)
	for _, endpoint := range redisEndpoints {
		storageLatestEviction[endpoint.ID] = 0
	}
	s := &StorageHandler{
		treeHeight: treeHeight,
		Z:          Z,
		S:          S,
		shift:      shift,
		storages:   storages,
		storageMus: storageMus,
		key:        []byte("passphrasewhichneedstobe32bytes!"),
	}
	return s
}

func (s *StorageHandler) GetMaxAccessCount() int {
	return s.S
}

func (s *StorageHandler) LockStorage(storageID int) {
	log.Debug().Msgf("Aquiring lock for storage %d", storageID)
	s.storageMus[storageID].Lock()
	log.Debug().Msgf("Aquired lock for storage %d", storageID)
}

func (s *StorageHandler) UnlockStorage(storageID int) {
	log.Debug().Msgf("Releasing lock for storage %d", storageID)
	s.storageMus[storageID].Unlock()
	log.Debug().Msgf("Released lock for storage %d", storageID)
}

func (s *StorageHandler) InitDatabase() error {
	log.Debug().Msgf("Initializing the redis database")
	for _, client := range s.storages {
		// Do not reinitialize the database if it is already initialized
		dbsize, err := client.DBSize(context.Background()).Result()
		if err != nil {
			return err
		}
		if dbsize == (int64((math.Pow(float64(s.shift+1), float64(s.treeHeight))))-1)*2 {
			continue
		}
		err = client.FlushAll(context.Background()).Err()
		if err != nil {
			return err
		}
		err = s.databaseInit(client)
		if err != nil {
			return err
		}
	}
	return nil
}

type BlockOffsetStatus struct {
	Offset     int
	IsReal     bool
	BlockFound string
}

// selectDummyBlock returns a still-valid dummy slot for a bucket. A read
// invalidates its selected slot, so a bucket must not assume that dummy1 is
// always available before the next early reshuffle.
func (s *StorageHandler) selectDummyBlock(bucketID int, blockMap map[string]int) (string, int, error) {
	for dummyNumber := 1; dummyNumber <= s.Z+s.S; dummyNumber++ {
		dummyID := "dummy" + strconv.Itoa(dummyNumber)
		if pos, exists := blockMap[dummyID]; exists {
			return dummyID, pos, nil
		}
	}
	return "", 0, fmt.Errorf("bucket %d has no valid dummy block", bucketID)
}

func (s *StorageHandler) BatchGetBlockOffset(bucketIDs []int, storageID int, blocks []string) (blockoffsetStatuses map[int]BlockOffsetStatus, err error) {
	allBlockMap, err := s.BatchGetAllMetaData(bucketIDs, storageID)
	if err != nil {
		log.Debug().Msgf("Error getting meta data")
		return nil, err
	}
	blockoffsetStatuses = make(map[int]BlockOffsetStatus)
	for _, bucketID := range bucketIDs {
		blockoffsetStatuses[bucketID] = BlockOffsetStatus{
			Offset:     -1,
			IsReal:     false,
			BlockFound: "",
		}
		blockMap := allBlockMap[bucketID]
		for _, block := range blocks {
			pos, exist := blockMap[block]
			if exist {
				log.Debug().Msgf("Found block %s in bucket %d", block, bucketID)
				blockoffsetStatuses[bucketID] = BlockOffsetStatus{
					Offset:     pos,
					IsReal:     true,
					BlockFound: block,
				}
			}
		}
		if blockoffsetStatuses[bucketID].Offset == -1 {
			log.Debug().Msgf("Did not find any block in bucket %d, returning dummy block", bucketID)
			dummyID, pos, dummyErr := s.selectDummyBlock(bucketID, blockMap)
			if dummyErr != nil {
				return nil, dummyErr
			}
			blockoffsetStatuses[bucketID] = BlockOffsetStatus{
				Offset:     pos,
				IsReal:     false,
				BlockFound: dummyID,
			}
		}
	}
	return blockoffsetStatuses, nil
}

// It returns the number of times a bucket was accessed for multiple buckets.
// This is helpful to know when to do an early reshuffle.
func (s *StorageHandler) BatchGetAccessCount(bucketIDs []int, storageID int) (counts map[int]int, err error) {
	ctx := context.Background()
	pipe := s.storages[storageID].Pipeline()
	resultsMap := make(map[int]*redis.StringCmd)
	counts = make(map[int]int)
	// Iterate over each bucketID
	for _, bucketID := range bucketIDs {
		// Issue HGET command for the access count of the current bucketID within the pipeline
		cmd := pipe.HGet(ctx, strconv.Itoa(-1*bucketID), "accessCount")
		resultsMap[bucketID] = cmd
	}

	// Execute the pipeline for all bucketIDs
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	// Process the results for each bucketID
	for bucketID, cmd := range resultsMap {
		accessCountS, err := cmd.Result()
		if err != nil {
			if err != redis.Nil {
				return nil, err
			}
			counts[bucketID] = 0
			log.Debug().Msgf("Access count for bucket %d and storage %d is %d", bucketID, storageID, 0)
		} else {
			accessCount, err := strconv.Atoi(accessCountS)
			if err != nil {
				return nil, err
			}
			counts[bucketID] = accessCount
			log.Debug().Msgf("Access count for bucket %d and storage %d is %d", bucketID, storageID, accessCount)
		}
	}

	return counts, nil
}

// It reads multiple buckets from a single storage shard.
func (s *StorageHandler) BatchReadBucket(bucketIDs []int, storageID int) (blocks map[int]map[string]string, err error) {
	metadataMap, err := s.BatchGetAllMetaData(bucketIDs, storageID)
	if err != nil {
		return nil, err
	}
	results := make(map[int]map[string]*redis.StringCmd)
	pipe := s.storages[storageID].Pipeline()
	ctx := context.Background()
	for bucketID, metadata := range metadataMap {
		i := 0
		results[bucketID] = make(map[string]*redis.StringCmd)
		for key, pos := range metadata {
			if !strings.HasPrefix(key, "dummy") {
				results[bucketID][key] = pipe.HGet(ctx, strconv.Itoa(bucketID), strconv.Itoa(pos))
				i++
			}
		}
		for ; i < s.Z; i++ {
			dummyID, pos, dummyErr := s.selectDummyBlock(bucketID, metadata)
			if dummyErr != nil {
				return nil, dummyErr
			}
			delete(metadata, dummyID)
			// We should do this data acess for not leaking access pattern
			pipe.HGet(ctx, strconv.Itoa(bucketID), strconv.Itoa(pos))
		}
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	blocks = make(map[int]map[string]string)
	for bucketID, result := range results {
		blocks[bucketID] = make(map[string]string)
		for key, cmd := range result {
			value, err := cmd.Result()
			if err != nil {
				return nil, err
			}
			value, err = Decrypt(value, s.key)
			if err != nil {
				return nil, err
			}
			blocks[bucketID][key] = value
		}
	}
	return blocks, nil
}

// creates a map of bucketIDs to blocks that can go in that bucket
func (s *StorageHandler) getBucketToValidBlocksMap(shardNodeBlocks map[string]BlockInfo) map[int][]string {
	bucketToValidBlocksMap := make(map[int][]string)
	for key, blockInfo := range shardNodeBlocks {
		leafID := int(math.Pow(2, float64(s.treeHeight-1)) + float64(blockInfo.Path) - 1)
		for bucketId := leafID; bucketId > 0; bucketId = bucketId >> s.shift {
			bucketToValidBlocksMap[bucketId] = append(bucketToValidBlocksMap[bucketId], key)
		}
	}
	return bucketToValidBlocksMap
}

// It writes blocks to multiple buckets in a single storage shard.
func (s *StorageHandler) BatchWriteBucket(storageID int, readBucketBlocksList map[int]map[string]string, shardNodeBlocks map[string]BlockInfo) (writtenBlocks map[string]string, err error) {
	pipe := s.storages[storageID].Pipeline()
	ctx := context.Background()
	dataResults := make(map[int]*redis.BoolCmd)
	metadataResults := make(map[int]*redis.BoolCmd)
	writtenBlocks = make(map[string]string)

	log.Debug().Msgf("buckets from readBucketBlocksList: %v", readBucketBlocksList)
	log.Debug().Msgf("shardNodeBlocks: %v", shardNodeBlocks)

	bucketToValidBlocksMap := s.getBucketToValidBlocksMap(shardNodeBlocks)

	for bucketID, readBucketBlocks := range readBucketBlocksList {
		values := make([]string, s.Z+s.S)
		metadatas := make([]string, s.Z+s.S)
		realIndex := make([]int, s.Z+s.S)
		for k := 0; k < s.Z+s.S; k++ {
			// Generate a random number between 0 and 9
			realIndex[k] = k
		}
		shuffleArray(realIndex)
		i := 0
		for key, value := range readBucketBlocks {
			if strings.HasPrefix(key, "dummy") {
				continue
			}
			if i < s.Z {
				writtenBlocks[key] = value
				values[realIndex[i]], err = Encrypt(value, s.key)
				if err != nil {
					return nil, err
				}
				metadatas[i] = strconv.Itoa(realIndex[i]) + key
				i++
				// pos_map is updated in server?
			} else {
				break
			}
		}
		for _, key := range bucketToValidBlocksMap[bucketID] {
			if strings.HasPrefix(key, "dummy") {
				continue
			}
			if i < s.Z {
				writtenBlocks[key] = shardNodeBlocks[key].Value
				values[realIndex[i]], err = Encrypt(shardNodeBlocks[key].Value, s.key)
				if err != nil {
					return nil, err
				}
				metadatas[i] = strconv.Itoa(realIndex[i]) + key
				i++
			} else {
				break
			}
		}
		dummyCount := 1
		for ; i < s.Z+s.S; i++ {
			dummyID := "dummy" + strconv.Itoa(dummyCount)
			dummyString := "b" + strconv.Itoa(bucketID) + "d" + strconv.Itoa(i)
			dummyString, err = Encrypt(dummyString, s.key)
			if err != nil {
				log.Error().Msgf("Error encrypting data")
				return nil, err
			}
			// push dummy to array
			values[realIndex[i]] = dummyString
			// push meta data of dummies to array
			metadatas[i] = strconv.Itoa(realIndex[i]) + dummyID
			dummyCount++
		}
		dataResults[i], metadataResults[i] = s.BatchPushDataAndMetadata(bucketID, values, metadatas, pipe)
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	for _, dataCmd := range dataResults {
		_, err := dataCmd.Result()
		if err != nil {
			if err != redis.Nil {
				return nil, err
			}
			return nil, err
		}
	}
	for _, metadataCmd := range dataResults {
		_, err := metadataCmd.Result()
		if err != nil {
			if err != redis.Nil {
				return nil, err
			}
			return nil, err
		}
	}
	return writtenBlocks, nil
}

// It reads multiple blocks from multiple buckets and returns the values.
func (s *StorageHandler) BatchReadBlock(bucketOffsets map[int]int, storageID int) (values map[int]string, err error) {
	ctx := context.Background()
	pipe := s.storages[storageID].Pipeline()
	resultsMap := make(map[int]*redis.StringCmd)
	for bucketID, offset := range bucketOffsets {
		// Issue HGET commands for the value stored in the current bucketID
		cmd := pipe.HGet(ctx, strconv.Itoa(bucketID), strconv.Itoa(offset))

		// Store the map of results for the current bucketID in the resultsMap
		resultsMap[bucketID] = cmd
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Debug().Msgf("error executing batch read block pipe: %v", err)
		return nil, err
	}
	values = make(map[int]string)
	for bucketID, cmd := range resultsMap {
		block, err := cmd.Result()
		if err != nil && err != redis.Nil {
			return nil, err
		}

		value, err := Decrypt(block, s.key)
		if err != nil {
			return nil, err
		}
		values[bucketID] = value
	}
	metadataFields, err := s.batchGetMetadataFieldsForOffsets(bucketOffsets, storageID)
	if err != nil {
		return nil, err
	}
	for bucketID := range bucketOffsets {
		// Metadata is stored in logical record order; its value encodes the
		// shuffled physical data offset. Invalidate the logical metadata field,
		// not the physical offset itself.
		pipe.HSet(ctx, strconv.Itoa(-1*bucketID), metadataFields[bucketID], "__null__")
		pipe.HIncrBy(ctx, strconv.Itoa(-1*bucketID), "accessCount", 1)
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Debug().Msgf("error executing batch read block pipe: %v", err)
		return nil, err
	}
	return values, nil
}

// GetBucketsInPaths return all the bucket ids for the passed paths.
func (s *StorageHandler) GetBucketsInPaths(paths []int) (bucketIDs []int, err error) {
	log.Debug().Msgf("Getting buckets in paths %v", paths)
	buckets := make(IntSet)
	for i := 0; i < len(paths); i++ {
		leafID := int(math.Pow(2, float64(s.treeHeight-1)) + float64(paths[i]) - 1)
		for bucketId := leafID; bucketId > 0; bucketId = bucketId >> s.shift {
			if buckets.Contains(bucketId) {
				break
			} else {
				buckets.Add(bucketId)
			}
		}
	}
	bucketIDs = make([]int, len(buckets))
	i := 0
	for key := range buckets {
		bucketIDs[i] = key
		i++
	}
	return bucketIDs, nil
}

// It returns valid randomly chosen path and storageID.
func GetRandomPathAndStorageID(treeHeight int, storageCount int) (path int, storageID int) {
	log.Debug().Msgf("Getting random path and storage id")
	paths := int(math.Pow(2, float64(treeHeight-1)))
	randomPath := rand.Intn(paths) + 1
	randomStorage := rand.Intn(storageCount)
	return randomPath, randomStorage
}

func (s *StorageHandler) GetRandomStorageID() int {
	log.Debug().Msgf("Getting random storage id")
	index := rand.Intn(len(s.storages))
	for storageID := range s.storages {
		if index == 0 {
			return storageID
		}
		index--
	}
	return -1
}

func (s *StorageHandler) GetMultipleReverseLexicographicPaths(evictionCount int, count int) (paths []int) {
	log.Debug().Msgf("Getting multiple reverse lexicographic paths")
	paths = make([]int, count)
	for i := 0; i < count; i++ {
		paths[i] = GetNextReverseLexicographicPath(evictionCount, s.treeHeight)
		evictionCount++
	}
	return paths
}

// evictionCount starts from zero and goes forward
func GetNextReverseLexicographicPath(evictionCount int, treeHeight int) (nextPath int) {
	evictionCount = evictionCount % int(math.Pow(2, float64(treeHeight-1)))
	log.Debug().Msgf("Getting next reverse lexicographic path")
	reverseBinary := utils.BinaryReverse(evictionCount, treeHeight-1)
	return reverseBinary + 1
}
