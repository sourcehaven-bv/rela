---
id: BUG-J3YJCC
type: bug
title: internal/dataentry/e2e_test.go does not compile under -tags e2e (NewApp signature drift)
description: 'go vet -tags e2e ./internal/dataentry/ fails to compile: e2e_test.go calls NewApp with 9 arguments but the current signature takes 12 (store.VersionService, search.VisibleSearcher and state.KV were added since). The e2e build tag is therefore already red on develop, so nothing guarded by it can catch regressions. Pre-existing; found during the TKT-NS3XPE review.'
priority: medium
effort: s
why1: e2e_test.go calls NewApp with 9 arguments; the current signature takes 12 (store.VersionService, search.VisibleSearcher and state.KV were added since).
why2: Nothing compiles the e2e build tag — the default build excludes the file, so the drift produced no failing signal when NewApp's signature changed.
why3: CI has no job that builds or vets with -tags e2e, so a tag-guarded file can rot indefinitely while every visible gate stays green.
status: backlog
---

Found incidentally during the TKT-NS3XPE code review; **pre-existing on
`develop`, unrelated to that PR.**

## Symptom

```
go vet -tags e2e ./internal/dataentry/
vet: internal/dataentry/e2e_test.go:62:2: not enough arguments in call to NewApp
    have (storage.FS, *project.Context, *metamodel.Metamodel, store.Store,
          entitymanager.EntityManager, search.Searcher, acl.ACL,
          NopFieldVerdictResolver, audit.Audit)
    want (storage.FS, *project.Context, *metamodel.Metamodel, store.Store,
          store.VersionService, entitymanager.EntityManager, search.Searcher,
          search.VisibleSearcher, acl.ACL, FieldVerdictResolver, audit.Audit,
          state.KV)
```

Reproduced on clean `develop` at 11c02c1b.

## Why it matters

The `e2e` build tag is **already red**, so anything guarded by it is effectively
unbuilt. That is not just a stale test — a future regression under that tag has
nothing to catch it, and the next person to touch it inherits a failure they did
not cause. The default build is unaffected, which is exactly why this went
unnoticed.

Note `search.VisibleSearcher` is the ACL-scoped searcher: whichever value the
fix passes should be the gated one, matching how the non-e2e test helpers wire
it — not a raw `search.Searcher`.

## Fix

Update the `NewApp` call in `e2e_test.go` to the current signature, then confirm
`go vet -tags e2e ./internal/dataentry/` and `go build -tags e2e ./...` are
clean.

**Prevention is the real fix:** add a CI step that builds/vets the `e2e` tag,
otherwise this drifts again the next time `NewApp` gains a parameter.
