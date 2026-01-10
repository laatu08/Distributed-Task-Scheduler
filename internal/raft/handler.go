package raft

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// handleRequestVote processes incoming vote requests
func (n *Node) handleRequestVote(w http.ResponseWriter, r *http.Request) {
	var req RequestVoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	resp := RequestVoteResponse{
		Term:        n.CurrentTerm,
		VoteGranted: false,
	}

	// If candidate's term is older, reject vote
	if req.Term < n.CurrentTerm {
		log.Printf("[%s] Rejecting vote for %s (stale term)", n.ID, req.CandidateID)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// If term is newer, update our term and reset vote
	if req.Term > n.CurrentTerm {
		n.CurrentTerm = req.Term
		n.VotedFor = ""
		n.State = Follower
	}

	// Grant vote if not voted yet or voted for same candidate
	if n.VotedFor == "" || n.VotedFor == req.CandidateID {
		n.VotedFor = req.CandidateID
		resp.VoteGranted = true
		resp.Term = n.CurrentTerm

		log.Printf("[%s] Voted for %s (term %d)", n.ID, req.CandidateID, n.CurrentTerm)
	}

	json.NewEncoder(w).Encode(resp)
}


// handleAppendEntries handles leader heartbeats
func (n *Node) handleAppendEntries(w http.ResponseWriter, r *http.Request) {
	var req AppendEntriesRequest
	json.NewDecoder(r.Body).Decode(&req)

	n.mu.Lock()
	defer n.mu.Unlock()

	resp := AppendEntriesResponse{
		Term:    n.CurrentTerm,
		Success: false,
	}

	// Reject stale leader
	if req.Term < n.CurrentTerm {
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Accept leader
	n.State = Follower
	n.CurrentTerm = req.Term
	n.LeaderID = req.LeaderID
	n.VotedFor = ""
	n.electionResetEvent = time.Now()

	resp.Success = true
	resp.Term = n.CurrentTerm

	json.NewEncoder(w).Encode(resp)
}
