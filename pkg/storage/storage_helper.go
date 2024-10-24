package storage

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"

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

// SerializeKeyValue takes a key and value (both strings) and serializes them into a single string
func serializeKeyValue(key, value string) string {
	return key + ":" + value
}

// DeserializeKeyValue takes a serialized string and returns the key and value as two separate strings
func deserializeKeyValue(serialized string) (string, string, error) {
	parts := strings.SplitN(serialized, ":", 2)
	if len(parts) < 2 {
		log.Error().Msgf("invalid serialized format string was: %s", serialized)
		return "", "", fmt.Errorf("invalid serialized format")
	}
	return parts[0], parts[1], nil
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
		values := make([]string, s.Z)
		for i := 0; i < s.Z; i++ {
			dummyString := serializeKeyValue("dummy", "dummy")
			dummyString, err = Encrypt(dummyString, s.key)
			values[i] = dummyString
			if err != nil {
				log.Error().Msgf("Error encrypting data")
				return err
			}
		}
		// push content of value array
		s.BatchPushData(bucketID, values, pipe)
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

func (s *StorageHandler) BatchPushData(bucketId int, valueData []string, pipe redis.Pipeliner) (dataCmd *redis.BoolCmd) {
	ctx := context.Background()
	kvpMapData := make(map[string]interface{})
	for i := 0; i < len(valueData); i++ {
		kvpMapData[strconv.Itoa(i)] = valueData[i]
	}

	dataCmd = pipe.HMSet(ctx, strconv.Itoa(bucketId), kvpMapData)

	return dataCmd
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
