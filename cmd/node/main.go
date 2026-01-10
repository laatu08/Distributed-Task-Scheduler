package main

import (
	"os"
	"distributed-scheduler/internal/raft"
)

func main() {
	id := os.Args[1]
	address := os.Args[2]
	peers := os.Args[3:]

	node := &raft.Node{
		ID:          id,
		Peers:      peers,
		State:       raft.Follower,
		CurrentTerm: 0,
	}

	node.ResetElectionTimer()

	go node.StartServer(address)
	go node.RunElectionTimer()

	select {}
}
