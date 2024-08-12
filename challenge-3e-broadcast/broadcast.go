package main

import (
	"encoding/json"
	"fmt"
	"log"
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
	BroadcastMutex  sync.Mutex
	BroadcastQueue  map[string][]int
}

func (h *Handler) BatchBroadcast() error {
	for {
		time.Sleep(500 * time.Millisecond)
		h.BroadcastMutex.Lock()
		for node, messages := range h.BroadcastQueue {
			if len(messages) == 0 || node == h.Node.ID() {
				continue
			}
			broadcast := map[string]any{
				"type":    "broadcast",
				"message": messages,
			}
			if err := h.Node.RPC(node, broadcast, func(_ maelstrom.Message) error {
				return nil
			}); err != nil {
				continue // Retry later
			}
			h.BroadcastQueue[node] = []int{}

		}
		h.BroadcastMutex.Unlock()
	}
}

func (h *Handler) Broadcast(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	h.StorageMutex.Lock()
	defer h.StorageMutex.Unlock()
	h.TopologyMutex.Lock()
	neighbors := h.TopologyStorage[h.Node.ID()]
	h.TopologyMutex.Unlock()

	if body["ttl"] != nil && to_i(body["ttl"]) <= 0 {
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

	messages := ToIntArray(body["message"])

	for _, message := range messages {
		if _, exists := h.Storage[message]; exists {
			continue
		}
		h.Storage[message] = struct{}{}
		for _, node := range neighbors {
			if msg.Src == node || msg.Dest == node {
				continue
			}
			h.BroadcastMutex.Lock()
			h.BroadcastQueue[node] = append(h.BroadcastQueue[node], message)
			h.BroadcastMutex.Unlock()
		}
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

func ToIntArray(any interface{}) []int {
	switch v := any.(type) {
	case float64:
		// If it's a single float64, convert it to an int and return as a slice of one element
		return []int{int(v)}
	case []interface{}:
		// If it's an array, iterate and convert each element to int
		intArray := make([]int, len(v))
		for i, elem := range v {
			intArray[i] = int(elem.(float64))
		}
		return intArray
	default:
		log.Fatal("Invalid type")
		return nil
	}
}
