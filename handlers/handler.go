package handlers

import (
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type Handler struct {
	Node            *maelstrom.Node
	StorageMutex    *sync.Mutex
	Storage         map[string]int
	TopologyStorage []string
	Ttl             int
}
