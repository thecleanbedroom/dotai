"""Startup dependency checking and .env loading."""

import os
import sys
from pathlib import Path
from typing import ClassVar


class DependencyChecker:
    """Validates required packages on startup, prints install instructions if missing."""

    REQUIRED: ClassVar[dict[str, str]] = {
        "mcp": "mcp>=1.0",
        "requests": "requests>=2.28",
    }

    @classmethod
    def load_dotenv(cls) -> None:
        """Load .env file from the memory directory if it exists."""
        env_path = Path(__file__).parent.parent / ".env"
        if not env_path.exists():
            return
        with open(env_path) as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                key, _, value = line.partition("=")
                key = key.strip()
                value = value.strip().strip('"').strip("'")
                if key and key not in os.environ:
                    os.environ[key] = value

    @classmethod
    def check(cls, *, skip: tuple[str, ...] = (), require_api_key: bool = False) -> None:
        """Check required packages and config. Exits with instructions if not met."""
        # Load .env first so API keys are available
        cls.load_dotenv()

        missing = []
        for module_name, pip_spec in cls.REQUIRED.items():
            if module_name in skip:
                continue
            try:
                __import__(module_name)
            except ImportError:
                missing.append(pip_spec)
        if missing:
            req_path = Path(__file__).parent.parent / "requirements.txt"
            print(
                f"Missing dependencies: {', '.join(missing)}\n"
                f"Install with:\n  pip install {' '.join(missing)}\n"
                f"Or:\n  pip install -r {req_path}",
                file=sys.stderr,
            )
            sys.exit(1)

        if require_api_key:
            from src.config.settings import Settings
            config = Settings.load()
            if not config.api_key():
                env_path = Path(__file__).parent.parent / ".env"
                print(
                    f"No API key found. Set OPENROUTER_API_KEY in one of:\n"
                    f"  1. {env_path}  (recommended)\n"
                    f"     OPENROUTER_API_KEY=sk-or-...\n"
                    f"  2. Shell environment:\n"
                    f"     export OPENROUTER_API_KEY=sk-or-...\n\n"
                    f"Get a key at: https://openrouter.ai/keys",
                    file=sys.stderr,
                )
                sys.exit(1)
