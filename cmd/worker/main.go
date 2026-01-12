package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"distributed-task-scheduler/pkg/logger"
	"github.com/google/uuid"
)

var schedulerURLs = []string{
	"http://localhost:8080",
	"http://localhost:8081",
	"http://localhost:8082",
}

var rrIndex = 0

type Task struct {
	ID    string `json:"id"`
	Epoch int64  `json:"epoch"`
}

func main() {
	rand.Seed(time.Now().UnixNano())
	workerID := uuid.New().String()

	for {
		if register(workerID) {
			break
		}
		time.Sleep(2 * time.Second)
	}

	go heartbeatLoop(workerID)

	for {
		fetchAndExecute(workerID)
		time.Sleep(1 * time.Second)
	}
}

func nextScheduler() string {
	url := schedulerURLs[rrIndex]
	rrIndex = (rrIndex + 1) % len(schedulerURLs)
	return url
}

/* ---------------- Registration ---------------- */
func register(workerID string) bool {
	body, _ := json.Marshal(map[string]string{
		"WorkerID": workerID,
	})

	for i := 0; i < len(schedulerURLs); i++ {
		scheduler := nextScheduler()

		resp, err := http.Post(
			scheduler+"/register",
			"application/json",
			bytes.NewBuffer(body),
		)

		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusForbidden {
			continue
		}

		logger.Info("Worker %s registered with %s", workerID, scheduler)
		return true
	}

	return false
}

/* ---------------- Heartbeat ---------------- */
func heartbeatLoop(workerID string) {
	for {
		body, _ := json.Marshal(map[string]string{
			"WorkerID": workerID,
		})

		for i := 0; i < len(schedulerURLs); i++ {
			scheduler := nextScheduler()

			resp, err := http.Post(
				scheduler+"/heartbeat",
				"application/json",
				bytes.NewBuffer(body),
			)

			if err != nil {
				continue
			}

			if resp.StatusCode == http.StatusForbidden {
				continue
			}

			break // heartbeat accepted
		}

		time.Sleep(3 * time.Second)
	}
}

/* ---------------- Task execution ---------------- */

func fetchAndExecute(workerID string) {
	for i := 0; i < len(schedulerURLs); i++ {
		scheduler := nextScheduler()

		resp, err := http.Get(scheduler + "/task?worker_id=" + workerID)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusForbidden ||
			resp.StatusCode == http.StatusNoContent {
			continue
		}

		var task Task
		if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
			continue
		}

		logger.Info(
			"Worker %s executing task %s (epoch=%d) via %s",
			workerID, task.ID, task.Epoch, scheduler,
		)

		time.Sleep(2 * time.Second)

		if rand.Intn(2) == 0 {
			failTask(workerID, task, scheduler)
		} else {
			completeTask(workerID, task, scheduler)
		}
		return
	}
}

/* ---------------- Completion ---------------- */

func completeTask(workerID string, task Task, scheduler string) {
	body, _ := json.Marshal(map[string]any{
		"task_id": task.ID,
		"epoch":   task.Epoch,
	})

	resp, err := http.Post(
		scheduler+"/task/complete",
		"application/json",
		bytes.NewBuffer(body),
	)

	if err != nil {
		return
	}

	if resp.StatusCode == http.StatusForbidden {
		logger.Info(
			"Completion rejected for task %s (stale epoch)",
			task.ID,
		)
		return
	}

	logger.Info(
		"Worker %s completed task %s (epoch=%d)",
		workerID, task.ID, task.Epoch,
	)
}

func failTask(workerID string, task Task, scheduler string) {
	body, _ := json.Marshal(map[string]any{
		"task_id": task.ID,
		"epoch":   task.Epoch,
	})

	resp, err := http.Post(
		scheduler+"/task/fail",
		"application/json",
		bytes.NewBuffer(body),
	)

	if err != nil {
		return
	}

	if resp.StatusCode == http.StatusForbidden {
		logger.Info(
			"Failure rejected for task %s (stale epoch)",
			task.ID,
		)
		return
	}

	logger.Error(
		"Worker %s FAILED task %s (epoch=%d)",
		workerID, task.ID, task.Epoch,
	)
}
