---
description: "Sweep the codebase for unreferenced code, validate with multi-signal analysis, quarantine confirmed dead code via rename, and remove previously quarantined code"
---

# /deadcode — Dead Code Removal

Find and safely remove unreferenced code. Uses multi-signal validation (static analysis + exhaustive search + AI inference) to minimize false positives. Dead code is **quarantined** (renamed) first — only removed after surviving a production cycle with no issues.

**Input**: Optional directory filter, optional `--purge` flag to remove previously quarantined files
**Output**: Quarantined files, debt doc for uncertain cases, removal of previously quarantined files (if `--purge`)

> [!CAUTION]
> **Accuracy over speed.** False positives delete working code. Every phase exists to eliminate false positives. Do not skip phases. Do not batch removals. Do not assume.

## Steps

### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

### Purge previously quarantined files (if `--purge`)

If invoked with `--purge`, skip all discovery/validation and go straight to cleanup:

// turbo

Search for quarantined files (prefixed with `__dead_YYYYMMDD__`):

```bash
find . -name '__dead_*' -not -path '*/vendor/*' -not -path '*/node_modules/*' -not -path '*/.*' | sort
```

For each quarantined file:

1. Parse the date from the `__dead_YYYYMMDD__` prefix
2. If quarantined **>1 week ago** and no errors referencing the original name in logs/monitoring → safe to delete
3. If quarantined **<1 week ago** → skip, needs more soak time
4. If any errors reference the original name → **restore immediately** (it wasn't dead)

Present the purge list for user approval. Delete only approved files.

**After purge, stop.** Do not continue to discovery.

---

### Phase 1 — Candidate Discovery

Cast a wide net. Every signal independently generates candidates. Survival requires failing *all* signals.

#### Run static analysis for unused symbols

// turbo

Run the project's static analysis tool with unused code detection enabled. Consult `.agent/rules/platform-*.md` for the specific command and configuration.

Collect every reported unused class, method, function, constant, and import.

#### Run test coverage baseline

// turbo

Run the full test suite with coverage. Identify every file with **0% coverage** (zero covered statements). These are candidates — not all dead code has 0 coverage, but 0 coverage code is more likely dead.

#### Exhaustive reference search

// turbo

For each source file in the project (excluding vendor, node_modules, build artifacts):

```bash
# List all project source files
find . -type f \( -name '*.php' -o -name '*.ts' -o -name '*.js' -o -name '*.py' -o -name '*.rb' \) \
  -not -path '*/vendor/*' -not -path '*/node_modules/*' -not -path '*/dist/*' -not -path '*/.*' | sort
```

For each source file, extract the primary symbol (class name, module export, function name). Search for references to that symbol across the entire codebase:

- Full qualified name
- Short/unqualified name
- Any aliases or alternative imports

A file is a candidate if its primary symbol has **zero references outside its own file and its test file**.

#### Build master candidate list

Merge results from all three signals. A candidate appears if flagged by **any** signal. Track which signals flagged each:

| File | Symbol | Static Analysis | Zero Coverage | Zero Refs | Signals |
|------|--------|----------------|---------------|-----------|---------|
| path | name   | ✅/—           | ✅/—          | ✅/—      | N/3     |

Sort by signal count descending (3/3 = most likely dead).

---

### Phase 2 — AI Validation Gauntlet

For each candidate, starting with highest signal count:

#### Read and understand the candidate

Read the full file. Determine:

- **What does it do?** Summarize its purpose in one line.
- **Who would call it?** What kind of consumer would use this code?
- **Is it an entry point?** CLI command, route handler, queue job, scheduled task, webhook, event listener, middleware — these are called by the framework, not by your code.
- **Is it part of a convention-based system?** Strategy pattern, plugin architecture, handler maps, auto-discovery — where the framework loads classes by convention rather than explicit reference.

#### Check registration and configuration

Search for the symbol in:

- **Service container / DI configuration** — bindings, providers, registrations
- **Event/listener mappings** — subscriber registrations, hook/filter systems
- **Route definitions** — controllers, middleware, route model binding
- **Config files** — class maps, driver configs, queue connections, cache stores
- **Build/deploy scripts** — entry points not visible to static analysis
- **Documentation** — API docs, README references suggesting external consumers

#### Check for dynamic dispatch patterns

Search for patterns that could reference the symbol dynamically:

- String concatenation building class names
- Variable class instantiation (`new $className`)
- Reflection usage
- Factory patterns using config-driven class maps
- Plugin/extension loading systems
- Convention-based namespaces (`App\Handlers\{$type}Handler`)

#### Classify confidence

| Confidence | Criteria | Action |
|------------|----------|--------|
| **🟢 Certain dead** | 3/3 signals, no dynamic patterns, no registration, not an entry point, no interface implementation, AI agrees it's dead | Quarantine |
| **🟡 Likely dead** | 2/3 signals, no dynamic patterns found but implements an interface or sits in a plugin-style directory | Present to user |
| **🔴 Uncertain** | 1/3 signals, or any dynamic loading pattern found, or AI suspects it might be reachable | Skip — note in debt doc |

---

### Phase 3 — Quarantine

For each 🟢 candidate (and user-approved 🟡 candidates):

#### Rename to quarantine

Rename the file with a `__dead_YYYYMMDD__` prefix (today's date). Do **not** delete — the rename makes the code unreachable while keeping it recoverable, and the date tells you exactly when it was quarantined.

```bash
# Example: quarantine a file (2026-03-06)
mv path/to/DeadClass.ext path/to/__dead_20260306__DeadClass.ext
```

Also quarantine the corresponding test file if one exists.

#### Run full test suite

// turbo

After each quarantine, run the full test suite immediately. If any test fails:

1. **Revert the rename instantly** — this code is not dead
2. Remove from candidate list
3. Reclassify as 🔴 — the test suite proved it's referenced

> [!IMPORTANT]
> **One file at a time.** Never batch quarantines. If quarantining file A and B together causes a failure, you won't know which one is referenced.

#### Run static analysis

// turbo

After each successful quarantine, run static analysis. If new errors appear referencing the quarantined symbol:

1. **Revert the rename** — a reference exists that grep missed
2. Reclassify as 🔴

#### Commit each quarantine

After each successful quarantine (tests green, static analysis clean), commit:

```
refactor(deadcode): quarantine <SymbolName>

Renamed to __dead_YYYYMMDD__ prefix. Remove after production soak
period with no errors referencing this symbol.

Signals: <N>/3 (static analysis, zero coverage, zero refs)
```

---

### Phase 4 — Report

#### File debt for uncertain cases

For any 🔴 candidates, follow `/close`'s _Create debt doc_ step. Set source to `/deadcode sweep`. Include the signal data and the reason for uncertainty.

#### Present results

Summarize:

- **Quarantined**: N files (list with signal counts)
- **User-approved**: M files (from 🟡 candidates)
- **Skipped as uncertain**: K files (filed as debt)
- **False positives caught**: J files (reverted after test/analysis failure)

Remind the user:

> Quarantined files are renamed with `__dead_YYYYMMDD__` prefix. They remain in the codebase but are unreachable. The date stamp shows exactly when they were quarantined. After running in production for 1+ weeks with no issues, run `/deadcode --purge` to permanently remove them.
