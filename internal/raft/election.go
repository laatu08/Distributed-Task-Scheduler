package raft

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// startElection is called when election timeout expires
func (n *Node) StartElection() {
	n.mu.Lock()

	n.State = Candidate
	n.CurrentTerm++
	n.VotedFor = n.ID

	term := n.CurrentTerm
	n.mu.Unlock()

	log.Printf("[%s] Starting election for term %d", n.ID, term)

	votes := 1 // self vote
	majority := (len(n.Peers)+1)/2 + 1

	// Send RequestVote to all peers
	for _, peer := range n.Peers {
		go func(peer string) {
			req := RequestVoteRequest{
				Term:        term,
				CandidateID: n.ID,
			}

			data, _ := json.Marshal(req)
			resp, err := http.Post(peer+"/request-vote", "application/json", bytes.NewBuffer(data))
			if err != nil {
				return
			}
			defer resp.Body.Close()

			var voteResp RequestVoteResponse
			json.NewDecoder(resp.Body).Decode(&voteResp)

			n.mu.Lock()
			defer n.mu.Unlock()

			// Step down if higher term discovered
			if voteResp.Term > n.CurrentTerm {
				n.CurrentTerm = voteResp.Term
				n.State = Follower
				n.VotedFor = ""
				return
			}

			if voteResp.VoteGranted && n.State == Candidate {
				votes++
				if votes >= majority {
					n.State = Leader
					n.LeaderID = n.ID
					n.electionResetEvent = time.Now()
					log.Printf("[%s] Became LEADER for term %d", n.ID, n.CurrentTerm)

					go n.startHeartbeats()
				}
			}
		}(peer)
	}
}
