package storage

import (
	"bufio"
	"context"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

func (s *StorageHandler) getClient(storageID int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     s.host,
		Password: "",
		DB:       s.db[storageID],
	})
}

func (s *StorageHandler) CloseClient() (err error) {
	for i := 0; i < numDB; i++ {
		client := s.getClient(i)
		err = client.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func shuffleArray(arr []int) {
	for i := len(arr) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		arr[i], arr[j] = arr[j], arr[i]
	}
}

func (s *StorageHandler) databaseInit(filepath string, storageID int) (position_map map[string]int, err error) {
	file, err := os.Open(filepath)
	if err != nil {
		log.Error().Msgf("Error opening file: %v", err)
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	// i keeps track of whether we should load dummies; when i reach Z, add dummies
	i := 0
	bucketCount := 1
	// userID of dummies
	dummyCount := 0
	// initialize map
	position_map = make(map[string]int)
	// initialize value array
	values := make([]string, Z+S)
	metadatas := make([]string, Z+S)
	realIndex := make([]int, Z+S)
	for k := 0; k < Z+S; k++ {
		// Generate a random number between 0 and 9
		realIndex[k] = k
	}
	shuffleArray(realIndex)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		userID := parts[1]
		value := parts[2]
		value, err = Encrypt(value, s.key[storageID])
		if err != nil {
			log.Error().Msgf("Error encrypting data")
			return nil, err
		}
		// add encrypted value to array
		values[realIndex[i]] = value
		// push meta data to array
		metadatas[i] = strconv.Itoa(realIndex[i]) + userID
		// put userId into position map
		position_map[userID] = bucketCount
		i++
		// push dummy values if the current bucket is full
		if i == Z {
			for ; i < Z+S; i++ {
				dummyID := "dummy" + strconv.Itoa(dummyCount)
				dummyString := "b" + strconv.Itoa(bucketCount) + "d" + strconv.Itoa(i)
				dummyString, err = Encrypt(dummyString, s.key[storageID])
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
			// push content of value array and meta data array
			err = s.Push(bucketCount, values, storageID)
			if err != nil {
				log.Error().Msgf("Error pushing values to db: %v", err)
				return nil, err
			}
			err = s.PushMetadata(bucketCount, metadatas, storageID)
			if err != nil {
				log.Error().Msgf("Error pushing metadatas to db: %v", err)
				return nil, err
			}
			i = 0
			bucketCount++
			// generate new random index for next bucket
			shuffleArray(realIndex)
		}
	}
	// fill last bucket with dummy
	if i != 0 {
		for ; i < Z+S; i++ {
			dummyID := "dummy" + strconv.Itoa(dummyCount)
			dummyString := "b" + strconv.Itoa(bucketCount) + "d" + strconv.Itoa(i)
			dummyString, err = Encrypt(dummyString, s.key[storageID])
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
		// push content of value array and meta data array
		err = s.Push(bucketCount, values, storageID)
		if err != nil {
			log.Error().Msgf("Error pushing values to db: %v", err)
			return nil, err
		}
		err = s.PushMetadata(bucketCount, metadatas, storageID)
		if err != nil {
			log.Error().Msgf("Error pushing metadatas to db: %v", err)
			return nil, err
		}
	}
	return position_map, nil
}

func (s *StorageHandler) DatabaseClear() (err error) {
	for i := 0; i < numDB; i++ {
		client := s.getClient(i)
		ctx := context.Background()
		err = client.FlushAll(ctx).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *StorageHandler) Push(bucketId int, value []string, storageID int) (err error) {
	client := s.getClient(storageID)
	ctx := context.Background()
	kvpMap := make(map[string]interface{})
	for i := 0; i < len(value); i++ {
		kvpMap[strconv.Itoa(i)] = value[i]
	}
	err = client.HMSet(ctx, strconv.Itoa(bucketId), kvpMap).Err()
	if err != nil {
		return err
	}
	return nil
}

func (s *StorageHandler) PushMetadata(bucketId int, value []string, storageID int) (err error) {
	client := s.getClient(storageID)
	ctx := context.Background()
	kvpMap := make(map[string]interface{})
	for i := 0; i < len(value); i++ {
		kvpMap[strconv.Itoa(i)] = value[i]
	}
	kvpMap["nextDummy"] = Z
	kvpMap["accessCount"] = 0
	err = client.HMSet(ctx, strconv.Itoa(-1*bucketId), kvpMap).Err()
	if err != nil {
		return err
	}
	return nil
}

// return pos + key as one string stored in metadata at bit
func (s *StorageHandler) GetMetadata(bucketId int, bit string, storageID int) (pos int, key string, err error) {
	client := s.getClient(storageID)
	ctx := context.Background()
	block, err := client.HGet(ctx, strconv.Itoa(-1*bucketId), bit).Result()
	if err != nil {
		return -1, "", err
	}
	// parse block into pos + key
	index := 0
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
