package types

import "time"

type TaskStatus string

const (
	TaskPending TaskStatus = "PENDING"
	TaskRunning TaskStatus = "RUNNING"
	TaskDone    TaskStatus = "DONE"
	TaskFailed  TaskStatus = "FAILED"
)

type Task struct {
	ID        string
	Payload   string
	Status    TaskStatus
	WorkerID  string
	Epoch     int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
