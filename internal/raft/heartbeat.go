package raft

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// startHeartbeats sends periodic heartbeats if leader
func (n *Node) startHeartbeats() {
	ticker := time.NewTicker(100 * time.Millisecond)

	for range ticker.C {
		n.mu.Lock()
		if n.State != Leader {
			n.mu.Unlock()
			return
		}

		term := n.CurrentTerm
		n.mu.Unlock()

		for _, peer := range n.Peers {
			go func(peer string) {
				req := AppendEntriesRequest{
					Term:     term,
					LeaderID: n.ID,
				}

				data, _ := json.Marshal(req)
				http.Post(peer+"/append-entries", "application/json", bytes.NewBuffer(data))
			}(peer)
		}
	}
}
