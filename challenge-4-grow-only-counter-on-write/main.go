package main

import (
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	counter := NewCounter(n)
	n.Handle("init", counter.Init)
	n.Handle("add", counter.Add)
	n.Handle("read", counter.Read)
	n.Handle("sync", counter.Sync)
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}

}
