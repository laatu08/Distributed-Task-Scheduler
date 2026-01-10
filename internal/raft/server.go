package raft

import (
	"log"
	"net/http"
)

// StartServer starts the HTTP server for this node
func (n *Node) StartServer(address string) {
	http.HandleFunc("/request-vote", n.handleRequestVote)
	http.HandleFunc("/append-entries", n.handleAppendEntries)

	log.Printf("[%s] Listening on %s", n.ID, address)
	http.ListenAndServe(address, nil)
}

