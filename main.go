package main

import (
	"encoding/json"
	"log"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"github.com/notzree/gossip-glomers/v2/handlers"
)

func main() {
	n := maelstrom.NewNode()
	h := handlers.Handler{
		Node:            n,
		StorageMutex:    &sync.Mutex{},
		Storage:         make(map[string]int),
		TopologyStorage: make([]string, 0),
		Ttl:             3,
	}

	// Start Echo workload
	n.Handle("echo", func(msg maelstrom.Message) error {
		// Unmarshal the message body as an loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type to return back.
		body["type"] = "echo_ok"

		// Echo the original message back with the updated message type.
		return n.Reply(msg, body)
	})
	// End Echo workload

	// Start UUID Generation workload
	n.Handle("generate", h.GenerateUuid)
	// End UUID Generation workload
	// Start Broadcast workload (#3A and 3B)
	// Note: Guaranteed that topology is the first message received after init
	n.Handle("broadcast", h.Broadcast)
	n.Handle("read", h.Read)
	n.Handle("topology", h.Topology)
	// End Broadcast workload

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}

}
