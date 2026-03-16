package gateway

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"syscall"
	"time"

	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/domain"
)

const (
	// maxErrorLen caps error messages stored in the database.
	maxErrorLen = 500
	// gracefulShutdownDelay is the time between SIGTERM and SIGKILL in killProcess.
	gracefulShutdownDelay = 500 * time.Millisecond
)

// Status returns queue status per model with health indicator.
func (g *Gateway) Status(ctx context.Context) (map[string]domain.ModelStatus, error) {
	result := make(map[string]domain.ModelStatus)

	g.registry.ForEach(func(alias, model string) {
		maxC := g.cfg.MaxConcurrent[alias]
		maxQ := g.cfg.MaxQueue[alias]

		counts, err := g.store.StatusCounts(ctx, model)
		if err != nil {
			g.logger.Warn("status: counts", "model", model, "error", err)
		}
		running := counts["running"]
		totalQueued := counts["waiting"] + counts["queued"]
		retrying := counts["retrying"]
		totalPending := running + totalQueued + retrying

		pacingState, _ := g.store.GetPacing(ctx, model)
		backoff := 0
		if pacingState != nil {
			backoff = pacingState.BackoffMs
		}

		health := "ok"
		if totalPending >= maxQ {
			health = "saturated"
		} else if running >= maxC {
			health = "busy"
		} else if backoff > 0 {
			health = "slow"
		}

		result[alias] = domain.ModelStatus{
			Running:             running,
			Queued:              totalQueued,
			Retrying:            retrying,
			AvailableConcurrent: max(0, maxC-running),
			AvailableQueue:      max(0, maxQ-totalPending),
			Health:              health,
		}
	})

	return result, nil
}

// Jobs returns all active jobs with timing info.
func (g *Gateway) Jobs(ctx context.Context) ([]domain.JobStatus, error) {
	requests, err := g.store.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	now := domain.NowUnix()
	jobs := make([]domain.JobStatus, 0, len(requests))
	for _, r := range requests {
		var runningTime *float64
		if r.Status == "running" && r.StartedAt != nil {
			t := math.Round((now-*r.StartedAt)*10) / 10
			runningTime = &t
		}

		jobs = append(jobs, domain.JobStatus{
			ID:           r.ID,
			Model:        g.registry.AliasFor(r.Model),
			Status:       r.Status,
			Label:        r.Label,
			RetryCount:   r.RetryCount,
			RunningTimeS: runningTime,
			Created:      domain.FormatTime(r.CreatedAt),
		})
	}
	return jobs, nil
}

// Pacing returns adaptive pacing state for all models.
func (g *Gateway) Pacing(ctx context.Context) (map[string]domain.PacingInfo, error) {
	result := make(map[string]domain.PacingInfo)

	g.registry.ForEach(func(alias, model string) {
		state, err := g.store.GetPacing(ctx, model)
		if err != nil {
			g.logger.Warn("pacing: get state", "model", model, "error", err)
			return
		}
		if state == nil {
			return
		}
		result[alias] = domain.PacingInfo{
			MinGapMs:         state.MinGapMs,
			BackoffMs:        state.BackoffMs,
			ConsecutiveOK:    state.ConsecutiveOK,
			TotalOK:          state.TotalOK,
			TotalRateLimited: state.TotalRateLimited,
		}
	})

	return result, nil
}

// Stats returns historical performance stats per model.
func (g *Gateway) Stats(ctx context.Context, last string) (*domain.StatsResult, error) {
	window := ParseDuration(last)
	var since time.Time
	if window > 0 {
		since = time.Now().Add(-window)
	}

	period := last
	if period == "" {
		period = "lifetime"
	}
	result := &domain.StatsResult{
		Period: period,
		Models: make(map[string]domain.ModelStats),
	}

	g.registry.ForEach(func(alias, model string) {
		rows, err := g.store.ListCompleted(ctx, model, since)
		if err != nil || len(rows) == 0 {
			result.Models[alias] = domain.ModelStats{TotalJobs: 0}
			return
		}

		total := len(rows)
		var succeeded, failed, cancelled, rateLimitedAttempts, timeouts int
		var execTimes, waitTimes []float64

		for _, r := range rows {
			switch r.Status {
			case "done":
				succeeded++
			case "failed":
				failed++
			case "cancelled":
				cancelled++
			}
			rateLimitedAttempts += r.RetryCount

			if r.StartedAt != nil && r.FinishedAt != nil {
				execTimes = append(execTimes, *r.FinishedAt-*r.StartedAt)
			}
			if r.StartedAt != nil {
				waitTimes = append(waitTimes, *r.StartedAt-r.CreatedAt)
			}
			if r.ExitCode != nil && *r.ExitCode == -1 {
				timeouts++
			}
		}

		var currentGap *int
		if ps, _ := g.store.GetPacing(ctx, model); ps != nil {
			currentGap = &ps.MinGapMs
		}

		successRate := 0.0
		if total > 0 {
			successRate = math.Round(float64(succeeded)/float64(total)*100) / 100
		}

		result.Models[alias] = domain.ModelStats{
			TotalJobs:           total,
			Succeeded:           succeeded,
			Failed:              failed,
			Cancelled:           cancelled,
			RateLimitedAttempts: rateLimitedAttempts,
			SuccessRate:         successRate,
			AvgExecutionS:       averageOf(execTimes),
			AvgWaitS:            averageOf(waitTimes),
			AvgRetries:          math.Round(float64(rateLimitedAttempts)/float64(total)*10) / 10,
			P95ExecutionS:       p95Of(execTimes),
			PeakConcurrent:      peakConcurrent(rows),
			Timeouts:            timeouts,
			CurrentMinGapMs:     currentGap,
		}
	})

	return result, nil
}

// averageOf returns the rounded average of a float64 slice, or nil if empty.
func averageOf(vals []float64) *float64 {
	if len(vals) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	avg := math.Round(sum/float64(len(vals))*10) / 10
	return &avg
}

// p95Of returns the 95th percentile of a float64 slice, or nil if empty.
func p95Of(vals []float64) *float64 {
	if len(vals) == 0 {
		return nil
	}
	sort.Float64s(vals)
	idx := min(int(float64(len(vals))*0.95), len(vals)-1)
	v := math.Round(vals[idx]*10) / 10
	return &v
}

// peakConcurrent computes the peak number of simultaneously running requests
// using a sweep-line algorithm over start/finish timestamps.
func peakConcurrent(rows []domain.Request) int {
	type event struct {
		t     float64
		delta int // +1 start, -1 end
	}
	var events []event
	for _, r := range rows {
		if r.StartedAt != nil && r.FinishedAt != nil {
			events = append(events, event{*r.StartedAt, 1}, event{*r.FinishedAt, -1})
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].t == events[j].t {
			return events[i].delta < events[j].delta // ends before starts at same time
		}
		return events[i].t < events[j].t
	})
	peak := 0
	concurrent := 0
	for _, e := range events {
		concurrent += e.delta
		if concurrent > peak {
			peak = concurrent
		}
	}
	return peak
}

// Errors returns recent failed jobs.
func (g *Gateway) Errors(ctx context.Context, last string) (*domain.ErrorsResult, error) {
	window := ParseDuration(last)
	var since time.Time
	if window > 0 {
		since = time.Now().Add(-window)
	}

	rows, err := g.store.ListFailed(ctx, since, 20)
	if err != nil {
		return nil, err
	}

	errors := make([]domain.ErrorInfo, 0, len(rows))
	for _, r := range rows {
		var execS *float64
		if r.StartedAt != nil && r.FinishedAt != nil {
			v := math.Round((*r.FinishedAt-*r.StartedAt)*10) / 10
			execS = &v
		}

		finished := ""
		if r.FinishedAt != nil {
			finished = domain.FormatTimeShort(*r.FinishedAt)
		}

		errors = append(errors, domain.ErrorInfo{
			ID:       r.ID,
			Label:    r.Label,
			Model:    g.registry.AliasFor(r.Model),
			ExitCode: r.ExitCode,
			Error:    r.Error,
			Retries:  r.RetryCount,
			ExecS:    execS,
			Finished: finished,
		})
	}

	return &domain.ErrorsResult{Count: len(errors), Errors: errors}, nil
}

// Cancel cancels jobs by ID, batch ID, or model.
func (g *Gateway) Cancel(ctx context.Context, jobID string, modelAlias string, batchID string) (*domain.CancelResult, error) {
	var requests []domain.Request
	var err error

	if batchID != "" {
		requests, err = g.store.ListActiveByBatchID(ctx, batchID)
	} else if modelAlias != "" {
		model, resolveErr := g.registry.Resolve(modelAlias)
		if resolveErr != nil {
			return &domain.CancelResult{Error: fmt.Sprintf("Unknown model: %s. Try gateway_status to see available models.", modelAlias)}, nil
		}
		requests, err = g.store.ListActiveByModel(ctx, model)
	} else if jobID != "" {
		var id int64
		if _, scanErr := fmt.Sscanf(jobID, "%d", &id); scanErr != nil {
			return &domain.CancelResult{Error: "Invalid job ID. Use gateway_jobs to see active job IDs."}, nil
		}
		req, getErr := g.store.GetRequest(ctx, id)
		if getErr != nil || req == nil {
			return &domain.CancelResult{Error: fmt.Sprintf("Job %s not found. Use gateway_jobs to see active jobs.", jobID)}, nil
		}
		requests = []domain.Request{*req}
	} else {
		return &domain.CancelResult{Error: "Specify id, model, or batch_id. Use gateway_jobs to see active jobs."}, nil
	}

	if err != nil {
		return nil, err
	}

	cancelled := make([]int64, 0)
	for _, r := range requests {
		if r.Status == "running" && r.PID > 0 {
			killProcess(r.PID)
		}
		_ = g.store.UpdateStatus(ctx, r.ID, "cancelled", map[string]any{
			"error":       "cancelled by user",
			"finished_at": domain.NowUnix(),
			"exit_code":   -9,
		})
		cancelled = append(cancelled, r.ID)
	}

	result := &domain.CancelResult{
		Cancelled: cancelled,
		Count:     len(cancelled),
	}
	if batchID != "" {
		result.BatchID = batchID
	}
	return result, nil
}

// Retry retries a failed job by ID using its stored prompt.
func (g *Gateway) Retry(ctx context.Context, jobID int64) (*domain.DispatchResult, error) {
	req, err := g.store.GetRequest(ctx, jobID)
	if err != nil || req == nil {
		return nil, fmt.Errorf("job %d not found. Use gateway_errors to see failed jobs", jobID)
	}
	if req.PromptText == "" {
		return nil, fmt.Errorf("job %d has no stored prompt", jobID)
	}

	alias := g.registry.AliasFor(req.Model)
	label := req.Label + "-retry"
	if label == "-retry" {
		label = fmt.Sprintf("retry-%d", jobID)
	}

	return g.Dispatch(ctx, DispatchRequest{
		Model:  alias,
		Prompt: req.PromptText,
		Label:  label,
		Cwd:    req.Cwd,
	})
}

// Result returns the full request details for a job ID.
func (g *Gateway) Result(ctx context.Context, jobID int64) (*domain.Request, error) {
	return g.store.GetRequest(ctx, jobID)
}

func killProcess(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Signal(syscall.SIGTERM) // best-effort

	// Escalate to SIGKILL after grace period without blocking the caller.
	go func() {
		timer := time.NewTimer(gracefulShutdownDelay)
		defer timer.Stop()
		<-timer.C
		_ = proc.Signal(syscall.SIGKILL) // best-effort
	}()
}
