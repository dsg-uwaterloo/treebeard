package oramnode

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/rs/zerolog/log"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/otel"
)

type beginEvictionData struct {
	toEvictCount int
	storageID    int
}

type beginReadPathData struct {
	paths     []int
	storageID int
}

type oramNodeFSM struct {
	unfinishedEviction   *beginEvictionData // unfinished eviction
	unfinishedEvictionMu sync.Mutex
	unfinishedReadPath   *beginReadPathData // unfinished read path
	unfinishedReadPathMu sync.Mutex
	evictionPathMap      map[int][]int // map of storage id to paths
}

func (fsm *oramNodeFSM) String() string {
	out := fmt.Sprintln("oramNodeFSM")
	out = out + fmt.Sprintf("unfinishedEviction: %v\n", fsm.unfinishedEviction)
	return out
}

func newOramNodeFSM() *oramNodeFSM {
	return &oramNodeFSM{
		evictionPathMap: make(map[int][]int),
	}
}

func (fsm *oramNodeFSM) handleBeginEvictionCommand(toEvictCount int, storageID int) {
	log.Debug().Msgf("Aquiring lock for oramNodeFSM in handleBeginEvictionCommand")
	fsm.unfinishedEvictionMu.Lock()
	log.Debug().Msgf("Aquired lock for oramNodeFSM in handleBeginEvictionCommand")
	defer func() {
		log.Debug().Msgf("Releasing lock for oramNodeFSM in handleBeginEvictionCommand")
		fsm.unfinishedEvictionMu.Unlock()
		log.Debug().Msgf("Released lock for oramNodeFSM in handleBeginEvictionCommand")
	}()

	fsm.unfinishedEviction = &beginEvictionData{toEvictCount, storageID}
}

func (fsm *oramNodeFSM) handleEndEvictionCommand(doneEvictCount int, storageID int) {
	log.Debug().Msgf("Aquiring lock for oramNodeFSM in handleEndEvictionCommand")
	fsm.unfinishedEvictionMu.Lock()
	log.Debug().Msgf("Aquired lock for oramNodeFSM in handleEndEvictionCommand")
	defer func() {
		log.Debug().Msgf("Releasing lock for oramNodeFSM in handleEndEvictionCommand")
		fsm.unfinishedEvictionMu.Unlock()
		log.Debug().Msgf("Released lock for oramNodeFSM in handleEndEvictionCommand")
	}()
	fsm.unfinishedEviction = nil
	fsm.evictionPathMap[storageID] = fsm.evictionPathMap[storageID][doneEvictCount:]
}

func (fsm *oramNodeFSM) handleBeginReadPathCommand(paths []int, storageID int) {
	tracer := otel.Tracer("")
	_, span := tracer.Start(context.Background(), "begin read path replication inside")
	log.Debug().Msgf("Aquiring lock for oramNodeFSM in handleBeginReadPathCommand")
	fsm.unfinishedReadPathMu.Lock()
	log.Debug().Msgf("Aquired lock for oramNodeFSM in handleBeginReadPathCommand")
	defer func() {
		log.Debug().Msgf("Releasing lock for oramNodeFSM in handleBeginReadPathCommand")
		fsm.unfinishedReadPathMu.Unlock()
		log.Debug().Msgf("Released lock for oramNodeFSM in handleBeginReadPathCommand")
	}()

	fsm.unfinishedReadPath = &beginReadPathData{paths, storageID}
	fsm.evictionPathMap[storageID] = append(fsm.evictionPathMap[storageID], paths...)
	span.End()
}

func (fsm *oramNodeFSM) handleEndReadPathCommand() {
	log.Debug().Msgf("Aquiring lock for oramNodeFSM in handleEndReadPathCommand")
	fsm.unfinishedReadPathMu.Lock()
	log.Debug().Msgf("Aquired lock for oramNodeFSM in handleEndReadPathCommand")
	defer func() {
		log.Debug().Msgf("Releasing lock for oramNodeFSM in handleEndReadPathCommand")
		fsm.unfinishedReadPathMu.Unlock()
		log.Debug().Msgf("Released lock for oramNodeFSM in handleEndReadPathCommand")
	}()
	fsm.unfinishedReadPath = nil
}

func (fsm *oramNodeFSM) Apply(rLog *raft.Log) interface{} {
	switch rLog.Type {
	case raft.LogCommand:
		var command Command
		err := msgpack.Unmarshal(rLog.Data, &command)
		if err != nil {
			return fmt.Errorf("could not unmarshall the command; %s", err)
		}
		if command.Type == ReplicateBeginEviction {
			log.Debug().Msgf("got replication command for replicate begin eviction")
			var payload ReplicateBeginEvictionPayload
			err := msgpack.Unmarshal(command.Payload, &payload)
			if err != nil {
				return fmt.Errorf("could not unmarshall the begin eviction replication command; %s", err)
			}
			fsm.handleBeginEvictionCommand(payload.ToEvictCount, payload.StorageID)
		} else if command.Type == ReplicateEndEviction {
			log.Debug().Msgf("got replication command for replicate end eviction")
			var payload ReplicateEndEvictionPayload
			err := msgpack.Unmarshal(command.Payload, &payload)
			if err != nil {
				return fmt.Errorf("could not unmarshall the end eviction replication command; %s", err)
			}
			fsm.handleEndEvictionCommand(payload.DoneEvictCount, payload.StorageID)
		} else if command.Type == ReplicateBeginReadPath {
			log.Debug().Msgf("got replication command for replicate begin read path")
			var payload ReplicateBeginReadPathPayload
			err := msgpack.Unmarshal(command.Payload, &payload)
			if err != nil {
				return fmt.Errorf("could not unmarshall the begin read path replication command; %s", err)
			}
			fsm.handleBeginReadPathCommand(payload.Paths, payload.StorageID)
		} else if command.Type == ReplicateEndReadPath {
			log.Debug().Msgf("got replication command for replicate end read path")
			fsm.handleEndReadPathCommand()
		} else {
			log.Error().Msgf("wrong command type")
		}
	default:
		return fmt.Errorf("unknown raft log type: %s", rLog.Type)
	}
	return nil
}

type snapshotNoop struct{}

func (sn snapshotNoop) Persist(_ raft.SnapshotSink) error { return nil }
func (sn snapshotNoop) Release()                          {}

func (fsm *oramNodeFSM) Snapshot() (raft.FSMSnapshot, error) {
	return snapshotNoop{}, nil
}

func (fsm *oramNodeFSM) Restore(rc io.ReadCloser) error {
	return fmt.Errorf("not implemented yet")
}

// TODO: the logic for startRaftServer is the same for both shardNode and OramNode.
// TOOD: it can be moved to a new raft-utils package to reduce code duplication

func startRaftServer(isFirst bool, bindip string, advip string, replicaID int, raftPort int, oramNodeFSM *oramNodeFSM) (*raft.Raft, error) {
	raftConfig := raft.DefaultConfig()
	// These should be here for the crash experiment
	raftConfig.ElectionTimeout = 1 * time.Second
	raftConfig.HeartbeatTimeout = 1 * time.Second
	raftConfig.LeaderLeaseTimeout = 1 * time.Second

	raftConfig.Logger = hclog.New(&hclog.LoggerOptions{Output: log.Logger})
	raftConfig.LocalID = raft.ServerID(strconv.Itoa(replicaID))

	store := raft.NewInmemStore()

	snapshots := raft.NewInmemSnapshotStore()

	bindAddr := fmt.Sprintf("%s:%d", bindip, raftPort)
	advAddr := fmt.Sprintf("%s:%d", advip, raftPort)
	tcpAdvertiseAddr, err := net.ResolveTCPAddr("tcp", advAddr)
	if err != nil {
		return nil, fmt.Errorf("could not resolve tcp addr; %s", err)
	}

	transport, err := raft.NewTCPTransport(bindAddr, tcpAdvertiseAddr, 10, time.Second*10, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("could not create tcp transport; %s", err)
	}

	r, err := raft.NewRaft(raftConfig, oramNodeFSM, store, store, snapshots, transport)
	if err != nil {
		return nil, fmt.Errorf("could not create raft instance; %s", err)
	}

	// This node becomes the cluster bootstraper if it is the first node and no joinAddr is specified
	if isFirst {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftConfig.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		r.BootstrapCluster(configuration)
	}
	return r, nil
}
