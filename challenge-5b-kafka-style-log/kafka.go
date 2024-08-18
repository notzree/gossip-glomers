package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

const (
	commitPrefix = "commit_"
	latestPrefix = "latest_" //latestPrefix_key stores the latest offset for a key (represents the latest)
	logPrefix    = "log_"    // logPrefix_key stores all the logs related to a key (array of ints)
)

type Kafka struct {
	Node         *maelstrom.Node
	StorageMutex *sync.Mutex
	Storage      *maelstrom.KV //shared thing that syncs between all nodes !?
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
	latestOffset, err := PrettyReadInt(k.Storage, fmtkey(latestPrefix, key))
	if err != nil {
		latestOffset = 0
	}
	//Read the most recent log, now we have to ensure that no other nodes are using the log
	for ; ; latestOffset++ {
		if err := k.Storage.CompareAndSwap(context.Background(),
			fmtkey(latestPrefix, key), latestOffset-1, latestOffset, true,
		); err != nil {
			log.Printf("cas retry: %v", err)
			continue
		}
		break
	}
	// At this point we have updated the logOffset with logOffset. This means that we can safely write with it.
	go func() {
		k.Node.Reply(msg, map[string]any{
			"type":   "send_ok",
			"offset": latestOffset,
		})
	}()
	return PrettyWriteInt(k.Storage, fmtkey(logPrefix, key, WithOffset(latestOffset)), value)
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
		latestOffset, err := PrettyReadInt(k.Storage, fmtkey(latestPrefix, key))
		if err != nil {
			continue
		}
		//The starting offset is greater than what exists
		if startingOffset >= latestOffset {
			continue
		}
		keyedMessages := make([][2]int, 0, latestOffset) //offset represents the number of logs there are
		for offset := startingOffset; offset <= latestOffset; offset += 1 {
			log, err := PrettyReadInt(k.Storage, fmtkey(logPrefix, key, WithOffset(offset)))
			if err != nil {
				// offset may not exist? idk actually
				continue
			}
			offsetValuePair := [2]int{offset, log}
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

	committedOffsets := make(map[string]int)
	if offsetsData, ok := body["offsets"].(map[string]interface{}); ok {
		for key, val := range offsetsData {
			committedOffsets[key] = int(val.(float64))
		}
	}
	k.StorageMutex.Lock()
	defer k.StorageMutex.Unlock()

	for key, commitedOffset := range committedOffsets {
		err := PrettyWriteInt(k.Storage, fmtkey(commitPrefix, key), commitedOffset)
		if err != nil {
			//trouble writing committed offset
			return err
		}
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
		value, err := PrettyReadInt(k.Storage, fmtkey(commitPrefix, key))
		if err != nil {
			continue // perhaps does not exist
		}

		offsets[key] = value
	}
	return k.Node.Reply(msg, map[string]any{
		"type":    "list_committed_offsets_ok",
		"offsets": offsets,
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
