# Php Coding Rules

## PHP Development Rules

## General PHP Standards

- Follow PSR-12 coding style
- Use strict types: `declare(strict_types=1);`
- Use type hints for all method parameters and return types
- Use `?` for nullable types
- Use constructor property promotion where applicable
- Group use statements by type (classes, functions, constants)
- Instead of deep nesting conditionals, return early

## Code Organization

- Use namespaces following PSR-4
- One class per file
- Class names in `StudlyCaps`
- Method names in `camelCase`
- Property names in `camelCase`
- Constants in `UPPER_CASE`
- Document all classes, methods, and properties with PHPDoc blocks
- Keep methods small and focused (single responsibility)
- Use return type declarations
