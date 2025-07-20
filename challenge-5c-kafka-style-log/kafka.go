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
	LogStorage   *maelstrom.KV //sequential KV (logs)
	MetaStorage  *maelstrom.KV //linearizable KV (leaders)
	OffsetLock   *sync.RWMutex
	OwnedOffsets map[string]int
	ID           int
}

func NewKafka(n *maelstrom.Node) *Kafka {
	return &Kafka{
		Node:         n,
		LogStorage:   maelstrom.NewSeqKV(n),
		MetaStorage:  maelstrom.NewLinKV(n),
		OffsetLock:   &sync.RWMutex{},
		OwnedOffsets: make(map[string]int),
	}
}

// CheckOrClaim returns the nodeId that owns the partition key, or claims the id for itself if nobody owns it.
func (kafka *Kafka) CheckOrClaim(partitionKey string) (string, error) {
	ctx := context.Background()

	// Try to read current owner
	val, err := kafka.MetaStorage.Read(ctx, partitionKey)
	if err == nil {
		// Key exists, return owner
		return val.(string), nil
	}

	// Key doesn't exist, try to claim
	err = kafka.MetaStorage.CompareAndSwap(ctx,
		partitionKey,
		nil,             // Compare with nil (not 0!)
		kafka.Node.ID(), // Store node ID as string
		true)

	if err != nil {
		// Someone else claimed it, read who
		val, err := kafka.MetaStorage.Read(ctx, partitionKey)
		if err != nil {
			return "", err
		}
		return val.(string), nil
	}

	return kafka.Node.ID(), nil
}

// GetLatestOffset will return the latest offset of the key that this node knows off
func (kafka Kafka) GetLatestOffset(partitionKey string) (int, error) {
	kafka.OffsetLock.RLock()
	defer kafka.OffsetLock.RUnlock()
	key, exists := kafka.OwnedOffsets[partitionKey]
	if exists {
		return key, nil
	}
	// we are not the leader of this partition, try to read from seqkv
	return PrettyReadInt(kafka.LogStorage, fmtkey(latestPrefix, partitionKey))
}

func (kafka *Kafka) SendRPC(rawMsg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(rawMsg.Body, &body); err != nil {
		return err
	}
	key := body["key"].(string)
	msg := int(body["msg"].(float64))

	// Try to claim or get owner
	ownerId, err := kafka.CheckOrClaim(key)
	if err != nil {
		return err
	}
	if ownerId != kafka.Node.ID() {
		return kafka.ForwardSendRequest(rawMsg, ownerId, key, msg)
	}

	// We're the leader - handle atomically
	kafka.OffsetLock.Lock()
	if _, exists := kafka.OwnedOffsets[key]; !exists {
		// First time - recover offset
		// recovered returns zero if we don't have it in logStorage
		recovered, _ := PrettyReadInt(kafka.LogStorage, fmtkey(latestPrefix, key))
		// TODO: Figure out why we can't just assign kafka.OwnedOffsets[key] = 0
		// I dont think there should be any changes in leadership but if there are, that might explain it
		kafka.OwnedOffsets[key] = recovered
	}

	currentOffset := kafka.OwnedOffsets[key]
	kafka.OwnedOffsets[key]++
	nextOffset := kafka.OwnedOffsets[key]
	kafka.OffsetLock.Unlock()

	// Write message
	err = PrettyWriteInt(kafka.LogStorage,
		fmtkey(logPrefix, key, WithOffset(currentOffset)), msg)
	if err != nil {
		return err
	}

	// Update high water mark
	err = PrettyWriteInt(kafka.LogStorage,
		fmtkey(latestPrefix, key), nextOffset)
	if err != nil {
		return err
	}

	return kafka.Node.Reply(rawMsg, map[string]any{
		"type":   "send_ok",
		"offset": currentOffset,
	})
}

func (kafka *Kafka) ForwardSendRequest(originalMsg maelstrom.Message,
	targetNode string, key string, msg int) error {

	// Create a new internal message
	forwardBody := map[string]any{
		"type": "send",
		"key":  key,
		"msg":  msg,
	}

	// Use RPC to wait for response
	return kafka.Node.RPC(targetNode, forwardBody,
		func(response maelstrom.Message) error {
			// Forward the response back to original client
			return kafka.Node.Reply(originalMsg, response.Body)
		})
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
	logs := make(map[string][][2]int)

	for key, start := range body.Offsets {
		latestOffset, err := kafka.GetLatestOffset(key)
		if err != nil {
			continue
		}
		if start >= latestOffset {
			continue
		}
		keyedMessages := make([][2]int, 0)
		for offset := start; offset < latestOffset; offset++ {
			log, err := PrettyReadInt(kafka.LogStorage, fmtkey(logPrefix, key, WithOffset(offset)))
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
	for key, offset := range body.Offsets {
		if err := PrettyWriteInt(kafka.LogStorage, fmtkey(commitPrefix, key), offset); err != nil {
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
	for _, key := range body.Keys {
		offset, err := PrettyReadInt(kafka.LogStorage, fmtkey(commitPrefix, key))
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
		return 0, err
	}
	return value, nil
}
func PrettyWriteInt(storage *maelstrom.KV, key string, value int) error {
	writeContext, writeCancel := context.WithCancel(context.Background())
	defer writeCancel()
	return storage.Write(writeContext, key, value)
}
