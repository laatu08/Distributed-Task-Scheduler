package scheduler

import (
	"sync"
	"time"

	"distributed-task-scheduler/internal/types"
)

type Scheduler struct {
	mu      sync.Mutex
	tasks   map[string]*types.Task
	workers map[string]*types.Worker
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks:   make(map[string]*types.Task),
		workers: make(map[string]*types.Worker),
	}
}

func (s *Scheduler) AddTask(task *types.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.Status = types.TaskPending
	task.CreatedAt = time.Now()
	s.tasks[task.ID] = task
}
