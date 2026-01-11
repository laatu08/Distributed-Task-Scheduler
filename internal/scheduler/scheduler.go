package scheduler

import (
	"errors"
	"sync"
	"time"

	"distributed-task-scheduler/internal/types"
	"distributed-task-scheduler/pkg/logger"
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
	task.UpdatedAt = time.Now()
	s.tasks[task.ID] = task
}

func (s *Scheduler) GetNextTask(workerID string) (*types.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.tasks {
		if task.Status == types.TaskPending {
			task.Status = types.TaskRunning
			task.WorkerID = workerID
			task.UpdatedAt = time.Now()
			return task, nil
		}
	}

	return nil, errors.New("no pending tasks")
}

func (s *Scheduler) CompleteTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, ok := s.tasks[taskID]; ok {
		task.Status = types.TaskDone
		task.UpdatedAt = time.Now()
	}
}

func (s *Scheduler) FailTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, ok := s.tasks[taskID]; ok {
		task.Status = types.TaskFailed
		task.UpdatedAt = time.Now()
	}
}


func (s *Scheduler) RegisterWorker(workerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.workers[workerID] = &types.Worker{
		ID:       workerID,
		LastSeen: time.Now(),
		Alive:    true,
	}

	logger.Info("Worker registered: %s", workerID)
}

func (s *Scheduler) Heartbeat(workerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if w, ok := s.workers[workerID]; ok {
		w.LastSeen = time.Now()
		if !w.Alive {
			logger.Info("Worker revived: %s", workerID)
		}
		w.Alive = true
	}
}

func (s *Scheduler) ReapDeadWorkers(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, w := range s.workers {
		if w.Alive && now.Sub(w.LastSeen) > timeout {
			w.Alive = false
			logger.Error("Worker DEAD: %s", w.ID)
		}
	}
}

