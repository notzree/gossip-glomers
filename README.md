<div align = "center">
<pre>
  ____  ___  ____ ____ ___ ____     ____ _     ___  __  __ _____ ____  ____
 / ___|/ _ \/ ___/ ___|_ _|  _ \   / ___| |   / _ \|  \/  | ____|  _ \/ ___|
| |  _| | | \___ \___ \| || |_) | | |  _| |  | | | | |\/| |  _| | |_) \___ \
 | |_| | |_| |___) |__) | ||  __/  | |_| | |__| |_| | |  | | |___|  _ < ___) |
 \____|\___/|____/____/___|_|      \____|_____\___/|_|  |_|_____|_| \_\____/

  -------------------------------------------------------
  My solution to Fly.io's [Distributed Systems Challenge](https://fly.io/dist-sys/)
</pre>
</div>

## [Challenge 1] Echo 
Nothing much to explain about this one. Just ack the message

## [Challenge 2] Unique ID Generator
[solution](https://github.com/notzree/gossip-glomers/blob/main/challenge-2-uuid-generation/) \
Also a pretty straightforward task, I just generated a uuid for each request.

## [Challenge 3a and 3b] Single / Multi node Broadcast
[3a solution](https://github.com/notzree/gossip-glomers/blob/main/challenge-3a-broadcast/) \
[3b solution](https://github.com/notzree/gossip-glomers/blob/main/challenge-3b-broadcast/) \
My solution used parts of the unique id generator to ensure that messages were not infinitely broadcasted. \
On each broadcast receive, I would simply send call the broadcast rpc to all neighbourign nodes.

## [Challenge 3c] Fault Tolerant Broadcast 
[3c solution (same as 3d)](https://github.com/notzree/gossip-glomers/blob/main/challenge-3d-broadcast/) \
In this solution, I changed the broadcast mechanism to spawn a goroutine which would continuously try to broadcast until the message was acked.
This would allow the goroutine to wait until the network partition healed to send the broadcast.

## [Challenge 3d] Efficient Broadcast 1
[3d solution](https://github.com/notzree/gossip-glomers/blob/main/challenge-3d-broadcast/) \
We are told that each message has a 100 ms delay. If we want to keep the median message time under 400 ms, this means that at most, a message should only be propogated between 4 nodes ( 300ms + other compute time).
Since the default network topology is a 2d 5x5 grid, it means that messages sent on this network would incur a minimum 400ms network travel time. 
In order to optimize the latency, I utilized a flat star graph:
<img width="1320" alt="image" src="https://github.com/user-attachments/assets/6aba36db-c18a-4807-9e49-5672ac05c51a">
Where each node is connected to some central node n0
This means to broadcast a message to any node, it would require n1 -> n0 -> n2 which is 200ms of latency
<br/>
Another optimization I made was to introduce a TTL to ensure that a message can only be propogated max x times, so bugs or unexpected behaviours won't lead to infinite broadcasting.

### Performance Results
Challenge requirements:
- Message per op under 30
- Median latency under 400ms
-  Maximum latency under 600ms


My results:
- Message per op: 24
- Median latency: 175ms
- Maximum latency: 208ms


## [Challege 3e] Efficient Broadcast 2
[3e solution](https://github.com/notzree/gossip-glomers/blob/main/challenge-3e-broadcast/) \
To further optimize this, I reduced the number of times I sent broadcast messages by using arrays to batch process them. Broadcasts would get added to a queue,
and every second a goroutine would continue to propogate a batch RPC broadcast request.

### Performance Results:
Challenge requirements:
- Message per op under 20
- Median latency under 1000ms
-  Maximum latency under 2000ms

My results:
- Message per op: 2
- Median latency: 462ms
- Maximum latency: 1037ms


## [Challenge 4] Grow only counter

[4 solution](https://github.com/notzree/gossip-glomers/blob/main/challenge-4-grow-only-counter/) \
To implement an eventually consistent grow-only counter, I implemented a synchronization-on-read approach. 
This means that when you incremenet the counter on a node, it only incremeents it's local counter. However, when you attempt to read from a Node n1, n1 will use an rpc call to sum the values of all the local counters to create the global counter.

//todo: implement with op crdt state crdt


