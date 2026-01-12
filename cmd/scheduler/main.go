package main

import (
	"encoding/json"
	"net/http"
	"time"

	"distributed-task-scheduler/internal/scheduler"
	"distributed-task-scheduler/internal/types"
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
	http.HandleFunc("/debug/tasks", func(w http.ResponseWriter, _ *http.Request) {
		sched.DumpTasks()
	})
	http.HandleFunc("/task/submit", submitTaskHandler)
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

func submitTaskHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      string `json:"id"`
		Payload string `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	task := &types.Task{
		ID:      req.ID,
		Payload: req.Payload,
	}

	sched.AddTask(task)

	logger.Info("Task submitted: %s", task.ID)
	w.WriteHeader(http.StatusCreated)
}
