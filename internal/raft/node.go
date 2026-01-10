package raft

import (
	"sync"
	"time"
)

type State string

const (
	Follower  State = "follower"
	Candidate State = "candidate"
	Leader    State = "leader"
)

type Node struct {
	mu sync.Mutex

	ID    string
	Peers []string
	State State

	CurrentTerm int
	VotedFor    string
	LeaderID    string

	// Channel used to reset election timer
	electionResetEvent time.Time
}

// resetElectionTimer records the time of last heartbeat or vote
func (n *Node) ResetElectionTimer() {
	n.electionResetEvent = time.Now()
}
