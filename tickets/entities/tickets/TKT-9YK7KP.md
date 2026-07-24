---
id: TKT-9YK7KP
type: ticket
title: Patch brace-expansion DoS advisory in frontend devDeps (GHSA-3jxr-9vmj-r5cp)
kind: chore
priority: medium
effort: xs
status: done
---

## Problem

Dependabot flagged 2 high-severity alerts for the transitive npm dependency
`brace-expansion` in `frontend/package-lock.json` (GHSA-3jxr-9vmj-r5cp — DoS via
exponential-time expansion of consecutive non-expanding `{}` groups):

- `brace-expansion` 2.1.1 (< 2.1.2), via `@vue/test-utils` → js-beautify → minimatch
- `brace-expansion` 5.0.6 (< 5.0.7), via eslint → minimatch

Both are **devDependencies** (test tooling + linter), never shipped to users, so
real-world risk is low and `govulncheck` (the Go call-graph check CI enforces)
was already clean. This clears the dependency-graph alerts.

## Change

`npm audit fix` (non-breaking): bump `brace-expansion` 2.1.1 → 2.1.2 and 5.0.6 →
5.0.8. Only `frontend/package-lock.json` changes (transitive; no `package.json`
edit).

## Acceptance criteria

- `npm audit` reports 0 vulnerabilities.
- Frontend unit tests pass; `npm run lint` and `npm run build` unaffected.
- Dependabot alerts for GHSA-3jxr-9vmj-r5cp close.
