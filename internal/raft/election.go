package raft

import "log"

// startElection is called when election timeout expires
func (n *Node) startElection() {
	n.mu.Lock()

	// Become candidate
	n.State = Candidate

	// Increment term for new election
	n.CurrentTerm++

	// Vote for self
	n.VotedFor = n.ID

	currentTerm := n.CurrentTerm

	n.mu.Unlock()

	log.Printf("[%s] Starting election for term %d", n.ID, currentTerm)

	// Vote count starts at 1 (self-vote)
	votes := 1

	// TODO:
	// 1. Send RequestVote RPCs to peers
	// 2. Count votes
	// 3. If majority → become leader
	_ = votes
}
