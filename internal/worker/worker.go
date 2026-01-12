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

		logger.Info(
			"Worker %s executing task %s (epoch=%d)",
			w.ID, task.ID, task.Epoch,
		)

		// pass epoch forward
		w.execute(task.ID, task.Epoch)
	}
}

func (w *Worker) execute(taskID string, epoch int64) {
	// Simulate work
	time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)

	// Random success/failure
	if rand.Intn(10) < 8 {
		w.Scheduler.CompleteTask(taskID, epoch)
		logger.Info(
			"Worker %s completed task %s (epoch=%d)",
			w.ID, taskID, epoch,
		)
	} else {
		w.Scheduler.FailTask(taskID, epoch)
		logger.Error(
			"Worker %s failed task %s (epoch=%d)",
			w.ID, taskID, epoch,
		)
	}
}
