package main

import (
	"os"
	"time"

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

	go node.StartServer(address)

	// Wait a bit before starting election
	time.Sleep(2 * time.Second)

	node.StartElection()

	select {} // keep process alive
}
