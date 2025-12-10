# JavaScript Rules

## Style Guide

- Prefer modern ES modules; use `import`/`export` syntax wherever supported.
- Enforce semicolons, single quotes, 2-space indentation, trailing commas (ES5), and bracket spacing.
- Keep arrow functions concise but always include parentheses around parameters for readability.

## Code Organization

- Place shared utilities in dedicated modules; avoid deep relative paths by using alias configs when available.
- Co-locate component files (JS + CSS) when it improves clarity, but avoid circular dependencies.
- Name scripts and components using `camelCase` filenames; tests follow `*.test.{js,ts,jsx,tsx}`.

## Type Safety

- Favor TypeScript or JSDoc typedefs for complex structures; provide explicit interfaces for API payloads.
- Validate external data with runtime guards (e.g., `zod`, `io-ts`, custom predicates) before use.

## Testing

- Provide `npm test` or `npm run test:js` scripts that cover unit and integration cases.
- Mock WordPress globals (e.g., `wp.i18n`) when running tests outside the browser.
