package main

import (
	"log"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	h := Handler{
		Node:            n,
		StorageMutex:    &sync.Mutex{},
		Storage:         make(map[int]struct{}),
		TopologyMutex:   &sync.Mutex{},
		TopologyStorage: make(map[string][]string),
		Ttl:             3,
	}
	n.Handle("broadcast", h.Broadcast)
	n.Handle("read", h.Read)
	n.Handle("topology", h.Topology)
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}

}
