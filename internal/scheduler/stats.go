package scheduler

type Stats struct {
	// leadership
	LeaderElections int64

	// task lifecycle
	TasksSubmitted   int64
	TasksAssigned    int64
	TasksCompleted   int64
	TasksFailed      int64
	TasksRetried     int64
	TasksRequeued    int64

	// safety
	StaleCompletions int64
	StaleFailures    int64
}
