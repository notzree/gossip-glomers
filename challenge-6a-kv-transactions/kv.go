package main

import (
	"encoding/json"
	"fmt"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type KeyValueStore struct {
	Node    *maelstrom.Node
	Storage AtomicStorage
}

func NewKVStore(node *maelstrom.Node, storage AtomicStorage) *KeyValueStore {
	return &KeyValueStore{
		Node:    node,
		Storage: storage,
	}
}

type OperationResult [3]any

func NewResult(op string, key int, value *int) OperationResult {
	return OperationResult{op, key, value}
}

type Operation [3]any

func (o *Operation) GetCmd() string {
	return o[0].(string)
}

func (o *Operation) GetKv() (int, *int, error) {
	keyFloat, ok := o[1].(float64)
	if !ok {
		return -1, nil, fmt.Errorf("expected float64 for key, got %T", o[1])
	}
	key := int(keyFloat)

	var value *int
	if o[2] != nil {
		if valueFloat, ok := o[2].(float64); ok {
			valueInt := int(valueFloat)
			value = &valueInt
		}
	}
	return key, value, nil
}

type TxnMessage struct {
	Type  string      `json:"type"`
	MsgId int         `json:"msg_ig"`
	Txn   []Operation `json:"txn"`
}

func (kv *KeyValueStore) TxnRPC(rawMsg maelstrom.Message) error {
	var body TxnMessage
	if err := json.Unmarshal(rawMsg.Body, &body); err != nil {
		return err
	}
	response := make([]OperationResult, len(body.Txn))
	for i, operation := range body.Txn {
		key, value, err := operation.GetKv()
		if err != nil {
			log.Printf("err occured when trying to deserialize operation: %v", err)
			continue
		}
		switch cmd := operation.GetCmd(); cmd {
		case "r":
			response[i] = NewResult("r", key, kv.Storage.Read(key))
		case "w":
			if value == nil {
				log.Printf("err: instructed to write nil value to store")
				continue
			}
			kv.Storage.Write(key, *value)
			response[i] = NewResult("w", key, value)
		default:
			continue

		}

	}

	return kv.Node.Reply(rawMsg, map[string]any{
		"type":        "txn_ok",
		"msg_id":      body.MsgId,
		"in_reply_to": body.MsgId,
		"txn":         response,
	})
}
