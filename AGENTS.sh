#!/usr/bin/env bash
# Install: bash -c "$(curl -fsSL https://raw.githubusercontent.com/thecleanbedroom/dotai/refs/heads/main/AGENTS.sh)"
# Update: bash ./AGENTS.sh
set -euo pipefail

# When run via curl pipe, BASH_SOURCE is empty — fall back to current directory
if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
else
    SCRIPT_DIR="$(pwd)"
fi

UPDATE_URL="https://raw.githubusercontent.com/thecleanbedroom/dotai/refs/heads/main/.agent/bin/update"

temp_script="$(mktemp)"
curl -fsSL "$UPDATE_URL" -o "$temp_script" || { rm -f "$temp_script"; echo "Error: failed to fetch update script" >&2; exit 1; }
bash "$temp_script" "$SCRIPT_DIR"
rm -f "$temp_script"
