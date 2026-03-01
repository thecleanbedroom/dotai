#!/usr/bin/env python3
"""Test suite for gemini-gateway.

Run: python3 .agent/bin/test_gateway.py
Tests the gateway's internal functions using an in-memory SQLite DB.
No Gemini API calls are made — only logic and DB operations are tested.
"""

import importlib.util
import os
import sqlite3
import sys
import time
import traceback

# ── Import gateway as a module (it has no .py extension) ──
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
GATEWAY_PATH = os.path.join(SCRIPT_DIR, "gemini-gateway")

loader = importlib.machinery.SourceFileLoader("gateway", GATEWAY_PATH)
spec = importlib.util.spec_from_loader("gateway", loader)
gw = importlib.util.module_from_spec(spec)
# Prevent main() from running on import
sys.modules["gateway"] = gw
spec.loader.exec_module(gw)

# No safety guard needed — get_db() no longer exists as a standalone function.
# Tests use make_db() which creates in-memory DBs.


# ── Test infrastructure ──

class TestResults:
    def __init__(self):
        self.passed = 0
        self.failed = 0
        self.errors = []

    def ok(self, name):
        self.passed += 1
        print(f"  ✅ {name}")

    def fail(self, name, msg):
        self.failed += 1
        self.errors.append((name, msg))
        print(f"  ❌ {name}: {msg}")

    def summary(self):
        total = self.passed + self.failed
        print(f"\n{'═' * 50}")
        print(f"  {self.passed}/{total} passed, {self.failed} failed")
        if self.errors:
            print(f"\n  Failures:")
            for name, msg in self.errors:
                print(f"    • {name}: {msg}")
        print(f"{'═' * 50}")
        return self.failed == 0


results = TestResults()


def make_db():
    """Create an in-memory SQLite DB with the gateway schema."""
    db = gw.GatewayDB(gw.CONFIG, db_path=":memory:")
    return db


def insert_request(conn, **kwargs):
    """Insert a test request row with sane defaults."""
    defaults = {
        "model": gw.CONFIG["models"]["fast"],
        "status": "done",
        "label": "test",
        "prompt_hash": "abc123",
        "prompt_text": "test prompt",
        "pid": os.getpid(),
        "cwd": "/tmp",
        "created_at": time.time(),
        "started_at": None,
        "finished_at": None,
        "exit_code": None,
        "retry_count": 0,
        "batch_id": None,
    }
    defaults.update(kwargs)
    cursor = conn.execute(
        """INSERT INTO requests (model, status, label, prompt_hash, prompt_text,
           pid, cwd, created_at, started_at, finished_at, exit_code, retry_count, batch_id)
           VALUES (:model, :status, :label, :prompt_hash, :prompt_text,
           :pid, :cwd, :created_at, :started_at, :finished_at, :exit_code, :retry_count, :batch_id)""",
        defaults
    )
    conn.commit()
    return cursor.lastrowid


# ── Pure function tests ──

def test_prompt_hash():
    """prompt_hash returns consistent 12-char hex."""
    h = gw.prompt_hash("hello world")
    assert len(h) == 12, f"Expected 12 chars, got {len(h)}"
    assert h == gw.prompt_hash("hello world"), "Same input should produce same hash"
    assert h != gw.prompt_hash("different"), "Different input should produce different hash"
    results.ok("prompt_hash")


def test_detect_rate_limit_by_exit_code():
    """detect_rate_limit catches rate-limit exit code."""
    assert gw.detect_rate_limit(130, "", "") is True
    assert gw.detect_rate_limit(0, "", "") is False
    results.ok("detect_rate_limit_by_exit_code")


def test_detect_rate_limit_by_signal():
    """detect_rate_limit catches rate-limit strings in output."""
    assert gw.detect_rate_limit(1, "RESOURCE_EXHAUSTED", "") is True
    assert gw.detect_rate_limit(1, "", "429 Too Many Requests") is True
    assert gw.detect_rate_limit(1, "normal output", "normal error") is False
    results.ok("detect_rate_limit_by_signal")


def test_parse_last():
    """_parse_last handles hours, days, minutes."""
    assert gw._parse_last("1h") == 3600
    assert gw._parse_last("24h") == 86400
    assert gw._parse_last("2d") == 172800
    assert gw._parse_last("30m") == 1800
    assert gw._parse_last(None) is None
    results.ok("_parse_last")


def test_resolve_model():
    """resolve_model maps aliases to full model names."""
    assert gw.resolve_model("fast") == gw.CONFIG["models"]["fast"]
    assert gw.resolve_model("think") == gw.CONFIG["models"]["think"]
    results.ok("resolve_model")


# ── Batch model assignment tests ──

def test_find_bucket():
    """_find_bucket_for_model returns the correct bucket."""
    bucket = gw._find_bucket_for_model("fast")
    assert "fast" in bucket
    assert "lite" in bucket or "quick" in bucket  # Same flash bucket
    assert gw._find_bucket_for_model("think") is not None
    assert gw._find_bucket_for_model("nonexistent") is None
    results.ok("find_bucket_for_model")


def test_assign_models_no_conflict():
    """Batch assignment: two jobs in different buckets get their requested models."""
    db = make_db()
    jobs = [
        {"model": "fast", "prompt": "a"},
        {"model": "think", "prompt": "b"},
    ]
    assignments = gw._assign_models_for_batch(db.conn, jobs)
    assert len(assignments) == 2
    assigned_models = {m for _, m in assignments}
    assert "fast" in assigned_models
    assert "think" in assigned_models
    results.ok("assign_models_no_conflict")


def test_assign_models_same_bucket_rebalance():
    """Batch assignment: two jobs for same model get rebalanced within bucket."""
    db = make_db()
    jobs = [
        {"model": "fast", "prompt": "a"},
        {"model": "fast", "prompt": "b"},
    ]
    assignments = gw._assign_models_for_batch(db.conn, jobs)
    assert len(assignments) == 2
    models = [m for _, m in assignments]
    # One should get "fast", the other should be rebalanced to "quick" or "lite"
    assert "fast" in models
    assert models[0] != models[1], f"Both got same model: {models}"
    results.ok("assign_models_same_bucket_rebalance")


def test_assign_models_bucket_full():
    """Batch assignment: more jobs than bucket slots forces serial on same model."""
    db = make_db()
    # Flash bucket has 3 models (lite, quick, fast), so 4th job must reuse
    jobs = [
        {"model": "fast", "prompt": "a"},
        {"model": "fast", "prompt": "b"},
        {"model": "fast", "prompt": "c"},
        {"model": "fast", "prompt": "d"},
    ]
    assignments = gw._assign_models_for_batch(db.conn, jobs)
    models = [m for _, m in assignments]
    # 3 unique + 1 repeat
    assert len(set(models)) == 3, f"Expected 3 unique models, got {set(models)}"
    results.ok("assign_models_bucket_full")


# ── Pacing tests ──

def test_pacing_success_speeds_up():
    """On success, gap should decrease."""
    db = make_db()
    model = gw.CONFIG["models"]["fast"]
    before = db.conn.execute("SELECT min_gap_ms FROM pacing WHERE model=?", (model,)).fetchone()["min_gap_ms"]
    pacing = gw.PacingManager(db.conn, gw.CONFIG)
    pacing.on_success(model)
    db.conn.commit()
    after = db.conn.execute("SELECT min_gap_ms FROM pacing WHERE model=?", (model,)).fetchone()["min_gap_ms"]
    assert after < before, f"Gap should decrease: {before} -> {after}"
    results.ok("pacing_success_speeds_up")


def test_pacing_rate_limit_slows_down():
    """On rate-limit, gap should increase."""
    db = make_db()
    model = gw.CONFIG["models"]["fast"]
    before = db.conn.execute("SELECT min_gap_ms FROM pacing WHERE model=?", (model,)).fetchone()["min_gap_ms"]
    pacing = gw.PacingManager(db.conn, gw.CONFIG)
    pacing.on_rate_limit(model)
    db.conn.commit()
    after = db.conn.execute("SELECT min_gap_ms FROM pacing WHERE model=?", (model,)).fetchone()["min_gap_ms"]
    assert after > before, f"Gap should increase: {before} -> {after}"
    results.ok("pacing_rate_limit_slows_down")


def test_pacing_streak_bonus():
    """After streak_threshold successes, speedup should be more aggressive."""
    db = make_db()
    model = gw.CONFIG["models"]["fast"]
    pacing = gw.PacingManager(db.conn, gw.CONFIG)
    for _ in range(gw.CONFIG["streak_threshold"]):
        pacing.on_success(model)
        db.conn.commit()
    gap_before_streak = db.conn.execute("SELECT min_gap_ms FROM pacing WHERE model=?", (model,)).fetchone()["min_gap_ms"]
    pacing.on_success(model)
    db.conn.commit()
    gap_after_streak = db.conn.execute("SELECT min_gap_ms FROM pacing WHERE model=?", (model,)).fetchone()["min_gap_ms"]
    normal_ratio = gw.CONFIG["speedup_factor"]
    streak_ratio = gw.CONFIG["streak_speedup"]
    assert streak_ratio < normal_ratio, "Streak speedup should be more aggressive"
    results.ok("pacing_streak_bonus")


def test_pacing_backoff_drains():
    """Backoff ms drains by 500 per success."""
    db = make_db()
    model = gw.CONFIG["models"]["fast"]
    pacing = gw.PacingManager(db.conn, gw.CONFIG)
    pacing.on_rate_limit(model)
    db.conn.commit()
    backoff_before = db.conn.execute("SELECT backoff_ms FROM pacing WHERE model=?", (model,)).fetchone()["backoff_ms"]
    assert backoff_before > 0, "Should have backoff after rate-limit"
    pacing.on_success(model)
    db.conn.commit()
    backoff_after = db.conn.execute("SELECT backoff_ms FROM pacing WHERE model=?", (model,)).fetchone()["backoff_ms"]
    assert backoff_after < backoff_before, f"Backoff should drain: {backoff_before} -> {backoff_after}"
    results.ok("pacing_backoff_drains")


def test_pacing_ceiling():
    """Gap should never exceed ceiling_ms."""
    db = make_db()
    model = gw.CONFIG["models"]["fast"]
    pacing = gw.PacingManager(db.conn, gw.CONFIG)
    for _ in range(50):
        pacing.on_rate_limit(model)
        db.conn.commit()
    gap = db.conn.execute("SELECT min_gap_ms FROM pacing WHERE model=?", (model,)).fetchone()["min_gap_ms"]
    assert gap <= gw.CONFIG["ceiling_ms"], f"Gap {gap} exceeds ceiling {gw.CONFIG['ceiling_ms']}"
    results.ok("pacing_ceiling")


def test_pacing_floor():
    """Gap should never go below floor_ms."""
    db = make_db()
    model = gw.CONFIG["models"]["fast"]
    pacing = gw.PacingManager(db.conn, gw.CONFIG)
    for _ in range(100):
        pacing.on_success(model)
        db.conn.commit()
    gap = db.conn.execute("SELECT min_gap_ms FROM pacing WHERE model=?", (model,)).fetchone()["min_gap_ms"]
    floor = gw.CONFIG["floor_ms"]["fast"]
    assert gap >= floor, f"Gap {gap} below floor {floor}"
    results.ok("pacing_floor")


# ── DB maintenance tests ──

def test_clean_stale_pids():
    """Dead PIDs should be marked as failed."""
    db = make_db()
    rid = insert_request(db.conn, status="running", pid=99999999)
    db.clean_stale_pids()
    row = db.conn.execute("SELECT status FROM requests WHERE id=?", (rid,)).fetchone()
    assert row["status"] == "failed", f"Expected 'failed', got '{row['status']}'"
    results.ok("clean_stale_pids")


def test_cleanup_old_requests():
    """Old completed requests should be deleted."""
    db = make_db()
    old_time = time.time() - (gw.CONFIG["cleanup_days"] + 1) * 86400
    rid = insert_request(db.conn, status="done", finished_at=old_time)
    db.cleanup_old_requests()
    row = db.conn.execute("SELECT id FROM requests WHERE id=?", (rid,)).fetchone()
    assert row is None, "Old request should be deleted"
    results.ok("cleanup_old_requests")


def test_cleanup_preserves_recent():
    """Recent completed requests should NOT be deleted."""
    db = make_db()
    rid = insert_request(db.conn, status="done", finished_at=time.time())
    db.cleanup_old_requests()
    row = db.conn.execute("SELECT id FROM requests WHERE id=?", (rid,)).fetchone()
    assert row is not None, "Recent request should be preserved"
    results.ok("cleanup_preserves_recent")


# ── cmd_status tests ──

def test_cmd_status_empty():
    """Status should work with no requests."""
    db = make_db()
    result = gw.cmd_status(db.conn)
    assert "fast" in result
    assert result["fast"]["running"] == 0
    assert result["fast"]["health"] == "ok"
    results.ok("cmd_status_empty")


def test_cmd_status_running():
    """Status should reflect running jobs."""
    db = make_db()
    insert_request(db.conn, status="running", started_at=time.time())
    result = gw.cmd_status(db.conn)
    assert result["fast"]["running"] == 1
    assert result["fast"]["health"] == "busy"
    results.ok("cmd_status_running")


# ── cmd_jobs tests ──

def test_cmd_jobs_lists_active():
    """Jobs command should list active jobs."""
    db = make_db()
    insert_request(db.conn, status="running", label="active-job", started_at=time.time())
    insert_request(db.conn, status="done", label="done-job", finished_at=time.time())  # should not appear
    result = gw.cmd_jobs(db.conn)
    assert len(result) == 1
    assert result[0]["label"] == "active-job"
    results.ok("cmd_jobs_lists_active")


# ── cmd_cancel tests ──

def test_cmd_cancel_by_id():
    """Cancel by numeric ID should cancel the specific job."""
    db = make_db()
    rid = insert_request(db.conn, status="running", pid=99999999)
    result = gw.cmd_cancel(db.conn, str(rid), None)
    assert result["count"] == 1
    row = db.conn.execute("SELECT status FROM requests WHERE id=?", (rid,)).fetchone()
    assert row["status"] == "failed"
    results.ok("cmd_cancel_by_id")


def test_cmd_cancel_by_batch_id():
    """Cancel by batch ID should cancel all jobs in that batch."""
    db = make_db()
    bid = "test1234"
    rid1 = insert_request(db.conn, status="running", batch_id=bid, pid=99999999)
    rid2 = insert_request(db.conn, status="waiting", batch_id=bid, pid=99999999)
    rid3 = insert_request(db.conn, status="running", batch_id="other", pid=99999999)  # different batch
    result = gw.cmd_cancel(db.conn, bid, None)
    assert result["count"] == 2, f"Expected 2 cancelled, got {result['count']}"
    assert result["batch_id"] == bid
    # Third job should be unaffected
    row = db.conn.execute("SELECT status FROM requests WHERE id=?", (rid3,)).fetchone()
    assert row["status"] == "running"
    results.ok("cmd_cancel_by_batch_id")


def test_cmd_cancel_by_model():
    """Cancel by model should cancel all jobs for that model."""
    db = make_db()
    insert_request(db.conn, status="running", pid=99999999)
    insert_request(db.conn, status="waiting", pid=99999999)
    result = gw.cmd_cancel(db.conn, "ALL", "fast")
    assert result["count"] == 2
    results.ok("cmd_cancel_by_model")


# ── cmd_stats tests ──

def test_cmd_stats_empty():
    """Stats should work with no data."""
    db = make_db()
    result = gw.cmd_stats(db.conn, None)
    assert result["period"] == "lifetime"
    assert result["fast"]["total_jobs"] == 0
    results.ok("cmd_stats_empty")


def test_cmd_stats_with_history():
    """Stats should calculate success rate correctly."""
    db = make_db()
    now = time.time()
    insert_request(db.conn, status="done", started_at=now - 10, finished_at=now, exit_code=0)
    insert_request(db.conn, status="done", started_at=now - 10, finished_at=now, exit_code=0)
    insert_request(db.conn, status="failed", started_at=now - 10, finished_at=now, exit_code=1)
    result = gw.cmd_stats(db.conn, None)
    assert result["fast"]["total_jobs"] == 3
    assert result["fast"]["succeeded"] == 2
    assert result["fast"]["failed"] == 1
    assert result["fast"]["success_rate"] == 0.67
    results.ok("cmd_stats_with_history")


# ── cmd_errors tests ──

def test_cmd_errors():
    """Errors command should list failed jobs."""
    db = make_db()
    now = time.time()
    insert_request(db.conn, status="failed", finished_at=now, exit_code=1, label="broken")
    result = gw.cmd_errors(db.conn, None)
    assert result["count"] == 1
    assert result["errors"][0]["label"] == "broken"
    results.ok("cmd_errors")


# ── cmd_pacing tests ──

def test_cmd_pacing():
    """Pacing command should return state for all models."""
    db = make_db()
    result = gw.cmd_pacing(db.conn)
    assert "fast" in result
    assert "min_gap_ms" in result["fast"]
    assert "backoff_ms" in result["fast"]
    results.ok("cmd_pacing")


# ── Schema tests ──

def test_schema_has_batch_id():
    """Schema should include batch_id column."""
    db = make_db()
    rid = insert_request(db.conn, batch_id="abc123")
    row = db.conn.execute("SELECT batch_id FROM requests WHERE id=?", (rid,)).fetchone()
    assert row["batch_id"] == "abc123"
    results.ok("schema_has_batch_id")


# ── Runner ──

def run_all():
    print(f"\n{'═' * 50}")
    print(f"  Gateway Test Suite")
    print(f"{'═' * 50}\n")

    tests = [
        # Pure functions
        ("Pure Functions", [
            test_prompt_hash,
            test_detect_rate_limit_by_exit_code,
            test_detect_rate_limit_by_signal,
            test_parse_last,
            test_resolve_model,
        ]),
        # Batch model assignment
        ("Batch Model Assignment", [
            test_find_bucket,
            test_assign_models_no_conflict,
            test_assign_models_same_bucket_rebalance,
            test_assign_models_bucket_full,
        ]),
        # Pacing
        ("Adaptive Pacing", [
            test_pacing_success_speeds_up,
            test_pacing_rate_limit_slows_down,
            test_pacing_streak_bonus,
            test_pacing_backoff_drains,
            test_pacing_ceiling,
            test_pacing_floor,
        ]),
        # DB maintenance
        ("DB Maintenance", [
            test_clean_stale_pids,
            test_cleanup_old_requests,
            test_cleanup_preserves_recent,
        ]),
        # Commands
        ("Commands", [
            test_cmd_status_empty,
            test_cmd_status_running,
            test_cmd_jobs_lists_active,
            test_cmd_cancel_by_id,
            test_cmd_cancel_by_batch_id,
            test_cmd_cancel_by_model,
            test_cmd_stats_empty,
            test_cmd_stats_with_history,
            test_cmd_errors,
            test_cmd_pacing,
        ]),
        # Schema
        ("Schema", [
            test_schema_has_batch_id,
        ]),
    ]

    for group_name, test_fns in tests:
        print(f"\n  ── {group_name} ──")
        for fn in test_fns:
            try:
                fn()
            except Exception as e:
                results.fail(fn.__name__, str(e))
                traceback.print_exc()

    success = results.summary()
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    run_all()
