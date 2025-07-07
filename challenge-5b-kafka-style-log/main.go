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
		StorageMutex: &sync.RWMutex{},
		Storage:      kv,
	}
	n.Handle("send", kafka.SendRPC)
	n.Handle("poll", kafka.PollRPC)
	n.Handle("commit_offsets", kafka.CommitOffsetsRPC)
	n.Handle("list_committed_offsets", kafka.ListCommittedOffsetsRPC)
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}

}
