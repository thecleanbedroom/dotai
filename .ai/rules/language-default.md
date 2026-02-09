## Code Editing Safety Rules

### Replacement Content Rules

- **Prefer small, focused edits** - When replacing code, target the smallest possible section. Avoid replacing entire function bodies when only a few lines need to change.
- **Never use escape sequences in replacement content** - ReplacementContent must contain actual newlines and whitespace, not `\n` or `\t` escape sequences. These will be inserted literally and corrupt the file.
- **Verify brace balance** - Before and after editing control structures (if/foreach/function), verify the opening and closing braces are balanced. Count them if necessary.

### Post-Edit Verification

- **Run syntax check after every edit** - Always run immediately after editing files. Do not proceed until syntax passes.
- **View the edited region after replacement** - After making an edit, view the modified lines to confirm the change was applied correctly before continuing.

### Multi-line Replacement Safety

- **Match TargetContent exactly** - The TargetContent must match the file exactly, including all whitespace and indentation. View the target lines immediately before editing to capture exact content.
- **Preserve surrounding structure** - When replacing code inside a block, ensure the replacement maintains all enclosing braces: if the target includes `}`, the replacement must include it too.

## Code Preservation Rules

### Comment Preservation

- **Never remove commented-out code** unless explicitly asked. Commented code often represents intentional placeholders, debugging aids, or future work.
- **Preserve all existing comments** including inline comments, block comments, and documentation comments.
- If refactoring moves code, move its associated comments with it.

### Logic Stability

- **Never arbitrarily change logic** - Only modify logic when explicitly required to complete the requested task.
- Do not "improve" working code unless the improvement is directly requested.
- When fixing a bug, change only the minimum code necessary to resolve the issue.
- If you believe logic should be changed but it wasn't requested, ask first before modifying.
