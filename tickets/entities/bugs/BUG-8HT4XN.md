---
id: BUG-8HT4XN
type: bug
title: 'Postgres CI misses two tagged packages; the cross-process SSE test has never run'
priority: medium
description: 'BUG-3KQW7P added a Postgres-job step for internal/appbuild and internal/cli, but two packages with postgres-tagged files were left out: internal/dataentry and internal/docscli. The dataentry one matters — store_bridge_postgres_test.go holds TestStoreEventBridgeCrossProcessSSE, the only end-to-end proof that a write in one process reaches another process''s SSE feed over LISTEN/NOTIFY (TKT-WZYWM9''s headline feature, and the path TKT-9TOEBH changed the wire format of). CI compiled it on every PR and never ran it. Both packages pass today, so nothing is broken; the defect is the missing gate. Fixed by extending the existing step to cover every postgres-tagged package, with the enumerating grep recorded inline so the list cannot silently fall behind again.'
status: done
why1: 'internal/dataentry and internal/docscli contain postgres-tagged files that no CI job executes.'
why2: 'The Postgres job runs pgstore plus the appbuild/cli step added by BUG-3KQW7P — a list that never included them.'
why3: 'BUG-3KQW7P was scoped to the packages whose tests were observed failing, rather than to every package the build tag changes.'
why4: 'The failing tests were the visible symptom; the two packages here compile and pass under the tag, so they produced no signal to scope toward.'
why5: 'Both this and BUG-3KQW7P share one systemic cause: CI coverage for a build tag is maintained as a hand-written package list with nothing tying it to the actual set of tagged files, so the list drifts silently as tagged code spreads.'
prevention: 'The step now documents the invariant (must cover every postgres-tagged package) and embeds the grep that enumerates the current set, so the next person extending the tag has the check in front of them. A test-level guard that fails when the two sets diverge is the stronger fix — deliberately not built here, since it needs a home outside any tagged package; recorded as the follow-up.'
---

## Description

Follow-up to BUG-3KQW7P, found by asking the question that bug's root cause
implies: *which packages actually have postgres-tagged files?*

```
$ grep -rl '^//go:build postgres' --include='*.go' internal/ cmd/ \
    | xargs -n1 dirname | sort -u
internal/appbuild
internal/appbuild/backendtest
internal/cli
internal/dataentry      <-- not in CI
internal/docscli        <-- not in CI
internal/store/pgstore
```

Both pass locally, so **nothing is broken today**. The defect is the absent
gate, which is what let BUG-3KQW7P's 13 failures accumulate unnoticed.

## Why internal/dataentry is the one that matters

`store_bridge_postgres_test.go` contains `TestStoreEventBridgeCrossProcessSSE`
— the only end-to-end assertion that a write committed by one process reaches
a *different* process's SSE feed via `LISTEN/NOTIFY`. That is the headline of
TKT-WZYWM9, and TKT-9TOEBH later changed that payload's wire format.

The test is sound; it simply never ran. Verified non-vacuous rather than
assumed: with `l.store.emit(fe.ev)` removed from the listener, it fails in
5.2s. (Two other mutations — swapping `kind`/`op` in the payload, dropping the
self-echo filter — correctly pass: the field swap is symmetric across encoder
and decoder, and self-echo is *local* double-emission, which a two-store test
cannot observe by construction.)

`internal/docscli`'s tagged file is a 19-line capturer; it is included for
completeness, not because it carries comparable risk.

## Fix

Extend the existing wiring step rather than adding a parallel one, so the
"cover every tagged package" rule lives in one place:

```yaml
go test -race -tags postgres -run 'TestStoreEventBridge' ./internal/dataentry/
go test -race -tags postgres ./internal/docscli/
```

`internal/dataentry` is `-run`-scoped for the same reason `internal/cli`
already is: the full package needs the command sandbox, and this job's runner
cannot create bubblewrap's loopback device. Those tests are backend-independent
and the `test` job owns them.

Adds ~14s to the job.
