package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

const (
	latestPrefix = "latest_" // stores latest log
	commitPrefix = "commit_" //stores latest committed log
	logPrefix    = "log_"    // stores the actual logs
)

type Kafka struct {
	Node         *maelstrom.Node
	StorageMutex *sync.RWMutex
	Storage      *maelstrom.KV //shared thing that syncs between all nodes !?
}

func NewKafka(n *maelstrom.Node) *Kafka {
	return &Kafka{
		Node:         n,
		StorageMutex: &sync.RWMutex{},
		// Linearizable KV
		// makes sure every node agrees on order of operation + all on the same time frame
		Storage: maelstrom.NewLinKV(n),
	}
}

func (kafka Kafka) SendRPC(rawMsg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(rawMsg.Body, &body); err != nil {
		return err
	}
	key := body["key"].(string)
	msg := int(body["msg"].(float64))
	kafka.StorageMutex.Lock()
	defer kafka.StorageMutex.Unlock()
	latestOffset, err := PrettyReadInt(kafka.Storage, fmtkey(latestPrefix, key))
	if err != nil {
		latestOffset = 1
	}
	// acquire "lock" on log
	// distributed counter ensuring that if multiple nodes contest the write, each node will end up with a uniquely assigned
	// offset value with no conflicts
	for ; ; latestOffset++ {
		err := kafka.Storage.CompareAndSwap(context.Background(), fmtkey(latestPrefix, key), latestOffset-1, latestOffset, true)
		if err != nil {
			continue
		}
		break
	}
	// now our node can write to the log prefix with the offset latestOffset (we own this, and no other node can write to it due to cas loop)
	go func() {
		kafka.Node.Reply(rawMsg, map[string]any{
			"type":   "send_ok",
			"offset": latestOffset,
		})
	}()
	return PrettyWriteInt(kafka.Storage, fmtkey(logPrefix, key, WithOffset(latestOffset)), msg)
}

type PollRPCBody struct {
	Type    string         `json:"type"`
	Offsets map[string]int `json:"offsets"`
}

func (kafka Kafka) PollRPC(rawMsg maelstrom.Message) error {
	var body PollRPCBody
	if err := json.Unmarshal(rawMsg.Body, &body); err != nil {
		return err
	}
	kafka.StorageMutex.RLock()
	defer kafka.StorageMutex.RUnlock()
	logs := make(map[string][][2]int)

	for key, start := range body.Offsets {
		latestOffset, err := PrettyReadInt(kafka.Storage, fmtkey(latestPrefix, key))
		if err != nil {
			continue
		}
		if start >= latestOffset {
			continue
		}
		keyedMessages := make([][2]int, 0)
		for offset := start; offset <= latestOffset; offset++ {
			log, err := PrettyReadInt(kafka.Storage, fmtkey(logPrefix, key, WithOffset(offset)))
			if err != nil {
				continue
			}
			keyedMessages = append(keyedMessages, [2]int{offset, log})
		}
		logs[key] = keyedMessages
	}
	return kafka.Node.Reply(rawMsg, map[string]any{
		"type": "poll_ok",
		"msgs": logs,
	})
}

type CommitOffsetsRPCBody struct {
	Type    string         `json:"type"`
	Offsets map[string]int `json:"offsets"`
}

func (kafka Kafka) CommitOffsetsRPC(rawMsg maelstrom.Message) error {
	var body CommitOffsetsRPCBody
	if err := json.Unmarshal(rawMsg.Body, &body); err != nil {
		return err
	}
	kafka.StorageMutex.Lock()
	defer kafka.StorageMutex.Unlock()
	for key, offset := range body.Offsets {
		if err := PrettyWriteInt(kafka.Storage, fmtkey(commitPrefix, key), offset); err != nil {
			return err
		}
	}

	return kafka.Node.Reply(rawMsg, map[string]any{
		"type": "commit_offsets_ok",
	})
}

type ListCommitedOffsetsRPCBody struct {
	Type string   `json:"type"`
	Keys []string `json:"keys"`
}

func (kafka Kafka) ListCommittedOffsetsRPC(rawMsg maelstrom.Message) error {
	var body ListCommitedOffsetsRPCBody
	if err := json.Unmarshal(rawMsg.Body, &body); err != nil {
		return err
	}
	keyedOffsets := make(map[string]int)
	kafka.StorageMutex.RLock()
	defer kafka.StorageMutex.RUnlock()
	for _, key := range body.Keys {
		offset, err := PrettyReadInt(kafka.Storage, fmtkey(commitPrefix, key))
		if err != nil {
			continue
		}
		keyedOffsets[key] = offset
	}
	return kafka.Node.Reply(rawMsg, map[string]any{
		"type":    "list_committed_offsets_ok",
		"offsets": keyedOffsets,
	})
}

type Option func(*string)

func fmtkey(prefix string, key string, opts ...Option) string {
	str := fmt.Sprintf("%s%s", prefix, key)
	for _, op := range opts {
		op(&str)
	}
	return str
}

func WithOffset(offset int) Option {
	return func(str *string) {
		*str = fmt.Sprintf("%s_%d", *str, offset)
	}
}

func PrettyReadInt(storage *maelstrom.KV, key string) (int, error) {
	readContext, readCancel := context.WithCancel(context.Background())
	defer readCancel()
	value, err := storage.ReadInt(readContext, key)
	if err != nil {
		return -1, err
	}
	return value, nil
}
func PrettyWriteInt(storage *maelstrom.KV, key string, value int) error {
	writeContext, writeCancel := context.WithCancel(context.Background())
	defer writeCancel()
	return storage.Write(writeContext, key, value)
}
