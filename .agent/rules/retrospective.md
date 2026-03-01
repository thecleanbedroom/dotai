---
trigger: always_on
---

# Retrospective Rules

General-purpose rules captured from development friction. These are staging entries — once validated across multiple sessions, promote them into the appropriate `core-*.md` or `platform-*.md` rule file via `/audit-agent`.

### Match the naming convention of the file you're editing

**Pattern**: Introduced `isBundleShell()` (camelCase) into `ShopifyProduct.php` which uses `snake_case` exclusively. User had to correct.
**Rule**: Before naming a new method/function/variable, scan the file for the dominant convention (snake_case, camelCase, PascalCase). Match it exactly. When in doubt, check 3–5 existing method names in the same class.

### DO NOT USE INLINE FQ Namespaces in code

**Pattern**: Introduced `\Illuminate\Support\Uri::of($url)` directly inside the code body. User had to correct.
**Rule**: Always declare a `use` statement at the top of the file for external dependencies, rather than using Fully Qualified namespaces inline.

### Never change conditional logic semantics without asking

**Pattern**: Converted `strpos($vendor, 'Holy Lamb')` (partial/substring match) to `in_array(normalize_key($vendor), ['holylamb', ...])` (exact match) while refactoring normalization. This silently changed the matching behavior — partial matches that previously passed would now fail. User had to catch and revert.
**Rule**: When refactoring code that contains conditional logic, never change the _type_ of comparison (partial → exact, case-sensitive → insensitive, allowlist → denylist, etc.) without explicitly calling out the behavioral difference and asking the user whether the change is intentional. If the refactoring task is "use normalize_key here", but the existing code does substring matching, say "this is a substring match — normalize_key does exact matching, which would change behavior. Should I leave it as-is?"

### Declare class properties with other properties

**Pattern**: Added `private ?WooApiClient $wooApiClient = null;` inline next to the method that uses it, instead of grouping it with the other property declarations at the top of the class. User had to correct.
**Rule**: Always declare new class properties in the property block at the top of the class, grouped with existing properties. Never declare properties inline next to the methods that use them.
