package main

import (
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	ss := NewSimpleStorage()
	KV := NewKVStore(n, ss)
	n.Handle("txn", KV.TxnRPC)
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}

}
