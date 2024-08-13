package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type Counter struct {
	Node    *maelstrom.Node
	KvMutex *sync.Mutex
	Kv      *maelstrom.KV
	Id      string
}

func NewCounter(n *maelstrom.Node) Counter {
	return Counter{
		Node:    n,
		KvMutex: &sync.Mutex{},
		Kv:      maelstrom.NewSeqKV(n),
	}
}

func (c *Counter) Init(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	c.KvMutex.Lock()
	defer c.KvMutex.Unlock()
	id := c.Node.ID()
	c.Id = id
	writeCtx, writeCancel := context.WithCancel(context.Background())
	defer writeCancel()
	if err := c.Kv.Write(writeCtx, id, 0); err != nil {
		log.Printf("Error initializing node: %v", err)
		return err
	}
	return nil
}

// Increments local counter
func (c *Counter) Add(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	go func() {
		c.Node.Reply(msg, map[string]any{
			"type": "add_ok",
		})
	}()
	delta := int(body["delta"].(float64))
	readCtx, readCancel := context.WithCancel(context.Background())
	defer readCancel()
	c.KvMutex.Lock()
	defer c.KvMutex.Unlock()
	value, err := c.Kv.ReadInt(readCtx, c.Id)
	if err != nil {
		log.Printf("Error reading node: %v", err)
		return err
	}
	writeCtx, writeCancel := context.WithCancel(context.Background())
	defer writeCancel()
	if err := c.Kv.Write(writeCtx, c.Id, value+delta); err != nil {
		log.Printf("Error writing to node: %v", err)
		return err
	}
	return nil
}

func (c *Counter) Read(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	value := 0
	nodes := c.Node.NodeIDs()
	for _, node := range nodes {
		if node == c.Id {
			readCtx, readCancel := context.WithCancel(context.Background())
			defer readCancel()
			localValue, err := c.Kv.ReadInt(readCtx, c.Id)
			if err != nil {
				log.Printf("Error reading node: %v", err)
				return err
			}
			value += localValue
		}
		_ = c.Node.RPC(node, map[string]any{
			"type": "sync",
		}, func(msg maelstrom.Message) error {
			var body map[string]any
			if err := json.Unmarshal(msg.Body, &body); err != nil {
				return err
			}
			value += int(body["value"].(float64))
			return nil
		})
	}
	return c.Node.Reply(msg, map[string]any{
		"type":  "read_ok",
		"value": value,
	})
}

// Reads local counter and returns it to sync with the read node to return global counter
func (c *Counter) Sync(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	c.KvMutex.Lock()
	defer c.KvMutex.Unlock()
	readCtx, readCancel := context.WithCancel(context.Background())
	defer readCancel()
	value, err := c.Kv.ReadInt(readCtx, c.Id)
	if err != nil {
		log.Printf("Error syncing node: %v", err)
		return err
	}
	return c.Node.Reply(msg, map[string]any{
		"type":  "sync_ok",
		"value": value,
	})
}

// Reads global counter by reading all local counters in network and summing them
func (c *Counter) Read2(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	c.KvMutex.Lock()
	defer c.KvMutex.Unlock()
	readCtx, readCancel := context.WithCancel(context.Background())
	defer readCancel()
	value, err := c.Kv.ReadInt(readCtx, c.Id)
	if err != nil {
		log.Printf("Error reading node: %v", err)
		return err
	}

	syncWg := &sync.WaitGroup{}
	valueMutex := &sync.Mutex{}
	nodes := c.Node.NodeIDs()

	for _, node := range nodes {
		if node == c.Id {
			continue
		}
		syncWg.Add(1)
		go func(node string, valueMutex *sync.Mutex, wg *sync.WaitGroup) {
			_ = c.Node.RPC(node, map[string]any{
				"type": "sync",
			}, func(msg maelstrom.Message) error {
				var body map[string]any
				if err := json.Unmarshal(msg.Body, &body); err != nil {
					return err
				}
				valueMutex.Lock()
				value += int(body["value"].(float64))
				valueMutex.Unlock()
				wg.Done()
				return nil
			})
		}(node, valueMutex, syncWg)
	}
	syncWg.Wait()
	return c.Node.Reply(msg, map[string]any{
		"type":  "read_ok",
		"value": value,
	})
}
