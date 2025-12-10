# CSS / SCSS Rules

## Style Guide

- Use 2-space indentation and double quotes for attribute values.
- Prefer BEM or another documented naming convention to avoid selector collisions.
- Keep selectors shallow; avoid chaining more than three levels deep.

## Organization

- Group related components into partials and import them into entry stylesheets.
- Store styles in `kebab-case` filenames, matching the component or block name.
- Centralize variables, breakpoints, and mixins in dedicated utility files.

## Preprocessing

- Use SCSS or PostCSS when needed for variables and nesting, but compile to plain CSS for production.
- Autoprefix builds based on the project’s supported browsers list; do not hand-write vendor prefixes.

## Performance & Accessibility

- Minimize !important usage; rely on specificity instead.
- Use CSS custom properties or utility classes for reusable values (spacing, color, typography).
- Ensure contrast ratios meet WCAG AA; document color palettes and semantic tokens.
- Prefer `prefers-reduced-motion` guards for animations; disable heavy effects for reduced-motion users.
