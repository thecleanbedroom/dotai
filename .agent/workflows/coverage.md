---
description: "Systematically raise code coverage to a target (default 80%) by triaging uncovered code, skipping smelly areas as debt, and writing tests for clean code paths"
---

# /coverage — Systematic Coverage Sprint

Raise code coverage to a target percentage by analyzing the coverage report, triaging uncovered files for code quality, and writing tests in impact order (largest gaps first, main paths before edge cases).

**Input**: Optional target percentage (default: 80%). Optional module filter (e.g., `RetailRocket`).
**Output**: New test files, debt docs for smelly code, coverage at or near target.

## Philosophy

- **Don't test bad code.** If a file has code smells (wrong abstraction, dead code, duplicated logic, tangled dependencies), skip it. Write a debt doc instead. Testing bad code wastes time — the tests will need rewriting after the code is fixed.
- **Main paths first.** For each file, test the happy path and primary error handling before edge cases. Get breadth of coverage before depth.
- **Batch by component.** Group related files and write their tests together for shared context.
- **No user intervention.** Run autonomously unless truly blocked.

## Steps

### 1. Run tests and parse coverage report

// turbo

Run the full test suite with coverage enabled. Parse the coverage output to identify the baseline.

```bash
# Run tests and generate coverage
make artisan-test   # or equivalent (check Makefile first)
```

After tests pass, parse the **Clover XML** coverage report (faster than HTML):

```bash
# Extract per-file coverage from Clover XML
php -r "
\$xml = simplexml_load_file('tests/results/coverage.xml');
\$files = [];
foreach (\$xml->project->package as \$pkg) {
    foreach (\$pkg->file as \$file) {
        \$metrics = \$file->metrics;
        \$total = (int)\$metrics['statements'];
        \$covered = (int)\$metrics['coveredstatements'];
        \$pct = \$total > 0 ? round(\$covered / \$total * 100, 1) : 100;
        \$files[] = ['file' => (string)\$file['name'], 'pct' => \$pct, 'total' => \$total, 'uncovered' => \$total - \$covered];
    }
}
// Also check for files directly under project (no package)
foreach (\$xml->project->file as \$file) {
    \$metrics = \$file->metrics;
    \$total = (int)\$metrics['statements'];
    \$covered = (int)\$metrics['coveredstatements'];
    \$pct = \$total > 0 ? round(\$covered / \$total * 100, 1) : 100;
    \$files[] = ['file' => (string)\$file['name'], 'pct' => \$pct, 'total' => \$total, 'uncovered' => \$total - \$covered];
}
// Sort by uncovered statements descending (biggest gaps first)
usort(\$files, fn(\$a, \$b) => \$b['uncovered'] <=> \$a['uncovered']);
// Print as table
printf(\"%-60s %6s %6s %8s\n\", 'FILE', 'COV%', 'TOTAL', 'UNCOV');
printf(\"%s\n\", str_repeat('-', 82));
\$totalStatements = 0; \$totalCovered = 0;
foreach (\$files as \$f) {
    \$basename = basename(\$f['file']);
    \$dir = basename(dirname(\$f['file']));
    printf(\"%-60s %5.1f%% %6d %8d\n\", \$dir.'/'.\$basename, \$f['pct'], \$f['total'], \$f['uncovered']);
    \$totalStatements += \$f['total'];
    \$totalCovered += \$f['total'] - \$f['uncovered'];
}
\$overall = \$totalStatements > 0 ? round(\$totalCovered / \$totalStatements * 100, 1) : 0;
printf(\"\n=== OVERALL: %s%% (%d/%d statements) ===\n\", \$overall, \$totalCovered, \$totalStatements);
" 2>/dev/null
```

Record the baseline coverage percentage and list of uncovered files.

### 2. Build the coverage priority queue

Create a prioritized list of files to test, sorted by **uncovered statements** (largest gaps = most impact per test written):

**Exclude from testing:**

- Config files, service providers, facades (low value)
- Migration files
- Files with < 5 uncovered statements (diminishing returns)

**Priority tiers:**

1. **Tier 1 — High impact**: Files with > 30 uncovered statements + clean code
2. **Tier 2 — Medium impact**: Files with 10–30 uncovered statements + clean code
3. **Tier 3 — Low impact**: Files with 5–10 uncovered statements + clean code

### 3. Triage each file for code smells (BEFORE writing tests)

For each file in the priority queue, **read it first** and check for code smells:

**Smell checklist:**

- [ ] Dead code (methods never called, `@deprecated` with no callers)
- [ ] Duplicated logic between files
- [ ] Wrong abstraction (concrete types where interfaces should be, god class, etc.)
- [ ] Tangled dependencies (constructor with 8+ params, circular calls)
- [ ] Magic strings/numbers everywhere
- [ ] Method does 3+ unrelated things
- [ ] Commented-out code blocks

**If smells are found:**

1. **Skip testing the file** — don't write tests for smelly code
2. Create a debt doc: `docs/YYYY-MM-DDTHHMM--coverage-debt-<slug>.md` with `> Status: Debt`
3. Log the skip with reason in the progress table
4. Move to next file

**If clean:** Proceed to write tests.

> [!IMPORTANT]
> **The smell check is a quick scan, not a deep audit.** Spend ~30 seconds per file. If a file is mostly clean with one minor smell, test the clean parts and note the smell. Only skip the file entirely if the smell is structural (wrong abstraction, dead code path, tangled deps).

### 4. Write tests in batches

For each clean file (or batch of related files):

#### a. Identify test targets

Read the file and identify:

1. **Main code paths** (happy path for each public method)
2. **Primary error handling** (explicit catch blocks, guard clauses, validation)
3. **Edge cases** (empty input, null handling, boundary conditions) — ONLY after main paths

#### b. Check for existing tests

```bash
# Find existing test coverage for this file
grep -rn "ClassName" Tests/Integration/ --include='*.php' | head -5
```

If a test file already exists, **extend it** rather than creating a new one.

#### c. Write the test

Follow project testing conventions:

- Use `PHPUnit\Framework\Attributes\Test` attribute style
- Use Mockery for mocking
- Follow the same patterns as existing test files (check neighbors)
- Use the `TestCase` base class from the module

**Test structure per file:**

```php
#[Test]
public function itDoesMainThing(): void          // happy path
#[Test]
public function itHandlesErrorCase(): void       // primary errors
#[Test]
public function itHandlesEdgeCase(): void        // edge cases (after breadth)
```

#### d. Run tests after each batch

// turbo

After writing tests for each batch (1–3 files), run the full test suite:

```bash
make artisan-test
```

**If tests fail:**

- Fix test issues immediately (wrong mock setup, missing dependencies)
- Do NOT fix production code bugs discovered during testing — file as debt
- Ensure all tests pass before moving to the next batch

### 5. Track progress

Maintain a progress table (in conversation or artifact) showing:

| File                 | Statements | Covered | Status           | Notes                   |
| -------------------- | ---------- | ------- | ---------------- | ----------------------- |
| WooConnection.php    | 200        | 45%     | ✅ Tests written | +8 tests                |
| MagentoProduct.php   | 150        | 30%     | 🗑️ Debt          | Dead code in buildQuery |
| StoreSyncService.php | 80         | 60%     | ✅ Tests written | +4 tests                |

After each batch, re-run the coverage parser from step 1 to check progress toward the target.

### 6. Iterate until target reached

Repeat steps 3–5 moving down the priority queue until:

- **Target reached** (e.g., 80%) — stop, report
- **Only smelly files remain** — stop, report, all debt docs filed
- **Diminishing returns** — remaining files have < 5 uncovered statements each

### 7. Final verification

// turbo

Run the full test suite one final time and parse coverage:

```bash
make artisan-test
# Re-run the coverage parser from step 1
```

### 8. Report

Summarize:

- **Baseline coverage**: X% → **Final coverage**: Y%
- **Tests written**: N new tests across M files
- **Debt filed**: K files skipped with reasons
- **Remaining gap**: What would be needed to close the remaining gap (if any)
- **True blockers**: Any files that are genuinely untestable (external deps, etc.)

Provide copy-paste git commit message:

```
test(coverage): raise coverage from X% to Y%

- Add N tests across M test files
- Skip K smelly files as debt (see docs/)
- Focus: main code paths and primary error handling

Coverage: X% → Y% (target: 80%)
```
