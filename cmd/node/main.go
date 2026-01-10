package main

import (
	"log"

	"distributed-scheduler/internal/raft"
)

func main() {
	// Create a single node (no peers yet)
	node := &raft.Node{
		ID:          "node1",
		Peers:      []string{},
		State:       raft.Follower,
		CurrentTerm: 0,
	}

	log.Printf("[%s] Node started as %s", node.ID, node.State)

	// Simulate election trigger
	node.startElection()
}
