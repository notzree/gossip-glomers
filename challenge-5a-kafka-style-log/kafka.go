package main

import (
	"encoding/json"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type Kafka struct {
	Node    *maelstrom.Node
	KvMutex *sync.RWMutex
	Kv      map[string]*Log
}

type Log struct {
	LastCommitedOffset int
	LastOffset         int
	Logs               []int
}

func NewLog() *Log {
	return &Log{
		LastCommitedOffset: -1,
		LastOffset:         -1,
		Logs:               make([]int, 0),
	}
}
func (l *Log) AddLog(value int) int {
	l.Logs = append(l.Logs, value)
	l.LastOffset = l.LastOffset + 1
	return l.LastOffset
}

func NewKafka(n *maelstrom.Node) *Kafka {
	return &Kafka{
		Node:    n,
		KvMutex: &sync.RWMutex{},
		Kv:      make(map[string]*Log),
	}
}

func (kafka Kafka) SendRPC(rawMsg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(rawMsg.Body, &body); err != nil {
		return err
	}
	key := body["key"].(string)
	msg := int(body["msg"].(float64))
	kafka.KvMutex.Lock()
	defer kafka.KvMutex.Unlock()
	if _, exists := kafka.Kv[key]; !exists {
		kafka.Kv[key] = NewLog()
	}
	offset := kafka.Kv[key].AddLog(msg)
	return kafka.Node.Reply(rawMsg, map[string]any{
		"type":   "send_ok",
		"offset": offset,
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
	kafka.KvMutex.RLock()
	defer kafka.KvMutex.RUnlock()
	logs := make(map[string][][2]int)
	for key, start := range body.Offsets {
		if _, exists := kafka.Kv[key]; !exists {
			// return errors.New("invalid key")
			continue
		}
		LogsPointer := kafka.Kv[key]
		if start >= len(LogsPointer.Logs) {
			// return errors.New("start offset does not exist")
			continue
		}
		// lets just return all messages
		keyMessages := make([][2]int, len(LogsPointer.Logs)-start)
		for i := start; i < len(LogsPointer.Logs); i += 1 {
			value := LogsPointer.Logs[i]
			keyMessages[i-start] = [2]int{i, value}
		}
		logs[key] = keyMessages
	}
	return kafka.Node.Reply(
		rawMsg, map[string]any{
			"type": "poll_ok",
			"msgs": logs,
		},
	)
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
	kafka.KvMutex.Lock()
	defer kafka.KvMutex.Unlock()
	for key, commitedOffset := range body.Offsets {
		if _, exists := kafka.Kv[key]; !exists {
			continue
		}
		LogsPointer := kafka.Kv[key]
		if LogsPointer.LastCommitedOffset > commitedOffset {
			continue
		}
		LogsPointer.LastCommitedOffset = commitedOffset

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
	kafka.KvMutex.RLock()
	defer kafka.KvMutex.RUnlock()
	for _, key := range body.Keys {
		if _, exists := kafka.Kv[key]; !exists {
			continue
		}
		if kafka.Kv[key].LastCommitedOffset < 0 {
			continue
		}

		keyedOffsets[key] = kafka.Kv[key].LastCommitedOffset
	}
	return kafka.Node.Reply(rawMsg, map[string]any{
		"type":    "list_committed_offsets_ok",
		"offsets": keyedOffsets,
	})
}
