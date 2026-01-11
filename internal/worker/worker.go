package worker

import (
	"math/rand"
	"time"

	"distributed-task-scheduler/internal/scheduler"
	"distributed-task-scheduler/pkg/logger"
)

type Worker struct {
	ID        string
	Scheduler *scheduler.Scheduler
}

func NewWorker(id string, s *scheduler.Scheduler) *Worker {
	return &Worker{
		ID:        id,
		Scheduler: s,
	}
}

func (w *Worker) Start() {
	logger.Info("Worker %s started", w.ID)

	for {
		task, err := w.Scheduler.GetNextTask(w.ID)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		logger.Info("Worker %s executing task %s", w.ID, task.ID)
		w.execute(task.ID)
	}
}

func (w *Worker) execute(taskID string) {
	// Simulate work
	time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)

	// Random success/failure
	if rand.Intn(10) < 8 {
		w.Scheduler.CompleteTask(taskID)
		logger.Info("Worker %s completed task %s", w.ID, taskID)
	} else {
		w.Scheduler.FailTask(taskID)
		logger.Error("Worker %s failed task %s", w.ID, taskID)
	}
}
