---
id: TKT-MLH774W
type: ticket
title: 'Bump vitest to 5.0.5 for GHSA-p458-7748-mcp3 (GH #1989)'
kind: chore
priority: medium
effort: xs
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

## Problem

The scheduled security scan (GitHub #1989) reported one osv-scanner finding:

- GHSA-p458-7748-mcp3 (medium): npm `@vitest/browser` and `@vitest/ui` 5.0.3
in `frontend/package-lock.json`, fixed in 5.0.5. Both are dev dependencies.
`@vitest/ui` comes in through `vitest`. `@vitest/browser` comes in through
`@vitest/browser-playwright`, `@vitest/coverage-v8` and storybook's
`@storybook/addon-vitest` in the `rela-components` workspace.

## Change

- Raise `vitest` to `^5.0.5` in `frontend/package.json`.
- Raise `vitest`, `@vitest/browser-playwright` and `@vitest/coverage-v8` to
`^5.0.5` in `frontend/packages/rela-components/package.json`.
- Regenerate `frontend/package-lock.json` so every vitest 5 package resolves to
5.0.5. The vitest packages pin each other as exact peers, so they must share one
version.

## Acceptance criteria

- osv-scanner reports no GHSA-p458-7748-mcp3 finding on
`frontend/package-lock.json`.
- `npm ls vitest @vitest/ui @vitest/browser` shows 5.0.5 only.
- Frontend unit tests, typecheck, lint and build pass, and the component
library checks and tests pass.
