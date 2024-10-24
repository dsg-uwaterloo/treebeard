package config

import (
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type RouterEndpoint struct {
	IP   string `yaml:"exposed_ip"`
	Port int
	ID   int
}

type ShardNodeEndpoint struct {
	IP        string `yaml:"exposed_ip"`
	Port      int
	ID        int
	ReplicaID int
}

type OramNodeEndpoint struct {
	IP        string `yaml:"exposed_ip"`
	Port      int
	ID        int
	ReplicaID int
}

type RedisEndpoint struct {
	IP         string `yaml:"exposed_ip"`
	Port       int
	ID         int
	ORAMNodeID int `yaml:"oramnode_id"`
}

type RouterConfig struct {
	Endpoints []RouterEndpoint
}

type ShardNodeConfig struct {
	Endpoints []ShardNodeEndpoint
}

type OramNodeConfig struct {
	Endpoints []OramNodeEndpoint
}

type RedisConfig struct {
	Endpoints []RedisEndpoint
}

type Parameters struct {
	MaxBlocksToSend   int     `yaml:"max-blocks-to-send"`
	EvictionRate      int     `yaml:"eviction-rate"`
	EvictPathCount    int     `yaml:"evict-path-count"`
	BatchTimout       float64 `yaml:"batch-timeout"`
	EpochTime         float64 `yaml:"epoch-time"`
	Trace             bool    `yaml:"trace"`
	Z                 int     `yaml:"Z"`
	Shift             int     `yaml:"shift"`
	TreeHeight        int     `yaml:"tree-height"`
	RedisPipelineSize int     `yaml:"redis-pipeline-size"`
	MaxRequests       int     `yaml:"max-requests"`
	BlockSize         int     `yaml:"block-size"`
	Log               bool    `yaml:"log"`
	Profile           bool    `yaml:"profile"`
}

func (o Parameters) String() string {
	output := ""
	output += "MaxBlocksToSend: " + strconv.Itoa(o.MaxBlocksToSend) + "\n"
	output += "EvictionRate: " + strconv.Itoa(o.EvictionRate) + "\n"
	output += "EvictPathCount: " + strconv.Itoa(o.EvictPathCount) + "\n"
	output += "BatchTimout: " + strconv.FormatFloat(o.BatchTimout, 'f', -1, 64) + "\n"
	output += "EpochTime: " + strconv.FormatFloat(o.EpochTime, 'f', -1, 64) + "\n"
	output += "Z: " + strconv.Itoa(o.Z) + "\n"
	output += "Shift: " + strconv.Itoa(o.Shift) + "\n"
	output += "TreeHeight: " + strconv.Itoa(o.TreeHeight) + "\n"
	output += "RedisPipelineSize: " + strconv.Itoa(o.RedisPipelineSize) + "\n"
	output += "MaxRequests: " + strconv.Itoa(o.MaxRequests) + "\n"
	output += "BlockSize: " + strconv.Itoa(o.BlockSize)
	return output
}

func ReadRouterEndpoints(path string) ([]RouterEndpoint, error) {
	log.Debug().Msgf("Reading router endpoints from the yaml file")
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config RouterConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	return config.Endpoints, nil
}

func ReadShardNodeEndpoints(path string) ([]ShardNodeEndpoint, error) {
	log.Debug().Msgf("Reading shard node endpoints from the yaml file")
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ShardNodeConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	return config.Endpoints, nil
}

func ReadOramNodeEndpoints(path string) ([]OramNodeEndpoint, error) {
	log.Debug().Msgf("Reading oram node endpoints from the yaml file")
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config OramNodeConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	return config.Endpoints, nil
}

func ReadRedisEndpoints(path string) ([]RedisEndpoint, error) {
	log.Debug().Msgf("Reading redis endpoints from the yaml file")
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config RedisConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	return config.Endpoints, nil
}

func ReadParameters(path string) (Parameters, error) {
	log.Debug().Msgf("Reading parameters from the yaml file")
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return Parameters{}, err
	}

	var params Parameters
	err = yaml.Unmarshal(yamlFile, &params)
	if err != nil {
		return Parameters{}, err
	}
	return params, nil
}
