package types

import "time"

type Worker struct {
	ID        string
	LastSeen  time.Time
	Alive     bool
}
