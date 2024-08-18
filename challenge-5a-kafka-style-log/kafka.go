package main

import (
	"encoding/json"
	"fmt"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type Kafka struct {
	Node         *maelstrom.Node
	StorageMutex *sync.Mutex
	Storage      map[string]*LogContainer
}

type LogContainer struct {
	LastCreatedOffset   int
	LastProcessedOffset int
	Logs                []int
}

func (l *LogContainer) AddLog(value int) {
	l.Logs = append(l.Logs, value)
	newOffset := l.LastCreatedOffset + 1
	l.LastCreatedOffset = newOffset
}

func (k *Kafka) Send(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	key := body["key"].(string)
	value := int(body["msg"].(float64))
	k.StorageMutex.Lock()
	defer k.StorageMutex.Unlock()
	if _, ok := k.Storage[key]; !ok {
		k.Storage[key] = &LogContainer{
			LastCreatedOffset:   -1,
			LastProcessedOffset: -1,
			Logs:                []int{},
		}
	}
	logContainer := k.Storage[key]
	logContainer.AddLog(value)
	return k.Node.Reply(msg, map[string]any{
		"type":   "send_ok",
		"offset": logContainer.LastCreatedOffset,
	})
}

func (k *Kafka) Poll(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	offsets := make(map[string]int)
	if offsetsData, ok := body["offsets"].(map[string]interface{}); ok {
		for key, val := range offsetsData {
			offsets[key] = int(val.(float64))
		}
	}

	k.StorageMutex.Lock()
	defer k.StorageMutex.Unlock()

	messages := make(map[string][][2]int)
	for key, startingOffset := range offsets {
		if _, exists := k.Storage[key]; !exists {
			continue
		}
		logContainer := k.Storage[key]
		if startingOffset >= len(logContainer.Logs) {
			continue
		}

		keyedMessages := make([][2]int, 0, len(logContainer.Logs))

		for index := startingOffset; index < len(logContainer.Logs); index += 1 {
			value := logContainer.Logs[index]
			offsetValuePair := [2]int{index, value}
			keyedMessages = append(keyedMessages, offsetValuePair)
		}
		messages[key] = keyedMessages
	}
	return k.Node.Reply(msg, map[string]any{
		"type": "poll_ok",
		"msgs": messages,
	})
}

func (k *Kafka) CommitOffsets(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	go func() {
		k.Node.Reply(msg, map[string]any{
			"type": "commit_offsets_ok",
		})
	}()

	offsets := make(map[string]int)
	if offsetsData, ok := body["offsets"].(map[string]interface{}); ok {
		for key, val := range offsetsData {
			offsets[key] = int(val.(float64))
		}
	}
	k.StorageMutex.Lock()
	defer k.StorageMutex.Unlock()
	for key, offset := range offsets {
		if _, exists := k.Storage[key]; !exists {
			continue
		}
		k.Storage[key].LastProcessedOffset = offset
	}
	return nil
}

func (k *Kafka) ListCommittedOffsets(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	keys := make([]string, 0)
	if keyData, ok := body["keys"].([]interface{}); ok {
		for _, key := range keyData {
			if keyStr, ok := key.(string); ok {
				keys = append(keys, keyStr)
			} else {
				return fmt.Errorf("invalid key type")
			}
		}
	} else {
		return fmt.Errorf("keys field not found or of wrong type")
	}
	offsets := make(map[string]int)
	k.StorageMutex.Lock()
	defer k.StorageMutex.Unlock()
	for _, key := range keys {
		if _, exists := k.Storage[key]; !exists {
			continue
		}
		if k.Storage[key].LastProcessedOffset < 0 {
			// has not been set
			continue
		}
		offsets[key] = k.Storage[key].LastProcessedOffset
	}
	return k.Node.Reply(msg, map[string]any{
		"type":    "list_committed_offsets_ok",
		"offsets": offsets,
	})

}
