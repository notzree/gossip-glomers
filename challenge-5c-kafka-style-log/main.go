package main

import (
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	kafka := NewKafka(n)
	n.Handle("send", kafka.SendRPC)
	n.Handle("poll", kafka.PollRPC)
	n.Handle("commit_offsets", kafka.CommitOffsetsRPC)
	n.Handle("list_committed_offsets", kafka.ListCommittedOffsetsRPC)
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}

}
