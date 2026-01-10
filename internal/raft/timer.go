package raft

import (
	"math/rand"
	"time"
)

// randomElectionTimeout returns a randomized duration
// between 150ms and 300ms
func randomElectionTimeout() time.Duration {
	return time.Duration(150+rand.Intn(150)) * time.Millisecond
}
