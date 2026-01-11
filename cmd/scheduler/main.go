package main

import (
	"encoding/json"
	"net/http"
	"time"

	"distributed-task-scheduler/internal/scheduler"
	"distributed-task-scheduler/pkg/logger"
)

var sched *scheduler.Scheduler

func main() {
	logger.Info("Starting scheduler server")

	sched = scheduler.NewScheduler()

	go reapLoop()

	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/heartbeat", heartbeatHandler)
	http.HandleFunc("/task", taskHandler)

	http.ListenAndServe(":8080", nil)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ WorkerID string }
	json.NewDecoder(r.Body).Decode(&req)

	sched.RegisterWorker(req.WorkerID)
	w.WriteHeader(http.StatusOK)
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ WorkerID string }
	json.NewDecoder(r.Body).Decode(&req)

	sched.Heartbeat(req.WorkerID)
	w.WriteHeader(http.StatusOK)
}

func taskHandler(w http.ResponseWriter, r *http.Request) {
	workerID := r.URL.Query().Get("worker_id")

	task, err := sched.GetNextTask(workerID)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	json.NewEncoder(w).Encode(task)
}

func reapLoop() {
	for {
		time.Sleep(5 * time.Second)
		sched.ReapDeadWorkers(10 * time.Second)
	}
}
