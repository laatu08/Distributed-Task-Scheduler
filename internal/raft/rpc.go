package raft

// RequestVoteRequest is sent by candidates to request votes
type RequestVoteRequest struct {
	Term        int    // Candidate's term
	CandidateID string // Candidate requesting vote
}

// RequestVoteResponse is the reply to RequestVote
type RequestVoteResponse struct {
	Term        int  // Receiver's term
	VoteGranted bool // True if vote granted
}

// AppendEntriesRequest is used for heartbeats (no logs yet)
type AppendEntriesRequest struct {
	Term     int    // Leader's term
	LeaderID string // Leader ID
}

// AppendEntriesResponse is the reply to heartbeat
type AppendEntriesResponse struct {
	Term    int  // Receiver's term
	Success bool // Always true for heartbeat
}
