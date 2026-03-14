package database

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/midweste/dotai/mcp-gemini-gateway/internal/config"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	cfg := config.Default()
	cfg.DBPath = ":memory:"
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	store, err := NewStore(cfg, ":memory:", logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func seedPacing(t *testing.T, s *Store) {
	t.Helper()
	cfg := config.Default()
	reg := domain.NewModelRegistry(cfg.Models)
	if err := s.SeedPacing(context.Background(), reg, cfg); err != nil {
		t.Fatalf("SeedPacing: %v", err)
	}
}

func insertReq(t *testing.T, s *Store, model, status string, pid int, createdAt float64) int64 {
	t.Helper()
	req := &domain.Request{
		Model: model, Status: status, PromptHash: "hash",
		PID: pid, Cwd: "/tmp", CreatedAt: createdAt,
	}
	id, err := s.InsertRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("InsertRequest: %v", err)
	}
	return id
}

func insertReqFull(t *testing.T, s *Store, model, status, label, batchID string) int64 {
	t.Helper()
	req := &domain.Request{
		Model: model, Status: status, PromptHash: "hash", Label: label,
		PID: 0, Cwd: "/tmp", CreatedAt: float64(time.Now().Unix()),
		BatchID: batchID, PromptText: "test prompt",
	}
	id, err := s.InsertRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("InsertRequest: %v", err)
	}
	return id
}

// ════════ Maintainer tests ════════

func TestCleanStalePIDs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	insertReq(t, s, "test-model", "running", 999999, float64(time.Now().Unix()))

	if err := s.CleanStalePIDs(ctx); err != nil {
		t.Fatalf("CleanStalePIDs: %v", err)
	}

	var status string
	err := s.db.QueryRowContext(ctx, "SELECT status FROM requests WHERE pid=999999").Scan(&status)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "failed" {
		t.Errorf("status=%q, want 'failed'", status)
	}
}

func TestCleanupOldRequests(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	oldTime := float64(time.Now().Unix()) - float64(s.cfg.CleanupDays+1)*86400
	id := insertReq(t, s, "test-model", "done", os.Getpid(), oldTime)
	s.UpdateStatus(ctx, id, "done", map[string]any{"finished_at": oldTime})

	if err := s.CleanupOldRequests(ctx); err != nil {
		t.Fatalf("CleanupOldRequests: %v", err)
	}

	var count int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM requests WHERE id=?", id).Scan(&count)
	if count != 0 {
		t.Errorf("old request not deleted, count=%d", count)
	}
}

func TestCleanupPreservesRecent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := float64(time.Now().Unix())
	id := insertReq(t, s, "test-model", "done", os.Getpid(), now)
	s.UpdateStatus(ctx, id, "done", map[string]any{"finished_at": now})

	if err := s.CleanupOldRequests(ctx); err != nil {
		t.Fatalf("CleanupOldRequests: %v", err)
	}

	var count int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM requests WHERE id=?", id).Scan(&count)
	if count != 1 {
		t.Errorf("recent request deleted, count=%d", count)
	}
}

func TestSchemaHasBatchID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	req := &domain.Request{
		Model: "test", Status: "waiting", PromptHash: "hash",
		PID: os.Getpid(), Cwd: "/tmp", CreatedAt: float64(time.Now().Unix()),
		BatchID: "test-batch-123",
	}
	id, err := s.InsertRequest(ctx, req)
	if err != nil {
		t.Fatalf("InsertRequest: %v", err)
	}

	var batchID string
	err = s.db.QueryRowContext(ctx, "SELECT batch_id FROM requests WHERE id=?", id).Scan(&batchID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if batchID != "test-batch-123" {
		t.Errorf("batch_id=%q, want 'test-batch-123'", batchID)
	}
}

// ════════ RequestReader tests ════════

func TestCountRunning(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	model := "test-model"

	count, err := s.CountRunning(ctx, model)
	if err != nil {
		t.Fatalf("CountRunning: %v", err)
	}
	if count != 0 {
		t.Errorf("empty count=%d, want 0", count)
	}

	insertReq(t, s, model, "running", 0, float64(time.Now().Unix()))
	insertReq(t, s, model, "running", 0, float64(time.Now().Unix()))
	insertReq(t, s, model, "waiting", 0, float64(time.Now().Unix()))

	count, err = s.CountRunning(ctx, model)
	if err != nil {
		t.Fatalf("CountRunning: %v", err)
	}
	if count != 2 {
		t.Errorf("count=%d, want 2", count)
	}
}

func TestCountPending(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	model := "test-model"

	insertReq(t, s, model, "running", 0, float64(time.Now().Unix()))
	insertReq(t, s, model, "waiting", 0, float64(time.Now().Unix()))
	insertReq(t, s, model, "queued", 0, float64(time.Now().Unix()))
	insertReq(t, s, model, "retrying", 0, float64(time.Now().Unix()))
	insertReq(t, s, model, "done", 0, float64(time.Now().Unix()))

	count, err := s.CountPending(ctx, model)
	if err != nil {
		t.Fatalf("CountPending: %v", err)
	}
	if count != 4 {
		t.Errorf("count=%d, want 4 (running+waiting+queued+retrying)", count)
	}
}

func TestCountByStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	model := "test-model"

	insertReq(t, s, model, "waiting", 0, float64(time.Now().Unix()))
	insertReq(t, s, model, "waiting", 0, float64(time.Now().Unix()))
	insertReq(t, s, model, "running", 0, float64(time.Now().Unix()))

	count, err := s.CountByStatus(ctx, model, "waiting")
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if count != 2 {
		t.Errorf("waiting count=%d, want 2", count)
	}
}

func TestGetRequest(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id := insertReqFull(t, s, "test-model", "running", "my-label", "batch-1")

	req, err := s.GetRequest(ctx, id)
	if err != nil {
		t.Fatalf("GetRequest: %v", err)
	}
	if req == nil {
		t.Fatal("GetRequest returned nil")
	}
	if req.Model != "test-model" {
		t.Errorf("model=%q, want 'test-model'", req.Model)
	}
	if req.Label != "my-label" {
		t.Errorf("label=%q, want 'my-label'", req.Label)
	}
	if req.BatchID != "batch-1" {
		t.Errorf("batch_id=%q, want 'batch-1'", req.BatchID)
	}
	if req.PromptText != "test prompt" {
		t.Errorf("prompt_text=%q, want 'test prompt'", req.PromptText)
	}
}

func TestGetRequest_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetRequest(context.Background(), 99999)
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestListActive(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	insertReq(t, s, "m", "running", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m", "waiting", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m", "queued", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m", "retrying", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m", "done", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m", "failed", 0, float64(time.Now().Unix()))

	active, err := s.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(active) != 4 {
		t.Errorf("len=%d, want 4 (running+waiting+queued+retrying)", len(active))
	}
}

func TestListFailed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := float64(time.Now().Unix())
	id := insertReq(t, s, "m", "failed", 0, now)
	s.UpdateStatus(ctx, id, "failed", map[string]any{
		"finished_at": now, "error": "test error", "exit_code": 1,
	})

	// Also insert an old failure that should be excluded by time filter
	oldTime := now - 86400*30
	oldID := insertReq(t, s, "m", "failed", 0, oldTime)
	s.UpdateStatus(ctx, oldID, "failed", map[string]any{"finished_at": oldTime})

	since := time.Now().Add(-24 * time.Hour)
	failed, err := s.ListFailed(ctx, since, 10)
	if err != nil {
		t.Fatalf("ListFailed: %v", err)
	}
	if len(failed) != 1 {
		t.Errorf("len=%d, want 1 (only recent)", len(failed))
	}
}

func TestListCompleted(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := float64(time.Now().Unix())
	id := insertReq(t, s, "m1", "done", 0, now)
	s.UpdateStatus(ctx, id, "done", map[string]any{"finished_at": now})

	id2 := insertReq(t, s, "m2", "done", 0, now)
	s.UpdateStatus(ctx, id2, "done", map[string]any{"finished_at": now})

	completed, err := s.ListCompleted(ctx, "m1", time.Time{})
	if err != nil {
		t.Fatalf("ListCompleted: %v", err)
	}
	if len(completed) != 1 {
		t.Errorf("len=%d, want 1 (only m1)", len(completed))
	}
}

func TestListActiveByBatchID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	insertReqFull(t, s, "m", "waiting", "a", "batch-A")
	insertReqFull(t, s, "m", "running", "b", "batch-A")
	insertReqFull(t, s, "m", "waiting", "c", "batch-B")

	active, err := s.ListActiveByBatchID(ctx, "batch-A")
	if err != nil {
		t.Fatalf("ListActiveByBatchID: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("len=%d, want 2", len(active))
	}
}

func TestListActiveByModel(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	insertReq(t, s, "m1", "waiting", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m1", "retrying", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m2", "running", 0, float64(time.Now().Unix()))

	active, err := s.ListActiveByModel(ctx, "m1")
	if err != nil {
		t.Fatalf("ListActiveByModel: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("len=%d, want 2", len(active))
	}
}

func TestRunningModels(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	insertReq(t, s, "m1", "running", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m1", "running", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m2", "running", 0, float64(time.Now().Unix()))
	insertReq(t, s, "m3", "waiting", 0, float64(time.Now().Unix()))

	models, err := s.RunningModels(ctx)
	if err != nil {
		t.Fatalf("RunningModels: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("len=%d, want 2 (m1, m2 — distinct)", len(models))
	}
}

// ════════ PacingStore tests ════════

func TestGetPacing_NotFound(t *testing.T) {
	s := newTestStore(t)
	state, err := s.GetPacing(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetPacing: %v", err)
	}
	if state != nil {
		t.Errorf("expected nil for nonexistent model, got %+v", state)
	}
}

func TestSeedAndGetPacing(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedPacing(t, s)

	cfg := config.Default()
	model := cfg.Models["fast"]
	state, err := s.GetPacing(ctx, model)
	if err != nil {
		t.Fatalf("GetPacing: %v", err)
	}
	if state == nil {
		t.Fatal("expected pacing state after seeding, got nil")
	}
	if state.MinGapMs != cfg.InitialGapForAlias("fast") {
		t.Errorf("min_gap_ms=%d, want %d", state.MinGapMs, cfg.InitialGapForAlias("fast"))
	}
}

func TestUpdatePacing(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedPacing(t, s)

	cfg := config.Default()
	model := cfg.Models["fast"]

	err := s.UpdatePacing(ctx, model, map[string]any{
		"min_gap_ms": 5000, "backoff_ms": 1000, "consecutive_ok": 3,
	})
	if err != nil {
		t.Fatalf("UpdatePacing: %v", err)
	}

	state, _ := s.GetPacing(ctx, model)
	if state.MinGapMs != 5000 {
		t.Errorf("min_gap_ms=%d, want 5000", state.MinGapMs)
	}
	if state.BackoffMs != 1000 {
		t.Errorf("backoff_ms=%d, want 1000", state.BackoffMs)
	}
	if state.ConsecutiveOK != 3 {
		t.Errorf("consecutive_ok=%d, want 3", state.ConsecutiveOK)
	}
}

func TestDB(t *testing.T) {
	s := newTestStore(t)
	db := s.DB()
	if db == nil {
		t.Error("DB() returned nil")
	}
}

func TestSeedPacingIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedPacing(t, s)

	// Seed again — should not error
	cfg := config.Default()
	reg := domain.NewModelRegistry(cfg.Models)
	if err := s.SeedPacing(ctx, reg, cfg); err != nil {
		t.Fatalf("second SeedPacing: %v", err)
	}

	// Values should still be correct
	model := cfg.Models["fast"]
	state, _ := s.GetPacing(ctx, model)
	if state == nil {
		t.Fatal("pacing state nil after double seed")
	}
}

func TestNewStore_FileDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	cfg := config.Default()
	cfg.DBPath = dbPath
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	store, err := NewStore(cfg, dbPath, logger)
	if err != nil {
		t.Fatalf("NewStore file: %v", err)
	}
	defer store.Close()

	// Should create the file
	if _, statErr := os.Stat(dbPath); statErr != nil {
		t.Errorf("db file not created: %v", statErr)
	}

	// Should create output directory
	outputDir := dir + "/gateway-output"
	if _, statErr := os.Stat(outputDir); statErr != nil {
		t.Errorf("output directory not created: %v", statErr)
	}
}

func TestNewStore_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.DBPath = dir + "/default.db"
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Use empty dbPath to exercise default path branch
	store, err := NewStore(cfg, "", logger)
	if err != nil {
		t.Fatalf("NewStore default: %v", err)
	}
	defer store.Close()
}
