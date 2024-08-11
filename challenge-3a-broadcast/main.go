package main

import (
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

	// Start Broadcast workload (#3A and 3B)
	n.Handle("broadcast", h.Broadcast)
	n.Handle("read", h.Read)
	n.Handle("topology", h.Topology)
	// End Broadcast workload

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}

}
