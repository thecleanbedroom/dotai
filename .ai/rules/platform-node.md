# Node.js Platform Rules

## Runtime & Dependency Management
- Target Node.js 20.x LTS unless project notes specify otherwise.
- Use npm as the default package manager; lock dependencies with `package-lock.json` committed.
- Keep production dependencies lean; move build/test tools to `devDependencies`.

## Security Practices
- Run `npm audit --production` (or `npm audit fix --force` only with approval) before publishing builds.
- Load secrets exclusively from environment variables or secrets managers; never commit `.env` files.
- Enforce HTTPS for outbound requests and validate external payloads (JSON schema or explicit guards).

## Testing & Linting
- Provide an `npm test` script that runs the full unit/integration suite.
- Add `npm run lint` (ESLint + Prettier) to CI; fail builds on lint errors.
- Prefer fast, deterministic tests; gate long-running e2e suites behind explicit flags.

## Build & Deployment
- Bundle assets via `npm run build` (esbuild/Vite/Webpack) and output to `dist/` or a documented folder.
- Avoid globally installed CLIs in pipelines; rely on local `node_modules/.bin` binaries.
- Document environment variables (e.g., `NODE_ENV`, API keys) in deployment playbooks.

## Observability
- Use structured logging (`JSON.stringify` or libraries like Pino) for services; redact sensitive fields.
- Surface unhandled promise rejections via process hooks and fail fast rather than ignoring errors.
