---
id: BUG-RNM0MU
type: bug
title: testifylint finding in sqlite-tagged test is not caught by CI lint
description: golangci-lint in CI runs without build tags, so a testifylint finding in a sqlite-tagged test reached develop.
priority: low
why1: The test asserts a boolean with require.Equal(t, expr, true) instead of require.True, which testifylint's bool-compare rule rejects.
why2: The finding was never reported. The file is behind //go:build sqlite and CI runs golangci-lint run ./... without build tags, so the linter excludes the file.
why3: Lint was set up when the default build was the only build. The sqlite and postgres tags added later got their own test and dependency jobs but no lint job.
why4: Nothing checks that every build-tag combination is covered by every quality gate. Tests and go list -deps were extended per tag; lint was not.
why5: Quality gates are configured per tool rather than per build variant, so adding a build tag does not automatically extend lint coverage to the files it guards.
prevention: Fixed the assertion. Extending CI lint to the sqlite and postgres tags is a separate decision; it first needs the 22 existing tagged findings cleared.
started: "2026-10-03"
completed: "2026-10-03"
status: done
---

## Summary

With the `sqlite` build tag, testifylint reports
`internal/store/sqlitestore/graphquery_naive_test.go:86`. The line uses
`require.Equal(t, <bool>, true, ...)` where it should use `require.True`.

CI runs `golangci-lint run ./...` with no build tags. Files behind the `sqlite`
tag are never type-checked by the linter, so the finding landed on develop
unnoticed (#1671).

## Reproduction

```bash
golangci-lint run --build-tags sqlite ./internal/store/sqlitestore/
```

## Fix

Replace the assertion with `require.True`. Whether CI should lint each backend
tag is a separate decision.

## Related findings (not fixed here)

The same blind spot hides other findings on develop at deb190a0:

- `sqlite` tag: 2 `unused` findings (`noopSQLiteCloser` in
`internal/appbuild/appbuild_sqlite.go`).
- `postgres` tag: 20 findings (govet, misspell, revive, testifylint, and
others).
- `memorybackend` tag: clean.
