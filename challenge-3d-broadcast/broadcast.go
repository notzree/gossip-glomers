package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type Handler struct {
	Node            *maelstrom.Node
	StorageMutex    *sync.Mutex
	Storage         map[int]struct{}
	TopologyMutex   *sync.Mutex
	TopologyStorage map[string][]string
	Ttl             int
}

func (h *Handler) Broadcast(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	message := to_i(body["message"])
	h.StorageMutex.Lock()
	defer h.StorageMutex.Unlock()
	if _, exists := h.Storage[message]; exists || body["ttl"] != nil && to_i(body["ttl"]) <= 0 {
		return nil
	}
	go func() {
		_ = h.Node.Reply(msg, map[string]any{
			"type": "broadcast_ok",
		})
	}()

	if body["ttl"] == nil {
		body["ttl"] = h.Ttl
	} else {
		body["ttl"] = to_i(body["ttl"]) - 1
	}

	h.Storage[message] = struct{}{}
	h.TopologyMutex.Lock()
	defer h.TopologyMutex.Unlock()
	neighbors := h.TopologyStorage[h.Node.ID()]
	for _, node := range neighbors {
		if msg.Src == node || msg.Dest == node {
			continue
		}

		// Create a copy of the body map for each goroutine
		newBody := make(map[string]any)
		for k, v := range body {
			newBody[k] = v
		}

		go func(node string, broadcast map[string]any) {
			for {
				if err := h.Node.RPC(node, broadcast, func(_ maelstrom.Message) error {
					return nil
				}); err == nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(node, newBody)
	}
	return nil
}

func (h *Handler) Read(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	h.StorageMutex.Lock()
	defer h.StorageMutex.Unlock()
	messages := make([]int, 0, len(h.Storage))
	for message := range h.Storage {
		messages = append(messages, message)
	}
	newBody := map[string]any{
		"type":     "read_ok",
		"messages": messages,
	}

	return h.Node.Reply(msg, newBody)
}

func (h *Handler) Topology(msg maelstrom.Message) error {
	go func() {
		_ = h.Node.Reply(msg, map[string]any{
			"type": "topology_ok",
		})
	}()
	tree := make(map[string][]string)
	nodeIds := h.Node.NodeIDs()
	for id := 1; id < len(nodeIds); id++ {
		tree[fmt.Sprintf("n%d", 0)] = append(tree[fmt.Sprintf("n%d", 0)], fmt.Sprintf("n%d", id))  // Add all nodes to the root node
		tree[fmt.Sprintf("n%d", id)] = append(tree[fmt.Sprintf("n%d", id)], fmt.Sprintf("n%d", 0)) // Add the root node to all nodes
	}
	h.TopologyMutex.Lock()
	defer h.TopologyMutex.Unlock()
	h.TopologyStorage = tree
	return nil
}

func to_i(any interface{}) int {
	return int(any.(float64))
}
