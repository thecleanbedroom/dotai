---
description: "Systematically raise code coverage to a target (default 80%) by triaging uncovered code, skipping smelly areas as debt, and writing tests for clean code paths"
---

# /coverage — Systematic Coverage Sprint

Raise code coverage to a target percentage by analyzing the coverage report, triaging uncovered files for code quality, and writing tests in impact order (largest gaps first, main paths before edge cases).

**Input**: Optional target percentage (default: 80%). Optional module filter.
**Output**: New test files, debt docs for smelly code, coverage at or near target.

## Philosophy

- **Don't test bad code.** If a file has code smells (wrong abstraction, dead code, duplicated logic, tangled dependencies), skip it. Write a debt doc instead. Testing bad code wastes time — the tests will need rewriting after the code is fixed.
- **Main paths first.** For each file, test the happy path and primary error handling before edge cases. Get breadth of coverage before depth.
- **Batch by component.** Group related files and write their tests together for shared context.
- **No user intervention.** Run autonomously unless truly blocked.

## Steps

### Evaluate skills

Follow /lib:#Evaluate Skills#.

### Run tests and parse coverage report

// turbo

Run the full test suite with coverage enabled. Consult `.agent/rules/platform-*.md` and the project's Makefile/scripts to determine the correct test command and coverage format.

Parse the coverage output (Clover XML, lcov, Istanbul JSON, or equivalent) to extract per-file coverage. Sort by **uncovered statements descending** (biggest gaps first). Record the baseline overall percentage.

**Output needed:** a table of files with coverage %, total statements, and uncovered statements.

### Build the coverage priority queue

Create a prioritized list of files to test, sorted by **uncovered statements** (largest gaps = most impact per test written):

**Exclude from testing:**

- Config files, framework bootstrapping, and boilerplate (low value)
- Migration / schema files
- Files with < 5 uncovered statements (diminishing returns)

**Priority tiers:**

1. **Tier 1 — High impact**: Files with > 30 uncovered statements + clean code
2. **Tier 2 — Medium impact**: Files with 10–30 uncovered statements + clean code
3. **Tier 3 — Low impact**: Files with 5–10 uncovered statements + clean code

### Triage each file for code smells (BEFORE writing tests)

For each file in the priority queue, **read it first** and apply /sniff:#Smell Checklist# (structural smells only — security, performance, and risk categories are not relevant here).

**If smells are found:**

1. **Skip testing the file** — don't write tests for smelly code
2. Follow /lib:#Create Debt Document#(source=/testcoverage triage) — describe the smell in `## Requirement`
3. Log the skip with reason in the progress table
4. Move to next file

**If clean:** Proceed to write tests.

> [!IMPORTANT]
> **The smell check is a quick scan, not a deep audit.** Spend ~30 seconds per file. If a file is mostly clean with one minor smell, test the clean parts and note the smell. Only skip the file entirely if the smell is structural (wrong abstraction, dead code path, tangled deps).

### Write tests in batches

For each clean file (or batch of related files):

#### Identify test targets

Read the file and identify:

1. **Main code paths** (happy path for each public method)
2. **Primary error handling** (explicit catch blocks, guard clauses, validation)
3. **Edge cases** (empty input, null handling, boundary conditions) — ONLY after main paths

#### Check for existing tests

Search the test directory for existing tests covering the target class or module. If a test file already exists, **extend it** rather than creating a new one.

#### Write the test

Follow the project's testing conventions. Before writing, **read existing test files in the same directory** to discover:

- Test framework and assertion style
- Base class / test utilities
- Mocking patterns
- Naming conventions

Match what you find — don't impose a different style.

#### Run tests after each batch

// turbo

After writing tests for each batch (1–3 files), run the full test suite to confirm nothing is broken.

**If tests fail:**

- Fix test issues immediately (wrong mock setup, missing dependencies)
- Do NOT fix production code bugs discovered during testing — file as debt
- Ensure all tests pass before moving to the next batch

### Track progress

Maintain a progress table (in conversation or artifact) showing:

| File              | Statements | Covered | Status           | Notes                   |
| ----------------- | ---------- | ------- | ---------------- | ----------------------- |
| `ConnectionSvc`   | 200        | 45%     | ✅ Tests written | +8 tests                |
| `ProductMapper`   | 150        | 30%     | 🗑️ Debt          | Dead code in buildQuery |
| `SyncService`     | 80         | 60%     | ✅ Tests written | +4 tests                |

After each batch, re-run the coverage parser from _Run tests and parse coverage report_ to check progress toward the target.

### Iterate until target reached

Repeat _Triage each file for code smells_ through _Track progress_, moving down the priority queue until:

- **Target reached** (e.g., 80%) — stop, report
- **Only smelly files remain** — stop, report, all debt docs filed
- **Diminishing returns** — remaining files have < 5 uncovered statements each

### Final verification

// turbo

Run the full test suite one final time with coverage and parse the results.

### Report

Summarize:

- **Baseline coverage**: X% → **Final coverage**: Y%
- **Tests written**: N new tests across M files
- **Debt filed**: K files skipped with reasons
- **Remaining gap**: What would be needed to close the remaining gap (if any)
- **True blockers**: Any files that are genuinely untestable (external deps, etc.)

Follow /commit:#Generate the Commit Message#.
