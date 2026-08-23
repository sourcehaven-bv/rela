---
id: TKT-8TJ2WN
type: ticket
title: 'storetest: cover Freshness.LastModified and declare the Tx tier in Capabilities'
kind: test
priority: medium
effort: s
status: in-progress
---

## Description

Two gaps in the shared conformance suite that let a backend pass while being
subtly wrong. Both were found surveying pgstore for work that would make the
SQLite backend (DEC-LFSYNY) safer to land, and both are the same shape as
TKT-415WA7: a contract that exists in prose but is not pinned.

## 1. `Freshness.LastModified` had ZERO conformance coverage

`grep -rn "LastModified" internal/store/storetest/` returned nothing. It is a
mandatory member of `store.Store` (`store.go:179`, contract at
`store.go:220-224`) with three genuinely different implementations:

- fsstore — max of four filesystem mtimes (`fsstore.go:325`)
- memstore — scan of two maps (`memstore.go:115`)
- pgstore — SQL `max()` over a `UNION ALL` (`pgstore.go:202`)

It gates search-index rebuild (`appbuild_fs.go:126,236`), so a wrong answer
degrades **silently**: a backend that returns `time.Now()`, forgets relations,
or returns non-zero when empty passes the entire suite and serves stale search
results with no error anywhere.

Extra trap for a SQL backend: a TEXT timestamp stored without a timezone parses
back to a time that compares wrong against every consumer's clock.

## 2. The transaction tier was not declared anywhere

`RunTxRollbackTests` is not part of `RunAll`; it was reachable only by calling
it separately, which exactly one backend did (`pgstore/conformance_test.go`).
Nothing recorded that a backend *claims* the strong tier, so a backend with
genuine rollback whose author forgot the extra call got zero coverage of its
most safety-critical behaviour — and the silence was indistinguishable from
fsstore/memstore, which omit it deliberately.

DEC-LFSYNY puts SQLite at exactly that tier, so this is directly in the path.

## Scope

IN: a `RunFreshnessTests` suite wired into `RunAll`; a `Capabilities.TxRollback`
flag that runs the rollback suite from `RunAll`; pgstore declaring it.

OUT: fixing any backend. All three pass as-is — these tests pin existing correct
behaviour so a *fourth* backend cannot get it wrong silently.

## Acceptance criteria

1. `RunFreshnessTests` covers: zero time when empty, advances on entity write,
**covers relation writes**, stable across reads, and a plausibility check that
catches a timezone-less timestamp.
2. All three existing backends pass unchanged.
3. The suite is non-vacuous — verified by deliberate regression, not just by
going green.
4. `Capabilities.TxRollback` runs `RunTxRollbackTests` from `RunAll`; pgstore
sets it and drops its separate entry point.
5. `just lint` (incl. `--build-tags postgres`), `just arch-lint` clean.
