#!/bin/bash
# Quick script to update agent skills from GitHub
# Usage: bash .agent/update-agent-skill.sh

set -e

REPO_URL="https://github.com/sickn33/antigravity-awesome-skills.git"
TEMP_DIR=$(mktemp -d)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="$SCRIPT_DIR/skills"

echo "Cloning skills repo to temp location..."
git clone --depth 1 "$REPO_URL" "$TEMP_DIR"

echo "Removing existing skills directory..."
rm -rf "$TARGET_DIR"

echo "Copying skills..."
cp -r "$TEMP_DIR/skills" "$TARGET_DIR"

echo "Cleaning up temp files..."
rm -rf "$TEMP_DIR"

echo "Done! Skills updated at $TARGET_DIR"
