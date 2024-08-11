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
}

func (h *Handler) Broadcast(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	go func() {
		_ = h.Node.Reply(msg, map[string]any{
			"type": "broadcast_ok",
		})
	}()
	message := to_i(body["message"])
	h.StorageMutex.Lock()
	defer h.StorageMutex.Unlock()
	if _, exists := h.Storage[message]; exists || body["ttl"] != nil && to_i(body["ttl"]) <= 0 {
		return nil
	}

	h.Storage[message] = struct{}{}

	updatedTtl := h.Ttl - 1
	body["ttl"] = updatedTtl
	h.TopologyMutex.Lock()
	defer h.TopologyMutex.Unlock()
	neighbors := h.TopologyStorage[h.Node.ID()]
	log.Println("Neighbors", neighbors)
	for _, node := range neighbors {
		if msg.Src == node || msg.Dest == node {
			continue
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
		}(node, body)
	}
	return nil
}

func (h *Handler) Read(msg maelstrom.Message) error {
	var body Readbody
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
		tree[fmt.Sprintf("n%d", 0)] = append(tree[fmt.Sprintf("n%d", 0)], fmt.Sprintf("n%d", id))
	}
	for id := 1; id < len(nodeIds); id++ {
		tree[fmt.Sprintf("n%d", id)] = append(tree[fmt.Sprintf("n%d", 0)], fmt.Sprintf("n%d", 0))
	}
	for node := range tree {
		tree[node] = append(tree[node], fmt.Sprintf("n%d", 0))
	}
	h.TopologyMutex.Lock()
	defer h.TopologyMutex.Unlock()
	h.TopologyStorage = tree
	return nil
}

func to_i(any interface{}) int {
	return int(any.(float64))
}
