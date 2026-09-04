---
id: TKT-NLX424
type: ticket
title: 'Extract nextActionAPI off dataentry.App: the advisory-suggestion adapter with mutex-guarded late-set state (~80 → ~69)'
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
status: ready
---

Sub-ticket of [[TKT-R68TV8]] (the `dataentry.App` arc under [[TKT-N0IKN9]]).

## Why this is an abstraction

The next-action cluster is a genuine domain adapter, not "the handlers for one
route": it binds `internal/nextaction` to this package's ACL-gated read paths,
owns a per-user feedback backend, has its own world-resolution rule (the source
world REPLACES the request world, nextaction.go:44), and carries its own
redaction obligation (`redactedForSuggestion`, closing the BUG-R9EHKV class
through a distinct door). The reason it lives in `dataentry` and not in
`nextaction` is stated at nextaction.go:20: the read gate is per-request here.

It also fixes a latent race: `SetUserState` writes `a.userState`
(nextaction.go:333) and `SetNextActionMatchers` writes `a.nextActionMatchers`
(:342) with no synchronisation while `handleV1NextActionPost` reads them
(nextaction_handler.go:157). Safe today only by the wire-before-serve
convention. The new type guards both behind a mutex, so it is safe by
construction.

## What moves (11 methods)

`handleV1NextAction` (nextaction_handler.go:92), `handleV1NextActionGet` (:103),
`handleV1NextActionPost` (:155), `nextActionEngine` (:288),
`nextActionCandidates` (nextaction.go:41), `nextActionOptions` (:161),
`queryCandidates` (:216), `redactedForSuggestion` (:233), `countCandidates`
(:255), `countIsZero` (:277). `applyNextActionFeedback`
(nextaction_handler.go:231) has an unused receiver — package function.

`SetUserState` / `SetNextActionMatchers` **stay on App** as one-line
delegations: they are public opt-in wiring API.

## Shape

```go
type nextActionAPI struct {
    schema      func() *Schema
    worlds      func() WorldLookup     // closure: SetWorlds runs AFTER NewApp
    queries     *queryService
    affordances affordanceService
    // Two ACL postures a source may take, named separately because the
    // difference between them IS the count_ungated disclosure decision
    // (see countCandidates' doc). One "read entities" seam would hide it.
    scoped     func(ctx context.Context, typeName string, q map[string][]string) ([]*entity.Entity, error)
    graphCount func(ctx context.Context, q store.GraphQuery) (any, int, error)

    mu       sync.Mutex // guards the late-set pair below
    userState userstate.Store
    matchers  NextActionMatcherFunc
}
```

`scoped` is `App.scopedSortedEntities` passed as a closure. **Do not move
`scopedSortedEntities` into this type** — it has five consumers (gantt, export,
list, scope, next-action); it stays on App for this arc (see the epic).

## Invariants

- `redactedForSuggestion` moves WITH `queryCandidates`; splitting them leaves
a raw-entity path into a suggestion message.
- Late-set fields (`worlds`, `userState`, `matchers`) are closures or
mutex-guarded — never captured by value at construction.
`viewsHandler.faceEdges`' doc records the bug that captured-values cause.
- One `schema()` snapshot per handler; `nextActionEngine` (:290) already
documents why.

## Also

Ratchet `//plimsoll:max-methods` (app.go:172) to the new count. Constructor
takes explicit deps, rejects nil, no `app *App` parameter.
