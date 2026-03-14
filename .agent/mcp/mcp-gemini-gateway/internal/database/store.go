package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/midweste/dotai/mcp-gemini-gateway/internal/config"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/domain"

	_ "modernc.org/sqlite"
)

// Store implements all database interfaces using SQLite.
// Satisfies: RequestReader, RequestWriter, PacingStore, Maintainer.
type Store struct {
	db     *sql.DB
	cfg    *config.Config
	logger *slog.Logger
}

// NewStore opens (or creates) the SQLite database and applies schema + migrations.
func NewStore(cfg *config.Config, dbPath string, logger *slog.Logger) (*Store, error) {
	if dbPath == "" {
		dbPath = cfg.DBPath
	}

	dsn := dbPath
	if dbPath == ":memory:" {
		dsn = "file::memory:?mode=memory&cache=shared"
	} else {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory %s: %w", dir, err)
		}
		dsn = dbPath + "?_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &Store{db: db, cfg: cfg, logger: logger}

	if err := s.applySchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	if err := s.runMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	if dbPath != ":memory:" {
		outputDir := filepath.Join(filepath.Dir(dbPath), "gateway-output")
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			logger.Warn("failed to create output directory", "dir", outputDir, "error", err)
		}
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for exclusive transaction use.
func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) applySchema() error {
	_, err := s.db.Exec(SchemaSQL)
	return err
}

func (s *Store) runMigrations() error {
	// Migration: add prompt_text column if missing
	if !s.columnExists("requests", "prompt_text") {
		if _, err := s.db.Exec("ALTER TABLE requests ADD COLUMN prompt_text TEXT"); err != nil {
			return fmt.Errorf("add prompt_text: %w", err)
		}
	}

	// Migration: add token stats columns if missing
	if !s.columnExists("requests", "tokens_in") {
		for _, col := range []string{
			"tokens_in INTEGER", "tokens_out INTEGER", "tokens_cached INTEGER",
			"tokens_thoughts INTEGER", "tool_calls INTEGER", "api_latency_ms INTEGER",
		} {
			if _, err := s.db.Exec("ALTER TABLE requests ADD COLUMN " + col); err != nil {
				return fmt.Errorf("add %s: %w", col, err)
			}
		}
	}

	// Migration: add batch_id column if missing
	if !s.columnExists("requests", "batch_id") {
		if _, err := s.db.Exec("ALTER TABLE requests ADD COLUMN batch_id TEXT"); err != nil {
			return fmt.Errorf("add batch_id: %w", err)
		}
	}

	return nil
}

func (s *Store) columnExists(table, column string) bool {
	rows, err := s.db.Query("SELECT " + column + " FROM " + table + " LIMIT 0")
	if err != nil {
		return false
	}
	rows.Close()
	return true
}

// ── RequestReader implementation ──

// CountRunning returns the number of running requests for a model.
func (s *Store) CountRunning(ctx context.Context, model string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM requests WHERE model=? AND status='running'", model,
	).Scan(&count)
	return count, err
}

// CountPending returns the total pending (queued+waiting+running+retrying) for a model.
func (s *Store) CountPending(ctx context.Context, model string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM requests WHERE model=? AND status IN ('queued','waiting','running','retrying')", model,
	).Scan(&count)
	return count, err
}

// CountByStatus returns count of requests with a given status for a model.
func (s *Store) CountByStatus(ctx context.Context, model, status string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM requests WHERE model=? AND status=?", model, status,
	).Scan(&count)
	return count, err
}

// GetRequest returns a single request by ID.
func (s *Store) GetRequest(ctx context.Context, id int64) (*domain.Request, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, model, status, label, prompt_hash, prompt_text, pid, cwd,
		        created_at, started_at, finished_at, exit_code, retry_count, error,
		        tokens_in, tokens_out, tokens_cached, tokens_thoughts, tool_calls,
		        api_latency_ms, batch_id
		 FROM requests WHERE id=?`, id,
	)
	return scanRequest(row)
}

// ListActive returns all requests in active states (queued, waiting, running, retrying).
func (s *Store) ListActive(ctx context.Context) ([]domain.Request, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, model, status, label, prompt_hash, prompt_text, pid, cwd,
		        created_at, started_at, finished_at, exit_code, retry_count, error,
		        tokens_in, tokens_out, tokens_cached, tokens_thoughts, tool_calls,
		        api_latency_ms, batch_id
		 FROM requests
		 WHERE status IN ('queued','waiting','running','retrying')
		 ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRequests(rows)
}

// ListFailed returns recent failed requests since a cutoff time, limited.
func (s *Store) ListFailed(ctx context.Context, since time.Time, limit int) ([]domain.Request, error) {
	cutoff := float64(since.Unix())
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, model, status, label, prompt_hash, prompt_text, pid, cwd,
		        created_at, started_at, finished_at, exit_code, retry_count, error,
		        tokens_in, tokens_out, tokens_cached, tokens_thoughts, tool_calls,
		        api_latency_ms, batch_id
		 FROM requests
		 WHERE status='failed' AND finished_at IS NOT NULL AND finished_at > ?
		 ORDER BY finished_at DESC LIMIT ?`, cutoff, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRequests(rows)
}

// ListCompleted returns completed requests for a model since a cutoff time.
func (s *Store) ListCompleted(ctx context.Context, model string, since time.Time) ([]domain.Request, error) {
	cutoff := float64(since.Unix())
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, model, status, label, prompt_hash, prompt_text, pid, cwd,
		        created_at, started_at, finished_at, exit_code, retry_count, error,
		        tokens_in, tokens_out, tokens_cached, tokens_thoughts, tool_calls,
		        api_latency_ms, batch_id
		 FROM requests
		 WHERE model=? AND finished_at IS NOT NULL AND finished_at > ?`, model, cutoff,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRequests(rows)
}

// ListActiveByBatchID returns active requests for a batch.
func (s *Store) ListActiveByBatchID(ctx context.Context, batchID string) ([]domain.Request, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, model, status, label, prompt_hash, prompt_text, pid, cwd,
		        created_at, started_at, finished_at, exit_code, retry_count, error,
		        tokens_in, tokens_out, tokens_cached, tokens_thoughts, tool_calls,
		        api_latency_ms, batch_id
		 FROM requests
		 WHERE batch_id=? AND status IN ('waiting','running','retrying')`, batchID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRequests(rows)
}

// ListActiveByModel returns active requests for a model.
func (s *Store) ListActiveByModel(ctx context.Context, model string) ([]domain.Request, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, model, status, label, prompt_hash, prompt_text, pid, cwd,
		        created_at, started_at, finished_at, exit_code, retry_count, error,
		        tokens_in, tokens_out, tokens_cached, tokens_thoughts, tool_calls,
		        api_latency_ms, batch_id
		 FROM requests
		 WHERE model=? AND status IN ('waiting','running','retrying')`, model,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRequests(rows)
}

// RunningModels returns all model names that have a running request.
func (s *Store) RunningModels(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT model FROM requests WHERE status='running'",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	return models, rows.Err()
}

// ── RequestWriter implementation ──

// InsertRequest inserts a new request and returns its ID.
func (s *Store) InsertRequest(ctx context.Context, req *domain.Request) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO requests (model, status, label, prompt_hash, prompt_text, pid, cwd,
		                       created_at, retry_count, batch_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Model, req.Status, req.Label, req.PromptHash, req.PromptText,
		req.PID, req.Cwd, req.CreatedAt, req.RetryCount, req.BatchID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateStatus updates a request's status and optional fields.
func (s *Store) UpdateStatus(ctx context.Context, id int64, status string, fields map[string]any) error {
	setClauses := "status=?"
	args := []any{status}

	for col, val := range fields {
		setClauses += ", " + col + "=?"
		args = append(args, val)
	}
	args = append(args, id)

	_, err := s.db.ExecContext(ctx,
		"UPDATE requests SET "+setClauses+" WHERE id=?", args...,
	)
	return err
}

// ── PacingStore implementation ──

// GetPacing returns the pacing state for a model.
func (s *Store) GetPacing(ctx context.Context, model string) (*domain.PacingState, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT model, min_gap_ms, last_request_at, backoff_ms, consecutive_ok, total_ok, total_rate_limited FROM pacing WHERE model=?",
		model,
	)
	var p domain.PacingState
	err := row.Scan(&p.Model, &p.MinGapMs, &p.LastRequestAt, &p.BackoffMs,
		&p.ConsecutiveOK, &p.TotalOK, &p.TotalRateLimited)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdatePacing updates pacing fields for a model.
func (s *Store) UpdatePacing(ctx context.Context, model string, fields map[string]any) error {
	setClauses := ""
	args := make([]any, 0, len(fields)+1)
	first := true
	for col, val := range fields {
		if !first {
			setClauses += ", "
		}
		setClauses += col + "=?"
		args = append(args, val)
		first = false
	}
	args = append(args, model)

	_, err := s.db.ExecContext(ctx,
		"UPDATE pacing SET "+setClauses+" WHERE model=?", args...,
	)
	return err
}

// SeedPacing inserts pacing rows for all configured models (INSERT OR IGNORE).
func (s *Store) SeedPacing(ctx context.Context, registry *domain.ModelRegistry, cfg *config.Config) error {
	stmt, err := s.db.PrepareContext(ctx,
		"INSERT OR IGNORE INTO pacing (model, min_gap_ms) VALUES (?, ?)",
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	registry.ForEach(func(alias, model string) {
		gap := cfg.InitialGapForAlias(alias)
		if _, execErr := stmt.ExecContext(ctx, model, gap); execErr != nil {
			s.logger.Warn("seed pacing failed", "model", model, "error", execErr)
		}
	})
	return nil
}

// ── Maintainer implementation ──

// CleanStalePIDs marks running/waiting/retrying requests with dead PIDs as failed.
func (s *Store) CleanStalePIDs(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, pid FROM requests WHERE status IN ('waiting','running','retrying') AND pid IS NOT NULL",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var staleIDs []int64
	for rows.Next() {
		var id int64
		var pid int
		if err := rows.Scan(&id, &pid); err != nil {
			return err
		}
		if !isProcessAlive(pid) {
			staleIDs = append(staleIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	now := float64(time.Now().Unix())
	for _, id := range staleIDs {
		if _, err := s.db.ExecContext(ctx,
			"UPDATE requests SET status='failed', error='process died (stale PID)', finished_at=? WHERE id=?",
			now, id,
		); err != nil {
			s.logger.Warn("failed to mark stale PID", "id", id, "error", err)
		}
	}
	return nil
}

// CleanupOldRequests deletes completed/failed requests older than the configured days.
func (s *Store) CleanupOldRequests(ctx context.Context) error {
	cutoff := float64(time.Now().Unix()) - float64(s.cfg.CleanupDays)*86400
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM requests WHERE status IN ('done','failed') AND finished_at < ?", cutoff,
	)
	return err
}

// isProcessAlive checks if a PID is still running.
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// ── Row scanning helpers (DRY) ──

type scannable interface {
	Scan(dest ...any) error
}

func scanRequest(row scannable) (*domain.Request, error) {
	var r domain.Request
	var label, promptText, errStr, batchID sql.NullString
	err := row.Scan(
		&r.ID, &r.Model, &r.Status, &label, &r.PromptHash, &promptText,
		&r.PID, &r.Cwd, &r.CreatedAt, &r.StartedAt, &r.FinishedAt,
		&r.ExitCode, &r.RetryCount, &errStr,
		&r.TokensIn, &r.TokensOut, &r.TokensCached, &r.TokensThoughts,
		&r.ToolCalls, &r.APILatencyMs, &batchID,
	)
	if err != nil {
		return nil, err
	}
	r.Label = label.String
	r.PromptText = promptText.String
	r.Error = errStr.String
	r.BatchID = batchID.String
	return &r, nil
}

func scanRequests(rows *sql.Rows) ([]domain.Request, error) {
	var requests []domain.Request
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, *r)
	}
	return requests, rows.Err()
}
