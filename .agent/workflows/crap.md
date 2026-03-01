---
description: "Analyze CRAP (Change Risk Anti-Patterns) index from coverage report, identify highest-risk methods, and create a /plan to refactor them"
---

# /crap — CRAP Index Reduction

Identify methods with the highest CRAP (Change Risk Anti-Patterns) index from the coverage report and create a refactoring plan to reduce risk.

**CRAP formula**: `CRAP(m) = complexity² × (1 - coverage)³ + complexity`

A high CRAP score means a method is both complex AND poorly tested — the worst combination for maintenance risk. The threshold for "needs attention" is CRAP ≥ 30.

**Input**: Optional CRAP threshold (default: 30). Optional module filter.
**Output**: A prioritized `/plan` doc for refactoring the highest-CRAP methods.

## Steps

### 1. Run tests and extract CRAP data

// turbo

Run the full test suite with coverage to generate fresh data:

```bash
make artisan-test
```

Then extract the CRAP index per method from the Clover XML:

```bash
# Extract CRAP index per method, sorted descending
docker compose exec -T -u 1001:1001 -w /var/www/html app php -r "
\$xml = simplexml_load_file('tests/results/coverage.xml');
\$methods = [];
foreach (\$xml->project->package as \$pkg) {
    foreach (\$pkg->file as \$file) {
        \$filePath = (string)\$file['name'];
        foreach (\$file->line as \$line) {
            \$attrs = \$line->attributes();
            if ((string)\$attrs['type'] === 'method') {
                \$crap = (float)\$attrs['crap'];
                \$complexity = (int)\$attrs['complexity'];
                \$count = (int)\$attrs['count'];
                \$methods[] = [
                    'file' => basename(dirname(\$filePath)).'/'.basename(\$filePath),
                    'method' => (string)\$attrs['name'],
                    'crap' => \$crap,
                    'complexity' => \$complexity,
                    'hits' => \$count,
                    'fullPath' => \$filePath,
                ];
            }
        }
    }
}
usort(\$methods, fn(\$a, \$b) => \$b['crap'] <=> \$a['crap']);
printf(\"%-40s %-30s %6s %6s %6s\n\", 'FILE', 'METHOD', 'CRAP', 'CMPLX', 'HITS');
printf(\"%s\n\", str_repeat('-', 90));
\$shown = 0;
foreach (\$methods as \$m) {
    if (\$m['crap'] < 30) break;  // Only show CRAP >= threshold
    printf(\"%-40s %-30s %6.1f %6d %6d\n\", \$m['file'], \$m['method'], \$m['crap'], \$m['complexity'], \$m['hits']);
    \$shown++;
}
printf(\"\n=== %d methods with CRAP >= 30 ===\n\", \$shown);
" 2>/dev/null
```

> [!TIP]
> Adjust the `if (\$m['crap'] < 30) break;` threshold if you want to see more or fewer results.

### 2. Categorize methods by fix strategy

For each high-CRAP method, read the source and categorize:

**Category A — Test-fixable (high complexity, low/zero coverage)**
Methods that are complex but structurally sound. Adding tests will reduce CRAP without code changes.

- Complexity is appropriate for what the method does
- Logic is clear, just untested

**Category B — Refactor-needed (high complexity, any coverage)**
Methods that are too complex and need structural refactoring. These are the real targets.

- Cyclomatic complexity > 10
- Multiple responsibilities in one method
- Deep nesting (3+ levels of if/for/try)
- Extract patterns: extract method, extract class, replace conditional with polymorphism

**Category C — Skip (external/framework code)**
Methods that are complex due to framework requirements, CLI commands, or boot/config logic.

- Service provider `boot()` / `register()` methods
- Artisan command `handle()` with framework orchestration
- Migration files

### 3. Prioritize by impact

Sort the actionable methods (A + B) by:

1. **CRAP score** (highest first = most dangerous)
2. **Change frequency** — check `git log --follow --format='%H' -- <file> | wc -l` for each file. Higher churn + high CRAP = top priority
3. **Blast radius** — methods called from many places are riskier than isolated ones

### 4. Create refactoring plan

Use `/plan` to create a planning doc targeting the top methods. The plan should include:

For **Category A** methods: Specific test cases to write that will cover the untested paths.

For **Category B** methods: Refactoring approach — which extraction pattern to use, expected complexity reduction.

**Plan naming**: `docs/YYYY-MM-DDTHHMM--crap-reduction-<area>.md`

Each method in the plan should specify:

```markdown
### `ClassName::methodName` (CRAP: XX → target: YY)

**File**: `path/to/file.php`
**Current**: Complexity X, Coverage Y%, CRAP Z
**Strategy**: [Test / Refactor / Extract]
**Changes**:

- [Specific change 1]
- [Specific change 2]
```

### 5. Verify after implementation

// turbo

After implementing changes, re-run the CRAP extraction from step 1 to verify scores decreased:

```bash
make artisan-test
# Re-run CRAP extraction
```

### 6. Report

Summarize:

- **Methods fixed**: N methods across M files
- **CRAP reduction**: Average CRAP before → after
- **Methods remaining**: K methods still above threshold
- **Strategies used**: X tests added, Y methods refactored, Z methods extracted

Provide commit message:

```
refactor(crap): reduce CRAP index for N high-risk methods

- [Test/Refactor] ClassName::method (CRAP XX → YY)
- ...

CRAP ≥ 30 methods: X → Y
```
