---
id: TKT-NEUH2
type: ticket
title: Frontend test fixture builders + ESLint rule banning hardcoded ID literals
kind: test
priority: low
effort: s
status: backlog
---

## Problem

Review-response pattern analysis surfaced ~10 distinct findings about hardcoded
entity IDs (`FEAT-001`, `BUG-001`, `TASK-001`) embedded in Vue components, e2e
specs, and TS fixtures. The Go side has fluent builders
(`internal/testutil/fixtures.go`); the frontend does not, and there is no ESLint
rule preventing the pattern from spreading.

CLAUDE.md already mandates fluent builders / no-hardcoded-IDs; the gap is
*enforcement* on the frontend.

## Scope

**In scope**

- Add a TS fluent builder (entity, relation) under `frontend/src/test/` or
similar, mirroring `internal/testutil/fixtures.go` semantics (auto-generated IDs
unless explicitly set).
- Add an ESLint rule to `eslint.config.js` that flags string literals
matching `/^[A-Z]+-\d{3,}$/` outside fixture loaders / test data directories.
Same rule for the `e2e/` workspace.
- Migrate one or two example test files to use the new builder so the
pattern is visible to future contributors.

**Out of scope**

- Bulk-migrating all ~50+ hardcoded-ID test sites in one PR. Migrate the
most-touched files; the lint rule does the rest of the work over time.

## Acceptance criteria

- New ESLint rule fails on a hardcoded `BUG-001`-style literal in a
non-fixture file.
- Builder API documented in `frontend/CLAUDE.md` Test section.
- At least one existing test migrated as the canonical example.
