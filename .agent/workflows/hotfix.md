---
description: "SDLC Shortcut — Fast-track a simple fix through plan → implement → close without implementation doc review"
---

# /hotfix — Fast-Track Fix

Runs `/plan` → `/implement` → `/close` in one pass with no review gates. Use best judgement. Only ask questions if the answer is genuinely ambiguous.

## SDLC Pipeline

**Full path**: `/plan` → `/implement` → `/close`
**Lightweight**: `/capture` (self-contained) | **`/hotfix`** (fast-track)

**You are here**: `/hotfix` — fast-tracking a simple fix

**Use when**: Small, well-scoped fixes (1–3 files, clear intent, no architectural decisions).
**Don't use when**: Multi-component changes, new abstractions, breaking changes, or anything you'd want a second opinion on. Use `/plan` → `/implement` → `/close` instead.

// turbo-all

## Steps

### 0. Evaluate skills

Follow `/plan`'s _Evaluate skills_ step.

### 1. Plan

Follow `/plan` with these overrides:

- **Skip** _Present for review_ and _Iterate until approved_ — no review gate
- **Do NOT ask clarifying questions** unless the fix could go in two genuinely different directions with different consequences. Make reasonable assumptions and state them.
- Keep research tight — you already know it's a small fix

### 2. Implement

Follow `/implement` with these overrides:

- **Skip** parallelism triage — small fix, sequential only
- **Skip** _Report completion_ review invitation — no review gate

### 3. Close

Follow `/close` with these overrides:

- **Skip** _Test new code_ and _Code smell sweep_ — small fix, no debt filing needed

### 4. Report

Follow `/close`'s _Report_ step — summarize changes, test results, and provide a copy-paste commit message.
