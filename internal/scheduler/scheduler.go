package scheduler

import (
	"errors"
	"sync"
	"sync/atomic"
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
	Stats Stats
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

	atomic.AddInt64(&s.Stats.TasksSubmitted, 1)
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

			atomic.AddInt64(&s.Stats.TasksAssigned, 1)

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

	atomic.AddInt64(&s.Stats.TasksCompleted, 1)

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
		atomic.AddInt64(&s.Stats.StaleFailures, 1)
		logger.Error(
			"Rejected failure of task %s: stale epoch",
			taskID,
		)
		return false
	}

	task.RetryCount++
	task.UpdatedAt = time.Now()
	atomic.AddInt64(&s.Stats.TasksFailed, 1)
	// retries exhausted → terminal failure
	if task.RetryCount > task.MaxRetries {
		task.Status = types.TaskFailed
		atomic.AddInt64(&s.Stats.TasksRetried, 1)
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

			atomic.AddInt64(&s.Stats.TasksRequeued, 1)

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
			"Task %s | status=%s | worker=%s | retries=%d/%d | nextRun=%v | epoch=%d",
			t.ID,
			t.Status,
			t.WorkerID,
			t.RetryCount,
			t.MaxRetries,
			t.NextRunAt,
			t.Epoch,
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

func (s *Scheduler) GetStats() Stats {
	return Stats{
		LeaderElections: atomic.LoadInt64(&s.Stats.LeaderElections),

		TasksSubmitted: atomic.LoadInt64(&s.Stats.TasksSubmitted),
		TasksAssigned:  atomic.LoadInt64(&s.Stats.TasksAssigned),
		TasksCompleted: atomic.LoadInt64(&s.Stats.TasksCompleted),
		TasksFailed:    atomic.LoadInt64(&s.Stats.TasksFailed),
		TasksRetried:   atomic.LoadInt64(&s.Stats.TasksRetried),
		TasksRequeued:  atomic.LoadInt64(&s.Stats.TasksRequeued),

		StaleCompletions: atomic.LoadInt64(&s.Stats.StaleCompletions),
		StaleFailures:    atomic.LoadInt64(&s.Stats.StaleFailures),
	}
}
