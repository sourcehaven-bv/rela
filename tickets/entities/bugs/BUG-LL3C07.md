---
id: BUG-LL3C07
type: bug
title: TestBuild_StateRows_WarnAtStartup fails on the sqlite build; CI never runs sqlite-tagged tests
description: 'The test seeds entities as markdown files, which the sqlite store never reads, so no content-state rows exist and the startup warning it asserts is correctly absent. Reproduced on clean develop. The larger issue: ci.yml only builds the sqlite tag, so no sqlite-tagged test ever runs on a PR — including sqlitestore''s own conformance suite.'
priority: medium
why1: The assertion fails because no content-state warning is emitted at startup.
why2: No warning is emitted because the store holds no content-state rows.
why3: The store holds no rows because the fixture seeds markdown files into the project directory, which is how fsstore discovers entities — the sqlite backend reads .rela/rela.db and never looks at them.
why4: The backend-specific fixture went unnoticed because the test was written against the default build and inherited by the sqlite build unchanged.
why5: It stayed broken because ci.yml exercises the sqlite tag only through go build and go list -deps; no job runs `go test -tags sqlite`, so the entire sqlite test surface is ungated on every PR.
prevention: 'Two changes, and the second is the one that matters. (1) The test now excludes the sqlite tag explicitly, matching the exclusion it already had for postgres for the identical reason. (2) A `Run sqlite-tagged tests` step in the Backends CI job runs `go test -tags sqlite` over internal/store, internal/appbuild and internal/cli — verified to fail against the reverted fix, so the class of defect is now caught rather than the instance. The general lesson: a build tag that CI only COMPILES is untested, and adding a backend under an existing `!postgres` exclusion silently opts it into every fs-shaped fixture.'
status: done
---

## Symptom

```
go test -tags sqlite ./internal/appbuild/
--- FAIL: TestBuild_StateRows_WarnAtStartup
    appbuild_states_warn_test.go:55: startup warning missing "content-state rows"
    appbuild_states_warn_test.go:55: startup warning missing "analyze states"
```

Reproduced on a clean `develop` worktree, so this is **not** introduced by any
in-flight branch. Found while running the sqlite suite for TKT-S1EVV7.

## Cause

The test seeds its fixture by writing **markdown files** into the project
directory (`writeEntityFile` → `entities/tickets/TKT-001@draft.md`). That is how
fsstore discovers entities. The sqlite backend reads its rows from
`.rela/rela.db` and never looks at those files, so the store has no
content-state rows, so the startup check correctly emits no warning — and the
assertion fails.

The test asserts a property of the *warning logic* but reaches it through a
*store-specific* seeding path. It is right on the default build and inapplicable
as written on sqlite.

## Why nobody noticed

`ci.yml` exercises the `sqlite` tag only through `go build` and `go list -deps`
(lines 570-602). **No CI job runs `go test -tags sqlite`.** So the entire
sqlite-tagged test surface — including `internal/store/sqlitestore`'s
conformance suite — is unverified on every PR.

That is the more important half of this bug. The failing assertion is one
symptom; the gap is that a backend with a full conformance harness has none of
it gated.

## Fix sketch

Two parts, and the second matters more:

1. Seed through the store rather than the filesystem, so the fixture is
backend-neutral — or skip on backends whose fixtures cannot express
content-state rows, stating which and why.
2. Add a `go test -tags sqlite` job to `ci.yml`. Scope it deliberately: the
sqlite build's own packages plus the shared ones whose behaviour it changes,
rather than the whole tree twice.

Expect (2) to surface further pre-existing failures; triage them in the same
pass rather than one ticket per assertion.
