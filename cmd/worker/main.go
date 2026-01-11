package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"distributed-task-scheduler/pkg/logger"
	"github.com/google/uuid"
)

const schedulerURL = "http://localhost:8080"

func main() {
	workerID := uuid.New().String()

	register(workerID)

	go heartbeatLoop(workerID)

	for {
		fetchAndExecute(workerID)
		time.Sleep(1 * time.Second)
	}
}

func register(workerID string) {
	body, _ := json.Marshal(map[string]string{
		"WorkerID": workerID,
	})
	http.Post(schedulerURL+"/register", "application/json", bytes.NewBuffer(body))
}

func heartbeatLoop(workerID string) {
	for {
		body, _ := json.Marshal(map[string]string{
			"WorkerID": workerID,
		})
		http.Post(schedulerURL+"/heartbeat", "application/json", bytes.NewBuffer(body))
		time.Sleep(3 * time.Second)
	}
}

func fetchAndExecute(workerID string) {
	resp, err := http.Get(schedulerURL + "/task?worker_id=" + workerID)
	if err != nil || resp.StatusCode == http.StatusNoContent {
		return
	}

	logger.Info("Worker %s executing task", workerID)
	time.Sleep(2 * time.Second)
}
