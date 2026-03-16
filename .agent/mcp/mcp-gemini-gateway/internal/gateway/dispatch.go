package gateway

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/domain"
)

// sandboxBackoffS defines escalating delays (seconds) between sandbox‐conflict retries.
var sandboxBackoffS = [3]int{3, 6, 12}

// DispatchRequest contains the parameters for a single dispatch.
type DispatchRequest struct {
	Model   string
	Prompt  string
	Label   string
	Cwd     string
	Sandbox bool
	BatchID string
}

// Dispatch executes the core flow: enqueue → pace → run Gemini → handle result.
// Status updates via UpdateStatus/UpdatePacing use _ = (fire-and-forget) because
// they are best-effort tracking; a status update failure should not abort the dispatch.
func (g *Gateway) Dispatch(ctx context.Context, req DispatchRequest) (*domain.DispatchResult, error) {
	model, err := g.registry.Resolve(req.Model)
	if err != nil {
		return nil, err
	}
	alias := req.Model
	phash := PromptHash(req.Prompt)
	maxConcurrent := g.cfg.MaxConcurrent[alias]
	maxQueue := g.cfg.MaxQueue[alias]

	if req.Cwd == "" {
		req.Cwd = g.cfg.ProjectRoot
	}

	// SECURITY: Ensure CWD is under ProjectRoot — never allow escape.
	absCwd, err := filepath.Abs(req.Cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve cwd %q: %w", req.Cwd, err)
	}
	absRoot, err := filepath.Abs(g.cfg.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root %q: %w", g.cfg.ProjectRoot, err)
	}
	if !strings.HasPrefix(absCwd, absRoot) {
		return &domain.DispatchResult{
			ExitCode: 1,
			Error:    fmt.Sprintf("cwd %q is outside project root %q — rejected for safety", req.Cwd, g.cfg.ProjectRoot),
		}, nil
	}
	req.Cwd = absCwd
	if req.BatchID == "" {
		req.BatchID = fmt.Sprintf("%08x", rand.Int31())
	}

	var requestID int64

	for attempt := range g.cfg.MaxRetries + 1 {
		// ── Atomic queue check + pacing reservation ──
		running, err := g.store.CountRunning(ctx, model)
		if err != nil {
			return nil, fmt.Errorf("count running: %w", err)
		}
		pending, err := g.store.CountPending(ctx, model)
		if err != nil {
			return nil, fmt.Errorf("count pending: %w", err)
		}

		// Queue full check (first attempt only)
		if attempt == 0 && pending >= maxQueue {
			return &domain.DispatchResult{
				ExitCode: 2,
				Error: fmt.Sprintf("Queue full: %s has %d/%d jobs pending. "+
					"Try gateway_status to check slot availability, or gateway_cancel to free slots.",
					alias, pending, maxQueue),
			}, nil
		}

		// Concurrency check — if busy, try bucket alternatives
		if attempt == 0 && running >= maxConcurrent {
			altAlias := g.findBucketAlternative(ctx, alias)
			if altAlias != "" {
				g.logger.Info("bucket rebalance", "from", alias, "to", altAlias)
				alias = altAlias
				model, _ = g.registry.Resolve(altAlias)
				maxConcurrent = g.cfg.MaxConcurrent[alias]
				maxQueue = g.cfg.MaxQueue[alias]
				running, _ = g.store.CountRunning(ctx, model)
			}

			if running >= maxConcurrent {
				// Still busy — enqueue and poll-wait
				dbReq := &domain.Request{
					Model: model, Status: "queued", Label: req.Label,
					PromptHash: phash, PromptText: req.Prompt,
					PID: os.Getpid(), Cwd: req.Cwd,
					CreatedAt: domain.NowUnix(), BatchID: req.BatchID,
				}
				requestID, err = g.store.InsertRequest(ctx, dbReq)
				if err != nil {
					return nil, fmt.Errorf("insert queued request: %w", err)
				}
				g.logger.Info("queued", "alias", alias, "running", running, "max", maxConcurrent)

				if err := g.pollForSlot(ctx, model, maxConcurrent); err != nil {
					return nil, err
				}
			}
		}

		// Read pacing data
		pacingState, err := g.store.GetPacing(ctx, model)
		if err != nil {
			return nil, fmt.Errorf("get pacing: %w", err)
		}

		var waitTime time.Duration
		if pacingState != nil {
			gapMs := pacingState.MinGapMs
			backoffMs := pacingState.BackoffMs
			jitterMs := rand.Intn(g.cfg.JitterMs[1]-g.cfg.JitterMs[0]+1) + g.cfg.JitterMs[0]

			earliest := pacingState.LastRequestAt + float64(gapMs+backoffMs+jitterMs)/1000.0
			now := domain.NowUnix()
			if earliest > now {
				waitTime = time.Duration((earliest - now) * float64(time.Second))
			}

			// Reserve the time slot
			_ = g.store.UpdatePacing(ctx, model, map[string]any{
				"last_request_at": now + waitTime.Seconds(),
			})
		}

		// Insert or update request row
		if requestID == 0 {
			dbReq := &domain.Request{
				Model: model, Status: "waiting", Label: req.Label,
				PromptHash: phash, PromptText: req.Prompt,
				PID: os.Getpid(), Cwd: req.Cwd,
				CreatedAt: domain.NowUnix(), BatchID: req.BatchID,
			}
			requestID, err = g.store.InsertRequest(ctx, dbReq)
			if err != nil {
				return nil, fmt.Errorf("insert request: %w", err)
			}
		} else {
			_ = g.store.UpdateStatus(ctx, requestID, "waiting", map[string]any{
				"retry_count": attempt,
			})
		}

		// ── Wait for pacing ──
		if waitTime > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitTime):
			}
		}

		// ── Mark as running ──
		_ = g.store.UpdateStatus(ctx, requestID, "running", map[string]any{
			"started_at": domain.NowUnix(),
		})

		// ── Execute Gemini CLI via stdin ──
		var cmd []string
		if req.Sandbox {
			cmd = []string{"gemini", "-m", model, "--sandbox", "false", "-o", "json"}
		} else {
			cmd = []string{"gemini", "-m", model, "--yolo", "-o", "json"}
		}

		fullPrompt := g.cfg.SystemPrefix + req.Prompt
		execCtx, execCancel := context.WithTimeout(ctx, time.Duration(g.cfg.TimeoutSeconds)*time.Second)
		stdout, stderr, exitCode, execErr := g.executor.Run(execCtx, cmd, req.Cwd, fullPrompt)
		execCancel()

		if execErr != nil {
			// Timeout or execution error
			_ = g.store.UpdateStatus(ctx, requestID, "failed", map[string]any{
				"error":       fmt.Sprintf("execution error: %v", execErr),
				"finished_at": domain.NowUnix(),
				"exit_code":   -1,
			})
			return &domain.DispatchResult{RequestID: requestID, ExitCode: 1, Error: execErr.Error()}, nil
		}

		// ── Rate-limited → back off and retry ──
		if DetectRateLimit(g.cfg, exitCode, stdout, stderr) {
			_ = g.pacer.OnRateLimit(ctx, model)

			if attempt < g.cfg.MaxRetries {
				_ = g.store.UpdateStatus(ctx, requestID, "retrying", nil)
				logFields := []any{
					"model", alias,
					"attempt", attempt + 1,
					"max", g.cfg.MaxRetries + 1,
				}
				if ps, _ := g.store.GetPacing(ctx, model); ps != nil {
					logFields = append(logFields,
						"min_gap_ms", ps.MinGapMs,
						"backoff_ms", ps.BackoffMs,
					)
				}
				g.logger.Info("rate limited, retrying", logFields...)
				continue
			}

			_ = g.store.UpdateStatus(ctx, requestID, "failed", map[string]any{
				"error":       "rate limit exhausted",
				"finished_at": domain.NowUnix(),
				"exit_code":   exitCode,
			})
			return &domain.DispatchResult{
				RequestID: requestID, ExitCode: g.cfg.RateLimitExitCode,
				Error: "rate limit exhausted after retries",
			}, nil
		}

		// ── Success ──
		if exitCode == 0 {
			_ = g.pacer.OnSuccess(ctx, model)

			responseText, tokenStats := parseGeminiOutput(stdout)

			fields := map[string]any{
				"finished_at":    domain.NowUnix(),
				"exit_code":      0,
				"response_text":  responseText,
			}
			for k, v := range tokenStats {
				fields[k] = v
			}
			_ = g.store.UpdateStatus(ctx, requestID, "done", fields)

			if responseText == "" && attempt < g.cfg.MaxRetries {
				// Empty response — auto-retry
				_ = g.store.UpdateStatus(ctx, requestID, "retrying", map[string]any{
					"retry_count": attempt + 1,
					"error":       "empty response, auto-retrying",
				})
				g.logger.Warn("empty response, retrying", "attempt", attempt+1)
				continue
			}

			return &domain.DispatchResult{
				RequestID: requestID, ExitCode: 0, Output: responseText,
			}, nil
		}

		// ── Sandbox conflict (exit -2) → retry ──
		if exitCode == -2 && attempt < g.cfg.MaxRetries {
			backoffS := sandboxBackoffS[min(attempt, len(sandboxBackoffS)-1)]
			_ = g.store.UpdateStatus(ctx, requestID, "retrying", map[string]any{
				"error": fmt.Sprintf("sandbox conflict, retry after %ds", backoffS),
			})
			g.logger.Info("sandbox conflict, retrying", "backoff_s", backoffS)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(backoffS) * time.Second):
			}
			continue
		}

		// ── Other failure ──
		errMsg := stderr
		if len(errMsg) > maxErrorLen {
			errMsg = errMsg[:maxErrorLen]
		}
		_ = g.store.UpdateStatus(ctx, requestID, "failed", map[string]any{
			"finished_at": domain.NowUnix(),
			"exit_code":   exitCode,
			"error":       errMsg,
		})
		return &domain.DispatchResult{
			RequestID: requestID, ExitCode: exitCode,
			Output: stdout, Error: stderr,
		}, nil
	}

	return &domain.DispatchResult{ExitCode: 1, Error: "exhausted retries"}, nil
}

func (g *Gateway) pollForSlot(ctx context.Context, model string, maxConcurrent int) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(g.cfg.QueuePollInterval):
			running, err := g.store.CountRunning(ctx, model)
			if err != nil {
				return fmt.Errorf("poll count running: %w", err)
			}
			if running < maxConcurrent {
				return nil
			}
		}
	}
}

func (g *Gateway) findBucketAlternative(ctx context.Context, alias string) string {
	bucket := FindBucketForModel(g.cfg, alias)
	if bucket == nil {
		return ""
	}

	runningModels, err := g.store.RunningModels(ctx)
	if err != nil {
		return ""
	}
	runningSet := make(map[string]bool)
	for _, m := range runningModels {
		runningSet[g.registry.AliasFor(m)] = true
	}

	return pickBucketAlternative(bucket, alias, runningSet)
}
