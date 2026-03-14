package server

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/gateway"
)

func (s *MCPServer) registerTools() {
	// ── gateway_dispatch ──
	s.mcp.AddTool(
		mcp.NewTool("gateway_dispatch",
			mcp.WithDescription("Execute a single Gemini CLI job via stdin pipe"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(true),
			}),
			mcp.WithString("model",
				mcp.Required(),
				mcp.Description("Model alias: lite, quick, fast, think, deep"),
				mcp.Enum("lite", "quick", "fast", "think", "deep"),
			),
			mcp.WithString("prompt",
				mcp.Required(),
				mcp.Description("The prompt text to send to Gemini CLI"),
			),
			mcp.WithString("label",
				mcp.Description("Optional human-readable label for this job"),
			),
			mcp.WithString("cwd",
				mcp.Description("Working directory for Gemini CLI execution"),
			),
			mcp.WithBoolean("sandbox",
				mcp.Description("If true, run with --sandbox false (default: use --yolo)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			dr := gateway.DispatchRequest{
				Model:  argStr(args, "model"),
				Prompt: argStr(args, "prompt"),
				Label:  argStr(args, "label"),
				Cwd:    argStr(args, "cwd"),
			}
			if v, ok := args["sandbox"].(bool); ok {
				dr.Sandbox = v
			}

			result, err := s.gateway.Dispatch(ctx, dr)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(result)), nil
		},
	)

	// ── gateway_batch_dispatch ──
	s.mcp.AddTool(
		mcp.NewTool("gateway_batch_dispatch",
			mcp.WithDescription("Execute multiple Gemini CLI jobs in parallel via goroutines, one per model slot"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(true),
			}),
			mcp.WithArray("jobs",
				mcp.Required(),
				mcp.Description("Array of job objects [{model, prompt, label?, cwd?, sandbox?}]"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			jobsRaw, ok := args["jobs"].([]any)
			if !ok {
				return mcp.NewToolResultError("jobs must be an array of objects"), nil
			}

			jobs := make([]gateway.DispatchRequest, 0, len(jobsRaw))
			for _, j := range jobsRaw {
				jm, ok := j.(map[string]any)
				if !ok {
					continue
				}
				dr := gateway.DispatchRequest{
					Model:  argStr(jm, "model"),
					Prompt: argStr(jm, "prompt"),
					Label:  argStr(jm, "label"),
					Cwd:    argStr(jm, "cwd"),
				}
				if v, ok := jm["sandbox"].(bool); ok {
					dr.Sandbox = v
				}
				jobs = append(jobs, dr)
			}

			results, err := s.gateway.RunBatch(ctx, jobs)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(results)), nil
		},
	)

	// ── gateway_status ──
	s.mcp.AddTool(
		mcp.NewTool("gateway_status",
			mcp.WithDescription("Queue status per model with health indicator (ok/busy/slow/saturated)"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    boolPtr(true),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			}),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			status, err := s.gateway.Status(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(status)), nil
		},
	)

	// ── gateway_jobs ──
	s.mcp.AddTool(
		mcp.NewTool("gateway_jobs",
			mcp.WithDescription("List all active jobs (queued, waiting, running, retrying)"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    boolPtr(true),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			}),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			jobs, err := s.gateway.Jobs(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(jobs)), nil
		},
	)

	// ── gateway_pacing ──
	s.mcp.AddTool(
		mcp.NewTool("gateway_pacing",
			mcp.WithDescription("Adaptive pacing state for all models (gap, backoff, streaks)"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    boolPtr(true),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			}),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pacing, err := s.gateway.Pacing(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(pacing)), nil
		},
	)

	// ── gateway_stats ──
	s.mcp.AddTool(
		mcp.NewTool("gateway_stats",
			mcp.WithDescription("Historical performance stats per model (success rate, timing, retries)"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    boolPtr(true),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			}),
			mcp.WithString("last",
				mcp.Description("Time window, e.g. '1h', '2d', '30m'. Empty = lifetime"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			last := argStr(req.GetArguments(), "last")
			stats, err := s.gateway.Stats(ctx, last)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(stats)), nil
		},
	)

	// ── gateway_errors ──
	s.mcp.AddTool(
		mcp.NewTool("gateway_errors",
			mcp.WithDescription("Recent failed jobs with error details and retry count"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    boolPtr(true),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			}),
			mcp.WithString("last",
				mcp.Description("Time window, e.g. '1h', '2d'. Empty = lifetime"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			last := argStr(req.GetArguments(), "last")
			errors, err := s.gateway.Errors(ctx, last)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(errors)), nil
		},
	)

	// ── gateway_cancel ──
	s.mcp.AddTool(
		mcp.NewTool("gateway_cancel",
			mcp.WithDescription("Cancel jobs by ID, batch ID, or model. Kills running processes."),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			}),
			mcp.WithString("id",
				mcp.Description("Cancel a specific job by numeric ID"),
			),
			mcp.WithString("model",
				mcp.Description("Cancel all active jobs for a model alias (e.g. 'fast')"),
			),
			mcp.WithString("batch_id",
				mcp.Description("Cancel all jobs in a batch"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			result, err := s.gateway.Cancel(ctx,
				argStr(args, "id"),
				argStr(args, "model"),
				argStr(args, "batch_id"),
			)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(result)), nil
		},
	)

	// ── gateway_retry ──
	s.mcp.AddTool(
		mcp.NewTool("gateway_retry",
			mcp.WithDescription("Retry a failed job using its stored prompt"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(true),
			}),
			mcp.WithNumber("id",
				mcp.Required(),
				mcp.Description("Job ID to retry (from gateway_errors)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			id := int64(args["id"].(float64))
			result, err := s.gateway.Retry(ctx, id)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(result)), nil
		},
	)
}

func argStr(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func boolPtr(b bool) *bool {
	return &b
}
