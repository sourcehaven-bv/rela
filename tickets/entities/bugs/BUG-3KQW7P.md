---
id: BUG-3KQW7P
type: bug
title: 'Composition-root tests fail under -tags postgres; CI compiles the tag but never runs or lints it'
priority: medium
description: '13 composition-root tests in internal/appbuild and internal/cli fail under -tags postgres because they inherit the postgres recipe''s DSN requirement while asserting build-agnostic wiring. CI never caught it: the Postgres job compiles every build-tag combination but scopes its test step to internal/store/pgstore, and golangci-lint runs untagged, so 13 lint findings were invisible too. Fixed with internal/appbuild/backendtest (supplies each build a backend; hard-fails rather than skips when the CI DSN is missing) plus a Postgres-job step running the wiring packages against the live database.'
status: done
why1: '13 tests in internal/appbuild and internal/cli fail under `-tags postgres`, and 13 lint findings appear only with the tag.'
why2: 'The failing tests call appbuild.New/Discover, and the postgres recipe refuses to open a store without a DSN — so each fails with "postgres build requires a database URL".'
why3: 'They are build-agnostic composition-root assertions (Services populated, Close idempotent, acl.yaml loaded) that need SOME store, not a postgres one — but no seam existed to hand a tagged build a usable backend, so they inherited the recipe''s DSN requirement.'
why4: 'Nothing ran them: the Postgres Backend job compiles every build-tag combination but scopes its test step to ./internal/store/pgstore/..., and golangci-lint runs untagged — so tagged code outside pgstore was compiled, never executed, never linted.'
why5: 'CI coverage for a build tag was scoped to the package that motivated the tag (the store) rather than to everything the tag changes. The store was treated as the backend-specific surface, but the composition root is build-tagged too — so its postgres half had no gate at all and silently rotted.'
prevention: 'Added internal/appbuild/backendtest so a tagged build can supply its own backend, and extended the Postgres job to run the wiring packages against the live database. When adding a build tag, gate every package the tag changes, not just the one it was introduced for.'
---

## Description

Found while running the postgres-tagged suite by hand during the multi-tenancy
work — not reported by CI, because CI never runs it.

**13 test failures** under `-tags postgres`, all identical in shape:

```
--- FAIL: TestDiscover_BuildsAllServices
    appbuild: postgres build requires a database URL (set RELA_DATABASE_URL)
```

**13 lint findings** that appear only with the tag: gosec G706, an `err`
shadow, four misspellings, an unused `nolint` directive, a whitespace nit.

## Why CI missed it

| Job | Covers |
|---|---|
| Postgres Backend | compiles all tag combinations; **runs tests only for `./internal/store/pgstore/...`** |
| Lint | **untagged only** |
| Cross-Compile | compiles `linux/darwin/windows × postgres` |

So postgres-tagged code outside `pgstore` was compiled on every PR but never
executed and never linted.

## Root cause

The failing tests assert **build-agnostic** composition-root behaviour: that
`Services` comes back fully populated, that `Close` is idempotent, that
`acl.yaml` loads into a `Declarative`, that a corrupt CalDAV cache doesn't brick
boot. They call `New`/`Discover` because that is the wiring entry point — not
because they test a store. The postgres recipe requires a DSN, so all of them
failed for a reason unrelated to what they assert.

Two things confirm this reading. `appbuildtest` **passed** throughout, because it
assembles over a memstore instead of going through the recipe. And within the
same files, tests asserting failures that occur *before* the backend opens
(`TestDiscover_MissingProject`, `TestNewMCPServices_BadMetamodel`) also passed.
That boundary is the seam the fix builds on.

## A latent test bug this surfaced

`TestCorruptAliasTableDoesNotBrickTheBinary` wrote a corrupt
`.rela/caldav/aliases.json`. Since TKT-VC27L3 the alias table is a `state_kv`
row on the postgres build — so there it corrupted a file nothing reads, the
table loaded clean, and the assertion failed **against correct production
code**. It was a filesystem test wearing a wiring test's name.

## Fix

`internal/appbuild/backendtest` supplies whatever the current build needs to
reach a store: nothing on fs/memory, a private migrated schema on postgres.

Skip policy follows `pgstore`'s precedent (RR-0EWZQW): without a database the
tests skip so a developer's `go test -tags postgres ./...` stays clean, but
`RELA_TEST_DATABASE_REQUIRED` — the same variable, not a second one — turns a
missing DSN into a hard failure. Tests that assert boot failures before any
store opens stay **ungated**, so they don't degrade into skips.

The corrupt-alias test now writes through `state.KV`, where the table lives on
either backend. Verified non-vacuous by mutation.

The Postgres job gained a step running the wiring packages against the same
database (and bubblewrap, which `internal/cli`'s render tests need).

## Verification

Against PostgreSQL 15: the 13 previously-failing tests pass, including under
`-race`; both builds lint clean; the full default suite is green; the strictness
gate hard-fails when the DSN is missing. A simulated postgres-only wiring
regression (dropping the state KV) is caught by the new step — it would have
merged silently before.
