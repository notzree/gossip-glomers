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
	id := c.Node.ID()
	c.Id = id
	neighbors := c.Node.NodeIDs()
	for _, node := range neighbors {
		c.KvMutex.Lock()
		writeCtx, writeCancel := context.WithCancel(context.Background())
		defer writeCancel()
		c.KvMutex.Unlock()
		if err := c.Kv.Write(writeCtx, node, 0); err != nil {
			log.Printf("Error initializing node %s: %v", node, err)
			return err
		}
	}
	return nil
}

// Increments local counter
func (c *Counter) Add(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	// go func() {
	// c.Node.Reply(msg, map[string]any{
	// 	"type": "add_ok",
	// })
	// }()
	delta := int(body["delta"].(float64))
	readCtx, readCancel := context.WithCancel(context.Background())
	defer readCancel()
	c.KvMutex.Lock()
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
	c.KvMutex.Unlock()
	//propogate write to neighbors
	neighbors := c.Node.NodeIDs()
	wg := &sync.WaitGroup{}
	wg.Add(len(neighbors))

	for _, node := range neighbors {
		if node == c.Id {
			continue
		}
		go func() {
			c.Node.RPC(node, map[string]any{
				"type":  "sync",
				"delta": delta,
			}, func(m maelstrom.Message) error {
				defer wg.Done()
				var body map[string]any
				if err := json.Unmarshal(m.Body, &body); err != nil {
					return err
				}
				return nil
			})
		}()
	}
	wg.Wait()
	return c.Node.Reply(msg, map[string]any{
		"type": "add_ok",
	})
}

func (c *Counter) Read(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	neighbors := c.Node.NodeIDs()
	// wg := &sync.WaitGroup{}
	// wg.Add(len(neighbors))

	sum := 0

	for _, node := range neighbors {

		readCtx, readCancel := context.WithCancel(context.Background())
		c.KvMutex.Lock()
		value, err := c.Kv.ReadInt(readCtx, node)
		c.KvMutex.Unlock()
		readCancel()
		if err != nil {
			log.Printf("Error reading node %s : %v", node, err)
		}
		sum += value
	}

	// for _, node := range neighbors {
	// 	go func(n string, wg *sync.WaitGroup) {
	// 		defer wg.Done()
	// 		readCtx, readCancel := context.WithCancel(context.Background())
	// 		c.KvMutex.Lock()
	// 		value, err := c.Kv.ReadInt(readCtx, n)
	// 		log.Default().Printf("Reading node %s with value %d", n, value)
	// 		c.KvMutex.Unlock()
	// 		readCancel()
	// 		if err != nil {
	// 			log.Printf("Error reading node %s : %v", n, err)
	// 		}
	// 		sumMutex.Lock()
	// 		log.Default().Printf("Adding %d to sum", sum)
	// 		sum += value
	// 		sumMutex.Unlock()
	// 	}(node, wg)
	// }
	// wg.Wait()
	log.Default().Printf("Returning sum %d", sum)
	return c.Node.Reply(msg, map[string]any{
		"type":  "read_ok",
		"value": sum,
	})
}

// Updates local state with an incoming operation
func (c *Counter) Sync(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	incomingId := msg.Src
	incomingDelta := int(body["delta"].(float64))
	c.KvMutex.Lock()
	readCtx, readCancel := context.WithCancel(context.Background())
	defer readCancel()
	value, err := c.Kv.ReadInt(readCtx, incomingId)
	if err != nil {
		return err
	}
	writeCtx, writeCancel := context.WithCancel(context.Background())
	err = c.Kv.Write(writeCtx, incomingId, value+incomingDelta)
	writeCancel()
	c.KvMutex.Unlock()
	if err != nil {
		log.Printf("Error syncing node: %v", err)
		return err
	}
	return c.Node.Reply(msg, map[string]any{
		"type": "sync_ok",
	})
}
