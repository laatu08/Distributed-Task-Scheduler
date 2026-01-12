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

func (e *DBElection) TryAcquire(nodeID string) (bool, error) {
	now := time.Now().Unix()
	expires := time.Now().Add(e.leaseTTL).Unix()

	tx, err := e.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// 1️⃣ Try to UPDATE existing lease
	res, err := tx.Exec(`
		UPDATE leader_leases
		SET holder_id = ?, lease_until = ?
		WHERE name = ?
		  AND (lease_until < ? OR holder_id = ?)
	`, nodeID, expires, e.leaseName, now, nodeID)
	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	// ✅ Successfully acquired or renewed
	if affected == 1 {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return true, nil
	}

	// 2️⃣ Try to INSERT only if lease does not exist
	res, err = tx.Exec(`
		INSERT OR IGNORE INTO leader_leases(name, holder_id, lease_until)
		VALUES (?, ?, ?)
	`, e.leaseName, nodeID, expires)
	if err != nil {
		return false, err
	}

	affected, err = res.RowsAffected()
	if err != nil {
		return false, err
	}

	// ✅ Inserted → leader
	if affected == 1 {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return true, nil
	}

	// ❌ Someone else holds the lease
	return false, nil
}


func (e *DBElection) IsLeader(nodeID string) (bool, error) {
	var holder string
	var until int64

	err := e.db.QueryRow(`
		SELECT holder_id, lease_until
		FROM leader_leases
		WHERE name = ?
	`, e.leaseName).Scan(&holder, &until)

	if err != nil {
		return false, err
	}

	return holder == nodeID && time.Now().Unix() < until, nil
}
