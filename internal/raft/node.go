package raft

import "sync"

// State represents the role of a Raft node
type State string

const (
	Follower  State = "follower"
	Candidate State = "candidate"
	Leader    State = "leader"
)

// Node represents a single Raft node
type Node struct {
	// Mutex protects concurrent access
	mu sync.Mutex

	// Unique ID of this node
	ID string

	// List of peer addresses (host:port)
	Peers []string

	// Current role of the node
	State State

	// Latest term this node has seen
	CurrentTerm int

	// Candidate ID that this node voted for in current term
	VotedFor string

	// Known leader ID
	LeaderID string
}
