---
id: AM-postgres-tag-coverage-complete
type: automated-measure
title: The Postgres CI step covers every postgres-tagged package
kind: ci
location: .github/workflows/ci.yml (Postgres Backend → "Run composition-root wiring tests against PostgreSQL")
status: active
description: 'Extends AM-postgres-tagged-wiring-tests from appbuild+cli to every package holding a postgres-tagged file — adding internal/dataentry and internal/docscli, which CI compiled on every PR and never ran. The load-bearing addition is internal/dataentry: TestStoreEventBridgeCrossProcessSSE is the only end-to-end proof that a write in one process reaches another process''s SSE feed over LISTEN/NOTIFY (TKT-WZYWM9), on the payload path TKT-9TOEBH rewrote. Verified non-vacuous by mutation: with l.store.emit(fe.ev) removed from the pgstore listener, the step fails in 5.2s; unmutated it passes under -race. The step carries the enumerating grep inline so a future tagged package is visibly in or out of the list rather than silently omitted.'
---

## What it catches

A regression in cross-process change delivery — the LISTEN/NOTIFY path behind
live-reload in a multi-process postgres deployment. Before this, that path had
**no** CI coverage at any level: the only test exercising it is postgres-tagged,
and no job ran tagged tests outside `pgstore` and (since BUG-3KQW7P)
`appbuild`/`cli`.

## Known limitation

The completeness property — "the step's package list equals the set of
postgres-tagged packages" — is enforced by a documented invariant and an inline
grep recipe, **not** by an executing check. A new tagged package still relies on
its author reading the comment.

A guard test comparing the two sets would close that, but it needs a home
outside any tagged package (it must run on the default build to be useful) and
would have to parse the workflow YAML. Deliberately not built here; recorded on
BUG-8HT4XN as the stronger follow-up rather than pretended.
