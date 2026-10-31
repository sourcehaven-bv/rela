---
id: TKT-MLH774W
type: ticket
title: 'Fix osv-scanner findings in frontend lockfile (GH #1989)'
kind: chore
priority: medium
effort: xs
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

## Problem

The scheduled security scan (GitHub #1989) reported one osv-scanner finding, and
osv-scanner reports three newer ones on the same lockfile:

- GHSA-p458-7748-mcp3 (medium): npm `@vitest/browser` and `@vitest/ui` 5.0.3
in `frontend/package-lock.json`, fixed in 5.0.5. Both are dev dependencies.
`@vitest/ui` comes in through `vitest`. `@vitest/browser` comes in through
`@vitest/browser-playwright`, `@vitest/coverage-v8` and storybook's
`@storybook/addon-vitest` in the `rela-components` workspace.
- GHSA-mrwp-f3q8-3g9p (high): npm `follow-redirects` 1.16.1, fixed in 1.16.2.
It comes in through `axios` (`^1.16.0`).
- GHSA-3g3m-2fjr-6pw4 (medium): npm `glob` 13.0.6 (dev), fixed in 14.0.2. It
comes in through `js-beautify` (`^13.0.6`), a dependency of `@vue/test-utils`.
- GHSA-g3hv-75wg-m249 (medium): npm `minimatch` 10.2.6 (dev), fixed in 11.0.1.
It comes in through `eslint`, `@eslint/config-array`, `typescript-eslint`,
`editorconfig` and `glob`, which all require minimatch 10.

## Change

- Raise `vitest` to `^5.0.5` in `frontend/package.json`.
- Raise `vitest`, `@vitest/browser-playwright` and `@vitest/coverage-v8` to
`^5.0.5` in `frontend/packages/rela-components/package.json`.
- Regenerate `frontend/package-lock.json` so every vitest 5 package resolves to
5.0.5. The vitest packages pin each other as exact peers, so they must share one
version.
- Bump `follow-redirects` to 1.16.2 in the lockfile; axios's range allows it.
- Add npm `overrides` for `glob` (`^14.0.2`) and `minimatch` (`^11.0.1`). No
release of their parents accepts the fixed majors. The consumers only use `new
Minimatch(pattern, { dot: true })` and `globSync` (js-beautify's CLI, which the
tests do not run). Minimatch 11 removes `makeRe`, `regexp`, `AST` and the
`debug` option, and none of the consumers use them.

## Acceptance criteria

- osv-scanner reports no findings on `frontend/package-lock.json`.
- `npm ls vitest @vitest/ui @vitest/browser` shows 5.0.5 only; `npm ls`
shows follow-redirects 1.16.2, glob 14.x and minimatch 11.x.
- Frontend unit tests, typecheck, lint and build pass, and the component
library checks and tests pass.

## Follow-up

Remove the `glob` and `minimatch` overrides from `frontend/package.json` when
eslint, typescript-eslint, editorconfig and js-beautify accept minimatch 11 and
glob 14 themselves.
