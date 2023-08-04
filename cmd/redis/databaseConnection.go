package main

import (
	"context"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

type client struct {
	host string
	db   int
	key  []byte
}

type data struct {
	name  string
	value int
}

const (
    Z = 4;
    S = 6;
)


func NewClient(host string, db int, key []byte) *client {
	return &client{
		host: host,
		db:   db,
		key:  key,
	}
}

func (info *client) CloseClient() (err error) {
	client := info.getClient()
	err = client.Close()
	if err != nil {
		return err
	}
	return nil
}

func (info *client) getClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     info.host,
		Password: "",
		DB:       info.db,
	})
}

func (info *client) Set(key string, value string) (err error) {
	client := info.getClient()
	ctx := context.Background()
	err = client.Set(ctx, key, value, 0).Err()
	if err != nil {
	 return err
	}
	return nil
   }
   
   func (info *client) Get(key string) (value string, err error) {
	client := info.getClient()
	ctx := context.Background()
	block, err := client.Get(ctx, key).Result()
	if err != nil {
	 return "", err
	}
	return block, nil
   }

func (info *client) databaseInit(filepath string) (position_map map[string]int, err error) {
	file, err := os.Open(filepath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	client := info.getClient()
	ctx := context.Background()
	
	// i keeps track of whether we should load dummies; when i reach Z, add dummies
	i := 0
	bucketCount := 1
	// userID of dummies
	dummyCount := 0
	// initialize map
	position_map = make(map[string]int)
	fmt.Println("init done")
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		userID := parts[1]
		value := parts[2]
		value, err = Encrypt(value, info.key)
		if err != nil {
			fmt.Println("Error encrypting data")
			return nil, err
		}
		// push data to db
		err = client.Set(ctx, userID, value, 0).Err()
		if err != nil {
			fmt.Println("Error pushing to db:", err)
			return nil, err
		}
		// push meta data to db
		err = client.RPush(ctx, strconv.Itoa(bucketCount), userID).Err()
		if err != nil {
			fmt.Println("Error pushing meta data to db:", err)
			return nil, err
		}
		// put userId into position map
		position_map[userID] = bucketCount
		i++
		// push dummy values if the current bucket is full
		if i == Z {
			for ; i < Z + S; i++ {
				dummyID := "dummy" + strconv.Itoa(dummyCount)
				dummyString := "b" + strconv.Itoa(bucketCount) + "d" + strconv.Itoa(i)
				dummyString, err = Encrypt(dummyString, info.key)
				if err != nil {
					fmt.Println("Error encrypting data")
					return nil, err
				}
				// push dummy to db
				err = client.Set(ctx, dummyID, dummyString, 0).Err()
				
				if err != nil {
					fmt.Println("Error pushing dummy to db:", err)
					return nil, err
				}
				
				// push meta data of dummies to db
				err = client.RPush(ctx, strconv.Itoa(bucketCount), dummyID).Err()
				if err != nil {
					fmt.Println("Error pushing dummy meta data to db:", err)
					return nil, err
				}
				dummyCount++
			}
			err = client.RPush(ctx, strconv.Itoa(bucketCount), Z).Err()
			if err != nil {
				fmt.Println("Error pushing dummy meta data start to db:", err)
				return nil, err
			}
			i = 0
			bucketCount++
		}
	}
	return position_map, nil
}

func (info *client) DatabaseClear() (err error) {
	client := info.getClient()
	ctx := context.Background()
	err = client.FlushAll(ctx).Err()
	if err != nil {
		return err
	}
	return nil
}

func (info *client) GetMetadata(pathId int, bit int) (value string, err error) {
	client := info.getClient()
	ctx := context.Background()
	value, err = client.LIndex(ctx, strconv.Itoa(pathId), int64(bit)).Result()
	if err != nil {
		return "", err
	}
	return value, nil
}

func (info *client) PushContent(bucketId int, value []string) (err error) {
	client := info.getClient()
	ctx := context.Background()
	for i := 0; i < len(value); i++ {
		err = client.RPush(ctx, strconv.Itoa(bucketId), value[i]).Err()
		if err != nil {
			return err
		}
	}
	return nil
}


func main() {
	key := []byte("passphrasewhichneedstobe32bytes!")
	info := NewClient("localhost:6379", 1, key)
	path := "./data.txt"

	posmap, err := info.databaseInit(path)
	if err != nil {
		fmt.Println("error initializing database")
	}
	pathId := posmap["user5241976879437760820"]
	fmt.Println(pathId)
	
	vmap, err := info.readPath(pathId, "user5241976879437760820")
	value, err := info.Get("user5241976879437760820")
	fmt.Println(value)
	for key, value := range vmap {
		fmt.Printf("Key: %s, Value: %s \n", key, value)
	}
	info.DatabaseClear()
	info.CloseClient()
}
