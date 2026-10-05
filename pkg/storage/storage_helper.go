package storage

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

func getClient(ip string, port int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     ip + ":" + strconv.Itoa(port),
		Password: "",
		DB:       0,
	})
}

func closeClient(client *redis.Client) (err error) {
	err = client.Close()
	if err != nil {
		return err
	}
	return nil
}

func shuffleArray(arr []int) {
	for i := len(arr) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		arr[i], arr[j] = arr[j], arr[i]
	}
}

func (s *StorageHandler) databaseInit(redisClient *redis.Client) (err error) {
	pipe := redisClient.Pipeline()
	pipeCount := 0
	for bucketID := 1; bucketID < int(math.Pow(2, float64(s.treeHeight))); bucketID++ {
		values := make([]string, s.Z+s.S)
		metadatas := make([]string, s.Z+s.S)
		realIndex := make([]int, s.Z+s.S)
		for k := 0; k < s.Z+s.S; k++ {
			// Generate a random number between 0 and 9
			realIndex[k] = k
		}
		// userID of dummies
		dummyCount := 1
		// initialize value array
		shuffleArray(realIndex)
		for i := 0; i < s.Z+s.S; i++ {
			dummyID := "dummy" + strconv.Itoa(dummyCount)
			dummyString := "b" + strconv.Itoa(bucketID) + "d" + strconv.Itoa(realIndex[i])
			dummyString, err = Encrypt(dummyString, s.key)
			if err != nil {
				log.Error().Msgf("Error encrypting data")
				return err
			}
			// push dummy to array
			values[realIndex[i]] = dummyString
			// push meta data of dummies to array
			metadatas[i] = strconv.Itoa(realIndex[i]) + dummyID
			dummyCount++
		}
		// push content of value array and meta data array
		s.BatchPushDataAndMetadata(bucketID, values, metadatas, pipe)
		pipeCount++
		if pipeCount == 10000 || bucketID == int(math.Pow(2, float64(s.treeHeight)))-1 {
			_, err = pipe.Exec(context.Background())
			if err != nil {
				log.Error().Msgf("Error pushing values to db: %v", err)
				return err
			}
			pipeCount = 0
			pipe = redisClient.Pipeline()
		}
	}
	return nil
}

func (s *StorageHandler) BatchPushDataAndMetadata(bucketId int, valueData []string, valueMetadata []string, pipe redis.Pipeliner) (dataCmd *redis.BoolCmd, metadataCmd *redis.BoolCmd) {
	ctx := context.Background()
	kvpMapData := make(map[string]interface{})
	for i := 0; i < len(valueData); i++ {
		kvpMapData[strconv.Itoa(i)] = valueData[i]
	}

	kvpMapMetadata := make(map[string]interface{})
	for i := 0; i < len(valueMetadata); i++ {
		kvpMapMetadata[strconv.Itoa(i)] = valueMetadata[i]
	}
	kvpMapMetadata["accessCount"] = 0

	dataCmd = pipe.HMSet(ctx, strconv.Itoa(bucketId), kvpMapData)
	metadataCmd = pipe.HMSet(ctx, strconv.Itoa(-1*bucketId), kvpMapMetadata)

	return dataCmd, metadataCmd
}

func parseMetadataBlock(block string) (pos int, key string, err error) {
	if block == "__null__" {
		return -1, "", nil
	}
	index := 0
	//log.Debug().Msgf("block: %s", block)
	for j, char := range block {
		if char < '0' || char > '9' {
			index = j
			break
		}
	}
	pos, err = strconv.Atoi(block[:index])
	if err != nil {
		return -1, "", err
	}
	key = block[index:]
	return pos, key, nil
}

func parseMetadataBlocks(bucketMetadata map[int]*redis.MapStringStringCmd) (blockOffsets map[int]map[string]int, err error) {
	allBlockOffsets := make(map[int]map[string]int)
	for bucketID, metadata := range bucketMetadata {
		result, err := metadata.Result()
		if err != nil {
			return nil, err
		}
		allBlockOffsets[bucketID] = make(map[string]int)
		for redisKey, block := range result {
			if redisKey == "accessCount" {
				continue
			}
			pos, blockKey, err := parseMetadataBlock(block)
			if err != nil {
				return nil, err
			}
			if pos == -1 {
				continue
			}
			allBlockOffsets[bucketID][blockKey] = pos
		}
	}
	return allBlockOffsets, nil
}

// metadataFieldForOffset returns the Redis hash field whose metadata value
// describes the requested physical data offset. Metadata records are stored in
// logical order while their values encode the shuffled physical position, so
// the metadata field is not necessarily equal to the data offset.
func metadataFieldForOffset(bucketID int, metadata map[string]string, offset int) (string, error) {
	matchingField := ""
	for field, block := range metadata {
		if field == "accessCount" || block == "__null__" {
			continue
		}
		pos, _, err := parseMetadataBlock(block)
		if err != nil {
			return "", err
		}
		if pos != offset {
			continue
		}
		if matchingField != "" {
			return "", fmt.Errorf("bucket %d has multiple metadata fields for physical offset %d", bucketID, offset)
		}
		matchingField = field
	}
	if matchingField == "" {
		return "", fmt.Errorf("bucket %d has no metadata field for physical offset %d", bucketID, offset)
	}
	return matchingField, nil
}

// batchGetMetadataFieldsForOffsets resolves physical data offsets back to the
// logical Redis metadata fields that must be invalidated after a read.
func (s *StorageHandler) batchGetMetadataFieldsForOffsets(bucketOffsets map[int]int, storageID int) (map[int]string, error) {
	ctx := context.Background()
	pipe := s.storages[storageID].Pipeline()
	results := make(map[int]*redis.MapStringStringCmd, len(bucketOffsets))
	for bucketID := range bucketOffsets {
		results[bucketID] = pipe.HGetAll(ctx, strconv.Itoa(-1*bucketID))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}

	fields := make(map[int]string, len(bucketOffsets))
	for bucketID, cmd := range results {
		metadata, err := cmd.Result()
		if err != nil {
			return nil, err
		}
		field, err := metadataFieldForOffset(bucketID, metadata, bucketOffsets[bucketID])
		if err != nil {
			return nil, err
		}
		fields[bucketID] = field
	}
	return fields, nil
}

// Returns a map of bucketID to a map of block to position. It returns all the valid real and dummy blocks in the bucket.
// The invalidated blocks are not returned.
func (s *StorageHandler) BatchGetAllMetaData(bucketIDs []int, storageID int) (map[int]map[string]int, error) {
	ctx := context.Background()
	// TODO: write a function to check for duplicate blocks here
	startTime := time.Now()
	pipe := s.storages[storageID].Pipeline()
	results := make(map[int]*redis.MapStringStringCmd)
	for _, bucketID := range bucketIDs {
		results[bucketID] = pipe.HGetAll(ctx, strconv.Itoa(-1*bucketID))
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	endTime := time.Now()
	log.Debug().Msgf("BatchGetAllMetaData took %v ms for %d buckets", endTime.Sub(startTime).Milliseconds(), len(bucketIDs))
	allBlockOffsets, err := parseMetadataBlocks(results)
	if err != nil {
		return nil, err
	}
	return allBlockOffsets, nil
}

type IntSet map[int]struct{}

func (s IntSet) Add(item int) {
	s[item] = struct{}{}
}

func (s IntSet) Remove(item int) {
	delete(s, item)
}

func (s IntSet) Contains(item int) bool {
	_, found := s[item]
	return found
}
