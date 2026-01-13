package scheduler

import (
	"errors"
	"sync"
	"time"

	"distributed-task-scheduler/internal/election"
	"distributed-task-scheduler/internal/types"
	"distributed-task-scheduler/pkg/logger"
)

type Scheduler struct {
	mu      sync.Mutex
	tasks   map[string]*types.Task
	workers map[string]*types.Worker

	nodeID   string
	election *election.DBElection

	epoch int64
}

func NewScheduler(nodeID string, election *election.DBElection) *Scheduler {
	return &Scheduler{
		tasks:    make(map[string]*types.Task),
		workers:  make(map[string]*types.Worker),
		nodeID:   nodeID,
		election: election,
	}
}

func (s *Scheduler) AddTask(task *types.Task) {
	if !s.IsLeader() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task.Status = types.TaskPending
	task.RetryCount = 0
	task.MaxRetries = 3
	task.NextRunAt = time.Now()
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	s.tasks[task.ID] = task
}

func (s *Scheduler) GetNextTask(workerID string) (*types.Task, error) {
	if !s.IsLeader() {
		return nil, errors.New("not leader")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	worker, ok := s.workers[workerID]
	if !ok || !worker.Alive {
		return nil, errors.New("worker not alive")
	}

	for _, task := range s.tasks {
		if task.Status == types.TaskPending && time.Now().After(task.NextRunAt) {
			task.Status = types.TaskRunning
			task.WorkerID = workerID
			task.Epoch = s.epoch
			task.UpdatedAt = time.Now()

			logger.Info("Assigned task %s to worker %s", task.ID, workerID)
			return task, nil
		}
	}

	return nil, errors.New("no pending tasks")
}

func (s *Scheduler) CompleteTask(taskID string, epoch int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		logger.Error("Completion for unknown task %s", taskID)
		return false
	}

	if task.Epoch != epoch {
		logger.Error(
			"Rejected completion of task %s: stale epoch (got=%d, expected=%d)",
			taskID, epoch, task.Epoch,
		)
		return false
	}

	task.Status = types.TaskDone
	task.UpdatedAt = time.Now()

	logger.Info(
		"Task %s marked DONE by epoch %d",
		taskID, epoch,
	)

	return true
}

func (s *Scheduler) FailTask(taskID string, epoch int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return false
	}

	// fencing
	if task.Epoch != epoch {
		logger.Error(
			"Rejected failure of task %s: stale epoch",
			taskID,
		)
		return false
	}

	task.RetryCount++
	task.UpdatedAt = time.Now()

	// retries exhausted → terminal failure
	if task.RetryCount > task.MaxRetries {
		task.Status = types.TaskFailed
		logger.Error(
			"Task %s permanently FAILED after %d retries",
			task.ID, task.RetryCount-1,
		)
		return true
	}

	// schedule retry
	delay := backoffDuration(task.RetryCount)
	task.NextRunAt = time.Now().Add(delay)
	task.Status = types.TaskPending
	task.WorkerID = ""

	logger.Info(
		"Task %s failed, retry %d/%d scheduled after %v",
		task.ID,
		task.RetryCount,
		task.MaxRetries,
		delay,
	)

	return true
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

			s.ReassignTasksFromDeadWorker(w.ID)
		}
	}
}

func (s *Scheduler) ReassignTasksFromDeadWorker(workerID string) {
	for _, task := range s.tasks {
		if task.WorkerID == workerID && task.Status == types.TaskRunning {
			task.Status = types.TaskPending
			task.WorkerID = ""
			task.UpdatedAt = time.Now()

			logger.Info(
				"Task %s re-queued due to worker %s failure",
				task.ID,
				workerID,
			)
		}
	}
}

func (s *Scheduler) DumpTasks() {
	logger.Info("This is analytics")
	for _, t := range s.tasks {
		logger.Info(
			"Task %s | status=%s | worker=%s",
			t.ID,
			t.Status,
			t.WorkerID,
		)
	}
}

// TryBecomeLeader attempts to acquire or renew leadership.
// Returns true if this node is the leader.
func (s *Scheduler) TryBecomeLeader() bool {
	ok, epoch, _ := s.election.TryAcquire(s.nodeID)
	if ok {
		s.epoch = epoch
	}
	return ok
}

func (s *Scheduler) CurrentEpoch() int64 {
	return s.epoch
}

func (s *Scheduler) IsLeader() bool {
	ok, _ := s.election.IsLeader(s.nodeID)
	return ok
}

func backoffDuration(retry int) time.Duration {
	return time.Duration(1<<retry) * time.Second
}
