#!/usr/bin/env bash
# Install: bash -c "$(curl -fsSL https://raw.githubusercontent.com/thecleanbedroom/dotai/main/AGENTS.sh)"
# Update: bash ./AGENTS.sh
set -euo pipefail

command -v git >/dev/null 2>&1 || { echo "Error: git not found." >&2; exit 1; }
command -v rsync >/dev/null 2>&1 || { echo "Error: rsync not found." >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="$SCRIPT_DIR"

REPO_URL="git@github.com:thecleanbedroom/dotai.git"
REFERENCE="main"

temp_dir="$(mktemp -d)"
cleanup() {
    rm -rf "$temp_dir"
}
trap cleanup EXIT


cat <<INFO
This script will clone the dotai policy repository and copy its contents into:
  $TARGET_DIR
Existing files may be overwritten if they also exist upstream (except PROJECT.md and excluded Git folders),
but local-only files will be preserved. Review the diff after it completes.
INFO

read -rp "Proceed with sync? [y/N] " confirm
if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

printf 'Fetching %s (%s)\n' "$REPO_URL" "$REFERENCE"
git clone --depth 1 --branch "$REFERENCE" "$REPO_URL" "$temp_dir/src" >/dev/null 2>&1 || {
    echo "Error: git clone failed" >&2
    exit 1
}

printf 'Syncing policy files to %s\n' "$TARGET_DIR"
rsync -av \
    --exclude '.git' \
    --exclude '.github' \
    --exclude '.gitmodules' \
    --exclude 'PROJECT.md' \
    "$temp_dir/src/" "$TARGET_DIR/"

echo 'Policy files synchronized. Review changes and commit as needed.'
