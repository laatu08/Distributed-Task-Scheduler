package main

import (
	"time"

	"distributed-task-scheduler/internal/scheduler"
	"distributed-task-scheduler/internal/types"
	"distributed-task-scheduler/internal/worker"
	"distributed-task-scheduler/pkg/logger"
)

func main() {
	logger.Info("Starting single-node scheduler")

	s := scheduler.NewScheduler()

	// Add tasks
	for i := 1; i <= 10; i++ {
		s.AddTask(&types.Task{
			ID:      "task-" + string(rune('A'+i)),
			Payload: "demo-task",
		})
	}

	// Start workers
	w1 := worker.NewWorker("worker-1", s)
	w2 := worker.NewWorker("worker-2", s)

	go w1.Start()
	go w2.Start()

	for {
		time.Sleep(10 * time.Second)
	}
}
