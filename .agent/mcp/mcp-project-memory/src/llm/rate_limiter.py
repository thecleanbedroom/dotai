"""Cross-process sliding-window rate limiter with 429 emergency braking.

Uses a shared JSON file with fcntl.flock for atomic access across
multiple concurrent build processes. All projects sharing the same
source code (via symlinks) automatically coordinate through the
same lock file in data/.

Two layers of protection:
1. Proactive pacing: acquire() checks a shared sliding window and
   sleeps until a slot opens rather than firing and getting rejected.
2. Reactive braking: on_rate_limit() writes a cooldown timestamp to
   the shared file, blocking all processes until it expires.

Thread-safe (threading.Lock) + process-safe (fcntl.flock).
"""

import fcntl
import json
import pathlib
import sys
import threading
import time


class RateLimiter:
    """Cross-process sliding-window rate limiter with 429 emergency braking."""

    def __init__(self, rpm: int = 20, *, data_dir: str | None = None):
        self._rpm = rpm
        # Shared file — follows symlinks so all projects coordinate
        if data_dir:
            self._state_file = str(
                pathlib.Path(data_dir) / "build" / "rate_limit.json"
            )
        else:
            # Default: source tree data/ dir (backwards compatible)
            self._state_file = str(
                pathlib.Path(__file__).resolve().parent.parent / "data" / "build" / "rate_limit.json"
            )
        # Thread-level lock (within this process)
        self._thread_lock = threading.Lock()
        # Emergency brake for threads in this process
        self._gate = threading.Event()
        self._gate.set()
        self._brake_lock = threading.Lock()
        self._consecutive_429s = 0
        self._cooldown_until = 0.0
        self._total_429s = 0

    @property
    def rpm(self) -> int:
        return self._rpm

    @property
    def total_429s(self) -> int:
        return self._total_429s

    def _read_state(self, f) -> dict:
        """Read state from locked file."""
        f.seek(0)
        try:
            return json.loads(f.read() or "{}")
        except (json.JSONDecodeError, ValueError):
            return {}

    def _write_state(self, f, state: dict) -> None:
        """Write state to locked file."""
        f.seek(0)
        f.truncate()
        f.write(json.dumps(state))
        f.flush()

    def acquire(self) -> None:
        """Call before each API request. Paces to stay within RPM.

        Coordinates across both threads (threading.Lock) and
        processes (fcntl.flock) using wall-clock timestamps.
        """
        # Wait for local emergency brake
        self._gate.wait()

        while True:
            now = time.time()

            with self._thread_lock:
                try:
                    with open(self._state_file, "r+") as f:
                        fcntl.flock(f, fcntl.LOCK_EX)
                        try:
                            state = self._read_state(f)

                            # Check cross-process cooldown
                            cooldown = state.get("cooldown_until", 0)
                            if cooldown > now:
                                wait = cooldown - now
                                fcntl.flock(f, fcntl.LOCK_UN)
                                time.sleep(wait + 0.1)
                                continue

                            # Sliding window — evict entries older than 60s
                            window = [t for t in state.get("window", [])
                                      if t > now - 60.0]

                            if len(window) < self._rpm:
                                window.append(now)
                                state["window"] = window
                                self._write_state(f, state)
                                return  # Slot acquired

                            # Full — calculate wait
                            sleep_until = window[0] + 60.0
                        finally:
                            fcntl.flock(f, fcntl.LOCK_UN)
                except FileNotFoundError:
                    # First use — create the file and retry
                    pathlib.Path(self._state_file).parent.mkdir(
                        parents=True, exist_ok=True,
                    )
                    with open(self._state_file, "w") as f:
                        f.write("{}")
                    continue

            wait = sleep_until - now
            if wait > 0:
                time.sleep(wait + 0.05)

            self._gate.wait()

    def on_success(self) -> None:
        """Call after a successful response."""
        with self._brake_lock:
            self._consecutive_429s = 0

    def on_rate_limit(self, retry_after: float | None = None) -> None:
        """Call when a 429 is received. Writes cooldown to shared file.

        All processes and threads will block until cooldown expires.
        RPM is NOT reduced — the sliding window already paces correctly,
        and dead processes' timestamps age out in 60s naturally.
        """
        with self._brake_lock:
            self._total_429s += 1
            self._consecutive_429s += 1

            if retry_after and retry_after > 0:
                wait = retry_after
            else:
                wait = min(2 ** self._consecutive_429s, 30)

            cooldown_until = time.time() + wait

            # Write cooldown to shared file so other processes see it
            try:
                with open(self._state_file, "r+") as f:
                    fcntl.flock(f, fcntl.LOCK_EX)
                    try:
                        state = self._read_state(f)
                        if cooldown_until > state.get("cooldown_until", 0):
                            state["cooldown_until"] = cooldown_until
                            self._write_state(f, state)
                    finally:
                        fcntl.flock(f, fcntl.LOCK_UN)
            except FileNotFoundError:
                pass

            self._cooldown_until = cooldown_until
            self._gate.clear()

            print(
                f"      ⏳ 429 #{self._total_429s} — waiting {wait:.0f}s",
                file=sys.stderr, flush=True,
            )

        timer = threading.Timer(wait, self._release)
        timer.daemon = True
        timer.start()

    def _release(self) -> None:
        """Re-open the gate after cooldown."""
        with self._brake_lock:
            if time.time() >= self._cooldown_until:
                self._gate.set()
