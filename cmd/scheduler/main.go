package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"time"

	"distributed-task-scheduler/internal/db"
	"distributed-task-scheduler/internal/election"
	"distributed-task-scheduler/internal/scheduler"
	"distributed-task-scheduler/internal/types"
	"distributed-task-scheduler/pkg/logger"
)

var sched *scheduler.Scheduler
var nodeID string
var port string

func main() {
	// ---- CLI flags ----
	flag.StringVar(&nodeID, "id", "", "scheduler node ID")
	flag.StringVar(&port, "port", "8080", "http port")
	flag.Parse()

	if nodeID == "" {
		logger.Error("node ID is required. Use --id=<node-id>")
		return
	}

	logger.Info("Starting scheduler server: %s", nodeID)

	// ---- Init scheduler with node ID ----
	dbConn, err := db.Init("leader.db")
	if err != nil {
		logger.Error("DB init failed: %v", err)
		return
	}

	election := election.NewDBElection(
		dbConn,
		"scheduler",
		5*time.Second,
	)

	sched = scheduler.NewScheduler(nodeID, election)

	// ---- Background loops ----
	go reapLoop()
	go leaderLoop()

	// ---- HTTP endpoints ----
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/heartbeat", heartbeatHandler)
	http.HandleFunc("/task", taskHandler)
	http.HandleFunc("/task/submit", submitTaskHandler)
	http.HandleFunc("/task/complete", taskCompleteHandler)

	http.HandleFunc("/debug/tasks", func(w http.ResponseWriter, _ *http.Request) {
		sched.DumpTasks()
	})

	addr := ":" + port
	logger.Info("Scheduler %s listening on %s", nodeID, addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Error("HTTP server failed: %v", err)
	}
}

/* -------------------- Leader Election -------------------- */

// func leaderLoop() {
// 	ticker := time.NewTicker(2 * time.Second)
// 	defer ticker.Stop()

// 	for range ticker.C {
// 		if sched.TryBecomeLeader() {
// 			logger.Info("Node %s is LEADER", nodeID)
// 		}
// 	}
// }

func leaderLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	wasLeader := false

	for range ticker.C {
		isLeader := sched.TryBecomeLeader()

		if isLeader && !wasLeader {
			logger.Info("Node %s became LEADER", nodeID)
		}

		if !isLeader && wasLeader {
			logger.Info("Node %s lost leadership", nodeID)
		}

		wasLeader = isLeader
	}
}

/* -------------------- Handlers -------------------- */

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if !sched.IsLeader() {
		http.Error(w, "not leader", http.StatusForbidden)
		return
	}

	var req struct{ WorkerID string }
	_ = json.NewDecoder(r.Body).Decode(&req)

	sched.RegisterWorker(req.WorkerID)
	w.WriteHeader(http.StatusOK)
}


func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if !sched.IsLeader() {
		http.Error(w, "not leader", http.StatusForbidden)
		return
	}

	var req struct{ WorkerID string }
	_ = json.NewDecoder(r.Body).Decode(&req)

	sched.Heartbeat(req.WorkerID)
	w.WriteHeader(http.StatusOK)
}


func taskHandler(w http.ResponseWriter, r *http.Request) {
	if !sched.IsLeader() {
		http.Error(w, "not leader", http.StatusForbidden)
		return
	}

	workerID := r.URL.Query().Get("worker_id")

	task, err := sched.GetNextTask(workerID)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	json.NewEncoder(w).Encode(task)
}


func submitTaskHandler(w http.ResponseWriter, r *http.Request) {
	if !sched.IsLeader() {
		http.Error(w, "not leader", http.StatusForbidden)
		return
	}

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

/* -------------------- Background -------------------- */

func reapLoop() {
	for {
		time.Sleep(5 * time.Second)
		sched.ReapDeadWorkers(10 * time.Second)
	}
}


func taskCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if !sched.IsLeader() {
		http.Error(w, "not leader", http.StatusForbidden)
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
		Epoch  int64  `json:"epoch"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ok := sched.CompleteTask(req.TaskID, req.Epoch)
	if !ok {
		// stale epoch or invalid task
		http.Error(w, "stale or invalid task completion", http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusOK)
}
