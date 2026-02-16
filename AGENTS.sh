#!/usr/bin/env bash
# Install: bash -c "$(curl -fsSL https://raw.githubusercontent.com/thecleanbedroom/dotai/refs/heads/main/AGENTS.sh)"
# Update: bash ./AGENTS.sh
set -euo pipefail

command -v git >/dev/null 2>&1 || { echo "Error: git not found." >&2; exit 1; }
command -v rsync >/dev/null 2>&1 || { echo "Error: rsync not found." >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="$SCRIPT_DIR"

POLICY_REPO_URL="git@github.com:thecleanbedroom/dotai.git"
REFERENCE="main"

temp_dir="$(mktemp -d)"
cleanup() {
    rm -rf "$temp_dir"
}
trap cleanup EXIT


cat <<INFO
This script will clone the dotai policy repository and sync rules/config into:
  $TARGET_DIR

Existing policy files may be overwritten (except PROJECT.md and Git folders).
Local-only files will be preserved. Review the diff after it completes.

To install skills, run the /build-skills workflow after syncing.
INFO

read -rp "Proceed with sync? [y/N] " confirm
if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

printf 'Fetching policy repo %s (%s)\n' "$POLICY_REPO_URL" "$REFERENCE"
git clone --depth 1 --branch "$REFERENCE" "$POLICY_REPO_URL" "$temp_dir/policy" >/dev/null 2>&1 || {
    echo "Error: git clone of policy repo failed" >&2
    exit 1
}

printf 'Syncing policy files to %s\n' "$TARGET_DIR"
rsync -av \
    --exclude '.git' \
    --exclude '.github' \
    --exclude '.gitmodules' \
    --exclude 'PROJECT.md' \
    --exclude '.agent/skills' \
    --exclude '.agent/dotai' \
    "$temp_dir/policy/" "$TARGET_DIR/"

echo 'Policy files synchronized. Review changes and commit as needed.'
echo 'Run /add-skills to install project-specific skills.'
