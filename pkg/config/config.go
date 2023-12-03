package config

import (
	"os"

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
	MaxBlocksToSend int  `yaml:"max-blocks-to-send"`
	EvictionRate    int  `yaml:"eviction-rate"`
	EvictPathCount  int  `yaml:"evict-path-count"`
	BatchTimout     int  `yaml:"batch-timeout"`
	EpochTime       int  `yaml:"epoch-time"`
	Trace           bool `yaml:"trace"`
	Z               int  `yaml:"Z"`
	S               int  `yaml:"S"`
	Shift           int  `yaml:"shift"`
	TreeHeight      int  `yaml:"tree-height"`
	MaxRequests     int  `yaml:"max-requests"`
	Log             bool `yaml:"log"`
	Profile         bool `yaml:"profile"`
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
