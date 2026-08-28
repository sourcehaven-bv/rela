---
id: AM-postgres-tagged-wiring-tests
type: automated-measure
title: The Postgres CI job runs the composition-root wiring tests, not just the store
description: 'The Postgres Backend job gained a step running `go test -race -tags postgres ./internal/appbuild/... ./internal/cli/...` against its live database. Before this, the job compiled every build-tag combination but scoped its test step to ./internal/store/pgstore/..., so postgres-tagged code in the composition root was compiled on every PR and never executed — which is how 13 failing tests sat on develop unnoticed. Verified to catch a real regression, not just the original symptom: with the state KV dropped from the postgres wiring path (a mutation that compiles and passes untagged), the step fails with "State is nil". Paired with backendtest''s RELA_TEST_DATABASE_REQUIRED gate, which hard-fails rather than skipping when the job''s DSN goes missing — so the measure cannot silently disarm the way an unguarded skip would.'
kind: ci
location: .github/workflows/ci.yml (Postgres Backend → "Run composition-root wiring tests against PostgreSQL")
status: active
---

## What it catches

Two distinct regressions:

1. **A postgres-only wiring break.** Any change to `appbuild`'s postgres recipe
   or the shared `prepare`/`assemble` path that works on fs/memory but not
   postgres. Verified by mutation: nulling the state KV on the postgres path
   compiles and passes the default suite, and this step fails it.

2. **The gate disarming itself.** `RELA_TEST_DATABASE_REQUIRED` (already set as
   job env for the pgstore suite) makes `backendtest` hard-fail on a missing
   DSN instead of skipping — so a dropped env var or renamed service container
   goes red rather than turning the step into a silent no-op.

## Why skips are tolerated here

Unlike the pgstore step, this one does **not** grep for `--- SKIP`. A few cases
in these packages assert boot failures that occur before any store is opened
(`TestDiscover_MissingProject`, `TestNewMCPServices_BadMetamodel`,
`TestDiscover_MalformedACL_FailsBoot`) and legitimately need no database; they
run ungated on every build. The failure mode worth guarding is a *missing DSN*,
which the env var already covers.
