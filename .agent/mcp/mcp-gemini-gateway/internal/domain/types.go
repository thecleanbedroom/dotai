package domain

import "time"

// Request represents a gateway job request stored in SQLite.
type Request struct {
	ID            int64
	Model         string
	Status        string
	Label         string
	PromptHash    string
	PromptText    string
	PID           int
	Cwd           string
	CreatedAt     float64
	StartedAt     *float64
	FinishedAt    *float64
	ExitCode      *int
	RetryCount    int
	Error         string
	TokensIn      *int
	TokensOut     *int
	TokensCached  *int
	TokensThoughts *int
	ToolCalls     *int
	APILatencyMs  *int
	BatchID       string
}

// PacingState represents the adaptive pacing state for a model.
type PacingState struct {
	Model            string
	MinGapMs         int
	LastRequestAt    float64
	BackoffMs        int
	ConsecutiveOK    int
	TotalOK          int
	TotalRateLimited int
}

// JobStatus represents an active job for the jobs command.
type JobStatus struct {
	ID           int64   `json:"id"`
	Model        string  `json:"model"`
	Status       string  `json:"status"`
	Label        string  `json:"label"`
	RetryCount   int     `json:"retry_count"`
	RunningTimeS *float64 `json:"running_time_s"`
	Created      string  `json:"created"`
}

// ModelStatus represents per-model queue health for the status command.
type ModelStatus struct {
	Running            int    `json:"running"`
	Queued             int    `json:"queued"`
	Retrying           int    `json:"retrying"`
	AvailableConcurrent int   `json:"available_concurrent"`
	AvailableQueue     int    `json:"available_queue"`
	Health             string `json:"health"`
}

// PacingInfo represents pacing state for the pacing command.
type PacingInfo struct {
	MinGapMs         int `json:"min_gap_ms"`
	BackoffMs        int `json:"backoff_ms"`
	ConsecutiveOK    int `json:"consecutive_ok"`
	TotalOK          int `json:"total_ok"`
	TotalRateLimited int `json:"total_rate_limited"`
}

// ModelStats represents historical performance for the stats command.
type ModelStats struct {
	TotalJobs           int      `json:"total_jobs"`
	Succeeded           int      `json:"succeeded,omitempty"`
	Failed              int      `json:"failed,omitempty"`
	Cancelled           int      `json:"cancelled,omitempty"`
	RateLimitedAttempts int      `json:"rate_limited_attempts,omitempty"`
	SuccessRate         float64  `json:"success_rate,omitempty"`
	AvgExecutionS       *float64 `json:"avg_execution_s,omitempty"`
	AvgWaitS            *float64 `json:"avg_wait_s,omitempty"`
	AvgRetries          float64  `json:"avg_retries,omitempty"`
	P95ExecutionS       *float64 `json:"p95_execution_s,omitempty"`
	PeakConcurrent      int      `json:"peak_concurrent,omitempty"`
	Timeouts            int      `json:"timeouts,omitempty"`
	CurrentMinGapMs     *int     `json:"current_min_gap_ms,omitempty"`
}

// ErrorInfo represents a failed job for the errors command.
type ErrorInfo struct {
	ID       int64   `json:"id"`
	Label    string  `json:"label"`
	Model    string  `json:"model"`
	ExitCode *int    `json:"exit_code"`
	Error    string  `json:"error"`
	Retries  int     `json:"retries"`
	ExecS    *float64 `json:"exec_s,omitempty"`
	Finished string  `json:"finished"`
}

// CancelResult represents the result of a cancel operation.
type CancelResult struct {
	Cancelled []int64 `json:"cancelled"`
	Count     int     `json:"count"`
	BatchID   string  `json:"batch_id,omitempty"`
	Error     string  `json:"error,omitempty"`
}

// DispatchResult represents the result of a dispatch or batch dispatch.
type DispatchResult struct {
	RequestID int64  `json:"request_id"`
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
}

// BatchResult represents a single job result within a batch.
type BatchResult struct {
	Label    string `json:"label"`
	Model    string `json:"model"`
	Status   string `json:"status"`
	ExitCode int    `json:"exit_code"`
}

// FormatTime formats a Unix timestamp for display.
func FormatTime(ts float64) string {
	return time.Unix(int64(ts), 0).Format("2006-01-02 15:04:05")
}

// FormatTimeShort formats a Unix timestamp with time only.
func FormatTimeShort(ts float64) string {
	return time.Unix(int64(ts), 0).Format("15:04:05")
}
