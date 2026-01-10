package raft

import (
	"log"
	"time"
)

// runElectionTimer runs in a loop and triggers elections
func (n *Node) RunElectionTimer() {
	for {
		timeout := randomElectionTimeout()

		n.mu.Lock()
		lastReset := n.electionResetEvent
		n.mu.Unlock()

		time.Sleep(timeout)

		n.mu.Lock()

		// If heartbeat arrived, skip election
		if n.State == Leader || time.Since(lastReset) < timeout {
			n.mu.Unlock()
			continue
		}

		log.Printf("[%s] Election timeout, starting election", n.ID)
		n.mu.Unlock()

		n.StartElection()
	}
}
