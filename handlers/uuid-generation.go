package handlers

import (
	"encoding/json"

	"github.com/google/uuid"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func (h *Handler) GenerateUuid(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	body["type"] = "generate_ok"

	uuid, err := uuid.NewUUID()
	if err != nil {
		return err
	}

	body["id"] = uuid.String()
	return h.Node.Reply(msg, body)
}
