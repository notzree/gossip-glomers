package handlers

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func (h *Handler) Broadcast(msg maelstrom.Message) error {
	var body BroadcastBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	responseBody := map[string]any{
		"type": "broadcast_ok",
	}
	err := h.Node.Reply(msg, responseBody)
	if err != nil {
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
	// Message has expired (only re-broadcasted messages have TTL)
	if body.Ttl != nil && *body.Ttl <= 0 {
		return nil
	}
	// First time receiving message, either messsage from another node or new broadcast
	var messageId string
	if body.MessageId == nil {
		messageId = uuid.New().String()
	} else {
		messageId = *body.MessageId
	}
	message := body.Message
	h.Storage[messageId] = message
	//message to broadcast to neighbouring nodes
	updatedTtl := h.Ttl - 1
	broadcast := BroadcastBody{
		Type:      "broadcast",
		Message:   message,
		MessageId: &messageId,
		Ttl:       &updatedTtl,
	}

	for _, node := range h.TopologyStorage {
		if msg.Src == node {
			continue
		}
		// if rand.Float64() > 0.8 { // probabilistic gossip
		// 	continue
		// }
		//For each node,  we will start a goroutine to try to broadcast that message
		go func(node string, broadcast BroadcastBody) {
			for {
				if err := h.Node.RPC(node, broadcast, func(_ maelstrom.Message) error {
					return nil
				}); err == nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(node, broadcast)
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

func remove(slice []int, i int) []int {
	return append(slice[:i], slice[i+1:]...)
}
