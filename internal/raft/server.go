package raft

import (
	"log"
	"net/http"
)

// StartServer starts the HTTP server for this node
func (n *Node) StartServer(address string) {
	http.HandleFunc("/request-vote", n.handleRequestVote)

	log.Printf("[%s] Listening on %s", n.ID, address)

	// Blocking call — server runs forever
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatal(err)
	}
}
