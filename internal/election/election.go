package election

import (
	"sync"
	"time"
)

type LeaderElection struct {
	mu          sync.Mutex
	leaderID    string
	leaseUntil  time.Time
	leasePeriod time.Duration
}

func NewLeaderElection(leasePeriod time.Duration) *LeaderElection {
	return &LeaderElection{
		leasePeriod: leasePeriod,
	}
}

func (le *LeaderElection) TryAcquire(nodeID string) bool {
	le.mu.Lock()
	defer le.mu.Unlock()

	now := time.Now()

	// No leader or lease expired
	if le.leaderID == "" || now.After(le.leaseUntil) {
		le.leaderID = nodeID
		le.leaseUntil = now.Add(le.leasePeriod)
		return true
	}

	// Already leader → renew lease
	if le.leaderID == nodeID {
		le.leaseUntil = now.Add(le.leasePeriod)
		return true
	}

	return false
}

func (le *LeaderElection) IsLeader(nodeID string) bool {
	le.mu.Lock()
	defer le.mu.Unlock()

	return le.leaderID == nodeID && time.Now().Before(le.leaseUntil)
}
