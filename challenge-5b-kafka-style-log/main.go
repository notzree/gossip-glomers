package main

import (
	"log"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	kv := maelstrom.NewLinKV(n)
	kafka := &Kafka{
		Node:         n,
		StorageMutex: &sync.Mutex{},
		Storage:      kv,
	}
	n.Handle("send", kafka.Send)
	n.Handle("poll", kafka.Poll)
	n.Handle("commit_offsets", kafka.CommitOffsets)
	n.Handle("list_committed_offsets", kafka.ListCommittedOffsets)
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}

}
