# Distributed Task Scheduler

A fault-tolerant, distributed task scheduler written in Go that provides at-most-once task execution guarantees through leader election, epoch-based fencing, and automatic failure recovery.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Key Features](#key-features)
- [System Flow](#system-flow)
- [Components](#components)
- [API Documentation](#api-documentation)
- [Setup & Installation](#setup--installation)
- [Running the System](#running-the-system)
- [Monitoring & Metrics](#monitoring--metrics)
- [Design Decisions](#design-decisions)

## Overview

This system implements a distributed task scheduler with multiple scheduler nodes competing for leadership. Only the leader can assign tasks, preventing duplicate execution. Workers fetch tasks from any scheduler node, but only the leader responds. The system handles:

- **Leader Election**: Database-backed leader election with automatic failover
- **Task Management**: Queue, assign, and track task execution
- **Worker Management**: Registration, heartbeat monitoring, and failure detection
- **Fault Tolerance**: Task reassignment on worker failure, retry with exponential backoff
- **Epoch-Based Fencing**: Prevents stale task completions after leadership changes

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Distributed System                        │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐    │
│  │  Scheduler 1 │   │  Scheduler 2 │   │  Scheduler 3 │    │
│  │  :8080       │   │  :8081       │   │  :8082       │    │
│  │  [LEADER]    │   │  [FOLLOWER]  │   │  [FOLLOWER]  │    │
│  └──────┬───────┘   └──────────────┘   └──────────────┘    │
│         │                                                     │
│         │                                                     │
│         ├─────────────────┐                                  │
│         │                 │                                  │
│    ┌────▼─────┐      ┌───▼──────┐                          │
│    │ SQLite   │      │  Tasks   │                           │
│    │ Leader   │      │  Queue   │                           │
│    │ Lease DB │      │          │                           │
│    └──────────┘      └──────────┘                           │
│                                                               │
│         ▲                                                     │
│         │                                                     │
│    ┌────┴──────────────────────────┐                        │
│    │                                │                        │
│  ┌─▼────────┐  ┌──────────┐  ┌────▼─────┐                 │
│  │ Worker 1 │  │ Worker 2 │  │ Worker 3 │                  │
│  │          │  │          │  │          │                   │
│  └──────────┘  └──────────┘  └──────────┘                  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## Key Features

### 1. **Leader Election**
- Database-backed leader election using SQLite
- Automatic lease renewal (5-second TTL)
- Epoch-based versioning for leadership changes
- Seamless failover when leader crashes

### 2. **Task Management**
- Submit tasks via HTTP API
- Automatic task assignment to healthy workers
- Retry mechanism with exponential backoff (max 3 retries)
- Task states: PENDING → RUNNING → DONE/FAILED

### 3. **Worker Management**
- Dynamic worker registration
- Heartbeat monitoring (3-second interval)
- Dead worker detection (10-second timeout)
- Automatic task reassignment from dead workers

### 4. **Fault Tolerance**
- Epoch-based fencing prevents duplicate execution
- Stale completion/failure rejection
- Task requeuing on worker failure
- Leadership transition safety

### 5. **Observability**
- Real-time metrics endpoint
- Task state debugging
- Comprehensive logging
- Statistics tracking

## System Flow

### Leader Election Flow

```
┌─────────────┐
│  Scheduler  │
│   Starts    │
└──────┬──────┘
       │
       ▼
┌─────────────────────┐
│ Try Acquire Lease   │
│ Every 2 seconds     │
└──────┬──────────────┘
       │
       ├─── Lease Expired or Not Held ───┐
       │                                   │
       ▼                                   ▼
┌──────────────┐              ┌────────────────────┐
│ Acquire &    │              │ Increment Epoch    │
│ Become Leader│              │ New Leadership Era │
└──────┬───────┘              └────────┬───────────┘
       │                               │
       ▼                               ▼
┌──────────────┐              ┌────────────────────┐
│ Handle       │              │ All running tasks  │
│ Requests     │              │ become invalid     │
└──────────────┘              └────────────────────┘
```

### Task Lifecycle Flow

```
┌──────────────┐
│ Task Submit  │
│ POST /task/  │
│    submit    │
└──────┬───────┘
       │
       ▼
┌──────────────┐      ┌───────────────┐
│   PENDING    │─────▶│  NextRunAt    │
│  Retry=0     │      │  Scheduled    │
└──────┬───────┘      └───────────────┘
       │
       │ Worker requests task
       ▼
┌──────────────┐
│   RUNNING    │
│  Epoch=N     │
│  WorkerID=X  │
└──────┬───────┘
       │
       ├─── Success ────┐
       │                │
       │                ▼
       │         ┌─────────────┐
       │         │    DONE     │
       │         └─────────────┘
       │
       ├─── Failure (Retry < Max) ───┐
       │                               │
       │                               ▼
       │                    ┌──────────────────┐
       │                    │    PENDING       │
       │                    │ Retry++          │
       │                    │ NextRunAt = now  │
       │                    │    + backoff     │
       │                    └────────┬─────────┘
       │                             │
       │                             │ Exponential Backoff
       │                             │ 2s, 4s, 8s...
       │                             │
       └─────────────────────────────┘
       │
       └─── Failure (Retry >= Max) ───┐
                                       │
                                       ▼
                               ┌──────────────┐
                               │    FAILED    │
                               │  (Terminal)  │
                               └──────────────┘
```

### Worker Execution Flow

```
┌──────────────┐
│   Worker     │
│   Starts     │
└──────┬───────┘
       │
       ▼
┌──────────────────┐
│ Register with    │
│ Leader           │
│ POST /register   │
└──────┬───────────┘
       │
       ├────────────────────────┐
       │                        │
       ▼                        ▼
┌──────────────────┐    ┌──────────────────┐
│ Heartbeat Loop   │    │ Task Fetch Loop  │
│ Every 3s         │    │ Every 1s         │
│ POST /heartbeat  │    │ GET /task        │
└──────────────────┘    └──────┬───────────┘
                               │
                               ▼
                        ┌──────────────────┐
                        │ Received Task?   │
                        └──────┬───────────┘
                               │
                 ┌─────────────┼─────────────┐
                 │ Yes                        │ No
                 ▼                            ▼
          ┌──────────────┐            ┌──────────────┐
          │ Execute Task │            │ Wait & Retry │
          │ (2s sleep)   │            └──────────────┘
          └──────┬───────┘
                 │
                 ├─── Random Success (50%) ───┐
                 │                             │
                 │                             ▼
                 │                  ┌──────────────────────┐
                 │                  │ POST /task/complete  │
                 │                  │ with TaskID & Epoch  │
                 │                  └──────────────────────┘
                 │
                 └─── Random Failure (50%) ───┐
                                               │
                                               ▼
                                    ┌──────────────────────┐
                                    │ POST /task/fail      │
                                    │ with TaskID & Epoch  │
                                    └──────────────────────┘
```

### Worker Failure Detection Flow

```
┌──────────────────┐
│ Heartbeat Loop   │
│ (Scheduler)      │
│ Every 5s         │
└──────┬───────────┘
       │
       ▼
┌──────────────────────┐
│ Check Last Heartbeat │
│ for Each Worker      │
└──────┬───────────────┘
       │
       ├─── LastSeen > 10s ───┐
       │                       │
       │                       ▼
       │            ┌──────────────────────┐
       │            │ Mark Worker DEAD     │
       │            └──────┬───────────────┘
       │                   │
       │                   ▼
       │            ┌──────────────────────┐
       │            │ Find Running Tasks   │
       │            │ Assigned to Worker   │
       │            └──────┬───────────────┘
       │                   │
       │                   ▼
       │            ┌──────────────────────┐
       │            │ Requeue Tasks        │
       │            │ Status: PENDING      │
       │            │ WorkerID: ""         │
       │            └──────────────────────┘
       │
       ▼
┌─────────���────────┐
│ Continue         │
└──────────────────┘
```

### Epoch-Based Fencing Flow

```
Leadership Change Scenario:

Time  │ Leader │ Epoch │ Task Status        │ Event
──────┼────────┼───────┼────────────────────┼─────────────────
T0    │ Node1  │  5    │ Task-A: PENDING    │
T1    │ Node1  │  5    │ Task-A: RUNNING    │ Worker-X gets task
                       │ Epoch=5            │
T2    │ Node1  │  5    │                    │ Worker-X executing
T3    │ Node2  │  6    │                    │ Node1 crashes!
                                             │ Node2 becomes leader
                                             │ Epoch incremented to 6
T4    │ Node2  │  6    │ Task-A: PENDING    │ Task requeued
T5    │ Node2  │  6    │ Task-A: RUNNING    │ Worker-Y gets task
                       │ Epoch=6            │
T6    │ Node2  │  6    │                    │ Worker-X tries to
                                             │ complete with Epoch=5
                                             │ ❌ REJECTED (stale)
T7    │ Node2  │  6    │ Task-A: DONE       │ Worker-Y completes
                                             │ with Epoch=6 ✅
```

## Components

### 1. Scheduler (`cmd/scheduler/main.go`)

The main scheduler process that:
- Manages task queue and worker registry
- Participates in leader election
- Exposes HTTP API endpoints
- Handles task assignment and completion
- Monitors worker health

**Command-line flags:**
- `--id`: Unique scheduler node identifier (required)
- `--port`: HTTP server port (default: 8080)

### 2. Worker (`cmd/worker/main.go`)

Independent worker processes that:
- Register with scheduler cluster
- Send periodic heartbeats
- Fetch and execute tasks
- Report task completion/failure
- Use round-robin to contact schedulers

### 3. Leader Election (`internal/election/`)

Database-backed leader election mechanism:
- **Lease-based**: 5-second TTL with automatic renewal
- **Epoch tracking**: Increments on leadership change
- **Atomic operations**: Transaction-safe lease acquisition

### 4. Task Scheduler (`internal/scheduler/scheduler.go`)

Core scheduling logic:
- Task queue management
- Worker lifecycle tracking
- Task assignment algorithm
- Retry mechanism with exponential backoff
- Dead worker detection and task reassignment

### 5. Database (`internal/db/db.go`)

SQLite database schema:

```sql
CREATE TABLE leader_leases (
    name TEXT PRIMARY KEY,
    holder_id TEXT,
    lease_until INTEGER,
    epoch INTEGER
);
```

## API Documentation

### Worker Management

#### Register Worker
```http
POST /register
Content-Type: application/json

{
  "WorkerID": "worker-uuid"
}

Response: 200 OK (if leader)
Response: 403 Forbidden (if not leader)
```

#### Send Heartbeat
```http
POST /heartbeat
Content-Type: application/json

{
  "WorkerID": "worker-uuid"
}

Response: 200 OK (if leader)
Response: 403 Forbidden (if not leader)
```

### Task Management

#### Submit Task
```http
POST /task/submit
Content-Type: application/json

{
  "id": "task-123",
  "payload": "task data"
}

Response: 201 Created (if leader)
Response: 403 Forbidden (if not leader)
```

#### Get Next Task
```http
GET /task?worker_id=worker-uuid

Response: 200 OK
{
  "id": "task-123",
  "payload": "task data",
  "epoch": 5
}

Response: 204 No Content (no tasks available)
Response: 403 Forbidden (if not leader)
```

#### Complete Task
```http
POST /task/complete
Content-Type: application/json

{
  "task_id": "task-123",
  "epoch": 5
}

Response: 200 OK (completion accepted)
Response: 403 Forbidden (stale epoch or invalid task)
```

#### Fail Task
```http
POST /task/fail
Content-Type: application/json

{
  "task_id": "task-123",
  "epoch": 5
}

Response: 200 OK (failure recorded, task requeued/failed)
Response: 403 Forbidden (stale epoch or invalid task)
```

### Monitoring

#### Get Metrics
```http
GET /metrics

Response: 200 OK
{
  "LeaderElections": 3,
  "TasksSubmitted": 100,
  "TasksAssigned": 98,
  "TasksCompleted": 85,
  "TasksFailed": 5,
  "TasksRetried": 8,
  "TasksRequeued": 3,
  "StaleCompletions": 2,
  "StaleFailures": 1
}
```

#### Debug Tasks
```http
GET /debug/tasks

Response: 200 OK
(Prints task details to scheduler logs)
```

## Setup & Installation

### Prerequisites

- Go 1.24.4 or higher
- SQLite3

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd distributed-task-scheduler

# Install dependencies
go mod download

# Build binaries
go build -o bin/scheduler cmd/scheduler/main.go
go build -o bin/worker cmd/worker/main.go
```

## Running the System

### Start Scheduler Cluster

Start 3 scheduler nodes on different ports:

```bash
# Terminal 1 - Scheduler Node 1
./bin/scheduler --id=scheduler-1 --port=8080

# Terminal 2 - Scheduler Node 2
./bin/scheduler --id=scheduler-2 --port=8081

# Terminal 3 - Scheduler Node 3
./bin/scheduler --id=scheduler-3 --port=8082
```

### Start Workers

Start multiple worker processes:

```bash
# Terminal 4 - Worker 1
./bin/worker

# Terminal 5 - Worker 2
./bin/worker

# Terminal 6 - Worker 3
./bin/worker
```

### Submit Tasks

Use curl or any HTTP client to submit tasks:

```bash
# Submit a task to the leader
curl -X POST http://localhost:8080/task/submit \
  -H "Content-Type: application/json" \
  -d '{"id":"task-1","payload":"process data"}'

curl -X POST http://localhost:8080/task/submit \
  -H "Content-Type: application/json" \
  -d '{"id":"task-2","payload":"send email"}'

curl -X POST http://localhost:8080/task/submit \
  -H "Content-Type: application/json" \
  -d '{"id":"task-3","payload":"generate report"}'
```

### Check Metrics

```bash
# Get metrics from any scheduler
curl http://localhost:8080/metrics
```

## Monitoring & Metrics

The system tracks the following metrics:

| Metric | Description |
|--------|-------------|
| `LeaderElections` | Number of times leadership changed |
| `TasksSubmitted` | Total tasks submitted to the system |
| `TasksAssigned` | Tasks assigned to workers |
| `TasksCompleted` | Successfully completed tasks |
| `TasksFailed` | Permanently failed tasks (after max retries) |
| `TasksRetried` | Tasks that were retried |
| `TasksRequeued` | Tasks reassigned due to worker failure |
| `StaleCompletions` | Rejected completions due to epoch mismatch |
| `StaleFailures` | Rejected failures due to epoch mismatch |

## Design Decisions

### 1. **SQLite for Leader Election**

**Why:** Simple, file-based, supports atomic operations via transactions, no external dependencies.

**Trade-off:** Single point of contention, not suitable for geographically distributed systems.

### 2. **Epoch-Based Fencing**

**Why:** Prevents duplicate task execution after leadership changes. Stale workers cannot complete tasks from previous epochs.

**How:** Each leadership change increments the epoch. Tasks carry the epoch when assigned. Completions/failures must match the current epoch.

### 3. **Exponential Backoff for Retries**

**Why:** Prevents overwhelming the system with repeated failures. Gives transient issues time to resolve.

**Formula:** `2^retry seconds` (2s, 4s, 8s for retries 1, 2, 3)

### 4. **At-Most-Once Execution**

**Why:** Critical for idempotency. A task should not be executed multiple times under normal conditions.

**Guarantees:**
- Epoch fencing prevents stale completions
- Only leader assigns tasks
- Dead worker detection triggers reassignment

### 5. **Push vs Pull Model**

**Chosen:** Pull model (workers fetch tasks)

**Why:**
- Workers control their load
- Simpler worker lifecycle management
- No need for worker addressing/routing
- Natural backpressure mechanism

### 6. **Heartbeat Intervals**

- **Worker heartbeat:** 3 seconds
- **Dead worker timeout:** 10 seconds
- **Leader lease TTL:** 5 seconds

**Rationale:** Balance between fast failure detection and system overhead.

### 7. **Round-Robin Scheduler Selection**

**Why:** Simple load distribution across scheduler nodes. Workers can contact any scheduler to handle transient failures.

### 8. **In-Memory Task Storage**

**Trade-off:** Fast access, but tasks lost on scheduler crash. For production, would use persistent queue (Redis, PostgreSQL, etc.).

## Limitations & Future Improvements

### Current Limitations

1. **Task Persistence**: Tasks are stored in-memory. Leader crash loses all tasks.
2. **Single Database**: Leader election uses single SQLite file (not distributed).
3. **No Priority Queue**: Tasks are processed FIFO, no prioritization.
4. **No Task Dependencies**: Cannot express task dependencies or workflows.
5. **Basic Metrics**: No time-series metrics or historical data.

### Future Improvements

1. **Persistent Task Queue**: Use PostgreSQL or Redis for task storage
2. **Distributed Consensus**: Replace SQLite with etcd/Consul for true distributed leader election
3. **Task Prioritization**: Add priority levels and priority queue
4. **Task Dependencies**: Support DAG-based task workflows
5. **Advanced Metrics**: Prometheus integration, task latency tracking
6. **Rate Limiting**: Per-worker task rate limits
7. **Task Scheduling**: Support cron-like scheduled tasks
8. **Dead Letter Queue**: Track permanently failed tasks separately
9. **Worker Pools**: Support different worker types/pools for different task types
10. **Authentication**: Add API authentication and authorization

---

## Project Structure

```
distributed-task-scheduler/
├── cmd/
│   ├── scheduler/       # Scheduler server entry point
│   │   └── main.go
│   └── worker/          # Worker process entry point
│       └── main.go
├── internal/
│   ├── db/              # Database initialization
│   │   └── db.go
│   ├── election/        # Leader election logic
│   │   ├── election.go
│   │   └── db_election.go
│   ├── scheduler/       # Core scheduling logic
│   │   ├── scheduler.go
│   │   └── stats.go
│   ├── types/           # Shared type definitions
│   │   ├── task.go
│   │   └── worker.go
│   └── worker/          # Worker execution logic (unused in current setup)
│       └── worker.go
├── pkg/
│   └── logger/          # Logging utilities
│       └── logger.go
├── go.mod               # Go module definition
├── go.sum               # Dependency checksums
└── README.md            # This file
```

## Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

[Add your license here]

---

**Built with Go** | **Distributed Systems** | **Fault Tolerance**
