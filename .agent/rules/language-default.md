# Code Editing & Preservation Rules

## Edit Safety

- Target smallest possible section; avoid replacing entire function bodies.
- **Never use escape sequences** (`\n`, `\t`) in replacement content — use actual newlines/whitespace.
- Verify brace balance before and after editing control structures.
- Run syntax check after every edit; do not proceed until it passes.
- View edited region after replacement to confirm correctness.
- Match TargetContent exactly (including whitespace). Preserve surrounding structure and enclosing braces.

## Code Preservation

- Never remove commented-out code unless asked. Preserve all existing comments. Move comments with refactored code.
- Never change logic unless required by the task. Don't "improve" working code unless requested. Fix bugs with minimum changes. Ask before modifying unrequested logic.

## Error Handling

- Prefer exceptions over silent fallbacks — fail loud, not quiet.
- When an error message contains multiple identifiers (advisory IDs, error codes, CVEs), extract and address all of them. Re-read the full error message before implementing the fix.
