package main

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type Handler struct {
	Node            *maelstrom.Node
	StorageMutex    *sync.Mutex
	Storage         map[string]int
	TopologyStorage []string
}

func (h *Handler) Broadcast(msg maelstrom.Message) error {
	var body BroadcastBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	h.StorageMutex.Lock()
	defer h.StorageMutex.Unlock()
	// Message was broadcasted before and already stored
	if body.MessageId != nil {
		if _, ok := h.Storage[*body.MessageId]; ok {
			return nil
		}
	}
	// Message was not broadcasted before, or was broadcasted but not stored
	var messageId string
	if body.MessageId == nil {
		messageId = uuid.New().String()
	} else {
		messageId = *body.MessageId
	}
	message := body.Message
	h.Storage[messageId] = message

	//broadcast to neighbouring nodes
	for _, node := range h.TopologyStorage {
		newBody := BroadcastBody{
			Type:      "broadcast",
			Message:   message,
			MessageId: &messageId,
		}
		if err := h.Node.RPC(node, newBody, func(_ maelstrom.Message) error { return nil }); err != nil {
			return err
		}
	}
	responseBody := map[string]any{
		"type": "broadcast_ok",
	}
	return h.Node.Reply(msg, responseBody)
}

func (h *Handler) Read(msg maelstrom.Message) error {
	var body Readbody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	h.StorageMutex.Lock()
	defer h.StorageMutex.Unlock()
	values := make([]int, 0, len(h.Storage))
	for _, value := range h.Storage {
		values = append(values, value)
	}
	newBody := map[string]any{
		"type":     "read_ok",
		"messages": values,
	}

	return h.Node.Reply(msg, newBody)
}

func (h *Handler) Topology(msg maelstrom.Message) error {
	var body TopologyBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	currentNodeId := h.Node.ID()
	h.TopologyStorage = body.Topology[currentNodeId]

	new_body := map[string]any{
		"type": "topology_ok",
	}

	return h.Node.Reply(msg, new_body)
}
