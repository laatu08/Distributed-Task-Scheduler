package main

import (
	"distributed-task-scheduler/internal/scheduler"
	"distributed-task-scheduler/pkg/logger"
)

func main() {
	logger.Info("Starting scheduler node")

	s := scheduler.NewScheduler()

	_ = s // temporary
	select {}
}
