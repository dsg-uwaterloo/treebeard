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
	shift      int
	storages   map[int]*redis.Client // map of storage id to redis client
	storageMus map[int]*sync.Mutex   // map of storage id to mutex
	key        []byte
}

type BlockInfo struct {
	Value string
	Path  int
}

func NewStorageHandler(treeHeight int, Z int, shift int, redisEndpoints []config.RedisEndpoint) *StorageHandler { // map of storage id to storage info
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
		shift:      shift,
		storages:   storages,
		storageMus: storageMus,
		key:        []byte("passphrasewhichneedstobe32bytes!"),
	}
	return s
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
		// dbsize, err := client.DBSize(context.Background()).Result()
		// if err != nil {
		// 	return err
		// }
		// if dbsize == (int64((math.Pow(float64(s.shift+1), float64(s.treeHeight)))) - 1) {
		// 	continue
		// }
		err := client.FlushAll(context.Background()).Err()
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

// It reads multiple buckets from a single storage shard.
func (s *StorageHandler) BatchReadBucket(bucketIDs []int, storageID int) (blocks map[int]map[string]string, err error) {
	// metadataMap, err := s.BatchGetAllMetaData(bucketIDs, storageID)
	results := make(map[int][]*redis.StringCmd)
	pipe := s.storages[storageID].Pipeline()
	ctx := context.Background()
	for _, bucketID := range bucketIDs {
		results[bucketID] = make([]*redis.StringCmd, s.Z)
		for i := 0; i < s.Z; i++ {
			results[bucketID][i] = pipe.HGet(ctx, strconv.Itoa(bucketID), strconv.Itoa(i))
		}
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("error executing batch read bucket pipe: %v", err)
	}
	blocks = make(map[int]map[string]string)
	for bucketID, result := range results {
		blocks[bucketID] = make(map[string]string)
		for _, cmd := range result {
			value, err := cmd.Result()
			if err != nil {
				return nil, err
			}
			value, err = Decrypt(value, s.key)
			if err != nil {
				return nil, err
			}
			key, val, err := deserializeKeyValue(value)
			if err != nil {
				return nil, err
			}
			if key != "dummy" {
				blocks[bucketID][key] = val
			}
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
	writtenBlocks = make(map[string]string)

	log.Debug().Msgf("buckets from readBucketBlocksList: %v", readBucketBlocksList)
	log.Debug().Msgf("shardNodeBlocks: %v", shardNodeBlocks)

	bucketToValidBlocksMap := s.getBucketToValidBlocksMap(shardNodeBlocks)

	for bucketID, readBucketBlocks := range readBucketBlocksList {
		values := make([]string, s.Z)
		i := 0
		for key, value := range readBucketBlocks {
			if strings.HasPrefix(key, "dummy") {
				continue
			}
			if i < s.Z {
				writtenBlocks[key] = value
				serialized := serializeKeyValue(key, value)
				values[i], err = Encrypt(serialized, s.key)
				if err != nil {
					return nil, err
				}
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
				serialized := serializeKeyValue(key, shardNodeBlocks[key].Value)
				values[i], err = Encrypt(serialized, s.key)
				if err != nil {
					return nil, err
				}
				i++
			} else {
				break
			}
		}
		for ; i < s.Z; i++ {
			dummyString := "dummy"
			serialized := serializeKeyValue(dummyString, dummyString)
			dummyString, err = Encrypt(serialized, s.key)
			if err != nil {
				log.Error().Msgf("Error encrypting data")
				return nil, err
			}
			values[i] = dummyString
		}
		dataResults[i] = s.BatchPushData(bucketID, values, pipe)
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
		_, val, err := deserializeKeyValue(value)
		if err != nil {
			return nil, err
		}
		values[bucketID] = val
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
