package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

func Init(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS leader_leases (
		name TEXT PRIMARY KEY,
		holder_id TEXT,
		lease_until INTEGER
	);
	`

	_, err = db.Exec(schema)
	if err != nil {
		return nil, err
	}

	return db, nil
}