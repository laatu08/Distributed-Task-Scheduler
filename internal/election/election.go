package election

import (
	// "sync"
	"time"
)

// type LeaderElection struct {
// 	mu          sync.Mutex
// 	leaderID    string
// 	leaseUntil  time.Time
// 	leasePeriod time.Duration
// }

// func NewLeaderElection(leasePeriod time.Duration) *LeaderElection {
// 	return &LeaderElection{
// 		leasePeriod: leasePeriod,
// 	}
// }

func (e *DBElection) TryAcquire(nodeID string) (bool, int64, error) {
	now := time.Now().Unix()
	expires := time.Now().Add(e.leaseTTL).Unix()

	tx, err := e.db.Begin()
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()

	// Try to update expired lease OR renew own lease
	res, err := tx.Exec(`
		UPDATE leader_leases
		SET holder_id = ?,
		    lease_until = ?,
		    epoch = CASE
		      WHEN holder_id = ? THEN epoch
		      ELSE epoch + 1
		    END
		WHERE name = ?
		  AND (lease_until < ? OR holder_id = ?)
	`, nodeID, expires, nodeID, e.leaseName, now, nodeID)
	if err != nil {
		return false, 0, err
	}

	affected, _ := res.RowsAffected()
	if affected == 1 {
		var epoch int64
		err := tx.QueryRow(`
			SELECT epoch FROM leader_leases WHERE name = ?
		`, e.leaseName).Scan(&epoch)
		if err != nil {
			return false, 0, err
		}

		if err := tx.Commit(); err != nil {
			return false, 0, err
		}
		return true, epoch, nil
	}

	// Try initial insert
	res, err = tx.Exec(`
		INSERT OR IGNORE INTO leader_leases(name, holder_id, lease_until, epoch)
		VALUES (?, ?, ?, 1)
	`, e.leaseName, nodeID, expires)
	if err != nil {
		return false, 0, err
	}

	affected, _ = res.RowsAffected()
	if affected == 1 {
		if err := tx.Commit(); err != nil {
			return false, 0, err
		}
		return true, 1, nil
	}

	return false, 0, nil
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
