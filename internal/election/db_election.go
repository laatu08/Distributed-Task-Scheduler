package election

import (
	"database/sql"
	"time"
)

type DBElection struct {
	db          *sql.DB
	leaseName  string
	leaseTTL   time.Duration
}

func NewDBElection(db *sql.DB, leaseName string, ttl time.Duration) *DBElection {
	return &DBElection{
		db:         db,
		leaseName: leaseName,
		leaseTTL:  ttl,
	}
}
